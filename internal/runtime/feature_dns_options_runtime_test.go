package runtime

import (
	"testing"

	"cderun/internal/container"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Runtime_Docker_DNSOptions_Mapping(t *testing.T) {
	cfg := &container.ContainerConfig{
		Image:      "alpine",
		DNSSearch:  []string{"example.com"},
		DNSOptions: []string{"ndots:3"},
	}

	_, hostCfg, _, err := toDockerContainerConfig(cfg)
	require.NoError(t, err)
	assert.Equal(t, []string{"example.com"}, hostCfg.DNSSearch)
	assert.Equal(t, []string{"ndots:3"}, hostCfg.DNSOptions)
}

func TestUnit_Runtime_Containerd_DNSOptions_Validation(t *testing.T) {
	rt := &ContainerdRuntime{}

	t.Run("unsupported DNSSearch", func(t *testing.T) {
		cfg := &container.ContainerConfig{
			Image:     "alpine",
			DNSSearch: []string{"example.com"},
		}
		err := rt.ValidateConfig(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "dns-search is not supported yet")
	})

	t.Run("unsupported DNSOptions", func(t *testing.T) {
		cfg := &container.ContainerConfig{
			Image:      "alpine",
			DNSOptions: []string{"timeout:2"},
		}
		err := rt.ValidateConfig(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "dns-option is not supported yet")
	})
}
