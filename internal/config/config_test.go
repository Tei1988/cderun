package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_LoadCDERun(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		mfs := &MockFileSystem{
			Files: make(map[string][]byte),
			Dirs:  map[string]bool{"/project": true},
			WD:    "/project",
		}
		loader := &ConfigLoader{fs: mfs, systemConfigDir: "/etc/cderun", runConfigDir: "/run/cderun"}
		cfg, paths, err := loader.LoadCDERunConfig()
		require.NoError(t, err)
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
		require.NoError(t, err)
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
		require.NoError(t, err)
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
		require.NoError(t, err)
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
		require.NoError(t, err)
		require.NotNil(t, cfg)
		require.NotNil(t, cfg.HostContext)
		assert.Equal(t, 1, cfg.HostContext.Level)
		assert.Equal(t, "/tmp/snap", cfg.HostContext.SnapshotDir)
		require.Len(t, cfg.HostContext.Mounts, 1)
		assert.Equal(t, "/host", cfg.HostContext.Mounts[0].Source)
	})
}

func TestUnit_Config_LoadTools(t *testing.T) {
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
		require.NoError(t, err)
		assert.NotNil(t, cfg)
		require.NotEmpty(t, paths)
		assert.Equal(t, "/project/.tools.yaml", paths[0])
		tool, ok := cfg["node"]
		assert.True(t, ok)
		assert.Equal(t, "node:20-alpine", tool.Image)
	})
}

func TestUnit_Config_SetDirs(t *testing.T) {
	t.Run("SetRunConfigDirForTest", func(t *testing.T) {
		original := defaultLoader.runConfigDir
		cleanup := SetRunConfigDirForTest("/tmp/run")
		assert.Equal(t, "/tmp/run", defaultLoader.runConfigDir)
		cleanup()
		assert.Equal(t, original, defaultLoader.runConfigDir)
	})

	t.Run("SetSystemConfigDirForTest", func(t *testing.T) {
		original := defaultLoader.systemConfigDir
		cleanup := SetSystemConfigDirForTest("/tmp/system")
		assert.Equal(t, "/tmp/system", defaultLoader.systemConfigDir)
		cleanup()
		assert.Equal(t, original, defaultLoader.systemConfigDir)
	})
}

func TestUnit_Config_LoadPath(t *testing.T) {
	t.Run("LoadCDERunConfigFromPath", func(t *testing.T) {
		content := `
runtime: podman
defaults:
  tty: true
`
		mfs := &MockFileSystem{
			Files: map[string][]byte{
				"/custom/cderun.yaml": []byte(content),
			},
			Dirs: map[string]bool{"/custom": true},
			WD:   "/project",
		}
		loader := &ConfigLoader{fs: mfs}
		cfg, paths, err := loader.LoadCDERunConfigFromPath("/custom/cderun.yaml")
		require.NoError(t, err)
		assert.NotNil(t, cfg)
		assert.Equal(t, []string{"/custom/cderun.yaml"}, paths)
		assert.Equal(t, "podman", cfg.Runtime)
		assert.True(t, *cfg.Defaults.TTY)
	})

	t.Run("LoadCDERunConfigFromPath - missing", func(t *testing.T) {
		mfs := &MockFileSystem{
			Files: make(map[string][]byte),
			WD:    "/project",
		}
		loader := &ConfigLoader{fs: mfs}
		_, _, err := loader.LoadCDERunConfigFromPath("/missing.yaml")
		require.Error(t, err)
	})

	t.Run("LoadToolsConfigFromPath", func(t *testing.T) {
		content := `
node:
  image: node:20-alpine
`
		mfs := &MockFileSystem{
			Files: map[string][]byte{
				"/custom/tools.yaml": []byte(content),
			},
			Dirs: map[string]bool{"/custom": true},
			WD:   "/project",
		}
		loader := &ConfigLoader{fs: mfs}
		cfg, paths, err := loader.LoadToolsConfigFromPath("/custom/tools.yaml")
		require.NoError(t, err)
		assert.NotNil(t, cfg)
		assert.Equal(t, []string{"/custom/tools.yaml"}, paths)
		assert.Equal(t, "node:20-alpine", cfg["node"].Image)
	})

	t.Run("LoadToolsConfigFromPath - missing", func(t *testing.T) {
		mfs := &MockFileSystem{
			Files: make(map[string][]byte),
			WD:    "/project",
		}
		loader := &ConfigLoader{fs: mfs}
		_, _, err := loader.LoadToolsConfigFromPath("/missing.yaml")
		require.Error(t, err)
	})

	t.Run("LoadCDERunConfig - malformed YAML", func(t *testing.T) {
		mfs := &MockFileSystem{
			Files: map[string][]byte{
				"/project/.cderun.yaml": []byte("invalid: yaml: ["),
			},
			WD: "/project",
		}
		loader := &ConfigLoader{fs: mfs}
		_, _, err := loader.LoadCDERunConfig()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal")
	})

	t.Run("LoadCDERunConfig - unknown field", func(t *testing.T) {
		mfs := &MockFileSystem{
			Files: map[string][]byte{
				"/project/.cderun.yaml": []byte("unknown_field: true"),
			},
			WD: "/project",
		}
		loader := &ConfigLoader{fs: mfs}
		_, _, err := loader.LoadCDERunConfig()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found in type config.CDERunConfig")
	})
}

