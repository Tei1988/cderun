package container

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestUnit_ContainerConfig_JSONSerializationTagMatching(t *testing.T) {
	orig := ContainerConfig{
		Image:       "ubuntu:22.04",
		Command:     []string{"bash", "-c", "uptime"},
		TTY:         true,
		Interactive: true,
		Remove:      true,
		ReadOnly:    true,
		Init:        true,

		Network:    "custom-net",
		Ports:      []string{"8080:80"},
		PublishAll: true,
		Expose:     []string{"80/tcp"},
		Hostname:   "test-host",
		DNS:        []string{"8.8.8.8"},
		AddHosts:   []string{"example.local:127.0.0.1"},

		Mounts: []Mount{
			{
				Type:     "bind",
				Source:   "/host/path",
				Target:   "/container/path",
				ReadOnly: true,
				Optional: false,
			},
			{
				Type:     "tmpfs",
				Source:   "",
				Target:   "/tmp",
				ReadOnly: false,
				Optional: true,
			},
		},

		Labels: map[string]string{
			"cderun": "true",
			"env":    "test",
		},

		Env:     []string{"FOO=BAR", "BAZ=QUX"},
		Workdir: "/workspace",
		User:    "1000:1000",

		Privileged: true,
		Pid:        "host",
		ShmSize:    "64m",
		CapAdd:     []string{"SYS_ADMIN"},
		CapDrop:    []string{"ALL"},
		Entrypoint: []string{"/entrypoint.sh"},

		Pull:   "missing",
		Memory: 1073741824,
		CPUs:   2.5,

		Devices: []DeviceMapping{
			{
				PathOnHost:        "/dev/kvm",
				PathInContainer:   "/dev/kvm",
				CgroupPermissions: "rwm",
			},
		},

		GroupAdd: []string{"1001"},

		Ulimits: []Ulimit{
			{
				Name: "nofile",
				Soft: 1024,
				Hard: 2048,
			},
		},

		Sysctls: map[string]string{
			"net.ipv4.ip_forward": "1",
		},

		IPC:         "host",
		SecurityOpt: []string{"seccomp=unconfined"},
		DNSSearch:   []string{"example.com"},
		DNSOptions:  []string{"ndots:2"},
		GPUs:        "all",
		Cgroupns:    "private",
		PidsLimit:   100,
		CPUShares:   1024,
		CpusetCpus:  "0-3",
		CpusetMems:  "0",
		Restart:     "always",
		OciRuntime:  "runc",
	}

	data, err := json.Marshal(orig)
	require.NoError(t, err)

	var decoded ContainerConfig
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, orig, decoded)

	// Verify exact JSON keys against map representation
	var rawMap map[string]any
	err = json.Unmarshal(data, &rawMap)
	require.NoError(t, err)

	expectedKeys := []string{
		"image", "command", "tty", "interactive", "remove", "read_only", "init",
		"network", "ports", "publish_all", "expose", "hostname", "dns", "add_hosts",
		"mounts", "labels", "env", "workdir", "user", "privileged", "pid",
		"shm_size", "cap_add", "cap_drop", "entrypoint", "pull", "memory", "cpus",
		"devices", "group_add", "ulimits", "sysctls", "ipc", "security_opt",
		"dns_search", "dns_options", "gpus", "cgroupns", "pids_limit", "cpu_shares",
		"cpuset_cpus", "cpuset_mems", "restart", "oci_runtime",
	}

	for _, key := range expectedKeys {
		assert.Contains(t, rawMap, key, "JSON map missing expected tag key: %s", key)
	}
}

