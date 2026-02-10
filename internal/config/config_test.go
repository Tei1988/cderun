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
				"/app/.cderun.yaml":                         os.ErrNotExist,
				"/home/user/.config/cderun/.cderun.yaml":    os.ErrNotExist,
				"/etc/cderun/.cderun.yaml":                  os.ErrNotExist,
				"/run/cderun/.cderun.yaml":                  os.ErrNotExist,
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
}
