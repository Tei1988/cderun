package command

import (
	"cderun/internal/runtime"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFlags(t *testing.T) {
	// Save and restore package-level state
	oldFactory := runtimeFactory
	oldExit := exitFunc
	t.Cleanup(func() {
		runtimeFactory = oldFactory
		exitFunc = oldExit
	})

	mockRuntime := &runtime.MockRuntime{}
	runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
		return mockRuntime, nil
	}
	exitFunc = func(code int) {}

	t.Run("P2 flags for new features", func(t *testing.T) {
		mockRuntime.CreatedConfig = nil

		_, err := executeCommand(
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
			"--tmpfs", "/tmp:rw",
			"--device", "/dev/fuse",
			"--image", "alpine",
			"sh", "ls", "-l",
		)
		assert.NoError(t, err)

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
		assert.Equal(t, 2.5, mockRuntime.CreatedConfig.CPUs)
		assert.Equal(t, []string{"/tmp:rw"}, mockRuntime.CreatedConfig.Tmpfs)
		require.Len(t, mockRuntime.CreatedConfig.Devices, 1)
		assert.Equal(t, "/dev/fuse", mockRuntime.CreatedConfig.Devices[0].PathOnHost)
	})

	t.Run("P1 flags override P2 for new features", func(t *testing.T) {
		mockRuntime.CreatedConfig = nil

		_, err := executeCommand(
			"--publish", "8080:80",
			"--user", "olduser",
			"--privileged=true",
			"--pull", "missing",
			"--memory", "1g",
			"--cpus", "1.0",
			"--image", "alpine",
			"sh", "ls", "-l",
			"--cderun-publish=9090:90",
			"--cderun-user=newuser",
			"--cderun-privileged=false",
			"--cderun-pull=always",
			"--cderun-memory=2g",
			"--cderun-cpus=2.0",
		)
		assert.NoError(t, err)

		require.NotNil(t, mockRuntime.CreatedConfig)
		assert.Equal(t, []string{"ls", "-l"}, mockRuntime.CreatedConfig.Command)
		assert.Equal(t, []string{"9090:90"}, mockRuntime.CreatedConfig.Ports)
		assert.Equal(t, "newuser", mockRuntime.CreatedConfig.User)
		assert.False(t, mockRuntime.CreatedConfig.Privileged)
		assert.Equal(t, "always", mockRuntime.CreatedConfig.Pull)
		assert.Equal(t, int64(2*1024*1024*1024), mockRuntime.CreatedConfig.Memory)
		assert.Equal(t, 2.0, mockRuntime.CreatedConfig.CPUs)
	})

	t.Run("Resolve from .tools.yaml for new features", func(t *testing.T) {
		mockRuntime.CreatedConfig = nil

		oldWd, err := os.Getwd()
		require.NoError(t, err)
		tmpDir := t.TempDir()
		err = os.Chdir(tmpDir)
		require.NoError(t, err)
		t.Cleanup(func() { _ = os.Chdir(oldWd) })

		toolsContent := `
node:
  image: node:20
  ports: ["8080:80"]
  privileged: true
  memory: 1g
  cpus: 1.5
`
		err = os.WriteFile(".tools.yaml", []byte(toolsContent), 0644)
		require.NoError(t, err)

		_, err = executeCommand("node", "app.js")
		assert.NoError(t, err)

		require.NotNil(t, mockRuntime.CreatedConfig)
		assert.Equal(t, []string{"app.js"}, mockRuntime.CreatedConfig.Command)
		assert.Equal(t, []string{"8080:80"}, mockRuntime.CreatedConfig.Ports)
		assert.True(t, mockRuntime.CreatedConfig.Privileged)
		assert.Equal(t, int64(1024*1024*1024), mockRuntime.CreatedConfig.Memory)
		assert.Equal(t, 1.5, mockRuntime.CreatedConfig.CPUs)
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
