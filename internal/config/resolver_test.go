package config

import (
	"testing"

	"dario.cat/mergo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolve(t *testing.T) {
	// Setup helper to create bool pointers
	ptr := func(b bool) *bool { return &b }
	cp := func(s string) ConfigPath { return ConfigPath{Raw: s} }
	cps := func(ss ...string) []ConfigPath {
		var res []ConfigPath
		for _, s := range ss {
			res = append(res, ConfigPath{Raw: s})
		}
		return res
	}

	t.Run("P2 CLI takes priority over P4 Tool and P5 Global", func(t *testing.T) {
		cli := CLIOptions{
			TTY:    true,
			TTYSet: true,
		}
		tools := ToolsConfig{
			"node": ToolConfig{
				Image: "node:20",
				TTY:   ptr(false),
			},
		}
		global := &CDERunConfig{
			Defaults: ConfigDefaults{
				TTY: ptr(false),
			},
		}

		res, err := Resolve("node", cli, tools, global)
		require.NoError(t, err)
		assert.True(t, res.TTY)
		assert.Equal(t, "node:20", res.Image)
	})

	t.Run("P1 Override takes priority over P2 CLI", func(t *testing.T) {
		cli := CLIOptions{
			TTY:          true,
			TTYSet:       true,
			CderunTTY:    false,
			CderunTTYSet: true,
		}
		tools := ToolsConfig{
			"node": ToolConfig{
				Image: "node:20",
			},
		}

		res, err := Resolve("node", cli, tools, nil)
		require.NoError(t, err)
		assert.False(t, res.TTY)
	})

	t.Run("P3 Env Var priority", func(t *testing.T) {
		t.Setenv("CDERUN_TTY", "true")
		t.Setenv("CDERUN_IMAGE", "env-image:latest")
		cli := CLIOptions{}
		tools := ToolsConfig{
			"node": ToolConfig{
				Image: "node:20",
				TTY:   ptr(false),
			},
		}

		res, err := Resolve("node", cli, tools, nil)
		require.NoError(t, err)
		assert.True(t, res.TTY)
		// Image should respect P3 environment variable
		assert.Equal(t, "env-image:latest", res.Image)
	})

	t.Run("Image resolution from ToolConfig", func(t *testing.T) {
		cli := CLIOptions{}
		tools := ToolsConfig{
			"python": ToolConfig{
				Image: "python:3.11",
			},
		}

		res, err := Resolve("python", cli, tools, nil)
		require.NoError(t, err)
		assert.Equal(t, "python:3.11", res.Image)
	})

	t.Run("Error if no image found", func(t *testing.T) {
		cli := CLIOptions{}
		_, err := Resolve("unknown", cli, nil, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no image mapping found")
	})

	t.Run("Allow empty subcommand for resolution", func(t *testing.T) {
		cli := CLIOptions{}
		res, err := Resolve("", cli, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, "", res.Image)
	})

	t.Run("Volume parsing", func(t *testing.T) {
		cli := CLIOptions{}
		tools := ToolsConfig{
			"node": ToolConfig{
				Image:   "node:20",
				Volumes: cps("/host/path:/container/path:ro", ".:/app"),
			},
		}

		res, err := Resolve("node", cli, tools, nil)
		require.NoError(t, err)
		assert.Len(t, res.Volumes, 2)
		assert.Equal(t, "/host/path", res.Volumes[0].HostPath)
		assert.Equal(t, "/container/path", res.Volumes[0].ContainerPath)
		assert.True(t, res.Volumes[0].ReadOnly)
		assert.Equal(t, ".", res.Volumes[1].HostPath)
		assert.Equal(t, "/app", res.Volumes[1].ContainerPath)
		assert.False(t, res.Volumes[1].ReadOnly)
	})

	t.Run("Windows-style volume parsing", func(t *testing.T) {
		cli := CLIOptions{}
		tools := ToolsConfig{
			"node": ToolConfig{
				Image: "node:20",
				Volumes: cps(
					`C:\host\path:/container/path`,
					`D:\data:/mnt:ro`,
					`Z:\shared folder:/app:rw`,
				),
			},
		}

		res, err := Resolve("node", cli, tools, nil)
		require.NoError(t, err)
		assert.Len(t, res.Volumes, 3)

		assert.Equal(t, `C:\host\path`, res.Volumes[0].HostPath)
		assert.Equal(t, `/container/path`, res.Volumes[0].ContainerPath)
		assert.False(t, res.Volumes[0].ReadOnly)

		assert.Equal(t, `D:\data`, res.Volumes[1].HostPath)
		assert.Equal(t, `/mnt`, res.Volumes[1].ContainerPath)
		assert.True(t, res.Volumes[1].ReadOnly)

		assert.Equal(t, `Z:\shared folder`, res.Volumes[2].HostPath)
		assert.Equal(t, `/app`, res.Volumes[2].ContainerPath)
		assert.False(t, res.Volumes[2].ReadOnly)
	})

	t.Run("Workdir resolution", func(t *testing.T) {
		cli := CLIOptions{
			Workdir:    "/cli/workdir",
			WorkdirSet: true,
		}
		tools := ToolsConfig{
			"node": ToolConfig{
				Image:   "node:20",
				Workdir: "/tool/workdir",
			},
		}

		res, err := Resolve("node", cli, tools, nil)
		require.NoError(t, err)
		assert.Equal(t, "/cli/workdir", res.Workdir)

		cli.WorkdirSet = false
		res, err = Resolve("node", cli, tools, nil)
		require.NoError(t, err)
		assert.Equal(t, "/tool/workdir", res.Workdir)
	})

	t.Run("SocketPath resolution from CDERUN_SOCKET_PATH", func(t *testing.T) {
		t.Setenv("CDERUN_SOCKET_PATH", "/custom/socket.sock")
		cli := CLIOptions{}
		res, err := Resolve("node", cli, ToolsConfig{"node": {Image: "node"}}, nil)
		require.NoError(t, err)
		assert.Equal(t, "/custom/socket.sock", res.SocketPath)
	})

	t.Run("MountSocket resolution", func(t *testing.T) {
		t.Setenv("CDERUN_MOUNT_SOCKET", "true")
		res, err := Resolve("node", CLIOptions{}, ToolsConfig{"node": {Image: "node"}}, nil)
		require.NoError(t, err)
		assert.True(t, res.MountSocket)
	})

	t.Run("DOCKER_HOST is ignored", func(t *testing.T) {
		t.Setenv("DOCKER_HOST", "/var/run/docker.sock")
		res, err := Resolve("node", CLIOptions{}, ToolsConfig{"node": {Image: "node"}}, nil)
		require.NoError(t, err)
		// It will fallback to default auto-detection or empty, but should NOT be /var/run/docker.sock from DOCKER_HOST
		assert.False(t, res.MountSocket)
	})

	t.Run("P1 CderunSocketPath overrides CLI and Env", func(t *testing.T) {
		t.Setenv("CDERUN_SOCKET_PATH", "/env/socket")
		cli := CLIOptions{
			SocketPath:          "/cli/socket",
			SocketPathSet:       true,
			CderunSocketPath:    "/p1/socket",
			CderunSocketPathSet: true,
		}
		res, err := Resolve("node", cli, ToolsConfig{"node": {Image: "node"}}, nil)
		require.NoError(t, err)
		assert.Equal(t, "/p1/socket", res.SocketPath)
	})

	t.Run("Runtime auto-detection from SocketPath", func(t *testing.T) {
		cli := CLIOptions{
			SocketPath:    "/run/user/1000/podman/podman.sock",
			SocketPathSet: true,
		}
		res, err := Resolve("node", cli, ToolsConfig{"node": {Image: "node"}}, nil)
		require.NoError(t, err)
		assert.Equal(t, "podman", res.Runtime)
		assert.Equal(t, "/run/user/1000/podman/podman.sock", res.SocketPath)

		cli = CLIOptions{
			SocketPath:    "/var/run/docker.sock",
			SocketPathSet: true,
		}
		res, err = Resolve("node", cli, ToolsConfig{"node": {Image: "node"}}, nil)
		require.NoError(t, err)
		assert.Equal(t, "docker", res.Runtime)
	})

	t.Run("Default socket for specified runtime", func(t *testing.T) {
		cli := CLIOptions{
			Runtime:    "podman",
			RuntimeSet: true,
		}
		res, err := Resolve("node", cli, ToolsConfig{"node": {Image: "node"}}, nil)
		require.NoError(t, err)
		assert.Equal(t, "podman", res.Runtime)
		assert.Equal(t, "/run/podman/podman.sock", res.SocketPath)

		cli = CLIOptions{
			Runtime:    "docker",
			RuntimeSet: true,
		}
		res, err = Resolve("node", cli, ToolsConfig{"node": {Image: "node"}}, nil)
		require.NoError(t, err)
		assert.Equal(t, "docker", res.Runtime)
		assert.Equal(t, "/var/run/docker.sock", res.SocketPath)
	})

	t.Run("MountCderun resolution", func(t *testing.T) {
		cli := CLIOptions{
			MountCderun:    true,
			MountCderunSet: true,
		}
		tools := ToolsConfig{
			"node": ToolConfig{
				Image:       "node:20",
				MountCderun: ptr(false),
			},
		}

		res, err := Resolve("node", cli, tools, nil)
		require.NoError(t, err)
		assert.True(t, res.MountCderun)

		cli.MountCderunSet = false
		res, err = Resolve("node", cli, tools, nil)
		require.NoError(t, err)
		assert.False(t, res.MountCderun)
	})

	t.Run("Logging resolution", func(t *testing.T) {
		cli := CLIOptions{
			LogLevel:     "debug",
			LogLevelSet:  true,
			LogFormat:    "json",
			LogFormatSet: true,
			Verbose:      0,
		}
		global := &CDERunConfig{
			Logging: LoggingConfig{
				Level: "info",
				File:  cp("/var/log/cderun.log"),
			},
		}
		tools := ToolsConfig{
			"node": ToolConfig{Image: "node"},
		}

		res, err := Resolve("node", cli, tools, global)
		require.NoError(t, err)
		assert.Equal(t, "debug", res.LogLevel)
		assert.Equal(t, "json", res.LogFormat)
		assert.Equal(t, "/var/log/cderun.log", res.LogFile)

		// Verbose override
		cli.Verbose = 2
		res, err = Resolve("node", cli, tools, global)
		require.NoError(t, err)
		assert.Equal(t, "debug", res.LogLevel)

		cli.Verbose = 3
		res, err = Resolve("node", cli, tools, global)
		require.NoError(t, err)
		assert.Equal(t, "trace", res.LogLevel)

		// P1 Override
		cli.CderunLogLevel = "error"
		cli.CderunLogLevelSet = true
		res, err = Resolve("node", cli, tools, global)
		require.NoError(t, err)
		assert.Equal(t, "error", res.LogLevel)
	})

	t.Run("Strict environment variable resolution", func(t *testing.T) {
		tools := ToolsConfig{
			"node": ToolConfig{
				Image: "node:20",
				Env:   []string{"EXISTING_VAR", "MISSING_VAR"},
			},
		}

		t.Setenv("EXISTING_VAR", "value")

		// Default: missing var is empty string
		res, err := Resolve("node", CLIOptions{}, tools, nil)
		require.NoError(t, err)
		assert.Contains(t, res.Env, "EXISTING_VAR=value")
		assert.Contains(t, res.Env, "MISSING_VAR=")

		// Strict mode from tool config
		toolsStrict := ToolsConfig{
			"node": ToolConfig{
				Image:     "node:20",
				Env:       []string{"MISSING_VAR"},
				StrictEnv: ptr(true),
			},
		}
		_, err = Resolve("node", CLIOptions{}, toolsStrict, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "required environment variable not found: MISSING_VAR")

		// Strict mode from global config
		globalStrict := &CDERunConfig{
			Defaults: ConfigDefaults{
				StrictEnv: ptr(true),
			},
		}
		_, err = Resolve("node", CLIOptions{}, tools, globalStrict)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "required environment variable not found: MISSING_VAR")

		// Strict mode from environment variable
		t.Setenv("CDERUN_STRICT_ENV", "true")
		_, err = Resolve("node", CLIOptions{}, tools, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "required environment variable not found: MISSING_VAR")
	})

	t.Run("Deferred path resolution with multiple layers", func(t *testing.T) {
		global := &CDERunConfig{
			SocketPath: ConfigPath{Raw: "./global.sock", BaseDir: "/etc/cderun"},
		}
		tools := ToolsConfig{
			"node": ToolConfig{
				Image:   "node",
				Volumes: []ConfigPath{{Raw: "./project-data:/data", BaseDir: "/home/user/project"}},
			},
		}

		res, err := Resolve("node", CLIOptions{}, tools, global)
		require.NoError(t, err)

		assert.Equal(t, "/etc/cderun/global.sock", res.SocketPath)
		require.Len(t, res.Volumes, 1)
		assert.Equal(t, "/home/user/project/project-data", res.Volumes[0].HostPath)
	})

	t.Run("Priority logic when tool value matches fallback", func(t *testing.T) {
		// Global sets network to host
		global := &CDERunConfig{
			Defaults: ConfigDefaults{Network: "host"},
		}
		// Tool sets network to bridge (which is the default fallback)
		tools := ToolsConfig{
			"node": ToolConfig{
				Image:   "node",
				Network: "bridge",
			},
		}

		res, err := Resolve("node", CLIOptions{}, tools, global)
		require.NoError(t, err)
		assert.Equal(t, "bridge", res.Network, "Tool config should take priority even if it matches fallback")
	})

	t.Run("Merging does not overwrite with empty Raw paths", func(t *testing.T) {
		// Low priority layer has a path
		merged := CDERunConfig{
			SocketPath: ConfigPath{Raw: "./low.sock", BaseDir: "/low"},
		}
		// High priority layer does NOT have the path, but has a BaseDir (assigned by SetBaseDir)
		highLayer := CDERunConfig{}
		highLayer.SetBaseDir("/high")

		err := mergo.Merge(&merged, &highLayer, mergo.WithOverride)
		require.NoError(t, err)

		assert.Equal(t, "./low.sock", merged.SocketPath.Raw)
		assert.Equal(t, "/low", merged.SocketPath.BaseDir)
	})
}
