package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)


func TestScenario_ConfigResolution_ComplexOverrides(t *testing.T) {
	t.Parallel()

	t.Run("P1 through P5 priority with complex expressions", func(t *testing.T) {
		// Given: A complex environment with multiple configuration layers and expressions
		mfs := &MockFileSystem{
			WD: "/home/user/project",
			Files: map[string][]byte{
				"/home/user/project/.go-version": []byte("1.25"),
			},
			Env: map[string]string{
				"PROJECT_ENV": "production",
				"CDERUN_IMAGE": "node:{{file:.go-version}}-{{env:PROJECT_ENV}}",
			},
		}

		cli := CLIOptions{
			CderunTTY:    true,
			CderunTTYSet: true, // P1
			TTY:          false,
			TTYSet:       true, // P2
		}

		tools := ToolsConfig{
			"node": ToolConfig{
				Image: "node:latest", // P4 (should be overridden by P3 Env Var with expression)
				TTY:   ptr(false),
			},
		}

		global := &CDERunConfig{
			Defaults: ConfigDefaults{
				TTY: ptr(false), // P5
			},
		}

		// When: Resolving configuration
		res, err := ResolveWithFS("node", &cli, tools, global, mfs)

		// Then: Priority and expressions should be resolved correctly
		require.NoError(t, err)

		// P1 (CderunTTY) wins over P2, P3, P4, P5
		assert.True(t, res.TTY)

		// P3 (Env Var CDERUN_IMAGE) wins over P4 (Tool Image)
		// and expression {{file:.go-version}} and {{env:PROJECT_ENV}} are resolved
		assert.Equal(t, "node:1.25-production", res.Image)
	})
}

func TestScenario_ConfigResolution_NestedOverrides(t *testing.T) {
	t.Parallel()

	t.Run("Nested tool config overrides global defaults", func(t *testing.T) {
		// Given: Global config with some defaults and Tool config with overrides
		mfs := &MockFileSystem{} // Empty isolated environment
		global := &CDERunConfig{
			Defaults: ConfigDefaults{
				Network: "bridge",
				Remove:  ptr(true),
				Env:     []string{"GLOBAL=1"},
			},
		}
		tools := ToolsConfig{
			"app": ToolConfig{
				Image:   "my-app",
				Network: "host",
				Env:     []string{"TOOL=1"},
			},
		}

		// When: Resolving configuration for "app" with isolated FS
		res, err := ResolveWithFS("app", &CLIOptions{}, tools, global, mfs)

		// Then: Tool settings should override global ones, and slices should not merge across layers
		require.NoError(t, err)
		assert.Equal(t, "host", res.Network)
		assert.True(t, res.Remove)
		assert.Equal(t, []string{"TOOL=1"}, res.Env)
	})
}

func TestScenario_Expression_EnvResolution(t *testing.T) {
	t.Parallel()
	// Given: Expression resolver and environment variable
	mfs := &MockFileSystem{
		Env: map[string]string{
			"TEST_VERSION": "1.2.3",
		},
	}
	resolver, err := NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

	// When: Resolving env expression
	result := resolver.Resolve("image:{{env:TEST_VERSION}}")

	// Then: It should resolve to the environment variable value
	assert.Equal(t, "image:1.2.3", result)
	require.NoError(t, resolver.Error())
}

func TestUnit_Config_SetBaseDir_Exhaustive(t *testing.T) {
	t.Parallel()
	cfg := &CDERunConfig{
		SocketPath: ConfigPath{Raw: "./sock"},
	}
	err := cfg.SetBaseDir("/base")
	require.NoError(t, err)
	assert.Equal(t, "/base", cfg.SocketPath.BaseDir)
}

