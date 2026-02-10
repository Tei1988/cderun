package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockFileSystem struct {
	OSFileSystem
	wd      string
	home    string
	files   map[string][]byte
	statErr map[string]error
}

func (m *mockFileSystem) Getwd() (string, error)       { return m.wd, nil }
func (m *mockFileSystem) UserHomeDir() (string, error) { return m.home, nil }
func (m *mockFileSystem) Stat(name string) (os.FileInfo, error) {
	if err, ok := m.statErr[name]; ok {
		return nil, err
	}
	if _, ok := m.files[name]; ok {
		return nil, nil // Return nil info as we only check error
	}
	return m.OSFileSystem.Stat(name)
}
func (m *mockFileSystem) ReadFile(name string) ([]byte, error) {
	if data, ok := m.files[name]; ok {
		return data, nil
	}
	return m.OSFileSystem.ReadFile(name)
}

func TestUnit_Config_LoadCDERunConfig(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		fs := &mockFileSystem{
			wd:    "/app",
			home:  "/home/user",
			files: make(map[string][]byte),
			statErr: map[string]error{
				"/app/.cderun.yaml":                      os.ErrNotExist,
				"/home/user/.config/cderun/.cderun.yaml": os.ErrNotExist,
				"/etc/cderun/.cderun.yaml":               os.ErrNotExist,
				"/run/cderun/.cderun.yaml":               os.ErrNotExist,
			},
		}
		loader := &ConfigLoader{
			FS:              fs,
			SystemConfigDir: "/etc/cderun",
			RunConfigDir:    "/run/cderun",
		}

		cfg, paths, err := loader.LoadCDERunConfig()
		assert.NoError(t, err)
		assert.Nil(t, cfg)
		assert.Empty(t, paths)
	})

	t.Run("found in current dir", func(t *testing.T) {
		fs := &mockFileSystem{
			wd:   "/app",
			home: "/home/user",
			files: map[string][]byte{
				"/app/.cderun.yaml": []byte("runtime: docker\ndefaults:\n  tty: true"),
			},
			statErr: map[string]error{
				"/home/user/.config/cderun/.cderun.yaml": os.ErrNotExist,
				"/etc/cderun/.cderun.yaml":               os.ErrNotExist,
				"/run/cderun/.cderun.yaml":               os.ErrNotExist,
			},
		}
		loader := &ConfigLoader{
			FS:              fs,
			SystemConfigDir: "/etc/cderun",
			RunConfigDir:    "/run/cderun",
		}

		cfg, paths, err := loader.LoadCDERunConfig()
		assert.NoError(t, err)
		assert.NotNil(t, cfg)
		require.NotEmpty(t, paths)
		assert.Contains(t, paths[0], ".cderun.yaml")
		assert.Equal(t, "docker", cfg.Runtime)
		assert.True(t, *cfg.Defaults.TTY)
	})

	t.Run("priority and merging", func(t *testing.T) {
		fs := &mockFileSystem{
			wd:   "/app",
			home: "/home/user",
			files: map[string][]byte{
				"/home/user/.config/cderun/.cderun.yaml": []byte("runtime: podman"),
				"/run/cderun/.cderun.yaml":               []byte("runtime: docker\ndefaults:\n  network: host"),
			},
			statErr: map[string]error{
				"/app/.cderun.yaml":        os.ErrNotExist,
				"/etc/cderun/.cderun.yaml": os.ErrNotExist,
			},
		}
		loader := &ConfigLoader{
			FS:              fs,
			SystemConfigDir: "/etc/cderun",
			RunConfigDir:    "/run/cderun",
		}

		cfg, paths, err := loader.LoadCDERunConfig()
		assert.NoError(t, err)
		assert.Equal(t, 2, len(paths))
		assert.Contains(t, paths[0], "/home/user")
		assert.Contains(t, paths[1], "/run/cderun")
		assert.Equal(t, "podman", cfg.Runtime)
		assert.Equal(t, "host", cfg.Defaults.Network)
	})

	t.Run("priority: home over run", func(t *testing.T) {
		homeDir, err := os.MkdirTemp("", "cderun-home-*")
		require.NoError(t, err)
		defer func() { _ = os.RemoveAll(homeDir) }()
		t.Setenv("HOME", homeDir)
		t.Setenv("USERPROFILE", homeDir)

		runDir, err := os.MkdirTemp("", "cderun-run-*")
		require.NoError(t, err)
		defer func() { _ = os.RemoveAll(runDir) }()

		originalRunConfigDir := runConfigDir
		runConfigDir = runDir
		defer func() { runConfigDir = originalRunConfigDir }()

		// Home config
		homeConfigDir := filepath.Join(homeDir, ".config", "cderun")
		require.NoError(t, os.MkdirAll(homeConfigDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(homeConfigDir, ".cderun.yaml"), []byte("runtime: podman"), 0644))

		// Run config
		require.NoError(t, os.WriteFile(filepath.Join(runDir, ".cderun.yaml"), []byte("runtime: docker"), 0644))

		cfg, paths, err := LoadCDERunConfig()
		assert.NoError(t, err)
		require.Equal(t, 2, len(paths))
		// Priority: higher first. Home > Run.
		assert.Contains(t, paths[0], homeDir)
		assert.Contains(t, paths[1], runDir)
		assert.Equal(t, "podman", cfg.Runtime)
	})

	t.Run("HostContext is loaded and merged", func(t *testing.T) {
		runDir, err := os.MkdirTemp("", "cderun-run-*")
		require.NoError(t, err)
		defer func() { _ = os.RemoveAll(runDir) }()

		originalRunConfigDir := runConfigDir
		runConfigDir = runDir
		defer func() { runConfigDir = originalRunConfigDir }()

		content := `
hostContext:
  level: 1
  snapshotDir: /tmp/snap
  mounts:
    - source: /host
      target: /container
      level: 1
`
		err = os.WriteFile(filepath.Join(runDir, ".cderun.yaml"), []byte(content), 0644)
		require.NoError(t, err)

		cfg, _, err := LoadCDERunConfig()
		assert.NoError(t, err)
		require.NotNil(t, cfg)
		require.NotNil(t, cfg.HostContext)
		assert.Equal(t, 1, cfg.HostContext.Level)
		assert.Equal(t, "/tmp/snap", cfg.HostContext.SnapshotDir)
		require.Len(t, cfg.HostContext.Mounts, 1)
		assert.Equal(t, "/host", cfg.HostContext.Mounts[0].Source)
		assert.Equal(t, "/container", cfg.HostContext.Mounts[0].Target)
		assert.Equal(t, 1, cfg.HostContext.Mounts[0].Level)
	})

	t.Run("invalid yaml syntax", func(t *testing.T) {
		err := os.WriteFile(".cderun.yaml", []byte("invalid: yaml: ["), 0644)
		require.NoError(t, err)
		defer func() { _ = os.Remove(".cderun.yaml") }()

		cfg, paths, err := LoadCDERunConfig()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal")
		assert.Nil(t, cfg)
		assert.Nil(t, paths)
	})

	t.Run("unknown field", func(t *testing.T) {
		err := os.WriteFile(".cderun.yaml", []byte("unknown_field: value"), 0644)
		require.NoError(t, err)
		defer func() { _ = os.Remove(".cderun.yaml") }()

		cfg, paths, err := LoadCDERunConfig()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "field unknown_field not found")
		assert.Nil(t, cfg)
		assert.Nil(t, paths)
	})
}

