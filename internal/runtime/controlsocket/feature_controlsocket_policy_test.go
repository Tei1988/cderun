package controlsocket

import (
	"context"
	"io"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/container"
	"cderun/internal/logging"
)

type mockPolicyDispatcher struct {
	createdID   string
	errToReturn error
}

func (m *mockPolicyDispatcher) CreateContainer(ctx context.Context, config *container.ContainerConfig) (string, error) {
	if m.errToReturn != nil {
		return "", m.errToReturn
	}
	if m.createdID != "" {
		return m.createdID, nil
	}
	return "c-policy-123", nil
}

func (m *mockPolicyDispatcher) StartContainer(ctx context.Context, containerID string) error {
	return nil
}

func (m *mockPolicyDispatcher) WaitContainer(ctx context.Context, containerID string) (int, error) {
	return 0, nil
}

func (m *mockPolicyDispatcher) RemoveContainer(ctx context.Context, containerID string) error {
	return nil
}

func (m *mockPolicyDispatcher) AttachContainer(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error {
	return nil
}

func (m *mockPolicyDispatcher) SignalContainer(ctx context.Context, containerID string, sig string) error {
	return nil
}

func (m *mockPolicyDispatcher) ResizeContainerTTY(ctx context.Context, containerID string, rows, cols uint) error {
	return nil
}

func TestUnit_ValidateInheritedCeiling_DirectRules(t *testing.T) {
	t.Parallel()

	t.Run("nil child config returns error", func(t *testing.T) {
		err := ValidateInheritedCeiling(nil, &container.ContainerConfig{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "child container config is nil")
	})

	t.Run("nil parent config allows all", func(t *testing.T) {
		child := &container.ContainerConfig{
			Privileged: true,
			CapAdd:     []string{"SYS_ADMIN"},
			Pid:        "host",
		}
		err := ValidateInheritedCeiling(child, nil)
		require.NoError(t, err)
	})

	t.Run("privileged parent allows all child options", func(t *testing.T) {
		parent := &container.ContainerConfig{Privileged: true}
		child := &container.ContainerConfig{
			Privileged: true,
			CapAdd:     []string{"SYS_ADMIN", "NET_ADMIN"},
			Pid:        "host",
			Network:    "host",
			IPC:        "host",
			Devices:    []container.DeviceMapping{{PathOnHost: "/dev/kvm", CgroupPermissions: "rwm"}},
		}
		err := ValidateInheritedCeiling(child, parent)
		require.NoError(t, err)
	})

	t.Run("privileged child rejected when parent non-privileged", func(t *testing.T) {
		parent := &container.ContainerConfig{Privileged: false}
		child := &container.ContainerConfig{Privileged: true}
		err := ValidateInheritedCeiling(child, parent)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "child requested privileged mode when parent is non-privileged")
	})

	t.Run("cap add and cap drop validation and normalization", func(t *testing.T) {
		parent := &container.ContainerConfig{
			CapAdd:  []string{"SYS_ADMIN", "CAP_NET_ADMIN"},
			CapDrop: []string{"SYS_PTRACE"},
		}

		// Allowed capabilities with variations in case and prefix, plus preserving parent's dropped cap
		childOK := &container.ContainerConfig{
			CapAdd:  []string{"sys_admin", "NET_ADMIN"},
			CapDrop: []string{"SYS_PTRACE"},
		}
		require.NoError(t, ValidateInheritedCeiling(childOK, parent))

		// Disallowed capability
		childBadCapAdd := &container.ContainerConfig{
			CapAdd:  []string{"SYS_ADMIN", "SYS_RAWIO"},
			CapDrop: []string{"SYS_PTRACE"},
		}
		err := ValidateInheritedCeiling(childBadCapAdd, parent)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "child requested capability \"SYS_RAWIO\" not permitted")

		// Failure to drop capability required to be dropped by parent
		childMissingCapDrop := &container.ContainerConfig{
			CapAdd: []string{"SYS_ADMIN"},
		}
		errDrop := ValidateInheritedCeiling(childMissingCapDrop, parent)
		require.Error(t, errDrop)
		assert.Contains(t, errDrop.Error(), "child must drop capability \"SYS_PTRACE\"")
	})

	t.Run("namespace isolation and runtime validation", func(t *testing.T) {
		parent := &container.ContainerConfig{
			Network:    "bridge",
			Pid:        "container",
			IPC:        "container",
			Cgroupns:   "private",
			OciRuntime: "runc",
		}

		// Disallowed host network
		errNet := ValidateInheritedCeiling(&container.ContainerConfig{Network: "host"}, parent)
		require.Error(t, errNet)
		assert.Contains(t, errNet.Error(), "host network mode")

		// Disallowed container network
		errContainerNet := ValidateInheritedCeiling(&container.ContainerConfig{Network: "container:other"}, parent)
		require.Error(t, errContainerNet)
		assert.Contains(t, errContainerNet.Error(), "container network mode")

		// Disallowed host PID
		errPid := ValidateInheritedCeiling(&container.ContainerConfig{Pid: "host"}, parent)
		require.Error(t, errPid)
		assert.Contains(t, errPid.Error(), "host PID mode")

		// Disallowed host IPC
		errIpc := ValidateInheritedCeiling(&container.ContainerConfig{IPC: "host"}, parent)
		require.Error(t, errIpc)
		assert.Contains(t, errIpc.Error(), "host IPC mode")

		// Disallowed host Cgroupns
		errCgroup := ValidateInheritedCeiling(&container.ContainerConfig{Cgroupns: "host"}, parent)
		require.Error(t, errCgroup)
		assert.Contains(t, errCgroup.Error(), "host cgroupns mode")

		// Mismatched OCI runtime
		errOci := ValidateInheritedCeiling(&container.ContainerConfig{OciRuntime: "crun"}, parent)
		require.Error(t, errOci)
		assert.Contains(t, errOci.Error(), "OCI runtime \"crun\" when parent specifies \"runc\"")
	})

	t.Run("sysctl validation", func(t *testing.T) {
		parent := &container.ContainerConfig{
			Sysctls: map[string]string{"net.ipv4.ip_forward": "1"},
		}

		// Allowed matching sysctl
		childOK := &container.ContainerConfig{
			Sysctls: map[string]string{"net.ipv4.ip_forward": "1"},
		}
		require.NoError(t, ValidateInheritedCeiling(childOK, parent))

		// Disallowed sysctl
		childBad := &container.ContainerConfig{
			Sysctls: map[string]string{"net.ipv4.ip_forward": "0"},
		}
		errBad := ValidateInheritedCeiling(childBad, parent)
		require.Error(t, errBad)
		assert.Contains(t, errBad.Error(), "sysctl net.ipv4.ip_forward=\"0\" not permitted")
	})

	t.Run("device mapping validation", func(t *testing.T) {
		devPath := filepath.Join(t.TempDir(), "dev_test")
		parent := &container.ContainerConfig{
			Devices: []container.DeviceMapping{
				{PathOnHost: devPath, CgroupPermissions: "r"},
			},
		}

		// Allowed device with same permissions
		childOK := &container.ContainerConfig{
			Devices: []container.DeviceMapping{
				{PathOnHost: devPath, CgroupPermissions: "r"},
			},
		}
		require.NoError(t, ValidateInheritedCeiling(childOK, parent))

		// Disallowed device
		childBadDev := &container.ContainerConfig{
			Devices: []container.DeviceMapping{
				{PathOnHost: "/dev/random", CgroupPermissions: "r"},
			},
		}
		errDev := ValidateInheritedCeiling(childBadDev, parent)
		require.Error(t, errDev)
		assert.Contains(t, errDev.Error(), "host device \"/dev/random\" not permitted")

		// Exceeding cgroup permissions (parent "r", child "rw")
		childExceedPerms := &container.ContainerConfig{
			Devices: []container.DeviceMapping{
				{PathOnHost: devPath, CgroupPermissions: "rw"},
			},
		}
		errPerm := ValidateInheritedCeiling(childExceedPerms, parent)
		require.Error(t, errPerm)
		assert.Contains(t, errPerm.Error(), "exceeding parent permissions")
	})

	t.Run("read only mount boundaries traversal and parent source containment", func(t *testing.T) {
		tmp := t.TempDir()
		parent := &container.ContainerConfig{
			Mounts: []container.Mount{
				{Type: "bind", Source: tmp, Target: "/parent/ro", ReadOnly: true},
			},
		}

		// Traversal check
		assert.True(t, hasPathTraversal("/foo/../bar"))
		assert.False(t, hasPathTraversal("/foo/..data/bar"))

		// Sibling path like "..data" or "data" is properly handled by isSubpathOrEqual
		assert.True(t, isSubpathOrEqual(filepath.Join(tmp, "sub"), tmp))
		assert.False(t, isSubpathOrEqual("/other/path", tmp))

		// Child read-only mount inside parent RO path is allowed
		childRO := &container.ContainerConfig{
			Mounts: []container.Mount{
				{Type: "bind", Source: filepath.Join(tmp, "sub"), Target: "/child/target", ReadOnly: true},
			},
		}
		require.NoError(t, ValidateInheritedCeiling(childRO, parent))

		// Child read-write mount inside parent RO path is rejected
		childRW := &container.ContainerConfig{
			Mounts: []container.Mount{
				{Type: "bind", Source: filepath.Join(tmp, "sub"), Target: "/child/target", ReadOnly: false},
			},
		}
		errRW := ValidateInheritedCeiling(childRW, parent)
		require.Error(t, errRW)
		assert.Contains(t, errRW.Error(), "within parent read-only path")

		// Child read-write mount with path traversal is rejected
		childTraversal := &container.ContainerConfig{
			Mounts: []container.Mount{
				{Type: "bind", Source: "/tmp/foo/../bar", Target: "/target", ReadOnly: false},
			},
		}
		errTraversal := ValidateInheritedCeiling(childTraversal, parent)
		require.Error(t, errTraversal)
		assert.Contains(t, errTraversal.Error(), "path traversal components")

		// Parent with read-write mount allows child read-write mount inside parent mount source
		rwTmp := t.TempDir()
		parentRW := &container.ContainerConfig{
			Mounts: []container.Mount{
				{Type: "bind", Source: rwTmp, Target: "/parent/rw", ReadOnly: false},
			},
		}
		childInsideRW := &container.ContainerConfig{
			Mounts: []container.Mount{
				{Type: "bind", Source: filepath.Join(rwTmp, "sub"), Target: "/child/target", ReadOnly: false},
			},
		}
		require.NoError(t, ValidateInheritedCeiling(childInsideRW, parentRW))

		// Parent with read-write mount rejects child read-write mount outside parent mount sources
		childOutsideRW := &container.ContainerConfig{
			Mounts: []container.Mount{
				{Type: "bind", Source: t.TempDir(), Target: "/child/target", ReadOnly: false},
			},
		}
		errOutside := ValidateInheritedCeiling(childOutsideRW, parentRW)
		require.Error(t, errOutside)
		assert.Contains(t, errOutside.Error(), "outside parent mount sources")

		// Parent root read-only rejects child read-write mount
		parentRootRO := &container.ContainerConfig{ReadOnly: true}
		errRootRO := ValidateInheritedCeiling(childRW, parentRootRO)
		require.Error(t, errRootRO)
		assert.Contains(t, errRootRO.Error(), "when parent root is read-only")
	})

	t.Run("security options validation", func(t *testing.T) {
		parent := &container.ContainerConfig{
			SecurityOpt: []string{"seccomp=custom.json"},
		}

		// Hardening opt always allowed
		childHardening := &container.ContainerConfig{
			SecurityOpt: []string{"no-new-privileges:true"},
		}
		require.NoError(t, ValidateInheritedCeiling(childHardening, parent))

		// Disallowed unconfined security opt
		childBad := &container.ContainerConfig{
			SecurityOpt: []string{"apparmor=unconfined"},
		}
		errBad := ValidateInheritedCeiling(childBad, parent)
		require.Error(t, errBad)
		assert.Contains(t, errBad.Error(), "security option \"apparmor=unconfined\" not permitted")
	})
}