func TestUnit_Config_LoadFromPath_Direct(t *testing.T) {
	t.Parallel()
	mfs := &MockFileSystem{
		WD: "/app",
		Files: map[string][]byte{
			"/app/custom.yaml": []byte("runtime: podman"),
		},
	}
	loader := NewConfigLoaderWithFS(mfs)
	cfg, paths, err := loader.LoadCDERunConfigFromPath("custom.yaml")
	require.NoError(t, err)
	assert.Equal(t, "podman", cfg.Runtime)
	assert.Equal(t, []string{"/app/custom.yaml"}, paths)
}

func TestUnit_Config_Expression_Resolve_Exhaustive(t *testing.T) {
	t.Parallel()

	t.Run("Resolve slice", func(t *testing.T) {
		mfs := &MockFileSystem{WD: "/app", HomeDir: "/home/user"}
		resolver, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		input := []any{"{{PWD}}", "fixed"}
		result := resolver.Resolve(input)
		v, ok := result.([]any)
		require.True(t, ok, "Resolve should return []any for slice input, got %T", result)
		assert.Equal(t, "/app", v[0])
		assert.Equal(t, "fixed", v[1])
	})

	t.Run("Resolve map", func(t *testing.T) {
		mfs := &MockFileSystem{WD: "/app", HomeDir: "/home/user"}
		resolver, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		input := map[string]any{"key": "{{HOME}}"}
		result := resolver.Resolve(input)
		m, ok := result.(map[string]any)
		require.True(t, ok, "Resolve should return map[string]any for map input, got %T", result)
		assert.Equal(t, "/home/user", m["key"])
	})

	t.Run("Resolve other", func(t *testing.T) {
		mfs := &MockFileSystem{WD: "/app", HomeDir: "/home/user"}
		resolver, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		assert.Equal(t, 123, resolver.Resolve(123))
	})
}

func TestUnit_Config_ConfigLoader_Exhaustive(t *testing.T) {
	t.Parallel()

	t.Run("LoadToolsConfig success", func(t *testing.T) {
		mfs := &MockFileSystem{WD: "/app", Files: map[string][]byte{"/app/.tools.yaml": []byte("node:\n  image: node:20")}}
		loader := NewConfigLoaderWithFS(mfs)
		cfg, paths, err := loader.LoadToolsConfig()
		require.NoError(t, err)
		assert.Equal(t, "node:20", cfg["node"].Image)
		require.NotEmpty(t, paths)
		assert.Contains(t, paths[0], ".tools.yaml")
	})

	t.Run("LoadCDERunConfig from path with tilde", func(t *testing.T) {
		mfs := &MockFileSystem{WD: "/app", HomeDir: "/home/user", Files: map[string][]byte{"/home/user/custom.yaml": []byte("runtime: docker")}}
		loader := NewConfigLoaderWithFS(mfs)
		cfg, _, err := loader.LoadCDERunConfigFromPath("~/custom.yaml")
		require.NoError(t, err)
		assert.Equal(t, "docker", cfg.Runtime)
	})
}

func TestUnit_Config_MockFS_FileDetails(t *testing.T) {
	t.Parallel()
	mfs := &MockFileSystem{
		Files: map[string][]byte{"/dir/f": []byte("data")},
	}
	info, err := mfs.Stat("/dir/f")
	require.NoError(t, err)
	// After fix in Stat, Name() should return only the base name "f"
	assert.Equal(t, "f", info.Name())
	assert.Equal(t, int64(0), info.Size())
	assert.Equal(t, os.FileMode(0), info.Mode())
	assert.False(t, info.IsDir())
	assert.Nil(t, info.Sys())
	assert.True(t, info.ModTime().IsZero())
}

func TestUnit_Config_MockFS_LookupEnv(t *testing.T) {
	t.Parallel()
	mfs := &MockFileSystem{
		Env: map[string]string{"K": "V"},
	}
	val, ok := mfs.LookupEnv("K")
	assert.True(t, ok)
	assert.Equal(t, "V", val)

	_, ok = mfs.LookupEnv("U")
	assert.False(t, ok)
}

