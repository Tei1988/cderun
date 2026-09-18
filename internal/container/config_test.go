package container

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestUnit_Container_ConfigInitialization(t *testing.T) {
	config := ContainerConfig{
		Image:       "alpine:latest",
		Command:     []string{"sh", "-c", "echo hello"},
		TTY:         true,
		Interactive: true,
		Remove:      true,
		Network:     "bridge",
		Mounts: []Mount{
			{
				Type:     "bind",
				Source:   "/tmp",
				Target:   "/data",
				ReadOnly: false,
			},
		},
		Env:     []string{"FOO=BAR"},
		Workdir: "/workspace",
		User:    "1000",
	}

	assert.Equal(t, "alpine:latest", config.Image)
	assert.Equal(t, []string{"sh", "-c", "echo hello"}, config.Command)
	assert.True(t, config.TTY)
	assert.True(t, config.Interactive)
	assert.True(t, config.Remove)
	assert.Equal(t, "bridge", config.Network)
	assert.Len(t, config.Mounts, 1)
	assert.Equal(t, "bind", config.Mounts[0].Type)
	assert.Equal(t, "/tmp", config.Mounts[0].Source)
	assert.Equal(t, "/data", config.Mounts[0].Target)
	assert.False(t, config.Mounts[0].ReadOnly)
	assert.Equal(t, []string{"FOO=BAR"}, config.Env)
	assert.Equal(t, "/workspace", config.Workdir)
	assert.Equal(t, "1000", config.User)
}

func TestUnit_Container_Mount(t *testing.T) {
	mount := Mount{
		Type:     "bind",
		Source:   "/etc/hosts",
		Target:   "/etc/hosts",
		ReadOnly: true,
		Optional: true,
	}

	assert.Equal(t, "bind", mount.Type)
	assert.Equal(t, "/etc/hosts", mount.Source)
	assert.Equal(t, "/etc/hosts", mount.Target)
	assert.True(t, mount.ReadOnly)
	assert.True(t, mount.Optional)
}

func TestUnit_ContainerConfig_JSONSerialization(t *testing.T) {
	cfg := ContainerConfig{
		Image:       "ubuntu:22.04",
		Command:     []string{"bash", "-c", "echo hello world"},
		TTY:         true,
		Interactive: false,
		Remove:      true,
		ReadOnly:    true,
		Init:        true,
		Network:     "custom_net",
		Ports:       []string{"8080:80"},
		PublishAll:  true,
		Expose:      []string{"80/tcp"},
		Hostname:    "test-host",
		DNS:         []string{"8.8.8.8"},
		AddHosts:    []string{"custom.host:127.0.0.1"},
		Mounts: []Mount{
			{Type: "bind", Source: "/host/path", Target: "/container/path", ReadOnly: true, Optional: false},
			{Type: "tmpfs", Target: "/tmp", ReadOnly: false, Optional: true},
		},
		Labels:     map[string]string{"env": "test", "app": "cderun"},
		Env:        []string{"KEY=VAL", "EMPTY="},
		Workdir:    "/app",
		User:       "1000:1000",
		Privileged: true,
		Pid:        "host",
		ShmSize:    "64m",
		CapAdd:     []string{"SYS_PTRACE"},
		CapDrop:    []string{"NET_RAW"},
		Entrypoint: []string{"/entrypoint.sh"},
		Pull:       "always",
		Memory:     1024 * 1024 * 512,
		CPUs:       1.5,
		Devices: []DeviceMapping{
			{PathOnHost: "/dev/null", PathInContainer: "/dev/null", CgroupPermissions: "rwm"},
		},
		GroupAdd: []string{"1001"},
		Ulimits: []Ulimit{
			{Name: "nofile", Soft: 1024, Hard: 2048},
		},
		Sysctls:     map[string]string{"net.ipv4.ip_forward": "1"},
		IPC:         "shareable",
		SecurityOpt: []string{"no-new-privileges:true"},
		DNSSearch:   []string{"example.com"},
		DNSOptions:  []string{"ndots:2"},
		GPUs:        "all",
		Cgroupns:    "private",
		PidsLimit:   100,
		CPUShares:   512,
		CpusetCpus:  "0-3",
		CpusetMems:  "0",
		Restart:     "unless-stopped",
		OciRuntime:  "runc",
	}

	data, err := json.Marshal(cfg)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	var unmarshaled ContainerConfig
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)
	assert.Equal(t, cfg, unmarshaled)
}

func TestUnit_ContainerConfig_YAMLSerialization(t *testing.T) {
	cfg := ContainerConfig{
		Image:   "redis:alpine",
		Command: []string{"redis-server"},
		Mounts: []Mount{
			{Type: "volume", Source: "redis-data", Target: "/data"},
		},
		Env:     []string{"REDIS_PASSWORD=secret"},
		Workdir: "/data",
		Labels:  map[string]string{"role": "db"},
	}

	data, err := yaml.Marshal(cfg)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	var unmarshaled ContainerConfig
	err = yaml.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)
	assert.Equal(t, cfg, unmarshaled)
}

func TestUnit_ContainerConfig_OmitemptyFields(t *testing.T) {
	cfg := ContainerConfig{
		Image:   "alpine",
		Command: []string{"true"},
	}

	data, err := json.Marshal(cfg)
	require.NoError(t, err)

	jsonStr := string(data)
	assert.NotContains(t, jsonStr, "read_only")
	assert.NotContains(t, jsonStr, "init")
	assert.NotContains(t, jsonStr, "ports")
	assert.NotContains(t, jsonStr, "publish_all")
	assert.NotContains(t, jsonStr, "shm_size")
	assert.NotContains(t, jsonStr, "gpus")
	assert.NotContains(t, jsonStr, "oci_runtime")
}

func TestUnit_ContainerConfig_DeviceMapping(t *testing.T) {
	dev := DeviceMapping{
		PathOnHost:        "/dev/snd",
		PathInContainer:   "/dev/snd",
		CgroupPermissions: "rwm",
	}

	data, err := json.Marshal(dev)
	require.NoError(t, err)

	var unmarshaled DeviceMapping
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, dev, unmarshaled)
}

func TestUnit_ContainerConfig_Ulimit(t *testing.T) {
	u := Ulimit{
		Name: "memlock",
		Soft: -1,
		Hard: -1,
	}

	data, err := json.Marshal(u)
	require.NoError(t, err)

	var unmarshaled Ulimit
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, u, unmarshaled)
}
