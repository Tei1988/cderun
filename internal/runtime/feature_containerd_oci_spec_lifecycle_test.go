//go:build linux

package runtime

import (
	"context"
	"testing"

	"cderun/internal/container"
	"cderun/internal/logging"

	"github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/containerd/errdefs"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func applySpecOptsToSpec(ctx context.Context, opts ...oci.SpecOpts) (*specs.Spec, error) {
	s := &specs.Spec{
		Linux: &specs.Linux{
			Resources: &specs.LinuxResources{},
		},
	}
	for _, opt := range opts {
		if err := opt(ctx, nil, nil, s); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func TestUnit_Containerd_BuildContainerdNamespaceSpecOpts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("host namespaces", func(t *testing.T) {
		t.Parallel()
		cfg := &container.ContainerConfig{
			Network:  "host",
			Pid:      "host",
			IPC:      "host",
			Cgroupns: "host",
		}
		opts := buildContainerdNamespaceSpecOpts(cfg)
		require.Len(t, opts, 4)

		s, err := applySpecOptsToSpec(ctx, opts...)
		require.NoError(t, err)
		require.NotNil(t, s.Linux)

		hasNet, hasPID, hasIPC, hasCgroup := false, false, false, false
		for _, ns := range s.Linux.Namespaces {
			switch ns.Type {
			case specs.NetworkNamespace:
				hasNet = true
			case specs.PIDNamespace:
				hasPID = true
			case specs.IPCNamespace:
				hasIPC = true
			case specs.CgroupNamespace:
				hasCgroup = true
			}
		}
		assert.False(t, hasNet, "host network removes network namespace")
		assert.False(t, hasPID, "host pid removes pid namespace")
		assert.False(t, hasIPC, "host ipc removes ipc namespace")
		assert.False(t, hasCgroup, "host cgroupns removes cgroup namespace")
	})

	t.Run("private cgroupns with nil Linux", func(t *testing.T) {
		t.Parallel()
		cfg := &container.ContainerConfig{
			Cgroupns: "private",
		}
		opts := buildContainerdNamespaceSpecOpts(cfg)
		require.Len(t, opts, 1)

		s, err := applySpecOptsToSpec(ctx, opts...)
		require.NoError(t, err)
		require.NotNil(t, s.Linux)
		require.Len(t, s.Linux.Namespaces, 1)
		assert.Equal(t, specs.CgroupNamespace, s.Linux.Namespaces[0].Type)
		assert.Empty(t, s.Linux.Namespaces[0].Path)
	})

	t.Run("private cgroupns with existing cgroupns path", func(t *testing.T) {
		t.Parallel()
		cfg := &container.ContainerConfig{
			Cgroupns: "private",
		}
		opts := buildContainerdNamespaceSpecOpts(cfg)
		require.Len(t, opts, 1)

		s := &specs.Spec{
			Linux: &specs.Linux{
				Namespaces: []specs.LinuxNamespace{
					{Type: specs.PIDNamespace, Path: "/proc/1/ns/pid"},
					{Type: specs.CgroupNamespace, Path: "/proc/1/ns/cgroup"},
				},
			},
		}
		err := opts[0](ctx, nil, nil, s)
		require.NoError(t, err)
		require.Len(t, s.Linux.Namespaces, 2)
		assert.Equal(t, specs.CgroupNamespace, s.Linux.Namespaces[1].Type)
		assert.Empty(t, s.Linux.Namespaces[1].Path, "path should be cleared for private cgroupns")
	})
}

func TestUnit_Containerd_BuildContainerdResourceSpecOpts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("all resource limits set with pidsLimit -1", func(t *testing.T) {
		t.Parallel()
		cfg := &container.ContainerConfig{
			Memory:     512 * 1024 * 1024,
			CPUs:       1.5,
			PidsLimit:  -1,
			CPUShares:  1024,
			CpusetCpus: "0,1",
			CpusetMems: "0",
		}

		opts := buildContainerdResourceSpecOpts(cfg, 150000, 100000)
		s, err := applySpecOptsToSpec(ctx, opts...)
		require.NoError(t, err)

		require.NotNil(t, s.Linux)
		require.NotNil(t, s.Linux.Resources)
		require.NotNil(t, s.Linux.Resources.Memory)
		require.NotNil(t, s.Linux.Resources.Memory.Limit)
		assert.Equal(t, int64(512*1024*1024), *s.Linux.Resources.Memory.Limit)

		require.NotNil(t, s.Linux.Resources.CPU)
		require.NotNil(t, s.Linux.Resources.CPU.Quota)
		assert.Equal(t, int64(150000), *s.Linux.Resources.CPU.Quota)
		require.NotNil(t, s.Linux.Resources.CPU.Period)
		assert.Equal(t, uint64(100000), *s.Linux.Resources.CPU.Period)

		require.NotNil(t, s.Linux.Resources.Pids)
		require.NotNil(t, s.Linux.Resources.Pids.Limit)
		assert.Equal(t, int64(0), *s.Linux.Resources.Pids.Limit, "-1 should map to 0 in OCI spec")

		require.NotNil(t, s.Linux.Resources.CPU.Shares)
		assert.Equal(t, uint64(1024), *s.Linux.Resources.CPU.Shares)
		assert.Equal(t, "0,1", s.Linux.Resources.CPU.Cpus)
		assert.Equal(t, "0", s.Linux.Resources.CPU.Mems)
	})

	t.Run("positive pidsLimit", func(t *testing.T) {
		t.Parallel()
		cfg := &container.ContainerConfig{
			PidsLimit: 256,
		}

		opts := buildContainerdResourceSpecOpts(cfg, 0, 0)
		s, err := applySpecOptsToSpec(ctx, opts...)
		require.NoError(t, err)

		require.NotNil(t, s.Linux)
		require.NotNil(t, s.Linux.Resources)
		require.NotNil(t, s.Linux.Resources.Pids)
		require.NotNil(t, s.Linux.Resources.Pids.Limit)
		assert.Equal(t, int64(256), *s.Linux.Resources.Pids.Limit)
	})
}

func TestUnit_Containerd_AppendCapabilityHostDeviceSecurityOpts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("capabilities and devices and hostname", func(t *testing.T) {
		t.Parallel()
		cfg := &container.ContainerConfig{
			Privileged: true,
			CapAdd:     []string{"sys_admin", "CAP_NET_ADMIN"},
			CapDrop:    []string{"chown"},
			Hostname:   "my-host",
			ReadOnly:   true,
			Devices: []container.DeviceMapping{
				{PathOnHost: "/dev/null", PathInContainer: "/dev/null", CgroupPermissions: "rwm"},
			},
			Ulimits: []container.Ulimit{
				{Name: "nofile", Soft: 1024, Hard: 2048},
			},
		}

		capOpts := appendContainerdCapabilitySpecOpts(nil, cfg)
		assert.NotEmpty(t, capOpts)

		hostOpts, err := appendContainerdHostAndDeviceSpecOpts(nil, cfg)
		require.NoError(t, err)
		assert.NotEmpty(t, hostOpts)

		s, err := applySpecOptsToSpec(ctx, append(capOpts, hostOpts...)...)
		require.NoError(t, err)

		assert.Equal(t, "my-host", s.Hostname)
		require.NotNil(t, s.Root)
		assert.True(t, s.Root.Readonly)

		require.NotNil(t, s.Process)
		require.NotEmpty(t, s.Process.Rlimits)
		assert.Equal(t, "RLIMIT_NOFILE", s.Process.Rlimits[0].Type)
		assert.Equal(t, uint64(1024), s.Process.Rlimits[0].Soft)
		assert.Equal(t, uint64(2048), s.Process.Rlimits[0].Hard)

		require.NotNil(t, s.Linux)
		require.NotEmpty(t, s.Linux.Devices)
		assert.Equal(t, "/dev/null", s.Linux.Devices[0].Path)
	})

	t.Run("invalid shm-size parsing errors", func(t *testing.T) {
		t.Parallel()
		cfgInvalidUnit := &container.ContainerConfig{ShmSize: "invalid-size"}
		_, err := appendContainerdHostAndDeviceSpecOpts(nil, cfgInvalidUnit)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid shm-size")

		cfgNegative := &container.ContainerConfig{ShmSize: "-100M"}
		_, err = appendContainerdHostAndDeviceSpecOpts(nil, cfgNegative)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid shm-size")
	})

	t.Run("security options and sysctls and groupAdd", func(t *testing.T) {
		t.Parallel()
		cfg := &container.ContainerConfig{
			SecurityOpt: []string{
				"no-new-privileges",
				"seccomp=unconfined",
				"apparmor=docker-default",
				"apparmor:cderun-custom",
			},
			GroupAdd: []string{"1001", "1002"},
			Sysctls: map[string]string{
				"net.ipv4.ip_forward": "1",
			},
		}

		secOpts := appendContainerdSecurityAndGroupSpecOpts(nil, cfg, []uint32{1001, 1002})
		require.Len(t, secOpts, 3)

		s, err := applySpecOptsToSpec(ctx, secOpts...)
		require.NoError(t, err)

		require.NotNil(t, s.Process)
		assert.True(t, s.Process.NoNewPrivileges)
		assert.Equal(t, "cderun-custom", s.Process.ApparmorProfile)
		require.Contains(t, s.Process.User.AdditionalGids, uint32(1001))
		require.Contains(t, s.Process.User.AdditionalGids, uint32(1002))

		require.NotNil(t, s.Linux)
		assert.Nil(t, s.Linux.Seccomp, "seccomp=unconfined should nil out Seccomp")
		require.NotNil(t, s.Linux.Sysctl)
		assert.Equal(t, "1", s.Linux.Sysctl["net.ipv4.ip_forward"])
	})

	t.Run("unsupported security option returns error", func(t *testing.T) {
		t.Parallel()
		cfg := &container.ContainerConfig{
			SecurityOpt: []string{"label=disable"},
		}
		secOpts := appendContainerdSecurityAndGroupSpecOpts(nil, cfg, nil)
		require.Len(t, secOpts, 1)

		s := &specs.Spec{}
		err := secOpts[0](ctx, nil, nil, s)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported security option: \"label=disable\"")
	})
}