func TestUnit_Config_DeepCopy(t *testing.T) {
	t.Run("CDERunConfig DeepCopy", func(t *testing.T) {
		tty := true
		orig := CDERunConfig{
			Runtime: "docker",
			Defaults: ConfigDefaults{
				TTY: &tty,
				Env: []string{"A=1"},
			},
			HostContext: &HostContext{
				Level:  1,
				Mounts: []MountMapping{{Source: "/a", Target: "/b"}},
			},
		}

		copy := orig.DeepCopy()

		// Verify deep copy of HostContext
		assert.NotSame(t, orig.HostContext, copy.HostContext)
		assert.Equal(t, orig.HostContext.Level, copy.HostContext.Level)
		assert.NotSame(t, &orig.HostContext.Mounts[0], &copy.HostContext.Mounts[0])

		// Verify deep copy of *bool
		assert.NotSame(t, orig.Defaults.TTY, copy.Defaults.TTY)
		assert.Equal(t, *orig.Defaults.TTY, *copy.Defaults.TTY)

		// Verify deep copy of slice
		assert.NotSame(t, &orig.Defaults.Env[0], &copy.Defaults.Env[0])
		assert.Equal(t, orig.Defaults.Env, copy.Defaults.Env)

		// Mutate copy and ensure original is unchanged
		*copy.Defaults.TTY = false
		copy.Defaults.Env[0] = "A=2"
		copy.HostContext.Level = 2

		assert.True(t, *orig.Defaults.TTY)
		assert.Equal(t, "A=1", orig.Defaults.Env[0])
		assert.Equal(t, 1, orig.HostContext.Level)
	})

	t.Run("ToolsConfig DeepCopy", func(t *testing.T) {
		orig := ToolsConfig{
			"node": ToolConfig{
				Image: "node:20",
				Env:   []string{"NODE_ENV=dev"},
			},
		}

		copy := orig.DeepCopy()

		assert.NotSame(t, &orig, &copy)
		nodeOrig := orig["node"]
		nodeCopy := copy["node"]
		assert.NotSame(t, &nodeOrig.Env[0], &nodeCopy.Env[0])

		nodeCopy.Env[0] = "NODE_ENV=prod"
		copy["node"] = nodeCopy

		assert.Equal(t, "NODE_ENV=dev", orig["node"].Env[0])
	})

	t.Run("DeepCopy all fields", func(t *testing.T) {
		b := true
		orig := CDERunConfig{
			Defaults: ConfigDefaults{
				TTY:             &b,
				Interactive:     &b,
				Remove:          &b,
				StrictEnv:       &b,
				MountCderun:     &b,
				MountSocket:     &b,
				MountAllTools:   &b,
				Privileged:      &b,
				MountTools:      []string{"t"},
				Ports:           []string{"p"},
				Expose:          []string{"e"},
				DNS:             []string{"d"},
				AddHosts:        []string{"a"},
				CapAdd:          []string{"ca"},
				CapDrop:         []string{"cd"},
				Entrypoint:      []string{"ep"},
				Command:         []string{"c"},
				Env:             []string{"ev"},
				Mounts:          []MountConfig{{Type: "bind"}},
				Devices:         []DeviceConfig{{Permissions: "r"}},
				MountCderunPath: ConfigPath{Raw: "rcp"},
				MountSocketPath: ConfigPath{Raw: "rsp"},
			},
		}

		copy := orig.DeepCopy()
		assert.Equal(t, orig, copy)
		assert.NotSame(t, orig.Defaults.TTY, copy.Defaults.TTY)
		assert.NotSame(t, &orig.Defaults.MountTools[0], &copy.Defaults.MountTools[0])
		assert.NotSame(t, &orig.Defaults.Mounts[0], &copy.Defaults.Mounts[0])
		assert.NotSame(t, &orig.Defaults.Devices[0], &copy.Defaults.Devices[0])
	})
}
