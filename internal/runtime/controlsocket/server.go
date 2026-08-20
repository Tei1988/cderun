package controlsocket

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"cderun/internal/logging"
	"cderun/internal/version"
)

// Server handles Control Socket connections from nested cderun instances.
type Server struct {
	socketPath string
	listener   net.Listener
	mu         sync.Mutex
	conns      map[net.Conn]struct{}
	closed     chan struct{}
	wg         sync.WaitGroup
	logger     *logging.Logger
}

// NewServer creates a new Control Socket Server for the specified socketPath.
func NewServer(socketPath string, logger *logging.Logger) *Server {
	if logger == nil {
		logger = logging.GetGlobalLogger()
	}
	return &Server{
		socketPath: socketPath,
		conns:      make(map[net.Conn]struct{}),
		closed:     make(chan struct{}),
		logger:     logger,
	}
}

// Start opens the Unix domain socket and starts accepting incoming connections in a background goroutine.
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.listener != nil {
		return errors.New("server is already running")
	}

	// Clean up stale socket file if it exists
	if err := os.Remove(s.socketPath); err != nil && !os.IsNotExist(err) {
		s.logger.Warn("Failed to remove existing control socket at %s: %v", s.socketPath, err)
	}

	listener, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("failed to listen on control socket %s: %w", s.socketPath, err)
	}

	// Restrict permissions on socket file
	if err := os.Chmod(s.socketPath, 0600); err != nil {
		s.logger.Warn("Failed to set 0600 permissions on control socket %s: %v", s.socketPath, err)
	}

	s.listener = listener
	s.wg.Add(1)
	go s.acceptLoop()

	s.logger.Debug("Control Socket server listening on %s (Protocol v%d)", s.socketPath, CurrentProtocolVersion)
	return nil
}

func (s *Server) acceptLoop() {
	defer s.wg.Done()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.closed:
				return
			default:
				s.logger.Debug("Accept error on control socket: %v", err)
				return
			}
		}

		s.trackConn(conn, true)
		s.wg.Add(1)
		go func(c net.Conn) {
			defer s.wg.Done()
			defer s.trackConn(c, false)
			defer c.Close()

			s.handleConn(c)
		}(conn)
	}
}

func (s *Server) trackConn(c net.Conn, add bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if add {
		s.conns[c] = struct{}{}
	} else {
		delete(s.conns, c)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	// 1. Handshake Phase
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		s.logger.Warn("Failed to set deadline for handshake: %v", err)
		return
	}

	reqBytes, err := ReadFrame(conn)
	if err != nil {
		s.logger.Warn("Failed to read handshake request: %v", err)
		return
	}

	var req HandshakeRequest
	if err := json.Unmarshal(reqBytes, &req); err != nil {
		s.logger.Warn("Malformed handshake request payload: %v", err)
		_ = s.sendHandshakeResponse(conn, false, fmt.Sprintf("invalid handshake payload: %v", err))
		return
	}

	if req.ProtocolVersion != CurrentProtocolVersion {
		errMsg := fmt.Sprintf("unsupported protocol version: client requested v%d, server supports v%d", req.ProtocolVersion, CurrentProtocolVersion)
		s.logger.Warn("Handshake rejected: %s (Client version: %s)", errMsg, req.ClientVersion)
		_ = s.sendHandshakeResponse(conn, false, errMsg)
		return
	}

	// Protocol version matches. Check binary release version skew for diagnostics logging.
	serverVer := version.Version
	if req.ClientVersion != serverVer {
		s.logger.Debug("Control socket version skew detected: client cderun version=%s, server cderun version=%s (both speak protocol v%d)", req.ClientVersion, serverVer, CurrentProtocolVersion)
	}

	if err := s.sendHandshakeResponse(conn, true, ""); err != nil {
		s.logger.Warn("Failed to send handshake response: %v", err)
		return
	}

	// Reset read deadline for normal operation
	if err := conn.SetReadDeadline(time.Time{}); err != nil {
		s.logger.Warn("Failed to clear read deadline after handshake: %v", err)
	}

	// 2. Request Loop (Phase 1 supports Ping/Pong)
	for {
		frameBytes, err := ReadFrame(conn)
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
				return
			}
			s.logger.Debug("Control socket frame read error: %v", err)
			return
		}

		var reqFrame RequestFrame
		if err := json.Unmarshal(frameBytes, &reqFrame); err != nil {
			s.sendErrorResponse(conn, fmt.Sprintf("invalid request frame: %v", err))
			continue
		}

		switch reqFrame.Type {
		case MsgPing:
			resp := ResponseFrame{Success: true, Payload: []byte("pong")}
			respBytes, _ := json.Marshal(resp)
			if err := WriteFrame(conn, respBytes); err != nil {
				return
			}
		default:
			s.sendErrorResponse(conn, fmt.Sprintf("unsupported request message type: %q", reqFrame.Type))
		}
	}
}

func (s *Server) sendHandshakeResponse(conn net.Conn, accepted bool, errMsg string) error {
	resp := HandshakeResponse{
		Accepted:        accepted,
		ProtocolVersion: CurrentProtocolVersion,
		ServerVersion:   version.Version,
		Error:           errMsg,
	}
	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	return WriteFrame(conn, data)
}

func (s *Server) sendErrorResponse(conn net.Conn, errMsg string) {
	resp := ResponseFrame{Success: false, Error: errMsg}
	data, _ := json.Marshal(resp)
	_ = WriteFrame(conn, data)
}

// Close gracefully stops the listener, closes all active connections, and removes the socket file.
func (s *Server) Close() error {
	s.mu.Lock()
	select {
	case <-s.closed:
		s.mu.Unlock()
		return nil
	default:
		close(s.closed)
	}

	if s.listener != nil {
		_ = s.listener.Close()
	}

	for c := range s.conns {
		_ = c.Close()
	}
	s.mu.Unlock()

	s.wg.Wait()

	// Clean up socket file
	if s.socketPath != "" {
		if err := os.Remove(s.socketPath); err != nil && !os.IsNotExist(err) {
			s.logger.Warn("Failed to remove control socket file %s on teardown: %v", s.socketPath, err)
		}
	}

	s.logger.Debug("Control Socket server stopped for %s", s.socketPath)
	return nil
}
