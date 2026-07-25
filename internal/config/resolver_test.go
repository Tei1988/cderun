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
			EnvKey:       "TEST_SLICE",
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
		}
		mfs := &MockFileSystem{Env: map[string]string{"TEST_FLOAT": "2.5"}}

		// Env
		res := must(resolveFloat64Opt(def, 1.0, false, 0, false, 0, "sub", nil, nil, mfs))
		assert.InDelta(t, 2.5, res, 1e-9)

		// Fallback
		mfs.Env = nil
		res = must(resolveFloat64Opt(def, 1.0, false, 0, false, 0, "sub", nil, nil, mfs))
		assert.InDelta(t, 1.0, res, 1e-9)

		// Invalid env
		mfs.Env = map[string]string{"TEST_FLOAT": "invalid"}
		_, err := resolveFloat64Opt(def, 1.0, false, 0, false, 0, "sub", nil, nil, mfs)
		require.Error(t, err)

		// Tool getter
		mfs.Env = nil
		f2 := 2.0
		def.ToolGetter = func(tc ToolConfig) *float64 { return &f2 }
		res = must(resolveFloat64Opt(def, 1.0, false, 0, false, 0, "node", ToolsConfig{"node": ToolConfig{}}, nil, mfs))
		assert.InDelta(t, 2.0, res, 1e-9)

		// Global getter
		def.ToolGetter = nil
		f3 := 3.0
		def.GlobalGetter = func(c CDERunConfig) *float64 { return &f3 }
		res = must(resolveFloat64Opt(def, 1.0, false, 0, false, 0, "node", nil, &CDERunConfig{}, mfs))
		assert.InDelta(t, 3.0, res, 1e-9)

		// P2 CLI
		res = must(resolveFloat64Opt(def, 1.0, false, 0, true, 4.0, "node", nil, nil, mfs))
		assert.InDelta(t, 4.0, res, 1e-9)

		// P1 Override
		res = must(resolveFloat64Opt(def, 1.0, true, 5.0, false, 0, "node", nil, nil, mfs))
		assert.InDelta(t, 5.0, res, 1e-9)
	})

	t.Run("resolveIntOpt", func(t *testing.T) {
		def := OptionDef[*int]{
			EnvKey: "TEST_INT",
		}
		mfs := &MockFileSystem{Env: map[string]string{"TEST_INT": "20"}}

		// Env
		res := must(resolveIntOpt(def, 10, false, 0, false, 0, "sub", nil, nil, mfs))
		assert.Equal(t, 20, res)

		// Fallback
		mfs.Env = nil
		res = must(resolveIntOpt(def, 10, false, 0, false, 0, "sub", nil, nil, mfs))
		assert.Equal(t, 10, res)

		// Invalid env
		mfs.Env = map[string]string{"TEST_INT": "invalid"}
		_, err := resolveIntOpt(def, 10, false, 0, false, 0, "sub", nil, nil, mfs)
		require.Error(t, err)

		// Tool getter
		mfs.Env = nil
		i2 := 30
		def.ToolGetter = func(tc ToolConfig) *int { return &i2 }
		res = must(resolveIntOpt(def, 10, false, 0, false, 0, "node", ToolsConfig{"node": ToolConfig{}}, nil, mfs))
		assert.Equal(t, 30, res)

		// Global getter
		def.ToolGetter = nil
		i3 := 40
		def.GlobalGetter = func(c CDERunConfig) *int { return &i3 }
		res = must(resolveIntOpt(def, 10, false, 0, false, 0, "node", nil, &CDERunConfig{}, mfs))
		assert.Equal(t, 40, res)

		// P2 CLI
		res = must(resolveIntOpt(def, 10, false, 0, true, 50, "node", nil, nil, mfs))
		assert.Equal(t, 50, res)

		// P1 Override
		res = must(resolveIntOpt(def, 10, true, 60, false, 0, "node", nil, nil, mfs))
		assert.Equal(t, 60, res)
	})

	t.Run("resolveEnvValues contains plaintext", func(t *testing.T) {
		// Use a key that MaskSensitiveEnv would normally target (e.g. MY_PASSWORD)
		mfs := &MockFileSystem{Env: map[string]string{"MY_PASSWORD": "secret"}}
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)
		res, err := resolveEnvValues([]string{"MY_PASSWORD"}, nil, false, r, mfs)
		require.NoError(t, err)
		// Verification: ensure the resolver returns plaintext for container execution
		assert.Equal(t, []string{"MY_PASSWORD=secret"}, res)
	})

	t.Run("resolveEnvValues with strict error", func(t *testing.T) {
		r, err := NewExpressionResolver(nil)
		require.NoError(t, err)
		mfs := &MockFileSystem{}
		_, err = resolveEnvValues([]string{"UNSET"}, nil, true, r, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "required environment variable not found")
	})

	t.Run("resolveEnvValues with expression error", func(t *testing.T) {
		mfs := &MockFileSystem{}
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)
		r.setError(assert.AnError)
		_, err = resolveEnvValues([]string{"ANY"}, nil, false, r, mfs)
		require.Error(t, err)
	})

	t.Run("negative hang-timeout duration", func(t *testing.T) {
		cli := CLIOptions{HangTimeout: ptr("-5s"), Image: ptr("alpine")}
		_, err := Resolve("node", &cli, nil, nil)
		var cfgErr *InvalidConfigError
		require.ErrorAs(t, err, &cfgErr)
		assert.Equal(t, "hang-timeout", cfgErr.Field)
		assert.Contains(t, cfgErr.Error(), "duration cannot be negative")
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
			Image:     ptr("alpine"),
			TTY:       ptr(true),
			CderunTTY: ptr(false),
		}
		res, err := Resolve("node", &cli, nil, nil)
		require.NoError(t, err)
		assert.False(t, res.TTY)
	})

	t.Run("P2 CLI takes priority over P3 Env", func(t *testing.T) {
		mfs := &MockFileSystem{
			Env: map[string]string{"CDERUN_TTY": "false"},
		}
		cli := CLIOptions{
			Image: ptr("alpine"),
			TTY:   ptr(true),
		}
		res, err := ResolveWithFS("node", &cli, nil, nil, mfs)
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
		res, err := ResolveWithFS("node", &CLIOptions{}, tools, nil, mfs)
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
		res, err := Resolve("node", &CLIOptions{}, tools, global)
		require.NoError(t, err)
		assert.True(t, res.TTY)
	})
}

