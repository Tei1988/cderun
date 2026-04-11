package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScenario_Config_WrapperMode(t *testing.T) {
	// Standard Wrapper Mode: cderun node --version
	// In this mode, cderun specific flags can be passed using P1 (e.g. --cderun-tty)
	// or standard flags (e.g. --tty) if they are hoisted.
	// NOTE: Hoisting happens in internal/command (preprocessArgs).
	// Here we test the result of that hoisting in the resolver.

	mfs := &MockFileSystem{
		WD: "/work",
	}

	tools := ToolsConfig{
		"node": ToolConfig{
			Image: "node:18",
			TTY:   ptr(false),
		},
	}

	t.Run("P1 override in Wrapper Mode", func(t *testing.T) {
		cli := &CLIOptions{
			CderunTTY:    true,
			CderunTTYSet: true,
		}
		res, err := ResolveWithFS("node", cli, tools, nil, mfs)
		require.NoError(t, err)
		assert.True(t, res.TTY, "P1 should override tool config")
	})

	t.Run("P2 override in Wrapper Mode", func(t *testing.T) {
		cli := &CLIOptions{
			TTY:    true,
			TTYSet: true,
		}
		res, err := ResolveWithFS("node", cli, tools, nil, mfs)
		require.NoError(t, err)
		assert.True(t, res.TTY, "P2 should override tool config")
	})

	t.Run("P1 and P2 both present: P1 wins", func(t *testing.T) {
		cli := &CLIOptions{
			CderunTTY:    true,
			CderunTTYSet: true,
			TTY:          false,
			TTYSet:       true,
		}
		res, err := ResolveWithFS("node", cli, tools, nil, mfs)
		require.NoError(t, err)
		assert.True(t, res.TTY, "P1 should win over P2")
	})
}

func TestScenario_Config_SymlinkMode(t *testing.T) {
	// Symlink Mode: node --version (node is a symlink to cderun)
	// In this mode, only --cderun- prefixed flags should be hoisted to CLIOptions.
	// Standard flags like --tty should be treated as passthrough if they are NOT in CLIOptions.
	// Again, hoisting is in internal/command. Here we verify resolver behavior.

	mfs := &MockFileSystem{
		WD: "/work",
	}

	tools := ToolsConfig{
		"node": ToolConfig{
			Image: "node:18",
			TTY:   ptr(false),
		},
	}

	t.Run("Only P1 overrides in Symlink Mode (simulated)", func(t *testing.T) {
		// Simulate 'node --cderun-tty --tty' where --tty is for node, but --cderun-tty is for cderun.
		// Preprocessing would move --cderun-tty to CderunTTY, but keep --tty as passthrough (not in cli).
		cli := &CLIOptions{
			CderunTTY:    true,
			CderunTTYSet: true,
		}
		res, err := ResolveWithFS("node", cli, tools, nil, mfs)
		require.NoError(t, err)
		assert.True(t, res.TTY)
	})

	t.Run("Tool config default used if no cderun flag", func(t *testing.T) {
		cli := &CLIOptions{}
		res, err := ResolveWithFS("node", cli, tools, nil, mfs)
		require.NoError(t, err)
		assert.False(t, res.TTY)
	})
}
