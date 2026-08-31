package runtime

import (
	"context"
	"testing"

	"cderun/internal/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestScenario_Runtime_AdvancedAdapterExpansion verifies MockRuntime lifecycle operations,
// Docker adapter conversion helpers, and containerd runtime configuration validations.
// Reference: docs/features/multi-runtime-support.md
func TestScenario_Runtime_AdvancedAdapterExpansion(t *testing.T) {
	t.Parallel()

	t.Run("MockRuntime_LifecycleOperations", func(t *testing.T) {
		t.Parallel()

		mock := NewMockRuntime()
		mock.CreatedContainerID = "mock-container-123"
		ctx := context.Background()

		cfg := &container.ContainerConfig{
			Image:   "alpine:latest",
			Command: []string{"echo", "hello"},
		}

		containerID, err := mock.CreateContainer(ctx, cfg)
		require.NoError(t, err)
		assert.Equal(t, "mock-container-123", containerID)

		err = mock.StartContainer(ctx, containerID)
		require.NoError(t, err)

		exitCode, err := mock.WaitContainer(ctx, containerID)
		require.NoError(t, err)
		assert.Equal(t, 0, exitCode)

		running, code, err := mock.InspectContainer(ctx, containerID)
		require.NoError(t, err)
		assert.False(t, running)
		assert.Equal(t, 0, code)

		err = mock.SignalContainer(ctx, containerID, "SIGTERM")
		require.NoError(t, err)

		err = mock.ResizeContainerTTY(ctx, containerID, 24, 80)
		require.NoError(t, err)

		err = mock.RemoveContainer(ctx, containerID)
		require.NoError(t, err)
	})

	t.Run("DockerAdapter_BuildMountsOptionalMountRejection", func(t *testing.T) {
		t.Parallel()

		cfg := &container.ContainerConfig{
			Image: "alpine:latest",
			Mounts: []container.Mount{
				{
					Type:     "bind",
					Source:   "/host/path",
					Target:   "/container/path",
					Optional: true,
				},
			},
		}

		_, err := buildDockerMounts(cfg.Mounts)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "optional mount is not supported")
	})

	t.Run("Containerd_ValidateConfigUnsupportedFeatures", func(t *testing.T) {
		t.Parallel()

		rt := &ContainerdRuntime{}

		t.Run("RejectPorts", func(t *testing.T) {
			cfg := &container.ContainerConfig{
				Image: "alpine:latest",
				Ports: []string{"8080:80"},
			}
			err := rt.ValidateConfig(cfg)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "port mapping is not supported")
		})

		t.Run("RejectVolumeMount", func(t *testing.T) {
			cfg := &container.ContainerConfig{
				Image: "alpine:latest",
				Mounts: []container.Mount{
					{Type: "volume", Source: "myvol", Target: "/data"},
				},
			}
			err := rt.ValidateConfig(cfg)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "volume mount type is not supported")
		})

		t.Run("RejectRestartPolicy", func(t *testing.T) {
			cfg := &container.ContainerConfig{
				Image:   "alpine:latest",
				Restart: "always",
			}
			err := rt.ValidateConfig(cfg)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "restart policy is not supported")
		})
	})
}