func TestUnit_ControlSocket_Server_ParentConfigPolicyEnforcement(t *testing.T) {
	t.Parallel()

	socketPath := filepath.Join(t.TempDir(), "ctrl_policy.sock")
	logger := logging.GetGlobalLogger()

	server := NewServer(socketPath, logger)
	disp := &mockPolicyDispatcher{}
	server.SetDispatcher(disp)

	parentCfg := &container.ContainerConfig{
		Privileged: false,
		CapAdd:     []string{"NET_BIND_SERVICE"},
		Network:    "bridge",
	}
	server.SetParentConfig(parentCfg)

	err := server.Start()
	require.NoError(t, err)
	defer func() {
		_ = server.Close()
	}()

	client, err := Connect(context.Background(), socketPath)
	require.NoError(t, err)
	defer client.Close()

	// 1. Allowed CreateContainer RPC
	allowedCfg := &container.ContainerConfig{
		Image:   "alpine:latest",
		Command: []string{"echo", "ok"},
		CapAdd:  []string{"NET_BIND_SERVICE"},
	}
	cid, err := client.CreateContainer(context.Background(), allowedCfg)
	require.NoError(t, err)
	assert.Equal(t, "c-policy-123", cid)

	// 2. Escalating CreateContainer RPC (Privileged) rejected by policy
	escalatingCfg := &container.ContainerConfig{
		Image:      "alpine:latest",
		Command:    []string{"sh"},
		Privileged: true,
	}
	_, errEsc := client.CreateContainer(context.Background(), escalatingCfg)
	require.Error(t, errEsc)
	assert.Contains(t, errEsc.Error(), "child requested privileged mode when parent is non-privileged")

	// 3. Escalating CreateContainer RPC (Unallowed capability) rejected by policy
	escalatingCapCfg := &container.ContainerConfig{
		Image:  "alpine:latest",
		CapAdd: []string{"SYS_ADMIN"},
	}
	_, errCap := client.CreateContainer(context.Background(), escalatingCapCfg)
	require.Error(t, errCap)
	assert.Contains(t, errCap.Error(), "capability \"SYS_ADMIN\" not permitted")
}
