package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUnit_Config_ExpandedValidation_EdgeCases verifies edge cases for configuration validators
// as specified in docs/features/security-validations.md and docs/features/command-line-options.md.
func TestUnit_Config_ExpandedValidation_EdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("ValidateToolName_ASCIIAndControlCharacters", func(t *testing.T) {
		assert.NoError(t, ValidateToolName("gcc"))
		assert.NoError(t, ValidateToolName("python3.10"))
		assert.NoError(t, ValidateToolName("my-tool_v1"))

		// Invalid cases
		assert.Error(t, ValidateToolName(""))
		assert.Error(t, ValidateToolName("tool/name"))
		assert.Error(t, ValidateToolName("tool\\name"))
		assert.Error(t, ValidateToolName("../tool"))
		assert.Error(t, ValidateToolName("tool\x00"))
		assert.Error(t, ValidateToolName("tool\nname"))
		assert.Error(t, ValidateToolName("工具")) // non-ASCII
	})

	t.Run("ValidateHostname_EdgeCases", func(t *testing.T) {
		assert.NoError(t, ValidateHostname("my-host"))
		assert.NoError(t, ValidateHostname("sub.domain.local"))

		assert.NoError(t, ValidateHostname("")) // empty is valid (unconfigured)
		assert.Error(t, ValidateHostname("-invalid"))
		assert.Error(t, ValidateHostname("invalid-"))
		assert.Error(t, ValidateHostname("host\x07name"))
		assert.Error(t, ValidateHostname("host name"))
	})

	t.Run("ValidateDNSOption_EdgeCases", func(t *testing.T) {
		assert.NoError(t, ValidateDNSOption("ndots:5"))
		assert.NoError(t, ValidateDNSOption("timeout:2"))
		assert.NoError(t, ValidateDNSOption("use-vc"))

		assert.NoError(t, ValidateDNSOption("")) // empty is valid
		assert.Error(t, ValidateDNSOption("option\x1b[31m"))
		assert.Error(t, ValidateDNSOption("option with spaces"))
	})

	t.Run("ValidateSecurityOpt_EdgeCases", func(t *testing.T) {
		assert.NoError(t, ValidateSecurityOpt("no-new-privileges:true"))
		assert.NoError(t, ValidateSecurityOpt("seccomp=unconfined"))
		assert.NoError(t, ValidateSecurityOpt("apparmor=unconfined"))

		assert.NoError(t, ValidateSecurityOpt("")) // empty is valid
		assert.Error(t, ValidateSecurityOpt("sec\x00opt"))
		assert.Error(t, ValidateSecurityOpt("sec opt"))
	})

	t.Run("ValidateSysctlKeyAndValue_EdgeCases", func(t *testing.T) {
		assert.NoError(t, ValidateSysctlKey("net.ipv4.ip_forward"))
		assert.NoError(t, ValidateSysctlKey("kernel.shmmax"))

		assert.Error(t, ValidateSysctlKey(""))
		assert.Error(t, ValidateSysctlKey("net..ipv4"))
		assert.Error(t, ValidateSysctlKey("sysctl\x01key"))

		assert.NoError(t, ValidateSysctlValue("1"))
		assert.NoError(t, ValidateSysctlValue("4096 87380 4194304"))

		assert.Error(t, ValidateSysctlValue("val\x00ue"))
		assert.Error(t, ValidateSysctlValue("val\nue"))
	})

	t.Run("ValidateGPUsAndCpuset_EdgeCases", func(t *testing.T) {
		assert.NoError(t, ValidateGPUs("all"))
		assert.NoError(t, ValidateGPUs("2"))
		assert.NoError(t, ValidateGPUs("device=0,1"))

		assert.NoError(t, ValidateGPUs("")) // empty is valid
		require.Error(t, ValidateGPUs(",0,1"))
		require.Error(t, ValidateGPUs("device=0,1,"))
		require.Error(t, ValidateGPUs("gpus\x00invalid"))

		assert.NoError(t, ValidateCpuset("0-3"))
		assert.NoError(t, ValidateCpuset("0,2,4"))

		assert.NoError(t, ValidateCpuset("")) // empty is valid
		assert.Error(t, ValidateCpuset("0-3-4"))
		assert.Error(t, ValidateCpuset(",0-3"))
		assert.Error(t, ValidateCpuset("cpuset\x00invalid"))
	})

	t.Run("ValidateAddHostAndImageName_EdgeCases", func(t *testing.T) {
		assert.NoError(t, ValidateAddHost("myhost:127.0.0.1"))
		assert.NoError(t, ValidateAddHost("host.local:::1"))

		assert.NoError(t, ValidateAddHost("")) // empty string is valid (unconfigured)
		assert.Error(t, ValidateAddHost(":127.0.0.1"))
		assert.Error(t, ValidateAddHost("myhost:"))
		assert.Error(t, ValidateAddHost("host\x00:127.0.0.1"))

		assert.NoError(t, ValidateImageName("alpine:latest"))
		assert.NoError(t, ValidateImageName("docker.io/library/ubuntu:22.04"))

		assert.NoError(t, ValidateImageName("")) // empty is valid
		assert.Error(t, ValidateImageName("alpine:"))
		assert.Error(t, ValidateImageName("ubuntu/"))
		assert.Error(t, ValidateImageName("repo//image"))
		assert.Error(t, ValidateImageName("image\x00tag"))
	})
}

