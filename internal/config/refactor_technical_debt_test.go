package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	inTest = true
}

func TestUnit_Refactor_DriftGuards(t *testing.T) {
	// First, let's verify that the expected indices match the current indices.
	initFieldInfo()

	for name, info := range fieldInfo {
		expected, exists := expectedFieldIndices[name]
		require.True(t, exists, "expected index for %s should exist", name)
		assert.Equal(t, info.p1ValIdx, expected.p1ValIdx, "p1 index drift for option: %s", name)
		assert.Equal(t, info.p2ValIdx, expected.p2ValIdx, "p2 index drift for option: %s", name)
	}
}

func TestUnit_Refactor_resolveBoolOption(t *testing.T) {
	cli := &CLIOptions{}
	tools := ToolsConfig{}
	global := &CDERunConfig{}
	fs := &MockFileSystem{}

	rv := &resolver{
		subcommand: "test",
		cli:        cli,
		tools:      tools,
		global:     global,
		fs:         fs,
		res:        &ResolvedConfig{},
	}

	// Test non-existent option
	_, err := rv.resolveBoolOption("non-existent-option", nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "registry mismatch")

	_, _, err = rv.resolveBoolOptionInfo("non-existent-option", nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "registry mismatch")

	// Test valid option
	val := true
	res, err := rv.resolveBoolOption("diagnosis", &val, nil)
	require.NoError(t, err)
	assert.True(t, res)

	res, spec, err := rv.resolveBoolOptionInfo("diagnosis", nil, &val)
	require.NoError(t, err)
	assert.True(t, res)
	assert.True(t, spec)
}