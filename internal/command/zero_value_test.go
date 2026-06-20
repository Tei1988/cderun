package command

import (
	"context"
	"testing"

	"cderun/internal/config"
	"cderun/internal/runtime"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Command_ExplicitZeroValue(t *testing.T) {
	t.Run("explicit --tty=false should be preserved", func(t *testing.T) {
		mockRuntime := &runtime.MockRuntime{}
		// Set a default in ToolConfig that would normally win if CLI didn't specify
		 _ = config.ToolsConfig{
			"node": config.ToolConfig{
				Image: "node:20",
				TTY:   config.Ptr(true),
			},
		}

		args := []string{"cderun", "--tty=false", "node", "ls"}

		err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.configLoader = config.NewConfigLoaderWithFS(&config.MockFileSystem{
				Files: map[string][]byte{
					"/.tools.yaml": []byte("node:\n  image: node:20\n  tty: true"),
				},
				WD: "/",
			})
			o.exitFunc = func(code int) {}
			o.isTerminal = func(fd int) bool { return true }
		})
		require.NoError(t, err)

		cfg := mockRuntime.GetCreatedConfig()
		require.NotNil(t, cfg)
		// CLI --tty=false (P2) should win over Tool TTY=true (P4)
		assert.False(t, cfg.TTY)
	})

	t.Run("explicit --cderun-tty=false should be preserved", func(t *testing.T) {
		mockRuntime := &runtime.MockRuntime{}
		args := []string{"cderun", "node", "--cderun-tty=false", "ls"}

		err := ExecuteContextWithOptions(context.Background(), args, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.configLoader = config.NewConfigLoaderWithFS(&config.MockFileSystem{
				Files: map[string][]byte{
					"/.tools.yaml": []byte("node:\n  image: node:20\n  tty: true"),
				},
				WD: "/",
			})
			o.exitFunc = func(code int) {}
			o.isTerminal = func(fd int) bool { return true }
		})
		require.NoError(t, err)

		cfg := mockRuntime.GetCreatedConfig()
		require.NotNil(t, cfg)
		assert.False(t, cfg.TTY)
	})
}