func TestUnit_Config_Resolver_Exhaustive_Coverage(t *testing.T) {
	t.Parallel()

	t.Run("resolveStringSliceCommaOpt", func(t *testing.T) {
		mfs := &MockFileSystem{Env: map[string]string{"CDERUN_MOUNT_TOOLS": "t1,t2"}}
		res, err := ResolveWithFS("node", &CLIOptions{Image: "alpine", ImageSet: true}, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, []string{"t1", "t2"}, res.MountTools)
	})

	t.Run("resolveFloat64Opt", func(t *testing.T) {
		mfs := &MockFileSystem{Env: map[string]string{"CDERUN_CPUS": "0.5"}}
		res, err := ResolveWithFS("node", &CLIOptions{Image: "alpine", ImageSet: true}, nil, nil, mfs)
		require.NoError(t, err)
		assert.InDelta(t, 0.5, res.CPUs, 0.0001)
	})

	t.Run("resolveEnvValues with strict error", func(t *testing.T) {
		mfs := &MockFileSystem{}
		cli := CLIOptions{Image: "alpine", ImageSet: true, Env: []string{"MISSING"}, StrictEnv: true, StrictEnvSet: true}
		_, err := ResolveWithFS("node", &cli, nil, nil, mfs)
		require.Error(t, err)
	})

	t.Run("resolveConfigPath with fallback and expression", func(t *testing.T) {
		mfs := &MockFileSystem{WD: "/work"}
		cli := CLIOptions{Image: "alpine", ImageSet: true, SocketPath: "{{PWD}}/docker.sock", SocketPathSet: true}
		res, err := ResolveWithFS("node", &cli, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, "/work/docker.sock", res.SocketPath)
	})
}

func TestIntegration_Config_PackageLevel_Exhaustive(t *testing.T) {
	t.Run("FindConfigs package level", func(t *testing.T) {
		paths := FindConfigs(".nonexistent")
		assert.Empty(t, paths)
	})

	t.Run("SetRunConfigDirForTest", func(t *testing.T) {
		restore := SetRunConfigDirForTest("/tmp/run")
		defer restore()
		assert.Equal(t, "/tmp/run", defaultLoader.runConfigDir)
	})

	t.Run("SetSystemConfigDirForTest", func(t *testing.T) {
		restore := SetSystemConfigDirForTest("/tmp/sys")
		defer restore()
		assert.Equal(t, "/tmp/sys", defaultLoader.systemConfigDir)
	})
}

func TestUnit_Config_Path_Exhaustive_Coverage(t *testing.T) {
	t.Parallel()

	t.Run("ResolveVolume with host remainder", func(t *testing.T) {
		mfs := &MockFileSystem{WD: "/app"}
		resolver, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)
		cp := ConfigPath{Raw: "/data:ro", BaseDir: "/app"}
		val, err := cp.ResolveVolume(resolver)
		require.NoError(t, err)
		assert.Equal(t, "/data:ro", val)
	})

	t.Run("ResolveDevice with host remainder", func(t *testing.T) {
		mfs := &MockFileSystem{WD: "/app"}
		resolver, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)
		cp := ConfigPath{Raw: "/dev/video0:/dev/video0:rw", BaseDir: "/app"}
		val, err := cp.ResolveDevice(resolver)
		require.NoError(t, err)
		assert.Equal(t, "/dev/video0:/dev/video0:rw", val)
	})
}

