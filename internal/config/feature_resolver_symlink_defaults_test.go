package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUnit_Config_SymlinkMode_Defaults verifies that in Symlink Mode (Polyglot Mode),
// when the subcommand name matches the tool defined in ToolsConfig, ResolveWithFS correctly
// uses that subcommand's defaults as the base values for priority merging.
func TestUnit_Config_SymlinkMode_Defaults(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{}

	// Scenario: Subcommand node is run through a symlink.
	// Only CderunImage (P1) is set, but no Image (P2) is set in cli options.
	// ToolsConfig contains default image for "node" as "node:18".
	cli := &CLIOptions{
		CderunImage: ptr("node:20"), // P1 internal override flag
	}

	tools := ToolsConfig{
		"node": ToolConfig{
			Image: "node:18", // P4 tool config
		},
	}

	t.Run("Symlink mode uses subcommand tools config as fallback", func(t *testing.T) {
		// Run with empty cli options first (representing no overrides)
		emptyCli := &CLIOptions{}
		res, err := ResolveWithFS("node", emptyCli, tools, nil, mfs)
		require.NoError(t, err)
		// Should resolve to the tool default
		assert.Equal(t, "node:18", res.Image)
	})

	t.Run("Symlink mode merges P1 override flag successfully", func(t *testing.T) {
		res, err := ResolveWithFS("node", cli, tools, nil, mfs)
		require.NoError(t, err)
		// Should resolve to P1 override image
		assert.Equal(t, "node:20", res.Image)
	})
}
