package config

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestValidatePathChars(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"safe/path", false},
		{"safe path", false},
		{"path/with/\t/tab", false},
		{"path/with/\n/newline", false},
		{"path/with/\r/return", false},
		{"path/with/\x00/null", true},
		{"path/with/\x1b/escape", true},
		{"path/with/\x7f/delete", true},
	}

	for _, tt := range tests {
		err := validatePathChars(tt.input)
		if tt.wantErr {
			assert.Error(t, err, "input: %q", tt.input)
		} else {
			assert.NoError(t, err, "input: %q", tt.input)
		}
	}
}

func TestValidateToolName(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"tool", false},
		{"tool-name", false},
		{"tool_name", false},
		{"", true},
		{"/abs/path", true},
		{"../parent", true},
		{"subdir/tool", true},
		{"subdir\\tool", true},
	}

	for _, tt := range tests {
		err := ValidateToolName(tt.input)
		if tt.wantErr {
			assert.Error(t, err, "input: %q", tt.input)
		} else {
			assert.NoError(t, err, "input: %q", tt.input)
		}
	}
}