func TestUnit_Config_LoadToolsConfig(t *testing.T) {
	t.Run("found in current dir", func(t *testing.T) {
		fs := &mockFileSystem{
			wd: "/app",
			files: map[string][]byte{
				"/app/.tools.yaml": []byte("node:\n  image: node:20-alpine\n  tty: true"),
			},
			statErr: map[string]error{
				"/home/user/.config/cderun/.tools.yaml": os.ErrNotExist,
				"/etc/cderun/.tools.yaml":               os.ErrNotExist,
				"/run/cderun/.tools.yaml":               os.ErrNotExist,
			},
		}
		loader := &ConfigLoader{
			FS:              fs,
			SystemConfigDir: "/etc/cderun",
			RunConfigDir:    "/run/cderun",
		}

		cfg, paths, err := loader.LoadToolsConfig()
		assert.NoError(t, err)
		assert.NotNil(t, cfg)
		require.NotEmpty(t, paths)
		tool, ok := cfg["node"]
		assert.True(t, ok)
		assert.Equal(t, "node:20-alpine", tool.Image)
		assert.True(t, *tool.TTY)
	})

	t.Run("found in run dir", func(t *testing.T) {
		runDir, err := os.MkdirTemp("", "cderun-run-*")
		require.NoError(t, err)
		defer func() { _ = os.RemoveAll(runDir) }()

		originalRunConfigDir := runConfigDir
		runConfigDir = runDir
		defer func() { runConfigDir = originalRunConfigDir }()

		content := `
node:
  image: node:18-alpine
`
		err = os.WriteFile(filepath.Join(runDir, ".tools.yaml"), []byte(content), 0644)
		require.NoError(t, err)

		cfg, paths, err := LoadToolsConfig()
		assert.NoError(t, err)
		assert.NotNil(t, cfg)
		require.NotEmpty(t, paths)
		assert.Contains(t, paths[0], ".tools.yaml")
		assert.Equal(t, "node:18-alpine", cfg["node"].Image)
	})

	t.Run("unknown field in tool config", func(t *testing.T) {
		content := `
node:
  image: alpine
  unknown_field: value
`
		err := os.WriteFile(".tools.yaml", []byte(content), 0644)
		require.NoError(t, err)
		defer func() { _ = os.Remove(".tools.yaml") }()

		cfg, paths, err := LoadToolsConfig()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "field unknown_field not found")
		assert.Nil(t, cfg)
		assert.Nil(t, paths)
	})
}
