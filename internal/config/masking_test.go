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
		patterns []string
		expected string
	}{
		{"Unset patterns (Mask All) - safe", "SAFE_VAR", "value", nil, "[REDACTED]"},
		{"Unset patterns (Mask All) - secret", "MY_PASSWORD", "secret", nil, "[REDACTED]"},
		{"Empty patterns (Mask None) - safe", "SAFE_VAR", "value", []string{}, "value"},
		{"Empty patterns (Mask None) - secret", "MY_PASSWORD", "secret", []string{}, "secret"},
		{"Exact match", "MY_PASSWORD", "secret", []string{"MY_PASSWORD"}, "[REDACTED]"},
		{"Exact match case-insensitive", "my_password", "secret", []string{"MY_PASSWORD"}, "[REDACTED]"},
		{"Glob match start", "DB_PASSWORD", "secret", []string{"DB_*"}, "[REDACTED]"},
		{"Glob match end", "MY_PASSWORD", "secret", []string{"*_PASSWORD"}, "[REDACTED]"},
		{"Glob match end non-ASCII", "MY_パスワード", "secret", []string{"*_パスワード"}, "[REDACTED]"},
		{"Glob match start non-ASCII", "パスワード_MY", "secret", []string{"パスワード_*"}, "[REDACTED]"},
		{"Glob match middle", "MY_PASSWORD_VAR", "secret", []string{"*_PASSWORD_*"}, "[REDACTED]"},
		{"No match", "SAFE_VAR", "value", []string{"*_PASSWORD"}, "value"},
		{"Empty value", "EMPTY_SECRET", "", []string{"*"}, ""},
		{"Invalid glob pattern (fail-closed)", "SECRET", "value", []string{"["}, "[REDACTED]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MaskSensitiveEnv(tt.key, tt.value, tt.patterns)
			assert.Equal(t, tt.expected, got, "key: %s", tt.key)
		})
	}
}

func TestMaskSensitiveEnvList(t *testing.T) {
	env := []string{"SAFE=VALUE", "MY_PASSWORD=secret", "NO_EQUALS"}
	orig := append([]string(nil), env...)

	t.Run("Mask all by default (nil patterns)", func(t *testing.T) {
		expected := []string{"SAFE=[REDACTED]", "MY_PASSWORD=[REDACTED]", "NO_EQUALS"}
		got := MaskSensitiveEnvList(env, nil)
		assert.Equal(t, expected, got)
	})

	t.Run("Mask none with empty patterns", func(t *testing.T) {
		expected := []string{"SAFE=VALUE", "MY_PASSWORD=secret", "NO_EQUALS"}
		got := MaskSensitiveEnvList(env, []string{})
		assert.Equal(t, expected, got)
	})

	t.Run("Mask patterns", func(t *testing.T) {
		expected := []string{"SAFE=VALUE", "MY_PASSWORD=[REDACTED]", "NO_EQUALS"}
		got := MaskSensitiveEnvList(env, []string{"*_PASSWORD"})
		assert.Equal(t, expected, got)
	})

	// Verify non-destructive behavior
	assert.Equal(t, orig, env, "original slice should not be modified")
}
