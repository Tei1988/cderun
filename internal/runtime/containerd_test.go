package runtime

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/containerd/errdefs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"cderun/internal/container"
)

func TestUnit_Containerd_Name(t *testing.T) {
	rt := &ContainerdRuntime{}
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
		{"invalid", true},
		{"0", true},
		{"65", true},
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
	rt := &ContainerdRuntime{}

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
