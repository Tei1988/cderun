package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
			Image:          "alpine:cli",
			ImageSet:       true,
			CderunImage:    "alpine:override",
			CderunImageSet: true,
		}

		res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, "alpine:override", res.Image)
	})

	// 2. P2 (CLI) takes precedence over P3 (Env), P4 (Tool), P5 (Global), P6 (Default)
	t.Run("P2 takes priority over P3 and lower", func(t *testing.T) {
		cli := &CLIOptions{
			Image:    "node:cli",
			ImageSet: true,
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

	t.Run("malformed memory values", func(t *testing.T) {
		mfs := &MockFileSystem{}
		cli := &CLIOptions{
			Image:     "alpine",
			ImageSet:  true,
			Memory:    "abcG",
			MemorySet: true,
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
