//go:build linux

package runtime

import (
	"math"
	"testing"

	"cderun/internal/container"

	"github.com/stretchr/testify/require"
)

func TestUnit_Containerd_ValidateConfig_RefactoredHelpers(t *testing.T) {
	t.Parallel()

	rt := &ContainerdRuntime{}

	t.Run("validateContainerdUnsupportedFeatures", func(t *testing.T) {
		t.Parallel()

		err := rt.ValidateConfig(&container.ContainerConfig{Init: true})
		require.Error(t, err)
		require.Contains(t, err.Error(), "init is not supported yet")

		err = rt.ValidateConfig(&container.ContainerConfig{GPUs: "all"})
		require.Error(t, err)
		require.Contains(t, err.Error(), "gpus is not supported yet")

		err = rt.ValidateConfig(&container.ContainerConfig{Restart: "always"})
		require.Error(t, err)
		require.Contains(t, err.Error(), "restart policy is not supported yet")

		err = rt.ValidateConfig(&container.ContainerConfig{DNSSearch: []string{"example.com"}})
		require.Error(t, err)
		require.Contains(t, err.Error(), "dns-search is not supported yet")

		err = rt.ValidateConfig(&container.ContainerConfig{DNSOptions: []string{"ndots:5"}})
		require.Error(t, err)
		require.Contains(t, err.Error(), "dns-option is not supported yet")
	})

	t.Run("validateContainerdNamespacesAndSecurity", func(t *testing.T) {
		t.Parallel()

		err := rt.ValidateConfig(&container.ContainerConfig{Pid: "invalid"})
		require.Error(t, err)
		require.Contains(t, err.Error(), "unsupported PID namespace mode")

		err = rt.ValidateConfig(&container.ContainerConfig{IPC: "invalid"})
		require.Error(t, err)
		require.Contains(t, err.Error(), "unsupported IPC namespace mode")

		err = rt.ValidateConfig(&container.ContainerConfig{Cgroupns: "invalid"})
		require.Error(t, err)
		require.Contains(t, err.Error(), "unsupported cgroup namespace mode")

		err = rt.ValidateConfig(&container.ContainerConfig{SecurityOpt: []string{"apparmor="}})
		require.Error(t, err)
		require.Contains(t, err.Error(), "empty AppArmor profile is not supported")

		err = rt.ValidateConfig(&container.ContainerConfig{SecurityOpt: []string{"unsupported_opt"}})
		require.Error(t, err)
		require.Contains(t, err.Error(), "unsupported security option")
	})

	t.Run("validateContainerdResources", func(t *testing.T) {
		t.Parallel()

		err := rt.ValidateConfig(&container.ContainerConfig{ShmSize: "invalid"})
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid shm-size")

		err = rt.ValidateConfig(&container.ContainerConfig{ShmSize: "9223372036854775807k"})
		require.Error(t, err)
		require.Contains(t, err.Error(), "shm-size cannot be negative")

		err = rt.ValidateConfig(&container.ContainerConfig{CPUs: math.NaN()})
		require.Error(t, err)
		require.Contains(t, err.Error(), "non-finite CPU limit")

		err = rt.ValidateConfig(&container.ContainerConfig{Memory: -100})
		require.Error(t, err)
		require.Contains(t, err.Error(), "negative memory limit")

		err = rt.ValidateConfig(&container.ContainerConfig{CPUs: -1.0})
		require.Error(t, err)
		require.Contains(t, err.Error(), "negative CPU limit")

		err = rt.ValidateConfig(&container.ContainerConfig{CPUs: 0.000000001})
		require.Error(t, err)
		require.Contains(t, err.Error(), "CPU quota")
	})

	t.Run("validateContainerdNetworking", func(t *testing.T) {
		t.Parallel()

		err := rt.ValidateConfig(&container.ContainerConfig{Network: "bridge"})
		require.Error(t, err)
		require.Contains(t, err.Error(), "Network \"bridge\" is not supported yet")

		err = rt.ValidateConfig(&container.ContainerConfig{Ports: []string{"80:80"}})
		require.Error(t, err)
		require.Contains(t, err.Error(), "port mapping is not supported yet")

		err = rt.ValidateConfig(&container.ContainerConfig{DNS: []string{"8.8.8.8"}})
		require.Error(t, err)
		require.Contains(t, err.Error(), "DNS setting is not supported yet")

		err = rt.ValidateConfig(&container.ContainerConfig{AddHosts: []string{"host:127.0.0.1"}})
		require.Error(t, err)
		require.Contains(t, err.Error(), "add-host is not supported yet")
	})

	t.Run("validateContainerdMountsAndGroups", func(t *testing.T) {
		t.Parallel()

		err := rt.ValidateConfig(&container.ContainerConfig{
			Mounts: []container.Mount{
				{Type: "volume", Target: "/data"},
			},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "volume mount type is not supported")

		err = rt.ValidateConfig(&container.ContainerConfig{
			Mounts: []container.Mount{
				{Type: "invalid_type", Target: "/data"},
			},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "unsupported mount type")

		err = rt.ValidateConfig(&container.ContainerConfig{
			GroupAdd: []string{"invalid_gid"},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "non-numeric GroupAdd GID")
	})
}
