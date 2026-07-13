package config

import (
	"time"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
			EnvKey:   "TEST_FLOAT",
			Fallback: ptr(1.0),
		}
		mfs := &MockFileSystem{Env: map[string]string{"TEST_FLOAT": "2.5"}}

		// Env
		res, err := resolveFloat64Opt(def, false, 0, false, 0, "sub", nil, nil, mfs)
		require.NoError(t, err)
		assert.InDelta(t, 2.5, res, 1e-9)

		// Fallback
		mfs.Env = nil
		res, err = resolveFloat64Opt(def, false, 0, false, 0, "sub", nil, nil, mfs)
		require.NoError(t, err)
		assert.InDelta(t, 1.0, res, 1e-9)

		// Invalid env
		mfs.Env = map[string]string{"TEST_FLOAT": "invalid"}
		_, err = resolveFloat64Opt(def, false, 0, false, 0, "sub", nil, nil, mfs)
		assert.Error(t, err)

		// Tool getter
		mfs.Env = nil
		f2 := 2.0
		def.ToolGetter = func(tc ToolConfig) *float64 { return &f2 }
		res, err = resolveFloat64Opt(def, false, 0, false, 0, "node", ToolsConfig{"node": ToolConfig{}}, nil, mfs)
		require.NoError(t, err)
		assert.InDelta(t, 2.0, res, 1e-9)

		// Global getter
		def.ToolGetter = nil
		f3 := 3.0
		def.GlobalGetter = func(c CDERunConfig) *float64 { return &f3 }
		res, err = resolveFloat64Opt(def, false, 0, false, 0, "node", nil, &CDERunConfig{}, mfs)
		require.NoError(t, err)
		assert.InDelta(t, 3.0, res, 1e-9)

		// P2 CLI
		res, err = resolveFloat64Opt(def, false, 0, true, 4.0, "node", nil, nil, mfs)
		require.NoError(t, err)
		assert.InDelta(t, 4.0, res, 1e-9)

		// P1 Override
		res, err = resolveFloat64Opt(def, true, 5.0, false, 0, "node", nil, nil, mfs)
		require.NoError(t, err)
		assert.InDelta(t, 5.0, res, 1e-9)
	})

	t.Run("resolveIntOpt", func(t *testing.T) {
		def := OptionDef[*int]{
			EnvKey:   "TEST_INT",
			Fallback: ptr(10),
		}
		mfs := &MockFileSystem{Env: map[string]string{"TEST_INT": "20"}}

		// Env
		res, err := resolveIntOpt(def, false, 0, false, 0, "sub", nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, 20, res)

		// Fallback
		mfs.Env = nil
		res, err = resolveIntOpt(def, false, 0, false, 0, "sub", nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, 10, res)

		// Invalid env
		mfs.Env = map[string]string{"TEST_INT": "invalid"}
		_, err = resolveIntOpt(def, false, 0, false, 0, "sub", nil, nil, mfs)
		assert.Error(t, err)

		// Tool getter
		mfs.Env = nil
		i2 := 30
		def.ToolGetter = func(tc ToolConfig) *int { return &i2 }
		res, err = resolveIntOpt(def, false, 0, false, 0, "node", ToolsConfig{"node": ToolConfig{}}, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, 30, res)

		// Global getter
		def.ToolGetter = nil
		i3 := 40
		def.GlobalGetter = func(c CDERunConfig) *int { return &i3 }
		res, err = resolveIntOpt(def, false, 0, false, 0, "node", nil, &CDERunConfig{}, mfs)
		require.NoError(t, err)
		assert.Equal(t, 40, res)

		// P2 CLI
		res, err = resolveIntOpt(def, false, 0, true, 50, "node", nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, 50, res)

		// P1 Override
		res, err = resolveIntOpt(def, true, 60, false, 0, "node", nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, 60, res)
	})

	t.Run("resolveEnvValues contains plaintext", func(t *testing.T) {
		mfs := &MockFileSystem{Env: map[string]string{"MY_PASSWORD": "secret"}}
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)
		res, err := resolveEnvValues([]string{"MY_PASSWORD"}, nil, false, r, mfs)
		require.NoError(t, err)
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
		res, err := Resolve("node", &cli, nil, nil)
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
	mfs := &MockFileSystem{}
	t.Run("Infer runtime from socket path", func(t *testing.T) {
		cli := CLIOptions{
			Image:         "alpine",
			ImageSet:      true,
			SocketPath:    "/run/podman/podman.sock",
			SocketPathSet: true,
		}
		res, err := ResolveWithFS("node", &cli, nil, nil, mfs)
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
		res, err := ResolveWithFS("node", &cli, nil, nil, mfs)
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
		res, err := ResolveWithFS("node", &CLIOptions{Image: "alpine", ImageSet: true}, nil, nil, mfs)
		require.NoError(t, err)
		require.Len(t, res.Mounts, 2)
		assert.Equal(t, "/a", res.Mounts[0].Source)
		assert.True(t, res.Mounts[1].ReadOnly)
	})

	t.Run("Mounts from environment with empty segments and extra spaces", func(t *testing.T) {
		mfs := &MockFileSystem{
			Env: map[string]string{"CDERUN_MOUNT": "  source=/a,target=/b  ; ;   source=/c,target=/d   "},
		}
		res, err := ResolveWithFS("node", &CLIOptions{Image: "alpine", ImageSet: true}, nil, nil, mfs)
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
			Image: "alpine", ImageSet: true,
			Env:       []string{"NONEXISTENT"},
			StrictEnv: true, StrictEnvSet: true,
		}
		_, err := ResolveWithFS("node", &cli, nil, nil, mfs)
		require.Error(t, err)
	})

	t.Run("Env from environment with empty segments and extra spaces", func(t *testing.T) {
		mfs2 := &MockFileSystem{
			Env: map[string]string{"CDERUN_ENV": "  A=1  ; ;   B=2   "},
		}
		res, err := ResolveWithFS("node", &CLIOptions{Image: "alpine", ImageSet: true}, nil, nil, mfs2)
		require.NoError(t, err)
		assert.Contains(t, res.Env, "A=1")
		assert.Contains(t, res.Env, "B=2")
		assert.Len(t, res.Env, 2)
	})
}

