package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_ExpressionEdgeCases_Refinement(t *testing.T) {
	t.Parallel()

	t.Run("nested_and_escaped_expressions", func(t *testing.T) {
		t.Parallel()

		mfs := &MockFileSystem{
			WD:      "/workspace/project",
			HomeDir: "/home/user",
			Env:     map[string]string{"APP_ENV": "production", "NESTED_KEY": "APP_ENV"},
		}

		hostCtx := &HostContext{
			WorkingDir: mfs.WD,
			HomeDir:    mfs.HomeDir,
		}

		// Double-brace escaping resolution: {{{{escaped_val}}}} -> {{escaped_val}}
		r1, err := NewExpressionResolverWithFS(hostCtx, mfs)
		require.NoError(t, err)
		res1, err := r1.ResolveString("Value is {{{{escaped_val}}}}")
		require.NoError(t, err)
		assert.Equal(t, "Value is {{escaped_val}}", res1)

		// Nested expression resolution: {{env:UNSET_VAR:-{{env:APP_ENV}}}}
		r2, err := NewExpressionResolverWithFS(hostCtx, mfs)
		require.NoError(t, err)
		res2, err := r2.ResolveString("{{env:UNSET_VAR:-{{env:APP_ENV}}}}")
		require.NoError(t, err)
		assert.Equal(t, "production", res2)

		// Direct existing env resolution
		r3, err := NewExpressionResolverWithFS(hostCtx, mfs)
		require.NoError(t, err)
		res3, err := r3.ResolveString("{{env:APP_ENV:-staging}}")
		require.NoError(t, err)
		assert.Equal(t, "production", res3)
	})

	t.Run("strict_env_validation", func(t *testing.T) {
		t.Parallel()

		mfs := &MockFileSystem{
			WD:      "/workspace",
			HomeDir: "/home/user",
		}

		tools := ToolsConfig{
			"python": ToolConfig{Image: "python:3.11"},
		}

		// With strictEnv = true, resolving missing environment variable yields error
		optsStrict := CLIOptions{
			Env:       []string{"MISSING_ENV_VAR"},
			StrictEnv: ptr(true),
		}
		_, err := ResolveWithFS("python", &optsStrict, tools, nil, mfs)
		require.Error(t, err)

		// With strictEnv = false (default), missing env expression evaluates without error
		hostCtx := &HostContext{WorkingDir: mfs.WD, HomeDir: mfs.HomeDir}
		r, err := NewExpressionResolverWithFS(hostCtx, mfs)
		require.NoError(t, err)

		res, err := r.ResolveString("{{env:MISSING_ENV_VAR}}")
		require.NoError(t, err)
		assert.Equal(t, "", res)
	})
}

func TestUnit_Config_MaskingPatternMatching_Refinement(t *testing.T) {
	t.Parallel()

	patterns := []string{"*TOKEN*", "*SECRET*", "API_KEY", "PASS*"}

	testCases := []struct {
		key      string
		expected bool
	}{
		{"MY_API_TOKEN_HERE", true},
		{"USER_SECRET", true},
		{"API_KEY", true},
		{"PASSWORD123", true},
		{"UNRELATED_VAR", false},
		{"api_key", true}, // Case insensitive
		{"my_pass_word", false},
	}

	for _, tc := range testCases {
		t.Run(tc.key, func(t *testing.T) {
			t.Parallel()
			res := MaskSensitiveEnv(tc.key, "secret_val", patterns)
			if tc.expected {
				assert.Equal(t, "[REDACTED]", res)
			} else {
				assert.Equal(t, "secret_val", res)
			}
		})
	}

	t.Run("empty_and_nil_patterns", func(t *testing.T) {
		t.Parallel()

		envList := []string{"FOO=123", "BAR=456"}
		// nil patterns defaults to mask-all behavior in MaskSensitiveEnvList
		masked := MaskSensitiveEnvList(envList, nil)
		for _, pair := range masked {
			assert.Contains(t, pair, "=[REDACTED]")
		}
	})
}

func TestUnit_Config_MultiSourceOptionPrecedence_Refinement(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD:      "/workspace",
		HomeDir: "/home/user",
	}

	// Setup CLI vs Config File
	opts := CLIOptions{
		DNSOptions: []string{"ndots:3"},
		Sysctls:    []string{"net.core.somaxconn=1024"},
	}

	cfg := &CDERunConfig{
		Defaults: ConfigDefaults{
			DNSOptions: []string{"ndots:5", "timeout:2"},
			Sysctls:    []string{"net.core.somaxconn=512", "net.ipv4.ip_forward=1"},
		},
	}

	tools := ToolsConfig{
		"python": ToolConfig{
			Image: "python:3.11",
		},
	}

	res, err := ResolveWithFS("python", &opts, tools, cfg, mfs)
	require.NoError(t, err)

	// CLI should take precedence for DNSOptions
	assert.Equal(t, []string{"ndots:3"}, res.DNSOptions)

	// CLI should take precedence for Sysctls
	assert.Equal(t, map[string]string{"net.core.somaxconn": "1024"}, res.Sysctls)
}

func TestUnit_Config_PathAndSecurityValidation_Refinement(t *testing.T) {
	t.Parallel()

	t.Run("sysctl_key_value_validation", func(t *testing.T) {
		t.Parallel()

		require.NoError(t, ValidateSysctlKey("net.ipv4.ip_forward"))
		require.NoError(t, ValidateSysctlValue("1"))

		// Reject null byte or control chars
		require.Error(t, ValidateSysctlKey("net.ipv4.\x00ip_forward"))
		require.Error(t, ValidateSysctlValue("1\r\n"))
	})

	t.Run("dns_option_validation", func(t *testing.T) {
		t.Parallel()

		require.NoError(t, ValidateDNSOption("ndots:5"))
		require.NoError(t, ValidateDNSOption("timeout:2"))
		require.Error(t, ValidateDNSOption("ndots:5\x00"))
		require.Error(t, ValidateDNSOption("ndots:5; rm -rf /"))
	})

	t.Run("security_opt_validation", func(t *testing.T) {
		t.Parallel()

		require.NoError(t, ValidateSecurityOpt("no-new-privileges:true"))
		require.NoError(t, ValidateSecurityOpt("seccomp=unconfined"))
		require.Error(t, ValidateSecurityOpt("seccomp=\x00unconfined"))
	})
}
