package runtime

import (
	"testing"

	"cderun/internal/container"

	"github.com/stretchr/testify/require"
)

func TestUnit_Runtime_ContainerLabelsPropagation(t *testing.T) {
	t.Run("toDockerContainerConfig includes cderun=true default label and custom labels", func(t *testing.T) {
		cfg := &container.ContainerConfig{
			Image: "alpine:latest",
			Labels: map[string]string{
				"env": "test",
				"app": "my-app",
			},
		}

		dockerCfg, _, _, err := toDockerContainerConfig(cfg)
		require.NoError(t, err)
		require.NotNil(t, dockerCfg)
		require.NotNil(t, dockerCfg.Labels)
		require.Equal(t, "true", dockerCfg.Labels["cderun"])
		require.Equal(t, "test", dockerCfg.Labels["env"])
		require.Equal(t, "my-app", dockerCfg.Labels["app"])
	})

	t.Run("toDockerContainerConfig preserves explicit cderun label override if present", func(t *testing.T) {
		cfg := &container.ContainerConfig{
			Image: "alpine:latest",
			Labels: map[string]string{
				"cderun": "custom-value",
			},
		}

		dockerCfg, _, _, err := toDockerContainerConfig(cfg)
		require.NoError(t, err)
		require.NotNil(t, dockerCfg)
		require.Equal(t, "custom-value", dockerCfg.Labels["cderun"])
	})
}
