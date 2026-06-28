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
		patterns []string
		expected string
	}{
		{"Explicit pattern match", "API_KEY_PASSWORD", "secret", []string{"*PASSWORD*"}, "[REDACTED]"},
		{"Mask all by default (nil)", "OAuthToken", "token", nil, "[REDACTED]"},
		{"Mask none with empty patterns", "OAuthToken", "token", []string{}, "token"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MaskSensitiveEnv(tt.key, tt.value, tt.patterns)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestUnit_Config_MaskSensitiveEnvList_Nil(t *testing.T) {
	assert.Nil(t, MaskSensitiveEnvList(nil, nil))
}
