//go:build linux

package runtime

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConformance_ContainerdRuntime(t *testing.T) {
	factory := func(t *testing.T) ContainerRuntime {
		socket := os.Getenv("CDERUN_SOCKET_PATH")
		if socket == "" {
			socket = "/run/containerd/containerd.sock"
		}

		if _, err := os.Stat(socket); err == nil && os.Getenv("CDERUN_RUNTIME") == "containerd" {
			rt, err := NewContainerdRuntime(socket)
			if err == nil {
				return rt
			}
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
