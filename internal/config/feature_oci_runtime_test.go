package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_OciRuntime_PrecedenceMatrix(t *testing.T) {
	t.Run("P1 overrides P2, P4, P5", func(t *testing.T) {
		cli := &CLIOptions{
			Image:            optPtr("alpine"),
			OciRuntime:       optPtr("crun"),
			CderunOciRuntime: optPtr("runc"),
		}
		fs := &MockFileSystem{Env: map[string]string{"CDERUN_OCI_RUNTIME": "nvidia"}}
		global := &CDERunConfig{
			Defaults: ConfigDefaults{
				OciRuntime: "kata",
			},
		}

		res, err := ResolveWithFS("echo", cli, nil, global, fs)
		require.NoError(t, err)
		assert.Equal(t, "runc", res.OciRuntime)
	})

	t.Run("P2 overrides P4, P5", func(t *testing.T) {
		cli := &CLIOptions{
			Image:      optPtr("alpine"),
			OciRuntime: optPtr("crun"),
		}
		fs := &MockFileSystem{Env: map[string]string{"CDERUN_OCI_RUNTIME": "nvidia"}}
		global := &CDERunConfig{
			Defaults: ConfigDefaults{
				OciRuntime: "kata",
			},
		}

		res, err := ResolveWithFS("echo", cli, nil, global, fs)
		require.NoError(t, err)
		assert.Equal(t, "crun", res.OciRuntime)
	})

	t.Run("P4 overrides P5", func(t *testing.T) {
		cli := &CLIOptions{
			Image: optPtr("alpine"),
		}
		fs := &MockFileSystem{Env: map[string]string{"CDERUN_OCI_RUNTIME": "nvidia"}}
		global := &CDERunConfig{
			Defaults: ConfigDefaults{
				OciRuntime: "kata",
			},
		}

		res, err := ResolveWithFS("echo", cli, nil, global, fs)
		require.NoError(t, err)
		assert.Equal(t, "nvidia", res.OciRuntime)
	})

	t.Run("P5 default config", func(t *testing.T) {
		cli := &CLIOptions{
			Image: optPtr("alpine"),
		}
		fs := &MockFileSystem{}
		global := &CDERunConfig{
			Defaults: ConfigDefaults{
				OciRuntime: "kata",
			},
		}

		res, err := ResolveWithFS("echo", cli, nil, global, fs)
		require.NoError(t, err)
		assert.Equal(t, "kata", res.OciRuntime)
	})
}

func TestUnit_Config_OciRuntime_YamlConfigLoading(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".cderun.yaml")
	yamlContent := []byte(`
defaults:
  ociRuntime: nvidia
`)
	err := os.WriteFile(configPath, yamlContent, 0644)
	require.NoError(t, err)

	loader := NewConfigLoader()
	cfg, paths, err := loader.LoadCDERunConfigFromPath(configPath)
	require.NoError(t, err)
	require.Len(t, paths, 1)
	assert.Equal(t, "nvidia", cfg.Defaults.OciRuntime)
}

func optPtr[T any](v T) *T {
	return &v
}
