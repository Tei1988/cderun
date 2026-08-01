package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_Resolve_InitOption(t *testing.T) {
	t.Parallel()

	ptr := func(v bool) *bool { return &v }
	strPtr := func(v string) *string { return &v }

	t.Run("P1 Override cderun-init takes priority over P2 CLI init", func(t *testing.T) {
		cli := CLIOptions{
			Image:      strPtr("alpine"),
			Init:       ptr(true),
			CderunInit: ptr(false),
		}
		res, err := Resolve("node", &cli, nil, nil)
		require.NoError(t, err)
		assert.False(t, res.Init)
	})

	t.Run("P2 CLI init takes priority over P3 Env CDERUN_INIT", func(t *testing.T) {
		mfs := &MockFileSystem{
			Env: map[string]string{"CDERUN_INIT": "false"},
		}
		cli := CLIOptions{
			Image: strPtr("alpine"),
			Init:  ptr(true),
		}
		res, err := ResolveWithFS("node", &cli, nil, nil, mfs)
		require.NoError(t, err)
		assert.True(t, res.Init)
	})

	t.Run("P3 Env CDERUN_INIT takes priority over P4 Tool init", func(t *testing.T) {
		mfs := &MockFileSystem{
			Env: map[string]string{"CDERUN_INIT": "true"},
		}
		tools := ToolsConfig{
			"node": ToolConfig{
				Image: "node",
				Init:  ptr(false),
			},
		}
		res, err := ResolveWithFS("node", &CLIOptions{}, tools, nil, mfs)
		require.NoError(t, err)
		assert.True(t, res.Init)
	})

	t.Run("P4 Tool init takes priority over P5 Global init", func(t *testing.T) {
		tools := ToolsConfig{
			"node": ToolConfig{
				Image: "node",
				Init:  ptr(true),
			},
		}
		global := &CDERunConfig{
			Defaults: ConfigDefaults{
				Init: ptr(false),
			},
		}
		res, err := Resolve("node", &CLIOptions{}, tools, global)
		require.NoError(t, err)
		assert.True(t, res.Init)
	})

	t.Run("P5 Global init works", func(t *testing.T) {
		cli := CLIOptions{
			Image: strPtr("alpine"),
		}
		global := &CDERunConfig{
			Defaults: ConfigDefaults{
				Init:  ptr(true),
			},
		}
		res, err := Resolve("node", &cli, nil, global)
		require.NoError(t, err)
		assert.True(t, res.Init)
	})
}
