package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestScenarios_ParameterValidators_Resilience verifies boundary cases, control characters,
// invalid UTF-8 sequences, and syntax limits across all parameter validators in internal/config.
// Reference: docs/features/command-line-options.md & docs/testing/strategy.md
func TestScenarios_ParameterValidators_Resilience(t *testing.T) {
	t.Parallel()

	t.Run("ValidateToolName", func(t *testing.T) {
		validNames := []string{"node", "python3", "my-tool_v1.0", "tool.sub"}
		for _, name := range validNames {
			require.NoError(t, ValidateToolName(name), "valid tool name should pass: %s", name)
		}

		invalidNames := []string{
			"",
			"tool name",
			"tool/name",
			"tool;name",
			"tool\x00name",
			"tool\x1b[31mname",
			"tool\xffname",
		}
		for _, name := range invalidNames {
			require.Error(t, ValidateToolName(name), "invalid tool name should fail: %q", name)
		}
	})

	t.Run("ValidateHostname", func(t *testing.T) {
		require.NoError(t, ValidateHostname("my-host.local"))
		require.NoError(t, ValidateHostname(""), "empty hostname is permitted as optional setting")
		require.Error(t, ValidateHostname("host_name"), "underscore is invalid in hostname RFC 1123")
		require.Error(t, ValidateHostname("host\x00name"))
	})

	t.Run("ValidateDNSOption", func(t *testing.T) {
		require.NoError(t, ValidateDNSOption("ndots:5"))
		require.NoError(t, ValidateDNSOption("timeout:2"))
		require.NoError(t, ValidateDNSOption(""), "empty dns option is permitted as optional setting")
		require.Error(t, ValidateDNSOption("invalid option!"))
		require.Error(t, ValidateDNSOption("ndots:\x005"))
	})

	t.Run("ValidateSecurityOpt", func(t *testing.T) {
		require.NoError(t, ValidateSecurityOpt("no-new-privileges:true"))
		require.NoError(t, ValidateSecurityOpt("seccomp=unconfined"))
		require.NoError(t, ValidateSecurityOpt(""), "empty security opt is permitted as optional setting")
		require.Error(t, ValidateSecurityOpt("security;\x00opt"))
	})

	t.Run("ValidateSysctlKeyAndValue", func(t *testing.T) {
		require.NoError(t, ValidateSysctlKey("net.ipv4.ip_forward"))
		require.Error(t, ValidateSysctlKey(""))
		require.Error(t, ValidateSysctlKey("net..ipv4"))
		require.Error(t, ValidateSysctlKey("net/ipv4"))

		require.NoError(t, ValidateSysctlValue("1"))
		require.NoError(t, ValidateSysctlValue("1 0 1"))
		require.NoError(t, ValidateSysctlValue(""), "empty sysctl value is permitted as optional setting")
		require.Error(t, ValidateSysctlValue("1;\x000"))
	})

	t.Run("ValidateGPUsAndCpuset", func(t *testing.T) {
		require.NoError(t, ValidateGPUs("all"))
		require.NoError(t, ValidateGPUs("device=0,1"))
		require.Error(t, ValidateGPUs("gpus,,"))
		require.Error(t, ValidateGPUs("gpus;\x00"))

		require.NoError(t, ValidateCpuset("0-3"))
		require.NoError(t, ValidateCpuset("0,2,4-6"))
		require.Error(t, ValidateCpuset("0--3"))
		require.Error(t, ValidateCpuset("0-3-4"))
		require.Error(t, ValidateCpuset("0,\x001"))
	})

	t.Run("ValidateAddHostAndImageName", func(t *testing.T) {
		require.NoError(t, ValidateAddHost("host.docker.internal:host-gateway"))
		require.Error(t, ValidateAddHost(":127.0.0.1"))
		require.Error(t, ValidateAddHost("hostname:"))

		require.NoError(t, ValidateImageName("node:20-alpine"))
		require.NoError(t, ValidateImageName("ghcr.io/owner/repo:tag"))
		require.Error(t, ValidateImageName("node/"))
		require.Error(t, ValidateImageName("node::20"))
		require.Error(t, ValidateImageName("node\x00:20"))
	})

	t.Run("ValidateEnvKeyAndMountType", func(t *testing.T) {
		require.NoError(t, ValidateEnvKey("MY_VAR_123"))
		require.Error(t, ValidateEnvKey("123_VAR"))
		require.Error(t, ValidateEnvKey("MY-VAR"))

		require.NoError(t, ValidateMountType("bind"))
		require.NoError(t, ValidateMountType("volume"))
		require.NoError(t, ValidateMountType("tmpfs"))
		require.Error(t, ValidateMountType("invalid"))
	})
}

