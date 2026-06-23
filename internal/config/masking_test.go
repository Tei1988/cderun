package config

import (
	"github.com/stretchr/testify/assert"
	"testing"
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
		{"CERT keyword", "MY_CERT", "value", "[REDACTED]"},
		{"PEM keyword", "PRIVATE_PEM", "value", "[REDACTED]"},
		{"PRIVATE keyword", "PRIVATE_DATA", "value", "[REDACTED]"},
		{"CREDENTIALS keyword", "AWS_CREDENTIALS", "value", "[REDACTED]"},
		{"PASSPHRASE keyword", "SSH_PASSPHRASE", "value", "[REDACTED]"},
		{"APIKEY keyword", "MY_APIKEY", "value", "[REDACTED]"},
		{"SESSION keyword", "SESSION_ID", "value", "[REDACTED]"},
		{"SIGNATURE keyword", "MY_SIGNATURE", "value", "[REDACTED]"},
		{"BEARER keyword", "BEARER_TOKEN", "value", "[REDACTED]"},
		{"OTP keyword", "MY_OTP_CODE", "value", "[REDACTED]"},
		{"SENSITIVE keyword", "SENSITIVE_DATA", "value", "[REDACTED]"},
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

func TestSensitiveKeywordsLength(t *testing.T) {
	// Regression test for T08/T23 optimization in masking.go
	// Ensure no keyword exceeds maxKeywordLen (16) used for stack-allocated buffer.
	for kw := range sensitiveKeywords {
		assert.LessOrEqual(t, len(kw), maxKeywordLen, "Keyword %q exceeds maxKeywordLen (%d)", kw, maxKeywordLen)
	}
}