func TestUnit_Resolver_AutoDetection(t *testing.T) {
	t.Parallel()
	t.Run("Infer runtime from socket path", func(t *testing.T) {
		cli := CLIOptions{
			Image:      ptr("alpine"),
			SocketPath: ptr("/run/podman/podman.sock"),
		}
		res, err := Resolve("node", &cli, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, "podman", res.Runtime)
	})

	t.Run("Default socket for runtime", func(t *testing.T) {
		cli := CLIOptions{
			Image:   ptr("alpine"),
			Runtime: ptr("docker"),
		}
		res, err := Resolve("node", &cli, nil, nil)
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
		res, err := ResolveWithFS("node", &CLIOptions{Image: ptr("alpine")}, nil, nil, mfs)
		require.NoError(t, err)
		require.Len(t, res.Mounts, 2)
		assert.Equal(t, "/a", res.Mounts[0].Source)
		assert.True(t, res.Mounts[1].ReadOnly)
	})

	t.Run("Mounts from environment with empty segments and extra spaces", func(t *testing.T) {
		mfs := &MockFileSystem{
			Env: map[string]string{"CDERUN_MOUNT": "  source=/a,target=/b  ; ;   source=/c,target=/d   "},
		}
		res, err := ResolveWithFS("node", &CLIOptions{Image: ptr("alpine")}, nil, nil, mfs)
		require.NoError(t, err)
		require.Len(t, res.Mounts, 2)
		assert.Equal(t, "/a", res.Mounts[0].Source)
		assert.Equal(t, "/c", res.Mounts[1].Source)
	})
}

