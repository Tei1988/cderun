package command

import (
	"cderun/internal/config"
	"cderun/internal/runtime"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScenario_NestedExecution(t *testing.T) {
	// This test modifies global state (runtimeFactory, exitFunc, runConfigDir)
	// and changes the working directory. It should not be run in parallel.

	// 1. Setup mock environment
	tmpDir := t.TempDir()

	// Create a dummy project on "host"
	hostProjectDir := filepath.Join(tmpDir, "host-project")
	require.NoError(t, os.MkdirAll(hostProjectDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(hostProjectDir, "hello.txt"), []byte("hello from host"), 0644))

	// Simulate being inside a container (Level 1)
	// Current directory in container is /app
	// Host mapping: /home/user/project -> /app
	runDir := filepath.Join(tmpDir, "run")
	require.NoError(t, os.MkdirAll(runDir, 0755))

	restoreRunDir := config.SetRunConfigDirForTest(runDir)
	defer restoreRunDir()

	simulatedAppDir := filepath.Join(tmpDir, "simulated-app")
	require.NoError(t, os.MkdirAll(filepath.Join(simulatedAppDir, "subdir"), 0755))

	nestedConfig := `
hostContext:
  level: 1
  binPath: "/usr/local/bin/cderun"
  workingDir: "/home/user/project"
  mounts:
    - source: "` + hostProjectDir + `"
      target: "` + simulatedAppDir + `"
      level: 1
`
	require.NoError(t, os.WriteFile(filepath.Join(runDir, ".cderun.yaml"), []byte(nestedConfig), 0644))

	// 2. Setup Mock Runtime
	prevFactory := runtimeFactory
	prevExit := exitFunc
	t.Cleanup(func() {
		runtimeFactory = prevFactory
		exitFunc = prevExit
	})

	mockRuntime := &runtime.MockRuntime{}
	runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
		return mockRuntime, nil
	}
	exitFunc = func(code int) {}

	// 3. Run cderun as if we are in the container
	// Current working directory is /app (simulated)
	// We want to mount a subdirectory: --mount type=bind,source=./subdir,target=/mnt
	// /app/subdir should be translated to hostProjectDir/subdir

	savedWd, err := os.Getwd()
	require.NoError(t, err)

	// In the simulated container, PWD is simulatedAppDir
	require.NoError(t, os.Chdir(simulatedAppDir))
	t.Cleanup(func() { _ = os.Chdir(savedWd) })

	_, err = executeCommand("--image", "alpine", "--mount", "type=bind,source=./subdir,target=/mnt", "sh")
	assert.NoError(t, err)

	// 4. Verify path translation
	require.NotNil(t, mockRuntime.CreatedConfig)
	found := false
	for _, m := range mockRuntime.CreatedConfig.Mounts {
		if m.Target == "/mnt" {
			// Source should be translated back to host path
			// hostProjectDir + /subdir
			expectedSource := filepath.Join(hostProjectDir, "subdir")
			assert.Equal(t, expectedSource, m.Source)
			found = true
		}
	}
	assert.True(t, found, "mount for /mnt not found")
}
