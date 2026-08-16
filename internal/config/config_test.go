package config

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type customMockFS struct {
	MockFileSystem
	homeDirErr  error
	absErr      error
	readFileErr error
}

func (m *customMockFS) UserHomeDir() (string, error) {
	if m.homeDirErr != nil {
		return "", m.homeDirErr
	}
	return m.MockFileSystem.UserHomeDir()
}

func (m *customMockFS) Abs(path string) (string, error) {
	if m.absErr != nil {
		return "", m.absErr
	}
	return m.MockFileSystem.Abs(path)
}

func (m *customMockFS) ReadFile(name string) ([]byte, error) {
	if m.readFileErr != nil {
		return nil, m.readFileErr
	}
	return m.MockFileSystem.ReadFile(name)
}

func TestUnit_Config_LoadCDERun(t *testing.T) {
	t.Run("ReadFile error", func(t *testing.T) {
		mfs := &customMockFS{
			MockFileSystem: MockFileSystem{
				Files: map[string][]byte{"/project/.cderun.yaml": []byte("runtime: docker")},
				WD:    "/project",
			},
			readFileErr: assert.AnError,
		}
		loader := &ConfigLoader{fs: mfs}
		_, _, err := loader.LoadCDERunConfig()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read config file")
	})

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
		assert.Equal(t, "docker", cfg.Engine)
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
		assert.Equal(t, "podman", cfg.Engine)
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

	t.Run("FindConfigs hierarchical search", func(t *testing.T) {
		mfs := &MockFileSystem{
			Dirs: map[string]bool{
				"/a/b/c": true,
				"/a/b":   true,
				"/a":     true,
				"/":      true,
			},
			Files: map[string][]byte{
				"/a/b/c/.cderun.yaml": []byte(""),
				"/a/.cderun.yaml":     []byte(""),
				"/.cderun.yaml":       []byte(""),
			},
			WD: "/a/b/c",
		}
		loader := &ConfigLoader{fs: mfs, systemConfigDir: "/etc/cderun", runConfigDir: "/run/cderun"}
		paths := loader.FindConfigs(".cderun.yaml")

		// Assert exact order: current dir -> parents -> root
		expected := []string{
			"/a/b/c/.cderun.yaml",
			"/a/.cderun.yaml",
			"/.cderun.yaml",
		}
		assert.Equal(t, expected, paths)
	})

	t.Run("FindConfigs Abs failure", func(t *testing.T) {
		mfs := &customMockFS{
			MockFileSystem: MockFileSystem{
				Files: map[string][]byte{"/a/.cderun.yaml": []byte("")},
				Dirs:  map[string]bool{"/a": true},
				WD:    "/a",
			},
			absErr: assert.AnError,
		}
		loader := &ConfigLoader{fs: mfs}
		paths := loader.FindConfigs(".cderun.yaml")
		assert.Contains(t, paths, "/a/.cderun.yaml") // Still included even if Abs fails, using the calculated path
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

	t.Run("BaseDir pollution check on merge", func(t *testing.T) {
		globalContent := `
defaults:
  mountCderunPath: cderun
  mountSocketPath: socket
`
		projectContent := `
runtime: docker
`
		mfs := &MockFileSystem{
			Files: map[string][]byte{
				"/home/user/.config/cderun/.cderun.yaml": []byte(globalContent),
				"/project/.cderun.yaml":                  []byte(projectContent),
			},
			Dirs: map[string]bool{
				"/project": true,
			},
			WD:      "/project",
			HomeDir: "/home/user",
		}
		loader := &ConfigLoader{fs: mfs, systemConfigDir: "/etc/cderun", runConfigDir: "/run/cderun"}
		cfg, _, err := loader.LoadCDERunConfig()
		require.NoError(t, err)
		require.NotNil(t, cfg)

		// Check mountCderunPath
		assert.Equal(t, "cderun", cfg.Defaults.MountCderunPath.Raw)
		assert.Equal(t, "/home/user/.config/cderun", cfg.Defaults.MountCderunPath.BaseDir)

		// Check mountSocketPath
		assert.Equal(t, "socket", cfg.Defaults.MountSocketPath.Raw)
		assert.Equal(t, "/home/user/.config/cderun", cfg.Defaults.MountSocketPath.BaseDir)
	})
}

func TestUnit_Config_LoadTools(t *testing.T) {
	t.Run("ReadFile error", func(t *testing.T) {
		mfs := &customMockFS{
			MockFileSystem: MockFileSystem{
				Files: map[string][]byte{"/project/.tools.yaml": []byte("node: {image: node}")},
				WD:    "/project",
			},
			readFileErr: assert.AnError,
		}
		loader := &ConfigLoader{fs: mfs}
		_, _, err := loader.LoadToolsConfig()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read tools file")
	})

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
		assert.Equal(t, "podman", cfg.Engine)
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

	t.Run("LoadCDERunConfigFromPath - homeDir failure", func(t *testing.T) {
		mfs := &customMockFS{
			homeDirErr: assert.AnError,
		}
		loader := &ConfigLoader{fs: mfs}
		_, _, err := loader.LoadCDERunConfigFromPath("~/config.yaml")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get home directory")
	})

	t.Run("LoadCDERunConfigFromPath - Abs failure", func(t *testing.T) {
		mfs := &customMockFS{
			absErr: assert.AnError,
		}
		loader := &ConfigLoader{fs: mfs}
		_, _, err := loader.LoadCDERunConfigFromPath("/config.yaml")
		require.Error(t, err)
	})

	t.Run("LoadCDERunConfigFromPath - ReadFile failure", func(t *testing.T) {
		mfs := &customMockFS{
			MockFileSystem: MockFileSystem{
				Files: map[string][]byte{"/config.yaml": []byte("foo")},
				WD:    "/",
			},
			readFileErr: assert.AnError,
		}
		loader := &ConfigLoader{fs: mfs}
		_, _, err := loader.LoadCDERunConfigFromPath("/config.yaml")
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

	t.Run("LoadToolsConfigFromPath - homeDir failure", func(t *testing.T) {
		mfs := &customMockFS{
			homeDirErr: assert.AnError,
		}
		loader := &ConfigLoader{fs: mfs}
		_, _, err := loader.LoadToolsConfigFromPath("~/tools.yaml")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get home directory")
	})

	t.Run("LoadToolsConfigFromPath - Abs failure", func(t *testing.T) {
		mfs := &customMockFS{
			absErr: assert.AnError,
		}
		loader := &ConfigLoader{fs: mfs}
		_, _, err := loader.LoadToolsConfigFromPath("/tools.yaml")
		require.Error(t, err)
	})

	t.Run("LoadToolsConfigFromPath - ReadFile failure", func(t *testing.T) {
		mfs := &customMockFS{
			MockFileSystem: MockFileSystem{
				Files: map[string][]byte{"/tools.yaml": []byte("foo")},
				WD:    "/",
			},
			readFileErr: assert.AnError,
		}
		loader := &ConfigLoader{fs: mfs}
		_, _, err := loader.LoadToolsConfigFromPath("/tools.yaml")
		require.Error(t, err)
	})
}

func TestUnit_Config_LoadCDERunErrors(t *testing.T) {
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

func TestUnit_Config_SetBaseDir(t *testing.T) {
	t.Run("CDERunConfig", func(t *testing.T) {
		cfg := &CDERunConfig{
			SocketPath: ConfigPath{Raw: "/sock"},
			Defaults: ConfigDefaults{
				MountCderunPath: ConfigPath{Raw: "cderun"},
				MountSocketPath: ConfigPath{Raw: "socket"},
				Mounts: []MountConfig{
					{Source: ConfigPath{Raw: "src"}, Target: ConfigPath{Raw: "dst"}},
				},
				Devices: []DeviceConfig{
					{Source: ConfigPath{Raw: "dev_src"}, Destination: ConfigPath{Raw: "dev_dst"}},
				},
			},
			HostContext: &HostContext{
				Mounts: []MountMapping{
					{Source: "./relative"},
				},
			},
		}
		err := cfg.SetBaseDir("/base")
		require.NoError(t, err)
		assert.Equal(t, "/base", cfg.SocketPath.BaseDir)
		assert.Equal(t, "/base", cfg.Defaults.MountCderunPath.BaseDir)
		assert.Equal(t, "/base", cfg.Defaults.MountSocketPath.BaseDir)
		assert.Equal(t, "/base", cfg.Defaults.Mounts[0].Source.BaseDir)
		assert.Empty(t, cfg.Defaults.Mounts[0].Target.BaseDir)
		assert.Equal(t, "/base", cfg.Defaults.Devices[0].Source.BaseDir)
		assert.Empty(t, cfg.Defaults.Devices[0].Destination.BaseDir)
		assert.Equal(t, "/base/relative", cfg.HostContext.Mounts[0].Source)
	})

	t.Run("ToolConfig", func(t *testing.T) {
		tc := &ToolConfig{
			MountCderunPath: ConfigPath{Raw: "cderun"},
			MountSocketPath: ConfigPath{Raw: "socket"},
			Mounts: []MountConfig{
				{Source: ConfigPath{Raw: "src"}, Target: ConfigPath{Raw: "dst"}},
			},
			Devices: []DeviceConfig{
				{Source: ConfigPath{Raw: "dev_src"}, Destination: ConfigPath{Raw: "dev_dst"}},
			},
		}
		tc.SetBaseDir("/base")
		assert.Equal(t, "/base", tc.MountCderunPath.BaseDir)
		assert.Equal(t, "/base", tc.MountSocketPath.BaseDir)
		assert.Equal(t, "/base", tc.Mounts[0].Source.BaseDir)
		assert.Empty(t, tc.Mounts[0].Target.BaseDir)
		assert.Equal(t, "/base", tc.Devices[0].Source.BaseDir)
		assert.Empty(t, tc.Devices[0].Destination.BaseDir)
	})

	t.Run("CDERunConfig SetBaseDir HostContext resolution", func(t *testing.T) {
		cfg := &CDERunConfig{
			HostContext: &HostContext{
				Mounts: []MountMapping{{Source: "/absolute/path"}},
			},
		}
		// SetBaseDir calls ResolvePath(..., nil) which uses RealFileSystem.
		err := cfg.SetBaseDir("/base")
		require.NoError(t, err)
		assert.Equal(t, "/absolute/path", cfg.HostContext.Mounts[0].Source)

		cfg.HostContext.Mounts[0].Source = "./rel"
		err = cfg.SetBaseDir("/base")
		require.NoError(t, err)
		assert.Equal(t, "/base/rel", cfg.HostContext.Mounts[0].Source)
	})
}

func TestUnit_Config_Helpers(t *testing.T) {
	t.Run("copyFloat64Ptr non-nil", func(t *testing.T) {
		f := 1.5
		res := copyFloat64Ptr(&f)
		assert.NotNil(t, res)
		assert.InDelta(t, f, *res, 1e-9)
		assert.NotSame(t, &f, res)
	})

	t.Run("copyFloat64Ptr nil", func(t *testing.T) {
		assert.Nil(t, copyFloat64Ptr(nil))
	})

	t.Run("copyIntPtr nil", func(t *testing.T) {
		assert.Nil(t, copyIntPtr(nil))
	})

	t.Run("copyIntPtr non-nil", func(t *testing.T) {
		i := 123
		res := copyIntPtr(&i)
		assert.NotNil(t, res)
		assert.Equal(t, i, *res)
		assert.NotSame(t, &i, res)
	})
}

func assertDeepCopyDistinct(t *testing.T, orig, cloned any) {
	t.Helper()

	vOrig := reflect.ValueOf(orig)
	vCloned := reflect.ValueOf(cloned)

	if !vOrig.IsValid() {
		assert.False(t, vCloned.IsValid())
		return
	}

	assert.Equal(t, orig, cloned)

	if vOrig.Kind() == reflect.Map {
		if !vOrig.IsNil() {
			if vOrig.Pointer() == vCloned.Pointer() {
				assert.Fail(t, "Map should have different internal pointers")
			}
			for _, k := range vOrig.MapKeys() {
				assertDeepCopyDistinct(t, vOrig.MapIndex(k).Interface(), vCloned.MapIndex(k).Interface())
			}
		}
		return
	}

	if vOrig.Kind() == reflect.Pointer {
		if !vOrig.IsNil() {
			assert.NotSame(t, vOrig.Interface(), vCloned.Interface(), "Pointer should be different")
			assertDeepCopyDistinct(t, vOrig.Elem().Interface(), vCloned.Elem().Interface())
		}
		return
	}

	if vOrig.Kind() != reflect.Struct {
		return
	}

	for i := 0; i < vOrig.NumField(); i++ {
		fOrig := vOrig.Field(i)
		fCloned := vCloned.Field(i)
		fieldName := vOrig.Type().Field(i).Name

		// We only check kinds that need deep-copy verification (pointers, slices, maps, nested structs).
		// Other kinds are already covered by the top-level assert.Equal.
		//nolint:exhaustive
		switch fOrig.Kind() {
		case reflect.Pointer:
			if !fOrig.IsNil() {
				assert.NotSame(t, fOrig.Interface(), fCloned.Interface(), "Field %s should be a different pointer", fieldName)
			}
		case reflect.Slice:
			if !fOrig.IsNil() && fOrig.Len() > 0 {
				assert.NotSame(t, fOrig.Index(0).Addr().Interface(), fCloned.Index(0).Addr().Interface(), "Field %s slice elements should have different addresses", fieldName)
			}
		case reflect.Struct:
			// Known leaf structs that don't need internal pointer check or have their own logic
			if vOrig.Type().Field(i).Type.Name() != "ConfigPath" {
				assertDeepCopyDistinct(t, fOrig.Interface(), fCloned.Interface())
			}
		case reflect.Map:
			if !fOrig.IsNil() && fOrig.Len() > 0 {
				if fOrig.Pointer() == fCloned.Pointer() {
					assert.Failf(t, "Map deep-copy failed", "Map %s should have different internal pointers", fieldName)
				}
			}
		default:
			// No deep-copy verification needed for other types
		}
	}
}

func TestUnit_Config_DeepCopy(t *testing.T) {
	t.Run("CDERunConfig DeepCopy", func(t *testing.T) {
		tty := true
		orig := CDERunConfig{
			Engine:  "docker",
			Defaults: ConfigDefaults{
				TTY: &tty,
				Env: []string{"A=1"},
			},
			HostContext: &HostContext{
				Level:  1,
				Mounts: []MountMapping{{Source: "/a", Target: "/b"}},
			},
		}

		cloned := orig.DeepCopy()
		assertDeepCopyDistinct(t, orig, cloned)

		// Mutate cloned and ensure original is unchanged
		*cloned.Defaults.TTY = false
		cloned.Defaults.Env[0] = "A=2"
		cloned.HostContext.Level = 2

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

		cloned := orig.DeepCopy()
		assertDeepCopyDistinct(t, orig, cloned)

		nodeCloned := cloned["node"]
		nodeCloned.Env[0] = "NODE_ENV=prod"
		cloned["node"] = nodeCloned

		assert.Equal(t, "NODE_ENV=dev", orig["node"].Env[0])
	})

	t.Run("ToolsConfig DeepCopy nil", func(t *testing.T) {
		var orig ToolsConfig
		cloned := orig.DeepCopy()
		assertDeepCopyDistinct(t, orig, cloned)
	})

	t.Run("DeepCopy all fields", func(t *testing.T) {
		b := true
		f := 2.0
		orig := CDERunConfig{
			Logging: LoggingConfig{
				Timestamp: &b,
			},
			Defaults: ConfigDefaults{
				TTY:             &b,
				Interactive:     &b,
				Remove:          &b,
				StrictEnv:       &b,
				MountCderun:     &b,
				MountSocket:     &b,
				MountAllTools:   &b,
				PublishAll:      &b,
				Privileged:      &b,
				DryRun:          &b,
				Diagnosis:       &b,
				CPUs:            &f,
				MountTools:      []string{"t"},
				Ports:           []string{"p"},
				Expose:          []string{"e"},
				DNS:             []string{"d"},
				AddHosts:        []string{"a"},
				CapAdd:          []string{"ca"},
				CapDrop:         []string{"cd"},
				Entrypoint:      []string{"ep"},
				Env:             []string{"ev"},
				Mounts:          []MountConfig{{Type: "bind", Target: ConfigPath{Raw: "/t"}}},
				Devices:         []DeviceConfig{{Source: ConfigPath{Raw: "/s"}, Destination: ConfigPath{Raw: "/d"}}},
				MountCderunPath: ConfigPath{Raw: "rcp"},
				MountSocketPath: ConfigPath{Raw: "rsp"},
			},
		}

		cloned := orig.DeepCopy()
		assert.Equal(t, orig, cloned)
		assertDeepCopyDistinct(t, orig, cloned)
	})

	t.Run("ToolConfig DeepCopy all fields", func(t *testing.T) {
		b := true
		f := 2.0
		orig := ToolConfig{
			TTY:             &b,
			Interactive:     &b,
			Remove:          &b,
			StrictEnv:       &b,
			MountCderun:     &b,
			MountSocket:     &b,
			MountAllTools:   &b,
			PublishAll:      &b,
			Privileged:      &b,
			LogTimestamp:    &b,
			DryRun:          &b,
			Diagnosis:       &b,
			CPUs:            &f,
			MountTools:      []string{"t"},
			Ports:           []string{"p"},
			Expose:          []string{"e"},
			DNS:             []string{"d"},
			AddHosts:        []string{"a"},
			CapAdd:          []string{"ca"},
			CapDrop:         []string{"cd"},
			Entrypoint:      []string{"ep"},
			Env:             []string{"ev"},
			Mounts:          []MountConfig{{Type: "bind", Target: ConfigPath{Raw: "/t"}}},
			Devices:         []DeviceConfig{{Source: ConfigPath{Raw: "/s"}, Destination: ConfigPath{Raw: "/d"}}},
			MountCderunPath: ConfigPath{Raw: "rcp"},
			MountSocketPath: ConfigPath{Raw: "rsp"},
		}

		cloned := orig.DeepCopy()
		assert.Equal(t, orig, cloned)
		assertDeepCopyDistinct(t, orig, cloned)
	})

}

func TestUnit_Config_FindConfigs_Cache(t *testing.T) {
	fs := &MockFileSystem{
		WD:      "/work",
		HomeDir: "/home/user",
		Dirs: map[string]bool{
			"/work":      true,
			"/home/user": true,
		},
		Files: map[string][]byte{
			"/work/.cderun.yaml": []byte("runtime: docker"),
		},
	}
	loader := NewConfigLoaderWithFS(fs)

	// First call to FindConfigs
	paths1 := loader.FindConfigs(".cderun.yaml")
	assert.Len(t, paths1, 1)
	assert.Equal(t, "/work/.cderun.yaml", paths1[0])

	count1 := len(fs.StatCalls)
	assert.Positive(t, count1)

	// Second call to FindConfigs for the same file should use cache
	paths2 := loader.FindConfigs(".cderun.yaml")
	assert.Equal(t, paths1, paths2)

	count2 := len(fs.StatCalls)
	assert.Equal(t, count1, count2, "Stat should not be called again for cached paths")

	// Call for a different file should trigger more Stat calls
	paths3 := loader.FindConfigs(".tools.yaml")
	assert.Empty(t, paths3)
	count3 := len(fs.StatCalls)
	assert.Greater(t, count3, count2, "Stat should be called for new filename")
}

func TestUnit_Config_CachedStat_DoubleCheck(t *testing.T) {
	fs := &MockFileSystem{
		Files: map[string][]byte{"/test": []byte("foo")},
	}
	loader := NewConfigLoaderWithFS(fs)

	// Access via cachedStat directly
	_, err := loader.cachedStat("/test")
	require.NoError(t, err)

	// stats should be 1
	assert.Len(t, fs.StatCalls, 1)

	// Access again
	_, err = loader.cachedStat("/test")
	require.NoError(t, err)
	assert.Len(t, fs.StatCalls, 1)
}