func TestUnit_Resolver_Env_Exhaustive(t *testing.T) {
	t.Parallel()
	mfs := &MockFileSystem{
		Env: map[string]string{
			"HOST_VAR":   "host-val",
			"CDERUN_ENV": "ENV_VAR=env-val; HOST_VAR",
		},
	}
	t.Run("Env resolution from all layers", func(t *testing.T) {
		tools := ToolsConfig{
			"node": ToolConfig{
				Image: "node",
				Env:   []string{"TOOL_VAR=tool-val"},
			},
		}
		res, err := ResolveWithFS("node", &CLIOptions{}, tools, nil, mfs)
		require.NoError(t, err)
		assert.Contains(t, res.Env, "ENV_VAR=env-val")
		assert.Contains(t, res.Env, "HOST_VAR=host-val")
	})

	t.Run("Strict mode env validation", func(t *testing.T) {
		cli := CLIOptions{
			Image: ptr("alpine"), Env: []string{"NONEXISTENT"},
			StrictEnv: ptr(true)}
		_, err := ResolveWithFS("node", &cli, nil, nil, mfs)
		require.Error(t, err)
	})

	t.Run("Env from environment with empty segments and extra spaces", func(t *testing.T) {
		mfs2 := &MockFileSystem{
			Env: map[string]string{"CDERUN_ENV": "  A=1  ; ;   B=2   "},
		}
		res, err := ResolveWithFS("node", &CLIOptions{Image: ptr("alpine")}, nil, nil, mfs2)
		require.NoError(t, err)
		assert.Contains(t, res.Env, "A=1")
		assert.Contains(t, res.Env, "B=2")
		assert.Len(t, res.Env, 2)
	})
}

func TestUnit_Resolver_Devices_Advanced(t *testing.T) {
	t.Parallel()
	t.Run("Device resolution priority", func(t *testing.T) {
		global := &CDERunConfig{
			Defaults: ConfigDefaults{
				Devices: []DeviceConfig{{
					Source:      ConfigPath{Raw: "/dev/global"},
					Destination: ConfigPath{Raw: "/dev/global"},
				}},
			},
		}
		tools := ToolsConfig{
			"node": ToolConfig{
				Image: "node",
				Devices: []DeviceConfig{{
					Source:      ConfigPath{Raw: "/dev/tool"},
					Destination: ConfigPath{Raw: "/dev/tool"},
				}},
			},
		}
		res, err := Resolve("node", &CLIOptions{}, tools, global)
		require.NoError(t, err)
		require.Len(t, res.Devices, 1)
		assert.Equal(t, "/dev/tool", res.Devices[0].PathOnHost)
	})
}

func TestUnit_Resolver_Misc_Exhaustive(t *testing.T) {
	t.Parallel()
	t.Run("HangTimeout parsing", func(t *testing.T) {
		cli := CLIOptions{
			Image:       ptr("alpine"),
			HangTimeout: ptr("5s"),
		}
		res, err := Resolve("node", &cli, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, 5*time.Second, res.HangTimeout)
	})

	t.Run("Memory string parsing", func(t *testing.T) {
		cli := CLIOptions{
			Image:  ptr("alpine"),
			Memory: ptr("512MiB"),
		}
		res, err := Resolve("node", &cli, nil, nil)
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
		res, err := ResolveWithFS("node", &CLIOptions{}, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, "node:1.0", res.Image)
	})
}

