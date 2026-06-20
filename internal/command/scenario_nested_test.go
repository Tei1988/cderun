package command

import (
	"context"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/config"
	"cderun/internal/logging"
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

	err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--mount-cderun", "node", "ls"}, func(o *rootOptions, cmd *cobra.Command) {
		o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
			return mockRuntime, nil
		}
		o.exitFunc = func(code int) {}
		o.fs = mfs
		o.configLoader = config.NewConfigLoaderWithFS(mfs)
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

func TestScenario_Nested_Level2Translation(t *testing.T) {
	t.Parallel()
	// Scenario: Inside Container A (Level 1), running cderun to start Container B (Level 2).
	// Host /host/project is mounted as /app in Container A.
	// Container A calls: cderun --mount type=bind,source=/app/src,target=/src node app.js
	// Container B should have /host/project/src mounted as /src.

	mfs := &config.MockFileSystem{
		WD: "/app",
		Files: map[string][]byte{
			"/.cderun.yaml": []byte(`
hostContext:
  level: 1
  mounts:
    - source: /host/project
      target: /app
`),
		},
		ExecPath: "/usr/local/bin/cderun",
	}

	mockRuntime := &runtime.MockRuntime{
		CreatedContainerID: "container-b",
	}

	err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "node:20", "--mount", "type=bind,source=/app/src,target=/src", "node", "app.js"}, func(o *rootOptions, cmd *cobra.Command) {
		o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
			return mockRuntime, nil
		}
		o.exitFunc = func(code int) {}
		o.fs = mfs
		o.configLoader = config.NewConfigLoaderWithFS(mfs)
	})
	require.NoError(t, err)

	cfg := mockRuntime.GetCreatedConfig()
	require.NotNil(t, cfg)

	// Verify reverse path resolution
	foundMount := false
	for _, m := range cfg.Mounts {
		if m.Target == "/src" {
			assert.Equal(t, "/host/project/src", m.Source)
			foundMount = true
		}
	}
	assert.True(t, foundMount, "Mount /src should be found with translated source")
}

func TestScenario_Nested_Level2RecursiveTranslation(t *testing.T) {
	t.Parallel()
	// Scenario: Testing recursive property of reverse resolution.
	// Current implementation only applies one step of resolution.
	// Host /host/data -> Container A /data
	// Container A /data/subdir -> Container B /mnt
	// Container B calls cderun with source /mnt/logs
	// /mnt/logs -> (step 1) /data/subdir/logs
	// If Level 1 mapping is ALSO in the same HostContext, we can reach host path by multi-pass OR properly structured mappings.

	mfs := &config.MockFileSystem{
		WD: "/mnt",
		Files: map[string][]byte{
			"/.cderun.yaml": []byte(`
hostContext:
  level: 2
  mounts:
    - source: /host/data/subdir
      target: /mnt
      level: 2
`),
		},
		ExecPath: "/usr/local/bin/cderun",
	}

	mockRuntime := &runtime.MockRuntime{
		CreatedContainerID: "container-c",
	}

	err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "--mount", "type=bind,source=/mnt/logs,target=/logs", "alpine", "cat", "/logs/err.log"}, func(o *rootOptions, cmd *cobra.Command) {
		o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
			return mockRuntime, nil
		}
		o.exitFunc = func(code int) {}
		o.fs = mfs
		o.configLoader = config.NewConfigLoaderWithFS(mfs)
	})
	require.NoError(t, err)

	cfg := mockRuntime.GetCreatedConfig()
	require.NotNil(t, cfg)

	foundMount := false
	for _, m := range cfg.Mounts {
		if m.Target == "/logs" {
			assert.Equal(t, "/host/data/subdir/logs", m.Source)
			foundMount = true
		}
	}
	assert.True(t, foundMount, "Mount /logs should be found with translated source")
}
