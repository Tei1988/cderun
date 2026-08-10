package runtime

import (
	"testing"

	"cderun/internal/container"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Docker_toDockerContainerConfig_ShmSize(t *testing.T) {
	cfg := &container.ContainerConfig{
		ShmSize: "256m",
	}

	_, hostConfig, _, err := toDockerContainerConfig(cfg)
	require.NoError(t, err)
	assert.Equal(t, int64(268435456), hostConfig.ShmSize)
}

func TestUnit_Containerd_CreateContainer_ShmSize(t *testing.T) {
	// Containerd adapter can be unit-tested directly via CreateContainer with mock client
	// let's verify specs mapping logic or helper
	cfg := &container.ContainerConfig{
		ShmSize: "512m",
	}

	rt := &ContainerdRuntime{}
	err := rt.ValidateConfig(cfg)
	require.NoError(t, err)

	// Verify that invalid formats are rejected
	cfgInvalid := &container.ContainerConfig{
		ShmSize: "invalid",
	}
	err = rt.ValidateConfig(cfgInvalid)
	require.Error(t, err)

	cfgNegative := &container.ContainerConfig{
		ShmSize: "-256m",
	}
	err = rt.ValidateConfig(cfgNegative)
	require.Error(t, err)
}
