package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ptr[T any](v T) *T {
	return &v
}

func TestUnit_Config_Option_Exhaustive(t *testing.T) {
	t.Run("resolveStringSliceCommaOpt", func(t *testing.T) {
		def := OptionDef[[]string]{
			EnvKey: "TEST_SLICE",
			GlobalGetter: func(c CDERunConfig) []string { return []string{"global"} },
		}
		mfs := &MockFileSystem{Env: map[string]string{"TEST_SLICE": "env1, env2"}}
		r, err := NewExpressionResolver(nil)
		require.NoError(t, err)

		// Env priority
		res := resolveStringSliceCommaOpt(def, false, "", false, "", "sub", nil, nil, r, mfs)
		assert.Equal(t, []string{"env1", "env2"}, res)

		// P2 priority
		res = resolveStringSliceCommaOpt(def, false, "", true, "cli1,cli2", "sub", nil, nil, r, mfs)
		assert.Equal(t, []string{"cli1", "cli2"}, res)

		// P1 priority
		res = resolveStringSliceCommaOpt(def, true, "p1a,p1b", false, "", "sub", nil, nil, r, mfs)
		assert.Equal(t, []string{"p1a", "p1b"}, res)

		// Fallback to global
		mfs.Env = nil
		res = resolveStringSliceCommaOpt(def, false, "", false, "", "sub", nil, &CDERunConfig{Defaults: ConfigDefaults{}}, r, mfs)
		assert.Equal(t, []string{"global"}, res)
	})

	t.Run("resolveFloat64Opt", func(t *testing.T) {
		def := OptionDef[*float64]{
			EnvKey: "TEST_FLOAT",
			Fallback: ptr(1.0),
		}
		mfs := &MockFileSystem{Env: map[string]string{"TEST_FLOAT": "2.5"}}

		// Env
		res := resolveFloat64Opt(def, false, 0, false, 0, "sub", nil, nil, mfs)
		assert.InDelta(t, 2.5, res, 1e-9)

		// Fallback
		mfs.Env = nil
		res = resolveFloat64Opt(def, false, 0, false, 0, "sub", nil, nil, mfs)
		assert.InDelta(t, 1.0, res, 1e-9)

		// Invalid env
		mfs.Env = map[string]string{"TEST_FLOAT": "invalid"}
		res = resolveFloat64Opt(def, false, 0, false, 0, "sub", nil, nil, mfs)
		assert.InDelta(t, 1.0, res, 1e-9)

		// Tool getter
		mfs.Env = nil
		f2 := 2.0
		def.ToolGetter = func(tc ToolConfig) *float64 { return &f2 }
		res = resolveFloat64Opt(def, false, 0, false, 0, "node", ToolsConfig{"node": ToolConfig{}}, nil, mfs)
		assert.InDelta(t, 2.0, res, 1e-9)

		// Global getter
		def.ToolGetter = nil
		f3 := 3.0
		def.GlobalGetter = func(c CDERunConfig) *float64 { return &f3 }
		res = resolveFloat64Opt(def, false, 0, false, 0, "node", nil, &CDERunConfig{}, mfs)
		assert.InDelta(t, 3.0, res, 1e-9)

		// P2 CLI
		res = resolveFloat64Opt(def, false, 0, true, 4.0, "node", nil, nil, mfs)
		assert.InDelta(t, 4.0, res, 1e-9)

		// P1 Override
		res = resolveFloat64Opt(def, true, 5.0, false, 0, "node", nil, nil, mfs)
		assert.InDelta(t, 5.0, res, 1e-9)
	})

	t.Run("resolveIntOpt", func(t *testing.T) {
		def := OptionDef[*int]{
			EnvKey:   "TEST_INT",
			Fallback: ptr(10),
		}
		mfs := &MockFileSystem{Env: map[string]string{"TEST_INT": "20"}}

		// Env
		res := resolveIntOpt(def, false, 0, false, 0, "sub", nil, nil, mfs)
		assert.Equal(t, 20, res)

		// Fallback
		mfs.Env = nil
		res = resolveIntOpt(def, false, 0, false, 0, "sub", nil, nil, mfs)
		assert.Equal(t, 10, res)

		// Invalid env
		mfs.Env = map[string]string{"TEST_INT": "invalid"}
		res = resolveIntOpt(def, false, 0, false, 0, "sub", nil, nil, mfs)
		assert.Equal(t, 10, res)

		// Tool getter
		mfs.Env = nil
		i2 := 30
		def.ToolGetter = func(tc ToolConfig) *int { return &i2 }
		res = resolveIntOpt(def, false, 0, false, 0, "node", ToolsConfig{"node": ToolConfig{}}, nil, mfs)
		assert.Equal(t, 30, res)

		// Global getter
		def.ToolGetter = nil
		i3 := 40
		def.GlobalGetter = func(c CDERunConfig) *int { return &i3 }
		res = resolveIntOpt(def, false, 0, false, 0, "node", nil, &CDERunConfig{}, mfs)
		assert.Equal(t, 40, res)

		// P2 CLI
		res = resolveIntOpt(def, false, 0, true, 50, "node", nil, nil, mfs)
		assert.Equal(t, 50, res)

		// P1 Override
		res = resolveIntOpt(def, true, 60, false, 0, "node", nil, nil, mfs)
		assert.Equal(t, 60, res)
	})

	t.Run("resolveEnvValues with strict error", func(t *testing.T) {
		r, err := NewExpressionResolver(nil)
		require.NoError(t, err)
		mfs := &MockFileSystem{}
		_, err = resolveEnvValues([]string{"UNSET"}, true, r, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "required environment variable not found")
	})

	t.Run("resolveEnvValues with expression error", func(t *testing.T) {
		mfs := &MockFileSystem{}
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)
		r.setError(assert.AnError)
		_, err = resolveEnvValues([]string{"ANY"}, false, r, mfs)
		require.Error(t, err)
	})

	t.Run("negative hang-timeout duration", func(t *testing.T) {
		cli := CLIOptions{HangTimeout: "-5s", HangTimeoutSet: true, Image: "alpine", ImageSet: true}
		_, err := Resolve("node", cli, nil, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "duration cannot be negative")
	})

	t.Run("resolveConfigPath with fallback and expression", func(t *testing.T) {
		mfs := &MockFileSystem{HomeDir: "/home/user"}
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)
		res, err := resolveConfigPath(false, "", false, "", "UNSET", "sub", nil, nil, nil, nil, "{{HOME}}/sock", r, "path", mfs)
		require.NoError(t, err)
		assert.Equal(t, "/home/user/sock", res)
	})

	t.Run("resolveStringOpt exhaustive", func(t *testing.T) {
		def := OptionDef[string]{EnvKey: "TEST_STR", Fallback: "fallback"}
		mfs := &MockFileSystem{Env: map[string]string{"TEST_STR": "env"}}
		r, err := NewExpressionResolver(nil)
		require.NoError(t, err)

		// Env
		res := resolveStringOpt(def, false, "", false, "", "sub", nil, nil, r, mfs)
		assert.Equal(t, "env", res)

		// Tool
		mfs.Env = nil
		def.ToolGetter = func(tc ToolConfig) string { return "tool" }
		res = resolveStringOpt(def, false, "", false, "", "node", ToolsConfig{"node": ToolConfig{}}, nil, r, mfs)
		assert.Equal(t, "tool", res)

		// Global
		def.ToolGetter = nil
		def.GlobalGetter = func(c CDERunConfig) string { return "global" }
		res = resolveStringOpt(def, false, "", false, "", "node", nil, &CDERunConfig{}, r, mfs)
		assert.Equal(t, "global", res)
	})

	t.Run("resolveStringSliceOpt exhaustive", func(t *testing.T) {
		def := OptionDef[[]string]{EnvKey: "TEST_SLICE"}
		mfs := &MockFileSystem{Env: map[string]string{"TEST_SLICE": "a:b"}}
		r, err := NewExpressionResolver(nil)
		require.NoError(t, err)

		// Env
		res := resolveStringSliceOpt(def, ":", nil, nil, "sub", nil, nil, r, mfs)
		assert.Equal(t, []string{"a", "b"}, res)

		// Tool
		mfs.Env = nil
		def.ToolGetter = func(tc ToolConfig) []string { return []string{"tool"} }
		res = resolveStringSliceOpt(def, ":", nil, nil, "node", ToolsConfig{"node": ToolConfig{}}, nil, r, mfs)
		assert.Equal(t, []string{"tool"}, res)

		// Global
		def.ToolGetter = nil
		def.GlobalGetter = func(c CDERunConfig) []string { return []string{"global"} }
		res = resolveStringSliceOpt(def, ":", nil, nil, "node", nil, &CDERunConfig{}, r, mfs)
		assert.Equal(t, []string{"global"}, res)

		// P2 CLI
		res = resolveStringSliceOpt(def, ":", nil, []string{"cli"}, "sub", nil, nil, r, mfs)
		assert.Equal(t, []string{"cli"}, res)

		// P1 Override
		res = resolveStringSliceOpt(def, ":", []string{"p1"}, nil, "sub", nil, nil, r, mfs)
		assert.Equal(t, []string{"p1"}, res)
	})
}

