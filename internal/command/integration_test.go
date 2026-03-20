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

func TestIntegration_Execution_SubcommandMapping(t *testing.T) {
	t.Parallel()
	mfs := &config.MockFileSystem{
		WD: "/project",
		Files: map[string][]byte{
			"/project/.tools.yaml": []byte("node:\n  image: node:20-alpine\n  network: host"),
		},
	}
	mockRuntime := runtime.NewMockRuntime()
	mockRuntime.ExitCode = 0

	err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "node", "--version"}, withMockRuntime(mockRuntime, withMockFS(mfs)))
	require.NoError(t, err)

	cfg := mockRuntime.GetCreatedConfig()
	require.NotNil(t, cfg)
	assert.Equal(t, "node:20-alpine", cfg.Image)
	assert.Equal(t, "host", cfg.Network)
}

func TestIntegration_Flags_InternalOverrides(t *testing.T) {
	t.Parallel()

	t.Run("cderun-tty overrides tty even if placed after subcommand", func(t *testing.T) {
		mfs := &config.MockFileSystem{
			WD: "/project",
			Files: map[string][]byte{
				"/project/.tools.yaml": []byte("node:\n  image: node:20-alpine"),
			},
		}
		mockRuntime := runtime.NewMockRuntime()
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--tty=true", "node", "--cderun-tty=false", "--version"}, withMockRuntime(mockRuntime, withMockFS(mfs), func(o *rootOptions, cmd *cobra.Command) {
			o.isTerminal = func(fd int) bool { return true }
		}))
		require.NoError(t, err)
		assert.False(t, mockRuntime.GetCreatedConfig().TTY)
	})

	t.Run("cderun internal overrides for network and mount", func(t *testing.T) {
		mfs := &config.MockFileSystem{
			WD: "/project",
			Files: map[string][]byte{
				"/project/.tools.yaml": []byte("sh:\n  image: alpine"),
			},
		}
		mockRuntime := runtime.NewMockRuntime()
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "--network=bridge", "--mount=type=bind,source=/h1,target=/c1", "sh", "--cderun-network=host", "--cderun-mount=type=bind,source=/h2,target=/c2"}, withMockRuntime(mockRuntime, withMockFS(mfs), func(o *rootOptions, cmd *cobra.Command) {
			o.isTerminal = func(fd int) bool { return true }
		}))
		require.NoError(t, err)
		cfg := mockRuntime.GetCreatedConfig()
		assert.Equal(t, "host", cfg.Network)
		assert.Len(t, cfg.Mounts, 1)
		assert.Equal(t, "/h2", cfg.Mounts[0].Source)
	})
}