func TestUnit_Containerd_GetShmSizeSpecOpt(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("mount /dev/shm does not exist adds tmpfs", func(t *testing.T) {
		t.Parallel()
		opt := getShmSizeSpecOpt(64 * 1024 * 1024)
		s, err := applySpecOptsToSpec(ctx, opt)
		require.NoError(t, err)
		require.Len(t, s.Mounts, 1)
		assert.Equal(t, "/dev/shm", s.Mounts[0].Destination)
		assert.Equal(t, "tmpfs", s.Mounts[0].Type)
		assert.Contains(t, s.Mounts[0].Options, "size=67108864")
	})

	t.Run("existing tmpfs /dev/shm replaces size option", func(t *testing.T) {
		t.Parallel()
		opt := getShmSizeSpecOpt(128 * 1024 * 1024)
		s := &specs.Spec{
			Mounts: []specs.Mount{
				{
					Destination: "/dev/shm",
					Type:        "tmpfs",
					Options:     []string{"mode=1777", "size=33554432", "nosuid"},
				},
			},
		}
		err := opt(ctx, nil, nil, s)
		require.NoError(t, err)
		require.Len(t, s.Mounts, 1)
		assert.Equal(t, []string{"mode=1777", "nosuid", "size=134217728"}, s.Mounts[0].Options)
	})

	t.Run("existing non-tmpfs /dev/shm returns error", func(t *testing.T) {
		t.Parallel()
		opt := getShmSizeSpecOpt(64 * 1024 * 1024)
		s := &specs.Spec{
			Mounts: []specs.Mount{
				{
					Destination: "/dev/shm",
					Type:        "bind",
					Options:     []string{"bind"},
				},
			},
		}
		err := opt(ctx, nil, nil, s)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot set shm-size on a non-tmpfs mount at /dev/shm (found type \"bind\")")
	})
}

