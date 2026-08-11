package runtime

import (
	"testing"

	"cderun/internal/container"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Runtime_Docker_RestartPolicy_Mapping(t *testing.T) {
	cfg := &container.ContainerConfig{
		Image:   "alpine",
		Restart: "on-failure:5",
	}

	_, hostCfg, _, err := toDockerContainerConfig(cfg)
	require.NoError(t, err)

	assert.Equal(t, "on-failure", string(hostCfg.RestartPolicy.Name))
	assert.Equal(t, 5, hostCfg.RestartPolicy.MaximumRetryCount)

	t.Run("invalid restart policy in docker adapter returns error", func(t *testing.T) {
		invalidCfg := &container.ContainerConfig{
			Image:   "alpine",
			Restart: "always:5",
		}
		_, _, _, err := toDockerContainerConfig(invalidCfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "does not support retry suffix")
	})
}

func TestUnit_Runtime_Containerd_RestartPolicy_Validation(t *testing.T) {
	rt := &ContainerdRuntime{}

	t.Run("unsupported restart policy is rejected", func(t *testing.T) {
		cfg := &container.ContainerConfig{
			Image:   "alpine",
			Restart: "always",
		}
		err := rt.ValidateConfig(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "restart policy is not supported yet")
	})
}
