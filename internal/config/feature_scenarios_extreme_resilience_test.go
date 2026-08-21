package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUnit_Config_Validators_ExtremeEdgeCases(t *testing.T) {
	t.Run("ValidateDNSOption edge cases", func(t *testing.T) {
		require.NoError(t, ValidateDNSOption(""))
		require.NoError(t, ValidateDNSOption("ndots:5"))
		require.NoError(t, ValidateDNSOption("timeout:2"))
		require.NoError(t, ValidateDNSOption("attempts:3.5"))

		require.Error(t, ValidateDNSOption("ndots:5/.."))
		require.Error(t, ValidateDNSOption("ndots:5\n"))
		require.Error(t, ValidateDNSOption("ndots:5\x00"))
		require.Error(t, ValidateDNSOption("ndots:5; rm -rf /"))
	})

	t.Run("ValidateSecurityOpt edge cases", func(t *testing.T) {
		require.NoError(t, ValidateSecurityOpt(""))
		require.NoError(t, ValidateSecurityOpt("no-new-privileges:true"))
		require.NoError(t, ValidateSecurityOpt("seccomp=unconfined"))
		require.NoError(t, ValidateSecurityOpt("apparmor:unconfined"))
		require.NoError(t, ValidateSecurityOpt("label=disable"))

		// Traversal-specific validation cases (character allowlisting passes, rejected by HasParentTraversal)
		require.Error(t, ValidateSecurityOpt("seccomp=/path/to/../profile"))
		require.Error(t, ValidateSecurityOpt("profile=/etc/apparmor.d/../unconfined"))

		require.Error(t, ValidateSecurityOpt("no-new-privileges\n"))
		require.Error(t, ValidateSecurityOpt("seccomp=\x00unconfined"))
		require.Error(t, ValidateSecurityOpt("apparmor; injection"))
	})

	t.Run("ValidateSysctlKey edge cases", func(t *testing.T) {
		require.Error(t, ValidateSysctlKey(""))
		require.NoError(t, ValidateSysctlKey("net.ipv4.ip_forward"))
		require.NoError(t, ValidateSysctlKey("kernel.shmmax"))

		require.Error(t, ValidateSysctlKey(".net.ipv4"))
		require.Error(t, ValidateSysctlKey("net.ipv4."))
		require.Error(t, ValidateSysctlKey("net..ipv4"))
		require.Error(t, ValidateSysctlKey("net.ipv4\n"))
		require.Error(t, ValidateSysctlKey("net.ipv4\x00"))
		require.Error(t, ValidateSysctlKey("net.ipv4; injection"))
	})

	t.Run("ValidateSysctlValue edge cases", func(t *testing.T) {
		require.NoError(t, ValidateSysctlValue(""))
		require.NoError(t, ValidateSysctlValue("1"))
		require.NoError(t, ValidateSysctlValue("1024 2048 4096"))
		require.NoError(t, ValidateSysctlValue("val-1,val-2"))

		require.Error(t, ValidateSysctlValue("1\n"))
		require.Error(t, ValidateSysctlValue("1\x00"))
		require.Error(t, ValidateSysctlValue("1; rm -rf /"))
	})

	t.Run("ValidateGPUs edge cases", func(t *testing.T) {
		require.NoError(t, ValidateGPUs(""))
		require.NoError(t, ValidateGPUs("all"))
		require.NoError(t, ValidateGPUs("device=0,1"))
		require.NoError(t, ValidateGPUs("count=2"))

		require.Error(t, ValidateGPUs("all\n"))
		require.Error(t, ValidateGPUs("device=0\x00"))
		require.Error(t, ValidateGPUs("device=0; injection"))
	})

	t.Run("ValidateCpuset edge cases", func(t *testing.T) {
		require.NoError(t, ValidateCpuset(""))
		require.NoError(t, ValidateCpuset("0-3"))
		require.NoError(t, ValidateCpuset("0,1,2,3"))
		require.NoError(t, ValidateCpuset("0-3,6,7"))

		require.Error(t, ValidateCpuset("0-3\n"))
		require.Error(t, ValidateCpuset("0-3\x00"))
		require.Error(t, ValidateCpuset("0-3; injection"))
	})

	t.Run("ValidateHostname edge cases", func(t *testing.T) {
		require.NoError(t, ValidateHostname(""))
		require.NoError(t, ValidateHostname("my-host"))
		require.NoError(t, ValidateHostname("host.domain.com"))

		require.Error(t, ValidateHostname("host_name")) // underscore not allowed in standard hostname label
		require.Error(t, ValidateHostname("host\n"))
		require.Error(t, ValidateHostname("host\x00"))
	})

	t.Run("ValidateGroupAdd edge cases", func(t *testing.T) {
		require.NoError(t, ValidateGroupAdd(""))
		require.NoError(t, ValidateGroupAdd("1000"))
		require.NoError(t, ValidateGroupAdd("docker"))
		require.NoError(t, ValidateGroupAdd("group-name_1"))

		require.Error(t, ValidateGroupAdd("group\n"))
		require.Error(t, ValidateGroupAdd("group\x00"))
		require.Error(t, ValidateGroupAdd("group; injection"))
	})

	t.Run("ValidateImageName edge cases", func(t *testing.T) {
		require.NoError(t, ValidateImageName(""))
		require.NoError(t, ValidateImageName("alpine"))
		require.NoError(t, ValidateImageName("alpine:latest"))
		require.NoError(t, ValidateImageName("docker.io/library/alpine:3.18"))
		require.NoError(t, ValidateImageName("my-registry:5000/my-image@sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"))

		// Path traversal-like image name cases
		require.Error(t, ValidateImageName("repo/../alpine"))

		require.Error(t, ValidateImageName("alpine\n"))
		require.Error(t, ValidateImageName("alpine\x00"))
		require.Error(t, ValidateImageName("alpine; injection"))
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
