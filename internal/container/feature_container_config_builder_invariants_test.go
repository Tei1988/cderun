package container

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// Specification: docs/features/direct-container-execution.md - Intermediate Representation (IR): ContainerConfig
// TestUnit_Container_ConfigFullFieldsJSONYAML verifies that all ContainerConfig fields, including nested Mount,
// DeviceMapping, and Ulimit structs, accurately marshal to and unmarshal from JSON and YAML formats without data loss,
// and preserves the exact JSON and YAML field tag names.
func TestUnit_Container_ConfigFullFieldsJSONYAML(t *testing.T) {
	cfg := ContainerConfig{
		Image:       "ubuntu:22.04",
		Command:     []string{"/bin/bash", "-c", "echo hello"},
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
		AddHosts:   []string{"custom.domain:127.0.0.1"},

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
				Target:   "/tmp",
				ReadOnly: false,
				Optional: true,
			},
		},

		Labels: map[string]string{
			"app":  "cderun",
			"tier": "test",
		},

		Env:     []string{"ENV_A=1", "ENV_B=2"},
		Workdir: "/app",
		User:    "1001:1001",

		Privileged: true,
		Pid:        "host",
		ShmSize:    "256m",
		CapAdd:     []string{"SYS_PTRACE", "NET_ADMIN"},
		CapDrop:    []string{"MKNOD"},
		Entrypoint: []string{"/entrypoint.sh"},

		Pull: "always",

		Memory: 1073741824, // 1GB
		CPUs:   1.5,

		Devices: []DeviceMapping{
			{
				PathOnHost:        "/dev/kvm",
				PathInContainer:   "/dev/kvm",
				CgroupPermissions: "rwm",
			},
		},

		GroupAdd: []string{"1000", "docker"},

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

		IPC:         "shareable",
		SecurityOpt: []string{"no-new-privileges:true"},
		DNSSearch:   []string{"example.com"},
		DNSOptions:  []string{"ndots:2"},
		GPUs:        "all",
		Cgroupns:    "private",
		PidsLimit:   500,
		CPUShares:   1024,
		CpusetCpus:  "0-3",
		CpusetMems:  "0",
		Restart:     "on-failure:3",
		OciRuntime:  "runc",
	}

	// 1. JSON Roundtrip & Field Name Verification
	jsonBytes, err := json.Marshal(cfg)
	require.NoError(t, err)
	require.NotEmpty(t, jsonBytes)

	jsonStr := string(jsonBytes)
	// Assert top-level and nested struct JSON field tag keys directly
	assert.Contains(t, jsonStr, `"path_on_host":"/dev/kvm"`)
	assert.Contains(t, jsonStr, `"path_in_container":"/dev/kvm"`)
	assert.Contains(t, jsonStr, `"cgroup_permissions":"rwm"`)
	assert.Contains(t, jsonStr, `"type":"bind"`)
	assert.Contains(t, jsonStr, `"source":"/host/path"`)
	assert.Contains(t, jsonStr, `"target":"/container/path"`)
	assert.Contains(t, jsonStr, `"oci_runtime":"runc"`)

	// Inspect nested Mount.ReadOnly in JSON map directly
	var jsonMap map[string]any
	require.NoError(t, json.Unmarshal(jsonBytes, &jsonMap))
	jsonMounts, ok := jsonMap["mounts"].([]any)
	require.True(t, ok)
	require.Len(t, jsonMounts, 2)
	firstJSONMount, ok := jsonMounts[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, true, firstJSONMount["read_only"])

	var jsonDecoded ContainerConfig
	err = json.Unmarshal(jsonBytes, &jsonDecoded)
	require.NoError(t, err)
	assert.Equal(t, cfg, jsonDecoded)

	// 2. YAML Roundtrip & Field Name Verification
	yamlBytes, err := yaml.Marshal(cfg)
	require.NoError(t, err)
	require.NotEmpty(t, yamlBytes)

	yamlStr := string(yamlBytes)
	// Assert top-level and nested struct YAML field tag keys directly
	assert.Contains(t, yamlStr, "path_on_host: /dev/kvm")
	assert.Contains(t, yamlStr, "path_in_container: /dev/kvm")
	assert.Contains(t, yamlStr, "cgroup_permissions: rwm")
	assert.Contains(t, yamlStr, "type: bind")
	assert.Contains(t, yamlStr, "source: /host/path")
	assert.Contains(t, yamlStr, "target: /container/path")
	assert.Contains(t, yamlStr, "oci_runtime: runc")

	// Inspect nested Mount.ReadOnly in YAML map directly
	var yamlMap map[string]any
	require.NoError(t, yaml.Unmarshal(yamlBytes, &yamlMap))
	yamlMounts, ok := yamlMap["mounts"].([]any)
	require.True(t, ok)
	require.Len(t, yamlMounts, 2)
	firstYAMLMount, ok := yamlMounts[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, true, firstYAMLMount["read_only"])

	var yamlDecoded ContainerConfig
	err = yaml.Unmarshal(yamlBytes, &yamlDecoded)
	require.NoError(t, err)
	assert.Equal(t, cfg, yamlDecoded)
}

