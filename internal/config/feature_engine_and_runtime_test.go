package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_EngineAndRuntimeResolution(t *testing.T) {
	t.Run("engine resolved from CLI flag", func(t *testing.T) {
		mfs := &MockFileSystem{}
		cli := &CLIOptions{Engine: ptr("podman"), Image: ptr("alpine")}
		res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, "podman", res.Engine)
		assert.Empty(t, res.Runtime)
	})

	t.Run("legacy CDERUN_RUNTIME env fallback to engine", func(t *testing.T) {
		mfs := &MockFileSystem{
			Env: map[string]string{"CDERUN_RUNTIME": "containerd"},
		}
		cli := &CLIOptions{Image: ptr("alpine")}
		res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, "containerd", res.Engine)
		assert.Empty(t, res.Runtime)
	})

	t.Run("OCI runtime resolved from CDERUN_OCI_RUNTIME", func(t *testing.T) {
		mfs := &MockFileSystem{
			Env: map[string]string{"CDERUN_OCI_RUNTIME": "crun"},
		}
		cli := &CLIOptions{Image: ptr("alpine")}
		res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, "docker", res.Engine)
		assert.Equal(t, "crun", res.Runtime)
	})

	t.Run("layered tool config merge preserves inherited engine when omitted in higher layer", func(t *testing.T) {
		mfs := &MockFileSystem{
			Dirs: map[string]bool{
				"/project":    true,
				"/etc/cderun": true,
			},
			Files: map[string][]byte{
				"/etc/cderun/.tools.yaml": []byte("node:\n  image: node:18\n  runtime: podman\n"),
				"/project/.tools.yaml":    []byte("node:\n  image: node:20\n  tty: true\n"),
			},
			WD: "/project",
		}
		loader := &ConfigLoader{fs: mfs, systemConfigDir: "/etc/cderun", runConfigDir: "/run/cderun"}
		cfg, _, err := loader.LoadToolsConfig()
		require.NoError(t, err)
		require.NotNil(t, cfg)
		node, ok := cfg["node"]
		require.True(t, ok)
		assert.Equal(t, "node:20", node.Image)
		assert.Equal(t, "podman", node.Engine)
		assert.Empty(t, node.Runtime)
		assert.True(t, *node.TTY)
	})

	t.Run("higher layer engine overrides lower layer legacy runtime", func(t *testing.T) {
		mfs := &MockFileSystem{
			Dirs: map[string]bool{
				"/project":    true,
				"/etc/cderun": true,
			},
			Files: map[string][]byte{
				"/etc/cderun/.tools.yaml": []byte("node:\n  image: node:18\n  runtime: podman\n"),
				"/project/.tools.yaml":    []byte("node:\n  image: node:20\n  engine: containerd\n  runtime: runc\n"),
			},
			WD: "/project",
		}
		loader := &ConfigLoader{fs: mfs, systemConfigDir: "/etc/cderun", runConfigDir: "/run/cderun"}
		cfg, _, err := loader.LoadToolsConfig()
		require.NoError(t, err)
		require.NotNil(t, cfg)
		node, ok := cfg["node"]
		require.True(t, ok)
		assert.Equal(t, "containerd", node.Engine)
		assert.Equal(t, "runc", node.Runtime)
	})
}
