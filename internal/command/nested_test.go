package command

import (
	"cderun/internal/runtime"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNestedCderunEnvironment(t *testing.T) {
	// Save and restore package-level state
	originalFactory := runtimeFactory
	originalExit := exitFunc
	t.Cleanup(func() {
		runtimeFactory = originalFactory
		exitFunc = originalExit
	})

	mockRuntime := &runtime.MockRuntime{}
	runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
		return mockRuntime, nil
	}
	exitFunc = func(code int) {}

	t.Run("mount-cderun propagates host paths via env vars", func(t *testing.T) {
		mockRuntime.CreatedConfig = nil

		_, err := executeCommand(
			"--image", "alpine",
			"--mount-cderun",
			"--mount-socket",
			"--socket-path", "/var/run/docker.sock",
			"--mount-socket-path", "/var/run/docker.sock",
			"sh",
		)
		assert.NoError(t, err)

		require.NotNil(t, mockRuntime.CreatedConfig)
		envs := mockRuntime.CreatedConfig.Env

		exePath, _ := os.Executable()

		assert.Contains(t, envs, "CDERUN_MOUNT_CDERUN_PATH="+exePath)
		assert.Contains(t, envs, "CDERUN_SOCKET_PATH=/var/run/docker.sock")
		assert.Contains(t, envs, "CDERUN_MOUNT_SOCKET_SOURCE_PATH=/var/run/docker.sock")
		assert.Contains(t, envs, "CDERUN_MOUNT_SOCKET=true")
		assert.Contains(t, envs, "CDERUN_RUNTIME=docker")
	})
}
