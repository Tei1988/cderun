package controlsocket

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/container"
	"cderun/internal/logging"
)

type mockDispatcher struct {
	createFunc func(ctx context.Context, config *container.ContainerConfig) (string, error)
	startFunc  func(ctx context.Context, containerID string) error
	waitFunc   func(ctx context.Context, containerID string) (int, error)
	removeFunc func(ctx context.Context, containerID string) error
	attachFunc func(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error
	resizeFunc func(ctx context.Context, containerID string, rows, cols uint) error
	signalFunc func(ctx context.Context, containerID string, sig string) error
}

func (m *mockDispatcher) CreateContainer(ctx context.Context, config *container.ContainerConfig) (string, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, config)
	}
	return "mock-container-id", nil
}

func (m *mockDispatcher) StartContainer(ctx context.Context, containerID string) error {
	if m.startFunc != nil {
		return m.startFunc(ctx, containerID)
	}
	return nil
}

func (m *mockDispatcher) WaitContainer(ctx context.Context, containerID string) (int, error) {
	if m.waitFunc != nil {
		return m.waitFunc(ctx, containerID)
	}
	return 0, nil
}

func (m *mockDispatcher) RemoveContainer(ctx context.Context, containerID string) error {
	if m.removeFunc != nil {
		return m.removeFunc(ctx, containerID)
	}
	return nil
}

func (m *mockDispatcher) AttachContainer(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error {
	if m.attachFunc != nil {
		return m.attachFunc(ctx, containerID, tty, stdin, stdout, stderr, ready)
	}
	if ready != nil {
		select {
		case ready <- struct{}{}:
		default:
		}
	}
	return nil
}

func (m *mockDispatcher) ResizeContainerTTY(ctx context.Context, containerID string, rows, cols uint) error {
	if m.resizeFunc != nil {
		return m.resizeFunc(ctx, containerID, rows, cols)
	}
	return nil
}

func (m *mockDispatcher) SignalContainer(ctx context.Context, containerID string, sig string) error {
	if m.signalFunc != nil {
		return m.signalFunc(ctx, containerID, sig)
	}
	return nil
}

func TestUnit_ControlSocket_RPC_Lifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "rpc_test.sock")

	disp := &mockDispatcher{
		createFunc: func(ctx context.Context, config *container.ContainerConfig) (string, error) {
			if config.Image == "invalid" {
				return "", errors.New("invalid image")
			}
			return "container-abc-123", nil
		},
		waitFunc: func(ctx context.Context, containerID string) (int, error) {
			if containerID == "container-abc-123" {
				return 42, nil
			}
			return 1, fmt.Errorf("unknown container %s", containerID)
		},
	}

	server := NewServer(socketPath, logging.NewLogger())
	server.SetDispatcher(disp)
	require.NoError(t, server.Start())
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	client, err := Connect(ctx, socketPath)
	require.NoError(t, err)
	defer client.Close()

	// 1. CreateContainer success
	cfg := &container.ContainerConfig{Image: "alpine:latest"}
	cID, err := client.CreateContainer(ctx, cfg)
	require.NoError(t, err)
	assert.Equal(t, "container-abc-123", cID)

	// 2. CreateContainer error
	_, err = client.CreateContainer(ctx, &container.ContainerConfig{Image: "invalid"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid image")

	// 3. StartContainer success
	err = client.StartContainer(ctx, cID)
	require.NoError(t, err)

	// 4. WaitContainer success
	exitCode, err := client.WaitContainer(ctx, cID)
	require.NoError(t, err)
	assert.Equal(t, 42, exitCode)

	// 5. RemoveContainer success
	err = client.RemoveContainer(ctx, cID)
	require.NoError(t, err)
}

func TestUnit_ControlSocket_RPC_NoDispatcherError(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "nodisp.sock")

	server := NewServer(socketPath, logging.NewLogger())
	require.NoError(t, server.Start())
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	client, err := Connect(ctx, socketPath)
	require.NoError(t, err)
	defer client.Close()

	_, err = client.CreateContainer(ctx, &container.ContainerConfig{Image: "alpine"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "server dispatcher not configured")
}

func TestUnit_ControlSocket_RPC_NilConfigRejection(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "nilcfg.sock")

	disp := &mockDispatcher{}
	server := NewServer(socketPath, logging.NewLogger())
	server.SetDispatcher(disp)
	require.NoError(t, server.Start())
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	client, err := Connect(ctx, socketPath)
	require.NoError(t, err)
	defer client.Close()

	_, err = client.CreateContainer(ctx, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "CreateContainer args.Config is nil")
}

func TestUnit_ControlSocket_RPC_ContextDeadlinePropagation(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "deadline.sock")

	var mu sync.Mutex
	var receivedDeadline time.Time
	disp := &mockDispatcher{
		createFunc: func(ctx context.Context, config *container.ContainerConfig) (string, error) {
			if dl, ok := ctx.Deadline(); ok {
				mu.Lock()
				receivedDeadline = dl
				mu.Unlock()
			}
			return "container-dl-123", nil
		},
	}

	server := NewServer(socketPath, logging.NewLogger())
	server.SetDispatcher(disp)
	require.NoError(t, server.Start())
	defer server.Close()

	expectedDeadline := time.Now().Add(5 * time.Second)
	ctx, cancel := context.WithDeadline(context.Background(), expectedDeadline)
	defer cancel()

	client, err := Connect(ctx, socketPath)
	require.NoError(t, err)
	defer client.Close()

	cID, err := client.CreateContainer(ctx, &container.ContainerConfig{Image: "alpine"})
	require.NoError(t, err)
	assert.Equal(t, "container-dl-123", cID)

	mu.Lock()
	dl := receivedDeadline
	mu.Unlock()

	assert.False(t, dl.IsZero())
	assert.WithinDuration(t, expectedDeadline, dl, 50*time.Millisecond)
}

