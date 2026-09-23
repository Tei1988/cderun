package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_ConfigLoader_LoadCDERunConfigFromPath_TildeExpansion(t *testing.T) {
	fs := &MockFileSystem{
		WD:      "/workspace",
		HomeDir: "/home/user",
		Files: map[string][]byte{
			"/home/user/custom-cderun.yaml": []byte(`
runtime: podman
defaults:
  network: host
  workdir: /workspace/app
`),
		},
	}

	loader := NewConfigLoaderWithFS(fs)
	cfg, paths, err := loader.LoadCDERunConfigFromPath("~/custom-cderun.yaml")

	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "podman", cfg.Runtime)
	assert.Equal(t, "host", cfg.Defaults.Network)
	assert.Equal(t, "/workspace/app", cfg.Defaults.Workdir)
	assert.Equal(t, []string{"/home/user/custom-cderun.yaml"}, paths)
}

func TestUnit_ConfigLoader_LoadToolsConfigFromPath_TildeExpansion(t *testing.T) {
	fs := &MockFileSystem{
		WD:      "/workspace",
		HomeDir: "/home/user",
		Files: map[string][]byte{
			"/home/user/custom-tools.yaml": []byte(`
node:
  image: node:20-alpine
  network: bridge
`),
		},
	}

	loader := NewConfigLoaderWithFS(fs)
	tools, paths, err := loader.LoadToolsConfigFromPath("~/custom-tools.yaml")

	require.NoError(t, err)
	require.NotNil(t, tools)
	assert.Contains(t, tools, "node")
	assert.Equal(t, "node:20-alpine", tools["node"].Image)
	assert.Equal(t, "bridge", tools["node"].Network)
	assert.Equal(t, []string{"/home/user/custom-tools.yaml"}, paths)
}

func TestUnit_ConfigLoader_LoadFromPath_InvalidCharacters(t *testing.T) {
	fs := &MockFileSystem{
		WD:      "/workspace",
		HomeDir: "/home/user",
	}
	loader := NewConfigLoaderWithFS(fs)

	t.Run("CDERunConfig_NullByteInPath", func(t *testing.T) {
		cfg, paths, err := loader.LoadCDERunConfigFromPath("/path/to/config\x00.yaml")
		require.Error(t, err)
		assert.Nil(t, cfg)
		assert.Nil(t, paths)
		assert.Contains(t, err.Error(), "security validation failed")
	})

	t.Run("ToolsConfig_ControlCharInPath", func(t *testing.T) {
		tools, paths, err := loader.LoadToolsConfigFromPath("/path/to/tools\x01.yaml")
		require.Error(t, err)
		assert.Nil(t, tools)
		assert.Nil(t, paths)
		assert.Contains(t, err.Error(), "security validation failed")
	})
}

func TestUnit_ConfigLoader_LoadFromPath_StrictYAML(t *testing.T) {
	fs := &MockFileSystem{
		WD:      "/workspace",
		HomeDir: "/home/user",
		Files: map[string][]byte{
			"/workspace/unknown-cderun-field.yaml": []byte(`
runtime: docker
invalid_field_unknown: true
`),
			"/workspace/unknown-tools-field.yaml": []byte(`
node:
  image: node:20-alpine
  unknown_nested_field: true
`),
		},
	}
	loader := NewConfigLoaderWithFS(fs)

	t.Run("CDERunConfig_StrictUnmarshalError", func(t *testing.T) {
		cfg, paths, err := loader.LoadCDERunConfigFromPath("unknown-cderun-field.yaml")
		require.Error(t, err)
		assert.Nil(t, cfg)
		assert.Nil(t, paths)
		assert.Contains(t, err.Error(), "failed to unmarshal config file")
	})

	t.Run("ToolsConfig_StrictUnmarshalError", func(t *testing.T) {
		tools, paths, err := loader.LoadToolsConfigFromPath("unknown-tools-field.yaml")
		require.Error(t, err)
		assert.Nil(t, tools)
		assert.Nil(t, paths)
		assert.Contains(t, err.Error(), "failed to unmarshal tools file")
	})
}

func TestUnit_ConfigLoader_FindConfigsAndLoad_Hierarchical(t *testing.T) {
	fs := &MockFileSystem{
		WD:      "/workspace/project/sub",
		HomeDir: "/home/user",
		Dirs: map[string]bool{
			"/workspace/project/sub": true,
			"/workspace/project":     true,
			"/workspace":             true,
			"/home/user":             true,
			"/etc/cderun":            true,
			"/run/cderun":            true,
		},
		Files: map[string][]byte{
			"/etc/cderun/.cderun.yaml": []byte(`
runtime: docker
defaults:
  network: system-net
  workdir: /etc/work
`),
			"/home/user/.config/cderun/.cderun.yaml": []byte(`
defaults:
  network: home-net
  workdir: /home/user/work
`),
			"/workspace/project/.cderun.yaml": []byte(`
defaults:
  network: project-net
  readOnly: true
`),
		},
	}

	loader := NewConfigLoaderWithFS(fs)

	t.Run("FindConfigs_HierarchySearch", func(t *testing.T) {
		paths := loader.FindConfigs(".cderun.yaml")
		expected := []string{
			"/workspace/project/.cderun.yaml",
			"/home/user/.config/cderun/.cderun.yaml",
			"/etc/cderun/.cderun.yaml",
		}
		assert.Equal(t, expected, paths)
	})

	t.Run("LoadCDERunConfig_HierarchicalMerge", func(t *testing.T) {
		cfg, loadedPaths, err := loader.LoadCDERunConfig()
		require.NoError(t, err)
		require.NotNil(t, cfg)

		// Verification of merged properties according to precedence (/workspace/project > home > /etc)
		// project-net overrides home-net and system-net
		assert.Equal(t, "docker", cfg.Runtime)
		assert.Equal(t, "project-net", cfg.Defaults.Network)
		// home workdir overrides system workdir
		assert.Equal(t, "/home/user/work", cfg.Defaults.Workdir)
		require.NotNil(t, cfg.Defaults.ReadOnly)
		assert.True(t, *cfg.Defaults.ReadOnly)

		expectedLoaded := []string{
			"/workspace/project/.cderun.yaml",
			"/home/user/.config/cderun/.cderun.yaml",
			"/etc/cderun/.cderun.yaml",
		}
		assert.Equal(t, expectedLoaded, loadedPaths)
	})
}
