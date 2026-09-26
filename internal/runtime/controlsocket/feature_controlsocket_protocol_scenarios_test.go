package controlsocket

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/container"
	"cderun/internal/logging"
)

type dummyDispatcher struct {
	createFn func(ctx context.Context, config *container.ContainerConfig) (string, error)
	startFn  func(ctx context.Context, containerID string) error
	waitFn   func(ctx context.Context, containerID string) (int, error)
	removeFn func(ctx context.Context, containerID string) error
	attachFn func(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error
	signalFn func(ctx context.Context, containerID string, sig string) error
	resizeFn func(ctx context.Context, containerID string, rows, cols uint) error
}

func (d *dummyDispatcher) CreateContainer(ctx context.Context, config *container.ContainerConfig) (string, error) {
	if d.createFn != nil {
		return d.createFn(ctx, config)
	}
	return "test-cid", nil
}

func (d *dummyDispatcher) StartContainer(ctx context.Context, containerID string) error {
	if d.startFn != nil {
		return d.startFn(ctx, containerID)
	}
	return nil
}

func (d *dummyDispatcher) WaitContainer(ctx context.Context, containerID string) (int, error) {
	if d.waitFn != nil {
		return d.waitFn(ctx, containerID)
	}
	return 0, nil
}

func (d *dummyDispatcher) RemoveContainer(ctx context.Context, containerID string) error {
	if d.removeFn != nil {
		return d.removeFn(ctx, containerID)
	}
	return nil
}

func (d *dummyDispatcher) AttachContainer(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error {
	if d.attachFn != nil {
		return d.attachFn(ctx, containerID, tty, stdin, stdout, stderr, ready)
	}
	if ready != nil {
		close(ready)
	}
	return nil
}

func (d *dummyDispatcher) SignalContainer(ctx context.Context, containerID string, sig string) error {
	if d.signalFn != nil {
		return d.signalFn(ctx, containerID, sig)
	}
	return nil
}

func (d *dummyDispatcher) ResizeContainerTTY(ctx context.Context, containerID string, rows, cols uint) error {
	if d.resizeFn != nil {
		return d.resizeFn(ctx, containerID, rows, cols)
	}
	return nil
}

func TestUnit_ControlSocket_Server_LifecycleAndHandlerTracking(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	sockPath := filepath.Join(tmpDir, "ctrl.sock")

	logger := logging.GetGlobalLogger()
	srv := NewServer(sockPath, logger)
	srv.SetCloseTimeout(100 * time.Millisecond)
	srv.SetIdleTimeout(500 * time.Millisecond)

	assert.Equal(t, 500*time.Millisecond, srv.getIdleTimeout())
	assert.Equal(t, 0, srv.ActiveHandlerCount())

	err := srv.Start()
	require.NoError(t, err)

	// Second Start() call should fail
	err = srv.Start()
	assert.ErrorContains(t, err, "server is already running")

	// Manual handler tracking check
	id, done := srv.trackHandlerStart("TestRPC")
	assert.Greater(t, id, uint64(0))
	assert.Equal(t, 1, srv.ActiveHandlerCount())
	done()
	assert.Equal(t, 0, srv.ActiveHandlerCount())

	// Start a simulated lingering handler before closing with timeout
	blockChan := make(chan struct{})
	srv.wg.Add(1)
	go func() {
		defer srv.wg.Done()
		_, finish := srv.trackHandlerStart("BlockingRPC")
		defer finish()
		<-blockChan
	}()

	time.Sleep(20 * time.Millisecond)
	assert.Equal(t, 1, srv.ActiveHandlerCount())

	// CloseWithTimeout should time out and return after ~100ms
	start := time.Now()
	err = srv.CloseWithTimeout(100 * time.Millisecond)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, time.Since(start), 90*time.Millisecond)

	// Clean up lingering goroutine
	close(blockChan)

	// Duplicate Close should be a no-op
	err = srv.Close()
	assert.NoError(t, err)
}

func TestUnit_ControlSocket_MalformedPayloadAndDispatcherErrors(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	sockPath := filepath.Join(tmpDir, "ctrl.sock")

	srv := NewServer(sockPath, nil)
	require.NoError(t, srv.Start())
	t.Cleanup(func() { _ = srv.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// 1. Connect client without setting a dispatcher on server
	client, err := Connect(ctx, sockPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })

	_, err = client.CreateContainer(ctx, &container.ContainerConfig{Image: "alpine"})
	assert.ErrorContains(t, err, "server dispatcher not configured")

	// 2. Set dispatcher and test CreateContainer with nil config
	disp := &dummyDispatcher{}
	srv.SetDispatcher(disp)

	_, err = client.CreateContainer(ctx, nil)
	assert.ErrorContains(t, err, "CreateContainer args.Config is nil")

	// 3. Send malformed raw JSON payload frames directly over connection
	rawConn, err := net.Dial("unix", sockPath)
	require.NoError(t, err)
	defer rawConn.Close()

	// Handshake
	hsReq, _ := json.Marshal(HandshakeRequest{ProtocolVersion: CurrentProtocolVersion, ClientVersion: "test"})
	require.NoError(t, WriteFrame(rawConn, hsReq))
	hsRespBytes, err := ReadFrame(rawConn)
	require.NoError(t, err)
	var hsResp HandshakeResponse
	require.NoError(t, json.Unmarshal(hsRespBytes, &hsResp))
	require.True(t, hsResp.Accepted)

	// Send malformed payload for CreateContainer
	badReq, _ := json.Marshal(RequestFrame{
		Type:    MsgCreateContainer,
		Payload: []byte("invalid-json"),
	})
	require.NoError(t, WriteFrame(rawConn, badReq))

	respBytes, err := ReadFrame(rawConn)
	require.NoError(t, err)
	var resp ResponseFrame
	require.NoError(t, json.Unmarshal(respBytes, &resp))
	assert.False(t, resp.Success)
	assert.Contains(t, resp.Error, "malformed CreateContainer args")

	// Send unsupported message type
	unsupportedReq, _ := json.Marshal(RequestFrame{
		Type:    MessageType("MsgUnknownAction"),
		Payload: nil,
	})
	require.NoError(t, WriteFrame(rawConn, unsupportedReq))

	respBytes, err = ReadFrame(rawConn)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(respBytes, &resp))
	assert.False(t, resp.Success)
	assert.Contains(t, resp.Error, "unsupported request message type")
}

func TestUnit_ControlSocket_Client_RPCDeadlinesAndVersionSkew(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	sockPath := filepath.Join(tmpDir, "ctrl.sock")

	srv := NewServer(sockPath, nil)
	slowDisp := &dummyDispatcher{
		startFn: func(ctx context.Context, containerID string) error {
			time.Sleep(300 * time.Millisecond)
			return nil
		},
	}
	srv.SetDispatcher(slowDisp)
	require.NoError(t, srv.Start())
	t.Cleanup(func() { _ = srv.Close() })

	// Connect client
	clientCtx, cancelClient := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelClient()

	client, err := Connect(clientCtx, sockPath)
	require.NoError(t, err)
	assert.NotEmpty(t, client.ServerVersion())

	// Client RPC call with a tight context deadline
	tightCtx, cancelTight := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancelTight()

	err = client.StartContainer(tightCtx, "c123")
	assert.Error(t, err)

	// Client Close with nil conn or active conn
	emptyClient := &Client{}
	assert.NoError(t, emptyClient.Close())
	assert.NoError(t, client.Close())
}

func TestUnit_ControlSocket_InternalHelpers(t *testing.T) {
	t.Parallel()

	// copyErrOrNil test cases
	assert.Nil(t, copyErrOrNil(io.EOF))
	assert.Nil(t, copyErrOrNil(net.ErrClosed))
	customErr := errors.New("stream error")
	assert.Equal(t, customErr, copyErrOrNil(customErr))
	assert.Nil(t, copyErrOrNil(nil))

	// isTemporaryAcceptError test cases
	assert.False(t, isTemporaryAcceptError(nil))
	assert.True(t, isTemporaryAcceptError(syscall.EAGAIN))
	assert.True(t, isTemporaryAcceptError(syscall.EMFILE))
	assert.True(t, isTemporaryAcceptError(syscall.ENOMEM))
	assert.True(t, isTemporaryAcceptError(syscall.ETIMEDOUT))
	assert.False(t, isTemporaryAcceptError(syscall.ECONNRESET))
	assert.False(t, isTemporaryAcceptError(os.ErrNotExist))
}

func TestUnit_ControlSocket_HandshakeRejection(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	sockPath := filepath.Join(tmpDir, "ctrl.sock")

	srv := NewServer(sockPath, nil)
	require.NoError(t, srv.Start())
	t.Cleanup(func() { _ = srv.Close() })

	// Send handshake request with incompatible protocol version
	rawConn, err := net.Dial("unix", sockPath)
	require.NoError(t, err)
	defer rawConn.Close()

	badHs, _ := json.Marshal(HandshakeRequest{
		ProtocolVersion: CurrentProtocolVersion + 99,
		ClientVersion:   "0.0.1-dev",
	})
	require.NoError(t, WriteFrame(rawConn, badHs))

	respBytes, err := ReadFrame(rawConn)
	require.NoError(t, err)

	var hsResp HandshakeResponse
	require.NoError(t, json.Unmarshal(respBytes, &hsResp))
	assert.False(t, hsResp.Accepted)
	assert.Contains(t, hsResp.Error, "unsupported protocol version")
}
