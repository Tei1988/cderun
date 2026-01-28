package runtime

import (
	"cderun/internal/container"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMockRuntime(t *testing.T) {
	mock := &MockRuntime{
		CreatedContainerID: "test-id",
		ExitCode:           42,
	}

	var _ ContainerRuntime = mock // Verify interface compliance

	ctx := context.Background()
	config := &container.ContainerConfig{Image: "alpine"}

	id, err := mock.CreateContainer(ctx, config)
	assert.NoError(t, err)
	assert.Equal(t, "test-id", id)
	assert.Equal(t, config, mock.CreatedConfig)

	err = mock.StartContainer(ctx, id)
	assert.NoError(t, err)
	assert.Equal(t, id, mock.StartedContainerID)

	code, err := mock.WaitContainer(ctx, id)
	assert.NoError(t, err)
	assert.Equal(t, 42, code)
	assert.Equal(t, id, mock.WaitedContainerID)

	err = mock.RemoveContainer(ctx, id)
	assert.NoError(t, err)
	assert.Equal(t, id, mock.RemovedContainerID)

	assert.Equal(t, "mock", mock.Name())
}