func TestIntegration_Config_RealFS_Exhaustive(t *testing.T) {
	fs := RealFileSystem{}

	t.Run("LookupEnv", func(t *testing.T) {
		expectedVal, expectedFound := os.LookupEnv("PATH")
		v, ok := fs.LookupEnv("PATH")
		assert.Equal(t, expectedFound, ok)
		assert.Equal(t, expectedVal, v)

		_, ok = fs.LookupEnv("NONEXISTENT_ENV_KEY_FOR_TEST")
		assert.False(t, ok)
	})

	t.Run("Other methods", func(t *testing.T) {
		wd, err := fs.Getwd()
		require.NoError(t, err)
		expectedWd, err := os.Getwd()
		require.NoError(t, err)
		assert.Equal(t, expectedWd, wd)

		home, err := fs.UserHomeDir()
		require.NoError(t, err)
		expectedHome, err := os.UserHomeDir()
		require.NoError(t, err)
		assert.Equal(t, expectedHome, home)

		exe, err := fs.Executable()
		require.NoError(t, err)
		expectedExe, err := os.Executable()
		require.NoError(t, err)
		assert.Equal(t, expectedExe, exe)

		expectedPath := os.Getenv("PATH")
		assert.Equal(t, expectedPath, fs.Getenv("PATH"))

		temp := fs.TempDir()
		assert.Equal(t, os.TempDir(), temp)

		tmpDir := t.TempDir()

		err = fs.MkdirAll(tmpDir+"/a", 0o755)
		require.NoError(t, err)

		err = fs.WriteFile(tmpDir+"/f", []byte("data"), 0o644)
		require.NoError(t, err)

		data, err := fs.ReadFile(tmpDir+"/f")
		require.NoError(t, err)
		assert.Equal(t, "data", string(data))

		info, err := fs.Stat(tmpDir+"/f")
		require.NoError(t, err)
		assert.False(t, info.IsDir())

		err = fs.RemoveAll(tmpDir+"/a")
		require.NoError(t, err)

		abs, err := fs.Abs(".")
		require.NoError(t, err)
		assert.Equal(t, expectedWd, abs)
	})
}

func TestIntegration_Config_PackageLevel_More(t *testing.T) {
	// Use isolated temp dir to avoid flakiness and probe real FS only deterministically
	tempDir := t.TempDir()

	restoreRun := SetRunConfigDirForTest(tempDir)
	defer restoreRun()
	restoreSys := SetSystemConfigDirForTest(tempDir)
	defer restoreSys()

	// Exercise package level wrappers and verify outcomes
	paths := FindConfigs(".cderun.yaml")
	assert.Empty(t, paths)

	cfg, paths, err := LoadCDERunConfig()
	require.NoError(t, err)
	assert.Nil(t, cfg)
	assert.Empty(t, paths)

	tCfg, paths, err := LoadToolsConfig()
	require.NoError(t, err)
	assert.Nil(t, tCfg)
	assert.Empty(t, paths)
}

func TestUnit_Config_Path_DeepCopy_Exhaustive(t *testing.T) {
	t.Parallel()
	cp := ConfigPath{Raw: "r", BaseDir: "b"}
	cp2 := cp.DeepCopy()
	assert.Equal(t, cp, cp2)

	mc := MountConfig{Type: "bind", Source: cp, Target: cp}
	mc2 := mc.DeepCopy()
	assert.Equal(t, mc, mc2)

	dc := DeviceConfig{Source: cp, Destination: cp, Permissions: "rw"}
	dc2 := dc.DeepCopy()
	assert.Equal(t, dc, dc2)
}

func TestUnit_Config_Resolver_Env_Deduplication(t *testing.T) {
	t.Parallel()
	base := []string{"A=1", "B=2"}
	p2 := []string{"B=3", "C=4"}
	p1 := []string{"C=5", "D=6"}
	merged := mergeEnv(base, p2, p1)

	assert.Len(t, merged, 4)
	assert.ElementsMatch(t, []string{"A=1", "B=3", "C=5", "D=6"}, merged)
}

func TestUnit_Config_Resolver_Devices_More(t *testing.T) {
	t.Parallel()

	t.Run("Resolve empty devices", func(t *testing.T) {
		mfs := &MockFileSystem{WD: "/app"}
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)
		res, err := resolveDevices(nil, nil, nil, nil, r, mfs)
		require.NoError(t, err)
		assert.Empty(t, res)
	})
}

