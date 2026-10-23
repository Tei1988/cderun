package runtime_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/container"
	"cderun/internal/logging"
	"cderun/internal/runtime"
	"cderun/internal/runtime/controlsocket"
)

func TestUnit_Containerd_ControlSocket_LifecycleAndErrorPropagation(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "cs.sock")

	logger := logging.NewLogger()
	ctrlServer := controlsocket.NewServer(socketPath, logger)

	mockRt := runtime.NewMockRuntime()
	mockRt.CreatedContainerID = "containerd-cs-001"
	mockRt.ExitCode = 0

	ctrlServer.SetDispatcher(mockRt)
	require.NoError(t, ctrlServer.Start())
	defer ctrlServer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	client, err := controlsocket.Connect(ctx, socketPath)
	require.NoError(t, err)
	defer client.Close()

	adapter := runtime.NewControlSocketRuntimeAdapter(mockRt, client, logger)

	assert.Equal(t, "mock-controlsocket", adapter.Name())

	cfg := &container.ContainerConfig{
		Image:   "docker.io/library/alpine:latest",
		Command: []string{"echo", "hello"},
	}

	// 1. ValidateConfig
	require.NoError(t, adapter.ValidateConfig(cfg))

	// 2. CreateContainer
	id, err := adapter.CreateContainer(ctx, cfg)
	require.NoError(t, err)
	assert.Equal(t, "containerd-cs-001", id)

	// 3. AttachContainer
	readyChan := make(chan struct{})
	attachErrChan := make(chan error, 1)
	go func() {
		attachErrChan <- adapter.AttachContainer(ctx, id, false, nil, nil, nil, readyChan)
	}()
	select {
	case <-readyChan:
	case err := <-attachErrChan:
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal("timed out waiting for AttachContainer ready signal")
	}

	// 4. StartContainer
	require.NoError(t, adapter.StartContainer(ctx, id))

	// 5. WaitContainer
	exitCode, err := adapter.WaitContainer(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, 0, exitCode)

	// Wait for background AttachContainer to finish cleanly
	select {
	case err := <-attachErrChan:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for AttachContainer to finish")
	}

	// 6. SignalContainer
	require.NoError(t, adapter.SignalContainer(ctx, id, "SIGTERM"))

	// 7. ResizeContainerTTY
	require.NoError(t, adapter.ResizeContainerTTY(ctx, id, 24, 80))

	// 8. RemoveContainer
	require.NoError(t, adapter.RemoveContainer(ctx, id))
}

func TestUnit_API_Engines_ControlSocket_InheritedCeilingPolicy(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "policy.sock")

	logger := logging.NewLogger()
	ctrlServer := controlsocket.NewServer(socketPath, logger)

	mockRt := runtime.NewMockRuntime()
	mockRt.CreatedContainerID = "api-child-123"

	// Parent configuration with restricted privileges
	parentCfg := &container.ContainerConfig{
		Image:      "alpine:latest",
		Privileged: false,
		CapDrop:    []string{"CAP_SYS_ADMIN"},
		Mounts: []container.Mount{
			{
				Source:   "/host/data",
				Target:   "/data",
				ReadOnly: true,
			},
		},
	}

	ctrlServer.SetDispatcher(mockRt)
	ctrlServer.SetParentConfig(parentCfg)
	require.NoError(t, ctrlServer.Start())
	defer ctrlServer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	client, err := controlsocket.Connect(ctx, socketPath)
	require.NoError(t, err)
	defer client.Close()

	adapter := runtime.NewControlSocketRuntimeAdapter(mockRt, client, logger)

	t.Run("child requesting privileged mode is rejected when parent is unprivileged", func(t *testing.T) {
		childCfg := &container.ContainerConfig{
			Image:      "alpine:latest",
			Privileged: true,
			CapDrop:    []string{"CAP_SYS_ADMIN"},
		}
		_, err := adapter.CreateContainer(ctx, childCfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "security violation: child requested privileged mode")
	})

	t.Run("child requesting dropped capability is rejected", func(t *testing.T) {
		childCfg := &container.ContainerConfig{
			Image:   "alpine:latest",
			CapAdd:  []string{"CAP_SYS_ADMIN"},
			CapDrop: []string{"CAP_SYS_ADMIN"},
		}
		_, err := adapter.CreateContainer(ctx, childCfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "security violation: child requested capability")
		assert.Contains(t, err.Error(), "dropped by parent")
	})

	t.Run("child requesting read-write mount outside parent read-only boundary is rejected", func(t *testing.T) {
		childCfg := &container.ContainerConfig{
			Image:   "alpine:latest",
			CapDrop: []string{"CAP_SYS_ADMIN"},
			Mounts: []container.Mount{
				{
					Source:   "/host/data",
					Target:   "/data",
					ReadOnly: false,
				},
			},
		}
		_, err := adapter.CreateContainer(ctx, childCfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "security violation")
	})

	t.Run("compliant child request is accepted and dispatched", func(t *testing.T) {
		childCfg := &container.ContainerConfig{
			Image:   "alpine:latest",
			CapDrop: []string{"CAP_SYS_ADMIN"},
			Mounts: []container.Mount{
				{
					Source:   "/host/data",
					Target:   "/data",
					ReadOnly: true,
				},
			},
		}
		id, err := adapter.CreateContainer(ctx, childCfg)
		require.NoError(t, err)
		assert.Equal(t, "api-child-123", id)
	})
}
