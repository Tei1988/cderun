package runtime

import (
	"testing"

	"cderun/internal/container"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Runtime_Docker_IPC_Mapping(t *testing.T) {
	cfg := &container.ContainerConfig{
		Image: "alpine",
		IPC:   "host",
	}

	_, hostCfg, _, err := toDockerContainerConfig(cfg)
	require.NoError(t, err)
	assert.Equal(t, "host", string(hostCfg.IpcMode))
}

func TestUnit_Runtime_Containerd_IPC_Validation(t *testing.T) {
	rt := &ContainerdRuntime{}

	cfg := &container.ContainerConfig{
		Image: "alpine",
		IPC:   "invalid-mode",
	}
	err := rt.ValidateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported IPC namespace mode")
}
