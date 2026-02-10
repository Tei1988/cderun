package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_Merge_Hierarchical(t *testing.T) {
	fs := &mockFileSystem{
		wd: "/app/child",
		files: map[string][]byte{
			"/app/.cderun.yaml":       []byte("runtime: docker\ndefaults:\n  tty: false\n  network: bridge"),
			"/app/.tools.yaml":        []byte("node:\n  image: node:14\n  env: [\"PARENT=1\"]"),
			"/app/child/.cderun.yaml": []byte("defaults:\n  tty: true"),
			"/app/child/.tools.yaml":  []byte("node:\n  image: node:16\npython:\n  image: python:3.9"),
		},
		statErr: map[string]error{
			"/home/user/.config/cderun/.cderun.yaml": os.ErrNotExist,
			"/etc/cderun/.cderun.yaml":               os.ErrNotExist,
			"/run/cderun/.cderun.yaml":               os.ErrNotExist,
			"/home/user/.config/cderun/.tools.yaml":  os.ErrNotExist,
			"/etc/cderun/.tools.yaml":                os.ErrNotExist,
			"/run/cderun/.tools.yaml":                os.ErrNotExist,
		},
	}
	loader := &ConfigLoader{
		FS:              fs,
		SystemConfigDir: "/etc/cderun",
		RunConfigDir:    "/run/cderun",
	}

	t.Run("CDERunConfig Merge", func(t *testing.T) {
		cfg, paths, err := loader.LoadCDERunConfig()
		assert.NoError(t, err)
		require.Len(t, paths, 2)
		assert.Contains(t, paths[0], "child")
		assert.Equal(t, "docker", cfg.Runtime)
		assert.True(t, *cfg.Defaults.TTY)
		assert.Equal(t, "bridge", cfg.Defaults.Network)
	})

	t.Run("ToolsConfig Merge", func(t *testing.T) {
		cfg, paths, err := loader.LoadToolsConfig()
		assert.NoError(t, err)
		require.Len(t, paths, 2)

		node := cfg["node"]
		assert.Equal(t, "node:16", node.Image)
		assert.Equal(t, []string{"PARENT=1"}, node.Env)

		python := cfg["python"]
		assert.Equal(t, "python:3.9", python.Image)
	})
}
