package runtime

import (
	"testing"

	"cderun/internal/container"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Runtime_Docker_ResourceLimits_Mapping(t *testing.T) {
	cfg := &container.ContainerConfig{
		Image:      "alpine",
		Cgroupns:   "host",
		GPUs:       "all",
		PidsLimit:  100,
		CPUShares:  512,
		CpusetCpus: "0,1",
		CpusetMems: "0",
	}

	_, hostCfg, _, err := toDockerContainerConfig(cfg)
	require.NoError(t, err)

	assert.Equal(t, "host", string(hostCfg.CgroupnsMode))
	require.NotNil(t, hostCfg.Resources.PidsLimit)
	assert.Equal(t, int64(100), *hostCfg.Resources.PidsLimit)
	assert.Equal(t, int64(512), hostCfg.Resources.CPUShares)
	assert.Equal(t, "0,1", hostCfg.Resources.CpusetCpus)
	assert.Equal(t, "0", hostCfg.Resources.CpusetMems)

	require.Len(t, hostCfg.Resources.DeviceRequests, 1)
	assert.Equal(t, "nvidia", hostCfg.Resources.DeviceRequests[0].Driver)
	assert.Equal(t, -1, hostCfg.Resources.DeviceRequests[0].Count)
}

func TestUnit_Runtime_Containerd_ResourceLimits_Validation(t *testing.T) {
	rt := &ContainerdRuntime{}

	t.Run("unsupported GPUs is rejected", func(t *testing.T) {
		cfg := &container.ContainerConfig{
			Image: "alpine",
			GPUs:  "all",
		}
		err := rt.ValidateConfig(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "gpus is not supported yet")
	})
}