func TestUnit_Resolver_Devices_Advanced(t *testing.T) {
	t.Parallel()
	mfs := &MockFileSystem{}
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
		res, err := ResolveWithFS("node", &CLIOptions{}, tools, global, mfs)
		require.NoError(t, err)
		require.Len(t, res.Devices, 1)
		assert.Equal(t, "/dev/tool", res.Devices[0].PathOnHost)
	})
}

func TestUnit_Resolver_Misc_Exhaustive(t *testing.T) {
	t.Parallel()
	mfs := &MockFileSystem{}
	t.Run("HangTimeout parsing", func(t *testing.T) {
		cli := CLIOptions{
			Image:          "alpine",
			ImageSet:       true,
			HangTimeout:    "5s",
			HangTimeoutSet: true,
		}
		res, err := ResolveWithFS("node", &cli, nil, nil, mfs)
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
		res, err := ResolveWithFS("node", &cli, nil, nil, mfs)
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
	mfs := &MockFileSystem{}
	t.Run("Diagnosis mode bypasses image check", func(t *testing.T) {
		cli := CLIOptions{Diagnosis: true, DiagnosisSet: true}
		res, err := ResolveWithFS("unknown", &cli, nil, nil, mfs)
		require.NoError(t, err)
		assert.True(t, res.Diagnosis)
	})

	t.Run("Transitive auto-enablement cderun to socket", func(t *testing.T) {
		cli := CLIOptions{Image: "alpine", ImageSet: true, MountCderun: true, MountCderunSet: true}
		res, err := ResolveWithFS("sh", &cli, nil, nil, mfs)
		require.NoError(t, err)
		assert.True(t, res.MountSocket)
	})

	t.Run("Float64 resolution", func(t *testing.T) {
		cli := CLIOptions{Image: "alpine", ImageSet: true, CPUs: 1.5, CPUsSet: true}
		res, err := ResolveWithFS("node", &cli, nil, nil, mfs)
		require.NoError(t, err)
		assert.InDelta(t, 1.5, res.CPUs, 0.0001)
	})

	t.Run("String slice with comma resolution", func(t *testing.T) {
		mfs := &MockFileSystem{Env: map[string]string{"CDERUN_DNS": "8.8.8.8,1.1.1.1"}}
		res, err := ResolveWithFS("node", &CLIOptions{Image: "alpine", ImageSet: true}, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, []string{"8.8.8.8", "1.1.1.1"}, res.DNS)
	})
}

func TestUnit_Resolver_Transitive_Exhaustive(t *testing.T) {
	t.Parallel()
	mfs := &MockFileSystem{}

	t.Run("mount-socket enabled via mount-cderun", func(t *testing.T) {
		cli := &CLIOptions{
			Image:             "alpine",
			ImageSet:          true,
			MountCderun:       true,
			MountCderunSet:    true,
			CderunMountSocket: false,
		}
		res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)
		assert.True(t, res.MountCderun)
		assert.True(t, res.MountSocket)
	})

	t.Run("mount-socket disabled explicitly in P1 overrides mount-cderun", func(t *testing.T) {
		cli := &CLIOptions{
			Image:                "alpine",
			ImageSet:             true,
			MountCderun:          true,
			MountCderunSet:       true,
			CderunMountSocket:    false,
			CderunMountSocketSet: true,
		}
		res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)
		assert.True(t, res.MountCderun)
		assert.False(t, res.MountSocket)
	})

	t.Run("mount-socket enabled via mount-all-tools", func(t *testing.T) {
		cli := &CLIOptions{
			Image:            "alpine",
			ImageSet:         true,
			MountAllTools:    true,
			MountAllToolsSet: true,
		}
		res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
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
		Image:        "alpine",
		ImageSet:     true,
		Env:          []string{"MISSING_HOST_VAR"},
		StrictEnv:    true,
		StrictEnvSet: true,
	}
	_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required environment variable not found: \"MISSING_HOST_VAR\"")
}

