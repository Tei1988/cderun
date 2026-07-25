package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_ResolverHelpers_AddEnv(t *testing.T) {
	t.Parallel()

	t.Run("basic addition", func(t *testing.T) {
		m := make(map[string]string)
		keys := []string{}
		env := []string{"KEY1=val1", "KEY2=val2"}

		addEnv(m, &keys, env)

		assert.Equal(t, map[string]string{"KEY1": "KEY1=val1", "KEY2": "KEY2=val2"}, m)
		assert.Equal(t, []string{"KEY1", "KEY2"}, keys)
	})

	t.Run("overwriting existing keys maintains first key order", func(t *testing.T) {
		m := map[string]string{"KEY1": "KEY1=old"}
		keys := []string{"KEY1"}
		env := []string{"KEY1=new", "KEY2=val2"}

		addEnv(m, &keys, env)

		assert.Equal(t, map[string]string{"KEY1": "KEY1=new", "KEY2": "KEY2=val2"}, m)
		assert.Equal(t, []string{"KEY1", "KEY2"}, keys)
	})
}

func TestUnit_Config_ResolverHelpers_DeduplicateEnv(t *testing.T) {
	t.Parallel()

	t.Run("empty list", func(t *testing.T) {
		var input []string = nil
		res := deduplicateEnv(input)
		assert.Nil(t, res)
	})

	t.Run("single element", func(t *testing.T) {
		input := []string{"KEY=val"}
		res := deduplicateEnv(input)
		assert.Equal(t, []string{"KEY=val"}, res)
	})

	t.Run("under 8 elements without duplicates", func(t *testing.T) {
		input := []string{"A=1", "B=2", "C=3"}
		res := deduplicateEnv(input)
		assert.Equal(t, input, res)
	})

	t.Run("under 8 elements with duplicates", func(t *testing.T) {
		input := []string{"A=1", "B=2", "A=3", "C=4", "B=5"}
		expected := []string{"A=3", "B=5", "C=4"}
		res := deduplicateEnv(input)
		assert.Equal(t, expected, res)
	})

	t.Run("over 8 elements without duplicates", func(t *testing.T) {
		input := []string{"A=1", "B=2", "C=3", "D=4", "E=5", "F=6", "G=7", "H=8", "I=9"}
		res := deduplicateEnv(input)
		assert.Equal(t, input, res)
	})

	t.Run("over 8 elements with duplicates", func(t *testing.T) {
		input := []string{"A=1", "B=2", "C=3", "D=4", "E=5", "F=6", "G=7", "H=8", "I=9", "A=10", "E=20"}
		expected := []string{"A=10", "B=2", "C=3", "D=4", "E=20", "F=6", "G=7", "H=8", "I=9"}
		res := deduplicateEnv(input)
		assert.Equal(t, expected, res)
	})
}

func TestUnit_Config_ResolverHelpers_MergeEnv(t *testing.T) {
	t.Parallel()

	t.Run("all empty sources", func(t *testing.T) {
		res := mergeEnv(nil, nil, nil)
		assert.Nil(t, res)
	})

	t.Run("optimization - base only non-empty", func(t *testing.T) {
		base := []string{"A=1", "A=2"}
		res := mergeEnv(base, nil, nil)
		assert.Equal(t, []string{"A=2"}, res)
	})

	t.Run("optimization - p2 only non-empty", func(t *testing.T) {
		p2 := []string{"B=1", "B=2"}
		res := mergeEnv(nil, p2, nil)
		assert.Equal(t, []string{"B=2"}, res)
	})

	t.Run("optimization - p1 only non-empty", func(t *testing.T) {
		p1 := []string{"C=1", "C=2"}
		res := mergeEnv(nil, nil, p1)
		assert.Equal(t, []string{"C=2"}, res)
	})

	t.Run("total elements <= 8 without duplicates", func(t *testing.T) {
		base := []string{"A=1", "B=2"}
		p2 := []string{"C=3", "D=4"}
		p1 := []string{"E=5"}

		res := mergeEnv(base, p2, p1)
		assert.Equal(t, []string{"A=1", "B=2", "C=3", "D=4", "E=5"}, res)
	})

	t.Run("total elements <= 8 with duplicates", func(t *testing.T) {
		base := []string{"A=1", "B=2"}
		p2 := []string{"C=3", "A=4"}
		p1 := []string{"B=5", "D=6"}

		res := mergeEnv(base, p2, p1)
		assert.Equal(t, []string{"A=4", "B=5", "C=3", "D=6"}, res)
	})

	t.Run("total elements > 8 without duplicates", func(t *testing.T) {
		base := []string{"A=1", "B=2", "C=3"}
		p2 := []string{"D=4", "E=5", "F=6"}
		p1 := []string{"G=7", "H=8", "I=9"}

		res := mergeEnv(base, p2, p1)
		assert.Equal(t, []string{"A=1", "B=2", "C=3", "D=4", "E=5", "F=6", "G=7", "H=8", "I=9"}, res)
	})

	t.Run("total elements > 8 with duplicates", func(t *testing.T) {
		base := []string{"A=1", "B=2", "C=3"}
		p2 := []string{"D=4", "E=5", "A=6"}
		p1 := []string{"G=7", "H=8", "B=9", "I=10"}

		res := mergeEnv(base, p2, p1)
		expected := []string{"A=6", "B=9", "C=3", "D=4", "E=5", "G=7", "H=8", "I=10"}
		assert.Equal(t, expected, res)
	})
}

