package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_Resolver_SysctlControlCharacterHardening(t *testing.T) {
	t.Parallel()
	mfs := &MockFileSystem{WD: "/work"}

	t.Run("Sysctl key with control character is rejected", func(t *testing.T) {
		cli := &CLIOptions{
			Image:   ptr("alpine"),
			Sysctls: []string{"net.ipv4.ip_forward\x01=1"},
		}

		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid character in sysctl key")
	})

	t.Run("Sysctl value with control character is rejected", func(t *testing.T) {
		cli := &CLIOptions{
			Image:   ptr("alpine"),
			Sysctls: []string{"net.ipv4.ip_forward=1\x01"},
		}

		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid character in sysctl value")
	})

	t.Run("Valid sysctl is accepted", func(t *testing.T) {
		cli := &CLIOptions{
			Image:   ptr("alpine"),
			Sysctls: []string{"net.ipv4.ip_forward=1"},
		}

		res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)
		require.Len(t, res.Sysctls, 1)
		assert.Equal(t, "1", res.Sysctls["net.ipv4.ip_forward"])
	})
}

func TestUnit_Config_Expression_EnvDefaultValueHardening(t *testing.T) {
	t.Parallel()
	mfs := &MockFileSystem{WD: "/work"}

	t.Run("Env directive default value with control character is rejected", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		_, err = r.ResolveString("{{env:SOME_UNSET_KEY:-default\x01value}}")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "security validation failed for env directive default value")
	})

	t.Run("Env directive default value with null byte is rejected", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		_, err = r.ResolveString("{{env:SOME_UNSET_KEY:-default\x00value}}")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "security validation failed for env directive default value")
	})

	t.Run("Valid env directive default value is accepted", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		val, err := r.ResolveString("{{env:SOME_UNSET_KEY:-default-value}}")
		require.NoError(t, err)
		assert.Equal(t, "default-value", val)
	})
}

func TestUnit_Config_Path_SimplifiedValidationChecks(t *testing.T) {
	t.Parallel()

	t.Run("ValidateHostname handles safe and unsafe characters correctly under new De Morgan rule", func(t *testing.T) {
		require.NoError(t, ValidateHostname("valid-host.com"))
		require.Error(t, ValidateHostname("invalid_host"))
		require.Error(t, ValidateHostname("-invalid-host"))
	})

	t.Run("ValidateUserName handles safe and unsafe characters correctly under new De Morgan rule", func(t *testing.T) {
		require.NoError(t, ValidateUserName("root"))
		require.NoError(t, ValidateUserName("user123:group456"))
		require.Error(t, ValidateUserName("invalid:user:group"))
		require.Error(t, ValidateUserName("invalid_user_name_with_capital_A"))
	})

	t.Run("ValidateCapability handles safe and unsafe capabilities", func(t *testing.T) {
		require.NoError(t, ValidateCapability("SYS_ADMIN"))
		require.Error(t, ValidateCapability("sys_admin"))
	})

	t.Run("ValidateWorkdir handles valid and invalid working directories", func(t *testing.T) {
		require.NoError(t, ValidateWorkdir("/valid/work-dir_1"))
		require.Error(t, ValidateWorkdir("relative/dir"))
		require.Error(t, ValidateWorkdir("/invalid/dir/../traversal"))
	})
}
