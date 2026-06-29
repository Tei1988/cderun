package runtime

import (
	"context"
	"fmt"
	"testing"
	"time"

	"cderun/internal/container"
	"cderun/internal/logging"
	"github.com/containerd/errdefs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Containerd_Name(t *testing.T) {
	rt := &ContainerdRuntime{logger: logging.GetGlobalLogger()}
	assert.Equal(t, "containerd", rt.Name())
}

func TestUnit_Containerd_Options(t *testing.T) {
	logger := logging.GetGlobalLogger()
	rt := &ContainerdRuntime{}
	opt := WithContainerdLogger(logger)
	opt(rt)
	assert.Equal(t, logger, rt.logger)
}

func TestUnit_Containerd_PullImage_Never(t *testing.T) {
	rt := &ContainerdRuntime{}
	err := rt.PullImage(context.Background(), "img", "never", 3, 1*time.Second)
	assert.NoError(t, err)
}

func TestUnit_Containerd_Lifecycle_Local(t *testing.T) {
	// NewContainerdRuntime usually succeeds even if socket is missing
	// as connection is lazy or handled by grpc.
	rt, err := NewContainerdRuntime("/tmp/containerd.sock")
	require.NoError(t, err)
	defer rt.Close()
	assert.Equal(t, "containerd", rt.Name())
}

func TestUnit_Containerd_NotifyWait(t *testing.T) {
	waitC := make(chan error, 1)
	rt := &ContainerdRuntime{
		ioWait: map[string]chan error{"c1": waitC},
	}
	err := fmt.Errorf("wait err")
	rt.notifyWait("c1", err)
	assert.Equal(t, err, <-waitC)

	// Should not block or panic for unknown id
	rt.notifyWait("unknown", err)
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
		{"invalid", true},
		{"0", true},
		{"65", true},
		{"SIGINVALID", true},
		{"SIG", true},
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

	t.Run("publish all unsupported", func(t *testing.T) {
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

func TestUnit_Containerd_ResizeContainerTTY_Validation(t *testing.T) {
	rt := &ContainerdRuntime{}
	t.Run("rows too large", func(t *testing.T) {
		err := rt.ResizeContainerTTY(context.Background(), "id", 1<<33, 80)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "terminal size exceeds maximum")
	})
	t.Run("cols too large", func(t *testing.T) {
		err := rt.ResizeContainerTTY(context.Background(), "id", 24, 1<<33)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "terminal size exceeds maximum")
	})
}


func TestUnit_Runtime_Common_IsRetryablePullError(t *testing.T) {
	tests := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{fmt.Errorf("some other error"), false},
		{fmt.Errorf("connection refused"), true},
		{fmt.Errorf("rate limit exceeded"), true},
		{fmt.Errorf("unauthorized: token expired"), true},
		{errdefs.ErrUnavailable, true},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%v", tt.err), func(t *testing.T) {
			assert.Equal(t, tt.want, IsRetryablePullError(tt.err))
		})
	}
}

func TestUnit_Runtime_Common_SleepFunc(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		err := SleepFunc(context.Background(), 1*time.Millisecond)
		require.NoError(t, err)
	})

	t.Run("cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := SleepFunc(ctx, 1*time.Second)
		require.ErrorIs(t, err, context.Canceled)
	})
}
