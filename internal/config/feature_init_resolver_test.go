package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Resolver_Init_Precedence(t *testing.T) {
	t.Parallel()

	t.Run("P1 cderun-init takes highest priority over P2 init", func(t *testing.T) {
		mfs := &MockFileSystem{}
		cli := &CLIOptions{
			Image:      ptr("alpine"),
			Init:       ptr(false),
			CderunInit: ptr(true),
		}

		res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)
		assert.True(t, res.Init)
	})

	t.Run("P2 init takes priority over Env and Configs", func(t *testing.T) {
		mfs := &MockFileSystem{
			Env: map[string]string{
				"CDERUN_INIT": "true",
			},
		}
		cli := &CLIOptions{
			Image: ptr("alpine"),
			Init:  ptr(false),
		}
		tools := ToolsConfig{
			"sh": ToolConfig{Init: ptr(true)},
		}
		global := &CDERunConfig{
			Defaults: ConfigDefaults{Init: ptr(true)},
		}

		res, err := ResolveWithFS("sh", cli, tools, global, mfs)
		require.NoError(t, err)
		assert.False(t, res.Init)
	})

	t.Run("CDERUN_INIT env takes priority over tool and global config", func(t *testing.T) {
		mfs := &MockFileSystem{
			Env: map[string]string{
				"CDERUN_INIT": "true",
			},
		}
		cli := &CLIOptions{Image: ptr("alpine")}
		tools := ToolsConfig{
			"sh": ToolConfig{Init: ptr(false)},
		}
		global := &CDERunConfig{
			Defaults: ConfigDefaults{Init: ptr(false)},
		}

		res, err := ResolveWithFS("sh", cli, tools, global, mfs)
		require.NoError(t, err)
		assert.True(t, res.Init)
	})

	t.Run("Tool config takes priority over global config", func(t *testing.T) {
		mfs := &MockFileSystem{}
		cli := &CLIOptions{Image: ptr("alpine")}
		tools := ToolsConfig{
			"sh": ToolConfig{Init: ptr(true)},
		}
		global := &CDERunConfig{
			Defaults: ConfigDefaults{Init: ptr(false)},
		}

		res, err := ResolveWithFS("sh", cli, tools, global, mfs)
		require.NoError(t, err)
		assert.True(t, res.Init)
	})

	t.Run("Global config fallback is used when others are empty", func(t *testing.T) {
		mfs := &MockFileSystem{}
		cli := &CLIOptions{Image: ptr("alpine")}
		global := &CDERunConfig{
			Defaults: ConfigDefaults{Init: ptr(true)},
		}

		res, err := ResolveWithFS("sh", cli, nil, global, mfs)
		require.NoError(t, err)
		assert.True(t, res.Init)
	})

	t.Run("Default fallback is false", func(t *testing.T) {
		mfs := &MockFileSystem{}
		cli := &CLIOptions{Image: ptr("alpine")}

		res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)
		assert.False(t, res.Init)
	})
}
