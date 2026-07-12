package command

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/config"
	"cderun/internal/logging"
	"cderun/internal/runtime"
)

func runPolyglotTest(ctx context.Context, args []string, mfs *config.MockFileSystem, mock *runtime.MockRuntime) error {
	return ExecuteContextWithOptions(ctx, args, func(o *rootOptions, cmd *cobra.Command) {
		o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
			return mock, nil
		}
		o.exitFunc = func(code int) {}
		o.fs = mfs
		o.configLoader = config.NewConfigLoaderWithFS(mfs)
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
	})
}

func TestScenario_Polyglot_Extra(t *testing.T) {
	t.Run("detect tool from absolute path", func(t *testing.T) {
		mock := &runtime.MockRuntime{}
		mfs := &config.MockFileSystem{
			WD: "/app",
			Files: map[string][]byte{
				"/app/.tools.yaml": []byte("node:\n  image: node:20"),
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		err := runPolyglotTest(ctx, []string{"/usr/local/bin/node", "--version"}, mfs, mock)

		require.NoError(t, err)
		cfg := mock.GetCreatedConfig()
		require.NotNil(t, cfg)
		assert.Equal(t, "node:20", cfg.Image)
		assert.Equal(t, []string{"--version"}, cfg.Command)
	})

	t.Run("P1 flags interleaved with tool flags in polyglot mode", func(t *testing.T) {
		mock := &runtime.MockRuntime{}
		mfs := &config.MockFileSystem{
			WD: "/app",
			Files: map[string][]byte{
				"/app/.tools.yaml": []byte("node:\n  image: node:20"),
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		// node -v --cderun-image node:alpine --foo
		err := runPolyglotTest(ctx, []string{"node", "-v", "--cderun-image", "node:alpine", "--foo"}, mfs, mock)

		require.NoError(t, err)
		cfg := mock.GetCreatedConfig()
		require.NotNil(t, cfg)
		assert.Equal(t, "node:alpine", cfg.Image)
		assert.Equal(t, []string{"-v", "--foo"}, cfg.Command)
	})

	t.Run("tool name with path separators in polyglot mode", func(t *testing.T) {
		mock := &runtime.MockRuntime{}
		mfs := &config.MockFileSystem{
			WD: "/app",
			Files: map[string][]byte{
				"/app/.tools.yaml": []byte("git:\n  image: alpine/git"),
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		// ./git status
		err := runPolyglotTest(ctx, []string{"./git", "status"}, mfs, mock)

		require.NoError(t, err)
		cfg := mock.GetCreatedConfig()
		require.NotNil(t, cfg)
		assert.Equal(t, "alpine/git", cfg.Image)
		assert.Equal(t, []string{"status"}, cfg.Command)
	})
}
