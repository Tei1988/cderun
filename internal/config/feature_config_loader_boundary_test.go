package config

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// TestUnit_ConfigLoader_FindConfigs_SearchSequence verifies the hierarchical directory
// traversal search order for .cderun.yaml and .tools.yaml specified in
// docs/features/configuration-file-support.md (Hierarchical Search and Merging Rules).
func TestUnit_ConfigLoader_FindConfigs_SearchSequence(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD:      "/work/project/sub",
		HomeDir: "/home/user",
		Dirs: map[string]bool{
			"/run/cderun/.cderun.yaml":               true,
			"/etc/cderun/.cderun.yaml":               true,
			"/home/user/.config/cderun/.cderun.yaml": true,
			"/work/.cderun.yaml":                     true,
			"/work/project/.cderun.yaml":             true,
			"/work/project/sub/.cderun.yaml":         true,
		},
	}

	loader := NewConfigLoaderWithFS(mfs)
	paths := loader.FindConfigs(".cderun.yaml")

	require.Len(t, paths, 6)
	// Order returned by FindConfigs: WD closest -> WD root -> Home -> System -> Run
	assert.Equal(t, "/work/project/sub/.cderun.yaml", paths[0])
	assert.Equal(t, "/work/project/.cderun.yaml", paths[1])
	assert.Equal(t, "/work/.cderun.yaml", paths[2])
	assert.Equal(t, "/home/user/.config/cderun/.cderun.yaml", paths[3])
	assert.Equal(t, "/etc/cderun/.cderun.yaml", paths[4])
	assert.Equal(t, "/run/cderun/.cderun.yaml", paths[5])
}

// TestUnit_ConfigLoader_FindConfigs_FileSystemErrors verifies graceful recovery
// when Getwd, UserHomeDir, or Abs return errors during configuration discovery.
// Ref: docs/features/configuration-file-support.md
func TestUnit_ConfigLoader_FindConfigs_FileSystemErrors(t *testing.T) {
	t.Parallel()

	t.Run("getwd_failure_skips_working_dir_traversal", func(t *testing.T) {
		mfs := &CoverageErrorFS{
			MockFileSystem: &MockFileSystem{
				HomeDir: "/home/user",
				Dirs: map[string]bool{
					"/home/user/.config/cderun/.cderun.yaml": true,
					"/etc/cderun/.cderun.yaml":               true,
				},
			},
			WDErr: errors.New("cannot determine working directory"),
		}

		loader := NewConfigLoaderWithFS(mfs)
		paths := loader.FindConfigs(".cderun.yaml")

		assert.NotEmpty(t, paths)
		assert.Contains(t, paths, "/home/user/.config/cderun/.cderun.yaml")
		assert.Contains(t, paths, "/etc/cderun/.cderun.yaml")
	})

	t.Run("home_dir_failure_skips_user_config", func(t *testing.T) {
		mfs := &CoverageErrorFS{
			MockFileSystem: &MockFileSystem{
				WD: "/app",
				Dirs: map[string]bool{
					"/app/.cderun.yaml":        true,
					"/etc/cderun/.cderun.yaml": true,
				},
			},
			HomeErr: errors.New("cannot determine user home directory"),
		}

		loader := NewConfigLoaderWithFS(mfs)
		paths := loader.FindConfigs(".cderun.yaml")

		assert.NotEmpty(t, paths)
		assert.Contains(t, paths, "/app/.cderun.yaml")
		assert.Contains(t, paths, "/etc/cderun/.cderun.yaml")
		for _, p := range paths {
			assert.NotContains(t, p, "/home")
		}
	})

	t.Run("abs_error_preserves_discovered_paths", func(t *testing.T) {
		mfs := &MockFileSystem{
			WD:      "/app",
			HomeDir: "/home/user",
			Dirs: map[string]bool{
				"/app/.cderun.yaml":                      true,
				"/home/user/.config/cderun/.cderun.yaml": true,
			},
			AbsErr: errors.New("abs path conversion failed"),
		}

		loader := NewConfigLoaderWithFS(mfs)
		paths := loader.FindConfigs(".cderun.yaml")

		assert.Contains(t, paths, "/app/.cderun.yaml")
		assert.Contains(t, paths, "/home/user/.config/cderun/.cderun.yaml")
	})
}

