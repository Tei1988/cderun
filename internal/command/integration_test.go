package command

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/config"
	"cderun/internal/runtime"
)

func TestIntegration_Execution_AlpineEcho(t *testing.T) {
	t.Parallel()
	t.Run("mount-cderun-path", func(t *testing.T) {
		customPath := "/tmp/custom-cderun"
		stdout, _, exitCode, err := runCderun("--image", "public.ecr.aws/docker/library/alpine:latest", "--mount-socket", "--mount-cderun", "--mount-cderun-path", customPath, "--dry-run", "--dry-run-format", "simple", "echo", "hello")
		require.NoError(t, err)
		assert.Equal(t, 0, exitCode)
		assert.Contains(t, stdout, "source=\""+customPath+"\",target=\"/usr/local/bin/cderun\"")
	})
}

func TestIntegration_Config_ToolsYAML(t *testing.T) {
	t.Parallel()
	mfs := &config.MockFileSystem{
		WD: "/project",
		Files: map[string][]byte{
			"/project/.tools.yaml": []byte(`
node:
  image: node:20-alpine
  tty: true
  network: host
  env:
    - KEY=VALUE
  mounts:
    - type: bind
      source: /host
      target: /container
`),
		},
	}

	mockRuntime := &runtime.MockRuntime{}
	err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "node", "app.js"}, withMockRuntime(mockRuntime, withMockFS(mfs)))
	require.NoError(t, err)

	require.NotNil(t, mockRuntime.CreatedConfig)
	assert.Equal(t, "node:20-alpine", mockRuntime.CreatedConfig.Image)
	assert.Equal(t, []string{"app.js"}, mockRuntime.CreatedConfig.Command)
	assert.True(t, mockRuntime.CreatedConfig.TTY)
	assert.Equal(t, "host", mockRuntime.CreatedConfig.Network)
	assert.Contains(t, mockRuntime.CreatedConfig.Env, "KEY=VALUE")
	assert.Len(t, mockRuntime.CreatedConfig.Mounts, 1)
	assert.Equal(t, "bind", mockRuntime.CreatedConfig.Mounts[0].Type)
	assert.Equal(t, "/host", mockRuntime.CreatedConfig.Mounts[0].Source)
	assert.Equal(t, "/container", mockRuntime.CreatedConfig.Mounts[0].Target)
}

func TestIntegration_Priority_EnvOverTools(t *testing.T) {
	mfs := &config.MockFileSystem{
		WD: "/project",
		Files: map[string][]byte{
			"/project/.tools.yaml": []byte("node:\n  image: node:20-alpine"),
		},
		Env: map[string]string{
			"CDERUN_IMAGE": "env-image:latest",
		},
	}

	mockRuntime := &runtime.MockRuntime{}
	err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "node", "app.js"}, withMockRuntime(mockRuntime, withMockFS(mfs)))
	require.NoError(t, err)
	assert.Equal(t, "env-image:latest", mockRuntime.CreatedConfig.Image)
}

func TestIntegration_Polyglot_ToolSymlink(t *testing.T) {
	t.Parallel()
	mfs := &config.MockFileSystem{
		WD: "/project",
		Files: map[string][]byte{
			"/project/.tools.yaml": []byte("node:\n  image: node:20-alpine"),
		},
	}

	mockRuntime := &runtime.MockRuntime{
		CreatedContainerID: "test-container-id",
		ExitCode:           0,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := ExecuteContextWithOptions(ctx, []string{"node", "--version"}, withMockRuntime(mockRuntime, withMockFS(mfs)))

	require.NoError(t, err)
}
