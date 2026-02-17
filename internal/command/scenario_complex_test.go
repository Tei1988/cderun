package command

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/config"
	"cderun/internal/logging"
	"cderun/internal/runtime"
)

func TestScenario_Command_Complex_MultiToolNestedExecution(t *testing.T) {
	tmpDir := setupTestDir(t)

	// Define tools
	toolsContent := `
git:
  image: alpine/git
  mountCderun: true
  mountTools: ["node"]
node:
  image: node:20-alpine
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".tools.yaml"), []byte(toolsContent), 0o644))

	// Setup Mock Runtime
	mrt := runtime.NewMockRuntime()
	originalFactory := runtimeFactory
	runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
		return mrt, nil
	}
	t.Cleanup(func() { runtimeFactory = originalFactory })

	// Set log level to debug for better trace
	_ = logging.Init("debug", "text", true)

	t.Run("execute git which mounts node", func(t *testing.T) {
		_, stderr, exitCode, err := runCderun("git", "status")
		require.NoError(t, err)
		assert.Equal(t, 0, exitCode, "stderr: %s", stderr)
		assert.Empty(t, stderr)

		// Verify git execution
		createdConfig := mrt.GetCreatedConfig()
		require.NotNil(t, createdConfig)
		assert.Equal(t, "alpine/git", createdConfig.Image)
		assert.Equal(t, []string{"status"}, createdConfig.Command)

		// Verify mounts for nested execution
		var foundCderun, foundNode, foundSnapshot bool
		for _, m := range createdConfig.Mounts {
			if m.Target == "/usr/local/bin/cderun" {
				foundCderun = true
			}
			if m.Target == "/usr/local/bin/node" {
				foundNode = true
			}
			if m.Target == "/run/cderun" {
				foundSnapshot = true
			}
		}
		assert.True(t, foundCderun, "cderun should be mounted")
		assert.True(t, foundNode, "node tool should be mounted")
		assert.True(t, foundSnapshot, "snapshot directory should be mounted for nested execution")

		// Now simulate nested execution by loading config from the snapshot
		var snapshotDir string
		for _, m := range createdConfig.Mounts {
			if m.Target == "/run/cderun" {
				snapshotDir = m.Source
				break
			}
		}
		require.NotEmpty(t, snapshotDir)

		// We need to use a trick to make FindConfigs look into our snapshotDir.
		// SetRunConfigDirForTest modifies the global defaultLoader.
		cleanup := config.SetRunConfigDirForTest(snapshotDir)
		defer cleanup()

		t.Run("nested execution of node", func(t *testing.T) {
			// In the nested execution, cderun would be called as 'node'
			// We simulate this by calling runCderun with "node"
			// and ensuring it uses the snapshot config.

			// Reset mock runtime for the next call
			mrt2 := runtime.NewMockRuntime()
			runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
				return mrt2, nil
			}

			_, stderr2, exitCode2, err2 := runCderun("node", "--version")
			require.NoError(t, err2)
			assert.Equal(t, 0, exitCode2, "stderr: %s", stderr2)

			createdConfig2 := mrt2.GetCreatedConfig()
			require.NotNil(t, createdConfig2)
			assert.Equal(t, "node:20-alpine", createdConfig2.Image)
			assert.Equal(t, []string{"--version"}, createdConfig2.Command)
		})
	})
}