func TestUnit_ControlSocket_RPC_ConcurrentRPCs(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "concurrent.sock")

	disp := &mockDispatcher{
		waitFunc: func(ctx context.Context, containerID string) (int, error) {
			if containerID == "c-1" {
				return 10, nil
			}
			if containerID == "c-2" {
				return 20, nil
			}
			return 0, nil
		},
	}

	server := NewServer(socketPath, logging.NewLogger())
	server.SetDispatcher(disp)
	require.NoError(t, server.Start())
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client, err := Connect(ctx, socketPath)
	require.NoError(t, err)
	defer client.Close()

	var wg sync.WaitGroup
	errs := make(chan error, 10)

	for i := range 10 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			cID := fmt.Sprintf("c-%d", (idx%2)+1)
			expectedCode := 10
			if cID == "c-2" {
				expectedCode = 20
			}

			code, err := client.WaitContainer(ctx, cID)
			if err != nil {
				errs <- err
				return
			}
			if code != expectedCode {
				errs <- fmt.Errorf("expected exit code %d for %s, got %d", expectedCode, cID, code)
				return
			}

			if err := client.Ping(ctx); err != nil {
				errs <- fmt.Errorf("ping failed: %w", err)
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		assert.NoError(t, err)
	}
}

func TestUnit_ControlSocket_RPC_SignalAndResize(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "signal_resize.sock")

	var signalCalled, resizeCalled bool
	var receivedSig string
	var receivedRows, receivedCols uint

	disp := &mockDispatcher{
		signalFunc: func(ctx context.Context, containerID string, sig string) error {
			signalCalled = true
			receivedSig = sig
			return nil
		},
		resizeFunc: func(ctx context.Context, containerID string, rows, cols uint) error {
			resizeCalled = true
			receivedRows = rows
			receivedCols = cols
			return nil
		},
	}

	server := NewServer(socketPath, logging.NewLogger())
	server.SetDispatcher(disp)
	require.NoError(t, server.Start())
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	client, err := Connect(ctx, socketPath)
	require.NoError(t, err)
	defer client.Close()

	// Signal
	require.NoError(t, client.SignalContainer(ctx, "c1", "SIGINT"))
	assert.True(t, signalCalled)
	assert.Equal(t, "SIGINT", receivedSig)

	// Resize
	require.NoError(t, client.ResizeContainerTTY(ctx, "c1", 24, 80))
	assert.True(t, resizeCalled)
	assert.Equal(t, uint(24), receivedRows)
	assert.Equal(t, uint(80), receivedCols)
}

func TestUnit_ControlSocket_RPC_AttachContainer_NonTTY(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "attach_nontty.sock")

	disp := &mockDispatcher{
		attachFunc: func(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error {
			if ready != nil {
				select {
				case ready <- struct{}{}:
				default:
				}
			}
			// Write stdout and stderr
			_, _ = stdout.Write([]byte("hello stdout"))
			_, _ = stderr.Write([]byte("hello stderr"))

			// Read stdin
			stdinBuf := new(bytes.Buffer)
			_, _ = io.Copy(stdinBuf, stdin)
			if stdinBuf.String() != "hello stdin" {
				return fmt.Errorf("unexpected stdin input: %s", stdinBuf.String())
			}
			return nil
		},
	}

	server := NewServer(socketPath, logging.NewLogger())
	server.SetDispatcher(disp)
	require.NoError(t, server.Start())
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client, err := Connect(ctx, socketPath)
	require.NoError(t, err)
	defer client.Close()

	stdin := bytes.NewBufferString("hello stdin")
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	ready := make(chan struct{}, 1)

	err = client.AttachContainer(ctx, "c1", false, stdin, stdout, stderr, ready)
	require.NoError(t, err)

	assert.Equal(t, "hello stdout", stdout.String())
	assert.Equal(t, "hello stderr", stderr.String())

	select {
	case <-ready:
	default:
		t.Fatal("ready channel was not signaled")
	}
}

func TestUnit_ControlSocket_RPC_AttachContainer_TTY(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "attach_tty.sock")

	disp := &mockDispatcher{
		attachFunc: func(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error {
			if ready != nil {
				select {
				case ready <- struct{}{}:
				default:
				}
			}
			_, _ = stdout.Write([]byte("tty output"))
			return nil
		},
	}

	server := NewServer(socketPath, logging.NewLogger())
	server.SetDispatcher(disp)
	require.NoError(t, server.Start())
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client, err := Connect(ctx, socketPath)
	require.NoError(t, err)
	defer client.Close()

	stdout := new(bytes.Buffer)
	ready := make(chan struct{}, 1)

	err = client.AttachContainer(ctx, "c1", true, nil, stdout, nil, ready)
	require.NoError(t, err)

	assert.Equal(t, "tty output", stdout.String())
}
