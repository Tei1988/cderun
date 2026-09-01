package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/container"
)

func TestDeepCoverage_MockRuntimeLifecycle(t *testing.T) {
	t.Parallel()

	mockRt := NewMockRuntime()
	mockRt.CreatedContainerID = "mock-container-123"
	ctx := context.Background()

	cc := &container.ContainerConfig{
		Image:   "alpine:latest",
		Command: []string{"echo", "hello"},
	}

	err := mockRt.PullImage(ctx, cc.Image, "missing", 3, time.Second)
	require.NoError(t, err)
	assert.Equal(t, "alpine:latest", mockRt.PulledImage)

	id, err := mockRt.CreateContainer(ctx, cc)
	require.NoError(t, err)
	assert.Equal(t, "mock-container-123", id)

	err = mockRt.StartContainer(ctx, id)
	require.NoError(t, err)

	err = mockRt.SignalContainer(ctx, id, "SIGTERM")
	require.NoError(t, err)
	assert.Equal(t, "SIGTERM", mockRt.Signal)

	err = mockRt.ResizeContainerTTY(ctx, id, 24, 80)
	require.NoError(t, err)
	assert.Equal(t, uint(24), mockRt.Rows)
	assert.Equal(t, uint(80), mockRt.Cols)

	code, err := mockRt.WaitContainer(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, 0, code)

	err = mockRt.RemoveContainer(ctx, id)
	require.NoError(t, err)
}

func TestDeepCoverage_DockerUlimitsConversion(t *testing.T) {
	t.Parallel()

	ulimitsRaw := []container.Ulimit{
		{Name: "nofile", Soft: 1024, Hard: 2048},
		{Name: "nproc", Soft: 4096, Hard: 4096},
	}

	dockerUlimits := buildDockerUlimits(ulimitsRaw)
	require.Len(t, dockerUlimits, 2)
	assert.Equal(t, "nofile", dockerUlimits[0].Name)
	assert.Equal(t, int64(1024), dockerUlimits[0].Soft)
	assert.Equal(t, int64(2048), dockerUlimits[0].Hard)

	assert.Nil(t, buildDockerUlimits(nil))
}

func TestDeepCoverage_ContainerdValidateConfigUnsupportedFeatures(t *testing.T) {
	t.Parallel()

	ctrd := &ContainerdRuntime{}

	t.Run("Rejects unsupported init flag", func(t *testing.T) {
		err := ctrd.ValidateConfig(&container.ContainerConfig{Init: true})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "init")
	})

	t.Run("Rejects unsupported GPUs option", func(t *testing.T) {
		err := ctrd.ValidateConfig(&container.ContainerConfig{GPUs: "all"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "gpus")
	})

	t.Run("Rejects unsupported restart policy", func(t *testing.T) {
		err := ctrd.ValidateConfig(&container.ContainerConfig{Restart: "always"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "restart")
	})

	t.Run("Accepts valid basic configuration", func(t *testing.T) {
		err := ctrd.ValidateConfig(&container.ContainerConfig{
			Image:   "alpine:latest",
			Command: []string{"sh"},
		})
		require.NoError(t, err)
	})
}
