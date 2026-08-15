package runtime

import (
	"context"
	"testing"
	"time"

	"cderun/internal/container"

	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUnit_Runtime_DockerAdapter_GPU_Parsing tests parseGPUs in Docker adapter.
// References: docs/features/command-line-options.md
func TestUnit_Runtime_DockerAdapter_GPU_Parsing(t *testing.T) {
	t.Parallel()

	t.Run("parseGPUs with all", func(t *testing.T) {
		t.Parallel()
		reqs, err := parseGPUs("all")
		require.NoError(t, err)
		require.Len(t, reqs, 1)
		assert.Equal(t, "nvidia", reqs[0].Driver)
		assert.Equal(t, -1, reqs[0].Count)
		assert.Equal(t, [][]string{{"gpu"}}, reqs[0].Capabilities)
	})

	t.Run("parseGPUs with count=2", func(t *testing.T) {
		t.Parallel()
		reqs, err := parseGPUs("count=2")
		require.NoError(t, err)
		require.Len(t, reqs, 1)
		assert.Equal(t, 2, reqs[0].Count)
		assert.Empty(t, reqs[0].DeviceIDs)
	})

	t.Run("parseGPUs with device=0,1", func(t *testing.T) {
		t.Parallel()
		reqs, err := parseGPUs("device=0,1")
		require.NoError(t, err)
		require.Len(t, reqs, 1)
		assert.Equal(t, []string{"0", "1"}, reqs[0].DeviceIDs)
		assert.Equal(t, 0, reqs[0].Count)
	})

	t.Run("parseGPUs invalid cases", func(t *testing.T) {
		t.Parallel()
		_, errEmptyDevice := parseGPUs("device=")
		assert.Error(t, errEmptyDevice)

		_, errEmptyCount := parseGPUs("count=")
		assert.Error(t, errEmptyCount)

		_, errInvalidCount := parseGPUs("count=-5")
		assert.Error(t, errInvalidCount)

		_, errMalformed := parseGPUs("invalid_selector")
		assert.Error(t, errMalformed)
	})
}

// TestUnit_Runtime_DockerAdapter_ToDockerContainerConfig tests toDockerContainerConfig mapping.
// References: docs/features/command-line-options.md
func TestUnit_Runtime_DockerAdapter_ToDockerContainerConfig(t *testing.T) {
	t.Parallel()

	cc := &container.ContainerConfig{
		Image:    "alpine:latest",
		ShmSize:  "512m",
		IPC:      "host",
		Pid:      "host",
		Cgroupns: "host",
		Sysctls: map[string]string{
			"net.ipv4.ip_forward": "1",
		},
		Ulimits: []container.Ulimit{
			{Name: "nofile", Soft: 1024, Hard: 2048},
		},
		Restart: "on-failure:3",
		GPUs:    "all",
	}

	containerConfig, hostConfig, _, err := toDockerContainerConfig(cc)
	require.NoError(t, err)
	require.NotNil(t, containerConfig)
	require.NotNil(t, hostConfig)

	assert.Equal(t, int64(536870912), hostConfig.ShmSize) // 512m
	assert.Equal(t, "host", string(hostConfig.IpcMode))
	assert.Equal(t, "host", string(hostConfig.PidMode))
	assert.Equal(t, "host", string(hostConfig.CgroupnsMode))
	assert.Equal(t, "1", hostConfig.Sysctls["net.ipv4.ip_forward"])
	assert.Equal(t, dockercontainer.RestartPolicyMode("on-failure"), hostConfig.RestartPolicy.Name)
	assert.Equal(t, 3, hostConfig.RestartPolicy.MaximumRetryCount)

	require.Len(t, hostConfig.Ulimits, 1)
	assert.Equal(t, "nofile", hostConfig.Ulimits[0].Name)
	assert.Equal(t, int64(1024), hostConfig.Ulimits[0].Soft)
	assert.Equal(t, int64(2048), hostConfig.Ulimits[0].Hard)

	require.Len(t, hostConfig.DeviceRequests, 1)
	assert.Equal(t, "nvidia", hostConfig.DeviceRequests[0].Driver)
}

// TestUnit_Runtime_MockRuntime_Lifecycle_Scenarios tests MockRuntime lifecycle and signal validation.
// References: docs/testing/strategy.md
func TestUnit_Runtime_MockRuntime_Lifecycle_Scenarios(t *testing.T) {
	t.Parallel()

	mock := NewMockRuntime()
	ctx := context.Background()

	// Pull image
	err := mock.PullImage(ctx, "alpine:latest", "always", 3, 10*time.Millisecond)
	require.NoError(t, err)
	assert.Equal(t, "alpine:latest", mock.GetPulledImage())

	// Create container
	cc := &container.ContainerConfig{
		Image:   "alpine:latest",
		Command: []string{"sh", "-c", "echo hello"},
	}
	id, err := mock.CreateContainer(ctx, cc)
	require.NoError(t, err)
	assert.Equal(t, mock.CreatedContainerID, id)
	assert.Equal(t, cc, mock.GetCreatedConfig())

	// Start container
	err = mock.StartContainer(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, id, mock.GetStartedContainerID())

	// Signal container with valid and invalid signals
	assert.NoError(t, mock.SignalContainer(ctx, id, "SIGTERM"))
	assert.NoError(t, mock.SignalContainer(ctx, id, "SIGKILL"))
	assert.Error(t, mock.SignalContainer(ctx, id, "INVALID_SIGNAL; rm -rf /"))

	// Remove container
	err = mock.RemoveContainer(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, id, mock.GetRemovedContainerID())

	// Close mock runtime
	assert.NoError(t, mock.Close())
}
