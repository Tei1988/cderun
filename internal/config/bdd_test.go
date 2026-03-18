package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScenario_ConfigResolution_ComplexOverrides(t *testing.T) {
	t.Parallel()

	t.Run("P1 through P5 priority with complex expressions", func(t *testing.T) {
		// Given: A complex environment with multiple configuration layers and expressions
		mfs := &MockFileSystem{
			WD: "/home/user/project",
			Files: map[string][]byte{
				"/home/user/project/.go-version": []byte("1.25"),
			},
			Env: map[string]string{
				"PROJECT_ENV": "production",
				"CDERUN_IMAGE": "node:{{file:.go-version}}-{{env:PROJECT_ENV}}",
			},
		}

		cli := CLIOptions{
			CderunTTY:    true,
			CderunTTYSet: true, // P1
			TTY:          false,
			TTYSet:       true, // P2
		}

		tools := ToolsConfig{
			"node": ToolConfig{
				Image: "node:latest", // P4 (should be overridden by P3 Env Var with expression)
				TTY:   ptr(false),
			},
		}

		global := &CDERunConfig{
			Defaults: ConfigDefaults{
				TTY: ptr(false), // P5
			},
		}

		// When: Resolving configuration
		res, err := ResolveWithFS("node", cli, tools, global, mfs)

		// Then: Priority and expressions should be resolved correctly
		require.NoError(t, err)

		// P1 (CderunTTY) wins over P2, P3, P4, P5
		assert.True(t, res.TTY)

		// P3 (Env Var CDERUN_IMAGE) wins over P4 (Tool Image)
		// and expression {{file:.go-version}} and {{env:PROJECT_ENV}} are resolved
		assert.Equal(t, "node:1.25-production", res.Image)
	})
}

func TestScenario_ConfigResolution_NestedOverrides(t *testing.T) {
	t.Parallel()

	t.Run("Nested tool config overrides global defaults", func(t *testing.T) {
		// Given: Global config with some defaults and Tool config with overrides
		global := &CDERunConfig{
			Defaults: ConfigDefaults{
				Network: "bridge",
				Remove:  ptr(true),
				Env:     []string{"GLOBAL=1"},
			},
		}
		tools := ToolsConfig{
			"app": ToolConfig{
				Image:   "my-app",
				Network: "host",
				Env:     []string{"TOOL=1"},
			},
		}

		// When: Resolving configuration for "app"
		res, err := Resolve("app", CLIOptions{}, tools, global)

		// Then: Tool settings should override global ones, and slices should not merge across layers
		require.NoError(t, err)
		assert.Equal(t, "host", res.Network)
		assert.True(t, res.Remove)
		assert.Contains(t, res.Env, "TOOL=1")
		assert.NotContains(t, res.Env, "GLOBAL=1")
	})
}
