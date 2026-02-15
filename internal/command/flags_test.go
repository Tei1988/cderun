package command

import (
	"testing"

	"cderun/internal/runtime"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Command_Flags_DockerCompatible(t *testing.T) {
	mockRuntime := &runtime.MockRuntime{}
	setupTestOptions(t)
	setupMockRuntime(t, mockRuntime)

	t.Run("P2 flags for Docker-compatible features", func(t *testing.T) {
		mockRuntime.CreatedConfig = nil

		_, _, err := executeCommand(
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
		)
		require.NoError(t, err)

		require.NotNil(t, mockRuntime.CreatedConfig)
		assert.Equal(t, []string{"ls", "-l"}, mockRuntime.CreatedConfig.Command)
		assert.Equal(t, []string{"8080:80"}, mockRuntime.CreatedConfig.Ports)
		assert.True(t, mockRuntime.CreatedConfig.PublishAll)
		assert.Equal(t, []string{"80"}, mockRuntime.CreatedConfig.Expose)
		assert.Equal(t, "myhost", mockRuntime.CreatedConfig.Hostname)
		assert.Equal(t, []string{"8.8.8.8"}, mockRuntime.CreatedConfig.DNS)
		assert.Equal(t, []string{"host:1.2.3.4"}, mockRuntime.CreatedConfig.AddHosts)
		assert.Equal(t, "1000:1000", mockRuntime.CreatedConfig.User)
		assert.True(t, mockRuntime.CreatedConfig.Privileged)
		assert.Equal(t, []string{"SYS_ADMIN"}, mockRuntime.CreatedConfig.CapAdd)
		assert.Equal(t, []string{"KILL"}, mockRuntime.CreatedConfig.CapDrop)
		assert.Equal(t, []string{"/bin/sh"}, mockRuntime.CreatedConfig.Entrypoint)
		assert.Equal(t, "always", mockRuntime.CreatedConfig.Pull)
		assert.Equal(t, int64(512*1024*1024), mockRuntime.CreatedConfig.Memory)
		assert.InDelta(t, 2.5, mockRuntime.CreatedConfig.CPUs, 0.0001)
		require.Len(t, mockRuntime.CreatedConfig.Mounts, 1)
		assert.Equal(t, "tmpfs", mockRuntime.CreatedConfig.Mounts[0].Type)
		assert.Equal(t, "/tmp", mockRuntime.CreatedConfig.Mounts[0].Target)
		require.Len(t, mockRuntime.CreatedConfig.Devices, 1)
		assert.Equal(t, "/dev/fuse", mockRuntime.CreatedConfig.Devices[0].PathOnHost)
	})

	t.Run("P1 flags override P2 for Docker-compatible features", func(t *testing.T) {
		mockRuntime.CreatedConfig = nil

		_, _, err := executeCommand(
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
		)
		require.NoError(t, err)

		require.NotNil(t, mockRuntime.CreatedConfig)
		assert.Equal(t, []string{"ls", "-l"}, mockRuntime.CreatedConfig.Command)
		assert.Equal(t, []string{"9090:90"}, mockRuntime.CreatedConfig.Ports)
		assert.Equal(t, "overrideUser", mockRuntime.CreatedConfig.User)
		assert.False(t, mockRuntime.CreatedConfig.Privileged)
		assert.Equal(t, "always", mockRuntime.CreatedConfig.Pull)
		assert.Equal(t, int64(2*1024*1024*1024), mockRuntime.CreatedConfig.Memory)
		assert.InDelta(t, 2.0, mockRuntime.CreatedConfig.CPUs, 0.0001)
	})

	t.Run("Invalid pull policy returns error", func(t *testing.T) {
		_, _, err := executeCommand("--pull", "invalid", "--image", "alpine", "sh")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid pull policy \"invalid\"")
		assert.Contains(t, err.Error(), "allowed values are \"always\", \"missing\", or \"never\"")
	})

	t.Run("Invalid pull policy in P1 returns error", func(t *testing.T) {
		_, _, err := executeCommand("--image", "alpine", "sh", "--cderun-pull=invalid")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid pull policy \"invalid\"")
	})
}
