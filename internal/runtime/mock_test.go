package runtime

import (
	"cderun/internal/container"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Runtime_Mock_AllMethods(t *testing.T) {
	mock := &MockRuntime{
		CreatedContainerID: "test-id",
		ExitCode:           42,
	}

	var _ ContainerRuntime = mock // Verify interface compliance

	ctx := context.Background()
	config := &container.ContainerConfig{Image: "alpine"}

	// Test PullImage
	err := mock.PullImage(ctx, "alpine", "always")
	require.NoError(t, err)
	assert.Equal(t, "alpine", mock.GetPulledImage())

	// Test CreateContainer
	id, err := mock.CreateContainer(ctx, config)
	require.NoError(t, err)
	assert.Equal(t, "test-id", id)
	assert.Equal(t, config, mock.GetCreatedConfig())

	// Test StartContainer
	err = mock.StartContainer(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, id, mock.GetStartedContainerID())

	// Test WaitContainer
	code, err := mock.WaitContainer(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, 42, code)
	assert.Equal(t, id, mock.GetWaitedContainerID())

	// Test RemoveContainer
	err = mock.RemoveContainer(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, id, mock.GetRemovedContainerID())

	// Test AttachContainer
	err = mock.AttachContainer(ctx, id, true, nil, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, id, mock.GetAttachedContainerID())

	// Test ResizeContainerTTY
	err = mock.ResizeContainerTTY(ctx, id, 24, 80)
	require.NoError(t, err)
	rows, cols := mock.GetTTYSize()
	assert.Equal(t, uint(24), rows)
	assert.Equal(t, uint(80), cols)

	// Test SignalContainer
	err = mock.SignalContainer(ctx, id, "SIGINT")
	require.NoError(t, err)

	assert.Equal(t, "mock", mock.Name())
}

func TestUnit_Runtime_Mock_New(t *testing.T) {
	m := NewMockRuntime()
	assert.NotNil(t, m)
}
