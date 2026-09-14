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

func TestUnit_Polyglot_ExecutionPathScenarios(t *testing.T) {
	t.Parallel()

	t.Run("symlink invocation with relative paths resolves tool name correctly", func(t *testing.T) {
		mock := &runtime.MockRuntime{}
		mfs := &config.MockFileSystem{
			Dirs: map[string]bool{"/project": true},
			WD:   "/project",
			Files: map[string][]byte{
				"/project/.tools.yaml": []byte("python3:\n  image: python:3.11-alpine\n"),
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		err := ExecuteContextWithOptions(ctx, []string{"../tools/python3", "-c", "print('hello')"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mock, nil
			}
			o.exitFunc = func(code int) {}
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
		})

		require.NoError(t, err)

		cfg := mock.GetCreatedConfig()
		require.NotNil(t, cfg)
		assert.Equal(t, "python:3.11-alpine", cfg.Image)
		assert.Equal(t, []string{"-c", "print('hello')"}, cfg.Command)
	})

	t.Run("symlink invocation with current directory prefix resolves tool name correctly", func(t *testing.T) {
		mock := &runtime.MockRuntime{}
		mfs := &config.MockFileSystem{
			Dirs: map[string]bool{"/project": true},
			WD:   "/project",
			Files: map[string][]byte{
				"/project/.tools.yaml": []byte("node:\n  image: node:20-alpine\n"),
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		err := ExecuteContextWithOptions(ctx, []string{"./node", "index.js", "--port", "8080"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mock, nil
			}
			o.exitFunc = func(code int) {}
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
		})

		require.NoError(t, err)

		cfg := mock.GetCreatedConfig()
		require.NotNil(t, cfg)
		assert.Equal(t, "node:20-alpine", cfg.Image)
		assert.Equal(t, []string{"index.js", "--port", "8080"}, cfg.Command)
	})

	t.Run("arguments preserved across double dash in polyglot mode", func(t *testing.T) {
		mock := &runtime.MockRuntime{}
		mfs := &config.MockFileSystem{
			Dirs: map[string]bool{"/project": true},
			WD:   "/project",
			Files: map[string][]byte{
				"/project/.tools.yaml": []byte("npm:\n  image: node:20-alpine\n"),
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		err := ExecuteContextWithOptions(ctx, []string{"npm", "run", "test", "--", "--grep", "spec"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mock, nil
			}
			o.exitFunc = func(code int) {}
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
		})

		require.NoError(t, err)

		cfg := mock.GetCreatedConfig()
		require.NotNil(t, cfg)
		assert.Equal(t, "node:20-alpine", cfg.Image)
		assert.Equal(t, []string{"run", "test", "--", "--grep", "spec"}, cfg.Command)
	})

	t.Run("P1 internal override flag hoisted even when placed after double dash in polyglot mode", func(t *testing.T) {
		mock := &runtime.MockRuntime{}
		mfs := &config.MockFileSystem{
			Dirs: map[string]bool{"/project": true},
			WD:   "/project",
			Files: map[string][]byte{
				"/project/.tools.yaml": []byte("node:\n  image: node:20-alpine\n"),
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		err := ExecuteContextWithOptions(ctx, []string{"node", "server.js", "--", "--cderun-image=node:22-alpine", "--port", "3000"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mock, nil
			}
			o.exitFunc = func(code int) {}
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
		})

		require.NoError(t, err)

		cfg := mock.GetCreatedConfig()
		require.NotNil(t, cfg)
		assert.Equal(t, "node:22-alpine", cfg.Image)
		assert.Equal(t, []string{"server.js", "--", "--port", "3000"}, cfg.Command)
	})

	t.Run("P1 space-separated internal override flag hoisted in polyglot mode", func(t *testing.T) {
		mock := &runtime.MockRuntime{}
		mfs := &config.MockFileSystem{
			Dirs: map[string]bool{"/project": true},
			WD:   "/project",
			Files: map[string][]byte{
				"/project/.tools.yaml": []byte("node:\n  image: node:20-alpine\n"),
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		err := ExecuteContextWithOptions(ctx, []string{"node", "--cderun-workdir", "/app/src", "main.js"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mock, nil
			}
			o.exitFunc = func(code int) {}
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
		})

		require.NoError(t, err)

		cfg := mock.GetCreatedConfig()
		require.NotNil(t, cfg)
		assert.Equal(t, "/app/src", cfg.Workdir)
		assert.Equal(t, []string{"main.js"}, cfg.Command)
	})
}
