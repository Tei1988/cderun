package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnit_Config_MaskSensitiveEnv_Advanced(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		value    string
		expected string
	}{
		{"Multiple sensitive keywords", "API_KEY_PASSWORD", "secret", "[REDACTED]"},
		{"Complex camelCase", "OAuthToken", "token", "[REDACTED]"},
		{"Acronym and CamelCase", "DBPasswordAcronym", "secret", "[REDACTED]"},
		{"Mixed snake and camel", "MY_appPassword", "secret", "[REDACTED]"},
		{"Multiple acronyms", "SSH_API_KEY", "secret", "[REDACTED]"},
		{"Acronym at the end", "MySSH", "value", "value"}, // SSH not a keyword
		{"Acronym at the end with keyword", "MySSHKey", "value", "[REDACTED]"},
		{"Single letter segments", "A_B_C_KEY", "value", "[REDACTED]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MaskSensitiveEnv(tt.key, tt.value)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestUnit_Config_MaskSensitiveEnvList_Nil(t *testing.T) {
	assert.Nil(t, MaskSensitiveEnvList(nil))
}
