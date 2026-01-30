package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolve(t *testing.T) {
	// Setup helper to create bool pointers
	ptr := func(b bool) *bool { return &b }

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

	t.Run("Volume parsing", func(t *testing.T) {
		cli := CLIOptions{}
		tools := ToolsConfig{
			"node": ToolConfig{
				Image:   "node:20",
				Volumes: []string{"/host/path:/container/path:ro", ".:/app"},
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
				Volumes: []string{
					`C:\host\path:/container/path`,
					`D:\data:/mnt:ro`,
					`Z:\shared folder:/app:rw`,
				},
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

	t.Run("Socket resolution from CDERUN_MOUNT_SOCKET", func(t *testing.T) {
		t.Setenv("CDERUN_MOUNT_SOCKET", "/custom/socket.sock")
		cli := CLIOptions{}
		res, err := Resolve("node", cli, ToolsConfig{"node": {Image: "node"}}, nil)
		require.NoError(t, err)
		assert.Equal(t, "/custom/socket.sock", res.Socket)
		assert.True(t, res.SocketSet)
	})

	t.Run("SocketSet is false for non-mountable paths", func(t *testing.T) {
		t.Setenv("CDERUN_MOUNT_SOCKET", "tcp://localhost:2375")
		res, err := Resolve("node", CLIOptions{}, ToolsConfig{"node": {Image: "node"}}, nil)
		require.NoError(t, err)
		assert.Equal(t, "tcp://localhost:2375", res.Socket)
		assert.False(t, res.SocketSet, "TCP socket should not be mountable")
	})

	t.Run("DOCKER_HOST does not affect SocketSet", func(t *testing.T) {
		t.Setenv("DOCKER_HOST", "/var/run/docker.sock")
		res, err := Resolve("node", CLIOptions{}, ToolsConfig{"node": {Image: "node"}}, nil)
		require.NoError(t, err)
		assert.False(t, res.SocketSet, "DOCKER_HOST should be ignored for SocketSet")
	})

	t.Run("P1 CderunMountSocket overrides CLI and Env", func(t *testing.T) {
		t.Setenv("CDERUN_MOUNT_SOCKET", "/env/socket")
		cli := CLIOptions{
			MountSocket:          "/cli/socket",
			MountSocketSet:       true,
			CderunMountSocket:    "/p1/socket",
			CderunMountSocketSet: true,
		}
		res, err := Resolve("node", cli, ToolsConfig{"node": {Image: "node"}}, nil)
		require.NoError(t, err)
		assert.Equal(t, "/p1/socket", res.Socket)
		assert.True(t, res.SocketSet)
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
}
