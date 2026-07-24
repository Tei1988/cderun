package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUnit_Config_EnvHelpers_Boundaries validates environment helpers addEnv, deduplicateEnv, and mergeEnv
// across small (<= 8) and large (> 8) boundaries and with/without duplicates.
func TestUnit_Config_EnvHelpers_Boundaries(t *testing.T) {
	t.Parallel()

	t.Run("addEnv handles fresh and duplicate keys", func(t *testing.T) {
		m := map[string]string{
			"A": "A=1",
		}
		keys := []string{"A"}

		addEnv(m, &keys, []string{"A=2", "B=3"})
		assert.Equal(t, "A=2", m["A"])
		assert.Equal(t, "B=3", m["B"])
		assert.Equal(t, []string{"A", "B"}, keys)
	})

	t.Run("deduplicateEnv length <= 1", func(t *testing.T) {
		assert.Nil(t, deduplicateEnv(nil))
		assert.Equal(t, []string{"A=1"}, deduplicateEnv([]string{"A=1"}))
	})

	t.Run("deduplicateEnv small list <= 8 with duplicates", func(t *testing.T) {
		input := []string{"A=1", "B=2", "A=3", "C=4", "B=5"}
		expected := []string{"A=3", "B=5", "C=4"}
		res := deduplicateEnv(input)
		assert.Equal(t, expected, res)
	})

	t.Run("deduplicateEnv small list <= 8 without duplicates", func(t *testing.T) {
		input := []string{"A=1", "B=2", "C=3", "D=4"}
		res := deduplicateEnv(input)
		// Should return the exact same slice (same reference) to avoid allocation
		assert.Equal(t, input, res)
	})

	t.Run("deduplicateEnv large list > 8 with duplicates", func(t *testing.T) {
		input := []string{
			"A=1", "B=2", "C=3", "D=4", "E=5", "F=6", "G=7", "H=8",
			"A=9", "I=10", "B=11",
		}
		expected := []string{"A=9", "B=11", "C=3", "D=4", "E=5", "F=6", "G=7", "H=8", "I=10"}
		res := deduplicateEnv(input)
		assert.Equal(t, expected, res)
	})

	t.Run("deduplicateEnv large list > 8 without duplicates", func(t *testing.T) {
		input := []string{
			"A=1", "B=2", "C=3", "D=4", "E=5", "F=6", "G=7", "H=8",
			"I=9", "J=10",
		}
		res := deduplicateEnv(input)
		assert.Equal(t, input, res)
	})

	t.Run("mergeEnv empty arguments", func(t *testing.T) {
		assert.Nil(t, mergeEnv(nil, nil, nil))
	})

	t.Run("mergeEnv optimizations: only one slice is populated", func(t *testing.T) {
		base := []string{"A=1", "A=2"}
		assert.Equal(t, []string{"A=2"}, mergeEnv(base, nil, nil))
		assert.Equal(t, []string{"A=2"}, mergeEnv(nil, base, nil))
		assert.Equal(t, []string{"A=2"}, mergeEnv(nil, nil, base))
	})

	t.Run("mergeEnv small list <= 8 total with duplicates", func(t *testing.T) {
		base := []string{"A=1", "B=2"}
		p2 := []string{"C=3", "A=4"}
		p1 := []string{"D=5", "B=6"}
		// Total strings = 6 (<= 8)
		res := mergeEnv(base, p2, p1)
		expected := []string{"A=4", "B=6", "C=3", "D=5"}
		assert.Equal(t, expected, res)
	})

	t.Run("mergeEnv large list > 8 total with duplicates", func(t *testing.T) {
		base := []string{"A=1", "B=2", "C=3"}
		p2 := []string{"D=4", "E=5", "F=6"}
		p1 := []string{"G=7", "H=8", "A=9", "D=10"}
		// Total strings = 10 (> 8)
		res := mergeEnv(base, p2, p1)
		expected := []string{"A=9", "B=2", "C=3", "D=10", "E=5", "F=6", "G=7", "H=8"}
		assert.Equal(t, expected, res)
	})
}

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
			Image:            "alpine",
			ImageSet:         true,
			Network:          "p2-net", // P2
			NetworkSet:       true,
			CderunNetwork:    "p1-net", // P1
			CderunNetworkSet: true,
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
			Image:      "alpine",
			ImageSet:   true,
			Network:    "p2-net", // P2
			NetworkSet: true,
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
			Image:    "alpine",
			ImageSet: true,
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
			Image:    "alpine",
			ImageSet: true,
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
			Image:    "alpine",
			ImageSet: true,
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
			Image:    "alpine",
			ImageSet: true,
		}

		res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)
		// Hardcoded fallback for Network is "bridge" (defined in registry.go / default fallback)
		assert.Equal(t, "bridge", res.Network)
	})
}

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
		CderunImage:    "node:20", // P1 internal override flag
		CderunImageSet: true,
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

// TestUnit_Config_Resolver_MemoryParsingBorderCases verifies parsing of extremely
// large sizes, invalid units, and border cases.
func TestUnit_Config_Resolver_MemoryParsingBorderCases(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{}

	t.Run("1024TiB represents 1PiB and parses successfully", func(t *testing.T) {
		cli := &CLIOptions{
			Image:     "alpine",
			ImageSet:  true,
			Memory:    "1024TiB",
			MemorySet: true,
		}
		res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)
		// 1024 TiB = 1024 * 1024^4 = 1125899906842624 bytes
		assert.Equal(t, int64(1125899906842624), res.Memory)
	})

	t.Run("1EiB is rejected due to units.RAMInBytes limitations", func(t *testing.T) {
		cli := &CLIOptions{
			Image:     "alpine",
			ImageSet:  true,
			Memory:    "1EiB",
			MemorySet: true,
		}
		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		var cfgErr *InvalidConfigError
		require.ErrorAs(t, err, &cfgErr)
		assert.Equal(t, "memory", cfgErr.Field)
	})
}

// TestUnit_Config_FQDN_HostnameValidation verifies FQDN validation scenarios in ValidateHostname.
func TestUnit_Config_FQDN_HostnameValidation(t *testing.T) {
	t.Parallel()

	validCases := []string{
		"dev.local.host",
		"sub-domain.example.com",
		"a.b.c.d.e.f",
		"hostname-with-63-characters-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
	}

	for _, tc := range validCases {
		t.Run("valid FQDN: "+tc, func(t *testing.T) {
			err := ValidateHostname(tc)
			assert.NoError(t, err)
		})
	}

	invalidCases := []struct {
		name string
		host string
	}{
		{"double dots", "invalid..domain.com"},
		{"trailing hyphen in segment", "invalid.domain.com-"},
		{"leading hyphen in segment", "-invalid.domain.com"},
		{"label segment too long", "label-too-long-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"},
		{"invalid character underscore", "host_name.com"},
	}

	for _, tc := range invalidCases {
		t.Run("invalid FQDN: "+tc.name, func(t *testing.T) {
			err := ValidateHostname(tc.host)
			assert.Error(t, err)
		})
	}
}
