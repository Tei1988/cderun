package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_Security_Volume_Sysctl_DNS_Hardening(t *testing.T) {
	t.Run("resolveVolumePath rejects parent traversal and control characters", func(t *testing.T) {
		r := &ExpressionResolver{}

		// Parent traversal in volume host or specification
		_, err := resolveVolumePath("../host_path:/container_path", "/tmp", r)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parent directory references")

		_, err = resolveVolumePath("vol_name:../container_path", "/tmp", r)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parent directory references")

		// Control character in volume specification
		_, err = resolveVolumePath("vol_name\x00:/container_path", "/tmp", r)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "security validation failed for volume")
	})

	t.Run("ValidateDNSOption rejects control characters and invalid UTF-8", func(t *testing.T) {
		// Valid options
		require.NoError(t, ValidateDNSOption("ndots:5"))
		require.NoError(t, ValidateDNSOption("timeout:2"))
		require.NoError(t, ValidateDNSOption("attempts:3"))

		// Control character
		err := ValidateDNSOption("ndots:5\x00")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid character in path or configuration")

		// Invalid UTF-8 sequence
		err = ValidateDNSOption("ndots:\xff\xfe")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid character in path or configuration")
	})

	t.Run("validateSysctlSecurity via validateSecurity rejects malformed sysctls", func(t *testing.T) {
		rv := &resolver{
			res: &ResolvedConfig{
				Runtime: "docker",
				Sysctls: map[string]string{
					".net.ipv4.ip_forward": "1", // Leading dot in sysctl key
				},
			},
		}

		err := rv.validateSecurity()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "security validation failed for sysctl key")

		rv = &resolver{
			res: &ResolvedConfig{
				Runtime: "docker",
				Sysctls: map[string]string{
					"net.ipv4.ip_forward": "1\x00", // Null byte in sysctl value
				},
			},
		}

		err = rv.validateSecurity()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "security validation failed for sysctl value")
	})
}
