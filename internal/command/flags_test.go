package command

import (
	"context"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/runtime"
)

func TestUnit_Flags_DockerCompatibilityMapping(t *testing.T) {
	t.Run("basic and complex Docker flags", func(t *testing.T) {
		mockRuntime := &runtime.MockRuntime{}
		args := []string{"cderun",
			"--publish", "8080:80",
			"--publish-all",
			"--expose", "80",
			"--hostname", "myhost",
			"--dns", "8.8.8.8",
			"--add-host", "host:1.2.3.4",
			"--user", "1000:1000",
			"--privileged",
			"--cap-add", "SYS_ADMIN",
			"--cap-drop", "KILL",
			"--entrypoint", "/bin/sh",
			"--pull", "always",
			"--memory", "512m",
			"--cpus", "2.5",
			"--mount", "type=tmpfs,target=/tmp",
			"--device", "/dev/fuse",
			"--image", "alpine",
			"alpine", "ls", "-l"}
		err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.exitFunc = func(code int) {}
			o.isTerminal = func(fd int) bool { return true }
		})
		require.NoError(t, err, "raw args: %v", args)

		cfg := mockRuntime.GetCreatedConfig()
		require.NotNil(t, cfg)
		assert.Equal(t, []string{"ls", "-l"}, cfg.Command)
		assert.Equal(t, []string{"8080:80"}, cfg.Ports)
		assert.True(t, cfg.PublishAll)
		assert.Equal(t, []string{"80"}, cfg.Expose)
		assert.Equal(t, "myhost", cfg.Hostname)
		assert.Equal(t, []string{"8.8.8.8"}, cfg.DNS)
		assert.Equal(t, []string{"host:1.2.3.4"}, cfg.AddHosts)
		assert.Equal(t, "1000:1000", cfg.User)
		assert.True(t, cfg.Privileged)
		assert.Equal(t, []string{"SYS_ADMIN"}, cfg.CapAdd)
		assert.Equal(t, []string{"KILL"}, cfg.CapDrop)
		assert.Equal(t, []string{"/bin/sh"}, cfg.Entrypoint)
		assert.Equal(t, "always", cfg.Pull)
		assert.Equal(t, int64(512*1024*1024), cfg.Memory)
		assert.InDelta(t, 2.5, cfg.CPUs, 0.0001)
		require.Len(t, cfg.Mounts, 1)
		assert.Equal(t, "tmpfs", cfg.Mounts[0].Type)
		assert.Equal(t, "/tmp", cfg.Mounts[0].Target)
		require.Len(t, cfg.Devices, 1)
		assert.Equal(t, "/dev/fuse", cfg.Devices[0].PathOnHost)
	})

	t.Run("P1 flags override P2 for Docker-compatible features", func(t *testing.T) {
		mockRuntime := &runtime.MockRuntime{}
		args := []string{"cderun",
			"--publish", "8080:80",
			"--user", "initialUser",
			"--privileged=true",
			"--pull", "missing",
			"--memory", "1g",
			"--cpus", "1.0",
			"--image", "alpine",
			"alpine",
			"--cderun-publish", "9090:90",
			"--cderun-user", "override-user",
			"--cderun-privileged=false",
			"--cderun-pull", "always",
			"--cderun-memory", "2g",
			"--cderun-cpus", "2.0",
			"ls", "-l"}
		err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.exitFunc = func(code int) {}
			o.isTerminal = func(fd int) bool { return true }
		})
		require.NoError(t, err, "raw args: %v", args)

		cfg := mockRuntime.GetCreatedConfig()
		require.NotNil(t, cfg)
		assert.Equal(t, []string{"ls", "-l"}, cfg.Command)
		assert.Equal(t, []string{"9090:90"}, cfg.Ports)
		assert.Equal(t, "override-user", cfg.User)
		assert.False(t, cfg.Privileged)
		assert.Equal(t, "always", cfg.Pull)
		assert.Equal(t, int64(2*1024*1024*1024), cfg.Memory)
		assert.InDelta(t, 2.0, cfg.CPUs, 0.0001)
	})
}

func TestUnit_Command_PreprocessArgs_EdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("P1 flag before subcommand is an error", func(t *testing.T) {
		args := []string{"cderun", "--cderun-tty", "node", "--version"}
		cmd := newRootCmd(&rootOptions{})
		_, err := preprocessArgs(cmd, args)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cderun internal override flag \"--cderun-tty\" must be placed after the subcommand")
	})

	t.Run("Hoisting complex P1 flags with values", func(t *testing.T) {
		// cderun node app.js --cderun-image node:20-alpine --cderun-tty --cderun-env KEY=VAL
		args := []string{"cderun", "node", "app.js", "--cderun-image", "node:20-alpine", "--cderun-tty", "--cderun-env", "KEY=VAL"}
		cmd := newRootCmd(&rootOptions{})
		processed, err := preprocessArgs(cmd, args)
		require.NoError(t, err)

		// Expected: cderun --cderun-image node:20-alpine --cderun-tty --cderun-env KEY=VAL node app.js
		expected := []string{"cderun", "--cderun-image", "node:20-alpine", "--cderun-tty", "--cderun-env", "KEY=VAL", "node", "app.js"}
		assert.Equal(t, expected, processed)
	})

	t.Run("P1 flag with equals sign (no skip next)", func(t *testing.T) {
		args := []string{"cderun", "node", "--cderun-image=alpine", "ls"}
		cmd := newRootCmd(&rootOptions{})
		processed, err := preprocessArgs(cmd, args)
		require.NoError(t, err)

		assert.Equal(t, []string{"cderun", "--cderun-image=alpine", "node", "ls"}, processed)
	})

	t.Run("shorthand group with value", func(t *testing.T) {
		// actually -p 80:80 node. 'p' takes arg.
		args := []string{"cderun", "-p", "80:80", "node", "ls"}
		cmd := newRootCmd(&rootOptions{})
		processed, err := preprocessArgs(cmd, args)
		require.NoError(t, err)
		assert.Equal(t, []string{"cderun", "-p", "80:80", "node", "ls"}, processed)
	})
}
