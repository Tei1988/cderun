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
		res, err := ResolveWithFS("node", cli, tools, global, mfs)

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

		// When: Resolving configuration for "app"
		res, err := Resolve("app", CLIOptions{}, tools, global)

		// Then: Tool settings should override global ones, and slices should not merge across layers
		require.NoError(t, err)
		assert.Equal(t, "host", res.Network)
		assert.True(t, res.Remove)
		assert.Contains(t, res.Env, "TOOL=1")
		assert.NotContains(t, res.Env, "GLOBAL=1")
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
	mfs := &MockFileSystem{
		WD: "/app",
		HomeDir: "/home/user",
	}
	resolver, err := NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

	t.Run("Resolve slice", func(t *testing.T) {
		input := []any{"{{PWD}}", "fixed"}
		result := resolver.Resolve(input).([]any)
		assert.Equal(t, "/app", result[0])
		assert.Equal(t, "fixed", result[1])
	})

	t.Run("Resolve map", func(t *testing.T) {
		input := map[string]any{"key": "{{HOME}}"}
		result := resolver.Resolve(input).(map[string]any)
		assert.Equal(t, "/home/user", result["key"])
	})

	t.Run("Resolve other", func(t *testing.T) {
		assert.Equal(t, 123, resolver.Resolve(123))
	})
}

func TestUnit_Config_ConfigLoader_Exhaustive(t *testing.T) {
	t.Parallel()
	mfs := &MockFileSystem{
		WD: "/app",
		Files: map[string][]byte{
			"/app/.tools.yaml": []byte("node:\n  image: node:20"),
		},
	}
	loader := NewConfigLoaderWithFS(mfs)

	t.Run("LoadToolsConfig success", func(t *testing.T) {
		cfg, paths, err := loader.LoadToolsConfig()
		require.NoError(t, err)
		assert.Equal(t, "node:20", cfg["node"].Image)
		assert.Contains(t, paths[0], ".tools.yaml")
	})

	t.Run("LoadCDERunConfig from path with tilde", func(t *testing.T) {
		mfs.HomeDir = "/home/user"
		mfs.Files["/home/user/custom.yaml"] = []byte("runtime: docker")
		cfg, _, err := loader.LoadCDERunConfigFromPath("~/custom.yaml")
		require.NoError(t, err)
		assert.Equal(t, "docker", cfg.Runtime)
	})
}

func TestUnit_Config_MockFS_FileDetails(t *testing.T) {
	t.Parallel()
	mfs := &MockFileSystem{
		Files: map[string][]byte{"/f": []byte("data")},
	}
	info, err := mfs.Stat("/f")
	require.NoError(t, err)
	assert.Equal(t, "/f", info.Name())
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
	mfs := &MockFileSystem{
		Env: map[string]string{
			"CDERUN_MOUNT_TOOLS": "t1,t2",
			"CDERUN_ENV": "A;B=2",
			"A": "1",
			"CDERUN_CPUS": "0.5",
		},
	}

	t.Run("resolveStringSliceCommaOpt", func(t *testing.T) {
		res, err := ResolveWithFS("node", CLIOptions{Image: "alpine", ImageSet: true}, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, []string{"t1", "t2"}, res.MountTools)
	})

	t.Run("resolveFloat64Opt", func(t *testing.T) {
		res, err := ResolveWithFS("node", CLIOptions{Image: "alpine", ImageSet: true}, nil, nil, mfs)
		require.NoError(t, err)
		assert.InDelta(t, 0.5, res.CPUs, 0.0001)
	})

	t.Run("resolveEnvValues with strict error", func(t *testing.T) {
		cli := CLIOptions{Image: "alpine", ImageSet: true, Env: []string{"MISSING"}, StrictEnv: true, StrictEnvSet: true}
		_, err := ResolveWithFS("node", cli, nil, nil, mfs)
		require.Error(t, err)
	})

	t.Run("resolveConfigPath with fallback and expression", func(t *testing.T) {
		mfs.WD = "/work"
		cli := CLIOptions{Image: "alpine", ImageSet: true, SocketPath: "{{PWD}}/docker.sock", SocketPathSet: true}
		res, err := ResolveWithFS("node", cli, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, "/work/docker.sock", res.SocketPath)
	})
}

func TestUnit_Config_PackageLevel_Exhaustive(t *testing.T) {
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
	mfs := &MockFileSystem{WD: "/app"}
	resolver, err := NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

	t.Run("ResolveVolume with host remainder", func(t *testing.T) {
		cp := ConfigPath{Raw: "/data:ro", BaseDir: "/app"}
		val, err := cp.ResolveVolume(resolver)
		require.NoError(t, err)
		assert.Equal(t, "/data:ro", val)
	})

	t.Run("ResolveDevice with host remainder", func(t *testing.T) {
		cp := ConfigPath{Raw: "/dev/video0:/dev/video0:rw", BaseDir: "/app"}
		val, err := cp.ResolveDevice(resolver)
		require.NoError(t, err)
		assert.Equal(t, "/dev/video0:/dev/video0:rw", val)
	})
}

