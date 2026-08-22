package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_ScenariosRefinement_ExpressionsAndValidation(t *testing.T) {
	t.Parallel()

	t.Run("template_expression_resolution_fallbacks_and_escaping", func(t *testing.T) {
		t.Parallel()

		mfs := &MockFileSystem{
			WD:      "/home/user/project",
			HomeDir: "/home/user",
			Env:     map[string]string{"EXISTING_VAR": "hello_world"},
		}

		hostCtx := &HostContext{
			WorkingDir: mfs.WD,
			HomeDir:    mfs.HomeDir,
		}

		resolver, err := NewExpressionResolverWithFS(hostCtx, mfs)
		require.NoError(t, err)

		// Test env fallback when unset
		resFallback, err := resolver.ResolveString("{{env:UNSET_VAR:-fallback_value}}")
		require.NoError(t, err)
		assert.Equal(t, "fallback_value", resFallback)

		// Test env existing var
		resExisting, err := resolver.ResolveString("{{env:EXISTING_VAR:-default_val}}")
		require.NoError(t, err)
		assert.Equal(t, "hello_world", resExisting)

		// Test double-brace escaping
		resEscaped, err := resolver.ResolveString("Escaped: {{{{raw_brace}}}}")
		require.NoError(t, err)
		assert.Equal(t, "Escaped: {{raw_brace}}", resEscaped)
	})

	t.Run("parameter_security_validators", func(t *testing.T) {
		t.Parallel()

		// DNS options
		require.NoError(t, ValidateDNSOption("ndots:5"))
		require.NoError(t, ValidateDNSOption("use-vc"))
		require.Error(t, ValidateDNSOption("ndots:5\x00"))
		require.Error(t, ValidateDNSOption("ndots:5\n"))

		// Security opts
		require.NoError(t, ValidateSecurityOpt("no-new-privileges:true"))
		require.NoError(t, ValidateSecurityOpt("seccomp=unconfined"))
		require.Error(t, ValidateSecurityOpt("seccomp=\x00unconfined"))

		// Sysctl keys and values
		require.NoError(t, ValidateSysctlKey("net.ipv4.ip_forward"))
		require.NoError(t, ValidateSysctlValue("1"))
		require.Error(t, ValidateSysctlKey(".net.ipv4.ip_forward"))
		require.Error(t, ValidateSysctlKey("net.ipv4..ip_forward"))
		require.Error(t, ValidateSysctlValue("1\x00"))
	})

	t.Run("sensitive_environment_variable_masking", func(t *testing.T) {
		t.Parallel()

		patterns := []string{"*TOKEN*", "SECRET_*", "*KEY"}

		assert.Equal(t, "[REDACTED]", MaskSensitiveEnv("API_TOKEN_VAL", "secret123", patterns))
		assert.Equal(t, "[REDACTED]", MaskSensitiveEnv("SECRET_PASSWORD", "secret123", patterns))
		assert.Equal(t, "[REDACTED]", MaskSensitiveEnv("PRIVATE_KEY", "secret123", patterns))
		assert.Equal(t, "public_val", MaskSensitiveEnv("PUBLIC_VAR", "public_val", patterns))

		// MaskSensitiveEnvList with nil pattern slice defaults to mask-all mode
		envList := []string{"USER=alice", "TOKEN=xyz123"}
		maskedList := MaskSensitiveEnvList(envList, nil)
		require.Len(t, maskedList, 2)
		assert.Equal(t, "USER=[REDACTED]", maskedList[0])
		assert.Equal(t, "TOKEN=[REDACTED]", maskedList[1])
	})

	t.Run("multi_source_option_resolution", func(t *testing.T) {
		t.Parallel()

		mfs := &MockFileSystem{
			WD:      "/workspace",
			HomeDir: "/home/user",
		}

		opts := CLIOptions{
			DNSSearch:   []string{"example.com"},
			SecurityOpt: []string{"no-new-privileges:true"},
		}

		cfg := &CDERunConfig{
			Defaults: ConfigDefaults{
				DNSSearch:   []string{"default.org"},
				SecurityOpt: []string{"seccomp=unconfined"},
				Sysctls:     []string{"net.ipv4.ip_forward=1"},
			},
		}

		tools := ToolsConfig{
			"app": ToolConfig{Image: "alpine:latest"},
		}

		resolved, err := ResolveWithFS("app", &opts, tools, cfg, mfs)
		require.NoError(t, err)

		assert.Equal(t, []string{"example.com"}, resolved.DNSSearch)
		assert.Equal(t, []string{"no-new-privileges:true"}, resolved.SecurityOpt)
		assert.Equal(t, map[string]string{"net.ipv4.ip_forward": "1"}, resolved.Sysctls)
	})
}
