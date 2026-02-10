package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockFileSystem struct {
	Files   map[string][]byte
	Dirs    map[string]bool
	WD      string
	HomeDir string
}

func (m *MockFileSystem) Getwd() (string, error) {
	return m.WD, nil
}

type mockFileInfo struct {
	os.FileInfo
}

func (m *MockFileSystem) Stat(name string) (os.FileInfo, error) {
	if _, ok := m.Files[name]; ok {
		return &mockFileInfo{}, nil
	}
	if m.Dirs[name] {
		return &mockFileInfo{}, nil
	}
	return nil, os.ErrNotExist
}

func (m *MockFileSystem) ReadFile(name string) ([]byte, error) {
	if data, ok := m.Files[name]; ok {
		return data, nil
	}
	return nil, os.ErrNotExist
}

func (m *MockFileSystem) UserHomeDir() (string, error) {
	return m.HomeDir, nil
}

func TestUnit_Config_Load_CDERunConfig(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		mfs := &MockFileSystem{
			Files: make(map[string][]byte),
			Dirs:  map[string]bool{"/project": true},
			WD:    "/project",
		}
		loader := &ConfigLoader{fs: mfs, systemConfigDir: "/etc/cderun", runConfigDir: "/run/cderun"}
		cfg, paths, err := loader.LoadCDERunConfig()
		assert.NoError(t, err)
		assert.Nil(t, cfg)
		assert.Empty(t, paths)
	})

	t.Run("found in current dir", func(t *testing.T) {
		content := `
runtime: docker
defaults:
  tty: true
`
		mfs := &MockFileSystem{
			Files: map[string][]byte{
				"/project/.cderun.yaml": []byte(content),
			},
			Dirs: map[string]bool{"/project": true},
			WD:   "/project",
		}
		loader := &ConfigLoader{fs: mfs, systemConfigDir: "/etc/cderun", runConfigDir: "/run/cderun"}
		cfg, paths, err := loader.LoadCDERunConfig()
		assert.NoError(t, err)
		assert.NotNil(t, cfg)
		require.NotEmpty(t, paths)
		assert.Equal(t, "/project/.cderun.yaml", paths[0])
		assert.Equal(t, "docker", cfg.Runtime)
		assert.True(t, *cfg.Defaults.TTY)
	})

	t.Run("found in home dir", func(t *testing.T) {
		mfs := &MockFileSystem{
			Files: map[string][]byte{
				"/home/user/.config/cderun/.cderun.yaml": []byte("runtime: podman"),
			},
			Dirs:    map[string]bool{"/project": true},
			WD:      "/project",
			HomeDir: "/home/user",
		}
		loader := &ConfigLoader{fs: mfs, systemConfigDir: "/etc/cderun", runConfigDir: "/run/cderun"}
		cfg, paths, err := loader.LoadCDERunConfig()
		assert.NoError(t, err)
		assert.NotNil(t, cfg)
		require.NotEmpty(t, paths)
		assert.Equal(t, "/home/user/.config/cderun/.cderun.yaml", paths[0])
		assert.Equal(t, "podman", cfg.Runtime)
	})

	t.Run("found in run dir", func(t *testing.T) {
		mfs := &MockFileSystem{
			Files: map[string][]byte{
				"/run/cderun/.cderun.yaml": []byte("defaults:\n  network: host"),
			},
			Dirs: map[string]bool{"/project": true},
			WD:   "/project",
		}
		loader := &ConfigLoader{fs: mfs, systemConfigDir: "/etc/cderun", runConfigDir: "/run/cderun"}
		cfg, paths, err := loader.LoadCDERunConfig()
		assert.NoError(t, err)
		assert.NotNil(t, cfg)
		require.NotEmpty(t, paths)
		assert.Equal(t, "/run/cderun/.cderun.yaml", paths[0])
		assert.Equal(t, "host", cfg.Defaults.Network)
	})

	t.Run("HostContext is loaded and merged", func(t *testing.T) {
		content := `
hostContext:
  level: 1
  snapshotDir: /tmp/snap
  mounts:
    - source: /host
      target: /container
      level: 1
`
		mfs := &MockFileSystem{
			Files: map[string][]byte{
				"/run/cderun/.cderun.yaml": []byte(content),
			},
			Dirs: map[string]bool{"/project": true},
			WD:   "/project",
		}
		loader := &ConfigLoader{fs: mfs, systemConfigDir: "/etc/cderun", runConfigDir: "/run/cderun"}
		cfg, _, err := loader.LoadCDERunConfig()
		assert.NoError(t, err)
		require.NotNil(t, cfg)
		require.NotNil(t, cfg.HostContext)
		assert.Equal(t, 1, cfg.HostContext.Level)
		assert.Equal(t, "/tmp/snap", cfg.HostContext.SnapshotDir)
		require.Len(t, cfg.HostContext.Mounts, 1)
		assert.Equal(t, "/host", cfg.HostContext.Mounts[0].Source)
	})
}

func TestUnit_Config_Load_ToolsConfig(t *testing.T) {
	t.Run("found in current dir", func(t *testing.T) {
		content := `
node:
  image: node:20-alpine
  tty: true
`
		mfs := &MockFileSystem{
			Files: map[string][]byte{
				"/project/.tools.yaml": []byte(content),
			},
			Dirs: map[string]bool{"/project": true},
			WD:   "/project",
		}
		loader := &ConfigLoader{fs: mfs, systemConfigDir: "/etc/cderun", runConfigDir: "/run/cderun"}
		cfg, paths, err := loader.LoadToolsConfig()
		assert.NoError(t, err)
		assert.NotNil(t, cfg)
		require.NotEmpty(t, paths)
		assert.Equal(t, "/project/.tools.yaml", paths[0])
		tool, ok := cfg["node"]
		assert.True(t, ok)
		assert.Equal(t, "node:20-alpine", tool.Image)
	})
}

func TestUnit_Config_RealFS_Integration(t *testing.T) {
	// Keep one test with real filesystem to ensure RealFileSystem works
	tmpDir, err := os.MkdirTemp("", "cderun-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	content := "runtime: docker"
	err = os.WriteFile(filepath.Join(tmpDir, ".cderun.yaml"), []byte(content), 0644)
	require.NoError(t, err)

	originalWd, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() { _ = os.Chdir(originalWd) })

	cfg, paths, err := LoadCDERunConfig()
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.NotEmpty(t, paths)
}
