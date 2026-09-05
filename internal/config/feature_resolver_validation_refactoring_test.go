package config

import (
	"testing"

	"cderun/internal/container"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Resolver_ValidateSecurity_RefactoredAuditPaths(t *testing.T) {
	fs := &MockFileSystem{WD: "/workspace"}

	t.Run("audit capability and execution mode warnings", func(t *testing.T) {
		cli := &CLIOptions{
			Image:       ptr("alpine:latest"),
			Privileged:  ptr(true),
			CapAdd:      []string{"SYS_ADMIN", "NET_ADMIN"},
			Network:     ptr("host"),
			Pid:         ptr("host"),
			IPC:         ptr("host"),
			Cgroupns:    ptr("host"),
			MountSocket: ptr(true),
			GroupAdd:    []string{"1000"},
		}

		res, err := ResolveWithFS("node", cli, nil, nil, fs)
		require.NoError(t, err)
		assert.True(t, res.Privileged)
		assert.Equal(t, []string{"SYS_ADMIN", "NET_ADMIN"}, res.CapAdd)
		assert.Equal(t, "host", res.Network)
		assert.Equal(t, "host", res.Pid)
		assert.Equal(t, "host", res.IPC)
		assert.Equal(t, "host", res.Cgroupns)
		assert.True(t, res.MountSocket)
	})

	t.Run("audit mount, device, and security-opt warnings", func(t *testing.T) {
		cli := &CLIOptions{
			Image: ptr("alpine:latest"),
			Mounts: []string{
				"type=bind,source=/etc,target=/container/etc",
				"type=bind,source=/var/run/docker.sock,target=/var/run/docker.sock",
			},
			Devices: []string{
				"/dev/sda:/dev/sda:r",
			},
			SecurityOpt: []string{
				"seccomp=unconfined",
				"apparmor=unconfined",
				"label=disable",
			},
		}

		res, err := ResolveWithFS("node", cli, nil, nil, fs)
		require.NoError(t, err)
		assert.Len(t, res.Mounts, 2)
		assert.Len(t, res.Devices, 1)
		assert.Equal(t, []string{"seccomp=unconfined", "apparmor=unconfined", "label=disable"}, res.SecurityOpt)
	})

	t.Run("audit capability without privileged mode", func(t *testing.T) {
		cli := &CLIOptions{
			Image:  ptr("alpine:latest"),
			CapAdd: []string{"CAP_SYS_ADMIN"},
		}

		res, err := ResolveWithFS("node", cli, nil, nil, fs)
		require.NoError(t, err)
		assert.Equal(t, []string{"CAP_SYS_ADMIN"}, res.CapAdd)
		assert.False(t, res.Privileged)
	})

	t.Run("audit non-sensitive mount and device", func(t *testing.T) {
		cli := &CLIOptions{
			Image: ptr("alpine:latest"),
			Mounts: []string{
				"type=bind,source=/workspace,target=/app",
			},
			Devices: []string{
				"/dev/null:/dev/null:r",
			},
		}

		res, err := ResolveWithFS("node", cli, nil, nil, fs)
		require.NoError(t, err)
		assert.Equal(t, []container.Mount{
			{Type: "bind", Source: "/workspace", Target: "/app"},
		}, res.Mounts)
	})
}
