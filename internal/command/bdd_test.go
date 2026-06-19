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

func TestScenario_Execution_WithCustomConfigs(t *testing.T) {
	t.Parallel()

	t.Run("Execution with custom .tools.yaml and .cderun.yaml", func(t *testing.T) {
		// Given: A project directory with .tools.yaml and .cderun.yaml
		projectDir := "/home/user/project"
		mfs := &config.MockFileSystem{
			WD: projectDir,
			Dirs: map[string]bool{
				projectDir: true,
			},
			Files: map[string][]byte{
				projectDir + "/.cderun.yaml": []byte(`
defaults:
  network: custom-net
  env: ["GLOBAL_VAR=1"]
`),
				projectDir + "/.tools.yaml": []byte(`
node:
  image: node:20
  env: ["TOOL_VAR=1"]
`),
			},
		}

		mockRuntime := &runtime.MockRuntime{}

		// When: Executing "node --version"
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "node", "--version"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.exitFunc = func(code int) {}
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
		})

		// Then: Container should be created with correct image, network, and merged environment
		require.NoError(t, err)
		cfg := mockRuntime.GetCreatedConfig()
		require.NotNil(t, cfg)
		assert.Equal(t, "node:20", cfg.Image)
		assert.Equal(t, "custom-net", cfg.Network)

		// Slices are not merged across layers, TOOL_VAR should be present, GLOBAL_VAR should not
		assert.Contains(t, cfg.Env, "TOOL_VAR=1")
		assert.NotContains(t, cfg.Env, "GLOBAL_VAR=1")
	})
}
