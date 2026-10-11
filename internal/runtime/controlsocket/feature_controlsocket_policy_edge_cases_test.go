package controlsocket

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/container"
)

func TestUnit_ValidateInheritedCeiling_PolicyEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("capability normalization with whitespace and prefixes", func(t *testing.T) {
		parent := &container.ContainerConfig{
			CapAdd:  []string{" cap_net_admin ", "CAP_SYS_ADMIN"},
			CapDrop: []string{"SYS_CHROOT"},
		}

		childOK := &container.ContainerConfig{
			CapAdd:  []string{"NET_ADMIN", "sys_admin"},
			CapDrop: []string{"SYS_CHROOT"},
		}
		require.NoError(t, ValidateInheritedCeiling(childOK, parent))

		childBadAdd := &container.ContainerConfig{
			CapAdd: []string{"CAP_SYS_PTRACE"},
		}
		errAdd := ValidateInheritedCeiling(childBadAdd, parent)
		require.Error(t, errAdd)
		assert.Contains(t, errAdd.Error(), "capability \"CAP_SYS_PTRACE\" not permitted")

		childMissingDrop := &container.ContainerConfig{}
		errDrop := ValidateInheritedCeiling(childMissingDrop, parent)
		require.Error(t, errDrop)
		assert.Contains(t, errDrop.Error(), "must drop capability \"SYS_CHROOT\"")
	})

	t.Run("device cgroup permissions default empty handling", func(t *testing.T) {
		devPath := filepath.Join(t.TempDir(), "dev_null")
		parentDefaultPerms := &container.ContainerConfig{
			Devices: []container.DeviceMapping{
				{PathOnHost: devPath, CgroupPermissions: ""}, // defaults to "rwm"
			},
		}

		childRO := &container.ContainerConfig{
			Devices: []container.DeviceMapping{
				{PathOnHost: devPath, CgroupPermissions: "r"},
			},
		}
		require.NoError(t, ValidateInheritedCeiling(childRO, parentDefaultPerms))

		parentRestrictedPerms := &container.ContainerConfig{
			Devices: []container.DeviceMapping{
				{PathOnHost: devPath, CgroupPermissions: "r"},
			},
		}

		childDefaultPerms := &container.ContainerConfig{
			Devices: []container.DeviceMapping{
				{PathOnHost: devPath, CgroupPermissions: ""}, // default "rwm" exceeds "r"
			},
		}
		errDefault := ValidateInheritedCeiling(childDefaultPerms, parentRestrictedPerms)
		require.Error(t, errDefault)
		assert.Contains(t, errDefault.Error(), "exceeding parent permissions")
	})

	t.Run("sysctl missing key vs value mismatch", func(t *testing.T) {
		parent := &container.ContainerConfig{
			Sysctls: map[string]string{
				"net.ipv4.ip_forward": "1",
			},
		}

		childMissingKey := &container.ContainerConfig{
			Sysctls: map[string]string{
				"kernel.shmmax": "18446744073709551615",
			},
		}
		errMissing := ValidateInheritedCeiling(childMissingKey, parent)
		require.Error(t, errMissing)
		assert.Contains(t, errMissing.Error(), "sysctl kernel.shmmax=\"18446744073709551615\" not permitted")

		childValueMismatch := &container.ContainerConfig{
			Sysctls: map[string]string{
				"net.ipv4.ip_forward": "0",
			},
		}
		errMismatch := ValidateInheritedCeiling(childValueMismatch, parent)
		require.Error(t, errMismatch)
		assert.Contains(t, errMismatch.Error(), "sysctl net.ipv4.ip_forward=\"0\" not permitted")
	})

	t.Run("non-bind mount types bypass source containment check", func(t *testing.T) {
		parent := &container.ContainerConfig{
			Mounts: []container.Mount{
				{Type: "bind", Source: t.TempDir(), Target: "/app", ReadOnly: false},
			},
		}

		childTmpfs := &container.ContainerConfig{
			Mounts: []container.Mount{
				{Type: "tmpfs", Target: "/tmp", ReadOnly: false},
				{Type: "volume", Target: "/data", ReadOnly: false},
			},
		}
		require.NoError(t, ValidateInheritedCeiling(childTmpfs, parent))
	})

	t.Run("security options hardening formats", func(t *testing.T) {
		parent := &container.ContainerConfig{}

		childHardening1 := &container.ContainerConfig{
			SecurityOpt: []string{"no-new-privileges"},
		}
		require.NoError(t, ValidateInheritedCeiling(childHardening1, parent))

		childHardening2 := &container.ContainerConfig{
			SecurityOpt: []string{"NO-NEW-PRIVILEGES:TRUE"},
		}
		require.NoError(t, ValidateInheritedCeiling(childHardening2, parent))

		childHardening3 := &container.ContainerConfig{
			SecurityOpt: []string{"no-new-privileges=true"},
		}
		require.NoError(t, ValidateInheritedCeiling(childHardening3, parent))

		childDisallowed := &container.ContainerConfig{
			SecurityOpt: []string{"label:type:container_runtime_t"},
		}
		errDisallowed := ValidateInheritedCeiling(childDisallowed, parent)
		require.Error(t, errDisallowed)
		assert.Contains(t, errDisallowed.Error(), "security option \"label:type:container_runtime_t\" not permitted")
	})
}
