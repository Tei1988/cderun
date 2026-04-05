package config

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUnit_Security_ValidatePathChars(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid", "alpine:latest", false},
		{"valid with path", "/usr/local/bin/tool", false},
		{"invalid null byte", "alpine\x00latest", true},
		{"invalid newline", "alpine\nlatest", true},
		{"invalid carriage return", "alpine\rlatest", true},
		{"invalid tab", "alpine\tlatest", true},
		{"invalid backspace", "alpine\blatest", true},
		{"invalid escape", "alpine\x1blatest", true},
		{"invalid delete", "alpine\x7flatest", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePathChars(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestUnit_Security_ValidateToolName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid simple", "node", false},
		{"valid nested", "my/tool", false},
		{"valid with dot", "tool.v1", false},
		{"invalid traversal", "../tool", true},
		{"invalid nested traversal", "my/../tool", true},
		{"invalid control char", "tool\nname", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateToolName(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestUnit_Security_MaskSensitiveEnv(t *testing.T) {
	tests := []struct {
		key      string
		value    string
		expected string
	}{
		{"API_KEY", "secret123", "********"},
		{"DB_PASSWORD", "mypass", "********"},
		{"GITHUB_TOKEN", "ghp_abc", "********"},
		{"APP_SECRET", "supersecret", "********"},
		{"AWS_ACCESS_KEY_ID", "AKIA...", "********"},
		{"APIKey", "secret", "********"},
		{"myApiKey", "secret", "********"},
		{"USER_NAME", "jules", "jules"},
		{"LOG_LEVEL", "debug", "debug"},
		{"PATH", "/usr/bin", "/usr/bin"},
		{"EMPTY", "", ""},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s/%s", tt.key, tt.value), func(t *testing.T) {
			require.Equal(t, tt.expected, MaskSensitiveEnv(tt.key, tt.value))
		})
	}
}

func TestUnit_Security_MaskSensitiveEnvList(t *testing.T) {
	env := []string{
		"USER=jules",
		"API_KEY=secret",
		"PASSWORD=mypass",
		"DEBUG=true",
		"TOKEN=ghp_123",
	}
	expected := []string{
		"USER=jules",
		"API_KEY=********",
		"PASSWORD=********",
		"DEBUG=true",
		"TOKEN=********",
	}

	masked := MaskSensitiveEnvList(env)
	require.Equal(t, expected, masked)
}