func TestUnit_Resolver_Priority_AllLayers(t *testing.T) {
	t.Parallel()
	t.Run("P1 Override takes priority over P2 CLI", func(t *testing.T) {
		cli := CLIOptions{
			Image:        "alpine",
			ImageSet:     true,
			TTY:          true,
			TTYSet:       true,
			CderunTTY:    false,
			CderunTTYSet: true,
		}
		res, err := Resolve("node", cli, nil, nil)
		require.NoError(t, err)
		assert.False(t, res.TTY)
	})

	t.Run("P2 CLI takes priority over P3 Env", func(t *testing.T) {
		mfs := &MockFileSystem{
			Env: map[string]string{"CDERUN_TTY": "false"},
		}
		cli := CLIOptions{
			Image:    "alpine",
			ImageSet: true,
			TTY:      true,
			TTYSet:   true,
		}
		res, err := ResolveWithFS("node", cli, nil, nil, mfs)
		require.NoError(t, err)
		assert.True(t, res.TTY)
	})

	t.Run("P3 Env takes priority over P4 Tool", func(t *testing.T) {
		mfs := &MockFileSystem{
			Env: map[string]string{"CDERUN_TTY": "true"},
		}
		tools := ToolsConfig{
			"node": ToolConfig{
				Image: "node",
				TTY:   ptr(false),
			},
		}
		res, err := ResolveWithFS("node", CLIOptions{}, tools, nil, mfs)
		require.NoError(t, err)
		assert.True(t, res.TTY)
	})

	t.Run("P4 Tool takes priority over P5 Global", func(t *testing.T) {
		tools := ToolsConfig{
			"node": ToolConfig{
				Image: "node",
				TTY:   ptr(true),
			},
		}
		global := &CDERunConfig{
			Defaults: ConfigDefaults{
				TTY: ptr(false),
			},
		}
		res, err := Resolve("node", CLIOptions{}, tools, global)
		require.NoError(t, err)
		assert.True(t, res.TTY)
	})
}

