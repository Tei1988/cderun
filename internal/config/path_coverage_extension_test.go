package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_Path_ValidateToolName_Exhaustive(t *testing.T) {
	tests := []struct {
		name    string
		tool    string
		wantErr bool
	}{
		{"Empty name", "", true},
		{"Absolute path", "/usr/bin/node", true},
		{"Current directory", ".", true},
		{"Parent directory", "..", true},
		{"Valid with dot", "node.js", false},
		{"Valid with underscore", "my_tool", false},
		{"Valid with hyphen", "tool-v1", false},
		{"Invalid character space", "node js", true},
		{"Invalid character colon", "node:js", true},
		{"Invalid character slash", "node/js", true},
		{"Invalid character backslash", "node\\js", true},
		{"Valid alphanumeric", "node123", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateToolName(tt.tool)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUnit_Config_Path_ResolvePath_AbsError(t *testing.T) {
	mfs := &MockFileSystem{
		AbsErr: assert.AnError,
		WD:     "/work",
	}
	hostCtx := &HostContext{
		Level: 1,
	}
	r, err := NewExpressionResolverWithFS(hostCtx, mfs)
	require.NoError(t, err)

	_, err = ResolvePath("relative/path", "/work", r)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get absolute path")
}
