package runtime

import (
	"context"
	"testing"

	"cderun/internal/container"

	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFeatureScenarios_AdapterBoundary_RuntimeOperations(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("MockRuntime_LifecycleOperations", func(t *testing.T) {
		t.Parallel()

		rt := NewMockRuntime()
		cfg := &container.ContainerConfig{
			Image:   "alpine:latest",
			Command: []string{"echo", "hello"},
		}

		containerID, err := rt.CreateContainer(ctx, cfg)
		require.NoError(t, err)

		err = rt.StartContainer(ctx, containerID)
		require.NoError(t, err)

		exitCode, err := rt.WaitContainer(ctx, containerID)
		require.NoError(t, err)
		assert.Equal(t, 0, exitCode)

		err = rt.RemoveContainer(ctx, containerID)
		require.NoError(t, err)
	})

	t.Run("DockerAdapter_ConversionHelpers", func(t *testing.T) {
		t.Parallel()

		cfg := &container.ContainerConfig{
			Image: "alpine:latest",
			Ulimits: []container.Ulimit{
				{Name: "nofile", Soft: 1024, Hard: 2048},
				{Name: "nproc", Soft: 4096, Hard: 4096},
			},
			Restart: "on-failure:3",
		}

		dockerCfg, hostCfg, _, err := toDockerContainerConfig(cfg)
		require.NoError(t, err)
		require.NotNil(t, dockerCfg)
		require.NotNil(t, hostCfg)

		assert.Equal(t, "alpine:latest", dockerCfg.Image)
		assert.Len(t, hostCfg.Ulimits, 2)
		assert.ElementsMatch(t, []container.Ulimit{
			{Name: "nofile", Soft: 1024, Hard: 2048},
			{Name: "nproc", Soft: 4096, Hard: 4096},
		}, []container.Ulimit{
			{Name: hostCfg.Ulimits[0].Name, Soft: hostCfg.Ulimits[0].Soft, Hard: hostCfg.Ulimits[0].Hard},
			{Name: hostCfg.Ulimits[1].Name, Soft: hostCfg.Ulimits[1].Soft, Hard: hostCfg.Ulimits[1].Hard},
		})
		assert.Equal(t, dockercontainer.RestartPolicyMode("on-failure"), hostCfg.RestartPolicy.Name)
		assert.Equal(t, 3, hostCfg.RestartPolicy.MaximumRetryCount)
	})

	t.Run("Containerd_ValidateConfigRules", func(t *testing.T) {
		t.Parallel()

		rt := &ContainerdRuntime{}

		// Valid configuration passes containerd validation
		validCfg := &container.ContainerConfig{
			Image:   "alpine:latest",
			Command: []string{"sh"},
		}
		require.NoError(t, rt.ValidateConfig(validCfg))

		// Unsupported restart policy in containerd returns error
		unsupportedCfg := &container.ContainerConfig{
			Image:   "alpine:latest",
			Restart: "always",
		}
		require.Error(t, rt.ValidateConfig(unsupportedCfg))
	})
}
