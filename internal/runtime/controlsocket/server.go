package controlsocket

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"cderun/internal/container"
	"cderun/internal/logging"
	"cderun/internal/version"

	"github.com/docker/docker/pkg/stdcopy"
)

// ContainerRuntimeDispatcher defines the subset of container lifecycle operations needed for dispatch.
type ContainerRuntimeDispatcher interface {
	CreateContainer(ctx context.Context, config *container.ContainerConfig) (string, error)
	StartContainer(ctx context.Context, containerID string) error
	WaitContainer(ctx context.Context, containerID string) (int, error)
	RemoveContainer(ctx context.Context, containerID string) error
	AttachContainer(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error
	ResizeContainerTTY(ctx context.Context, containerID string, rows, cols uint) error
	SignalContainer(ctx context.Context, containerID string, sig string) error
}

type gatedWriter struct {
	mu     sync.Mutex
	w      io.Writer
	gated  bool
	buffer bytes.Buffer
}

func (g *gatedWriter) Write(p []byte) (int, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if !g.gated {
		return g.buffer.Write(p)
	}
	return g.w.Write(p)
}

func (g *gatedWriter) Enable() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.gated = true
	if g.buffer.Len() > 0 {
		_, err := g.w.Write(g.buffer.Bytes())
		g.buffer.Reset()
		return err
	}
	return nil
}

// Server handles Control Socket connections from nested cderun instances.
type Server struct {
	socketPath string
	listener   net.Listener
	mu         sync.Mutex
	conns      map[net.Conn]struct{}
	closed     chan struct{}
	wg         sync.WaitGroup
	logger     *logging.Logger
	dispatcher ContainerRuntimeDispatcher
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

// SetDispatcher configures the underlying ContainerRuntimeDispatcher for servicing RPC requests.
func (s *Server) SetDispatcher(dispatcher ContainerRuntimeDispatcher) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dispatcher = dispatcher
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

		if !s.registerConn(conn) {
			_ = conn.Close()
			continue
		}

		s.wg.Add(1)
		go func(c net.Conn) {
			defer s.wg.Done()
			defer s.unregisterConn(c)
			defer c.Close()

			s.handleConn(c)
		}(conn)
	}
}

func (s *Server) registerConn(c net.Conn) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	select {
	case <-s.closed:
		return false
	default:
		s.conns[c] = struct{}{}
		return true
	}
}

func (s *Server) unregisterConn(c net.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.conns, c)
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

	// 2. Request Loop
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

		if hijacked := s.dispatchRequest(conn, &reqFrame); hijacked {
			return
		}
	}
}

func (s *Server) buildRequestContext(reqFrame *RequestFrame) (context.Context, context.CancelFunc) {
	ctx := context.Background()
	if reqFrame.Deadline != nil && !reqFrame.Deadline.IsZero() {
		return context.WithDeadline(ctx, *reqFrame.Deadline)
	}
	return context.WithCancel(ctx)
}

func (s *Server) dispatchRequest(conn net.Conn, reqFrame *RequestFrame) bool {
	ctx, cancel := s.buildRequestContext(reqFrame)
	defer cancel()

	switch reqFrame.Type {
	case MsgPing:
		resp := ResponseFrame{Success: true, Payload: []byte("pong")}
		respBytes, _ := json.Marshal(resp)
		_ = WriteFrame(conn, respBytes)

	case MsgCreateContainer:
		s.handleCreateContainer(ctx, conn, reqFrame.Payload)

	case MsgStartContainer:
		s.handleStartContainer(ctx, conn, reqFrame.Payload)

	case MsgWaitContainer:
		s.handleWaitContainer(ctx, conn, reqFrame.Payload)

	case MsgRemoveContainer:
		s.handleRemoveContainer(ctx, conn, reqFrame.Payload)

	case MsgSignalContainer:
		s.handleSignalContainer(ctx, conn, reqFrame.Payload)

	case MsgResizeContainerTTY:
		s.handleResizeContainerTTY(ctx, conn, reqFrame.Payload)

	case MsgAttachContainer:
		s.handleAttachContainer(ctx, conn, reqFrame.Payload)
		return true

	default:
		s.sendErrorResponse(conn, fmt.Sprintf("unsupported request message type: %q", reqFrame.Type))
	}
	return false
}

