package runtime

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"cderun/internal/container"
)

func TestUnit_Runtime_Mock_AllMethods(t *testing.T) {
	mock := NewMockRuntime()
	ctx := context.Background()

	// PullImage
	_ = mock.PullImage(ctx, "img", "always")
	assert.Equal(t, "img", mock.GetPulledImage())

	// CreateContainer
	config := &container.ContainerConfig{Image: "img"}
	mock.CreatedContainerID = "c1"
	id, _ := mock.CreateContainer(ctx, config)
	assert.Equal(t, "c1", id)
	assert.Equal(t, config, mock.GetCreatedConfig())

	// StartContainer
	_ = mock.StartContainer(ctx, "c1")
	assert.Equal(t, "c1", mock.GetStartedContainerID())

	// WaitContainer
	mock.ExitCode = 42
	code, _ := mock.WaitContainer(ctx, "c1")
	assert.Equal(t, 42, code)
	assert.Equal(t, "c1", mock.GetWaitedContainerID())

	// RemoveContainer
	_ = mock.RemoveContainer(ctx, "c1")
	assert.Equal(t, "c1", mock.GetRemovedContainerID())

	// AttachContainer
	_ = mock.AttachContainer(ctx, "c1", false, nil, nil, nil, nil)
	assert.Equal(t, "c1", mock.GetAttachedContainerID())

	// ResizeContainerTTY
	_ = mock.ResizeContainerTTY(ctx, "c1", 10, 20)
	assert.Equal(t, "c1", mock.ResizedContainerID)
	r, c := mock.GetTTYSize()
	assert.Equal(t, uint(10), r)
	assert.Equal(t, uint(20), c)

	// SignalContainer
	_ = mock.SignalContainer(ctx, "c1", "SIGINT")
	assert.Equal(t, "c1", mock.SignaledContainerID)
	assert.Equal(t, "SIGINT", mock.Signal)

	// Name
	assert.Equal(t, "mock", mock.Name())

	// ResetCreatedConfig
	mock.ResetCreatedConfig()
	assert.Nil(t, mock.GetCreatedConfig())
}

func TestUnit_Runtime_Mock_New(t *testing.T) {
	mock := NewMockRuntime()
	assert.NotNil(t, mock)
}
