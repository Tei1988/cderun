package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnit_Validators_ExtremeEdgeCases(t *testing.T) {
	t.Run("ValidateToolName edge cases", func(t *testing.T) {
		validTools := []string{"python", "node-js", "go_1.21", "tool.v2", "-leadinghyphen"}
		for _, tool := range validTools {
			assert.NoError(t, ValidateToolName(tool), "expected valid tool name: %s", tool)
		}

		invalidTools := []string{
			"",             // empty
			".",            // dot
			"..",           // parent traversal
			"../tool",      // path traversal
			"tool/sub",     // slash
			"tool\\sub",    // backslash
			"tool\x00name", // null byte
			"tool\x1b[31m", // ANSI escape control sequence
			"tool name",    // space
			"tool:name",    // colon
		}
		for _, tool := range invalidTools {
			assert.Error(t, ValidateToolName(tool), "expected invalid tool name: %s", tool)
		}
	})

	t.Run("ValidateSysctlKey edge cases", func(t *testing.T) {
		validKeys := []string{"net.ipv4.ip_forward", "kernel.shmmax", "fs.file-max"}
		for _, key := range validKeys {
			assert.NoError(t, ValidateSysctlKey(key), "expected valid sysctl key: %s", key)
		}

		invalidKeys := []string{
			"",                   // empty
			".net.ipv4",          // leading dot
			"net.ipv4.",          // trailing dot
			"net..ipv4",          // consecutive dots
			"net.ipv4\x00key",    // null byte
			"net.ipv4;rm -rf /", // command injection
		}
		for _, key := range invalidKeys {
			assert.Error(t, ValidateSysctlKey(key), "expected invalid sysctl key: %s", key)
		}
	})

	t.Run("ValidateSysctlValue edge cases", func(t *testing.T) {
		validVals := []string{"", "1", "0", "1024 65535", "enabled"}
		for _, val := range validVals {
			assert.NoError(t, ValidateSysctlValue(val), "expected valid sysctl value: %s", val)
		}

		invalidVals := []string{
			"val\x00ue",   // null byte
			"val\x1b[31m", // control char
			"val;cmd",     // forbidden char
		}
		for _, val := range invalidVals {
			assert.Error(t, ValidateSysctlValue(val), "expected invalid sysctl value: %s", val)
		}
	})

	t.Run("ValidateDNSOption edge cases", func(t *testing.T) {
		validOpts := []string{"", "ndots:5", "timeout:2", "attempts:3", "use-vc", "edns0"}
		for _, opt := range validOpts {
			assert.NoError(t, ValidateDNSOption(opt), "expected valid dns option: %s", opt)
		}

		invalidOpts := []string{
			"ndots:5\x00",  // null byte
			"ndots:5\n",    // newline
			"ndots:5;drop", // forbidden character
		}
		for _, opt := range invalidOpts {
			assert.Error(t, ValidateDNSOption(opt), "expected invalid dns option: %s", opt)
		}
	})

	t.Run("ValidateSecurityOpt edge cases", func(t *testing.T) {
		validOpts := []string{"", "seccomp=unconfined", "no-new-privileges:true", "apparmor=unconfined", "label=disable"}
		for _, opt := range validOpts {
			assert.NoError(t, ValidateSecurityOpt(opt), "expected valid security opt: %s", opt)
		}

		invalidOpts := []string{
			"seccomp\x00path",      // null byte
			"seccomp=unconfined\r", // control char
			"seccomp=unconfined;cmd",
		}
		for _, opt := range invalidOpts {
			assert.Error(t, ValidateSecurityOpt(opt), "expected invalid security opt: %s", opt)
		}
	})
}
