package runtime

import (
	"context"
	"testing"
	"time"

	"cderun/internal/container"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Runtime_ScenariosRefinement_MockAndAdapters(t *testing.T) {
	t.Parallel()

	t.Run("mock_runtime_container_lifecycle", func(t *testing.T) {
		t.Parallel()

		mockRt := &MockRuntime{
			CreatedContainerID: "test-mock-id-999",
			ExitCode:           42,
		}

		ctx := context.Background()
		cfg := &container.ContainerConfig{
			Image:   "alpine:latest",
			Command: []string{"echo", "hello"},
		}

		// Create
		id, err := mockRt.CreateContainer(ctx, cfg)
		require.NoError(t, err)
		assert.Equal(t, "test-mock-id-999", id)
		assert.Equal(t, cfg, mockRt.GetCreatedConfig())

		// Start
		err = mockRt.StartContainer(ctx, id)
		require.NoError(t, err)

		// Attach
		err = mockRt.AttachContainer(ctx, id, false, nil, nil, nil, nil)
		require.NoError(t, err)

		// Wait
		exitCode, err := mockRt.WaitContainer(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, 42, exitCode)

		// Remove
		err = mockRt.RemoveContainer(ctx, id)
		require.NoError(t, err)
	})

	t.Run("docker_adapter_gpu_parsing", func(t *testing.T) {
		t.Parallel()

		reqsAll, err := parseGPUs("all")
		require.NoError(t, err)
		require.Len(t, reqsAll, 1)
		assert.Equal(t, []string{"gpu"}, reqsAll[0].Capabilities[0])
		assert.Equal(t, -1, reqsAll[0].Count)

		reqsCount, err := parseGPUs("count=2")
		require.NoError(t, err)
		require.Len(t, reqsCount, 1)
		assert.Equal(t, 2, reqsCount[0].Count)

		reqsDev, err := parseGPUs("device=0,1")
		require.NoError(t, err)
		require.Len(t, reqsDev, 1)
		assert.Equal(t, []string{"0", "1"}, reqsDev[0].DeviceIDs)
	})

	t.Run("containerd_adapter_validation", func(t *testing.T) {
		t.Parallel()

		rt := &ContainerdRuntime{}

		// ValidateConfig with unsupported features (e.g. ports, network)
		cfgUnsupported := &container.ContainerConfig{
			Image: "alpine:latest",
			Ports: []string{"8080:80"},
		}

		err := rt.ValidateConfig(cfgUnsupported)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "containerd runtime")

		// ValidateConfig with valid config
		cfgValid := &container.ContainerConfig{
			Image:   "alpine:latest",
			Command: []string{"sh"},
		}

		err = rt.ValidateConfig(cfgValid)
		require.NoError(t, err)
	})
}

func TestUnit_Runtime_ScenariosRefinement_Extra(t *testing.T) {
	t.Parallel()

	t.Run("mock_runtime_pull_and_inspect", func(t *testing.T) {
		t.Parallel()

		mockRt := &MockRuntime{}
		ctx := context.Background()

		err := mockRt.PullImage(ctx, "alpine:latest", "always", 1, 100*time.Millisecond)
		require.NoError(t, err)
		assert.Equal(t, "alpine:latest", mockRt.GetPulledImage())

		running, exitCode, err := mockRt.InspectContainer(ctx, "test-mock-id-999")
		require.NoError(t, err)
		assert.False(t, running)
		assert.Equal(t, 0, exitCode)
	})
}
