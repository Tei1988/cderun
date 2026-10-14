package container

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestUnit_ContainerConfig_NilVersusEmptySliceSerialization(t *testing.T) {
	t.Run("nil slices serialize to null in JSON", func(t *testing.T) {
		cfg := ContainerConfig{
			Image:   "alpine:latest",
			Command: nil,
			Env:     nil,
			Mounts:  nil,
		}

		data, err := json.Marshal(cfg)
		require.NoError(t, err)

		var raw map[string]any
		err = json.Unmarshal(data, &raw)
		require.NoError(t, err)

		assert.Nil(t, raw["command"])
		assert.Nil(t, raw["env"])
		assert.Nil(t, raw["mounts"])
	})

	t.Run("empty non-nil slices serialize to empty array in JSON", func(t *testing.T) {
		cfg := ContainerConfig{
			Image:   "alpine:latest",
			Command: []string{},
			Env:     []string{},
			Mounts:  []Mount{},
		}

		data, err := json.Marshal(cfg)
		require.NoError(t, err)

		var raw map[string]any
		err = json.Unmarshal(data, &raw)
		require.NoError(t, err)

		assert.NotNil(t, raw["command"])
		assert.Empty(t, raw["command"])
		assert.NotNil(t, raw["env"])
		assert.Empty(t, raw["env"])
		assert.NotNil(t, raw["mounts"])
		assert.Empty(t, raw["mounts"])
	})
}

func TestUnit_ContainerConfig_MountTypeVariations(t *testing.T) {
	mounts := []Mount{
		{
			Type:     "bind",
			Source:   "/host/data/dir",
			Target:   "/container/data/dir",
			ReadOnly: true,
			Optional: false,
		},
		{
			Type:     "tmpfs",
			Source:   "",
			Target:   "/tmp/cache",
			ReadOnly: false,
			Optional: true,
		},
		{
			Type:     "volume",
			Source:   "my-named-vol",
			Target:   "/var/lib/db",
			ReadOnly: false,
			Optional: false,
		},
	}

	for _, m := range mounts {
		t.Run("mount_type_"+m.Type, func(t *testing.T) {
			data, err := json.Marshal(m)
			require.NoError(t, err)

			var restored Mount
			err = json.Unmarshal(data, &restored)
			require.NoError(t, err)

			assert.Equal(t, m.Type, restored.Type)
			assert.Equal(t, m.Source, restored.Source)
			assert.Equal(t, m.Target, restored.Target)
			assert.Equal(t, m.ReadOnly, restored.ReadOnly)
			assert.Equal(t, m.Optional, restored.Optional)
		})
	}
}

func TestUnit_ContainerConfig_UlimitPermutations(t *testing.T) {
	ulimits := []Ulimit{
		{Name: "nofile", Soft: 1024, Hard: 4096},
		{Name: "nproc", Soft: 2048, Hard: 2048},
		{Name: "memlock", Soft: -1, Hard: -1},
		{Name: "core", Soft: 0, Hard: math.MaxInt64},
	}

	for _, u := range ulimits {
		t.Run("ulimit_"+u.Name, func(t *testing.T) {
			jsonBytes, err := json.Marshal(u)
			require.NoError(t, err)

			var fromJSON Ulimit
			err = json.Unmarshal(jsonBytes, &fromJSON)
			require.NoError(t, err)
			assert.Equal(t, u, fromJSON)

			yamlBytes, err := yaml.Marshal(u)
			require.NoError(t, err)

			var fromYAML Ulimit
			err = yaml.Unmarshal(yamlBytes, &fromYAML)
			require.NoError(t, err)
			assert.Equal(t, u, fromYAML)
		})
	}
}

