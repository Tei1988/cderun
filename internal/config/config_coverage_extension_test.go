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

func TestUnit_Config_LoadCDERunConfig_Errors(t *testing.T) {
	t.Run("malformed YAML in system config", func(t *testing.T) {
		mfs := &MockFileSystem{
			Dirs:  map[string]bool{"/etc/cderun": true},
			Files: map[string][]byte{"/etc/cderun/.cderun.yaml": []byte("runtime: [")},
			WD:    "/project",
		}
		loader := &ConfigLoader{fs: mfs, systemConfigDir: "/etc/cderun"}
		_, _, err := loader.LoadCDERunConfig()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal")
	})
}

func TestUnit_Config_LoadToolsConfig_Errors(t *testing.T) {
	t.Run("malformed YAML in system config", func(t *testing.T) {
		mfs := &MockFileSystem{
			Dirs:  map[string]bool{"/etc/cderun": true},
			Files: map[string][]byte{"/etc/cderun/.tools.yaml": []byte("node: [")},
			WD:    "/project",
		}
		loader := &ConfigLoader{fs: mfs, systemConfigDir: "/etc/cderun"}
		_, _, err := loader.LoadToolsConfig()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal")
	})

	t.Run("missing tool config is not an error", func(t *testing.T) {
		mfs := &MockFileSystem{
			WD: "/project",
		}
		loader := &ConfigLoader{fs: mfs, systemConfigDir: "/etc/cderun", runConfigDir: "/run/cderun"}
		cfg, paths, err := loader.LoadToolsConfig()
		require.NoError(t, err)
		assert.Empty(t, cfg)
		assert.Empty(t, paths)
	})
}

func TestUnit_Config_CachedStat_EdgeCases(t *testing.T) {
	t.Run("cachedStat error handling", func(t *testing.T) {
		mfs := &MockFileSystem{
			StatErr: assert.AnError,
		}
		loader := NewConfigLoaderWithFS(mfs)
		_, err := loader.cachedStat("/error")
		require.Error(t, err)

		// Second call should return same error from cache
		_, err2 := loader.cachedStat("/error")
		assert.Same(t, err, err2)
		assert.Len(t, mfs.StatCalls, 1)
	})
}
