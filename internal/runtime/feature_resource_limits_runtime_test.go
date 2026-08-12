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

	t.Run("valid count= GPU selector", func(t *testing.T) {
		cfgGpus := &container.ContainerConfig{Image: "alpine", GPUs: "count=2"}
		_, hostCfg, _, err := toDockerContainerConfig(cfgGpus)
		require.NoError(t, err)
		require.Len(t, hostCfg.Resources.DeviceRequests, 1)
		assert.Equal(t, 2, hostCfg.Resources.DeviceRequests[0].Count)
	})

	t.Run("valid count=-1 GPU selector", func(t *testing.T) {
		cfgGpus := &container.ContainerConfig{Image: "alpine", GPUs: "count=-1"}
		_, hostCfg, _, err := toDockerContainerConfig(cfgGpus)
		require.NoError(t, err)
		require.Len(t, hostCfg.Resources.DeviceRequests, 1)
		assert.Equal(t, -1, hostCfg.Resources.DeviceRequests[0].Count)
	})

	t.Run("valid device= GPU selector", func(t *testing.T) {
		cfgGpus := &container.ContainerConfig{Image: "alpine", GPUs: "device=0,1"}
		_, hostCfg, _, err := toDockerContainerConfig(cfgGpus)
		require.NoError(t, err)
		require.Len(t, hostCfg.Resources.DeviceRequests, 1)
		assert.Equal(t, 0, hostCfg.Resources.DeviceRequests[0].Count)
		assert.Equal(t, []string{"0", "1"}, hostCfg.Resources.DeviceRequests[0].DeviceIDs)
	})

	t.Run("invalid grammar GPU selector", func(t *testing.T) {
		cfgGpus := &container.ContainerConfig{Image: "alpine", GPUs: "invalid=gpu"}
		_, _, _, err := toDockerContainerConfig(cfgGpus)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown gpu selector or malformed grammar")
	})

	t.Run("malformed count value GPU selector", func(t *testing.T) {
		cfgGpus := &container.ContainerConfig{Image: "alpine", GPUs: "count=-2"}
		_, _, _, err := toDockerContainerConfig(cfgGpus)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid count value")
	})

	t.Run("zero count value GPU selector", func(t *testing.T) {
		cfgGpus := &container.ContainerConfig{Image: "alpine", GPUs: "count=0"}
		_, _, _, err := toDockerContainerConfig(cfgGpus)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid count value")
	})

	t.Run("empty device value GPU selector", func(t *testing.T) {
		cfgGpus := &container.ContainerConfig{Image: "alpine", GPUs: "device="}
		_, _, _, err := toDockerContainerConfig(cfgGpus)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "empty device selector")
	})
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

	t.Run("unsupported Cgroupns mode", func(t *testing.T) {
		cfg := &container.ContainerConfig{
			Image:    "alpine",
			Cgroupns: "invalid-mode",
		}
		err := rt.ValidateConfig(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported cgroup namespace mode")
	})
}