// TestUnit_Config_ExpandedExpressions_AndFallback verifies template expression evaluation
// and fallback handling according to docs/features/value-resolution.md.
func TestUnit_Config_ExpandedExpressions_AndFallback(t *testing.T) {
	mfs := &MockFileSystem{
		Env: map[string]string{
			"TEST_EXP_VAR_SET":   "custom_val",
			"TEST_EXP_VAR_EMPTY": "",
		},
		Files: map[string][]byte{
			"test.txt": []byte("file_content_data"),
		},
	}

	r, err := NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

	// Test env variable expression with fallback
	resolved, err := r.ResolveString("{{env:TEST_EXP_VAR_SET:-default_val}}")
	require.NoError(t, err)
	assert.Equal(t, "custom_val", resolved)

	resolvedDefault, err := r.ResolveString("{{env:TEST_EXP_VAR_UNSET:-default_val}}")
	require.NoError(t, err)
	assert.Equal(t, "default_val", resolvedDefault)

	resolvedEmptyFallback, err := r.ResolveString("{{env:TEST_EXP_VAR_EMPTY:-default_val}}")
	require.NoError(t, err)
	assert.Equal(t, "default_val", resolvedEmptyFallback)

	// Test file expression
	resolvedFile, err := r.ResolveString("{{file:test.txt}}")
	require.NoError(t, err)
	assert.Equal(t, "file_content_data", resolvedFile)

	// Test file expression fallback on missing file
	resolvedMissing, err := r.ResolveString("{{file:nonexistent.txt:-fallback_content}}")
	require.NoError(t, err)
	assert.Equal(t, "fallback_content", resolvedMissing)
}

// TestUnit_Config_ExpandedMasking_Invariants verifies sensitive environment variable masking rules
// as specified in docs/features/security-validations.md.
func TestUnit_Config_ExpandedMasking_Invariants(t *testing.T) {
	t.Parallel()

	rawEnvs := []string{
		"SECRET_TOKEN=super_secret_123",
		"API_KEY=key_xyz_987",
		"PUBLIC_PORT=8080",
	}

	masked := MaskSensitiveEnvList(rawEnvs, []string{"SECRET_*", "*KEY*"})
	assert.Contains(t, masked, "SECRET_TOKEN=[REDACTED]")
	assert.Contains(t, masked, "API_KEY=[REDACTED]")
	assert.Contains(t, masked, "PUBLIC_PORT=8080")
}