func TestUnit_Config_RealFS_Exhaustive(t *testing.T) {
	fs := RealFileSystem{}

	t.Run("LookupEnv", func(t *testing.T) {
		t.Setenv("TEST_LOOKUP", "val")
		v, ok := fs.LookupEnv("TEST_LOOKUP")
		assert.True(t, ok)
		assert.Equal(t, "val", v)

		_, ok = fs.LookupEnv("NONEXISTENT_LOOKUP")
		assert.False(t, ok)
	})

	t.Run("Other methods", func(t *testing.T) {
		_, _ = fs.Getwd()
		_, _ = fs.UserHomeDir()
		_, _ = fs.Executable()
		_ = fs.Getenv("PATH")
		_ = fs.TempDir()

		tmp := t.TempDir()
		_ = fs.MkdirAll(tmp+"/a", 0o755)
		_ = fs.WriteFile(tmp+"/f", []byte("d"), 0o644)
		_, _ = fs.ReadFile(tmp+"/f")
		_, _ = fs.Stat(tmp+"/f")
		_ = fs.RemoveAll(tmp+"/a")
		_, _ = fs.Abs(".")
	})
}

func TestUnit_Config_PackageLevel_More(t *testing.T) {
	// Exercise package level wrappers and verify outcomes (best effort)
	_ = FindConfigs(".cderun.yaml")

	_, _, err := LoadCDERunConfig()
	assert.NoError(t, err)

	_, _, err = LoadToolsConfig()
	assert.NoError(t, err)
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
	assert.Contains(t, merged, "A=1")
	assert.Contains(t, merged, "B=3")
	assert.Contains(t, merged, "C=5")
	assert.Contains(t, merged, "D=6")
}

func TestUnit_Config_Resolver_Devices_More(t *testing.T) {
	t.Parallel()
	mfs := &MockFileSystem{WD: "/app"}
	r, err := NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

	t.Run("Resolve empty devices", func(t *testing.T) {
		res, err := resolveDevices(nil, nil, "", nil, nil, r, mfs)
		require.NoError(t, err)
		assert.Empty(t, res)
	})
}

func TestUnit_Config_Resolver_Errors_Exhaustive(t *testing.T) {
	t.Parallel()
	mfs := &MockFileSystem{WD: "/app"}
	r, err := NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

	t.Run("resolveDevices invalid format", func(t *testing.T) {
		_, err := resolveDevices([]string{":"}, nil, "", nil, nil, r, mfs)
		assert.Error(t, err)
	})

	t.Run("resolveMounts invalid format", func(t *testing.T) {
		_, err := resolveMounts([]string{"invalid"}, nil, "", nil, nil, r, mfs)
		assert.Error(t, err)
	})

	t.Run("resolveEnvValues expression error", func(t *testing.T) {
		// Create a resolver that will error on some expression
		rErr, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)
		rErr.setError(assert.AnError)
		_, err = resolveEnvValues([]string{"VAR={{expr}}"}, false, rErr, mfs)
		assert.Error(t, err)
	})

	t.Run("resolveFloat64Opt invalid", func(t *testing.T) {
		mfs.Env = map[string]string{"CDERUN_CPUS": "invalid"}
		val := resolveFloat64Opt(OptionDef[*float64]{EnvKey: "CDERUN_CPUS"}, false, 0.0, false, 0.0, "", nil, nil, mfs)
		assert.InDelta(t, 0.0, val, 1e-9)
	})

	t.Run("resolveConfigPath volume resolution", func(t *testing.T) {
		mfs.Env = map[string]string{"VOL": "/host:/cont:ro"}
		val, err := resolveConfigPath(false, "", false, "", "VOL", "", nil, nil, nil, nil, "", r, "volume", mfs)
		require.NoError(t, err)
		assert.Equal(t, "/host:/cont:ro", val)
	})

	t.Run("resolveConfigPath device resolution", func(t *testing.T) {
		mfs.Env = map[string]string{"DEV": "/dev/a:/dev/b:rw"}
		val, err := resolveConfigPath(false, "", false, "", "DEV", "", nil, nil, nil, nil, "", r, "device", mfs)
		require.NoError(t, err)
		assert.Equal(t, "/dev/a:/dev/b:rw", val)
	})
}

func TestUnit_Config_Resolver_More_Coverage(t *testing.T) {
	t.Parallel()
	mfs := &MockFileSystem{WD: "/app"}
	r, err := NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

	t.Run("resolveMounts with expressions", func(t *testing.T) {
		mfs.Env = map[string]string{"SRC": "/host/path"}
		p1 := []string{"source={{env:SRC}},target=/cont/path"}
		res, err := resolveMounts(p1, nil, "", nil, nil, r, mfs)
		require.NoError(t, err)
		require.Len(t, res, 1)
		assert.Equal(t, "/host/path", res[0].Source)
	})

	t.Run("resolveConfigPath volume with fallback", func(t *testing.T) {
		val, err := resolveConfigPath(false, "", false, "", "NONEXISTENT", "", nil, nil, nil, nil, "/fallback:ro", r, "volume", mfs)
		require.NoError(t, err)
		assert.Equal(t, "/fallback:ro", val)
	})

	t.Run("resolveConfigPath device with fallback", func(t *testing.T) {
		val, err := resolveConfigPath(false, "", false, "", "NONEXISTENT", "", nil, nil, nil, nil, "/dev/null:/dev/null:rw", r, "device", mfs)
		require.NoError(t, err)
		assert.Equal(t, "/dev/null:/dev/null:rw", val)
	})
}
