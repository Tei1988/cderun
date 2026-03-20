package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScenario_ConfigResolution_CollectionOverrides(t *testing.T) {
	t.Parallel()

	t.Run("Explicit empty env in Tool overrides non-empty Global", func(t *testing.T) {
		t.Parallel()
		global := &CDERunConfig{
			Defaults: ConfigDefaults{
				Env: []string{"GLOBAL=1"},
			},
		}
		tools := ToolsConfig{
			"app": ToolConfig{
				Image: "my-app",
				Env:   []string{},
			},
		}
		res, err := ResolveWithFS("app", CLIOptions{}, tools, global, &MockFileSystem{})
		require.NoError(t, err)
		assert.Empty(t, res.Env)
	})

	t.Run("Explicit empty mounts in Tool overrides non-empty Global", func(t *testing.T) {
		t.Parallel()
		global := &CDERunConfig{
			Defaults: ConfigDefaults{
				Mounts: []MountConfig{
					{Source: ConfigPath{Raw: "/host"}, Target: ConfigPath{Raw: "/container"}, Type: "bind"},
				},
			},
		}
		tools := ToolsConfig{
			"app": ToolConfig{
				Image:  "my-app",
				Mounts: []MountConfig{},
			},
		}
		res, err := ResolveWithFS("app", CLIOptions{}, tools, global, &MockFileSystem{})
		require.NoError(t, err)
		assert.Empty(t, res.Mounts)
	})

	t.Run("Omitted collections in Tool fall back to Global", func(t *testing.T) {
		t.Parallel()
		global := &CDERunConfig{
			Defaults: ConfigDefaults{
				Env: []string{"GLOBAL=1"},
			},
		}
		tools := ToolsConfig{
			"app": ToolConfig{
				Image: "my-app",
			},
		}
		res, err := ResolveWithFS("app", CLIOptions{}, tools, global, &MockFileSystem{})
		require.NoError(t, err)
		assert.Equal(t, []string{"GLOBAL=1"}, res.Env)
	})
}
