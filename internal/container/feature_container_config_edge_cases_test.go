package container

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestUnit_ContainerConfig_ZeroValueInitialization(t *testing.T) {
	var cfg ContainerConfig

	assert.Empty(t, cfg.Image)
	assert.Nil(t, cfg.Command)
	assert.False(t, cfg.TTY)
	assert.False(t, cfg.Interactive)
	assert.False(t, cfg.Remove)
	assert.False(t, cfg.ReadOnly)
	assert.False(t, cfg.Init)
	assert.Empty(t, cfg.Network)
	assert.Nil(t, cfg.Ports)
	assert.Nil(t, cfg.Expose)
	assert.Nil(t, cfg.Mounts)
	assert.Nil(t, cfg.Labels)
	assert.Nil(t, cfg.Env)
	assert.Empty(t, cfg.Workdir)
	assert.Empty(t, cfg.User)
	assert.Nil(t, cfg.CapAdd)
	assert.Nil(t, cfg.CapDrop)
	assert.Nil(t, cfg.Entrypoint)
	assert.Zero(t, cfg.Memory)
	assert.Zero(t, cfg.CPUs)
	assert.Nil(t, cfg.Devices)
	assert.Nil(t, cfg.GroupAdd)
	assert.Nil(t, cfg.Ulimits)
	assert.Nil(t, cfg.Sysctls)
	assert.Nil(t, cfg.SecurityOpt)
	assert.Nil(t, cfg.DNSSearch)
	assert.Nil(t, cfg.DNSOptions)
	assert.Zero(t, cfg.PidsLimit)
	assert.Zero(t, cfg.CPUShares)
	assert.Empty(t, cfg.CpusetCpus)
	assert.Empty(t, cfg.CpusetMems)
	assert.Empty(t, cfg.Restart)
	assert.Empty(t, cfg.OciRuntime)
}

func TestUnit_ContainerConfig_FullFieldAssignment(t *testing.T) {
	cfg := ContainerConfig{
		Image:       "ubuntu:22.04",
		Command:     []string{"/bin/bash", "-c", "echo test"},
		TTY:         true,
		Interactive: true,
		Remove:      true,
		ReadOnly:    true,
		Init:        true,
		Network:     "custom_net",
		Ports:       []string{"8080:80/tcp", "8443:443/tcp"},
		PublishAll:  true,
		Expose:      []string{"8080/tcp"},
		Hostname:    "my-container",
		DNS:         []string{"1.1.1.1", "8.8.8.8"},
		AddHosts:    []string{"host.docker.internal:host-gateway"},
		Mounts: []Mount{
			{Type: "bind", Source: "/host/path", Target: "/container/path", ReadOnly: true, Optional: false},
			{Type: "tmpfs", Target: "/tmp", ReadOnly: false, Optional: true},
		},
		Labels:      map[string]string{"env": "test", "app": "cderun"},
		Env:         []string{"KEY1=VAL1", "KEY2=VAL2=WITH_EQUALS"},
		Workdir:     "/workspace/app",
		User:        "1000:1000",
		Privileged:  true,
		Pid:         "host",
		ShmSize:     "2g",
		CapAdd:      []string{"SYS_PTRACE", "NET_ADMIN"},
		CapDrop:     []string{"ALL"},
		Entrypoint:  []string{"/entrypoint.sh"},
		Pull:        "always",
		Memory:      1073741824, // 1GB
		CPUs:        2.5,
		Devices: []DeviceMapping{
			{PathOnHost: "/dev/net/tun", PathInContainer: "/dev/net/tun", CgroupPermissions: "rwm"},
		},
		GroupAdd: []string{"1001", "docker"},
		Ulimits: []Ulimit{
			{Name: "nofile", Soft: 1024, Hard: 2048},
			{Name: "nproc", Soft: 65535, Hard: 65535},
		},
		Sysctls: map[string]string{
			"net.ipv4.ip_forward": "1",
		},
		IPC:         "shareable",
		SecurityOpt: []string{"no-new-privileges:true", "seccomp=unconfined"},
		DNSSearch:   []string{"example.com"},
		DNSOptions:  []string{"ndots:2", "timeout:1"},
		GPUs:        "all",
		Cgroupns:    "private",
		PidsLimit:   1000,
		CPUShares:   1024,
		CpusetCpus:  "0-3",
		CpusetMems:  "0",
		Restart:     "unless-stopped",
		OciRuntime:  "runc",
	}

	assert.Equal(t, "ubuntu:22.04", cfg.Image)
	assert.Equal(t, []string{"/bin/bash", "-c", "echo test"}, cfg.Command)
	assert.True(t, cfg.ReadOnly)
	assert.True(t, cfg.Init)
	assert.Equal(t, "custom_net", cfg.Network)
	assert.Len(t, cfg.Ports, 2)
	assert.True(t, cfg.PublishAll)
	assert.Len(t, cfg.Mounts, 2)
	assert.Equal(t, "tmpfs", cfg.Mounts[1].Type)
	assert.True(t, cfg.Mounts[1].Optional)
	assert.Equal(t, "test", cfg.Labels["env"])
	assert.Equal(t, "1000:1000", cfg.User)
	assert.Equal(t, int64(1073741824), cfg.Memory)
	assert.InDelta(t, 2.5, cfg.CPUs, 0.001)
	assert.Equal(t, "shareable", cfg.IPC)
	assert.Equal(t, []string{"no-new-privileges:true", "seccomp=unconfined"}, cfg.SecurityOpt)
	assert.Equal(t, "all", cfg.GPUs)
	assert.Equal(t, "private", cfg.Cgroupns)
	assert.Equal(t, int64(1000), cfg.PidsLimit)
	assert.Equal(t, int64(1024), cfg.CPUShares)
	assert.Equal(t, "0-3", cfg.CpusetCpus)
	assert.Equal(t, "0", cfg.CpusetMems)
	assert.Equal(t, "unless-stopped", cfg.Restart)
	assert.Equal(t, "runc", cfg.OciRuntime)
}

