package runtime

import (
	"cderun/internal/container"
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnit_Mock_Lifecycle(t *testing.T) {
	mock := &MockRuntime{
		CreatedContainerID: "test-id",
		ExitCode:           42,
	}

	var _ ContainerRuntime = mock // Verify interface compliance

	ctx := context.Background()
	config := &container.ContainerConfig{Image: "alpine"}

	// Pull
	err := mock.PullImage(ctx, "alpine", "always")
	assert.NoError(t, err)
	assert.Equal(t, "alpine", mock.GetPulledImage())

	// Create
	id, err := mock.CreateContainer(ctx, config)
	assert.NoError(t, err)
	assert.Equal(t, "test-id", id)
	assert.Equal(t, config, mock.GetCreatedConfig())

	// Start
	err = mock.StartContainer(ctx, id)
	assert.NoError(t, err)
	assert.Equal(t, id, mock.GetStartedContainerID())

	// Wait
	code, err := mock.WaitContainer(ctx, id)
	assert.NoError(t, err)
	assert.Equal(t, 42, code)
	assert.Equal(t, id, mock.GetWaitedContainerID())

	// Attach
	err = mock.AttachContainer(ctx, id, true, nil, nil, nil)
	assert.NoError(t, err)
	assert.Equal(t, id, mock.GetAttachedContainerID())

	// Resize
	err = mock.ResizeContainerTTY(ctx, id, 24, 80)
	assert.NoError(t, err)
	assert.Equal(t, id, mock.ResizedContainerID)
	rows, cols := mock.GetTTYSize()
	assert.Equal(t, uint(24), rows)
	assert.Equal(t, uint(80), cols)

	// Signal
	err = mock.SignalContainer(ctx, id, "SIGTERM")
	assert.NoError(t, err)
	assert.Equal(t, id, mock.SignaledContainerID)
	assert.Equal(t, "SIGTERM", mock.Signal)

	// Remove
	err = mock.RemoveContainer(ctx, id)
	assert.NoError(t, err)
	assert.Equal(t, id, mock.GetRemovedContainerID())

	assert.Equal(t, "mock", mock.Name())
}

func TestUnit_Mock_Errors(t *testing.T) {
	mock := &MockRuntime{
		PullErr:   assert.AnError,
		CreateErr: assert.AnError,
		StartErr:  assert.AnError,
		WaitErr:   assert.AnError,
		RemoveErr: assert.AnError,
		AttachErr: assert.AnError,
		ResizeErr: assert.AnError,
		SignalErr: assert.AnError,
	}

	ctx := context.Background()

	assert.ErrorIs(t, mock.PullImage(ctx, "img", "always"), assert.AnError)
	_, err := mock.CreateContainer(ctx, &container.ContainerConfig{})
	assert.ErrorIs(t, err, assert.AnError)
	assert.ErrorIs(t, mock.StartContainer(ctx, "id"), assert.AnError)
	_, err = mock.WaitContainer(ctx, "id")
	assert.ErrorIs(t, err, assert.AnError)
	assert.ErrorIs(t, mock.RemoveContainer(ctx, "id"), assert.AnError)
	assert.ErrorIs(t, mock.AttachContainer(ctx, "id", true, nil, nil, nil), assert.AnError)
	assert.ErrorIs(t, mock.ResizeContainerTTY(ctx, "id", 24, 80), assert.AnError)
	assert.ErrorIs(t, mock.SignalContainer(ctx, "id", "SIGKILL"), assert.AnError)
}

func TestUnit_Mock_AttachIO(t *testing.T) {
	mock := &MockRuntime{}
	stdin := strings.NewReader("input")
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	err := mock.AttachContainer(context.Background(), "id", false, stdin, stdout, stderr)
	assert.NoError(t, err)
	assert.Equal(t, "id", mock.GetAttachedContainerID())
}
