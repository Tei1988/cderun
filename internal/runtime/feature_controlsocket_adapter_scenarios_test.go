package runtime_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"cderun/internal/container"
	"cderun/internal/logging"
	"cderun/internal/runtime"
	"cderun/internal/runtime/controlsocket"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type dummyCloseErrRuntime struct {
	*runtime.MockRuntime
	closeErr error
}

func (d *dummyCloseErrRuntime) Close() error {
	return d.closeErr
}

type fullDispatcher struct {
	mu           sync.Mutex
	created      bool
	started      bool
	waited       bool
	removed      bool
	signaled     string
	ttyRows      uint
	ttyCols      uint
	attachCalled bool
}

func (m *fullDispatcher) CreateContainer(ctx context.Context, config *container.ContainerConfig) (string, error) {
	m.mu.Lock()
	m.created = true
	m.mu.Unlock()
	return "c-full-123", nil
}

func (m *fullDispatcher) StartContainer(ctx context.Context, containerID string) error {
	m.mu.Lock()
	m.started = true
	m.mu.Unlock()
	return nil
}

func (m *fullDispatcher) WaitContainer(ctx context.Context, containerID string) (int, error) {
	m.mu.Lock()
	m.waited = true
	m.mu.Unlock()
	return 0, nil
}

func (m *fullDispatcher) RemoveContainer(ctx context.Context, containerID string) error {
	m.mu.Lock()
	m.removed = true
	m.mu.Unlock()
	return nil
}

func (m *fullDispatcher) SignalContainer(ctx context.Context, containerID string, sig string) error {
	m.mu.Lock()
	m.signaled = sig
	m.mu.Unlock()
	return nil
}

func (m *fullDispatcher) ResizeContainerTTY(ctx context.Context, containerID string, rows, cols uint) error {
	m.mu.Lock()
	m.ttyRows = rows
	m.ttyCols = cols
	m.mu.Unlock()
	return nil
}

func (m *fullDispatcher) AttachContainer(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error {
	m.mu.Lock()
	m.attachCalled = true
	m.mu.Unlock()

	if ready != nil {
		close(ready)
	}
	if stdout != nil {
		_, _ = stdout.Write([]byte("ATTACH_OK"))
	}
	return nil
}

func TestUnit_ControlSocketRuntimeAdapter_DelegationAndLifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "adapter_full.sock")

	disp := &fullDispatcher{}
	server := controlsocket.NewServer(socketPath, logging.NewLogger())
	server.SetDispatcher(disp)
	require.NoError(t, server.Start())
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client, err := controlsocket.Connect(ctx, socketPath)
	require.NoError(t, err)

	mockRuntime := runtime.NewMockRuntime()
	mockRuntime.InspectFunc = func(ctx context.Context, containerID string) (bool, int, error) {
		if containerID == "c123" {
			return true, 0, nil
		}
		return false, 1, nil
	}

	// Create adapter with nil logger to verify logging.GetGlobalLogger fallback
	adapter := runtime.NewControlSocketRuntimeAdapter(mockRuntime, client, nil)
	require.NotNil(t, adapter)

	// Test Name
	assert.Equal(t, "mock-controlsocket", adapter.Name())

	// Test PullImage delegation
	err = adapter.PullImage(ctx, "alpine:latest", "always", 1, 10*time.Millisecond)
	assert.NoError(t, err)

	// Test ValidateConfig delegation
	validCfg := &container.ContainerConfig{Image: "alpine:latest"}
	assert.NoError(t, adapter.ValidateConfig(validCfg))

	// Test InspectContainer delegation
	exists, exitCode, err := adapter.InspectContainer(ctx, "c123")
	assert.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, 0, exitCode)

	// Test PruneContainers delegation
	pruned, err := adapter.PruneContainers(ctx)
	assert.NoError(t, err)
	assert.Nil(t, pruned)

	// Test CreateContainer over socket
	cid, err := adapter.CreateContainer(ctx, validCfg)
	require.NoError(t, err)
	assert.Equal(t, "c-full-123", cid)

	// Test StartContainer over socket
	err = adapter.StartContainer(ctx, cid)
	assert.NoError(t, err)

	// Test SignalContainer over socket
	err = adapter.SignalContainer(ctx, cid, "SIGTERM")
	assert.NoError(t, err)

	// Test ResizeContainerTTY over socket
	err = adapter.ResizeContainerTTY(ctx, cid, 24, 80)
	assert.NoError(t, err)

	// Test AttachContainer over socket
	var stdout bytes.Buffer
	ready := make(chan struct{})
	err = adapter.AttachContainer(ctx, cid, false, nil, &stdout, nil, ready)
	assert.NoError(t, err)
	assert.Equal(t, "ATTACH_OK", stdout.String())

	// Test WaitContainer over socket
	code, err := adapter.WaitContainer(ctx, cid)
	assert.NoError(t, err)
	assert.Equal(t, 0, code)

	// Test RemoveContainer over socket
	err = adapter.RemoveContainer(ctx, cid)
	assert.NoError(t, err)

	// Verify dispatcher received calls
	disp.mu.Lock()
	assert.True(t, disp.created)
	assert.True(t, disp.started)
	assert.True(t, disp.waited)
	assert.True(t, disp.removed)
	assert.Equal(t, "SIGTERM", disp.signaled)
	assert.Equal(t, uint(24), disp.ttyRows)
	assert.Equal(t, uint(80), disp.ttyCols)
	assert.True(t, disp.attachCalled)
	disp.mu.Unlock()

	// Test Close
	assert.NoError(t, adapter.Close())
}

func TestUnit_ControlSocketRuntimeAdapter_CloseErrorUnwrapping(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "adapter_close_err.sock")

	server := controlsocket.NewServer(socketPath, logging.NewLogger())
	require.NoError(t, server.Start())
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client, err := controlsocket.Connect(ctx, socketPath)
	require.NoError(t, err)

	underlyingErr := errors.New("underlying runtime close error")
	dummyRt := &dummyCloseErrRuntime{
		MockRuntime: runtime.NewMockRuntime(),
		closeErr:    underlyingErr,
	}

	adapter := runtime.NewControlSocketRuntimeAdapter(dummyRt, client, logging.NewLogger())
	err = adapter.Close()
	require.Error(t, err)
	assert.True(t, errors.Is(err, underlyingErr))
}