func TestUnit_ContainerConfig_PartialJSONDeserialization(t *testing.T) {
	jsonInput := `{
		"image": "alpine:3.18",
		"command": ["echo", "hello"],
		"pids_limit": 500,
		"oci_runtime": "crun",
		"gpus": "device=0"
	}`

	var cfg ContainerConfig
	err := json.Unmarshal([]byte(jsonInput), &cfg)
	require.NoError(t, err)

	assert.Equal(t, "alpine:3.18", cfg.Image)
	assert.Equal(t, []string{"echo", "hello"}, cfg.Command)
	assert.Equal(t, int64(500), cfg.PidsLimit)
	assert.Equal(t, "crun", cfg.OciRuntime)
	assert.Equal(t, "device=0", cfg.GPUs)
	assert.False(t, cfg.TTY)
	assert.Nil(t, cfg.Env)
}

func TestUnit_ContainerConfig_PartialYAMLDeserialization(t *testing.T) {
	yamlInput := `
image: redis:alpine
network: host
memory: 524288000
sysctls:
  net.core.somaxconn: "511"
security_opt:
  - label:disable
`

	var cfg ContainerConfig
	err := yaml.Unmarshal([]byte(yamlInput), &cfg)
	require.NoError(t, err)

	assert.Equal(t, "redis:alpine", cfg.Image)
	assert.Equal(t, "host", cfg.Network)
	assert.Equal(t, int64(524288000), cfg.Memory)
	assert.Equal(t, "511", cfg.Sysctls["net.core.somaxconn"])
	assert.Equal(t, []string{"label:disable"}, cfg.SecurityOpt)
}

func TestUnit_ContainerConfig_SpecialCharacterHandling(t *testing.T) {
	cfg := ContainerConfig{
		Image:   "my-registry.org/test/image:v1.0.0",
		Command: []string{"sh", "-c", "echo 'hello world' && exit 0"},
		Env:     []string{"SPECIAL_VAR=foo=bar&baz=123", "UNICODE=こんにちは"},
		Workdir: "/path/with spaces/and_underscores",
		Mounts: []Mount{
			{
				Type:   "bind",
				Source: "/host/path with spaces",
				Target: "/container/path with spaces",
			},
		},
	}

	jsonData, err := json.Marshal(cfg)
	require.NoError(t, err)

	var unmarshaled ContainerConfig
	err = json.Unmarshal(jsonData, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, cfg.Command, unmarshaled.Command)
	assert.Equal(t, cfg.Env, unmarshaled.Env)
	assert.Equal(t, cfg.Workdir, unmarshaled.Workdir)
	assert.Equal(t, cfg.Mounts[0].Source, unmarshaled.Mounts[0].Source)
	assert.Equal(t, cfg.Mounts[0].Target, unmarshaled.Mounts[0].Target)
}

func TestUnit_Ulimit_BoundaryValues(t *testing.T) {
	tests := []struct {
		name     string
		ulimit   Ulimit
		expected Ulimit
	}{
		{
			name:     "zero_values",
			ulimit:   Ulimit{Name: "nofile", Soft: 0, Hard: 0},
			expected: Ulimit{Name: "nofile", Soft: 0, Hard: 0},
		},
		{
			name:     "unlimited_negative_one",
			ulimit:   Ulimit{Name: "memlock", Soft: -1, Hard: -1},
			expected: Ulimit{Name: "memlock", Soft: -1, Hard: -1},
		},
		{
			name:     "max_int64",
			ulimit:   Ulimit{Name: "fsize", Soft: math.MaxInt64, Hard: math.MaxInt64},
			expected: Ulimit{Name: "fsize", Soft: math.MaxInt64, Hard: math.MaxInt64},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.ulimit)
			require.NoError(t, err)

			var result Ulimit
			err = json.Unmarshal(data, &result)
			require.NoError(t, err)

			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUnit_DeviceMapping_Fields(t *testing.T) {
	dev := DeviceMapping{
		PathOnHost:        "/dev/kvm",
		PathInContainer:   "/dev/kvm",
		CgroupPermissions: "rwm",
	}

	data, err := yaml.Marshal(dev)
	require.NoError(t, err)

	var restored DeviceMapping
	err = yaml.Unmarshal(data, &restored)
	require.NoError(t, err)

	assert.Equal(t, dev.PathOnHost, restored.PathOnHost)
	assert.Equal(t, dev.PathInContainer, restored.PathInContainer)
	assert.Equal(t, dev.CgroupPermissions, restored.CgroupPermissions)
}
