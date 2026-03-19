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

func TestUnit_Config_Resolver_Priority_AllLayers(t *testing.T) {
	t.Parallel()
	t.Run("P1 Override takes priority over P2 CLI", func(t *testing.T) {
		t.Parallel()
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
		t.Parallel()
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
		t.Parallel()
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
		t.Parallel()
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

func TestUnit_Config_Resolver_AutoDetection(t *testing.T) {
	t.Parallel()
	t.Run("Infer runtime from socket path", func(t *testing.T) {
		t.Parallel()
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
		t.Parallel()
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

func TestUnit_Config_Resolver_Mounts_Exhaustive(t *testing.T) {
	t.Parallel()
	t.Run("Multiple mounts from environment", func(t *testing.T) {
		t.Parallel()
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

func TestUnit_Config_Resolver_Env_Exhaustive(t *testing.T) {
	t.Parallel()
	mfsShared := &MockFileSystem{
		Env: map[string]string{
			"HOST_VAR":   "host-val",
			"CDERUN_ENV": "ENV_VAR=env-val; HOST_VAR",
		},
	}
	t.Run("Env resolution from all layers", func(t *testing.T) {
		t.Parallel()
		tools := ToolsConfig{
			"node": ToolConfig{
				Image: "node",
				Env:   []string{"TOOL_VAR=tool-val"},
			},
		}
		res, err := ResolveWithFS("node", CLIOptions{}, tools, nil, mfsShared)
		require.NoError(t, err)
		assert.Contains(t, res.Env, "ENV_VAR=env-val")
		assert.Contains(t, res.Env, "HOST_VAR=host-val")
	})

	t.Run("Strict mode env validation", func(t *testing.T) {
		t.Parallel()
		cli := CLIOptions{
			Image: "alpine", ImageSet: true,
			Env:       []string{"NONEXISTENT"},
			StrictEnv: true, StrictEnvSet: true,
		}
		_, err := ResolveWithFS("node", cli, nil, nil, mfsShared)
		require.Error(t, err)
	})
}

func TestUnit_Config_Resolver_Devices_Advanced(t *testing.T) {
	t.Parallel()
	t.Run("Device resolution priority", func(t *testing.T) {
		t.Parallel()
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

func TestUnit_Config_Resolver_Misc_Exhaustive(t *testing.T) {
	t.Parallel()
	t.Run("HangTimeout parsing", func(t *testing.T) {
		t.Parallel()
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
		t.Parallel()
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

func TestUnit_Config_Resolver_Expressions_Exhaustive(t *testing.T) {
	t.Parallel()
	t.Run("Expression in environment variable override", func(t *testing.T) {
		t.Parallel()
		mfs := &MockFileSystem{
			WD: "/app",
			Files: map[string][]byte{
				"/app/.version": []byte("1.0"),
			},
			Env: map[string]string{"CDERUN_IMAGE": "node:{{file:.version}}"},
		}
		res, err := ResolveWithFS("node", CLIOptions{}, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, "node:1.0", res.Image)
	})
}

func TestUnit_Config_Resolver_Exhaustive_Additional(t *testing.T) {
	t.Parallel()
	t.Run("Diagnosis mode bypasses image check", func(t *testing.T) {
		t.Parallel()
		cli := CLIOptions{Diagnosis: true, DiagnosisSet: true}
		res, err := Resolve("unknown", cli, nil, nil)
		require.NoError(t, err)
		assert.True(t, res.Diagnosis)
	})

	t.Run("Transitive auto-enablement cderun to socket", func(t *testing.T) {
		t.Parallel()
		cli := CLIOptions{Image: "alpine", ImageSet: true, MountCderun: true, MountCderunSet: true}
		res, err := Resolve("sh", cli, nil, nil)
		require.NoError(t, err)
		assert.True(t, res.MountSocket)
	})

	t.Run("Float64 resolution", func(t *testing.T) {
		t.Parallel()
		cli := CLIOptions{Image: "alpine", ImageSet: true, CPUs: 1.5, CPUsSet: true}
		res, err := Resolve("node", cli, nil, nil)
		require.NoError(t, err)
		assert.InDelta(t, 1.5, res.CPUs, 0.0001)
	})

	t.Run("String slice with comma resolution", func(t *testing.T) {
		t.Parallel()
		mfs := &MockFileSystem{Env: map[string]string{"CDERUN_DNS": "8.8.8.8,1.1.1.1"}}
		res, err := ResolveWithFS("node", CLIOptions{Image: "alpine", ImageSet: true}, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, []string{"8.8.8.8", "1.1.1.1"}, res.DNS)
	})
}

func TestUnit_Config_Resolver_Exhaustive_Advanced(t *testing.T) {
	t.Parallel()
	t.Run("MountTools resolution", func(t *testing.T) {
		t.Parallel()
		cli := CLIOptions{Image: "alpine", ImageSet: true, MountTools: "tool1,tool2", MountToolsSet: true}
		res, err := Resolve("sh", cli, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, []string{"tool1", "tool2"}, res.MountTools)
	})

	t.Run("Log settings resolution", func(t *testing.T) {
		t.Parallel()
		cli := CLIOptions{Image: "alpine", ImageSet: true, LogLevel: "debug", LogLevelSet: true}
		res, err := Resolve("sh", cli, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, "debug", res.LogLevel)
	})
}