func TestUnit_ContainerConfig_DeviceMappingPermutations(t *testing.T) {
	devices := []DeviceMapping{
		{PathOnHost: "/dev/kvm", PathInContainer: "/dev/kvm", CgroupPermissions: "rwm"},
		{PathOnHost: "/dev/snd/pcmC0D0p", PathInContainer: "/dev/snd/pcmC0D0p", CgroupPermissions: "rw"},
		{PathOnHost: "/dev/ttyUSB0", PathInContainer: "/dev/ttyUSB0", CgroupPermissions: "r"},
		{PathOnHost: "/dev/dri/card0", PathInContainer: "/dev/dri/card0", CgroupPermissions: ""},
	}

	for _, dev := range devices {
		t.Run("device_"+dev.PathOnHost, func(t *testing.T) {
			jsonBytes, err := json.Marshal(dev)
			require.NoError(t, err)

			var restored DeviceMapping
			err = json.Unmarshal(jsonBytes, &restored)
			require.NoError(t, err)

			assert.Equal(t, dev.PathOnHost, restored.PathOnHost)
			assert.Equal(t, dev.PathInContainer, restored.PathInContainer)
			assert.Equal(t, dev.CgroupPermissions, restored.CgroupPermissions)
		})
	}
}

func TestUnit_ContainerConfig_ComplexExecutionOptions(t *testing.T) {
	cfg := ContainerConfig{
		Image:       "node:20-alpine",
		Command:     []string{"node", "-e", "console.log('hello world')"},
		TTY:         true,
		Interactive: false,
		Remove:      true,
		ReadOnly:    true,
		Init:        true,
		Network:     "bridge",
		Ports:       []string{"3000:3000/tcp", "8080-8085:8080-8085/tcp"},
		PublishAll:  false,
		Expose:      []string{"3000/tcp"},
		Hostname:    "node-app",
		DNS:         []string{"1.1.1.1", "2001:4860:4860::8888"},
		AddHosts:    []string{"db.local:192.168.1.100"},
		Mounts: []Mount{
			{Type: "bind", Source: "/home/user/app", Target: "/app", ReadOnly: false},
			{Type: "tmpfs", Target: "/tmp"},
		},
		Labels: map[string]string{
			"com.example.vendor": "Acme",
			"com.example.version": "1.0.0-rc1",
		},
		Env: []string{
			"NODE_ENV=production",
			"DATABASE_URL=postgres://user:pass@db:5432/app?sslmode=disable",
			"EMPTY_VAR=",
		},
		Workdir:    "/app",
		User:       "1000:1000",
		Privileged: false,
		Pid:        "host",
		ShmSize:    "512m",
		CapAdd:     []string{"SYS_PTRACE", "NET_ADMIN"},
		CapDrop:    []string{"ALL"},
		Entrypoint: []string{"/sbin/tini", "--"},
		Pull:       "ifnotpresent",
		Memory:     536870912, // 512MB
		CPUs:       1.5,
		Devices: []DeviceMapping{
			{PathOnHost: "/dev/net/tun", PathInContainer: "/dev/net/tun", CgroupPermissions: "rwm"},
		},
		GroupAdd: []string{"999", "1001"},
		Ulimits: []Ulimit{
			{Name: "nofile", Soft: 4096, Hard: 8192},
		},
		Sysctls: map[string]string{
			"net.core.somaxconn": "1024",
		},
		IPC:         "shareable",
		SecurityOpt: []string{"no-new-privileges:true", "seccomp=unconfined"},
		DNSSearch:   []string{"internal.domain"},
		DNSOptions:  []string{"ndots:3"},
		GPUs:        "device=0,1",
		Cgroupns:    "host",
		PidsLimit:   200,
		CPUShares:   512,
		CpusetCpus:  "0,1",
		CpusetMems:  "0",
		Restart:     "on-failure:3",
		OciRuntime:  "runc",
	}

	data, err := json.Marshal(cfg)
	require.NoError(t, err)

	var restored ContainerConfig
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)

	assert.Equal(t, cfg.Image, restored.Image)
	assert.Equal(t, cfg.Command, restored.Command)
	assert.Equal(t, cfg.Ports, restored.Ports)
	assert.Equal(t, cfg.DNS, restored.DNS)
	assert.Equal(t, cfg.Env, restored.Env)
	assert.Equal(t, cfg.CapAdd, restored.CapAdd)
	assert.Equal(t, cfg.CapDrop, restored.CapDrop)
	assert.Equal(t, cfg.Memory, restored.Memory)
	assert.InDelta(t, cfg.CPUs, restored.CPUs, 0.001)
	assert.Equal(t, cfg.SecurityOpt, restored.SecurityOpt)
	assert.Equal(t, cfg.Restart, restored.Restart)
	assert.Equal(t, cfg.OciRuntime, restored.OciRuntime)
}
