package runtime

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	"cderun/internal/container"
	"cderun/internal/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Containerd_Name(t *testing.T) {
	rt := &ContainerdRuntime{logger: logging.GetGlobalLogger()}
	assert.Equal(t, "containerd", rt.Name())
}

func TestUnit_Containerd_ParseSignal(t *testing.T) {
	tests := []struct {
		sig     string
		wanterr bool
	}{
		{"TERM", false},
		{"sigterm", false},
		{"9", false},
		{"15", false},
		{"HUP", false},
		{"INT", false},
		{"KILL", false},
		{"QUIT", false},
		{"USR1", false},
		{"USR2", false},
		{"WINCH", false},
		{"SIGKILL", false},
		{"sigkill", false},
		{"invalid", true},
		{"0", true},
		{"65", true},
		{"", true},
	}

	for _, tt := range tests {
		t.Run(tt.sig, func(t *testing.T) {
			_, err := parseSignal(tt.sig)
			if tt.wanterr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUnit_Containerd_CreateContainer_Validation(t *testing.T) {
	rt := &ContainerdRuntime{logger: logging.GetGlobalLogger()}

	t.Run("negative memory limit", func(t *testing.T) {
		_, err := rt.CreateContainer(context.Background(), &container.ContainerConfig{
			Memory: -1,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "negative memory limit")
	})

	t.Run("negative CPU limit", func(t *testing.T) {
		_, err := rt.CreateContainer(context.Background(), &container.ContainerConfig{
			CPUs: -1.0,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "negative CPU limit")
	})

	t.Run("effectively zero CPUs", func(t *testing.T) {
		_, err := rt.CreateContainer(context.Background(), &container.ContainerConfig{
			CPUs: 0.000001,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "too small")
	})

	t.Run("unsupported network", func(t *testing.T) {
		_, err := rt.CreateContainer(context.Background(), &container.ContainerConfig{
			Network: "custom",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Network \"custom\" is not supported yet")
	})

	t.Run("port mapping unsupported", func(t *testing.T) {
		_, err := rt.CreateContainer(context.Background(), &container.ContainerConfig{
			Ports: []string{"80:80"},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "port mapping is not supported yet")
	})

	t.Run("publish-all unsupported", func(t *testing.T) {
		_, err := rt.CreateContainer(context.Background(), &container.ContainerConfig{
			PublishAll: true,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "port mapping is not supported yet")
	})

	t.Run("expose unsupported", func(t *testing.T) {
		_, err := rt.CreateContainer(context.Background(), &container.ContainerConfig{
			Expose: []string{"80"},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "port mapping is not supported yet")
	})
}

func TestUnit_Containerd_PullImage_EarlyReturn(t *testing.T) {
	rt := &ContainerdRuntime{logger: logging.GetGlobalLogger()}

	t.Run("pull policy never", func(t *testing.T) {
		err := rt.PullImage(context.Background(), "alpine", "never", 3, 1*time.Second)
		assert.NoError(t, err)
	})
}

func TestUnit_Containerd_ResizeContainerTTY_Validation(t *testing.T) {
	rt := &ContainerdRuntime{logger: logging.GetGlobalLogger()}

	t.Run("rows overflow", func(t *testing.T) {
		// math.MaxUint32 + 1 can overflow at compile time if rows is uint and it's 32-bit.
		// Use a value that is guaranteed to overflow uint32 but still fits in uint if 64-bit,
		// or just use a large constant that we know is greater than math.MaxUint32.
		var largeVal uint = math.MaxUint32 + 1
		err := rt.ResizeContainerTTY(context.Background(), "cont1", largeVal, 80)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exceeds maximum")
	})

	t.Run("cols overflow", func(t *testing.T) {
		var largeVal uint = math.MaxUint32 + 1
		err := rt.ResizeContainerTTY(context.Background(), "cont1", 24, largeVal)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exceeds maximum")
	})
}

func TestUnit_Containerd_New_Error(t *testing.T) {
	// client.New will fail if the socket path is invalid or empty
	_, err := NewContainerdRuntime("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect to containerd")
}

func TestUnit_Containerd_NotifyWait_NoPanic(t *testing.T) {
	rt := &ContainerdRuntime{
		logger: logging.GetGlobalLogger(),
		ioWait: make(map[string]chan error),
	}

	// Should not panic when containerID is not in ioWait
	rt.notifyWait("non-existent", nil)

	// Should not block when channel is full
	waitC := make(chan error, 1)
	waitC <- fmt.Errorf("initial")
	rt.ioWait["cont1"] = waitC
	rt.notifyWait("cont1", fmt.Errorf("new"))

	err := <-waitC
	assert.Equal(t, "initial", err.Error())
}