func TestUnit_Resolver_AutoDetection(t *testing.T) {
	t.Parallel()
	t.Run("Infer runtime from socket path", func(t *testing.T) {
		cli := CLIOptions{
			Image:         "alpine",
			ImageSet:      true,
			SocketPath:    "/run/podman/podman.sock",
			SocketPathSet: true,
		}
		res, err := Resolve("node", cli, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, "podman", res.Runtime)
	})

	t.Run("Default socket for runtime", func(t *testing.T) {
		cli := CLIOptions{
			Image:      "alpine",
			ImageSet:   true,
			Runtime:    "docker",
			RuntimeSet: true,
		}
		res, err := Resolve("node", cli, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, "/var/run/docker.sock", res.SocketPath)
	})
}

func TestUnit_Resolver_Mounts_Exhaustive(t *testing.T) {
	t.Parallel()
	t.Run("Multiple mounts from environment", func(t *testing.T) {
		mfs := &MockFileSystem{
			Env: map[string]string{"CDERUN_MOUNT": "source=/a,target=/b ; source=/c,target=/d,readonly"},
		}
		res, err := ResolveWithFS("node", CLIOptions{Image: "alpine", ImageSet: true}, nil, nil, mfs)
		require.NoError(t, err)
		require.Len(t, res.Mounts, 2)
		assert.Equal(t, "/a", res.Mounts[0].Source)
		assert.True(t, res.Mounts[1].ReadOnly)
	})
}