func TestUnit_Config_Resolve_InvalidEnvErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		env     map[string]string
		wantErr string
	}{
		{
			name:    "Invalid bool env",
			env:     map[string]string{"CDERUN_TTY": "not-a-bool"},
			wantErr: "invalid CDERUN_TTY value \"not-a-bool\"",
		},
		{
			name:    "Invalid int env",
			env:     map[string]string{"CDERUN_PULL_MAX_RETRIES": "abc"},
			wantErr: "invalid CDERUN_PULL_MAX_RETRIES value \"abc\"",
		},
		{
			name:    "Invalid float env",
			env:     map[string]string{"CDERUN_CPUS": "two"},
			wantErr: "invalid CDERUN_CPUS value \"two\"",
		},
		{
			name:    "Invalid duration env (hang-timeout)",
			env:     map[string]string{"CDERUN_HANG_TIMEOUT": "invalid"},
			wantErr: "invalid hang-timeout value \"invalid\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mfs := &MockFileSystem{Env: tt.env}
			cli := &CLIOptions{
				Image:    "alpine",
				ImageSet: true,
			}
			_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)

			var ice *InvalidConfigError
			assert.ErrorAs(t, err, &ice)
		})
	}
}

func TestUnit_Config_Resolve_ExpressionErrorInDuration(t *testing.T) {
	t.Parallel()
	mfs := &MockFileSystem{}
	cli := &CLIOptions{
		Image:    "alpine",
		ImageSet: true,
		HangTimeout: "{{file:nonexistent}}",
		HangTimeoutSet: true,
	}

	_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "file not found: \"nonexistent\"")
	assert.NotContains(t, err.Error(), "invalid hang-timeout value")
}
