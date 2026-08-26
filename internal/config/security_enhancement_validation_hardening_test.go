package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_SecurityEnhancements_ValidationHardening(t *testing.T) {
	t.Run("ValidateMountType control character hardening", func(t *testing.T) {
		validTypes := []string{"", "bind", "volume", "tmpfs"}
		for _, vt := range validTypes {
			require.NoError(t, ValidateMountType(vt), "valid mount type should pass: %q", vt)
		}

		invalidTypes := []string{"overlay", "sshfs", "nfs"}
		for _, it := range invalidTypes {
			require.Error(t, ValidateMountType(it), "unsupported mount type should fail: %q", it)
		}

		controlCharTypes := []string{"bind\x00", "bind\x01", "volume\x07", "tmpfs\r\n"}
		for _, cct := range controlCharTypes {
			err := ValidateMountType(cct)
			require.Error(t, err, "mount type with control character should fail: %q", cct)
			assert.Contains(t, err.Error(), "invalid character in path or configuration")
		}
	})

	t.Run("ValidateSysctlValue control character hardening", func(t *testing.T) {
		validSysctlValues := []string{"1", "68719476736", "1024 2048 4096", "val-1,val-2"}
		for _, sv := range validSysctlValues {
			assert.NoError(t, ValidateSysctlValue(sv), "valid sysctl value should pass: %q", sv)
		}

		invalidSysctlValues := []string{"1\x00", "1\x01", "1\r\n", "1; rm -rf /"}
		for _, isv := range invalidSysctlValues {
			assert.Error(t, ValidateSysctlValue(isv), "invalid sysctl value should fail: %q", isv)
		}
	})

	t.Run("ValidateToolName control character hardening", func(t *testing.T) {
		validToolNames := []string{"python", "node_18", "go-1.21", "my.tool"}
		for _, tn := range validToolNames {
			assert.NoError(t, ValidateToolName(tn), "valid tool name should pass: %q", tn)
		}

		invalidToolNames := []string{"", "..", ".", "/usr/bin/python", "tool name", "tool\x00", "tool\x07"}
		for _, itn := range invalidToolNames {
			assert.Error(t, ValidateToolName(itn), "invalid tool name should fail: %q", itn)
		}
	})

	t.Run("resolveEnv control character rejection in host environment variables", func(t *testing.T) {
		mfs := &MockFileSystem{
			Env: map[string]string{
				"SAFE_VAR":     "safe_value",
				"BAD_VAR_NULL": "bad\x00value",
				"BAD_VAR_CTRL": "bad\x07value",
			},
		}

		expr, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		// Safe environment variable
		res, err := expr.ResolveString("{{env:SAFE_VAR}}")
		require.NoError(t, err)
		assert.Equal(t, "safe_value", res)

		// Environment variable with null byte
		_, err = expr.ResolveString("{{env:BAD_VAR_NULL}}")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "security validation failed for env directive value")

		// Environment variable with control character
		_, err = expr.ResolveString("{{env:BAD_VAR_CTRL}}")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "security validation failed for env directive value")
	})

	t.Run("validateSecurity cross-platform sensitive mount path checking", func(t *testing.T) {
		buf := setupTestLogger(t, "warn")

		mfs := &MockFileSystem{WD: "/work"}
		opts := &CLIOptions{
			Image: strPtrVal("ubuntu:latest"),
			Mounts: []string{
				"type=bind,source=/usr/../etc/ssl,target=/container/etc",
			},
		}

		res, err := ResolveWithFS("sh", opts, nil, nil, mfs)
		require.NoError(t, err)
		require.Len(t, res.Mounts, 1)
		assert.Equal(t, "/etc/ssl", res.Mounts[0].Source)
		assert.Contains(t, buf.String(), "Mounting highly sensitive host path")
	})
}

func strPtrVal(s string) *string {
	return &s
}
