package controlsocket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/docker/docker/pkg/stdcopy"

	"cderun/internal/container"
	"cderun/internal/logging"
	"cderun/internal/version"
)

// ContainerRuntimeDispatcher defines the subset of container lifecycle operations needed for dispatch.
type ContainerRuntimeDispatcher interface {
	CreateContainer(ctx context.Context, config *container.ContainerConfig) (string, error)
	StartContainer(ctx context.Context, containerID string) error
	WaitContainer(ctx context.Context, containerID string) (int, error)
	RemoveContainer(ctx context.Context, containerID string) error
	AttachContainer(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error
	SignalContainer(ctx context.Context, containerID string, sig string) error
	ResizeContainerTTY(ctx context.Context, containerID string, rows, cols uint) error
}

type connState struct {
	net.Conn
	writeMu sync.Mutex
}

func (c *connState) WriteFrame(payload []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return WriteFrame(c.Conn, payload)
}

// Server handles Control Socket connections from nested cderun instances.
type Server struct {
	socketPath     string
	listener       net.Listener
	mu             sync.Mutex
	conns          map[net.Conn]struct{}
	closed         chan struct{}
	wg             sync.WaitGroup
	logger         *logging.Logger
	dispatcher     ContainerRuntimeDispatcher
	ctx            context.Context
	cancelCtx      context.CancelFunc
	closeTimeout   time.Duration
	activeHandlers map[string]time.Time
	handlerSeq     uint64
}

// NewServer creates a new Control Socket Server for the specified socketPath.
func NewServer(socketPath string, logger *logging.Logger) *Server {
	if logger == nil {
		logger = logging.GetGlobalLogger()
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		socketPath:     socketPath,
		conns:          make(map[net.Conn]struct{}),
		closed:         make(chan struct{}),
		logger:         logger,
		ctx:            ctx,
		cancelCtx:      cancel,
		closeTimeout:   5 * time.Second,
		activeHandlers: make(map[string]time.Time),
	}
}

// SetCloseTimeout configures the maximum time Server.Close() waits for active RPC handlers to finish.
func (s *Server) SetCloseTimeout(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closeTimeout = d
}

// SetDispatcher configures the underlying ContainerRuntimeDispatcher for servicing RPC requests.
func (s *Server) SetDispatcher(dispatcher ContainerRuntimeDispatcher) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dispatcher = dispatcher
}

// ActiveHandlerCount returns the number of active or lingering RPC handlers.
func (s *Server) ActiveHandlerCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.activeHandlers)
}

func (s *Server) trackHandlerStart(rpcName string) (uint64, func()) {
	s.mu.Lock()
	s.handlerSeq++
	id := s.handlerSeq
	key := fmt.Sprintf("%s#%d", rpcName, id)
	if s.activeHandlers == nil {
		s.activeHandlers = make(map[string]time.Time)
	}
	s.activeHandlers[key] = time.Now()
	s.mu.Unlock()

	return id, func() {
		s.mu.Lock()
		delete(s.activeHandlers, key)
		s.mu.Unlock()
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

		if !s.registerConn(conn) {
			_ = conn.Close()
			continue
		}

		s.wg.Add(1)
		go func(c net.Conn) {
			defer s.wg.Done()
			defer s.unregisterConn(c)
			defer c.Close()

			cs := &connState{Conn: c}
			s.handleConn(cs)
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

func (s *Server) handleConn(cs *connState) {
	s.mu.Lock()
	serverCtx := s.ctx
	s.mu.Unlock()

	connCtx, cancelConn := context.WithCancel(serverCtx)
	defer cancelConn()

	// 1. Handshake Phase
	if err := cs.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		s.logger.Warn("Failed to set deadline for handshake: %v", err)
		return
	}

	reqBytes, err := ReadFrame(cs.Conn)
	if err != nil {
		s.logger.Warn("Failed to read handshake request: %v", err)
		return
	}

	var req HandshakeRequest
	if err := json.Unmarshal(reqBytes, &req); err != nil {
		s.logger.Warn("Malformed handshake request payload: %v", err)
		_ = s.sendHandshakeResponse(cs, false, fmt.Sprintf("invalid handshake payload: %v", err))
		return
	}

	if req.ProtocolVersion != CurrentProtocolVersion {
		errMsg := fmt.Sprintf("unsupported protocol version: client requested v%d, server supports v%d", req.ProtocolVersion, CurrentProtocolVersion)
		s.logger.Warn("Handshake rejected: %s (Client version: %s)", errMsg, req.ClientVersion)
		_ = s.sendHandshakeResponse(cs, false, errMsg)
		return
	}

	// Protocol version matches. Check binary release version skew for diagnostics logging.
	serverVer := version.Version
	if req.ClientVersion != serverVer {
		s.logger.Debug("Control socket version skew detected: client cderun version=%s, server cderun version=%s (both speak protocol v%d)", req.ClientVersion, serverVer, CurrentProtocolVersion)
	}

	if err := s.sendHandshakeResponse(cs, true, ""); err != nil {
		s.logger.Warn("Failed to send handshake response: %v", err)
		return
	}

	// Reset read deadline for normal operation
	if err := cs.SetReadDeadline(time.Time{}); err != nil {
		s.logger.Warn("Failed to clear read deadline after handshake: %v", err)
	}

	// 2. Request Loop (Single connection-owning loop)
	for {
		frameBytes, err := ReadFrame(cs.Conn)
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
				cancelConn()
				return
			}
			s.logger.Debug("Control socket frame read error: %v", err)
			cancelConn()
			return
		}

		var reqFrame RequestFrame
		if err := json.Unmarshal(frameBytes, &reqFrame); err != nil {
			s.sendErrorResponse(cs, fmt.Sprintf("invalid request frame: %v", err))
			continue
		}

		if reqFrame.Type == MsgAttachContainer {
			s.dispatchRequest(cs, connCtx, cancelConn, &reqFrame)
		} else {
			s.wg.Add(1)
			go func(rf RequestFrame) {
				defer s.wg.Done()
				s.dispatchRequest(cs, connCtx, cancelConn, &rf)
			}(reqFrame)
		}
	}
}

