package runtime

import (
	"context"
	"testing"

	"cderun/internal/container"
	"cderun/internal/logging"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Docker_CreateContainer_ShmSize(t *testing.T) {
	t.Parallel()

	mock := &mockDockerClient{}
	runtime := &DockerRuntime{logger: logging.GetGlobalLogger(), client: mock, sleepFunc: noopSleepFunc}

	config := &container.ContainerConfig{
		Image:   "test-image",
		ShmSize: 1024 * 1024 * 256, // 256MB
	}

	_, err := runtime.CreateContainer(context.Background(), config)
	require.NoError(t, err)
	assert.Equal(t, int64(268435456), mock.createHostConfig.ShmSize)
}

func TestUnit_Containerd_ValidateConfig_ShmSize(t *testing.T) {
	t.Parallel()

	rt := &ContainerdRuntime{logger: logging.GetGlobalLogger()}

	t.Run("negative shm-size is rejected", func(t *testing.T) {
		cfg := &container.ContainerConfig{
			Image:   "alpine",
			ShmSize: -1,
		}
		err := rt.ValidateConfig(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "negative shm-size")
	})

	t.Run("positive shm-size is accepted", func(t *testing.T) {
		cfg := &container.ContainerConfig{
			Image:   "alpine",
			ShmSize: 268435456,
		}
		err := rt.ValidateConfig(cfg)
		assert.NoError(t, err)
	})
}
