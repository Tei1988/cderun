package runtime

import (
	"testing"

	"cderun/internal/container"

	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_DockerAdapter_ToDockerContainerConfig_NilHandling(t *testing.T) {
	containerCfg, hostCfg, netCfg, err := toDockerContainerConfig(nil)
	require.Error(t, err)
	assert.Nil(t, containerCfg)
	assert.Nil(t, hostCfg)
	assert.Nil(t, netCfg)
	assert.Contains(t, err.Error(), "nil container config")
}

func TestUnit_DockerAdapter_BuildDockerContainerConfig_Mappings(t *testing.T) {
	t.Run("default label injection and field propagation", func(t *testing.T) {
		cfg := &container.ContainerConfig{
			Image:       "alpine:latest",
			Command:     []string{"echo", "hello"},
			TTY:         true,
			Interactive: true,
			Env:         []string{"KEY=VAL", "FOO=BAR"},
			Workdir:     "/workspace",
			User:        "1000:1000",
			Hostname:    "test-host",
			Entrypoint:  []string{"/bin/sh", "-c"},
			Labels: map[string]string{
				"custom.label": "my-value",
			},
			Expose: []string{"8080", "9000/udp", "3000/tcp"},
		}

		containerCfg, err := buildDockerContainerConfig(cfg)
		require.NoError(t, err)
		require.NotNil(t, containerCfg)

		assert.Equal(t, "alpine:latest", containerCfg.Image)
		assert.Equal(t, []string{"echo", "hello"}, []string(containerCfg.Cmd))
		assert.True(t, containerCfg.Tty)
		assert.True(t, containerCfg.StdinOnce)
		assert.True(t, containerCfg.OpenStdin)
		assert.Equal(t, []string{"KEY=VAL", "FOO=BAR"}, containerCfg.Env)
		assert.Equal(t, "/workspace", containerCfg.WorkingDir)
		assert.Equal(t, "1000:1000", containerCfg.User)
		assert.Equal(t, "test-host", containerCfg.Hostname)
		assert.Equal(t, []string{"/bin/sh", "-c"}, []string(containerCfg.Entrypoint))

		// Default cderun label and custom label
		assert.Equal(t, "true", containerCfg.Labels["cderun"])
		assert.Equal(t, "my-value", containerCfg.Labels["custom.label"])

		// Exposed ports
		expectedPort1 := nat.Port("8080/tcp")
		expectedPort2 := nat.Port("9000/udp")
		expectedPort3 := nat.Port("3000/tcp")
		_, hasPort1 := containerCfg.ExposedPorts[expectedPort1]
		_, hasPort2 := containerCfg.ExposedPorts[expectedPort2]
		_, hasPort3 := containerCfg.ExposedPorts[expectedPort3]
		assert.True(t, hasPort1)
		assert.True(t, hasPort2)
		assert.True(t, hasPort3)
	})

	t.Run("explicit cderun label preservation", func(t *testing.T) {
		cfg := &container.ContainerConfig{
			Image: "alpine:latest",
			Labels: map[string]string{
				"cderun": "custom-override",
			},
		}

		containerCfg, err := buildDockerContainerConfig(cfg)
		require.NoError(t, err)
		assert.Equal(t, "custom-override", containerCfg.Labels["cderun"])
	})

	t.Run("invalid exposed port format", func(t *testing.T) {
		cfg := &container.ContainerConfig{
			Image:  "alpine:latest",
			Expose: []string{"invalid-port/tcp"},
		}

		containerCfg, err := buildDockerContainerConfig(cfg)
		require.Error(t, err)
		assert.Nil(t, containerCfg)
		assert.Contains(t, err.Error(), "invalid expose port")
	})
}

func TestUnit_DockerAdapter_BuildDockerHostConfig_Mappings(t *testing.T) {
	t.Run("base host configuration and resource limits", func(t *testing.T) {
		cfg := &container.ContainerConfig{
			Image:           "alpine:latest",
			Remove:          true,
			Network:         "bridge",
			Privileged:      true,
			Pid:             "host",
			IPC:             "host",
			SecurityOpt:     []string{"seccomp=unconfined"},
			DNSSearch:       []string{"example.com"},
			DNSOptions:      []string{"ndots:2"},
			Cgroupns:        "private",
			CapAdd:          []string{"SYS_ADMIN"},
			CapDrop:         []string{"NET_RAW"},
			DNS:             []string{"8.8.8.8"},
			AddHosts:        []string{"host.internal:127.0.0.1"},
			PublishAll:      true,
			GroupAdd:        []string{"1001"},
			ReadOnly:        true,
			Init:            true,
			Sysctls:         map[string]string{"net.ipv4.ip_forward": "1"},
			OciRuntime:      "runc",
			Memory:          1024 * 1024 * 512,
			CPUs:            1.5,
			CPUShares:       512,
			CpusetCpus:      "0,1",
			CpusetMems:      "0",
			PidsLimit:       100,
			Restart:         "on-failure:3",
			GPUs:            "all",
			ShmSize:         "64m",
			Ulimits: []container.Ulimit{
				{Name: "nofile", Soft: 1024, Hard: 2048},
			},
			Ports: []string{"8080:80/tcp"},
			Devices: []container.DeviceMapping{
				{PathOnHost: "/dev/fuse", PathInContainer: "/dev/fuse", CgroupPermissions: "rwm"},
			},
			Mounts: []container.Mount{
				{Type: "bind", Source: "/host/dir", Target: "/container/dir", ReadOnly: true},
				{Type: "volume", Source: "my-vol", Target: "/container/vol", ReadOnly: false},
				{Type: "tmpfs", Source: "tmpfs", Target: "/tmp", ReadOnly: false},
			},
		}

		containerCfg, err := buildDockerContainerConfig(cfg)
		require.NoError(t, err)

		hostCfg, err := buildDockerHostConfig(cfg, containerCfg)
		require.NoError(t, err)
		require.NotNil(t, hostCfg)

		// Base execution & isolation
		assert.True(t, hostCfg.AutoRemove)
		assert.Equal(t, dockercontainer.NetworkMode("bridge"), hostCfg.NetworkMode)
		assert.True(t, hostCfg.Privileged)
		assert.Equal(t, dockercontainer.PidMode("host"), hostCfg.PidMode)
		assert.Equal(t, dockercontainer.IpcMode("host"), hostCfg.IpcMode)
		assert.Equal(t, []string{"seccomp=unconfined"}, hostCfg.SecurityOpt)
		assert.Equal(t, []string{"example.com"}, hostCfg.DNSSearch)
		assert.Equal(t, []string{"ndots:2"}, hostCfg.DNSOptions)
		assert.Equal(t, dockercontainer.CgroupnsMode("private"), hostCfg.CgroupnsMode)
		assert.Equal(t, []string{"SYS_ADMIN"}, []string(hostCfg.CapAdd))
		assert.Equal(t, []string{"NET_RAW"}, []string(hostCfg.CapDrop))
		assert.Equal(t, []string{"8.8.8.8"}, hostCfg.DNS)
		assert.Equal(t, []string{"host.internal:127.0.0.1"}, hostCfg.ExtraHosts)
		assert.True(t, hostCfg.PublishAllPorts)
		assert.Equal(t, []string{"1001"}, hostCfg.GroupAdd)
		assert.True(t, hostCfg.ReadonlyRootfs)
		require.NotNil(t, hostCfg.Init)
		assert.True(t, *hostCfg.Init)
		assert.Equal(t, "1", hostCfg.Sysctls["net.ipv4.ip_forward"])
		assert.Equal(t, "runc", hostCfg.Runtime)

		// Resources
		assert.Equal(t, int64(1024*1024*512), hostCfg.Resources.Memory)
		assert.Equal(t, int64(1.5*1e9), hostCfg.Resources.NanoCPUs)
		assert.Equal(t, int64(512), hostCfg.Resources.CPUShares)
		assert.Equal(t, "0,1", hostCfg.Resources.CpusetCpus)
		assert.Equal(t, "0", hostCfg.Resources.CpusetMems)

		// PidsLimit
		require.NotNil(t, hostCfg.PidsLimit)
		assert.Equal(t, int64(100), *hostCfg.PidsLimit)

		// Restart policy
		assert.Equal(t, dockercontainer.RestartPolicyMode("on-failure"), hostCfg.RestartPolicy.Name)
		assert.Equal(t, 3, hostCfg.RestartPolicy.MaximumRetryCount)

		// GPUs
		require.Len(t, hostCfg.DeviceRequests, 1)
		assert.Equal(t, "nvidia", hostCfg.DeviceRequests[0].Driver)
		assert.Equal(t, -1, hostCfg.DeviceRequests[0].Count)

		// ShmSize
		assert.Equal(t, int64(64*1024*1024), hostCfg.ShmSize)

		// Ulimits
		require.Len(t, hostCfg.Ulimits, 1)
		assert.Equal(t, "nofile", hostCfg.Ulimits[0].Name)
		assert.Equal(t, int64(1024), hostCfg.Ulimits[0].Soft)
		assert.Equal(t, int64(2048), hostCfg.Ulimits[0].Hard)

		// Port bindings
		portKey := nat.Port("80/tcp")
		bindings, ok := hostCfg.PortBindings[portKey]
		require.True(t, ok)
		require.Len(t, bindings, 1)
		assert.Equal(t, "8080", bindings[0].HostPort)

		// Devices
		require.Len(t, hostCfg.Devices, 1)
		assert.Equal(t, "/dev/fuse", hostCfg.Devices[0].PathOnHost)
		assert.Equal(t, "/dev/fuse", hostCfg.Devices[0].PathInContainer)
		assert.Equal(t, "rwm", hostCfg.Devices[0].CgroupPermissions)

		// Mounts
		require.Len(t, hostCfg.Mounts, 3)
		assert.Equal(t, mount.TypeBind, hostCfg.Mounts[0].Type)
		assert.Equal(t, "/host/dir", hostCfg.Mounts[0].Source)
		assert.Equal(t, "/container/dir", hostCfg.Mounts[0].Target)
		assert.True(t, hostCfg.Mounts[0].ReadOnly)

		assert.Equal(t, mount.TypeVolume, hostCfg.Mounts[1].Type)
		assert.Equal(t, "my-vol", hostCfg.Mounts[1].Source)
		assert.Equal(t, "/container/vol", hostCfg.Mounts[1].Target)
		assert.False(t, hostCfg.Mounts[1].ReadOnly)

		assert.Equal(t, mount.TypeTmpfs, hostCfg.Mounts[2].Type)
		assert.Equal(t, "tmpfs", hostCfg.Mounts[2].Source)
		assert.Equal(t, "/tmp", hostCfg.Mounts[2].Target)
		assert.False(t, hostCfg.Mounts[2].ReadOnly)
	})

	t.Run("optional mount rejection", func(t *testing.T) {
		cfg := &container.ContainerConfig{
			Image: "alpine:latest",
			Mounts: []container.Mount{
				{Type: "bind", Source: "/host", Target: "/app", Optional: true},
			},
		}

		containerCfg, err := buildDockerContainerConfig(cfg)
		require.NoError(t, err)

		hostCfg, err := buildDockerHostConfig(cfg, containerCfg)
		require.Error(t, err)
		assert.Nil(t, hostCfg)
		assert.Contains(t, err.Error(), "optional mount is not supported")
	})

	t.Run("invalid mount type error", func(t *testing.T) {
		cfg := &container.ContainerConfig{
			Image: "alpine:latest",
			Mounts: []container.Mount{
				{Type: "invalid-type", Source: "/host", Target: "/app"},
			},
		}

		containerCfg, err := buildDockerContainerConfig(cfg)
		require.NoError(t, err)

		hostCfg, err := buildDockerHostConfig(cfg, containerCfg)
		require.Error(t, err)
		assert.Nil(t, hostCfg)
		assert.Contains(t, err.Error(), "invalid mount type")
	})

	t.Run("invalid port spec error", func(t *testing.T) {
		cfg := &container.ContainerConfig{
			Image: "alpine:latest",
			Ports: []string{"invalid:port:spec:too:many"},
		}

		containerCfg, err := buildDockerContainerConfig(cfg)
		require.NoError(t, err)

		hostCfg, err := buildDockerHostConfig(cfg, containerCfg)
		require.Error(t, err)
		assert.Nil(t, hostCfg)
		assert.Contains(t, err.Error(), "failed to parse port specs")
	})

	t.Run("invalid shm-size error", func(t *testing.T) {
		cfg := &container.ContainerConfig{
			Image:   "alpine:latest",
			ShmSize: "invalid-size-format",
		}

		containerCfg, err := buildDockerContainerConfig(cfg)
		require.NoError(t, err)

		hostCfg, err := buildDockerHostConfig(cfg, containerCfg)
		require.Error(t, err)
		assert.Nil(t, hostCfg)
		assert.Contains(t, err.Error(), "invalid shm-size")
	})
}