func TestUnit_Config_Resolver_Errors_Exhaustive(t *testing.T) {
	t.Parallel()

	t.Run("resolveDevices invalid format", func(t *testing.T) {
		mfs := &MockFileSystem{WD: "/app"}
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)
		_, err = resolveDevices([]string{":"}, nil, nil, nil, r, mfs)
		require.Error(t, err)
	})

	t.Run("resolveMounts invalid format", func(t *testing.T) {
		mfs := &MockFileSystem{WD: "/app"}
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)
		_, err = resolveMounts([]string{"invalid"}, nil, nil, nil, r, mfs)
		require.Error(t, err)
	})

	t.Run("resolveEnvValues expression error", func(t *testing.T) {
		mfs := &customMockFS{
			MockFileSystem: MockFileSystem{WD: "/app"},
			readFileErr:    assert.AnError,
		}
		// Create a resolver that will error on some expression
		rErr, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)
		// {{file:expr}} will trigger an error when it tries to read the file
		_, err = resolveEnvValues([]string{"VAR={{file:expr}}"}, false, rErr, mfs)
		require.Error(t, err)
	})

	t.Run("resolveFloat64Opt invalid", func(t *testing.T) {
		mfs := &MockFileSystem{Env: map[string]string{"CDERUN_CPUS": "invalid"}}
		val := resolveFloat64Opt(OptionDef[*float64]{EnvKey: "CDERUN_CPUS"}, false, 0.0, false, 0.0, nil, nil, mfs)
		assert.InDelta(t, 0.0, val, 1e-9)
	})

	t.Run("resolveConfigPath volume resolution", func(t *testing.T) {
		mfs := &MockFileSystem{Env: map[string]string{"VOL": "/host:/cont:ro"}, WD: "/app"}
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)
		val, err := resolveConfigPath(false, "", false, "", "VOL", nil, nil, nil, nil, "", r, "volume", mfs)
		require.NoError(t, err)
		assert.Equal(t, "/host:/cont:ro", val)
	})

	t.Run("resolveConfigPath device resolution", func(t *testing.T) {
		mfs := &MockFileSystem{Env: map[string]string{"DEV": "/dev/a:/dev/b:rw"}, WD: "/app"}
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)
		val, err := resolveConfigPath(false, "", false, "", "DEV", nil, nil, nil, nil, "", r, "device", mfs)
		require.NoError(t, err)
		assert.Equal(t, "/dev/a:/dev/b:rw", val)
	})
}

func TestUnit_Config_Resolver_More_Coverage(t *testing.T) {
	t.Parallel()

	t.Run("resolveMounts with expressions", func(t *testing.T) {
		mfs := &MockFileSystem{WD: "/app", Env: map[string]string{"SRC": "/host/path"}}
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)
		p1 := []string{"source={{env:SRC}},target=/cont/path"}
		res, err := resolveMounts(p1, nil, nil, nil, r, mfs)
		require.NoError(t, err)
		require.Len(t, res, 1)
		assert.Equal(t, "/host/path", res[0].Source)
	})

	t.Run("resolveConfigPath volume with fallback", func(t *testing.T) {
		mfs := &MockFileSystem{WD: "/app"}
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)
		val, err := resolveConfigPath(false, "", false, "", "NONEXISTENT", nil, nil, nil, nil, "/fallback:ro", r, "volume", mfs)
		require.NoError(t, err)
		assert.Equal(t, "/fallback:ro", val)
	})

	t.Run("resolveConfigPath device with fallback", func(t *testing.T) {
		mfs := &MockFileSystem{WD: "/app"}
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)
		val, err := resolveConfigPath(false, "", false, "", "NONEXISTENT", nil, nil, nil, nil, "/dev/null:/dev/null:rw", r, "device", mfs)
		require.NoError(t, err)
		assert.Equal(t, "/dev/null:/dev/null:rw", val)
	})
}
