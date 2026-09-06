//go:build linux

package runtime

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConformance_ContainerdRuntime(t *testing.T) {
	factory := func(t *testing.T) ContainerRuntime {
		if os.Getenv("CDERUN_RUNTIME") == "containerd" {
			socket := os.Getenv("CDERUN_SOCKET_PATH")
			if socket == "" {
				socket = "/run/containerd/containerd.sock"
			}
			_, err := os.Stat(socket)
			require.NoError(t, err, "containerd runtime socket should exist when CDERUN_RUNTIME=containerd")
			rt, err := NewContainerdRuntime(socket)
			require.NoError(t, err, "failed to create live containerd runtime")
			return rt
		}

		mc := newMockFullClient()
		rt, err := NewContainerdRuntime("/dummy/socket.sock", WithContainerdClient(mc))
		require.NoError(t, err)
		return rt
	}
	caps := ConformanceCapabilities{
		SupportsVolumes:    false,
		SupportsTmpfs:      true,
		SupportsPorts:      false,
		SupportsGPUs:       false,
		SupportsDNSSearch:  false,
		SupportsDNSOptions: false,
		SupportsInit:       false,
		RequiresCapPrefix:  true,
	}
	RunConformanceTests(t, factory, caps)
}
