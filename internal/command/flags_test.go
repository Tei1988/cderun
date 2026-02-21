package command

import (
	"context"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/runtime"
)

func TestUnit_Command_Flags_DockerCompatible(t *testing.T) {
	t.Run("P2 flags for Docker-compatible features", func(t *testing.T) {
		mockRuntime := &runtime.MockRuntime{}
		// We use ExecuteContextWithOptions directly to inject mockRuntime
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun",
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
			"sh", "ls", "-l",
		}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.exitFunc = func(code int) {}
			o.isTerminal = func(fd int) bool { return true }
		})
		require.NoError(t, err)

		require.NotNil(t, mockRuntime.GetCreatedConfig())
		assert.Equal(t, []string{"ls", "-l"}, mockRuntime.GetCreatedConfig().Command)
		assert.Equal(t, []string{"8080:80"}, mockRuntime.GetCreatedConfig().Ports)
		assert.True(t, mockRuntime.GetCreatedConfig().PublishAll)
		assert.Equal(t, []string{"80"}, mockRuntime.GetCreatedConfig().Expose)
		assert.Equal(t, "myhost", mockRuntime.GetCreatedConfig().Hostname)
		assert.Equal(t, []string{"8.8.8.8"}, mockRuntime.GetCreatedConfig().DNS)
		assert.Equal(t, []string{"host:1.2.3.4"}, mockRuntime.GetCreatedConfig().AddHosts)
		assert.Equal(t, "1000:1000", mockRuntime.GetCreatedConfig().User)
		assert.True(t, mockRuntime.GetCreatedConfig().Privileged)
		assert.Equal(t, []string{"SYS_ADMIN"}, mockRuntime.GetCreatedConfig().CapAdd)
		assert.Equal(t, []string{"KILL"}, mockRuntime.GetCreatedConfig().CapDrop)
		assert.Equal(t, []string{"/bin/sh"}, mockRuntime.GetCreatedConfig().Entrypoint)
		assert.Equal(t, "always", mockRuntime.GetCreatedConfig().Pull)
		assert.Equal(t, int64(512*1024*1024), mockRuntime.GetCreatedConfig().Memory)
		assert.InDelta(t, 2.5, mockRuntime.GetCreatedConfig().CPUs, 0.0001)
		require.Len(t, mockRuntime.GetCreatedConfig().Mounts, 1)
		assert.Equal(t, "tmpfs", mockRuntime.GetCreatedConfig().Mounts[0].Type)
		assert.Equal(t, "/tmp", mockRuntime.GetCreatedConfig().Mounts[0].Target)
		require.Len(t, mockRuntime.GetCreatedConfig().Devices, 1)
		assert.Equal(t, "/dev/fuse", mockRuntime.GetCreatedConfig().Devices[0].PathOnHost)
	})

	t.Run("P1 flags override P2 for Docker-compatible features", func(t *testing.T) {
		mockRuntime := &runtime.MockRuntime{}
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun",
			"--publish", "8080:80",
			"--user", "initialUser",
			"--privileged=true",
			"--pull", "missing",
			"--memory", "1g",
			"--cpus", "1.0",
			"--image", "alpine",
			"sh", "ls", "-l",
			"--cderun-publish=9090:90",
			"--cderun-user=overrideUser",
			"--cderun-privileged=false",
			"--cderun-pull=always",
			"--cderun-memory=2g",
			"--cderun-cpus=2.0",
		}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.exitFunc = func(code int) {}
			o.isTerminal = func(fd int) bool { return true }
		})
		require.NoError(t, err)

		require.NotNil(t, mockRuntime.GetCreatedConfig())
		assert.Equal(t, []string{"ls", "-l"}, mockRuntime.GetCreatedConfig().Command)
		assert.Equal(t, []string{"9090:90"}, mockRuntime.GetCreatedConfig().Ports)
		assert.Equal(t, "overrideUser", mockRuntime.GetCreatedConfig().User)
		assert.False(t, mockRuntime.GetCreatedConfig().Privileged)
		assert.Equal(t, "always", mockRuntime.GetCreatedConfig().Pull)
		assert.Equal(t, int64(2*1024*1024*1024), mockRuntime.GetCreatedConfig().Memory)
		assert.InDelta(t, 2.0, mockRuntime.GetCreatedConfig().CPUs, 0.0001)
	})

	t.Run("Invalid pull policy returns error", func(t *testing.T) {
		_, err := executeCommand("--pull", "invalid", "--image", "alpine", "sh")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid pull policy \"invalid\"")
		assert.Contains(t, err.Error(), "allowed values are \"always\", \"missing\", or \"never\"")
	})

	t.Run("Invalid pull policy in P1 returns error", func(t *testing.T) {
		_, err := executeCommand("--image", "alpine", "sh", "--cderun-pull=invalid")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid pull policy \"invalid\"")
	})
}
