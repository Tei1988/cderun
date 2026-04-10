package runtime

import (
	"context"
	"testing"
	"time"

	"cderun/internal/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// NOTE: Containerd client uses many concrete types and is hard to mock deeply without a lot of boilerplate.
// Here we test Name() and basic instantiation/close if possible, or use a mock if needed.
// Since we don't have a containerd daemon in the unit test environment, we focus on
// Name() and ensuring it implements the interface.

func TestUnit_ContainerdRuntime_Name(t *testing.T) {
	t.Parallel()
	rt := &ContainerdRuntime{name: "containerd"}
	assert.Equal(t, "containerd", rt.Name())
}

func TestUnit_ContainerdRuntime_Interface(t *testing.T) {
	t.Parallel()
	var _ ContainerRuntime = (*ContainerdRuntime)(nil)
}

func TestUnit_ContainerdRuntime_ParseSignal(t *testing.T) {
	t.Parallel()
	tests := []struct {
		sig      string
		expected int
		wantErr  bool
	}{
		{"", 15, false},
		{"SIGTERM", 15, false},
		{"TERM", 15, false},
		{"9", 9, false},
		{"SIGKILL", 9, false},
		{"KILL", 9, false},
		{"SIGINT", 2, false},
		{"INT", 2, false},
		{"SIGHUP", 1, false},
		{"HUP", 1, false},
		{"SIGQUIT", 3, false},
		{"QUIT", 3, false},
		{"INVALID", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.sig, func(t *testing.T) {
			got, err := parseSignal(tt.sig)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, int(got))
			}
		})
	}
}

func TestUnit_ContainerdRuntime_PullImage_Never(t *testing.T) {
	t.Parallel()
	rt := &ContainerdRuntime{}
	err := rt.PullImage(context.Background(), "alpine", "never", 3, time.Second)
	assert.NoError(t, err)
}

func TestUnit_ContainerdRuntime_CreateContainer_Validations(t *testing.T) {
	t.Parallel()
	rt := &ContainerdRuntime{}
	ctx := context.Background()

	t.Run("CapDrop not supported", func(t *testing.T) {
		_, err := rt.CreateContainer(ctx, &container.ContainerConfig{CapDrop: []string{"ALL"}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "CapDrop is not supported yet")
	})

	t.Run("Network not supported", func(t *testing.T) {
		_, err := rt.CreateContainer(ctx, &container.ContainerConfig{Network: "custom"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Network \"custom\" is not supported yet")
	})

	t.Run("Hostname not supported", func(t *testing.T) {
		_, err := rt.CreateContainer(ctx, &container.ContainerConfig{Hostname: "host"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Hostname is not supported yet")
	})

	t.Run("DNS not supported", func(t *testing.T) {
		_, err := rt.CreateContainer(ctx, &container.ContainerConfig{DNS: []string{"8.8.8.8"}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "DNS is not supported yet")
	})

	t.Run("AddHosts not supported", func(t *testing.T) {
		_, err := rt.CreateContainer(ctx, &container.ContainerConfig{AddHosts: []string{"h:1.2.3.4"}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "AddHosts is not supported yet")
	})

	t.Run("Port mapping not supported", func(t *testing.T) {
		_, err := rt.CreateContainer(ctx, &container.ContainerConfig{Ports: []string{"80:80"}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Port mapping is not supported yet")
	})
}
