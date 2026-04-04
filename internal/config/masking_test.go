package config

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestMaskSensitiveEnv(t *testing.T) {
	tests := []struct {
		key      string
		value    string
		expected string
	}{
		{"SAFE_VAR", "value", "value"},
		{"MY_PASSWORD", "secret", "[REDACTED]"},
		{"DB_SECRET_VAR", "secret", "[REDACTED]"},
		{"AUTH_TOKEN", "token", "[REDACTED]"},
		{"PRIVATE_KEY", "key", "[REDACTED]"},
		{"API_AUTH_VAR", "auth", "[REDACTED]"},
		{"SIGNAL_SIG", "sig", "[REDACTED]"},
		{"MONKEY", "value", "value"},
		{"KEYAKI", "value", "value"},
		{"KEYWORD", "value", "value"},
		{"APPKEY", "[REDACTED]", "[REDACTED]"}, // Although it's already masked, it should match segment
		{"EMPTY_SECRET", "", ""},
		// Case insensitive
		{"my_password", "secret", "[REDACTED]"},
	}

	for _, tt := range tests {
		got := MaskSensitiveEnv(tt.key, tt.value)
		assert.Equal(t, tt.expected, got, "key: %s", tt.key)
	}
}