func TestUnit_Resolver_Env_Exhaustive(t *testing.T) {
	t.Parallel()
	mfs := &MockFileSystem{
		Env: map[string]string{
			"HOST_VAR": "host-val",
			"CDERUN_ENV": "ENV_VAR=env-val; HOST_VAR",
		},
	}
	t.Run("Env resolution from all layers", func(t *testing.T) {
		tools := ToolsConfig{
			"node": ToolConfig{
				Image: "node",
				Env: []string{"TOOL_VAR=tool-val"},
			},
		}
		res, err := ResolveWithFS("node", CLIOptions{}, tools, nil, mfs)
		require.NoError(t, err)
		assert.Contains(t, res.Env, "ENV_VAR=env-val")
		assert.Contains(t, res.Env, "HOST_VAR=host-val")
	})

	t.Run("Strict mode env validation", func(t *testing.T) {
		cli := CLIOptions{
			Image: "alpine", ImageSet: true,
			Env: []string{"NONEXISTENT"},
			StrictEnv: true, StrictEnvSet: true,
		}
		_, err := ResolveWithFS("node", cli, nil, nil, mfs)
		require.Error(t, err)
	})
}

func TestUnit_Resolver_Devices_Advanced(t *testing.T) {
	t.Parallel()
	t.Run("Device resolution priority", func(t *testing.T) {
		global := &CDERunConfig{
			Defaults: ConfigDefaults{
				Devices: []DeviceConfig{{
					Source: ConfigPath{Raw: "/dev/global"},
					Destination: ConfigPath{Raw: "/dev/global"},
				}},
			},
		}
		tools := ToolsConfig{
			"node": ToolConfig{
				Image: "node",
				Devices: []DeviceConfig{{
					Source: ConfigPath{Raw: "/dev/tool"},
					Destination: ConfigPath{Raw: "/dev/tool"},
				}},
			},
		}
		res, err := Resolve("node", CLIOptions{}, tools, global)
		require.NoError(t, err)
		require.Len(t, res.Devices, 1)
		assert.Equal(t, "/dev/tool", res.Devices[0].PathOnHost)
	})
}

func TestUnit_Resolver_Misc_Exhaustive(t *testing.T) {
	t.Parallel()
	t.Run("HangTimeout parsing", func(t *testing.T) {
		cli := CLIOptions{
			Image:          "alpine",
			ImageSet:       true,
			HangTimeout:    "5s",
			HangTimeoutSet: true,
		}
		res, err := Resolve("node", cli, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, 5*time.Second, res.HangTimeout)
	})

	t.Run("Memory string parsing", func(t *testing.T) {
		cli := CLIOptions{
			Image:     "alpine",
			ImageSet:  true,
			Memory:    "512MiB",
			MemorySet: true,
		}
		res, err := Resolve("node", cli, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, int64(512*1024*1024), res.Memory)
	})
}

func TestUnit_Resolver_Expressions_Exhaustive(t *testing.T) {
	t.Parallel()
	mfs := &MockFileSystem{
		WD: "/app",
		Files: map[string][]byte{
			"/app/.version": []byte("1.0"),
		},
	}
	t.Run("Expression in environment variable override", func(t *testing.T) {
		mfs.Env = map[string]string{"CDERUN_IMAGE": "node:{{file:.version}}"}
		res, err := ResolveWithFS("node", CLIOptions{}, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, "node:1.0", res.Image)
	})
}