// Specification: docs/features/direct-container-execution.md - Intermediate Representation (IR): ContainerConfig
// TestUnit_Container_ConfigOmitEmptyFieldsJSON verifies that optional ContainerConfig fields with `omitempty` tags
// are excluded from JSON output when zero-valued or nil.
func TestUnit_Container_ConfigOmitEmptyFieldsJSON(t *testing.T) {
	// Minimum configuration with zero values for optional fields
	cfg := ContainerConfig{
		Image:   "alpine:latest",
		Command: []string{"echo", "hi"},
		Network: "bridge",
		Workdir: "/",
		User:    "0",
	}

	jsonBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	jsonStr := string(jsonBytes)

	// Confirm omitempty fields are NOT serialized
	assert.NotContains(t, jsonStr, "read_only")
	assert.NotContains(t, jsonStr, "init")
	assert.NotContains(t, jsonStr, "ports")
	assert.NotContains(t, jsonStr, "publish_all")
	assert.NotContains(t, jsonStr, "expose")
	assert.NotContains(t, jsonStr, "hostname")
	assert.NotContains(t, jsonStr, "dns")
	assert.NotContains(t, jsonStr, "add_hosts")
	assert.NotContains(t, jsonStr, "labels")
	assert.NotContains(t, jsonStr, "privileged")
	assert.NotContains(t, jsonStr, "pid")
	assert.NotContains(t, jsonStr, "shm_size")
	assert.NotContains(t, jsonStr, "cap_add")
	assert.NotContains(t, jsonStr, "cap_drop")
	assert.NotContains(t, jsonStr, "entrypoint")
	assert.NotContains(t, jsonStr, "pull")
	assert.NotContains(t, jsonStr, "memory")
	assert.NotContains(t, jsonStr, "cpus")
	assert.NotContains(t, jsonStr, "devices")
	assert.NotContains(t, jsonStr, "group_add")
	assert.NotContains(t, jsonStr, "ulimits")
	assert.NotContains(t, jsonStr, "sysctls")
	assert.NotContains(t, jsonStr, "ipc")
	assert.NotContains(t, jsonStr, "security_opt")
	assert.NotContains(t, jsonStr, "dns_search")
	assert.NotContains(t, jsonStr, "dns_options")
	assert.NotContains(t, jsonStr, "gpus")
	assert.NotContains(t, jsonStr, "cgroupns")
	assert.NotContains(t, jsonStr, "pids_limit")
	assert.NotContains(t, jsonStr, "cpu_shares")
	assert.NotContains(t, jsonStr, "cpuset_cpus")
	assert.NotContains(t, jsonStr, "cpuset_mems")
	assert.NotContains(t, jsonStr, "restart")
	assert.NotContains(t, jsonStr, "oci_runtime")

	// Confirm non-omitempty fields ARE present
	assert.Contains(t, jsonStr, `"image":"alpine:latest"`)
	assert.Contains(t, jsonStr, `"command":["echo","hi"]`)
	assert.Contains(t, jsonStr, `"network":"bridge"`)
	assert.Contains(t, jsonStr, `"workdir":"/"`)
	assert.Contains(t, jsonStr, `"user":"0"`)
}

// Specification: docs/features/direct-container-execution.md - Intermediate Representation (IR): ContainerConfig
// TestUnit_Container_ZeroValueConfigInvariants verifies that an uninitialized ContainerConfig struct starts in a valid
// zero-value state where all slice, map, and scalar attributes default to nil or zero values.
func TestUnit_Container_ZeroValueConfigInvariants(t *testing.T) {
	var cfg ContainerConfig

	assert.Empty(t, cfg.Image)
	assert.Nil(t, cfg.Command)
	assert.False(t, cfg.TTY)
	assert.False(t, cfg.Interactive)
	assert.False(t, cfg.Remove)
	assert.False(t, cfg.ReadOnly)
	assert.False(t, cfg.Init)
	assert.Empty(t, cfg.Network)
	assert.Nil(t, cfg.Mounts)
	assert.Nil(t, cfg.Labels)
	assert.Nil(t, cfg.Env)
	assert.Nil(t, cfg.Devices)
	assert.Nil(t, cfg.Ulimits)
	assert.Nil(t, cfg.Sysctls)
	assert.Zero(t, cfg.Memory)
	assert.Zero(t, cfg.CPUs)
}
