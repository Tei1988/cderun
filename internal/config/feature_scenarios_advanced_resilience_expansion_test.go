package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestUnit_Config_AdvancedResilienceValidators verifies parameter safety validators
// against edge case inputs and boundary conditions.
// Reference: docs/features/security-validations.md
func TestUnit_Config_AdvancedResilienceValidators(t *testing.T) {
	t.Parallel()

	t.Run("ValidateToolName", func(t *testing.T) {
		validTools := []string{"node", "python3.11", "go_tool-v1", "npm-check", "tool..name"}
		for _, tool := range validTools {
			require.NoError(t, ValidateToolName(tool), "expected valid tool: %s", tool)
		}

		invalidTools := []string{"", "tool/1", "..", ".", "tool name", "tool$", "tool\x00"}
		for _, tool := range invalidTools {
			require.Error(t, ValidateToolName(tool), "expected invalid tool: %s", tool)
		}
	})

	t.Run("ValidateHostname", func(t *testing.T) {
		validHosts := []string{"", "localhost", "node-1.example.com", "my-host"}
		for _, host := range validHosts {
			require.NoError(t, ValidateHostname(host), "expected valid host: %s", host)
		}

		invalidHosts := []string{"host_name", "-invalid", "invalid-", "host..name", "host\x07"}
		for _, host := range invalidHosts {
			require.Error(t, ValidateHostname(host), "expected invalid host: %s", host)
		}
	})

	t.Run("ValidateDNSOption", func(t *testing.T) {
		validOptions := []string{"", "ndots:5", "timeout:2", "attempts:3", "use-vc", "rotate"}
		for _, opt := range validOptions {
			require.NoError(t, ValidateDNSOption(opt), "expected valid dns opt: %s", opt)
		}

		invalidOptions := []string{"opt with space", "opt\x00", "opt;bad", "../etc/resolv.conf"}
		for _, opt := range invalidOptions {
			require.Error(t, ValidateDNSOption(opt), "expected invalid dns opt: %s", opt)
		}
	})

	t.Run("ValidateSecurityOpt", func(t *testing.T) {
		validSecOpts := []string{"", "no-new-privileges:true", "seccomp=unconfined", "apparmor=unconfined", "label=disable"}
		for _, opt := range validSecOpts {
			require.NoError(t, ValidateSecurityOpt(opt), "expected valid sec opt: %s", opt)
		}

		invalidSecOpts := []string{"opt\x00", "opt;bad", "../etc/security", "opt$bad"}
		for _, opt := range invalidSecOpts {
			require.Error(t, ValidateSecurityOpt(opt), "expected invalid sec opt: %s", opt)
		}
	})

	t.Run("ValidateSysctlKeyAndValue", func(t *testing.T) {
		require.NoError(t, ValidateSysctlKey("net.ipv4.ip_forward"))
		require.NoError(t, ValidateSysctlValue("1"))

		require.Error(t, ValidateSysctlKey(""))
		require.Error(t, ValidateSysctlKey("net/ipv4"))
		require.Error(t, ValidateSysctlKey("net.ipv4.ip_forward\x00"))

		require.Error(t, ValidateSysctlValue("1\x00"))
		require.Error(t, ValidateSysctlValue("val;bad"))
	})

	t.Run("ValidateGPUsAndCpuset", func(t *testing.T) {
		require.NoError(t, ValidateGPUs("all"))
		require.NoError(t, ValidateGPUs("device=0,1"))
		require.NoError(t, ValidateGPUs(""))
		require.Error(t, ValidateGPUs("device=0\x00"))

		require.NoError(t, ValidateCpuset("0-3"))
		require.NoError(t, ValidateCpuset("0,1,2,3"))
		require.NoError(t, ValidateCpuset(""))
		require.Error(t, ValidateCpuset("0-3-4"))
		require.Error(t, ValidateCpuset("0-a"))
	})

	t.Run("ValidateAddHostAndImageName", func(t *testing.T) {
		require.NoError(t, ValidateAddHost("host.docker.internal:127.0.0.1"))
		require.Error(t, ValidateAddHost("invalidhost"))
		require.Error(t, ValidateAddHost(":127.0.0.1"))

		require.NoError(t, ValidateImageName("alpine:3.18"))
		require.NoError(t, ValidateImageName("ghcr.io/org/repo:tag"))
		require.Error(t, ValidateImageName("alpine:"))
		require.Error(t, ValidateImageName("alpine//tag"))
	})

	t.Run("ValidateEnvKeyAndMountType", func(t *testing.T) {
		require.NoError(t, ValidateEnvKey("MY_VAR_123"))
		require.Error(t, ValidateEnvKey("123_VAR"))
		require.Error(t, ValidateEnvKey("MY-VAR"))

		require.NoError(t, ValidateMountType("bind"))
		require.NoError(t, ValidateMountType("volume"))
		require.NoError(t, ValidateMountType("tmpfs"))
		require.Error(t, ValidateMountType("invalid_type"))
	})
}

// TestUnit_Config_AdvancedExpressionFallbacksAndMasking verifies fallback syntax
// and sensitive environment masking rules in expression resolution.
// Reference: docs/features/value-resolution.md
func TestUnit_Config_AdvancedExpressionFallbacksAndMasking(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		Files: map[string][]byte{
			"/app/existing.txt": []byte("content_from_file"),
		},
		WD: "/app",
	}

	r, err := NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

	t.Run("ExpressionFallbacks", func(t *testing.T) {
		res, err := r.ResolveString("{{file:missing.txt:-fallback_value}}")
		require.NoError(t, err)
		require.Equal(t, "fallback_value", res)

		resExisting, err := r.ResolveString("{{file:existing.txt:-fallback_value}}")
		require.NoError(t, err)
		require.Equal(t, "content_from_file", resExisting)
	})

	t.Run("DoubleBraceEscaping", func(t *testing.T) {
		res, err := r.ResolveString("literal_{{HOME}}_escaped")
		require.NoError(t, err)
		require.Contains(t, res, "literal_")
	})

	t.Run("SensitiveEnvMasking", func(t *testing.T) {
		envSlice := []string{
			"API_KEY=secret123",
			"DATABASE_PASSWORD=supersecret",
			"NORMAL_VAR=hello",
		}
		masked := MaskSensitiveEnvList(envSlice, nil)
		require.Len(t, masked, 3)
		for _, item := range masked {
			if item == "NORMAL_VAR=hello" {
				continue
			}
			require.Contains(t, item, "[REDACTED]")
		}
	})
}
