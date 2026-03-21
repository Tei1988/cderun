package config

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failHomeFS struct {
	MockFileSystem
}

func (f *failHomeFS) UserHomeDir() (string, error) {
	return "", errors.New("home error")
}

func TestUnit_Config_ResolveMounts_Robustness(t *testing.T) {
	t.Parallel()

	t.Run("Invalid format in CDERUN_MOUNT", func(t *testing.T) {
		mfs := &MockFileSystem{
			Env: map[string]string{"CDERUN_MOUNT": "invalid_format"},
			WD:  "/work",
		}
		cli := CLIOptions{
			Image:    "node",
			ImageSet: true,
		}
		_, err := ResolveWithFS("node", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid mount config in CDERUN_MOUNT")
	})

	t.Run("Invalid format in P1 Internal Override", func(t *testing.T) {
		cli := CLIOptions{
			Image:        "node",
			ImageSet:     true,
			CderunMounts: []string{"invalid_format"},
		}
		_, err := Resolve("node", cli, nil, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid mount config (override)")
	})

	t.Run("Invalid format in P2 CLI Flags", func(t *testing.T) {
		cli := CLIOptions{
			Image:    "node",
			ImageSet: true,
			Mounts:   []string{"invalid_format"},
		}
		_, err := Resolve("node", cli, nil, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid mount config")
	})

	t.Run("BaseDir application for relative paths in P1", func(t *testing.T) {
		mfs := &MockFileSystem{
			WD: "/current/dir",
		}
		cli := CLIOptions{
			Image:        "node",
			ImageSet:     true,
			CderunMounts: []string{"type=bind,source=./src,target=/app"},
		}
		res, err := ResolveWithFS("node", cli, nil, nil, mfs)
		require.NoError(t, err)
		require.Len(t, res.Mounts, 1)
		assert.Equal(t, "/current/dir/src", res.Mounts[0].Source)
	})

	t.Run("BaseDir application for relative paths in CDERUN_MOUNT", func(t *testing.T) {
		mfs := &MockFileSystem{
			Env: map[string]string{"CDERUN_MOUNT": "type=bind,source=./src,target=/app"},
			WD:  "/current/dir",
		}
		cli := CLIOptions{
			Image:    "node",
			ImageSet: true,
		}
		res, err := ResolveWithFS("node", cli, nil, nil, mfs)
		require.NoError(t, err)
		require.Len(t, res.Mounts, 1)
		assert.Equal(t, "/current/dir/src", res.Mounts[0].Source)
	})
}

func TestUnit_Config_ResolveDevices_Robustness(t *testing.T) {
	t.Parallel()

	t.Run("Invalid format in CDERUN_DEVICE", func(t *testing.T) {
		mfs := &MockFileSystem{
			Env: map[string]string{"CDERUN_DEVICE": ":"},
			WD:  "/work",
		}
		cli := CLIOptions{
			Image:    "node",
			ImageSet: true,
		}
		_, err := ResolveWithFS("node", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid device config in CDERUN_DEVICE")
	})

	t.Run("Invalid format in P1 Internal Override", func(t *testing.T) {
		cli := CLIOptions{
			Image:         "node",
			ImageSet:      true,
			CderunDevices: []string{":"},
		}
		_, err := Resolve("node", cli, nil, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid device config (override)")
	})

	t.Run("Invalid format in P2 CLI Flags", func(t *testing.T) {
		cli := CLIOptions{
			Image:    "node",
			ImageSet: true,
			Devices:  []string{":"},
		}
		_, err := Resolve("node", cli, nil, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid device config")
	})
}

func TestUnit_Config_ResolveEnv_Robustness(t *testing.T) {
	t.Parallel()

	t.Run("Passthrough environment variables across layers", func(t *testing.T) {
		mfs := &MockFileSystem{
			Env: map[string]string{
				"HOST_VAR": "host_value",
			},
		}
		cli := CLIOptions{
			Image:    "node",
			ImageSet: true,
			Env:      []string{"HOST_VAR"},
		}
		res, err := ResolveWithFS("node", cli, nil, nil, mfs)
		require.NoError(t, err)
		assert.Contains(t, res.Env, "HOST_VAR=host_value")
	})

	t.Run("Env merging with different priorities", func(t *testing.T) {
		mfs := &MockFileSystem{
			Env: map[string]string{
				"VAR1": "env_val1",
			},
		}
		cli := CLIOptions{
			Image:    "node",
			ImageSet: true,
			Env:      []string{"VAR1=cli_val1", "VAR2=cli_val2"},
		}
		tools := ToolsConfig{
			"node": ToolConfig{
				Env: []string{"VAR2=tool_val2", "VAR3=tool_val3"},
			},
		}
		res, err := ResolveWithFS("node", cli, tools, nil, mfs)
		require.NoError(t, err)
		assert.Contains(t, res.Env, "VAR1=cli_val1")
		assert.Contains(t, res.Env, "VAR2=cli_val2")
		for _, e := range res.Env {
			assert.NotContains(t, e, "VAR3=")
		}
	})
}

func TestUnit_Config_ExpressionResolver_Robustness(t *testing.T) {
	t.Parallel()

	t.Run("File not found error", func(t *testing.T) {
		mfs := &MockFileSystem{
			WD: "/work",
		}
		resolver, _ := NewExpressionResolverWithFS(nil, mfs)
		_, err := resolver.ResolveString("{{file:missing.txt}}")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "file not found")
	})

	t.Run("FindDir not found error", func(t *testing.T) {
		mfs := &MockFileSystem{
			WD: "/work",
		}
		resolver, _ := NewExpressionResolverWithFS(nil, mfs)
		_, err := resolver.ResolveString("{{find_dir:.git}}")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "item not found for find_dir")
	})
}

func TestUnit_Config_PathResolution_Robustness(t *testing.T) {
	t.Parallel()

	t.Run("ParseMountFlag invalid format", func(t *testing.T) {
		_, err := ParseMountFlag("source=/tmp") // missing target
		require.Error(t, err)
		assert.Contains(t, err.Error(), "mount target is required")
	})

	t.Run("ParseMountFlag invalid readonly", func(t *testing.T) {
		_, err := ParseMountFlag("source=/tmp,target=/app,readonly=maybe")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid readonly value")
	})

	t.Run("ResolvePath with tilde and relative", func(t *testing.T) {
		mfs := &MockFileSystem{
			WD:      "/work/dir",
			HomeDir: "/home/user",
		}
		resolver, _ := NewExpressionResolverWithFS(nil, mfs)
		path, _ := ResolvePath("~/src", "/work/dir", resolver)
		assert.Equal(t, "/home/user/src", path)

		pathRel, _ := ResolvePath("./src", "/work/dir", resolver)
		assert.Equal(t, "/work/dir/src", pathRel)
	})
}

func TestUnit_Config_ResolveEnv_Merging_Robustness(t *testing.T) {
	t.Parallel()

	t.Run("P1 Override overrides all", func(t *testing.T) {
		mfs := &MockFileSystem{
			Env: map[string]string{"CDERUN_ENV": "VAR=env"},
		}
		cli := CLIOptions{
			Image:     "node",
			ImageSet:  true,
			Env:       []string{"VAR=cli"},
			CderunEnv: []string{"VAR=p1"},
		}
		tools := ToolsConfig{
			"node": ToolConfig{
				Env: []string{"VAR=tool"},
			},
		}
		res, err := ResolveWithFS("node", cli, tools, nil, mfs)
		require.NoError(t, err)
		assert.Contains(t, res.Env, "VAR=p1")
		assert.NotContains(t, res.Env, "VAR=cli")
		assert.NotContains(t, res.Env, "VAR=env")
		assert.NotContains(t, res.Env, "VAR=tool")
	})

	t.Run("Empty P4 Tool falls back to P5 Global", func(t *testing.T) {
		tools := ToolsConfig{
			"node": ToolConfig{
				Image: "node",
				// Env is empty (nil)
			},
		}
		global := &CDERunConfig{
			Defaults: ConfigDefaults{
				Env: []string{"VAR=global"},
			},
		}
		res, err := Resolve("node", CLIOptions{Image: "node", ImageSet: true}, tools, global)
		require.NoError(t, err)
		assert.Contains(t, res.Env, "VAR=global")
	})

	t.Run("Explicit empty list in P4 Tool falls back to P5 Global", func(t *testing.T) {
		tools := ToolsConfig{
			"node": ToolConfig{
				Image: "node",
				Env:   []string{}, // Explicit empty
			},
		}
		global := &CDERunConfig{
			Defaults: ConfigDefaults{
				Env: []string{"VAR=global"},
			},
		}
		res, err := Resolve("node", CLIOptions{Image: "node", ImageSet: true}, tools, global)
		require.NoError(t, err)
		assert.Contains(t, res.Env, "VAR=global")
	})
}

func TestUnit_Config_ExpressionResolver_Advanced_Robustness(t *testing.T) {
	t.Parallel()

	t.Run("Multiple expressions in a single string and error propagation", func(t *testing.T) {
		mfs := &MockFileSystem{
			WD: "/work",
			Env: map[string]string{
				"A": "alpha",
			},
		}
		resolver, _ := NewExpressionResolverWithFS(nil, mfs)

		// One succeeds, one fails.
		_, err := resolver.ResolveString("{{env:A}} and {{file:missing.txt}}")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "file not found")
	})

	t.Run("Nested or recursive expressions are NOT supported", func(t *testing.T) {
		mfs := &MockFileSystem{
			Env: map[string]string{
				"A": "{{env:B}}",
				"B": "beta",
			},
		}
		resolver, _ := NewExpressionResolverWithFS(nil, mfs)
		val, err := resolver.ResolveString("{{env:A}}")
		require.NoError(t, err)
		// Should return "{{env:B}}" literally, as it's not resolved recursively.
		assert.Equal(t, "{{env:B}}", val)
	})

	t.Run("Invalid directive format", func(t *testing.T) {
		mfs := &MockFileSystem{}
		resolver, _ := NewExpressionResolverWithFS(nil, mfs)
		val, err := resolver.ResolveString("{{invalid}}")
		require.NoError(t, err)
		// Not recognized as a directive (no ':'), so it stays as is if not magic word.
		assert.Equal(t, "{{invalid}}", val)
	})
}

func TestUnit_Config_ResolveDevices_Advanced_Robustness(t *testing.T) {
	t.Parallel()

	t.Run("BaseDir application for relative paths in CDERUN_DEVICE", func(t *testing.T) {
		mfs := &MockFileSystem{
			Env: map[string]string{"CDERUN_DEVICE": "./dev/video0:/dev/video0"},
			WD:  "/current/dir",
		}
		cli := CLIOptions{
			Image:    "node",
			ImageSet: true,
		}
		res, err := ResolveWithFS("node", cli, nil, nil, mfs)
		require.NoError(t, err)
		require.Len(t, res.Devices, 1)
		assert.Equal(t, "/current/dir/dev/video0", res.Devices[0].PathOnHost)
	})

	t.Run("Device merging from Tool config", func(t *testing.T) {
		tools := ToolsConfig{
			"node": ToolConfig{
				Image: "node",
				Devices: []DeviceConfig{
					{
						Source:      ConfigPath{Raw: "/dev/video1"},
						Destination: ConfigPath{Raw: "/dev/video1"},
					},
				},
			},
		}
		res, err := Resolve("node", CLIOptions{}, tools, nil)
		require.NoError(t, err)
		require.Len(t, res.Devices, 1)
		assert.Equal(t, "/dev/video1", res.Devices[0].PathOnHost)
	})

	t.Run("Devices fallback to Global config", func(t *testing.T) {
		global := &CDERunConfig{
			Defaults: ConfigDefaults{
				Devices: []DeviceConfig{
					{
						Source:      ConfigPath{Raw: "/dev/global"},
						Destination: ConfigPath{Raw: "/dev/global"},
					},
				},
			},
		}
		res, err := Resolve("node", CLIOptions{Image: "node", ImageSet: true}, nil, global)
		require.NoError(t, err)
		require.Len(t, res.Devices, 1)
		assert.Equal(t, "/dev/global", res.Devices[0].PathOnHost)
	})
}

func TestUnit_Config_ResolveMounts_Advanced_Robustness(t *testing.T) {
	t.Parallel()

	t.Run("Mounts from Tool config", func(t *testing.T) {
		tools := ToolsConfig{
			"node": ToolConfig{
				Image: "node",
				Mounts: []MountConfig{
					{
						Source: ConfigPath{Raw: "/src"},
						Target: ConfigPath{Raw: "/app"},
					},
				},
			},
		}
		res, err := Resolve("node", CLIOptions{}, tools, nil)
		require.NoError(t, err)
		require.Len(t, res.Mounts, 1)
		assert.Equal(t, "/src", res.Mounts[0].Source)
	})

	t.Run("Mounts fallback to Global config", func(t *testing.T) {
		global := &CDERunConfig{
			Defaults: ConfigDefaults{
				Mounts: []MountConfig{
					{
						Source: ConfigPath{Raw: "/global"},
						Target: ConfigPath{Raw: "/app"},
					},
				},
			},
		}
		res, err := Resolve("node", CLIOptions{Image: "node", ImageSet: true}, nil, global)
		require.NoError(t, err)
		require.Len(t, res.Mounts, 1)
		assert.Equal(t, "/global", res.Mounts[0].Source)
	})
}

func TestUnit_Config_ConfigPath_Resolve_Error_Robustness(t *testing.T) {
	t.Parallel()

	t.Run("ConfigPath.Resolve error with expression", func(t *testing.T) {
		cp := ConfigPath{Raw: "{{file:missing}}"}
		mfs := &MockFileSystem{}
		resolver, _ := NewExpressionResolverWithFS(nil, mfs)
		_, err := cp.Resolve(resolver)
		require.Error(t, err)
	})

	t.Run("ConfigPath.ResolveVolume error with expression", func(t *testing.T) {
		mfs := &failHomeFS{}
		resolver, _ := NewExpressionResolverWithFS(nil, mfs)
		cp := ConfigPath{Raw: "~/foo"}
		_, err := cp.ResolveVolume(resolver)
		require.Error(t, err)
	})

	t.Run("ConfigPath.ResolveDevice error with expression", func(t *testing.T) {
		mfs := &failHomeFS{}
		resolver, _ := NewExpressionResolverWithFS(nil, mfs)
		cp := ConfigPath{Raw: "~/foo"}
		_, err := cp.ResolveDevice(resolver)
		require.Error(t, err)
	})
}

func TestUnit_Config_DeviceConfig_Resolve_Error_Robustness(t *testing.T) {
	t.Parallel()

	t.Run("DeviceConfig.Resolve error in Source", func(t *testing.T) {
		mfs := &failHomeFS{}
		resolver, _ := NewExpressionResolverWithFS(nil, mfs)
		dc := DeviceConfig{
			Source:      ConfigPath{Raw: "~/source"},
			Destination: ConfigPath{Raw: "/dest"},
		}
		_, err := dc.Resolve(resolver)
		require.Error(t, err)
	})

	t.Run("DeviceConfig.Resolve error in Destination", func(t *testing.T) {
		mfs := &failHomeFS{}
		resolver, _ := NewExpressionResolverWithFS(nil, mfs)
		dc := DeviceConfig{
			Source:      ConfigPath{Raw: "/source"},
			Destination: ConfigPath{Raw: "~/dest"},
		}
		_, err := dc.Resolve(resolver)
		require.Error(t, err)
	})
}

func TestUnit_Config_ResolveMounts_EmptyEnv_Robustness(t *testing.T) {
	t.Parallel()

	t.Run("Empty segments in CDERUN_MOUNT", func(t *testing.T) {
		mfs := &MockFileSystem{
			Env: map[string]string{"CDERUN_MOUNT": "type=bind,source=/src,target=/app; ;"},
			WD:  "/work",
		}
		cli := CLIOptions{
			Image:    "node",
			ImageSet: true,
		}
		res, err := ResolveWithFS("node", cli, nil, nil, mfs)
		require.NoError(t, err)
		assert.Len(t, res.Mounts, 1)
	})
}

func TestUnit_Config_ResolveDevices_ErrorInResolution(t *testing.T) {
	t.Parallel()

	t.Run("Error during final resolution in resolveDevices", func(t *testing.T) {
		mfs := &failHomeFS{}

		cli := CLIOptions{
			Image:    "node",
			ImageSet: true,
			Devices:  []string{"~/dev/video0:/dev/video0"},
		}

		_, err := ResolveWithFS("node", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "home error")
	})
}

func TestUnit_Config_ResolveMounts_ErrorInResolution(t *testing.T) {
	t.Parallel()

	t.Run("Error during final resolution in resolveMounts", func(t *testing.T) {
		mfs := &failHomeFS{}
		cli := CLIOptions{
			Image:    "node",
			ImageSet: true,
			Mounts:   []string{"type=bind,source=~/src,target=/app"},
		}
		_, err := ResolveWithFS("node", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "home error")
	})
}

func TestUnit_Config_Helpers_Robustness(t *testing.T) {
	t.Parallel()

	t.Run("copyFloat64Ptr with non-nil", func(t *testing.T) {
		f := 1.23
		res := copyFloat64Ptr(&f)
		require.NotNil(t, res)
		assert.InDelta(t, f, *res, 1e-9)
	})
}

func TestUnit_Config_SetBaseDir_Robustness(t *testing.T) {
	t.Parallel()

	t.Run("CDERunConfig.SetBaseDir with HostContext and mounts", func(t *testing.T) {
		cfg := &CDERunConfig{
			HostContext: &HostContext{
				Mounts: []MountMapping{
					{Source: "./src", Target: "/app"},
				},
			},
		}
		err := cfg.SetBaseDir("/base")
		require.NoError(t, err)
		assert.Equal(t, "/base/src", cfg.HostContext.Mounts[0].Source)
	})

	t.Run("CDERunConfig.SetBaseDir with SocketPath", func(t *testing.T) {
		cfg := &CDERunConfig{
			SocketPath: ConfigPath{Raw: "./docker.sock"},
		}
		err := cfg.SetBaseDir("/base")
		require.NoError(t, err)
		assert.Equal(t, "/base", cfg.SocketPath.BaseDir)
	})
}

func TestUnit_Config_ResolveDevices_InvalidInEnv_Robustness(t *testing.T) {
	t.Parallel()

	t.Run("Invalid format in CDERUN_DEVICE env", func(t *testing.T) {
		mfs := &MockFileSystem{
			Env: map[string]string{"CDERUN_DEVICE": ":container"},
			WD:  "/work",
		}
		cli := CLIOptions{Image: "node", ImageSet: true}
		_, err := ResolveWithFS("node", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid device config in CDERUN_DEVICE")
	})
}

func TestUnit_Config_ResolveDevices_Error_Robustness(t *testing.T) {
	t.Parallel()

	t.Run("Invalid format in P1 Override", func(t *testing.T) {
		cli := CLIOptions{
			Image:         "node",
			ImageSet:      true,
			CderunDevices: []string{":container"},
		}
		_, err := Resolve("node", cli, nil, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid device config (override)")
	})
}

func TestUnit_Config_ResolveConfigPath_Robustness(t *testing.T) {
	t.Parallel()

	t.Run("Tool config getter returns empty", func(t *testing.T) {
		tools := ToolsConfig{
			"node": ToolConfig{}, // Empty config
		}
		getter := func(tc ToolConfig) ConfigPath { return tc.MountSocketPath }
		mfs := &MockFileSystem{WD: "/work"}
		resolver, _ := NewExpressionResolverWithFS(nil, mfs)

		res, err := resolveConfigPath(false, "", false, "", "ENV", "node", tools, getter, nil, nil, "/fallback", resolver, "normal", mfs)
		require.NoError(t, err)
		assert.Equal(t, "/fallback", res)
	})

	t.Run("PathType default case", func(t *testing.T) {
		mfs := &MockFileSystem{WD: "/work"}
		resolver, _ := NewExpressionResolverWithFS(nil, mfs)

		res, err := resolveConfigPath(true, "/p1", false, "", "", "node", nil, nil, nil, nil, "", resolver, "unknown", mfs)
		require.NoError(t, err)
		assert.Equal(t, "/p1", res)
	})
}

func TestUnit_Config_SetBaseDir_Full_Robustness(t *testing.T) {
	t.Parallel()

	t.Run("ConfigDefaults.SetBaseDir with slices", func(t *testing.T) {
		d := &ConfigDefaults{
			Mounts: []MountConfig{{Source: ConfigPath{Raw: "./src"}}},
			Devices: []DeviceConfig{{Source: ConfigPath{Raw: "/dev/v"}}},
		}
		d.SetBaseDir("/base")
		assert.Equal(t, "/base", d.Mounts[0].Source.BaseDir)
		assert.Equal(t, "/base", d.Devices[0].Source.BaseDir)
	})

	t.Run("ToolConfig.SetBaseDir with slices", func(t *testing.T) {
		tc := &ToolConfig{
			Mounts: []MountConfig{{Source: ConfigPath{Raw: "./src"}}},
			Devices: []DeviceConfig{{Source: ConfigPath{Raw: "/dev/v"}}},
		}
		tc.SetBaseDir("/base")
		assert.Equal(t, "/base", tc.Mounts[0].Source.BaseDir)
		assert.Equal(t, "/base", tc.Devices[0].Source.BaseDir)
	})
}

func TestUnit_Config_ResolveDevices_CLI_Invalid_Robustness(t *testing.T) {
	t.Parallel()

	t.Run("Invalid format in P2 CLI device", func(t *testing.T) {
		cli := CLIOptions{
			Image:    "node",
			ImageSet: true,
			Devices:  []string{":container"},
		}
		_, err := Resolve("node", cli, nil, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid device config:")
	})
}

func TestUnit_Config_ResolveConfigPath_Global_Robustness(t *testing.T) {
	t.Parallel()

	t.Run("Global config getter returns empty", func(t *testing.T) {
		global := &CDERunConfig{
			Defaults: ConfigDefaults{},
		}
		getter := func(c CDERunConfig) ConfigPath { return c.Defaults.MountSocketPath }
		mfs := &MockFileSystem{WD: "/work"}
		resolver, _ := NewExpressionResolverWithFS(nil, mfs)

		res, err := resolveConfigPath(false, "", false, "", "ENV", "node", nil, nil, global, getter, "/fallback", resolver, "normal", mfs)
		require.NoError(t, err)
		assert.Equal(t, "/fallback", res)
	})
}

func TestUnit_Config_ResolveConfigPath_Types_Robustness(t *testing.T) {
	t.Parallel()

	t.Run("PathType volume", func(t *testing.T) {
		mfs := &MockFileSystem{WD: "/work"}
		resolver, _ := NewExpressionResolverWithFS(nil, mfs)
		res, err := resolveConfigPath(true, "/src:/dst", false, "", "", "node", nil, nil, nil, nil, "", resolver, "volume", mfs)
		require.NoError(t, err)
		assert.Equal(t, "/src:/dst", res)
	})

	t.Run("PathType device", func(t *testing.T) {
		mfs := &MockFileSystem{WD: "/work"}
		resolver, _ := NewExpressionResolverWithFS(nil, mfs)
		res, err := resolveConfigPath(true, "/dev/v:/dev/v", false, "", "", "node", nil, nil, nil, nil, "", resolver, "device", mfs)
		require.NoError(t, err)
		assert.Equal(t, "/dev/v:/dev/v", res)
	})
}

func TestUnit_Config_ResolveFloat64Opt_Robustness(t *testing.T) {
	t.Parallel()

	t.Run("Invalid float in env", func(t *testing.T) {
		mfs := &MockFileSystem{
			Env: map[string]string{"CDERUN_CPUS": "invalid"},
		}
		def := OptionDef[*float64]{
			EnvKey:   "CDERUN_CPUS",
			Fallback: ptr(2.0),
			GlobalGetter: func(c CDERunConfig) *float64 { return c.Defaults.CPUs },
		}
		// Should fallback to Global/Fallback if env is invalid
		res := resolveFloat64Opt(def, false, 0, false, 0, "node", nil, nil, mfs)
		assert.InDelta(t, 2.0, res, 1e-9)
	})
}

func TestUnit_Config_ResolveFloat64Opt_More_Robustness(t *testing.T) {
	t.Parallel()

	t.Run("Fallback nil", func(t *testing.T) {
		def := OptionDef[*float64]{
			Fallback: nil,
		}
		res := resolveFloat64Opt(def, false, 0, false, 0, "node", nil, nil, &MockFileSystem{})
		assert.InDelta(t, 0.0, res, 1e-9)
	})
}
