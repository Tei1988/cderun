package runtime

import (
	"context"
	"testing"

	"cderun/internal/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_RuntimeAdapters_ComprehensiveEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("MockRuntimeLifecycleOperations", func(t *testing.T) {
		t.Parallel()

		mock := NewMockRuntime()
		ctx := context.Background()

		cfg := &container.ContainerConfig{
			Image:   "alpine:latest",
			Command: []string{"echo", "hello"},
		}

		id, err := mock.CreateContainer(ctx, cfg)
		require.NoError(t, err)

		// Start container
		err = mock.StartContainer(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, id, mock.StartedContainerID)

		// Wait container
		exitCode, err := mock.WaitContainer(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, 0, exitCode)
		assert.Equal(t, id, mock.WaitedContainerID)

		// Remove container
		err = mock.RemoveContainer(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, id, mock.RemovedContainerID)
	})

	t.Run("DockerAdapterParseGPUs", func(t *testing.T) {
		t.Parallel()

		// Test parseGPUs through ContainerConfig conversion
		cfg := &container.ContainerConfig{
			Image: "nvidia/cuda:11.0-base",
			GPUs:  "all",
		}

		_, hostCfg, _, err := toDockerContainerConfig(cfg)
		require.NoError(t, err)
		require.NotNil(t, hostCfg)
		assert.NotEmpty(t, hostCfg.Resources.DeviceRequests)
		assert.Equal(t, []string{"gpu"}, hostCfg.Resources.DeviceRequests[0].Capabilities[0])
	})

	t.Run("ContainerdValidateConfigUnsupportedOptions", func(t *testing.T) {
		t.Parallel()

		rt := &ContainerdRuntime{}

		// Containerd should reject unsupported port mappings
		cfgPort := &container.ContainerConfig{
			Image: "alpine:latest",
			Ports: []string{"8080:80"},
		}

		err := rt.ValidateConfig(cfgPort)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "port mapping is not supported")
	})
}
