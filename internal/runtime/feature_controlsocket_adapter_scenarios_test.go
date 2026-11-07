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

func TestUnit_ControlSocketRuntimeAdapter_NilGuardsAndReadyClosure(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("nil adapter receiver methods return safe errors and defaults", func(t *testing.T) {
		var nilAdapter *runtime.ControlSocketRuntimeAdapter

		require.NoError(t, nilAdapter.Close())
		assert.Equal(t, "controlsocket", nilAdapter.Name())

		require.Error(t, nilAdapter.PullImage(ctx, "alpine", "always", 1, time.Second))
		require.Error(t, nilAdapter.ValidateConfig(&container.ContainerConfig{Image: "alpine"}))

		exists, code, err := nilAdapter.InspectContainer(ctx, "c123")
		assert.False(t, exists)
		assert.Equal(t, -1, code)
		require.Error(t, err)

		pruned, err := nilAdapter.PruneContainers(ctx)
		assert.Nil(t, pruned)
		require.Error(t, err)

		cid, err := nilAdapter.CreateContainer(ctx, &container.ContainerConfig{Image: "alpine"})
		assert.Empty(t, cid)
		require.Error(t, err)

		require.Error(t, nilAdapter.StartContainer(ctx, "c123"))

		exitCode, err := nilAdapter.WaitContainer(ctx, "c123")
		assert.Equal(t, -1, exitCode)
		require.Error(t, err)

		require.Error(t, nilAdapter.RemoveContainer(ctx, "c123"))
		require.Error(t, nilAdapter.SignalContainer(ctx, "c123", "SIGTERM"))
		require.Error(t, nilAdapter.ResizeContainerTTY(ctx, "c123", 24, 80))

		ready := make(chan struct{})
		err = nilAdapter.AttachContainer(ctx, "c123", false, nil, nil, nil, ready)
		require.Error(t, err)
		select {
		case <-ready:
			// OK, closed
		default:
			t.Fatal("expected ready channel to be closed on nil adapter AttachContainer")
		}
	})

	t.Run("adapter with nil underlying runtime returns safe errors and defaults", func(t *testing.T) {
		adapter := runtime.NewControlSocketRuntimeAdapter(nil, nil, nil)

		require.NoError(t, adapter.Close())
		assert.Equal(t, "controlsocket", adapter.Name())

		require.Error(t, adapter.PullImage(ctx, "alpine", "always", 1, time.Second))
		require.Error(t, adapter.ValidateConfig(&container.ContainerConfig{Image: "alpine"}))

		exists, code, err := adapter.InspectContainer(ctx, "c123")
		assert.False(t, exists)
		assert.Equal(t, -1, code)
		require.Error(t, err)

		pruned, err := adapter.PruneContainers(ctx)
		assert.Nil(t, pruned)
		require.Error(t, err)
	})

	t.Run("adapter with nil control socket client returns safe errors and closes ready", func(t *testing.T) {
		mockRuntime := runtime.NewMockRuntime()
		adapter := runtime.NewControlSocketRuntimeAdapter(mockRuntime, nil, nil)

		require.NoError(t, adapter.Close())
		assert.Equal(t, "mock-controlsocket", adapter.Name())

		cid, err := adapter.CreateContainer(ctx, &container.ContainerConfig{Image: "alpine"})
		assert.Empty(t, cid)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "client is not configured")

		require.Error(t, adapter.StartContainer(ctx, "c123"))

		code, err := adapter.WaitContainer(ctx, "c123")
		assert.Equal(t, -1, code)
		require.Error(t, err)

		require.Error(t, adapter.RemoveContainer(ctx, "c123"))
		require.Error(t, adapter.SignalContainer(ctx, "c123", "SIGTERM"))
		require.Error(t, adapter.ResizeContainerTTY(ctx, "c123", 24, 80))

		ready := make(chan struct{})
		err = adapter.AttachContainer(ctx, "c123", false, nil, nil, nil, ready)
		require.Error(t, err)
		select {
		case <-ready:
			// OK, closed
		default:
			t.Fatal("expected ready channel to be closed on nil client AttachContainer")
		}
	})
}