func TestUnit_Resolver_Exhaustive_Additional(t *testing.T) {
	t.Parallel()
	t.Run("Diagnosis mode bypasses image check", func(t *testing.T) {
		cli := CLIOptions{Diagnosis: true, DiagnosisSet: true}
		res, err := Resolve("unknown", cli, nil, nil)
		require.NoError(t, err)
		assert.True(t, res.Diagnosis)
	})

	t.Run("Transitive auto-enablement cderun to socket", func(t *testing.T) {
		cli := CLIOptions{Image: "alpine", ImageSet: true, MountCderun: true, MountCderunSet: true}
		res, err := Resolve("sh", cli, nil, nil)
		require.NoError(t, err)
		assert.True(t, res.MountSocket)
	})

	t.Run("Float64 resolution", func(t *testing.T) {
		cli := CLIOptions{Image: "alpine", ImageSet: true, CPUs: 1.5, CPUsSet: true}
		res, err := Resolve("node", cli, nil, nil)
		require.NoError(t, err)
		assert.InDelta(t, 1.5, res.CPUs, 0.0001)
	})

	t.Run("String slice with comma resolution", func(t *testing.T) {
		mfs := &MockFileSystem{Env: map[string]string{"CDERUN_DNS": "8.8.8.8,1.1.1.1"}}
		res, err := ResolveWithFS("node", CLIOptions{Image: "alpine", ImageSet: true}, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, []string{"8.8.8.8", "1.1.1.1"}, res.DNS)
	})
}

