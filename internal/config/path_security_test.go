package config

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestValidatePathChars(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"Safe path", "safe/path", false},
		{"Path with space", "safe path", false},
		{"Path with tab (control)", "path/with/\t/tab", true},
		{"Path with newline (control)", "path/with/\n/newline", true},
		{"Path with carriage return (control)", "path/with/\r/return", true},
		{"Path with null byte", "path/with/\x00/null", true},
		{"Path with escape char", "path/with/\x1b/escape", true},
		{"Path with delete char", "path/with/\x7f/delete", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePathChars(tt.input)
			if tt.wantErr {
				require.Error(t, err, "input: %q", tt.input)
			} else {
				require.NoError(t, err, "input: %q", tt.input)
			}
		})
	}
}

func TestValidateToolName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"Basic tool name", "tool", false},
		{"Tool name with hyphen", "tool-name", false},
		{"Tool name with underscore", "tool_name", false},
		{"Tool name with double dot", "tool..name", false},
		{"Empty tool name", "", true},
		{"Absolute path tool name", "/abs/path", true},
		{"Parent directory traversal", "../parent", true},
		{"Subdirectory tool name (Linux)", "subdir/tool", true},
		{"Subdirectory tool name (Windows)", "subdir\\tool", true},
		{"Control character in tool name", "tool\tname", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateToolName(tt.input)
			if tt.wantErr {
				require.Error(t, err, "input: %q", tt.input)
			} else {
				require.NoError(t, err, "input: %q", tt.input)
			}
		})
	}
}

func TestUnit_Config_Mount_AbsoluteTarget(t *testing.T) {
	t.Parallel()
	r := &ExpressionResolver{Pwd: "/host"}

	t.Run("absolute target is accepted", func(t *testing.T) {
		mc := MountConfig{
			Type:   "bind",
			Source: ConfigPath{Raw: "/src"},
			Target: ConfigPath{Raw: "/tgt"},
		}
		m, err := mc.Resolve(r)
		require.NoError(t, err)
		assert.Equal(t, "/tgt", m.Target)
	})

	t.Run("relative target is rejected", func(t *testing.T) {
		mc := MountConfig{
			Type:   "bind",
			Source: ConfigPath{Raw: "/src"},
			Target: ConfigPath{Raw: "relative/path"},
		}
		_, err := mc.Resolve(r)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "mount target must be an absolute path")
	})
}