func (s *Server) buildRequestContext(parentCtx context.Context, reqFrame *RequestFrame) (context.Context, context.CancelFunc) {
	if parentCtx == nil {
		parentCtx = s.ctx
	}
	if reqFrame.Deadline != nil && !reqFrame.Deadline.IsZero() {
		return context.WithDeadline(parentCtx, *reqFrame.Deadline)
	}
	return context.WithCancel(parentCtx)
}

func (s *Server) getDispatcher() ContainerRuntimeDispatcher {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dispatcher
}

func (s *Server) dispatchRequest(cs *connState, connCtx context.Context, cancelConn context.CancelFunc, reqFrame *RequestFrame) {
	reqCtx, cancelReq := s.buildRequestContext(connCtx, reqFrame)
	defer cancelReq()

	_, doneHandler := s.trackHandlerStart(string(reqFrame.Type))
	defer doneHandler()

	switch reqFrame.Type {
	case MsgPing:
		_ = s.sendSuccessResponse(cs, []byte("pong"))

	case MsgCreateContainer:
		s.handleCreateContainer(reqCtx, cs, reqFrame.Payload)

	case MsgStartContainer:
		s.handleStartContainer(reqCtx, cs, reqFrame.Payload)

	case MsgWaitContainer:
		s.handleWaitContainer(reqCtx, cs, reqFrame.Payload)

	case MsgRemoveContainer:
		s.handleRemoveContainer(reqCtx, cs, reqFrame.Payload)

	case MsgAttachContainer:
		s.handleAttachContainer(reqCtx, cs, cancelConn, reqFrame.Payload)

	case MsgSignalContainer:
		s.handleSignalContainer(reqCtx, cs, reqFrame.Payload)

	case MsgResizeContainerTTY:
		s.handleResizeContainerTTY(reqCtx, cs, reqFrame.Payload)

	default:
		s.sendErrorResponse(cs, fmt.Sprintf("unsupported request message type: %q", reqFrame.Type))
	}
}

// handleRPCWithResult abstracts RPC request handling for operations returning a payload.
func handleRPCWithResult[TArgs any, TRes any](
	s *Server,
	ctx context.Context,
	cs *connState,
	payload []byte,
	rpcName string,
	fn func(ctx context.Context, d ContainerRuntimeDispatcher, args TArgs) (TRes, error),
) {
	d := s.getDispatcher()
	if d == nil {
		s.sendErrorResponse(cs, "server dispatcher not configured")
		return
	}

	var args TArgs
	if err := json.Unmarshal(payload, &args); err != nil {
		s.sendErrorResponse(cs, fmt.Sprintf("malformed %s args: %v", rpcName, err))
		return
	}

	res, err := fn(ctx, d, args)
	if err != nil {
		s.sendErrorResponse(cs, err.Error())
		return
	}

	resBytes, err := json.Marshal(res)
	if err != nil {
		s.sendErrorResponse(cs, fmt.Sprintf("failed to marshal %s result: %v", rpcName, err))
		return
	}

	_ = s.sendSuccessResponse(cs, resBytes)
}

