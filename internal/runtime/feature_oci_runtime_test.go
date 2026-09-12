package runtime

import (
	"testing"

	"cderun/internal/container"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Runtime_OciRuntime_DockerAdapter(t *testing.T) {
	t.Run("toDockerContainerConfig passes OciRuntime to HostConfig.Runtime", func(t *testing.T) {
		cfg := &container.ContainerConfig{
			Image:      "alpine",
			OciRuntime: "runsc",
		}

		_, hostCfg, _, err := toDockerContainerConfig(cfg)
		require.NoError(t, err)
		assert.Equal(t, "runsc", hostCfg.Runtime)
	})

	t.Run("toDockerContainerConfig defaults HostConfig.Runtime to empty string", func(t *testing.T) {
		cfg := &container.ContainerConfig{
			Image: "alpine",
		}

		_, hostCfg, _, err := toDockerContainerConfig(cfg)
		require.NoError(t, err)
		assert.Equal(t, "", hostCfg.Runtime)
	})
}

func TestUnit_Runtime_OciRuntime_ContainerdAdapter(t *testing.T) {
	t.Run("ValidateConfig returns error when OciRuntime is specified for containerd", func(t *testing.T) {
		rt := &ContainerdRuntime{}
		cfg := &container.ContainerConfig{
			Image:      "alpine",
			OciRuntime: "crun",
		}

		err := rt.ValidateConfig(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "containerd runtime: oci-runtime is not supported yet")
	})

	t.Run("ValidateConfig passes when OciRuntime is empty for containerd", func(t *testing.T) {
		rt := &ContainerdRuntime{}
		cfg := &container.ContainerConfig{
			Image: "alpine",
		}

		err := rt.ValidateConfig(cfg)
		require.NoError(t, err)
	})
}