func TestUnit_Containerd_InspectAndPrune_MockScenarios(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("PruneContainers with uninitialized client", func(t *testing.T) {
		t.Parallel()
		rt := &ContainerdRuntime{}
		pruned, err := rt.PruneContainers(ctx)
		require.Error(t, err)
		assert.Nil(t, pruned)
		assert.Contains(t, err.Error(), "containerd client is not initialized")
	})

	t.Run("InspectContainer with task not found returns false and 0", func(t *testing.T) {
		t.Parallel()
		mockC := &mockContainer{
			id:   "c1",
			task: nil, // task nil in mockContainer causes Task() to return errdefs.ErrNotFound
		}
		mockCl := &mockContainerdClient{
			loadContainerFunc: func(ctx context.Context, id string) (client.Container, error) {
				return mockC, nil
			},
		}
		rt := &ContainerdRuntime{
			client: mockCl,
			logger: logging.GetGlobalLogger(),
		}

		running, exitCode, err := rt.InspectContainer(ctx, "c1")
		require.NoError(t, err)
		assert.False(t, running)
		assert.Equal(t, 0, exitCode)
	})

	t.Run("InspectContainer with running task returns true", func(t *testing.T) {
		t.Parallel()
		mockT := &mockTask{
			status: client.Status{
				Status:     client.Running,
				ExitStatus: 0,
			},
		}
		mockC := &mockContainer{
			id:   "c2",
			task: mockT,
		}
		mockCl := &mockContainerdClient{
			loadContainerFunc: func(ctx context.Context, id string) (client.Container, error) {
				return mockC, nil
			},
		}
		rt := &ContainerdRuntime{
			client: mockCl,
			logger: logging.GetGlobalLogger(),
		}

		running, exitCode, err := rt.InspectContainer(ctx, "c2")
		require.NoError(t, err)
		assert.True(t, running)
		assert.Equal(t, 0, exitCode)
	})

	t.Run("PruneContainers filters active tasks and prunes exited ones", func(t *testing.T) {
		t.Parallel()
		activeTask := &mockTask{
			status: client.Status{Status: client.Running},
		}
		activeContainer := &mockContainer{
			id:   "active-c",
			task: activeTask,
		}

		pausedTask := &mockTask{
			status: client.Status{Status: client.Paused},
		}
		pausedContainer := &mockContainer{
			id:   "paused-c",
			task: pausedTask,
		}

		exitedTask := &mockTask{
			status: client.Status{Status: client.Stopped},
		}
		exitedContainer := &mockContainer{
			id:   "exited-c",
			task: exitedTask,
		}

		notFoundContainer := &mockContainer{
			id:     "not-found-c",
			delErr: errdefs.ErrNotFound,
		}

		mockCl := &mockContainerdClient{
			containersFunc: func(ctx context.Context, filters ...string) ([]client.Container, error) {
				return []client.Container{activeContainer, pausedContainer, exitedContainer, notFoundContainer}, nil
			},
		}
		rt := &ContainerdRuntime{
			client: mockCl,
			logger: logging.GetGlobalLogger(),
		}

		pruned, err := rt.PruneContainers(ctx)
		require.NoError(t, err)
		assert.Contains(t, pruned, "exited-c")
		assert.NotContains(t, pruned, "active-c")
		assert.NotContains(t, pruned, "paused-c")
	})
}
