package config

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestMaskSensitiveEnv(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		value    string
		expected string
	}{
		{"Safe variable", "SAFE_VAR", "value", "value"},
		{"Standard snake_case password", "MY_PASSWORD", "secret", "[REDACTED]"},
		{"Standard snake_case secret", "DB_SECRET_VAR", "secret", "[REDACTED]"},
		{"Standard snake_case token", "AUTH_TOKEN", "token", "[REDACTED]"},
		{"Standard snake_case key", "PRIVATE_KEY", "key", "[REDACTED]"},
		{"Standard snake_case auth", "API_AUTH_VAR", "auth", "[REDACTED]"},
		{"Standard snake_case sig", "SIGNAL_SIG", "sig", "[REDACTED]"},
		{"False positive MONKEY", "MONKEY", "value", "value"},
		{"False positive KEYAKI", "KEYAKI", "value", "value"},
		{"False positive KEYWORD", "KEYWORD", "value", "value"},
		{"CamelCase password", "dbPassword", "secret", "[REDACTED]"},
		{"CamelCase token", "apiToken", "secret", "[REDACTED]"},
		{"CamelCase key", "appKey", "secret", "[REDACTED]"},
		{"Acronym APIKey", "APIKey", "secret", "[REDACTED]"},
		{"Acronym DBPassword", "DBPassword", "secret", "[REDACTED]"},
		{"Acronym SSHKey", "SSHKey", "secret", "[REDACTED]"},
		{"Already masked value", "APPKEY", "[REDACTED]", "[REDACTED]"},
		{"Empty value", "EMPTY_SECRET", "", ""},
		{"Lowercase password", "my_password", "secret", "[REDACTED]"},
		{"Letter-digit boundary password", "PASSWORD2", "secret", "[REDACTED]"},
		{"Digit-letter boundary key", "1KEY", "secret", "[REDACTED]"},
		{"Letter-digit camel boundary", "dbPassword2", "secret", "[REDACTED]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MaskSensitiveEnv(tt.key, tt.value)
			assert.Equal(t, tt.expected, got, "key: %s", tt.key)
		})
	}
}

func TestMaskSensitiveEnvList(t *testing.T) {
	env := []string{"SAFE=VALUE", "MY_PASSWORD=secret", "NO_EQUALS"}
	orig := append([]string(nil), env...)
	expected := []string{"SAFE=VALUE", "MY_PASSWORD=[REDACTED]", "NO_EQUALS"}

	got := MaskSensitiveEnvList(env)
	assert.Equal(t, expected, got)
	// Verify non-destructive behavior
	assert.Equal(t, orig, env, "original slice should not be modified")
}
