package runtime

import (
	"context"
	"errors"
	"testing"

	"cderun/internal/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Runtime_DeepAdapter_MockRuntime(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("MockRuntime successful container lifecycle", func(t *testing.T) {
		mock := NewMockRuntime()
		mock.CreatedContainerID = "mock-container-id-123"
		assert.Equal(t, "mock", mock.Name())

		cc := &container.ContainerConfig{
			Image:   "alpine:latest",
			Command: []string{"echo", "hello"},
		}

		id, err := mock.CreateContainer(ctx, cc)
		require.NoError(t, err)
		assert.Equal(t, "mock-container-id-123", id)

		err = mock.StartContainer(ctx, id)
		require.NoError(t, err)

		exitCode, err := mock.WaitContainer(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, 0, exitCode)

		running, exitCode, err := mock.InspectContainer(ctx, id)
		require.NoError(t, err)
		assert.False(t, running)
		assert.Equal(t, 0, exitCode)

		err = mock.RemoveContainer(ctx, id)
		require.NoError(t, err)
	})

	t.Run("MockRuntime error propagation", func(t *testing.T) {
		mock := NewMockRuntime()
		mock.CreateErr = errors.New("failed to create")
		mock.StartErr = errors.New("failed to start")
		mock.WaitErr = errors.New("failed to wait")
		mock.RemoveErr = errors.New("failed to remove")

		cc := &container.ContainerConfig{Image: "alpine:latest"}

		_, err := mock.CreateContainer(ctx, cc)
		assert.ErrorContains(t, err, "failed to create")

		err = mock.StartContainer(ctx, "c1")
		assert.ErrorContains(t, err, "failed to start")

		_, err = mock.WaitContainer(ctx, "c1")
		assert.ErrorContains(t, err, "failed to wait")

		err = mock.RemoveContainer(ctx, "c1")
		assert.ErrorContains(t, err, "failed to remove")
	})
}

func TestUnit_Runtime_DeepAdapter_DockerAdapterHelpers(t *testing.T) {
	t.Parallel()

	t.Run("parseDockerRestartPolicy valid and invalid policies", func(t *testing.T) {
		p1, err := parseDockerRestartPolicy("always")
		require.NoError(t, err)
		assert.Equal(t, "always", string(p1.Name))

		p2, err := parseDockerRestartPolicy("on-failure:5")
		require.NoError(t, err)
		assert.Equal(t, "on-failure", string(p2.Name))
		assert.Equal(t, 5, p2.MaximumRetryCount)

		_, err = parseDockerRestartPolicy("invalid-policy")
		assert.Error(t, err)
	})

	t.Run("buildDockerUlimits conversion", func(t *testing.T) {
		ulimitsRaw := []container.Ulimit{
			{Name: "nofile", Soft: 1024, Hard: 2048},
			{Name: "nproc", Soft: 4096, Hard: 4096},
		}

		converted := buildDockerUlimits(ulimitsRaw)
		require.Len(t, converted, 2)
		assert.Equal(t, "nofile", converted[0].Name)
		assert.Equal(t, int64(1024), converted[0].Soft)
		assert.Equal(t, int64(2048), converted[0].Hard)

		assert.Nil(t, buildDockerUlimits(nil))
	})
}

func TestUnit_Runtime_DeepAdapter_ContainerdValidation(t *testing.T) {
	t.Parallel()

	ctrd := &ContainerdRuntime{}

	t.Run("ValidateConfig approves standard container config", func(t *testing.T) {
		cc := &container.ContainerConfig{
			Image:   "alpine:latest",
			Command: []string{"sh"},
		}
		assert.NoError(t, ctrd.ValidateConfig(cc))
	})

	t.Run("ValidateConfig rejects unsupported features", func(t *testing.T) {
		assert.Error(t, ctrd.ValidateConfig(&container.ContainerConfig{Init: true}))
		assert.Error(t, ctrd.ValidateConfig(&container.ContainerConfig{GPUs: "all"}))
		assert.Error(t, ctrd.ValidateConfig(&container.ContainerConfig{Restart: "always"}))
		assert.Error(t, ctrd.ValidateConfig(&container.ContainerConfig{DNSSearch: []string{"example.com"}}))
	})
}