func TestUnit_Config_Resolver_WrapperMode_Precedence(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		Env: map[string]string{
			"CDERUN_IMAGE": "node:env",
			"CDERUN_CPUS":  "2.5",
			"CDERUN_TTY":   "false",
		},
	}

	// 1. P1 overrides takes precedence over everything
	t.Run("P1 takes absolute priority", func(t *testing.T) {
		cli := &CLIOptions{
			Image:       ptr("alpine:cli"),
			CderunImage: ptr("alpine:override"),
		}

		res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, "alpine:override", res.Image)
	})

	// 2. P2 (CLI) takes precedence over P3 (Env), P4 (Tool), P5 (Global), P6 (Default)
	t.Run("P2 takes priority over P3 and lower", func(t *testing.T) {
		cli := &CLIOptions{
			Image: ptr("node:cli"),
		}
		tools := ToolsConfig{
			"sh": ToolConfig{
				Image: "node:tool",
			},
		}
		global := &CDERunConfig{
			Defaults: ConfigDefaults{
				StrictEnv: ptr(true),
			},
		}

		res, err := ResolveWithFS("sh", cli, tools, global, mfs)
		require.NoError(t, err)
		assert.Equal(t, "node:cli", res.Image)
	})

	// 3. P3 (Env) takes precedence over P4 (Tool)
	t.Run("P3 takes priority over P4", func(t *testing.T) {
		cli := &CLIOptions{}
		tools := ToolsConfig{
			"sh": ToolConfig{
				Image: "node:tool",
			},
		}

		res, err := ResolveWithFS("sh", cli, tools, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, "node:env", res.Image)
	})
}

func TestUnit_Config_Resolver_SymlinkMode_Defaults(t *testing.T) {
	t.Parallel()

	t.Run("resolving configurations for a mapped symlink tool", func(t *testing.T) {
		mfs := &MockFileSystem{}
		tools := ToolsConfig{
			"python": ToolConfig{
				Image: "python:3.11",
				TTY:   ptr(true),
				CPUs:  ptr(4.0),
			},
		}
		global := &CDERunConfig{
			Defaults: ConfigDefaults{
				Memory: "1024MB",
			},
		}

		res, err := ResolveWithFS("python", &CLIOptions{}, tools, global, mfs)
		require.NoError(t, err)

		assert.Equal(t, "python:3.11", res.Image)
		assert.True(t, res.TTY)
		assert.InDelta(t, 4.0, res.CPUs, 0.0001)
		assert.Equal(t, int64(1024*1024*1024), res.Memory)
	})
}

func TestUnit_Config_Resolver_NegativeMemoryParserBorderCases(t *testing.T) {
	t.Parallel()

	t.Run("extremely large valid memory limit 1024TiB", func(t *testing.T) {
		mfs := &MockFileSystem{}
		cli := &CLIOptions{
			Image:  ptr("alpine"),
			Memory: ptr("1024TiB"),
		}

		res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, int64(1024*1024*1024*1024*1024), res.Memory)
	})

	t.Run("malformed memory values", func(t *testing.T) {
		mfs := &MockFileSystem{}
		cli := &CLIOptions{
			Image:  ptr("alpine"),
			Memory: ptr("abcG"),
		}

		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		var cfgErr *InvalidConfigError
		require.ErrorAs(t, err, &cfgErr)
		assert.Equal(t, "memory", cfgErr.Field)
	})
}

func TestUnit_Config_Resolver_ValidateHostname_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"FQDN with multiple dots", "server.dev.cluster.local", false},
		{"Single hyphen host", "a-b", false},
		{"Multiple hyphens host", "a-b-c-d", false},
		{"IP-like hostname", "192.168.1.1", false},
		{"Hostname with dot but starts with dot", ".web-server", true},
		{"Hostname too long", "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyz01234567.abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyz01234567.abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyz01234567.abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyz01234567.abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyz01234567", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateHostname(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
