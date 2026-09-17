package controlsocket

import (
	"context"
	"errors"
	"io"
	"net"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/container"
	"cderun/internal/logging"
)

type tempNetTimeoutErr struct{}

func (tempNetTimeoutErr) Error() string   { return "timeout" }
func (tempNetTimeoutErr) Timeout() bool   { return true }
func (tempNetTimeoutErr) Temporary() bool { return true }

type customErrReader struct {
	err error
}

func (r *customErrReader) Read(p []byte) (int, error) {
	return 0, r.err
}

type mockDispatcherForStreamTest struct {
	attachErr error
}

func (m *mockDispatcherForStreamTest) CreateContainer(ctx context.Context, config *container.ContainerConfig) (string, error) {
	return "c-123", nil
}

func (m *mockDispatcherForStreamTest) StartContainer(ctx context.Context, containerID string) error {
	return nil
}

func (m *mockDispatcherForStreamTest) WaitContainer(ctx context.Context, containerID string) (int, error) {
	return 0, nil
}

func (m *mockDispatcherForStreamTest) RemoveContainer(ctx context.Context, containerID string) error {
	return nil
}

func (m *mockDispatcherForStreamTest) AttachContainer(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error {
	if ready != nil {
		close(ready)
	}
	if stdin != nil {
		buf := make([]byte, 10)
		_, err := stdin.Read(buf)
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
	}
	if stdout != nil {
		_, _ = stdout.Write([]byte("out"))
	}
	if stderr != nil {
		_, _ = stderr.Write([]byte("err"))
	}
	return m.attachErr
}

func (m *mockDispatcherForStreamTest) SignalContainer(ctx context.Context, containerID string, sig string) error {
	return nil
}

func (m *mockDispatcherForStreamTest) ResizeContainerTTY(ctx context.Context, containerID string, rows, cols uint) error {
	return nil
}

func TestUnit_ControlSocket_IsTemporaryAcceptError_EdgeCases(t *testing.T) {
	assert.False(t, isTemporaryAcceptError(nil))
	assert.True(t, isTemporaryAcceptError(tempNetTimeoutErr{}))

	temporarySyscalls := []syscall.Errno{
		syscall.ECONNABORTED,
		syscall.EMFILE,
		syscall.ENFILE,
		syscall.EINTR,
		syscall.ENOBUFS,
		syscall.ENOMEM,
		syscall.ETIMEDOUT,
		syscall.EAGAIN,
	}

	for _, sysErr := range temporarySyscalls {
		assert.True(t, isTemporaryAcceptError(sysErr), "expected %v to be temporary accept error", sysErr)
	}

	permanentSyscalls := []syscall.Errno{
		syscall.EPERM,
		syscall.EACCES,
		syscall.EBADF,
	}

	for _, sysErr := range permanentSyscalls {
		assert.False(t, isTemporaryAcceptError(sysErr), "expected %v to not be temporary accept error", sysErr)
	}
}

func TestUnit_ControlSocket_Server_UnconfiguredDispatcher(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "s.sock")

	server := NewServer(socketPath, logging.NewLogger())
	err := server.Start()
	require.NoError(t, err)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	client, err := Connect(ctx, socketPath)
	require.NoError(t, err)
	defer client.Close()

	_, err = client.CreateContainer(ctx, &container.ContainerConfig{Image: "alpine"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "server dispatcher not configured")

	err = client.StartContainer(ctx, "c-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "server dispatcher not configured")

	_, err = client.WaitContainer(ctx, "c-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "server dispatcher not configured")

	err = client.RemoveContainer(ctx, "c-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "server dispatcher not configured")

	err = client.SignalContainer(ctx, "c-1", "SIGKILL")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "server dispatcher not configured")

	err = client.ResizeContainerTTY(ctx, "c-1", 24, 80)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "server dispatcher not configured")

	ready := make(chan struct{})
	err = client.AttachContainer(ctx, "c-1", false, nil, nil, nil, ready)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "server dispatcher not configured")
}

func TestUnit_ControlSocket_Server_MalformedRPCArgs(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "s.sock")

	server := NewServer(socketPath, logging.NewLogger())
	server.SetDispatcher(&mockDispatcherForStreamTest{})
	err := server.Start()
	require.NoError(t, err)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	client, err := Connect(ctx, socketPath)
	require.NoError(t, err)
	defer client.Close()

	malformedPayload := []byte("{invalid-json")

	msgTypes := []MessageType{
		MsgCreateContainer,
		MsgStartContainer,
		MsgWaitContainer,
		MsgRemoveContainer,
		MsgSignalContainer,
		MsgResizeContainerTTY,
		MsgAttachContainer,
	}

	for _, msgType := range msgTypes {
		_, err := client.sendRPC(ctx, msgType, malformedPayload)
		require.Error(t, err, "expected error for malformed payload on %s", msgType)
		assert.Contains(t, err.Error(), "malformed")
	}
}