// TestScenarios_ExpressionResolution_FallbacksAndResilience tests template expression
// fallbacks ({{env:VAR:-default}}, {{file:...:-default}}, {{find_dir:...:-default}}).
// Reference: docs/features/value-resolution.md & Task T92
func TestScenarios_ExpressionResolution_FallbacksAndResilience(t *testing.T) {
	mfs := &MockFileSystem{
		WD:      "/workspace",
		HomeDir: "/workspace/home",
		Env: map[string]string{
			"CDERUN_TEST_SET_ENV_VAL": "custom_value",
		},
		Files: map[string][]byte{
			"/workspace/config.txt": []byte("file_content"),
		},
		Dirs: map[string]bool{
			"/workspace/target_dir": true,
		},
	}
	r, err := NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

	t.Run("Env Fallback Handling", func(t *testing.T) {
		resSet, err := r.ResolveString("val={{env:CDERUN_TEST_SET_ENV_VAL:-fallback_val}}")
		require.NoError(t, err)
		assert.Equal(t, "val=custom_value", resSet)

		resUnset, err := r.ResolveString("val={{env:CDERUN_TEST_UNSET_ENV_VAL:-fallback_val}}")
		require.NoError(t, err)
		assert.Equal(t, "val=fallback_val", resUnset)
	})

	t.Run("File Directive Fallback Handling", func(t *testing.T) {
		resExist, err := r.ResolveString("data={{file:config.txt:-default_data}}")
		require.NoError(t, err)
		assert.Equal(t, "data=file_content", resExist)

		resMissing, err := r.ResolveString("data={{file:missing.txt:-default_data}}")
		require.NoError(t, err)
		assert.Equal(t, "data=default_data", resMissing)
	})

	t.Run("FindDir Directive Fallback Handling", func(t *testing.T) {
		resExist, err := r.ResolveString("dir={{find_dir:target_dir:-fallback_dir}}")
		require.NoError(t, err)
		assert.Equal(t, "dir=/workspace", resExist, "find_dir returns directory containing target_dir")

		resMissing, err := r.ResolveString("dir={{find_dir:non_existent_dir:-fallback_dir}}")
		require.NoError(t, err)
		assert.Equal(t, "dir=fallback_dir", resMissing)
	})
}

// TestScenarios_SensitiveEnvMasking_Resilience verifies masking functionality on sensitive
// environment variable key lists across glob patterns, uppercase/lowercase keys, and non-ASCII strings.
// Reference: docs/features/command-line-options.md & docs/testing/strategy.md
func TestScenarios_SensitiveEnvMasking_Resilience(t *testing.T) {
	t.Parallel()

	envList := []string{
		"NORMAL_VAR=hello",
		"API_KEY=secret_key_123",
		"MY_PASSWORD=super_secret",
		"AUTH_TOKEN=bearer_xyz",
		"Custom_Secret_Val=masked_data",
		"NON_ASCII_🔑=value",
	}

	patterns := []string{
		"*KEY*",
		"*PASSWORD*",
		"*TOKEN*",
		"Custom_Secret_*",
	}

	masked := MaskSensitiveEnvList(envList, patterns)
	require.Len(t, masked, len(envList))

	expected := map[string]string{
		"NORMAL_VAR":        "hello",
		"API_KEY":           "[REDACTED]",
		"MY_PASSWORD":       "[REDACTED]",
		"AUTH_TOKEN":        "[REDACTED]",
		"Custom_Secret_Val": "[REDACTED]",
		"NON_ASCII_🔑":       "value",
	}

	seenKeys := make(map[string]bool)
	for _, item := range masked {
		k, v, found := parseKeyValue(item)
		require.True(t, found, "item should be key=value pair: %s", item)
		require.False(t, seenKeys[k], "duplicate key observed in masked list: %s", k)
		seenKeys[k] = true

		expVal, inExpected := expected[k]
		require.True(t, inExpected, "unexpected key found in masked list: %s", k)
		assert.Equal(t, expVal, v, "mismatch for key: %s", k)
	}

	assert.Len(t, seenKeys, len(expected), "all expected keys must be present in masked output")

	// Empty list and empty patterns slice edge cases
	assert.Empty(t, MaskSensitiveEnvList(nil, patterns))
	assert.Equal(t, []string{"A=B"}, MaskSensitiveEnvList([]string{"A=B"}, []string{}))
}

func parseKeyValue(item string) (string, string, bool) {
	for i := 0; i < len(item); i++ {
		if item[i] == '=' {
			return item[:i], item[i+1:], true
		}
	}
	return "", "", false
}

// TestScenarios_PrecedenceMatrix_MultiLayer verifies configuration evaluation precedence
// across CLI options, environment variables, local configs, and default settings.
// Reference: docs/features/argument-priority-logic.md
func TestScenarios_PrecedenceMatrix_MultiLayer(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{WD: "/workspace", HomeDir: "/workspace/home"}

	cliOpts := CLIOptions{
		Runtime:  localStrPtr("docker"),
		Pid:      localStrPtr("host"),
		ShmSize:  localStrPtr("2g"),
		ReadOnly: localBoolPtr(true),
		Restart:  localStrPtr("unless-stopped"),
		Remove:   localBoolPtr(false),
		IPC:      localStrPtr("host"),
		Image:    localStrPtr("node:22-alpine"),
	}

	toolsCfg := ToolsConfig{
		"node": ToolConfig{
			Image: "node:20-alpine",
		},
	}
	globalCfg := &CDERunConfig{
		Runtime: "podman",
	}

	res, err := ResolveWithFS("node", &cliOpts, toolsCfg, globalCfg, mfs)
	require.NoError(t, err)

	assert.Equal(t, "docker", res.Runtime, "CLI option should override local config")
	assert.Equal(t, "host", res.Pid)
	assert.Equal(t, "2g", res.ShmSize)
	assert.True(t, res.ReadOnly)
	assert.False(t, res.Remove)
	assert.Equal(t, "unless-stopped", res.Restart)
	assert.Equal(t, "host", res.IPC)
	assert.Equal(t, "node:22-alpine", res.Image, "Explicit CLI image should override tool image")
}

func localStrPtr(s string) *string {
	return &s
}

func localBoolPtr(b bool) *bool {
	return &b
}
