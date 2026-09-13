package controlsocket

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/container"
	"cderun/internal/logging"
)

type tempNetError struct{}

func (e tempNetError) Error() string   { return "temporary net error" }
func (e tempNetError) Timeout() bool   { return true }
func (e tempNetError) Temporary() bool { return true }

type mockListener struct {
	acceptFunc func() (net.Conn, error)
	closeFunc  func() error
	addrFunc   func() net.Addr
}

func (m *mockListener) Accept() (net.Conn, error) {
	if m.acceptFunc != nil {
		return m.acceptFunc()
	}
	return nil, errors.New("not implemented")
}

func (m *mockListener) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

func (m *mockListener) Addr() net.Addr {
	if m.addrFunc != nil {
		return m.addrFunc()
	}
	return &net.UnixAddr{Name: "/dummy/socket.sock", Net: "unix"}
}

type mockDispatcherForRobustness struct {
	waitDelay  time.Duration
	waitExit   int
	waitErr    error
	attachDelay time.Duration
}

func (m *mockDispatcherForRobustness) CreateContainer(ctx context.Context, config *container.ContainerConfig) (string, error) {
	return "test-cid", nil
}

func (m *mockDispatcherForRobustness) StartContainer(ctx context.Context, containerID string) error {
	return nil
}

func (m *mockDispatcherForRobustness) WaitContainer(ctx context.Context, containerID string) (int, error) {
	if m.waitDelay > 0 {
		select {
		case <-time.After(m.waitDelay):
		case <-ctx.Done():
			return 0, ctx.Err()
		}
	}
	return m.waitExit, m.waitErr
}

func (m *mockDispatcherForRobustness) RemoveContainer(ctx context.Context, containerID string) error {
	return nil
}

func (m *mockDispatcherForRobustness) AttachContainer(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error {
	if ready != nil {
		close(ready)
	}
	if m.attachDelay > 0 {
		select {
		case <-time.After(m.attachDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if stdout != nil {
		_, _ = stdout.Write([]byte("attach-output\n"))
	}
	return nil
}

func (m *mockDispatcherForRobustness) SignalContainer(ctx context.Context, containerID string, sig string) error {
	return nil
}

func (m *mockDispatcherForRobustness) ResizeContainerTTY(ctx context.Context, containerID string, rows, cols uint) error {
	return nil
}

func TestUnit_ControlSocket_AcceptLoop_TemporaryErrorRetry(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test_temp_accept.sock")

	server := NewServer(socketPath, logging.GetGlobalLogger())

	var mu sync.Mutex
	failCount := 0
	realListener, err := net.Listen("unix", socketPath)
	require.NoError(t, err)

	mListener := &mockListener{
		acceptFunc: func() (net.Conn, error) {
			mu.Lock()
			fc := failCount
			failCount++
			mu.Unlock()

			if fc < 2 {
				return nil, tempNetError{}
			}
			return realListener.Accept()
		},
		closeFunc: func() error {
			return realListener.Close()
		},
		addrFunc: func() net.Addr {
			return realListener.Addr()
		},
	}

	server.listener = mListener
	server.wg.Add(1)
	go server.acceptLoop()
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client, err := Connect(ctx, socketPath)
	require.NoError(t, err)
	defer client.Close()

	err = client.Ping(ctx)
	require.NoError(t, err)

	mu.Lock()
	assert.GreaterOrEqual(t, failCount, 2)
	mu.Unlock()
}

func TestUnit_ControlSocket_AcceptLoop_PermanentErrorExit(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test_perm_accept.sock")

	server := NewServer(socketPath, logging.GetGlobalLogger())

	doneAccept := make(chan struct{})
	mListener := &mockListener{
		acceptFunc: func() (net.Conn, error) {
			close(doneAccept)
			return nil, errors.New("permanent non-net error")
		},
		closeFunc: func() error {
			return nil
		},
	}

	server.listener = mListener
	server.wg.Add(1)
	go server.acceptLoop()

	select {
	case <-doneAccept:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for acceptLoop to encounter permanent error")
	}

	// Wait for acceptLoop goroutine to return
	wgDone := make(chan struct{})
	go func() {
		server.wg.Wait()
		close(wgDone)
	}()

	select {
	case <-wgDone:
	case <-time.After(2 * time.Second):
		t.Fatal("acceptLoop goroutine did not exit on permanent error")
	}
}

func TestUnit_ControlSocket_IdleTimeout_ReclaimsConnection(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test_idle.sock")

	server := NewServer(socketPath, logging.GetGlobalLogger())
	server.SetIdleTimeout(50 * time.Millisecond)
	err := server.Start()
	require.NoError(t, err)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client, err := Connect(ctx, socketPath)
	require.NoError(t, err)
	defer client.Close()

	err = client.Ping(ctx)
	require.NoError(t, err)

	// Wait for idle timeout to trigger on server side
	time.Sleep(150 * time.Millisecond)

	// Subsequent ping should fail because server closed the connection on idle timeout
	err = client.Ping(ctx)
	require.Error(t, err)
}

func TestUnit_ControlSocket_IdleTimeout_ExemptsWaitAndAttachContainer(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test_idle_exempt.sock")

	dispatcher := &mockDispatcherForRobustness{
		waitDelay:   150 * time.Millisecond,
		waitExit:    42,
		attachDelay: 150 * time.Millisecond,
	}

	server := NewServer(socketPath, logging.GetGlobalLogger())
	server.SetIdleTimeout(50 * time.Millisecond)
	server.SetDispatcher(dispatcher)
	err := server.Start()
	require.NoError(t, err)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client, err := Connect(ctx, socketPath)
	require.NoError(t, err)
	defer client.Close()

	// WaitContainer takes 150ms which exceeds the 50ms idleTimeout.
	// It should complete successfully without being killed by idle timeout.
	exitCode, err := client.WaitContainer(ctx, "cid-1")
	require.NoError(t, err)
	assert.Equal(t, 42, exitCode)

	// AttachContainer takes 150ms which exceeds the 50ms idleTimeout.
	var stdoutBuf bytes.Buffer
	err = client.AttachContainer(ctx, "cid-2", false, nil, &stdoutBuf, nil, nil)
	require.NoError(t, err)
	assert.Contains(t, stdoutBuf.String(), "attach-output")
}

func TestUnit_ControlSocket_IdleTimeout_RestoredAfterWaitContainerCompletes(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test_idle_restore.sock")

	dispatcher := &mockDispatcherForRobustness{
		waitDelay: 20 * time.Millisecond,
		waitExit:  0,
	}

	server := NewServer(socketPath, logging.GetGlobalLogger())
	server.SetIdleTimeout(60 * time.Millisecond)
	server.SetDispatcher(dispatcher)
	err := server.Start()
	require.NoError(t, err)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client, err := Connect(ctx, socketPath)
	require.NoError(t, err)
	defer client.Close()

	// Perform WaitContainer
	exitCode, err := client.WaitContainer(ctx, "cid-1")
	require.NoError(t, err)
	assert.Equal(t, 0, exitCode)

	// Now stay idle for 150ms (> 60ms idleTimeout)
	time.Sleep(150 * time.Millisecond)

	// Subsequent ping should fail because normal idle timeout was restored and connection closed
	err = client.Ping(ctx)
	require.Error(t, err)
}