func TestUnit_Resolver_Exhaustive_Additional(t *testing.T) {
	t.Parallel()
	t.Run("Diagnosis mode bypasses image check", func(t *testing.T) {
		cli := CLIOptions{Diagnosis: ptr(true)}
		res, err := Resolve("unknown", &cli, nil, nil)
		require.NoError(t, err)
		assert.True(t, res.Diagnosis)
	})

	t.Run("Transitive auto-enablement cderun to socket", func(t *testing.T) {
		cli := CLIOptions{Image: ptr("alpine"), MountCderun: ptr(true)}
		res, err := Resolve("sh", &cli, nil, nil)
		require.NoError(t, err)
		assert.True(t, res.MountSocket)
	})

	t.Run("Float64 resolution", func(t *testing.T) {
		cli := CLIOptions{Image: ptr("alpine"), CPUs: ptr(1.5)}
		res, err := Resolve("node", &cli, nil, nil)
		require.NoError(t, err)
		assert.InDelta(t, 1.5, res.CPUs, 0.0001)
	})

	t.Run("String slice with comma resolution", func(t *testing.T) {
		mfs := &MockFileSystem{Env: map[string]string{"CDERUN_DNS": "8.8.8.8,1.1.1.1"}}
		res, err := ResolveWithFS("node", &CLIOptions{Image: ptr("alpine")}, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, []string{"8.8.8.8", "1.1.1.1"}, res.DNS)
	})
}