func TestUnit_Resolver_Exhaustive_Advanced(t *testing.T) {
	t.Parallel()
	t.Run("MountTools resolution", func(t *testing.T) {
		cli := CLIOptions{Image: "alpine", ImageSet: true, MountTools: "tool1,tool2", MountToolsSet: true}
		res, err := Resolve("sh", cli, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, []string{"tool1", "tool2"}, res.MountTools)
	})

	t.Run("Log settings resolution", func(t *testing.T) {
		cli := CLIOptions{Image: "alpine", ImageSet: true, LogLevel: "debug", LogLevelSet: true}
		res, err := Resolve("sh", cli, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, "debug", res.LogLevel)
	})

	t.Run("resolveDevices from environment", func(t *testing.T) {
		mfs := &MockFileSystem{Env: map[string]string{"CDERUN_DEVICE": "/dev/a:/dev/b:rw"}}
		res, err := ResolveWithFS("node", CLIOptions{Image: "alpine", ImageSet: true}, nil, nil, mfs)
		require.NoError(t, err)
		require.Len(t, res.Devices, 1)
		assert.Equal(t, "/dev/a", res.Devices[0].PathOnHost)
	})

	t.Run("resolveDevices invalid format in CDERUN_DEVICE", func(t *testing.T) {
		mfs := &MockFileSystem{Env: map[string]string{"CDERUN_DEVICE": ":"}}
		_, err := ResolveWithFS("node", CLIOptions{Image: "alpine", ImageSet: true}, nil, nil, mfs)
		require.Error(t, err)
	})

	t.Run("resolveMounts from environment", func(t *testing.T) {
		mfs := &MockFileSystem{Env: map[string]string{"CDERUN_MOUNT": "source=/a,target=/b"}}
		res, err := ResolveWithFS("node", CLIOptions{Image: "alpine", ImageSet: true}, nil, nil, mfs)
		require.NoError(t, err)
		require.Len(t, res.Mounts, 1)
		assert.Equal(t, "/a", res.Mounts[0].Source)
	})

	t.Run("resolveMounts invalid format in CDERUN_MOUNT", func(t *testing.T) {
		mfs := &MockFileSystem{Env: map[string]string{"CDERUN_MOUNT": "invalid"}}
		_, err := ResolveWithFS("node", CLIOptions{Image: "alpine", ImageSet: true}, nil, nil, mfs)
		require.Error(t, err)
	})

	t.Run("resolveConfigPath from CDERUN_SOCKET_PATH", func(t *testing.T) {
		mfs := &MockFileSystem{Env: map[string]string{"CDERUN_SOCKET_PATH": "/run/my.sock"}}
		res, err := ResolveWithFS("node", CLIOptions{Image: "alpine", ImageSet: true}, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, "/run/my.sock", res.SocketPath)
	})

	t.Run("resolveConfigPath from global config", func(t *testing.T) {
		global := &CDERunConfig{
			SocketPath: ConfigPath{Raw: "/global.sock"},
		}
		res, err := ResolveWithFS("node", CLIOptions{Image: "alpine", ImageSet: true}, nil, global, &MockFileSystem{})
		require.NoError(t, err)
		assert.Equal(t, "/global.sock", res.SocketPath)
	})

	t.Run("Auto-detection exhaustive", func(t *testing.T) {
		// docker.sock exists
		mfs := &MockFileSystem{
			Dirs: map[string]bool{"/var/run": true},
			Files: map[string][]byte{"/var/run/docker.sock": []byte("")},
		}
		res, err := ResolveWithFS("sh", CLIOptions{Image: "alpine", ImageSet: true}, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, "docker", res.Runtime)
		assert.Equal(t, "/var/run/docker.sock", res.SocketPath)

		// podman.sock exists
		mfs = &MockFileSystem{
			Dirs: map[string]bool{"/run/podman": true},
			Files: map[string][]byte{"/run/podman/podman.sock": []byte("")},
		}
		res, err = ResolveWithFS("sh", CLIOptions{Image: "alpine", ImageSet: true}, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, "podman", res.Runtime)
		assert.Equal(t, "/run/podman/podman.sock", res.SocketPath)

		// specified podman but no socket, should use default podman socket
		res, err = ResolveWithFS("sh", CLIOptions{Image: "alpine", ImageSet: true, Runtime: "podman", RuntimeSet: true}, nil, nil, &MockFileSystem{})
		require.NoError(t, err)
		assert.Equal(t, "podman", res.Runtime)
		assert.Equal(t, "/run/podman/podman.sock", res.SocketPath)

		// specified docker but no socket
		res, err = ResolveWithFS("sh", CLIOptions{Image: "alpine", ImageSet: true, Runtime: "docker", RuntimeSet: true}, nil, nil, &MockFileSystem{})
		require.NoError(t, err)
		assert.Equal(t, "docker", res.Runtime)
		assert.Equal(t, "/var/run/docker.sock", res.SocketPath)

		// specified unknown runtime (e.g. from global)
		global := &CDERunConfig{Runtime: "containerd"}
		res, err = ResolveWithFS("sh", CLIOptions{Image: "alpine", ImageSet: true}, nil, global, &MockFileSystem{})
		require.NoError(t, err)
		assert.Equal(t, "containerd", res.Runtime)
		assert.Equal(t, "/var/run/docker.sock", res.SocketPath) // Fallback to docker socket
	})

	t.Run("Resolve coverage final", func(t *testing.T) {
		mfs := &MockFileSystem{WD: "/app"}
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		// resolveDevices P2
		resDevices, err := resolveDevices(nil, []string{"/dev/p2:/dev/p2"}, "", nil, nil, r, mfs)
		require.NoError(t, err)
		assert.Equal(t, "/dev/p2", resDevices[0].PathOnHost)

		// resolveMounts P2 and Global
		resMounts, err := resolveMounts(nil, []string{"source=/p2,target=/p2"}, "", nil, nil, r, mfs)
		require.NoError(t, err)
		assert.Equal(t, "/p2", resMounts[0].Source)

		global := &CDERunConfig{Defaults: ConfigDefaults{Mounts: []MountConfig{{Source: ConfigPath{Raw: "/global"}, Target: ConfigPath{Raw: "/global"}}}}}
		resMounts, err = resolveMounts(nil, nil, "", nil, global, r, mfs)
		require.NoError(t, err)
		assert.Equal(t, "/global", resMounts[0].Source)

		// resolveConfigPath P1 and CLI
		resPath, err := resolveConfigPath(true, "/p1", false, "", "", "", nil, nil, nil, nil, "", r, "path", mfs)
		require.NoError(t, err)
		assert.Equal(t, "/p1", resPath)

		resPath, err = resolveConfigPath(false, "", true, "/cli", "", "", nil, nil, nil, nil, "", r, "path", mfs)
		require.NoError(t, err)
		assert.Equal(t, "/cli", resPath)
	})

	t.Run("Resolve errors", func(t *testing.T) {
		// no image
		_, err := ResolveWithFS("sh", CLIOptions{}, nil, nil, &MockFileSystem{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no image mapping found")

		// resolveDevices invalid format in CLI
		cliDev := CLIOptions{Image: "alpine", ImageSet: true, Devices: []string{":"}}
		_, err = ResolveWithFS("sh", cliDev, nil, nil, &MockFileSystem{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid device config")

		// resolveMounts invalid format in CLI
		cliMnt := CLIOptions{Image: "alpine", ImageSet: true, Mounts: []string{"invalid"}}
		_, err = ResolveWithFS("sh", cliMnt, nil, nil, &MockFileSystem{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid mount config")

		// invalid memory
		_, err = ResolveWithFS("sh", CLIOptions{Image: "alpine", ImageSet: true, Memory: "invalid", MemorySet: true}, nil, nil, &MockFileSystem{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid memory value")

		// Expression resolver error
		mfsError := &customMockFS{homeDirErr: assert.AnError}
		resError, err := ResolveWithFS("sh", CLIOptions{Image: "alpine", ImageSet: true}, nil, nil, mfsError)
		require.NoError(t, err)
		assert.NotNil(t, resError)

		// Expression resolver error is recorded but doesn't immediately stop ResolveWithFS until the end or specific points.
		// Actually NewExpressionResolverWithFS swallows homeDirErr and sets home to "".
		// Let's try to trigger an error in ResolveWithFS.

		// Expression error
		mfsExpr := &MockFileSystem{WD: "/app"}
		cli := CLIOptions{Image: "alpine", ImageSet: true, Env: []string{"VAR={{file:missing}}"}}
		_, err = ResolveWithFS("sh", cli, nil, nil, mfsExpr)
		require.Error(t, err)

		// Test resolveEnv with Tool getter
		tools := ToolsConfig{"node": ToolConfig{Env: []string{"TOOL=1"}}}
		res, err := ResolveWithFS("node", CLIOptions{Image: "alpine", ImageSet: true}, tools, nil, &MockFileSystem{})
		require.NoError(t, err)
		assert.Contains(t, res.Env, "TOOL=1")

		// Test resolveDevices with Tool getter
		tools = ToolsConfig{"node": ToolConfig{Devices: []DeviceConfig{{Source: ConfigPath{Raw: "/dev/t"}}}}}
		res, err = ResolveWithFS("node", CLIOptions{Image: "alpine", ImageSet: true}, tools, nil, &MockFileSystem{})
		require.NoError(t, err)
		require.Len(t, res.Devices, 1)
		assert.Equal(t, "/dev/t", res.Devices[0].PathOnHost)

		// Test resolveMounts with Tool getter
		tools = ToolsConfig{"node": ToolConfig{Mounts: []MountConfig{{Source: ConfigPath{Raw: "/s"}, Target: ConfigPath{Raw: "/t"}}}}}
		res, err = ResolveWithFS("node", CLIOptions{Image: "alpine", ImageSet: true}, tools, nil, &MockFileSystem{})
		require.NoError(t, err)
		require.Len(t, res.Mounts, 1)
		assert.Equal(t, "/s", res.Mounts[0].Source)
	})

	t.Run("PullMaxRetries and PullBackoffBase errors", func(t *testing.T) {
		mfs := &MockFileSystem{}
		cli := CLIOptions{Image: "alpine", ImageSet: true}

		// PullMaxRetries <= 0
		cli.CderunPullMaxRetries = 0
		cli.CderunPullMaxRetriesSet = true
		_, err := ResolveWithFS("node", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be greater than 0")

		// PullBackoffBase invalid
		cli.CderunPullMaxRetries = 3
		cli.CderunPullBackoffBase = "invalid"
		cli.CderunPullBackoffBaseSet = true
		_, err = ResolveWithFS("node", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse PullBackoffBase")

		// PullBackoffBase non-positive
		cli.CderunPullBackoffBase = "0s"
		_, err = ResolveWithFS("node", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be positive")
	})

	t.Run("Memory and Expression errors", func(t *testing.T) {
		mfs := &MockFileSystem{}
		cli := CLIOptions{Image: "alpine", ImageSet: true}

		// Invalid memory format
		cli.CderunMemory = "invalid"
		cli.CderunMemorySet = true
		_, err := ResolveWithFS("node", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid memory value")

		// Expression error already present
		mfsExpr := &MockFileSystem{WD: "/app"}
		cliExpr := CLIOptions{Image: "alpine", ImageSet: true, Env: []string{"VAR={{file:missing}}"}, Memory: "1G", MemorySet: true}
		_, err = ResolveWithFS("node", cliExpr, nil, nil, mfsExpr)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "file not found")
	})
}

func TestUnit_Resolver_Optional_Mounts(t *testing.T) {
	t.Parallel()
	mfs := &MockFileSystem{
		WD: "/app",
		Files: map[string][]byte{
			"/app/exists": []byte("content"),
		},
	}

	t.Run("Skip optional mount when source is missing", func(t *testing.T) {
		cli := CLIOptions{
			Image:    "alpine",
			ImageSet: true,
			Mounts:   []string{"source=/app/missing,target=/data,optional"},
		}
		res, err := ResolveWithFS("node", cli, nil, nil, mfs)
		require.NoError(t, err)
		assert.Empty(t, res.Mounts)
	})

	t.Run("Keep optional mount when source exists", func(t *testing.T) {
		cli := CLIOptions{
			Image:    "alpine",
			ImageSet: true,
			Mounts:   []string{"source=/app/exists,target=/data,optional"},
		}
		res, err := ResolveWithFS("node", cli, nil, nil, mfs)
		require.NoError(t, err)
		require.Len(t, res.Mounts, 1)
		assert.Equal(t, "/app/exists", res.Mounts[0].Source)
		assert.True(t, res.Mounts[0].Optional)
	})

	t.Run("Non-optional mount remains even if source is missing (handled by runtime)", func(t *testing.T) {
		cli := CLIOptions{
			Image:    "alpine",
			ImageSet: true,
			Mounts:   []string{"source=/app/missing,target=/data"},
		}
		res, err := ResolveWithFS("node", cli, nil, nil, mfs)
		require.NoError(t, err)
		require.Len(t, res.Mounts, 1)
		assert.Equal(t, "/app/missing", res.Mounts[0].Source)
		assert.False(t, res.Mounts[0].Optional)
	})
}

func TestUnit_Resolver_Optional_Mount_With_Expression(t *testing.T) {
	t.Parallel()
	mfs := &MockFileSystem{
		WD: "/app",
		Files: map[string][]byte{
			"/host/config/foo": []byte("content"),
		},
	}
	hostCtx := &HostContext{
		Level: 1,
		Mounts: []MountMapping{
			{Source: "/host/config", Target: "/config", Level: 1},
		},
	}
	r, err := NewExpressionResolverWithFS(hostCtx, mfs)
	require.NoError(t, err)

	t.Run("Resolve source with HostContext correctly", func(t *testing.T) {
		mcs := []MountConfig{
			{
				Type:     "bind",
				Source:   ConfigPath{Raw: "/config/foo"},
				Target:   ConfigPath{Raw: "/data"},
				Optional: true,
			},
		}
		// Resolve the mounts. /config/foo should resolve to /host/config/foo via HostContext.
		res, err := resolveMounts(nil, nil, "", nil, &CDERunConfig{Defaults: ConfigDefaults{Mounts: mcs}}, r, mfs)
		require.NoError(t, err)
		require.Len(t, res, 1)
		assert.Equal(t, "/host/config/foo", res[0].Source)
	})

	t.Run("Skip when source resolved via HostContext is missing", func(t *testing.T) {
		mcs := []MountConfig{
			{
				Type:     "bind",
				Source:   ConfigPath{Raw: "/config/missing"},
				Target:   ConfigPath{Raw: "/data"},
				Optional: true,
			},
		}
		res, err := resolveMounts(nil, nil, "", nil, &CDERunConfig{Defaults: ConfigDefaults{Mounts: mcs}}, r, mfs)
		require.NoError(t, err)
		assert.Empty(t, res)
	})

	t.Run("Error when Stat fails with non-NotExist error", func(t *testing.T) {
		mfsError := &MockFileSystem{
			WD:      "/app",
			StatErr: assert.AnError,
		}
		mcs := []MountConfig{
			{
				Type:     "bind",
				Source:   ConfigPath{Raw: "/host/config/foo"},
				Target:   ConfigPath{Raw: "/data"},
				Optional: true,
			},
		}
		rError, err := NewExpressionResolverWithFS(nil, mfsError)
		require.NoError(t, err)

		_, err = resolveMounts(nil, nil, "", nil, &CDERunConfig{Defaults: ConfigDefaults{Mounts: mcs}}, rError, mfsError)
		require.Error(t, err)
		assert.Equal(t, assert.AnError, err)
	})
}
