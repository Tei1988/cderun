package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUnit_Config_WrapperMode_PrecedenceResolving verifies resolving priority (P1 to P6)
// on various configuration options under different combinations using ResolveWithFS.
func TestUnit_Config_WrapperMode_PrecedenceResolving(t *testing.T) {
	t.Parallel()

	// 1. P1 (internal override flag `--cderun-`) is absolute winner
	t.Run("P1 (internal override flag) wins over all other layers", func(t *testing.T) {
		mfs := &MockFileSystem{
			Env: map[string]string{
				"CDERUN_NETWORK": "p3-net",
			},
		}
		cli := &CLIOptions{
			Image:         ptr("alpine"),
			Network:       ptr("p2-net"), // P2
			CderunNetwork: ptr("p1-net"), // P1
		}
		tools := ToolsConfig{
			"sh": ToolConfig{
				Network: "p4-net", // P4
			},
		}
		global := &CDERunConfig{
			Defaults: ConfigDefaults{
				Network: "p5-net", // P5
			},
		}

		res, err := ResolveWithFS("sh", cli, tools, global, mfs)
		require.NoError(t, err)
		assert.Equal(t, "p1-net", res.Network)
	})

	// 2. P2 (standard flag `--`) wins over P3 to P6
	t.Run("P2 (standard flag) wins over environment, tools, and global layers", func(t *testing.T) {
		mfs := &MockFileSystem{
			Env: map[string]string{
				"CDERUN_NETWORK": "p3-net", // P3
			},
		}
		cli := &CLIOptions{
			Image:   ptr("alpine"),
			Network: ptr("p2-net"), // P2
		}
		tools := ToolsConfig{
			"sh": ToolConfig{
				Network: "p4-net", // P4
			},
		}
		global := &CDERunConfig{
			Defaults: ConfigDefaults{
				Network: "p5-net", // P5
			},
		}

		res, err := ResolveWithFS("sh", cli, tools, global, mfs)
		require.NoError(t, err)
		assert.Equal(t, "p2-net", res.Network)
	})

	// 3. P3 (environment variable) wins over P4 to P6
	t.Run("P3 (environment variable) wins over tools, global, and hardcoded layers", func(t *testing.T) {
		mfs := &MockFileSystem{
			Env: map[string]string{
				"CDERUN_NETWORK": "p3-net", // P3
			},
		}
		cli := &CLIOptions{
			Image: ptr("alpine"),
		}
		tools := ToolsConfig{
			"sh": ToolConfig{
				Network: "p4-net", // P4
			},
		}
		global := &CDERunConfig{
			Defaults: ConfigDefaults{
				Network: "p5-net", // P5
			},
		}

		res, err := ResolveWithFS("sh", cli, tools, global, mfs)
		require.NoError(t, err)
		assert.Equal(t, "p3-net", res.Network)
	})

	// 4. P4 (tool-specific config) wins over P5 and P6
	t.Run("P4 (tool-specific config) wins over global and hardcoded defaults", func(t *testing.T) {
		mfs := &MockFileSystem{}
		cli := &CLIOptions{
			Image: ptr("alpine"),
		}
		tools := ToolsConfig{
			"sh": ToolConfig{
				Network: "p4-net", // P4
			},
		}
		global := &CDERunConfig{
			Defaults: ConfigDefaults{
				Network: "p5-net", // P5
			},
		}

		res, err := ResolveWithFS("sh", cli, tools, global, mfs)
		require.NoError(t, err)
		assert.Equal(t, "p4-net", res.Network)
	})

	// 5. P5 (global defaults config) wins over P6
	t.Run("P5 (global defaults config) wins over P6 (hardcoded default)", func(t *testing.T) {
		mfs := &MockFileSystem{}
		cli := &CLIOptions{
			Image: ptr("alpine"),
		}
		global := &CDERunConfig{
			Defaults: ConfigDefaults{
				Network: "p5-net", // P5
			},
		}

		res, err := ResolveWithFS("sh", cli, nil, global, mfs)
		require.NoError(t, err)
		assert.Equal(t, "p5-net", res.Network)
	})

	// 6. P6 (hardcoded default) is fallback when nothing is specified
	t.Run("P6 (hardcoded default) resolves when no layer overrides it", func(t *testing.T) {
		mfs := &MockFileSystem{}
		cli := &CLIOptions{
			Image: ptr("alpine"),
		}

		res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)
		// Hardcoded fallback for Network is "bridge" (defined in registry.go / default fallback)
		assert.Equal(t, "bridge", res.Network)
	})
}