func TestUnit_Resolver_Exhaustive_Advanced(t *testing.T) {
	t.Parallel()
	t.Run("MountTools resolution", func(t *testing.T) {
		cli := CLIOptions{Image: ptr("alpine"), MountTools: ptr("tool1,tool2")}
		res, err := Resolve("sh", &cli, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, []string{"tool1", "tool2"}, res.MountTools)
	})

	t.Run("Log settings resolution", func(t *testing.T) {
		cli := CLIOptions{Image: ptr("alpine"), LogLevel: ptr("debug")}
		res, err := Resolve("sh", &cli, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, "debug", res.LogLevel)
	})

	t.Run("resolveDevices from environment", func(t *testing.T) {
		mfs := &MockFileSystem{Env: map[string]string{"CDERUN_DEVICE": "/dev/a:/dev/b:rw"}}
		res, err := ResolveWithFS("node", &CLIOptions{Image: ptr("alpine")}, nil, nil, mfs)
		require.NoError(t, err)
		require.Len(t, res.Devices, 1)
		assert.Equal(t, "/dev/a", res.Devices[0].PathOnHost)
	})

	t.Run("resolveDevices invalid format in CDERUN_DEVICE", func(t *testing.T) {
		mfs := &MockFileSystem{Env: map[string]string{"CDERUN_DEVICE": ":"}}
		_, err := ResolveWithFS("node", &CLIOptions{Image: ptr("alpine")}, nil, nil, mfs)
		require.Error(t, err)
	})

	t.Run("resolveMounts from environment", func(t *testing.T) {
		mfs := &MockFileSystem{Env: map[string]string{"CDERUN_MOUNT": "source=/a,target=/b"}}
		res, err := ResolveWithFS("node", &CLIOptions{Image: ptr("alpine")}, nil, nil, mfs)
		require.NoError(t, err)
		require.Len(t, res.Mounts, 1)
		assert.Equal(t, "/a", res.Mounts[0].Source)
	})

	t.Run("resolveMounts invalid format in CDERUN_MOUNT", func(t *testing.T) {
		mfs := &MockFileSystem{Env: map[string]string{"CDERUN_MOUNT": "invalid"}}
		_, err := ResolveWithFS("node", &CLIOptions{Image: ptr("alpine")}, nil, nil, mfs)
		require.Error(t, err)
	})

	t.Run("resolveConfigPath from CDERUN_SOCKET_PATH", func(t *testing.T) {
		mfs := &MockFileSystem{Env: map[string]string{"CDERUN_SOCKET_PATH": "/run/my.sock"}}
		res, err := ResolveWithFS("node", &CLIOptions{Image: ptr("alpine")}, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, "/run/my.sock", res.SocketPath)
	})

	t.Run("resolveConfigPath from global config", func(t *testing.T) {
		global := &CDERunConfig{
			SocketPath: ConfigPath{Raw: "/global.sock"},
		}
		res, err := ResolveWithFS("node", &CLIOptions{Image: ptr("alpine")}, nil, global, &MockFileSystem{})
		require.NoError(t, err)
		assert.Equal(t, "/global.sock", res.SocketPath)
	})

	t.Run("Auto-detection exhaustive", func(t *testing.T) {
		// docker.sock exists
		mfs := &MockFileSystem{
			Dirs:  map[string]bool{"/var/run": true},
			Files: map[string][]byte{"/var/run/docker.sock": []byte("")},
		}
		res, err := ResolveWithFS("sh", &CLIOptions{Image: ptr("alpine")}, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, "docker", res.Runtime)
		assert.Equal(t, "/var/run/docker.sock", res.SocketPath)

		// podman.sock exists
		mfs = &MockFileSystem{
			Dirs:  map[string]bool{"/run/podman": true},
			Files: map[string][]byte{"/run/podman/podman.sock": []byte("")},
		}
		res, err = ResolveWithFS("sh", &CLIOptions{Image: ptr("alpine")}, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, "podman", res.Runtime)
		assert.Equal(t, "/run/podman/podman.sock", res.SocketPath)

		// specified podman but no socket, should use default podman socket
		res, err = ResolveWithFS("sh", &CLIOptions{Image: ptr("alpine"), Runtime: ptr("podman")}, nil, nil, &MockFileSystem{})
		require.NoError(t, err)
		assert.Equal(t, "podman", res.Runtime)
		assert.Equal(t, "/run/podman/podman.sock", res.SocketPath)

		// SocketPath explicit containerd
		res, err = ResolveWithFS("sh", &CLIOptions{Image: ptr("alpine"), SocketPath: ptr("/run/containerd/containerd.sock")}, nil, nil, &MockFileSystem{})
		require.NoError(t, err)
		assert.Equal(t, "containerd", res.Runtime)
		assert.Equal(t, "/run/containerd/containerd.sock", res.SocketPath)

		// specified docker but no socket
		res, err = ResolveWithFS("sh", &CLIOptions{Image: ptr("alpine"), Runtime: ptr("docker")}, nil, nil, &MockFileSystem{})
		require.NoError(t, err)
		assert.Equal(t, "docker", res.Runtime)
		assert.Equal(t, "/var/run/docker.sock", res.SocketPath)

		// containerd.sock exists
		mfs = &MockFileSystem{
			Dirs:  map[string]bool{"/run/containerd": true},
			Files: map[string][]byte{"/run/containerd/containerd.sock": []byte("")},
		}
		res, err = ResolveWithFS("sh", &CLIOptions{Image: ptr("alpine")}, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, "containerd", res.Runtime)
		assert.Equal(t, "/run/containerd/containerd.sock", res.SocketPath)

		// Priority: docker > containerd > podman
		mfs = &MockFileSystem{
			Dirs: map[string]bool{"/var/run": true, "/run/containerd": true, "/run/podman": true},
			Files: map[string][]byte{
				"/var/run/docker.sock":            []byte(""),
				"/run/containerd/containerd.sock": []byte(""),
				"/run/podman/podman.sock":         []byte(""),
			},
		}
		res, err = ResolveWithFS("sh", &CLIOptions{Image: ptr("alpine")}, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, "docker", res.Runtime)
		assert.Equal(t, "/var/run/docker.sock", res.SocketPath)

		// Priority: containerd > podman
		mfs = &MockFileSystem{
			Dirs: map[string]bool{"/run/containerd": true, "/run/podman": true},
			Files: map[string][]byte{
				"/run/containerd/containerd.sock": []byte(""),
				"/run/podman/podman.sock":         []byte(""),
			},
		}
		res, err = ResolveWithFS("sh", &CLIOptions{Image: ptr("alpine")}, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, "containerd", res.Runtime)
		assert.Equal(t, "/run/containerd/containerd.sock", res.SocketPath)

		// specified containerd but no socket
		res, err = ResolveWithFS("sh", &CLIOptions{Image: ptr("alpine"), Runtime: ptr("containerd")}, nil, nil, &MockFileSystem{})
		require.NoError(t, err)
		assert.Equal(t, "containerd", res.Runtime)
		assert.Equal(t, "/run/containerd/containerd.sock", res.SocketPath)

		// specified unknown runtime (e.g. from global)
		global := &CDERunConfig{Runtime: "unknown"}
		_, err = ResolveWithFS("sh", &CLIOptions{Image: ptr("alpine")}, nil, global, &MockFileSystem{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported runtime: \"unknown\"")
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
		_, err := ResolveWithFS("sh", &CLIOptions{}, nil, nil, &MockFileSystem{})
		var imgErr *ImageNotFoundError
		require.ErrorAs(t, err, &imgErr)
		assert.Equal(t, "sh", imgErr.Tool)

		// resolveDevices invalid format in CLI
		cliDev := CLIOptions{Image: ptr("alpine"), Devices: []string{":"}}
		_, err = ResolveWithFS("sh", &cliDev, nil, nil, &MockFileSystem{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid device config")

		// resolveMounts invalid format in CLI
		cliMnt := CLIOptions{Image: ptr("alpine"), Mounts: []string{"invalid"}}
		_, err = ResolveWithFS("sh", &cliMnt, nil, nil, &MockFileSystem{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid mount config")

		// invalid memory
		_, err = ResolveWithFS("sh", &CLIOptions{Image: ptr("alpine"), Memory: ptr("invalid")}, nil, nil, &MockFileSystem{})
		var cfgErr *InvalidConfigError
		require.ErrorAs(t, err, &cfgErr)
		assert.Equal(t, "memory", cfgErr.Field)

		// Expression resolver error
		mfsError := &customMockFS{homeDirErr: assert.AnError}
		_, err = ResolveWithFS("sh", &CLIOptions{Image: ptr("alpine")}, nil, nil, mfsError)
		require.Error(t, err)

		// Expression error
		mfsExpr := &MockFileSystem{WD: "/app"}
		cli := CLIOptions{Image: ptr("alpine"), Env: []string{"VAR={{file:missing}}"}}
		_, err = ResolveWithFS("sh", &cli, nil, nil, mfsExpr)
		require.Error(t, err)

		// Test resolveEnv with Tool getter
		tools := ToolsConfig{"node": ToolConfig{Env: []string{"TOOL=1"}}}
		res, err := ResolveWithFS("node", &CLIOptions{Image: ptr("alpine")}, tools, nil, &MockFileSystem{})
		require.NoError(t, err)
		assert.Contains(t, res.Env, "TOOL=1")

		// Test resolveDevices with Tool getter
		tools = ToolsConfig{"node": ToolConfig{Devices: []DeviceConfig{{Source: ConfigPath{Raw: "/dev/t"}}}}}
		res, err = ResolveWithFS("node", &CLIOptions{Image: ptr("alpine")}, tools, nil, &MockFileSystem{})
		require.NoError(t, err)
		require.Len(t, res.Devices, 1)
		assert.Equal(t, "/dev/t", res.Devices[0].PathOnHost)

		// Test resolveMounts with Tool getter
		tools = ToolsConfig{"node": ToolConfig{Mounts: []MountConfig{{Source: ConfigPath{Raw: "/s"}, Target: ConfigPath{Raw: "/t"}}}}}
		res, err = ResolveWithFS("node", &CLIOptions{Image: ptr("alpine")}, tools, nil, &MockFileSystem{})
		require.NoError(t, err)
		require.Len(t, res.Mounts, 1)
		assert.Equal(t, "/s", res.Mounts[0].Source)
	})

	t.Run("PullMaxRetries and PullBackoffBase errors", func(t *testing.T) {
		mfs := &MockFileSystem{}
		cli := CLIOptions{Image: ptr("alpine")}

		// PullMaxRetries <= 0
		cli.CderunPullMaxRetries = ptr(0)
		_, err := ResolveWithFS("node", &cli, nil, nil, mfs)
		var cfgErr *InvalidConfigError
		require.ErrorAs(t, err, &cfgErr)
		assert.Equal(t, "pull-max-retries", cfgErr.Field)
		assert.Contains(t, cfgErr.Error(), "must be greater than 0")

		// PullBackoffBase invalid
		cli.CderunPullMaxRetries = ptr(3)
		cli.CderunPullBackoffBase = ptr("invalid")
		_, err = ResolveWithFS("node", &cli, nil, nil, mfs)
		require.ErrorAs(t, err, &cfgErr)
		assert.Equal(t, "pull-backoff-base", cfgErr.Field)

		// PullBackoffBase non-positive
		cli.CderunPullBackoffBase = ptr("0s")
		_, err = ResolveWithFS("node", &cli, nil, nil, mfs)
		require.ErrorAs(t, err, &cfgErr)
		assert.Equal(t, "pull-backoff-base", cfgErr.Field)
		assert.Contains(t, cfgErr.Error(), "must be positive")
	})

	t.Run("Memory and Expression errors", func(t *testing.T) {
		mfs := &MockFileSystem{}
		cli := CLIOptions{Image: ptr("alpine")}

		// Invalid memory format
		cli.CderunMemory = ptr("invalid")
		_, err := ResolveWithFS("node", &cli, nil, nil, mfs)
		var cfgErr *InvalidConfigError
		require.ErrorAs(t, err, &cfgErr)
		assert.Equal(t, "memory", cfgErr.Field)

		// Expression error already present
		mfsExpr := &MockFileSystem{WD: "/app"}
		cliExpr := CLIOptions{Image: ptr("alpine"), Env: []string{"VAR={{file:missing}}"}, Memory: ptr("1G")}
		_, err = ResolveWithFS("node", &cliExpr, nil, nil, mfsExpr)
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
			Image:  ptr("alpine"),
			Mounts: []string{"source=/app/missing,target=/data,optional"},
		}
		res, err := ResolveWithFS("node", &cli, nil, nil, mfs)
		require.NoError(t, err)
		assert.Empty(t, res.Mounts)
	})

	t.Run("Keep optional mount when source exists", func(t *testing.T) {
		cli := CLIOptions{
			Image:  ptr("alpine"),
			Mounts: []string{"source=/app/exists,target=/data,optional"},
		}
		res, err := ResolveWithFS("node", &cli, nil, nil, mfs)
		require.NoError(t, err)
		require.Len(t, res.Mounts, 1)
		assert.Equal(t, "/app/exists", res.Mounts[0].Source)
		assert.True(t, res.Mounts[0].Optional)
	})

	t.Run("Non-optional mount remains even if source is missing (handled by runtime)", func(t *testing.T) {
		cli := CLIOptions{
			Image:  ptr("alpine"),
			Mounts: []string{"source=/app/missing,target=/data"},
		}
		res, err := ResolveWithFS("node", &cli, nil, nil, mfs)
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

func TestUnit_Resolver_ValidateImageRegistryMatch(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		cliImage    string
		configImage string
		wantErr     bool
	}{
		{"exact match", "node:20", "node:18", false},
		{"with registry match", "docker.io/library/node:20", "node:18", false},
		{"implicit library match", "node:20", "library/node:18", false},
		{"explicit host library match", "docker.io/node:20", "node:18", false},
		{"explicit host match", "docker.io/library/node:20", "docker.io/node:18", false},
		{"mismatch host", "my-reg.com/node:20", "node:18", true},
		{"mismatch repo", "library/python:3", "library/node:18", true},
		{"custom registry match", "my-reg.com/my-tool:v1", "my-reg.com/my-tool:latest", false},
		{"custom registry mismatch", "other-reg.com/my-tool:v1", "my-reg.com/my-tool:latest", true},
		{"ghcr match", "ghcr.io/user/repo:v1", "ghcr.io/user/repo:latest", false},
		{"localhost match", "localhost:5000/my-tool:v1", "localhost:5000/my-tool:latest", false},
		{"nested repo match", "my-reg.com/org/team/tool:v1", "my-reg.com/org/team/tool:latest", false},
		{"one side empty", "node:20", "", false},
		{"other side empty", "", "node:18", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateImageRegistryMatch(tt.cliImage, tt.configImage)
			if tt.wantErr {
				require.Error(t, err)
				var regErr *RegistryMismatchError
				require.ErrorAs(t, err, &regErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestUnit_Resolver_ResolveWithFS_RegistryMismatch(t *testing.T) {
	t.Parallel()
	mfs := &MockFileSystem{}
	tools := ToolsConfig{
		"node": ToolConfig{Image: "node:18-alpine"},
	}

	t.Run("mismatch from CLI", func(t *testing.T) {
		cli := &CLIOptions{
			Image: ptr("my-reg.com/node:20")}
		_, err := ResolveWithFS("node", cli, tools, nil, mfs)
		require.Error(t, err)
		var regErr *RegistryMismatchError
		require.ErrorAs(t, err, &regErr)
		assert.Equal(t, "docker.io/library/node", regErr.ExpectedRegistry)
		assert.Equal(t, "my-reg.com/node", regErr.ActualRegistry)
	})

	t.Run("mismatch from Env", func(t *testing.T) {
		mfsEnv := &MockFileSystem{
			Env: map[string]string{"CDERUN_IMAGE": "other.io/node:latest"},
		}
		_, err := ResolveWithFS("node", nil, tools, nil, mfsEnv)
		require.Error(t, err)
		var regErr *RegistryMismatchError
		assert.ErrorAs(t, err, &regErr)
	})

	t.Run("match from CLI", func(t *testing.T) {
		cli := &CLIOptions{
			Image: ptr("docker.io/library/node:20")}
		res, err := ResolveWithFS("node", cli, tools, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, "docker.io/library/node:20", res.Image)
	})

	t.Run("templated config match", func(t *testing.T) {
		mfsEnv := &MockFileSystem{
			Env: map[string]string{"REG": "my-reg.com"},
		}
		toolsTemplated := ToolsConfig{
			"node": ToolConfig{Image: "{{env:REG}}/node:18-alpine"},
		}
		cli := &CLIOptions{
			Image: ptr("my-reg.com/node:20")}
		res, err := ResolveWithFS("node", cli, toolsTemplated, nil, mfsEnv)
		require.NoError(t, err)
		assert.Equal(t, "my-reg.com/node:20", res.Image)
	})
}

func TestUnit_Resolver_Transitive_Env(t *testing.T) {
	t.Parallel()
	t.Run("mount-tools from env transitively enables mount-cderun and mount-socket", func(t *testing.T) {
		mfs := &MockFileSystem{
			Env: map[string]string{
				"CDERUN_MOUNT_TOOLS": "git,node",
			},
		}
		res, err := ResolveWithFS("sh", &CLIOptions{Image: ptr("alpine")}, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, []string{"git", "node"}, res.MountTools)
		assert.True(t, res.MountCderun)
		assert.True(t, res.MountSocket)
	})

	t.Run("mount-all-tools from env transitively enables mount-cderun and mount-socket", func(t *testing.T) {
		mfs := &MockFileSystem{
			Env: map[string]string{
				"CDERUN_MOUNT_ALL_TOOLS": "true",
			},
		}
		res, err := ResolveWithFS("sh", &CLIOptions{Image: ptr("alpine")}, nil, nil, mfs)
		require.NoError(t, err)
		assert.True(t, res.MountAllTools)
		assert.True(t, res.MountCderun)
		assert.True(t, res.MountSocket)
	})
}

func TestUnit_Resolver_Env_Strict_Missing(t *testing.T) {
	t.Parallel()
	mfs := &MockFileSystem{}
	cli := &CLIOptions{
		Image:     ptr("alpine"),
		Env:       []string{"MISSING_HOST_VAR"},
		StrictEnv: ptr(true),
	}
	_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required environment variable not found: \"MISSING_HOST_VAR\"")
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

func TestUnit_Config_Resolve_ExpressionErrorInDuration(t *testing.T) {
	mfs := &MockFileSystem{}
	cli := &CLIOptions{
		Image:       ptr("alpine"),
		HangTimeout: ptr("{{file:nonexistent}}"),
	}
	_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "file not found")
}
