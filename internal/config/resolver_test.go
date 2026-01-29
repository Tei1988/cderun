package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolve(t *testing.T) {
	// Setup helper to create bool pointers
	ptr := func(b bool) *bool { return &b }

	t.Run("P2 CLI takes priority over P4 Tool and P5 Global", func(t *testing.T) {
		cli := CLIOptions{
			TTY:    true,
			TTYSet: true,
		}
		tools := ToolsConfig{
			"node": ToolConfig{
				Image: "node:20",
				TTY:   ptr(false),
			},
		}
		global := &CDERunConfig{
			Defaults: ConfigDefaults{
				TTY: ptr(false),
			},
		}

		res, err := Resolve("node", cli, tools, global)
		require.NoError(t, err)
		assert.True(t, res.TTY)
		assert.Equal(t, "node:20", res.Image)
	})

	t.Run("P1 Override takes priority over P2 CLI", func(t *testing.T) {
		cli := CLIOptions{
			TTY:          true,
			TTYSet:       true,
			CderunTTY:    false,
			CderunTTYSet: true,
		}
		tools := ToolsConfig{
			"node": ToolConfig{
				Image: "node:20",
			},
		}

		res, err := Resolve("node", cli, tools, nil)
		require.NoError(t, err)
		assert.False(t, res.TTY)
	})

	t.Run("P3 Env Var priority", func(t *testing.T) {
		t.Setenv("CDERUN_TTY", "true")
		cli := CLIOptions{}
		tools := ToolsConfig{
			"node": ToolConfig{
				Image: "node:20",
				TTY:   ptr(false),
			},
		}

		res, err := Resolve("node", cli, tools, nil)
		require.NoError(t, err)
		assert.True(t, res.TTY)
	})

	t.Run("Image resolution from ToolConfig", func(t *testing.T) {
		cli := CLIOptions{}
		tools := ToolsConfig{
			"python": ToolConfig{
				Image: "python:3.11",
			},
		}

		res, err := Resolve("python", cli, tools, nil)
		require.NoError(t, err)
		assert.Equal(t, "python:3.11", res.Image)
	})

	t.Run("Error if no image found", func(t *testing.T) {
		cli := CLIOptions{}
		_, err := Resolve("unknown", cli, nil, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no image mapping found")
	})

	t.Run("Volume parsing", func(t *testing.T) {
		cli := CLIOptions{}
		tools := ToolsConfig{
			"node": ToolConfig{
				Image:   "node:20",
				Volumes: []string{"/host/path:/container/path:ro", ".:/app"},
			},
		}

		res, err := Resolve("node", cli, tools, nil)
		require.NoError(t, err)
		assert.Len(t, res.Volumes, 2)
		assert.Equal(t, "/host/path", res.Volumes[0].HostPath)
		assert.Equal(t, "/container/path", res.Volumes[0].ContainerPath)
		assert.True(t, res.Volumes[0].ReadOnly)
		assert.Equal(t, ".", res.Volumes[1].HostPath)
		assert.Equal(t, "/app", res.Volumes[1].ContainerPath)
		assert.False(t, res.Volumes[1].ReadOnly)
	})
}
