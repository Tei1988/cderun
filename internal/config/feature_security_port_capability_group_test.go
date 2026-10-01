package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUnit_Config_SecurityEnhancement_Port_Capability_Group(t *testing.T) {
	t.Run("ValidateCapability edge cases", func(t *testing.T) {
		validCaps := []string{
			"SYS_ADMIN",
			"NET_BIND_SERVICE",
			"CAP_SYS_ADMIN",
			"A",
			"A_B_C",
			"CAP123",
		}
		for _, c := range validCaps {
			require.NoError(t, ValidateCapability(c), "valid capability: %s", c)
		}

		invalidCaps := []string{
			"CAP_",
			"_SYS_ADMIN",
			"SYS_ADMIN_",
			"SYS__ADMIN",
			"sys_admin",
			"SYS-ADMIN",
			"SYS_ADMIN\x00",
			"SYS_ADMIN\n",
		}
		for _, c := range invalidCaps {
			require.Error(t, ValidateCapability(c), "invalid capability: %s", c)
		}
	})

	t.Run("ValidateGroupAdd and ValidateUserName GID/UID 32-bit uint bounds", func(t *testing.T) {
		validGroups := []string{
			"0",
			"1000",
			"4294967295",
			"dialout",
			"docker",
			"group_1",
			"group-2$",
		}
		for _, g := range validGroups {
			require.NoError(t, ValidateGroupAdd(g), "valid group: %s", g)
		}

		invalidGroups := []string{
			"4294967296",
			"18446744073709551615",
			"group\n",
			"group\x00",
		}
		for _, g := range invalidGroups {
			require.Error(t, ValidateGroupAdd(g), "invalid group: %s", g)
		}

		validUsers := []string{
			"0",
			"1000",
			"4294967295",
			"root",
			"root:0",
			"user:4294967295",
			"user:dialout",
		}
		for _, u := range validUsers {
			require.NoError(t, ValidateUserName(u), "valid user: %s", u)
		}

		invalidUsers := []string{
			"4294967296",
			"user:4294967296",
			"root:group:extra",
			"user\n",
		}
		for _, u := range invalidUsers {
			require.Error(t, ValidateUserName(u), "invalid user: %s", u)
		}
	})

	t.Run("ValidatePort range syntax and length matching", func(t *testing.T) {
		validPorts := []string{
			"8080",
			"8080-8085",
			"8080:8080",
			"8080-8085:8080-8085",
			"127.0.0.1:8080-8085:8080-8085",
			"127.0.0.1:8080:8080",
			"8080-8085/tcp",
			"8080-8085/udp",
		}
		for _, p := range validPorts {
			require.NoError(t, ValidatePort(p), "valid port: %s", p)
		}

		invalidPorts := []string{
			"8085-8080",            // range start > end
			"8080-8085:8080-8082", // range length mismatch (6 vs 3)
			"8080:8080-8082",      // single vs range length mismatch
			"8080:0",              // container port 0 not allowed
			"70000",               // out of range
			"8080-70000",          // range end out of bounds
			"8080/sctp",           // invalid protocol
			"8080\n",              // control character
		}
		for _, p := range invalidPorts {
			require.Error(t, ValidatePort(p), "invalid port: %s", p)
		}
	})
}
