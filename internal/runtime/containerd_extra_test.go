package runtime

import (
	"testing"

	"cderun/internal/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Containerd_WithContainerdLogger(t *testing.T) {
	logger := logging.GetGlobalLogger()
	rt := &ContainerdRuntime{}
	opt := WithContainerdLogger(logger)
	opt(rt)
	assert.Equal(t, logger, rt.logger)
}

func TestUnit_Containerd_NewContainerdRuntime_Error(t *testing.T) {
	// Empty socket should cause an error in client.New
	_, err := NewContainerdRuntime("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect to containerd")
}

func TestUnit_Containerd_NotifyWait(t *testing.T) {
	rt := &ContainerdRuntime{
		ioWait: make(map[string]chan error),
	}
	containerID := "test-container"
	waitC := make(chan error, 1)
	rt.ioWait[containerID] = waitC

	expectedErr := assert.AnError
	rt.notifyWait(containerID, expectedErr)

	select {
	case err := <-waitC:
		assert.Equal(t, expectedErr, err)
	default:
		t.Fatal("expected error in waitC")
	}
}

func TestUnit_Containerd_NotifyWait_NoChannel(t *testing.T) {
	rt := &ContainerdRuntime{
		ioWait: make(map[string]chan error),
	}
	// Should not panic
	rt.notifyWait("non-existent", nil)
}

func TestUnit_Containerd_NotifyWait_FullChannel(t *testing.T) {
	rt := &ContainerdRuntime{
		ioWait: make(map[string]chan error),
	}
	containerID := "test-container"
	waitC := make(chan error, 1)
	rt.ioWait[containerID] = waitC

	// Fill the channel
	waitC <- assert.AnError

	// Should not block
	rt.notifyWait(containerID, assert.AnError)
}
