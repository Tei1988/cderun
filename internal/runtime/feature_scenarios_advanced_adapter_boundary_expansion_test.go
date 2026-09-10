package runtime

import (
	"context"
	"testing"

	"cderun/internal/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFeatureScenarios_AdvancedAdapterBoundaryExpansion_MockRuntime(t *testing.T) {
	t.Parallel()

	rt := NewMockRuntime()
	ctx := context.Background()

	assert.Equal(t, "mock", rt.Name())

	cfg := &container.ContainerConfig{
		Image:   "alpine:latest",
		Command: []string{"echo", "test"},
	}

	err := rt.ValidateConfig(cfg)
	require.NoError(t, err)

	cid, err := rt.CreateContainer(ctx, cfg)
	require.NoError(t, err)

	err = rt.StartContainer(ctx, cid)
	require.NoError(t, err)

	status, err := rt.WaitContainer(ctx, cid)
	require.NoError(t, err)
	assert.Equal(t, 0, status)

	err = rt.RemoveContainer(ctx, cid)
	require.NoError(t, err)
}

func TestFeatureScenarios_AdvancedAdapterBoundaryExpansion_ContainerdValidateConfig(t *testing.T) {
	t.Parallel()

	cr := &ContainerdRuntime{}

	t.Run("Valid config", func(t *testing.T) {
		t.Parallel()
		cfg := &container.ContainerConfig{
			Image: "alpine:latest",
		}
		err := cr.ValidateConfig(cfg)
		require.NoError(t, err)
	})

	t.Run("Unsupported ports error", func(t *testing.T) {
		t.Parallel()
		cfg := &container.ContainerConfig{
			Image: "alpine:latest",
			Ports: []string{"8080:80"},
		}
		err := cr.ValidateConfig(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "port mapping is not supported")
	})
}
