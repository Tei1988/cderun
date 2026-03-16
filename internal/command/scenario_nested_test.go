package command

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/config"
	"cderun/internal/runtime"
)

func TestScenario_Nested_RecursiveToolFlow(t *testing.T) {
	t.Parallel()
	// Scenario: Standard Docker environment (level 0) running a container (level 1).
	// We verify that a snapshot is created when requested.

	mfs := &config.MockFileSystem{
		WD: "/project",
		Files: map[string][]byte{
			"/project/.tools.yaml": []byte("node:\n  image: node:20"),
		},
		ExecPath: "/usr/local/bin/cderun",
	}

	// 2. Execution (simulate cderun node ls)
	mockRuntime := &runtime.MockRuntime{
		CreatedContainerID: "test-container-id",
	}

	err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--mount-cderun", "node", "ls"}, withMockRuntime(mockRuntime, withMockFS(mfs)))
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
