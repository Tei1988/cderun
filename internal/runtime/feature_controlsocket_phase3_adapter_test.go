package runtime_test

import (
	"bytes"
	"context"
	"io"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/container"
	"cderun/internal/logging"
	"cderun/internal/runtime"
	"cderun/internal/runtime/controlsocket"
)

type phase3Dispatcher struct {
	mu           sync.Mutex
	lastSignal   string
	lastTTYRows  uint
	lastTTYCols  uint
	attachCalled bool
}

func (m *phase3Dispatcher) CreateContainer(ctx context.Context, config *container.ContainerConfig) (string, error) {
	return "c-phase3-123", nil
}

func (m *phase3Dispatcher) StartContainer(ctx context.Context, containerID string) error {
	return nil
}

func (m *phase3Dispatcher) WaitContainer(ctx context.Context, containerID string) (int, error) {
	return 0, nil
}

func (m *phase3Dispatcher) RemoveContainer(ctx context.Context, containerID string) error {
	return nil
}

func (m *phase3Dispatcher) SignalContainer(ctx context.Context, containerID string, sig string) error {
	m.mu.Lock()
	m.lastSignal = sig
	m.mu.Unlock()
	return nil
}

func (m *phase3Dispatcher) ResizeContainerTTY(ctx context.Context, containerID string, rows, cols uint) error {
	m.mu.Lock()
	m.lastTTYRows = rows
	m.lastTTYCols = cols
	m.mu.Unlock()
	return nil
}

func (m *phase3Dispatcher) AttachContainer(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error {
	m.mu.Lock()
	m.attachCalled = true
	m.mu.Unlock()

	if ready != nil {
		close(ready)
	}

	if tty {
		if stdout != nil {
			_, _ = stdout.Write([]byte("TTY_OUTPUT"))
		}
	}
	return nil
}

func TestUnit_ControlSocket_Phase3_AdapterDispatch(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "phase3_adapter.sock")

	disp := &phase3Dispatcher{}
	server := controlsocket.NewServer(socketPath, logging.NewLogger())
	server.SetDispatcher(disp)
	require.NoError(t, server.Start())
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client, err := controlsocket.Connect(ctx, socketPath)
	require.NoError(t, err)
	defer client.Close()

	mockRuntime := runtime.NewMockRuntime()
	adapter := runtime.NewControlSocketRuntimeAdapter(mockRuntime, client, logging.NewLogger())
	defer adapter.Close()

	// 1. SignalContainer via adapter
	err = adapter.SignalContainer(ctx, "c-phase3-123", "SIGINT")
	require.NoError(t, err)
	disp.mu.Lock()
	assert.Equal(t, "SIGINT", disp.lastSignal)
	disp.mu.Unlock()

	// 2. ResizeContainerTTY via adapter
	err = adapter.ResizeContainerTTY(ctx, "c-phase3-123", 40, 120)
	require.NoError(t, err)
	disp.mu.Lock()
	assert.Equal(t, uint(40), disp.lastTTYRows)
	assert.Equal(t, uint(120), disp.lastTTYCols)
	disp.mu.Unlock()

	// 3. AttachContainer via adapter
	var stdout bytes.Buffer
	ready := make(chan struct{})
	err = adapter.AttachContainer(ctx, "c-phase3-123", true, nil, &stdout, nil, ready)
	require.NoError(t, err)
	select {
	case <-ready:
	default:
		t.Fatal("ready channel should be closed")
	}
	assert.Equal(t, "TTY_OUTPUT", stdout.String())
}
