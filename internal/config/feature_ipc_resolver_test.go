package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Resolver_Ipc_Precedence(t *testing.T) {
	t.Parallel()

	t.Run("P1 cderun-ipc takes highest priority over P2 ipc", func(t *testing.T) {
		mfs := &MockFileSystem{}
		cli := &CLIOptions{
			Image:     ptr("alpine"),
			Ipc:       ptr("private"),
			CderunIpc: ptr("host"),
		}

		res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, "host", res.Ipc)
	})

	t.Run("P2 ipc takes priority over Env and Configs", func(t *testing.T) {
		mfs := &MockFileSystem{
			Env: map[string]string{
				"CDERUN_IPC": "host",
			},
		}
		cli := &CLIOptions{
			Image: ptr("alpine"),
			Ipc:   ptr("private"),
		}
		tools := ToolsConfig{
			"sh": ToolConfig{Ipc: "host"},
		}
		global := &CDERunConfig{
			Defaults: ConfigDefaults{Ipc: "host"},
		}

		res, err := ResolveWithFS("sh", cli, tools, global, mfs)
		require.NoError(t, err)
		assert.Equal(t, "private", res.Ipc)
	})

	t.Run("CDERUN_IPC env takes priority over tool and global config", func(t *testing.T) {
		mfs := &MockFileSystem{
			Env: map[string]string{
				"CDERUN_IPC": "host",
			},
		}
		cli := &CLIOptions{Image: ptr("alpine")}
		tools := ToolsConfig{
			"sh": ToolConfig{Ipc: "private"},
		}
		global := &CDERunConfig{
			Defaults: ConfigDefaults{Ipc: "private"},
		}

		res, err := ResolveWithFS("sh", cli, tools, global, mfs)
		require.NoError(t, err)
		assert.Equal(t, "host", res.Ipc)
	})

	t.Run("Tool config takes priority over global config", func(t *testing.T) {
		mfs := &MockFileSystem{}
		cli := &CLIOptions{Image: ptr("alpine")}
		tools := ToolsConfig{
			"sh": ToolConfig{Ipc: "host"},
		}
		global := &CDERunConfig{
			Defaults: ConfigDefaults{Ipc: "private"},
		}

		res, err := ResolveWithFS("sh", cli, tools, global, mfs)
		require.NoError(t, err)
		assert.Equal(t, "host", res.Ipc)
	})

	t.Run("Global config fallback is used when others are empty", func(t *testing.T) {
		mfs := &MockFileSystem{}
		cli := &CLIOptions{Image: ptr("alpine")}
		global := &CDERunConfig{
			Defaults: ConfigDefaults{Ipc: "private"},
		}

		res, err := ResolveWithFS("sh", cli, nil, global, mfs)
		require.NoError(t, err)
		assert.Equal(t, "private", res.Ipc)
	})

	t.Run("Default fallback is empty", func(t *testing.T) {
		mfs := &MockFileSystem{}
		cli := &CLIOptions{Image: ptr("alpine")}

		res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, "", res.Ipc)
	})

	t.Run("Validation rejects invalid IPC format", func(t *testing.T) {
		mfs := &MockFileSystem{}
		cli := &CLIOptions{
			Image: ptr("alpine"),
			Ipc:   ptr("invalid-namespace-mode"),
		}

		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "security validation failed for \"ipc\"")
		assert.Contains(t, err.Error(), "unsupported ipc namespace")
	})

	t.Run("Validation allows valid IPC formats", func(t *testing.T) {
		validModes := []string{"host", "private", "shareable", "none", "container:some_id"}
		mfs := &MockFileSystem{}

		for _, mode := range validModes {
			cli := &CLIOptions{
				Image: ptr("alpine"),
				Ipc:   ptr(mode),
			}
			res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
			require.NoError(t, err)
			assert.Equal(t, mode, res.Ipc)
		}
	})
}
