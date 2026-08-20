package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_Validators_ExtremeEdgeCases(t *testing.T) {
	t.Run("ValidateDNSOption edge cases", func(t *testing.T) {
		assert.NoError(t, ValidateDNSOption(""))
		assert.NoError(t, ValidateDNSOption("ndots:5"))
		assert.NoError(t, ValidateDNSOption("timeout:2"))
		assert.NoError(t, ValidateDNSOption("attempts:3.5"))

		assert.Error(t, ValidateDNSOption("ndots:5/.."))
		assert.Error(t, ValidateDNSOption("ndots:5\n"))
		assert.Error(t, ValidateDNSOption("ndots:5\x00"))
		assert.Error(t, ValidateDNSOption("ndots:5; rm -rf /"))
	})

	t.Run("ValidateSecurityOpt edge cases", func(t *testing.T) {
		assert.NoError(t, ValidateSecurityOpt(""))
		assert.NoError(t, ValidateSecurityOpt("no-new-privileges:true"))
		assert.NoError(t, ValidateSecurityOpt("seccomp=unconfined"))
		assert.NoError(t, ValidateSecurityOpt("apparmor:unconfined"))
		assert.NoError(t, ValidateSecurityOpt("label=disable"))

		assert.Error(t, ValidateSecurityOpt("no-new-privileges\n"))
		assert.Error(t, ValidateSecurityOpt("seccomp=\x00unconfined"))
		assert.Error(t, ValidateSecurityOpt("apparmor; injection"))
	})

	t.Run("ValidateSysctlKey edge cases", func(t *testing.T) {
		assert.Error(t, ValidateSysctlKey(""))
		assert.NoError(t, ValidateSysctlKey("net.ipv4.ip_forward"))
		assert.NoError(t, ValidateSysctlKey("kernel.shmmax"))

		assert.Error(t, ValidateSysctlKey(".net.ipv4"))
		assert.Error(t, ValidateSysctlKey("net.ipv4."))
		assert.Error(t, ValidateSysctlKey("net..ipv4"))
		assert.Error(t, ValidateSysctlKey("net.ipv4\n"))
		assert.Error(t, ValidateSysctlKey("net.ipv4\x00"))
		assert.Error(t, ValidateSysctlKey("net.ipv4; injection"))
	})

	t.Run("ValidateSysctlValue edge cases", func(t *testing.T) {
		assert.NoError(t, ValidateSysctlValue(""))
		assert.NoError(t, ValidateSysctlValue("1"))
		assert.NoError(t, ValidateSysctlValue("1024 2048 4096"))
		assert.NoError(t, ValidateSysctlValue("val-1,val-2"))

		assert.Error(t, ValidateSysctlValue("1\n"))
		assert.Error(t, ValidateSysctlValue("1\x00"))
		assert.Error(t, ValidateSysctlValue("1; rm -rf /"))
	})

	t.Run("ValidateGPUs edge cases", func(t *testing.T) {
		assert.NoError(t, ValidateGPUs(""))
		assert.NoError(t, ValidateGPUs("all"))
		assert.NoError(t, ValidateGPUs("device=0,1"))
		assert.NoError(t, ValidateGPUs("count=2"))

		assert.Error(t, ValidateGPUs("all\n"))
		assert.Error(t, ValidateGPUs("device=0\x00"))
		assert.Error(t, ValidateGPUs("device=0; injection"))
	})

	t.Run("ValidateCpuset edge cases", func(t *testing.T) {
		assert.NoError(t, ValidateCpuset(""))
		assert.NoError(t, ValidateCpuset("0-3"))
		assert.NoError(t, ValidateCpuset("0,1,2,3"))
		assert.NoError(t, ValidateCpuset("0-3,6,7"))

		assert.Error(t, ValidateCpuset("0-3\n"))
		assert.Error(t, ValidateCpuset("0-3\x00"))
		assert.Error(t, ValidateCpuset("0-3; injection"))
	})

	t.Run("ValidateHostname edge cases", func(t *testing.T) {
		assert.NoError(t, ValidateHostname(""))
		assert.NoError(t, ValidateHostname("my-host"))
		assert.NoError(t, ValidateHostname("host.domain.com"))

		assert.Error(t, ValidateHostname("host_name")) // underscore not allowed in standard hostname label
		assert.Error(t, ValidateHostname("host\n"))
		assert.Error(t, ValidateHostname("host\x00"))
	})

	t.Run("ValidateGroupAdd edge cases", func(t *testing.T) {
		assert.NoError(t, ValidateGroupAdd(""))
		assert.NoError(t, ValidateGroupAdd("1000"))
		assert.NoError(t, ValidateGroupAdd("docker"))
		assert.NoError(t, ValidateGroupAdd("group-name_1"))

		assert.Error(t, ValidateGroupAdd("group\n"))
		assert.Error(t, ValidateGroupAdd("group\x00"))
		assert.Error(t, ValidateGroupAdd("group; injection"))
	})

	t.Run("ValidateImageName edge cases", func(t *testing.T) {
		assert.NoError(t, ValidateImageName(""))
		assert.NoError(t, ValidateImageName("alpine"))
		assert.NoError(t, ValidateImageName("alpine:latest"))
		assert.NoError(t, ValidateImageName("docker.io/library/alpine:3.18"))
		assert.NoError(t, ValidateImageName("my-registry:5000/my-image@sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"))

		assert.Error(t, ValidateImageName("alpine\n"))
		assert.Error(t, ValidateImageName("alpine\x00"))
		assert.Error(t, ValidateImageName("alpine; injection"))
	})

	t.Run("ValidateEnvKey edge cases", func(t *testing.T) {
		require.Error(t, ValidateEnvKey("")) // empty key is invalid
		require.NoError(t, ValidateEnvKey("MY_VAR"))
		require.NoError(t, ValidateEnvKey("VAR123"))

		require.Error(t, ValidateEnvKey("123VAR"))  // starts with digit
		require.Error(t, ValidateEnvKey("VAR-KEY")) // contains hyphen
		require.Error(t, ValidateEnvKey("VAR\n"))
		require.Error(t, ValidateEnvKey("VAR\x00"))
	})
}
