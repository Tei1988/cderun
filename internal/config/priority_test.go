package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Priority_ListFallback(t *testing.T) {
	global := &CDERunConfig{
		Defaults: ConfigDefaults{
			Env: []string{"GLOBAL=1"},
			Mounts: []MountConfig{
				{Type: "bind", Source: ConfigPath{Raw: "/global"}, Target: ConfigPath{Raw: "/global"}},
			},
		},
	}

	subcommand := "test-tool"

	t.Run("nil at higher level falls through", func(t *testing.T) {
		// P4 Tool Config is nil for Env and Mounts
		tools := ToolsConfig{
			subcommand: ToolConfig{
				Image:  "alpine",
				Env:    nil,
				Mounts: nil,
			},
		}
		cli := CLIOptions{}
		res, err := Resolve(subcommand, cli, tools, global)
		require.NoError(t, err)

		// Should fall through to P5 Global Defaults
		assert.Equal(t, []string{"GLOBAL=1"}, res.Env)
		assert.Len(t, res.Mounts, 1)
		assert.Equal(t, "/global", res.Mounts[0].Source)
	})

	t.Run("empty slice at higher level overrides", func(t *testing.T) {
		// P4 Tool Config has explicit empty slices
		tools := ToolsConfig{
			subcommand: ToolConfig{
				Image:  "alpine",
				Env:    []string{},
				Mounts: []MountConfig{},
			},
		}
		cli := CLIOptions{}
		res, err := Resolve(subcommand, cli, tools, global)
		require.NoError(t, err)

		// Should NOT fall through to P5
		assert.Empty(t, res.Env)
		assert.Empty(t, res.Mounts)
	})

	t.Run("non-empty slice at higher level overrides", func(t *testing.T) {
		// P4 Tool Config has non-empty slices
		tools := ToolsConfig{
			subcommand: ToolConfig{
				Image: "alpine",
				Env:   []string{"TOOL=1"},
				Mounts: []MountConfig{
					{Type: "bind", Source: ConfigPath{Raw: "/tool"}, Target: ConfigPath{Raw: "/tool"}},
				},
			},
		}
		cli := CLIOptions{}
		res, err := Resolve(subcommand, cli, tools, global)
		require.NoError(t, err)

		// Should NOT fall through to P5, uses P4
		assert.Equal(t, []string{"TOOL=1"}, res.Env)
		assert.Len(t, res.Mounts, 1)
		assert.Equal(t, "/tool", res.Mounts[0].Source)
	})

	t.Run("P1 overrides everything including empty P2", func(t *testing.T) {
		tools := ToolsConfig{
			subcommand: ToolConfig{Image: "alpine"},
		}
		cli := CLIOptions{
			Env:       []string{"P2=1"}, // P2 non-empty
			CderunEnv: []string{"P1=1"}, // P1 non-empty
		}
		res, err := Resolve(subcommand, cli, tools, global)
		require.NoError(t, err)
		assert.Equal(t, []string{"P1=1"}, res.Env)

		cli = CLIOptions{
			Env:       []string{"P2=1"},
			CderunEnv: []string{}, // P1 empty slice -> should win
		}
		res, err = Resolve(subcommand, cli, tools, global)
		require.NoError(t, err)
		assert.Empty(t, res.Env)
	})
}
