package command

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/runtime"
)

func TestScenario_Nested_ExecutionFlow(t *testing.T) {
	t.Parallel()
	// Scenario: Standard Docker environment (level 0) running a container (level 1).
	// We verify that a snapshot is created when requested.

	tmpDir := t.TempDir()
	hostProjectDir := filepath.Join(tmpDir, "project")
	_ = os.MkdirAll(hostProjectDir, 0o755)

	// 1. Initial State (Host side)
	err := os.WriteFile(filepath.Join(hostProjectDir, ".tools.yaml"), []byte("node:\n  image: node:20"), 0o644)
	require.NoError(t, err)

	// 2. Execution (simulate cderun node ls)
	mockRuntime := &runtime.MockRuntime{
		CreatedContainerID: "test-container-id",
	}

	fs := testFileSystem{wd: hostProjectDir}
	err = ExecuteContextWithOptions(context.Background(), []string{"cderun", "--mount-cderun", "node", "ls"}, func(o *rootOptions, cmd *cobra.Command) {
		o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
			return mockRuntime, nil
		}
		o.exitFunc = func(code int) {}
		o.fs = &fs
	})
	require.NoError(t, err)
	cfg := mockRuntime.GetCreatedConfig()
	require.NotNil(t, cfg)

	// Check that snapshot was created and mounted
	foundSnapshot := false
	for _, m := range cfg.Mounts {
		if m.Target == "/run/cderun" {
			foundSnapshot = true
			break
		}
	}
	assert.True(t, foundSnapshot, "snapshot should be mounted")
}