// TestUnit_ConfigLoader_LoadFromPath_ValidationAndErrors verifies explicit path
// configuration loading boundaries as specified in docs/features/configuration-file-support.md.
func TestUnit_ConfigLoader_LoadFromPath_ValidationAndErrors(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD:      "/workspace",
		HomeDir: "/home/tester",
		Files: map[string][]byte{
			"/home/tester/custom.yaml": []byte("engine: docker\n"),
			"/workspace/invalid.yaml":   []byte("engine: [invalid\n"),
		},
	}

	loader := NewConfigLoaderWithFS(mfs)

	t.Run("explicit_path_with_tilde_expansion", func(t *testing.T) {
		cfg, paths, err := loader.LoadCDERunConfigFromPath("~/custom.yaml")
		require.NoError(t, err)
		require.Len(t, paths, 1)
		assert.Equal(t, "/home/tester/custom.yaml", paths[0])
		require.NotNil(t, cfg)
		assert.Equal(t, "docker", cfg.Engine)
	})

	t.Run("non_existent_path_returns_error", func(t *testing.T) {
		_, _, err := loader.LoadCDERunConfigFromPath("/nonexistent/config.yaml")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read config file")
	})

	t.Run("malformed_yaml_syntax_returns_error", func(t *testing.T) {
		_, _, err := loader.LoadCDERunConfigFromPath("invalid.yaml")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal config file")
	})
}

// TestUnit_ConfigLoader_StrictYAMLDecoding_KnownFields verifies that unknown
// configuration keys are strictly rejected with decoding errors as defined in
// docs/features/configuration-file-support.md (Format and Decoding Rules).
func TestUnit_ConfigLoader_StrictYAMLDecoding_KnownFields(t *testing.T) {
	t.Parallel()

	t.Run("unknown_root_key_in_cderun_config_fails", func(t *testing.T) {
		content := []byte(`
engine: docker
unregistered_key: "unexpected_value"
`)
		var cfg CDERunConfig
		decoder := yaml.NewDecoder(bytes.NewReader(content))
		decoder.KnownFields(true)
		err := decoder.Decode(&cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "field unregistered_key not found")
	})

	t.Run("unknown_key_in_tools_config_fails", func(t *testing.T) {
		content := []byte(`
node:
  image: "node:20-alpine"
  invalidOption: "bad"
`)
		mfs := &MockFileSystem{
			WD: "/app",
			Files: map[string][]byte{
				"/app/.tools.yaml": content,
			},
		}
		loader := NewConfigLoaderWithFS(mfs)
		_, _, err := loader.LoadToolsConfig()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalidOption not found")
	})

	t.Run("prohibited_config_path_keys_fail", func(t *testing.T) {
		// Specifying config or toolConfig inside config files is strictly prohibited
		content := []byte(`
engine: docker
config: "/etc/cderun.yaml"
`)
		mfs := &MockFileSystem{
			WD: "/app",
			Files: map[string][]byte{
				"/app/.cderun.yaml": content,
			},
		}
		loader := NewConfigLoaderWithFS(mfs)
		_, _, err := loader.LoadCDERunConfig()
		require.Error(t, err)
	})
}

// TestUnit_ConfigLoader_DeepCopy_Isolation verifies that DeepCopy produces
// completely isolated copies of CDERunConfig and ToolConfig structures.
// Ref: docs/features/configuration-file-support.md
func TestUnit_ConfigLoader_DeepCopy_Isolation(t *testing.T) {
	t.Parallel()

	orig := CDERunConfig{
		Engine:     "docker",
		SocketPath: ConfigPath{Raw: "/var/run/docker.sock"},
		Defaults: ConfigDefaults{
			Network: "bridge",
			Devices: []DeviceConfig{
				{Source: ConfigPath{Raw: "/dev/snd"}, Destination: ConfigPath{Raw: "/dev/snd"}},
			},
			Mounts: []MountConfig{
				{Source: ConfigPath{Raw: "/host"}, Target: ConfigPath{Raw: "/container"}},
			},
			Env: []string{"FOO=BAR"},
		},
	}

	cloned := orig.DeepCopy()

	// Mutate clone
	cloned.Engine = "podman"
	cloned.Defaults.Devices[0].Source.Raw = "/dev/null"
	cloned.Defaults.Mounts[0].Source.Raw = "/mutated"
	cloned.Defaults.Env[0] = "FOO=MUTATED"

	// Verify original remains untouched
	assert.Equal(t, "docker", orig.Engine)
	assert.Equal(t, "/dev/snd", orig.Defaults.Devices[0].Source.Raw)
	assert.Equal(t, "/host", orig.Defaults.Mounts[0].Source.Raw)
	assert.Equal(t, "FOO=BAR", orig.Defaults.Env[0])
}
