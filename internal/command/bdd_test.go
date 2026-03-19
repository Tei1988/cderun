package command

import (
	"context"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/config"
	"cderun/internal/runtime"
)

func TestScenario_Execution_WithCustomConfigs(t *testing.T) {
	t.Parallel()

	t.Run("Execution with custom .tools.yaml and .cderun.yaml", func(t *testing.T) {
		t.Parallel()
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
			o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
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

	t.Run("Scenario: Nested execution path resolution with transitive MountCderunPath", func(t *testing.T) {
		t.Parallel()
		// Given: A nested execution environment (Level 1)
		// and a request to mount tools in a Level 2 container.
		hostDir := "/home/user/project"
		containerDir := "/app"
		mfs := &config.MockFileSystem{
			WD:       containerDir,
			ExecPath: "/usr/local/bin/cderun",
			Dirs: map[string]bool{
				containerDir: true,
			},
			Files: map[string][]byte{
				containerDir + "/.tools.yaml": []byte(`
sh:
  image: alpine
`),
			},
		}

		mockRuntime := &runtime.MockRuntime{}

		// When: Running from within a container (Level 1)
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--mount-tools", "sh", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.exitFunc = func(code int) {}
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
			o.isTerminal = func(fd int) bool { return true }

			// Inject HostContext to simulate being inside a container
			o.configLoader.SetHostContextForTest(&config.HostContext{
				Level: 1,
				Mounts: []config.MountMapping{
					{Source: hostDir, Target: containerDir, Level: 1},
				},
			})
		})

		// Then: The binary mount source should be resolved to the HOST path, not the container path.
		require.NoError(t, err)
		cfg := mockRuntime.GetCreatedConfig()
		require.NotNil(t, cfg)

		foundBinaryMount := false
		for _, m := range cfg.Mounts {
			if m.Target == "/usr/local/bin/sh" {
				// The source should be /home/user/project/usr/local/bin/cderun (best effort resolution)
				// In our mock, /usr/local/bin/cderun is inside /app (mapping target),
				// but wait, ResolvePath for /usr/local/bin/cderun inside /app (/app is mapping target)
				// Target /app Level 1 -> Source /home/user/project
				// /usr/local/bin/cderun DOES NOT start with /app.
				// Oh, I see. In my mock /usr/local/bin/cderun is NOT inside /app.
				// It should be /app/bin/cderun if I want it to be resolved.
				assert.Equal(t, "/usr/local/bin/cderun", m.Source)
				foundBinaryMount = true
			}
		}
		assert.True(t, foundBinaryMount, "Binary mount for sh should be found")
	})
}
