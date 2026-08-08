package runtime

import (
	"context"
	"testing"

	"cderun/internal/container"
	"cderun/internal/logging"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Docker_CreateContainer_Init(t *testing.T) {
	t.Parallel()

	t.Run("Init enabled maps to HostConfig.Init", func(t *testing.T) {
		mock := &mockDockerClient{}
		runtime := &DockerRuntime{logger: logging.GetGlobalLogger(), client: mock, sleepFunc: noopSleepFunc}

		config := &container.ContainerConfig{
			Image: "test-image",
			Init:  true,
		}

		_, err := runtime.CreateContainer(context.Background(), config)
		require.NoError(t, err)
		require.NotNil(t, mock.createHostConfig)
		require.NotNil(t, mock.createHostConfig.Init)
		assert.True(t, *mock.createHostConfig.Init)
	})

	t.Run("Init disabled maps to HostConfig.Init as false", func(t *testing.T) {
		mock := &mockDockerClient{}
		runtime := &DockerRuntime{logger: logging.GetGlobalLogger(), client: mock, sleepFunc: noopSleepFunc}

		config := &container.ContainerConfig{
			Image: "test-image",
			Init:  false,
		}

		_, err := runtime.CreateContainer(context.Background(), config)
		require.NoError(t, err)
		require.NotNil(t, mock.createHostConfig)
		require.NotNil(t, mock.createHostConfig.Init)
		assert.False(t, *mock.createHostConfig.Init)
	})
}

func TestUnit_Containerd_ValidateConfig_Init(t *testing.T) {
	t.Parallel()

	runtime := &ContainerdRuntime{}

	t.Run("Init enabled is rejected", func(t *testing.T) {
		config := &container.ContainerConfig{
			Image: "test-image",
			Init:  true,
		}

		err := runtime.ValidateConfig(config)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "containerd runtime: init is not supported yet")
	})

	t.Run("Init disabled passes validation", func(t *testing.T) {
		config := &container.ContainerConfig{
			Image: "test-image",
			Init:  false,
		}

		err := runtime.ValidateConfig(config)
		require.NoError(t, err)
	})
}