// handleRPCAction abstracts RPC request handling for operations returning only success status.
func handleRPCAction[TArgs any](
	s *Server,
	ctx context.Context,
	cs *connState,
	payload []byte,
	rpcName string,
	fn func(ctx context.Context, d ContainerRuntimeDispatcher, args TArgs) error,
) {
	d := s.getDispatcher()
	if d == nil {
		s.sendErrorResponse(cs, "server dispatcher not configured")
		return
	}

	var args TArgs
	if err := json.Unmarshal(payload, &args); err != nil {
		s.sendErrorResponse(cs, fmt.Sprintf("malformed %s args: %v", rpcName, err))
		return
	}

	if err := fn(ctx, d, args); err != nil {
		s.sendErrorResponse(cs, err.Error())
		return
	}

	_ = s.sendSuccessResponse(cs, nil)
}

func (s *Server) handleCreateContainer(ctx context.Context, cs *connState, payload []byte) {
	handleRPCWithResult(s, ctx, cs, payload, "CreateContainer", func(ctx context.Context, d ContainerRuntimeDispatcher, args CreateContainerArgs) (CreateContainerResult, error) {
		if args.Config == nil {
			return CreateContainerResult{}, errors.New("CreateContainer args.Config is nil")
		}
		containerID, err := d.CreateContainer(ctx, args.Config)
		if err != nil {
			return CreateContainerResult{}, err
		}
		return CreateContainerResult{ContainerID: containerID}, nil
	})
}

func (s *Server) handleStartContainer(ctx context.Context, cs *connState, payload []byte) {
	handleRPCAction(s, ctx, cs, payload, "StartContainer", func(ctx context.Context, d ContainerRuntimeDispatcher, args ContainerIDArgs) error {
		return d.StartContainer(ctx, args.ContainerID)
	})
}

func (s *Server) handleWaitContainer(ctx context.Context, cs *connState, payload []byte) {
	handleRPCWithResult(s, ctx, cs, payload, "WaitContainer", func(ctx context.Context, d ContainerRuntimeDispatcher, args ContainerIDArgs) (WaitContainerResult, error) {
		exitCode, err := d.WaitContainer(ctx, args.ContainerID)
		if err != nil {
			return WaitContainerResult{}, err
		}
		return WaitContainerResult{ExitCode: exitCode}, nil
	})
}

func (s *Server) handleRemoveContainer(ctx context.Context, cs *connState, payload []byte) {
	handleRPCAction(s, ctx, cs, payload, "RemoveContainer", func(ctx context.Context, d ContainerRuntimeDispatcher, args ContainerIDArgs) error {
		return d.RemoveContainer(ctx, args.ContainerID)
	})
}

func (s *Server) handleSignalContainer(ctx context.Context, cs *connState, payload []byte) {
	handleRPCAction(s, ctx, cs, payload, "SignalContainer", func(ctx context.Context, d ContainerRuntimeDispatcher, args SignalContainerArgs) error {
		return d.SignalContainer(ctx, args.ContainerID, args.Signal)
	})
}

func (s *Server) handleResizeContainerTTY(ctx context.Context, cs *connState, payload []byte) {
	handleRPCAction(s, ctx, cs, payload, "ResizeContainerTTY", func(ctx context.Context, d ContainerRuntimeDispatcher, args ResizeContainerTTYArgs) error {
		return d.ResizeContainerTTY(ctx, args.ContainerID, args.Rows, args.Cols)
	})
}

