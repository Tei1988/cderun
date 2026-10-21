package container

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestUnit_ContainerConfig_MalformedJSONDeserializationErrors(t *testing.T) {
	tests := []struct {
		name      string
		jsonInput string
	}{
		{
			name:      "invalid_json_syntax",
			jsonInput: `{"image": "alpine", "command": [`,
		},
		{
			name:      "invalid_memory_type_string",
			jsonInput: `{"image": "alpine", "memory": "100MB"}`,
		},
		{
			name:      "invalid_cpus_type_string",
			jsonInput: `{"image": "alpine", "cpus": "two"}`,
		},
		{
			name:      "invalid_tty_type_number",
			jsonInput: `{"image": "alpine", "tty": 1}`,
		},
		{
			name:      "invalid_mounts_type_object",
			jsonInput: `{"image": "alpine", "mounts": {"target": "/tmp"}}`,
		},
		{
			name:      "invalid_env_type_object",
			jsonInput: `{"image": "alpine", "env": {"KEY": "VAL"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cfg ContainerConfig
			err := json.Unmarshal([]byte(tt.jsonInput), &cfg)
			require.Error(t, err)
		})
	}
}

func TestUnit_ContainerConfig_MalformedYAMLDeserializationErrors(t *testing.T) {
	tests := []struct {
		name      string
		yamlInput string
	}{
		{
			name:      "invalid_yaml_indentation",
			yamlInput: "image: alpine\n  command: [sh]\n command: bad_indent",
		},
		{
			name:      "invalid_memory_type_string",
			yamlInput: "image: alpine\nmemory: invalid_number\n",
		},
		{
			name:      "invalid_ulimits_type_string",
			yamlInput: "image: alpine\nulimits: invalid_list\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cfg ContainerConfig
			err := yaml.Unmarshal([]byte(tt.yamlInput), &cfg)
			require.Error(t, err)
		})
	}
}

func TestUnit_ContainerConfig_MapFieldInvariants(t *testing.T) {
	t.Run("nil vs empty map JSON serialization", func(t *testing.T) {
		nilMapCfg := ContainerConfig{
			Image:   "alpine:latest",
			Labels:  nil,
			Sysctls: nil,
		}

		dataNil, err := json.Marshal(nilMapCfg)
		require.NoError(t, err)

		var rawNil map[string]any
		err = json.Unmarshal(dataNil, &rawNil)
		require.NoError(t, err)

		assert.NotContains(t, rawNil, "labels")
		assert.NotContains(t, rawNil, "sysctls")

		emptyMapCfg := ContainerConfig{
			Image:   "alpine:latest",
			Labels:  map[string]string{},
			Sysctls: map[string]string{},
		}

		dataEmpty, err := json.Marshal(emptyMapCfg)
		require.NoError(t, err)

		var rawEmpty map[string]any
		err = json.Unmarshal(dataEmpty, &rawEmpty)
		require.NoError(t, err)

		// Both labels and sysctls have omitempty tag, so empty maps should be omitted
		assert.NotContains(t, rawEmpty, "labels")
		assert.NotContains(t, rawEmpty, "sysctls")
	})

	t.Run("unicode and CJK map keys and values", func(t *testing.T) {
		cfg := ContainerConfig{
			Image: "alpine:latest",
			Labels: map[string]string{
				"環境":      "開発",
				"app.id":  "コンテナ_01",
				"special": "val=123&foo=bar",
			},
			Sysctls: map[string]string{
				"net.ipv4.ip_forward": "1",
			},
		}

		data, err := json.Marshal(cfg)
		require.NoError(t, err)

		var decoded ContainerConfig
		err = json.Unmarshal(data, &decoded)
		require.NoError(t, err)

		assert.Equal(t, cfg.Labels["環境"], decoded.Labels["環境"])
		assert.Equal(t, cfg.Labels["app.id"], decoded.Labels["app.id"])
		assert.Equal(t, cfg.Labels["special"], decoded.Labels["special"])
	})
}

func TestUnit_ContainerConfig_EnvArrayComplexRepresentations(t *testing.T) {
	cfg := ContainerConfig{
		Image: "alpine:latest",
		Env: []string{
			"EMPTY_VAR=",
			"MULTI_EQUALS=A=B=C",
			"MULTILINE=line1\nline2\nline3",
			"UNICODE_VAR=日本語テスト",
			"WHITESPACE_VAR=  spaced value  ",
			"SPECIAL_CHARS=!@#$%^&*()_+-=[]{}|;':\",./<>?",
		},
	}

	data, err := json.Marshal(cfg)
	require.NoError(t, err)

	var decoded ContainerConfig
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, cfg.Env, decoded.Env)
}

func TestUnit_Ulimit_StructuralEdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		ulimit Ulimit
	}{
		{
			name:   "soft_greater_than_hard",
			ulimit: Ulimit{Name: "nofile", Soft: 8192, Hard: 4096},
		},
		{
			name:   "zero_soft_and_hard",
			ulimit: Ulimit{Name: "core", Soft: 0, Hard: 0},
		},
		{
			name:   "negative_one_unlimited",
			ulimit: Ulimit{Name: "memlock", Soft: -1, Hard: -1},
		},
		{
			name:   "extreme_max_int64",
			ulimit: Ulimit{Name: "fsize", Soft: math.MaxInt64, Hard: math.MaxInt64},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonBytes, err := json.Marshal(tt.ulimit)
			require.NoError(t, err)

			var decoded Ulimit
			err = json.Unmarshal(jsonBytes, &decoded)
			require.NoError(t, err)

			assert.Equal(t, tt.ulimit, decoded)
		})
	}
}

func TestUnit_DeviceMapping_StructuralEdgeCases(t *testing.T) {
	devices := []DeviceMapping{
		{
			PathOnHost:        "/dev/kvm",
			PathInContainer:   "/dev/kvm",
			CgroupPermissions: "rwm",
		},
		{
			PathOnHost:        "/dev/dri/card0",
			PathInContainer:   "/dev/dri/card0",
			CgroupPermissions: "",
		},
		{
			PathOnHost:        "/dev/custom_host",
			PathInContainer:   "/dev/custom_container",
			CgroupPermissions: "rw",
		},
	}

	for _, dev := range devices {
		t.Run("device_"+dev.PathOnHost, func(t *testing.T) {
			data, err := json.Marshal(dev)
			require.NoError(t, err)

			var decoded DeviceMapping
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)

			assert.Equal(t, dev, decoded)
		})
	}
}

func TestUnit_Mount_StructuralEdgeCases(t *testing.T) {
	mounts := []Mount{
		{
			Type:     "bind",
			Source:   "/host/readonly/dir",
			Target:   "/container/readonly/dir",
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
		{
			Type:     "volume",
			Source:   "my_volume_name",
			Target:   "/var/lib/data",
			ReadOnly: true,
			Optional: false,
		},
	}

	for _, m := range mounts {
		t.Run("mount_"+m.Type+"_"+m.Target, func(t *testing.T) {
			data, err := json.Marshal(m)
			require.NoError(t, err)

			var decoded Mount
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)

			assert.Equal(t, m, decoded)
		})
	}
}