func (s *Server) handleCreateContainer(ctx context.Context, conn net.Conn, payload []byte) {
	s.mu.Lock()
	d := s.dispatcher
	s.mu.Unlock()

	if d == nil {
		s.sendErrorResponse(conn, "server dispatcher not configured")
		return
	}

	var args CreateContainerArgs
	if err := json.Unmarshal(payload, &args); err != nil {
		s.sendErrorResponse(conn, fmt.Sprintf("malformed CreateContainer args: %v", err))
		return
	}

	if args.Config == nil {
		s.sendErrorResponse(conn, "CreateContainer args.Config is nil")
		return
	}

	containerID, err := d.CreateContainer(ctx, args.Config)
	if err != nil {
		s.sendErrorResponse(conn, err.Error())
		return
	}

	res := CreateContainerResult{ContainerID: containerID}
	resBytes, err := json.Marshal(res)
	if err != nil {
		s.sendErrorResponse(conn, fmt.Sprintf("failed to marshal CreateContainer result: %v", err))
		return
	}

	resp := ResponseFrame{Success: true, Payload: resBytes}
	respBytes, _ := json.Marshal(resp)
	_ = WriteFrame(conn, respBytes)
}

func (s *Server) handleStartContainer(ctx context.Context, conn net.Conn, payload []byte) {
	s.mu.Lock()
	d := s.dispatcher
	s.mu.Unlock()

	if d == nil {
		s.sendErrorResponse(conn, "server dispatcher not configured")
		return
	}

	var args ContainerIDArgs
	if err := json.Unmarshal(payload, &args); err != nil {
		s.sendErrorResponse(conn, fmt.Sprintf("malformed StartContainer args: %v", err))
		return
	}

	if err := d.StartContainer(ctx, args.ContainerID); err != nil {
		s.sendErrorResponse(conn, err.Error())
		return
	}

	resp := ResponseFrame{Success: true}
	respBytes, _ := json.Marshal(resp)
	_ = WriteFrame(conn, respBytes)
}

func (s *Server) handleWaitContainer(ctx context.Context, conn net.Conn, payload []byte) {
	s.mu.Lock()
	d := s.dispatcher
	s.mu.Unlock()

	if d == nil {
		s.sendErrorResponse(conn, "server dispatcher not configured")
		return
	}

	var args ContainerIDArgs
	if err := json.Unmarshal(payload, &args); err != nil {
		s.sendErrorResponse(conn, fmt.Sprintf("malformed WaitContainer args: %v", err))
		return
	}

	exitCode, err := d.WaitContainer(ctx, args.ContainerID)
	if err != nil {
		s.sendErrorResponse(conn, err.Error())
		return
	}

	res := WaitContainerResult{ExitCode: exitCode}
	resBytes, err := json.Marshal(res)
	if err != nil {
		s.sendErrorResponse(conn, fmt.Sprintf("failed to marshal WaitContainer result: %v", err))
		return
	}

	resp := ResponseFrame{Success: true, Payload: resBytes}
	respBytes, _ := json.Marshal(resp)
	_ = WriteFrame(conn, respBytes)
}

func (s *Server) handleRemoveContainer(ctx context.Context, conn net.Conn, payload []byte) {
	s.mu.Lock()
	d := s.dispatcher
	s.mu.Unlock()

	if d == nil {
		s.sendErrorResponse(conn, "server dispatcher not configured")
		return
	}

	var args ContainerIDArgs
	if err := json.Unmarshal(payload, &args); err != nil {
		s.sendErrorResponse(conn, fmt.Sprintf("malformed RemoveContainer args: %v", err))
		return
	}

	if err := d.RemoveContainer(ctx, args.ContainerID); err != nil {
		s.sendErrorResponse(conn, err.Error())
		return
	}

	resp := ResponseFrame{Success: true}
	respBytes, _ := json.Marshal(resp)
	_ = WriteFrame(conn, respBytes)
}

