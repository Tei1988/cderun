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

func TestIntegration_Scenario_NestedRecursiveToolFlow(t *testing.T) {
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

	savedWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(hostProjectDir))
	t.Cleanup(func() { _ = os.Chdir(savedWd) })

	err = ExecuteContextWithOptions(context.Background(), []string{"cderun", "--mount-cderun", "node", "ls"}, func(o *rootOptions, cmd *cobra.Command) {
		o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
			return mockRuntime, nil
		}
		o.exitFunc = func(code int) {}
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