func (s *Server) handleAttachContainer(ctx context.Context, cs *connState, cancelConn context.CancelFunc, payload []byte) {
	d := s.getDispatcher()
	if d == nil {
		s.sendErrorResponse(cs, "server dispatcher not configured")
		return
	}

	var args AttachContainerArgs
	if err := json.Unmarshal(payload, &args); err != nil {
		s.sendErrorResponse(cs, fmt.Sprintf("malformed AttachContainer args: %v", err))
		return
	}

	var stdinReader io.Reader
	if args.HasStdin {
		stdinReader = cs.Conn
	} else {
		go func() {
			sc, ok := cs.Conn.(syscall.Conn)
			if ok {
				if rawConn, err := sc.SyscallConn(); err == nil {
					buf := make([]byte, 1)
					_ = rawConn.Read(func(fd uintptr) bool {
						n, _, pErr := syscall.Recvfrom(int(fd), buf, syscall.MSG_PEEK)
						if n == 0 && pErr == nil {
							cancelConn()
							return true
						}
						if pErr != nil {
							if errors.Is(pErr, syscall.EAGAIN) || errors.Is(pErr, syscall.EWOULDBLOCK) {
								return false
							}
							cancelConn()
							return true
						}
						return false
					})
				}
			}
		}()
	}

	gate := make(chan struct{})
	var releaseGateOnce sync.Once
	releaseGate := func() {
		releaseGateOnce.Do(func() {
			close(gate)
		})
	}
	defer releaseGate()
	defer cs.Close()

	var stdoutWriter, stderrWriter io.Writer
	if args.TTY {
		stdoutWriter = &gateWriter{w: cs.Conn, gate: gate}
		stderrWriter = &gateWriter{w: cs.Conn, gate: gate}
	} else {
		stdoutWriter = &gateWriter{w: stdcopy.NewStdWriter(cs.Conn, stdcopy.Stdout), gate: gate}
		stderrWriter = &gateWriter{w: stdcopy.NewStdWriter(cs.Conn, stdcopy.Stderr), gate: gate}
	}

	readyChan := make(chan struct{})
	startRes := make(chan error, 1)
	streamErrChan := make(chan error, 1)

	go func() {
		err := d.AttachContainer(ctx, args.ContainerID, args.TTY, stdinReader, stdoutWriter, stderrWriter, readyChan)
		startRes <- err
		streamErrChan <- err
	}()

	select {
	case err := <-startRes:
		if err != nil {
			s.sendErrorResponse(cs, err.Error())
			return
		}
	case <-readyChan:
		select {
		case err := <-startRes:
			if err != nil {
				s.sendErrorResponse(cs, err.Error())
				return
			}
		default:
		}
	case <-ctx.Done():
		s.sendErrorResponse(cs, ctx.Err().Error())
		return
	}

	if err := s.sendSuccessResponse(cs, nil); err != nil {
		s.logger.Warn("Failed to send AttachContainer success response: %v", err)
		return
	}
	releaseGate()

	select {
	case err := <-streamErrChan:
		if err != nil {
			s.logger.Debug("AttachContainer streaming finished with error: %v", err)
		}
	case <-ctx.Done():
		s.logger.Debug("AttachContainer context canceled")
	}
}

type gateWriter struct {
	w    io.Writer
	gate <-chan struct{}
}

func (g *gateWriter) Write(p []byte) (int, error) {
	<-g.gate
	return g.w.Write(p)
}

func (s *Server) sendHandshakeResponse(cs *connState, accepted bool, errMsg string) error {
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
	return cs.WriteFrame(data)
}

func (s *Server) sendSuccessResponse(cs *connState, payload []byte) error {
	resp := ResponseFrame{Success: true, Payload: payload}
	respBytes, _ := json.Marshal(resp)
	return cs.WriteFrame(respBytes)
}

func (s *Server) sendErrorResponse(cs *connState, errMsg string) {
	resp := ResponseFrame{Success: false, Error: errMsg}
	data, _ := json.Marshal(resp)
	_ = cs.WriteFrame(data)
}

// CloseWithTimeout gracefully stops the listener, cancels all request contexts, closes active connections,
// and performs a bounded wait up to timeout for handlers to finish.
func (s *Server) CloseWithTimeout(timeout time.Duration) error {
	s.mu.Lock()
	select {
	case <-s.closed:
		s.mu.Unlock()
		return nil
	default:
		close(s.closed)
	}

	if s.cancelCtx != nil {
		s.cancelCtx()
	}

	if s.listener != nil {
		_ = s.listener.Close()
	}

	for c := range s.conns {
		_ = c.Close()
	}
	s.mu.Unlock()

	// Bounded wait for handler goroutines
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	s.mu.Lock()
	if timeout <= 0 {
		timeout = s.closeTimeout
	}
	s.mu.Unlock()
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	select {
	case <-done:
		s.logger.Debug("Control Socket server stopped gracefully")
	case <-time.After(timeout):
		count := s.ActiveHandlerCount()
		s.logger.Warn("Control Socket server Close timed out after %v with %d lingering handler(s)", timeout, count)
	}

	// Clean up socket file
	if s.socketPath != "" {
		if err := os.Remove(s.socketPath); err != nil && !os.IsNotExist(err) {
			s.logger.Warn("Failed to remove control socket file %s on teardown: %v", s.socketPath, err)
		}
	}

	s.logger.Debug("Control Socket server stopped for %s", s.socketPath)
	return nil
}

// Close gracefully stops the listener, closes all active connections, and removes the socket file.
func (s *Server) Close() error {
	s.mu.Lock()
	timeout := s.closeTimeout
	s.mu.Unlock()
	return s.CloseWithTimeout(timeout)
}