func (s *Server) handleSignalContainer(ctx context.Context, conn net.Conn, payload []byte) {
	s.mu.Lock()
	d := s.dispatcher
	s.mu.Unlock()

	if d == nil {
		s.sendErrorResponse(conn, "server dispatcher not configured")
		return
	}

	var args SignalContainerArgs
	if err := json.Unmarshal(payload, &args); err != nil {
		s.sendErrorResponse(conn, fmt.Sprintf("malformed SignalContainer args: %v", err))
		return
	}

	if err := d.SignalContainer(ctx, args.ContainerID, args.Signal); err != nil {
		s.sendErrorResponse(conn, err.Error())
		return
	}

	resp := ResponseFrame{Success: true}
	respBytes, _ := json.Marshal(resp)
	_ = WriteFrame(conn, respBytes)
}

func (s *Server) handleResizeContainerTTY(ctx context.Context, conn net.Conn, payload []byte) {
	s.mu.Lock()
	d := s.dispatcher
	s.mu.Unlock()

	if d == nil {
		s.sendErrorResponse(conn, "server dispatcher not configured")
		return
	}

	var args ResizeContainerTTYArgs
	if err := json.Unmarshal(payload, &args); err != nil {
		s.sendErrorResponse(conn, fmt.Sprintf("malformed ResizeContainerTTY args: %v", err))
		return
	}

	if err := d.ResizeContainerTTY(ctx, args.ContainerID, args.Rows, args.Cols); err != nil {
		s.sendErrorResponse(conn, err.Error())
		return
	}

	resp := ResponseFrame{Success: true}
	respBytes, _ := json.Marshal(resp)
	_ = WriteFrame(conn, respBytes)
}

func (s *Server) handleAttachContainer(ctx context.Context, conn net.Conn, payload []byte) {
	s.mu.Lock()
	d := s.dispatcher
	s.mu.Unlock()

	if d == nil {
		s.sendErrorResponse(conn, "server dispatcher not configured")
		return
	}

	var args AttachContainerArgs
	if err := json.Unmarshal(payload, &args); err != nil {
		s.sendErrorResponse(conn, fmt.Sprintf("malformed AttachContainer args: %v", err))
		return
	}

	readyChan := make(chan struct{})
	attachErrCh := make(chan error, 1)

	gw := &gatedWriter{w: conn}

	var stdinReader io.Reader = conn
	var stdoutWriter io.Writer
	var stderrWriter io.Writer

	if args.TTY {
		stdoutWriter = gw
		stderrWriter = gw
	} else {
		stdoutWriter = stdcopy.NewStdWriter(gw, stdcopy.Stdout)
		stderrWriter = stdcopy.NewStdWriter(gw, stdcopy.Stderr)
	}

	go func() {
		attachErrCh <- d.AttachContainer(ctx, args.ContainerID, args.TTY, stdinReader, stdoutWriter, stderrWriter, readyChan)
	}()

	select {
	case <-readyChan:
		// Send initial response acknowledging AttachContainer request only after setup signals readiness
		resp := ResponseFrame{Success: true}
		respBytes, _ := json.Marshal(resp)
		if err := WriteFrame(conn, respBytes); err != nil {
			s.logger.Warn("Failed to send AttachContainer response frame: %v", err)
			return
		}
		_ = gw.Enable()
		if err := <-attachErrCh; err != nil {
			s.logger.Debug("AttachContainer dispatcher returned error for container %s: %v", args.ContainerID, err)
		}
	case err := <-attachErrCh:
		if err == nil {
			resp := ResponseFrame{Success: true}
			respBytes, _ := json.Marshal(resp)
			_ = WriteFrame(conn, respBytes)
			_ = gw.Enable()
		} else {
			s.sendErrorResponse(conn, err.Error())
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
