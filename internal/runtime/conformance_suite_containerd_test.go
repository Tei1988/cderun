//go:build linux

package runtime

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConformance_ContainerdRuntime_MockClient(t *testing.T) {
	mc := newMockFullClient()
	factory := func(t *testing.T) ContainerRuntime {
		rt, err := NewContainerdRuntime("/dummy/socket.sock", WithContainerdClient(mc))
		require.NoError(t, err)
		return rt
	}
	caps := ConformanceCapabilities{
		SupportsVolumes:    false,
		SupportsPorts:      false,
		SupportsGPUs:       false,
		SupportsDNSSearch:  false,
		SupportsDNSOptions: false,
		RequiresCapPrefix:  true,
	}
	RunConformanceTests(t, factory, caps)
}