func TestUnit_ContainerConfig_OmitemptyTagFiltering(t *testing.T) {
	minConfig := ContainerConfig{
		Image:   "alpine:latest",
		Command: []string{"echo"},
		Network: "bridge",
		Workdir: "/",
		User:    "0",
		Env:     []string{},
		Mounts:  []Mount{},
	}

	data, err := json.Marshal(minConfig)
	require.NoError(t, err)

	var rawMap map[string]any
	err = json.Unmarshal(data, &rawMap)
	require.NoError(t, err)

	// Keys that do NOT have omitempty should be present
	assert.Contains(t, rawMap, "image")
	assert.Contains(t, rawMap, "command")
	assert.Contains(t, rawMap, "tty")
	assert.Contains(t, rawMap, "interactive")
	assert.Contains(t, rawMap, "remove")
	assert.Contains(t, rawMap, "network")
	assert.Contains(t, rawMap, "mounts")
	assert.Contains(t, rawMap, "env")
	assert.Contains(t, rawMap, "workdir")
	assert.Contains(t, rawMap, "user")

	// Keys that DO have omitempty and are zero-valued should NOT be present
	omittedKeys := []string{
		"read_only", "init", "ports", "publish_all", "expose", "hostname",
		"dns", "add_hosts", "labels", "privileged", "pid", "shm_size",
		"cap_add", "cap_drop", "entrypoint", "pull", "memory", "cpus",
		"devices", "group_add", "ulimits", "sysctls", "ipc", "security_opt",
		"dns_search", "dns_options", "gpus", "cgroupns", "pids_limit",
		"cpu_shares", "cpuset_cpus", "cpuset_mems", "restart", "oci_runtime",
	}

	for _, key := range omittedKeys {
		assert.NotContains(t, rawMap, key, "JSON map should omit empty key: %s", key)
	}
}

func TestUnit_ContainerConfig_YAMLSerializationTagMatching(t *testing.T) {
	orig := ContainerConfig{
		Image:      "redis:alpine",
		Command:    []string{"redis-server"},
		Network:    "host",
		User:       "999",
		Env:        []string{},
		OciRuntime: "crun",
		Mounts: []Mount{
			{
				Type:   "volume",
				Source: "redis-data",
				Target: "/data",
			},
		},
	}

	yamlBytes, err := yaml.Marshal(orig)
	require.NoError(t, err)

	var decoded ContainerConfig
	err = yaml.Unmarshal(yamlBytes, &decoded)
	require.NoError(t, err)

	assert.Equal(t, orig, decoded)
}

func TestUnit_ContainerConfig_JSONYAMLRoundtrip(t *testing.T) {
	orig := ContainerConfig{
		Image:       "python:3.11",
		Command:     []string{"python3", "-c", "print('hello')"},
		TTY:         false,
		Interactive: false,
		Remove:      true,
		Network:     "none",
		Workdir:     "/app",
		User:        "1000",
		Env:         []string{"ENV=prod"},
		Mounts:      []Mount{},
		CapAdd:      []string{"NET_ADMIN"},
		PidsLimit:   50,
		CPUShares:   512,
		Restart:     "no",
		OciRuntime:  "runc",
	}

	// Step 1: Struct -> JSON -> Struct
	jsonBytes, err := json.Marshal(orig)
	require.NoError(t, err)

	var fromJSON ContainerConfig
	err = json.Unmarshal(jsonBytes, &fromJSON)
	require.NoError(t, err)
	assert.Equal(t, orig, fromJSON)

	// Step 2: Struct -> YAML -> Struct
	yamlBytes, err := yaml.Marshal(fromJSON)
	require.NoError(t, err)

	var fromYAML ContainerConfig
	err = yaml.Unmarshal(yamlBytes, &fromYAML)
	require.NoError(t, err)
	assert.Equal(t, orig, fromYAML)
}

func TestUnit_Container_SubStructs(t *testing.T) {
	t.Run("Mount", func(t *testing.T) {
		m := Mount{
			Type:     "bind",
			Source:   "/var/log",
			Target:   "/var/log",
			ReadOnly: true,
			Optional: true,
		}

		data, err := json.Marshal(m)
		require.NoError(t, err)

		var decoded Mount
		err = json.Unmarshal(data, &decoded)
		require.NoError(t, err)

		assert.Equal(t, m, decoded)
	})

	t.Run("Ulimit", func(t *testing.T) {
		u := Ulimit{
			Name: "nofile",
			Soft: 65536,
			Hard: 65536,
		}

		data, err := json.Marshal(u)
		require.NoError(t, err)

		var decoded Ulimit
		err = json.Unmarshal(data, &decoded)
		require.NoError(t, err)

		assert.Equal(t, u, decoded)
	})

	t.Run("DeviceMapping", func(t *testing.T) {
		d := DeviceMapping{
			PathOnHost:        "/dev/net/tun",
			PathInContainer:   "/dev/net/tun",
			CgroupPermissions: "rwm",
		}

		data, err := json.Marshal(d)
		require.NoError(t, err)

		var decoded DeviceMapping
		err = json.Unmarshal(data, &decoded)
		require.NoError(t, err)

		assert.Equal(t, d, decoded)
	})
}