func TestUnit_ControlSocket_AttachContainer_ErrorAndFallbackPaths(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "s.sock")

	mockDisp := &mockDispatcherForStreamTest{}
	server := NewServer(socketPath, logging.NewLogger())
	server.SetDispatcher(mockDisp)
	err := server.Start()
	require.NoError(t, err)
	defer server.Close()

	t.Run("Dial failure closes ready channel", func(t *testing.T) {
		client := &Client{socketPath: filepath.Join(tmpDir, "none.sock")}
		ready := make(chan struct{})
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		err := client.AttachContainer(ctx, "c-1", false, nil, nil, nil, ready)
		require.Error(t, err)
		select {
		case <-ready:
			// ready channel successfully closed
		default:
			t.Fatal("expected ready channel to be closed on dial failure")
		}
	})

	t.Run("Nil stdout and stderr fallbacks in TTY and Non-TTY modes", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		client, err := Connect(ctx, socketPath)
		require.NoError(t, err)
		defer client.Close()

		// Non-TTY with nil stdout/stderr
		ready1 := make(chan struct{})
		err = client.AttachContainer(ctx, "c-1", false, nil, nil, nil, ready1)
		require.NoError(t, err)

		// TTY with nil stdout/stderr
		ready2 := make(chan struct{})
		err = client.AttachContainer(ctx, "c-1", true, nil, nil, nil, ready2)
		require.NoError(t, err)
	})

	t.Run("Stdin reader error propagation", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		client, err := Connect(ctx, socketPath)
		require.NoError(t, err)
		defer client.Close()

		expectedErr := errors.New("custom stdin read failure")
		badStdin := &customErrReader{err: expectedErr}

		ready := make(chan struct{})
		err = client.AttachContainer(ctx, "c-1", false, badStdin, nil, nil, ready)
		require.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)
	})
}

func TestUnit_ControlSocket_ConnState_LongRunningOps(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	cs := &connState{Conn: c1}

	assert.Equal(t, 0, cs.longRunningOps)

	cs.incLongRunningOps()
	assert.Equal(t, 1, cs.longRunningOps)

	cs.incLongRunningOps()
	assert.Equal(t, 2, cs.longRunningOps)

	cs.decLongRunningOps(50 * time.Millisecond)
	assert.Equal(t, 1, cs.longRunningOps)

	cs.decLongRunningOps(50 * time.Millisecond)
	assert.Equal(t, 0, cs.longRunningOps)

	// Underflow protection check
	cs.decLongRunningOps(50 * time.Millisecond)
	assert.Equal(t, 0, cs.longRunningOps)

	cs.updateReadDeadlineForNextFrame(50 * time.Millisecond)

	// Verify that the set read deadline causes an idle read timeout on the pipe
	buf := make([]byte, 10)
	errChan := make(chan error, 1)
	go func() {
		_, err := c1.Read(buf)
		errChan <- err
	}()

	select {
	case err := <-errChan:
		require.Error(t, err)
		var netErr net.Error
		require.True(t, errors.As(err, &netErr) && netErr.Timeout(), "expected read timeout error, got %v", err)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected read on net.Pipe to time out after updateReadDeadlineForNextFrame")
	}
}

func TestUnit_ControlSocket_CopyErrOrNil(t *testing.T) {
	assert.NoError(t, copyErrOrNil(nil))
	assert.NoError(t, copyErrOrNil(io.EOF))
	assert.NoError(t, copyErrOrNil(net.ErrClosed))

	customErr := errors.New("stream copy error")
	assert.Equal(t, customErr, copyErrOrNil(customErr))
}
