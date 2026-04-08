package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_LoadCDERunConfig_FromPath_Hierarchical(t *testing.T) {
	t.Run("successive merges of global config layers", func(t *testing.T) {
		mfs := &MockFileSystem{
			Dirs: map[string]bool{
				"/project":    true,
				"/etc/cderun": true,
			},
			Files: map[string][]byte{
				"/etc/cderun/.cderun.yaml": []byte("runtime: docker\nlogging:\n  level: info"),
				"/project/.cderun.yaml":    []byte("runtime: podman"),
			},
			WD: "/project",
		}
		loader := &ConfigLoader{fs: mfs, systemConfigDir: "/etc/cderun", runConfigDir: "/run/cderun"}
		cfg, paths, err := loader.LoadCDERunConfig()
		require.NoError(t, err)
		require.NotNil(t, cfg)
		assert.Equal(t, "podman", cfg.Runtime)
		assert.Equal(t, "info", cfg.Logging.Level)
		assert.Len(t, paths, 2)
	})
}

func TestUnit_Config_LoadToolsConfig_FromPath_Merging(t *testing.T) {
	t.Run("merging tool config layers", func(t *testing.T) {
		mfs := &MockFileSystem{
			Dirs: map[string]bool{
				"/project":    true,
				"/etc/cderun": true,
			},
			Files: map[string][]byte{
				"/etc/cderun/.tools.yaml": []byte("node:\n  image: node:18\n  tty: true"),
				"/project/.tools.yaml":    []byte("node:\n  image: node:20"),
			},
			WD: "/project",
		}
		loader := &ConfigLoader{fs: mfs, systemConfigDir: "/etc/cderun", runConfigDir: "/run/cderun"}
		cfg, _, err := loader.LoadToolsConfig()
		require.NoError(t, err)
		require.NotNil(t, cfg)
		node, ok := cfg["node"]
		require.True(t, ok)
		require.NotNil(t, node.TTY)
		assert.Equal(t, "node:20", node.Image)
		assert.True(t, *node.TTY)
	})
}
