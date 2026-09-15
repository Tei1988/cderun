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

func TestUnit_CommandScenarios_WrapperMode_ComplexHoisting(t *testing.T) {
	t.Parallel()

	t.Run("wrapper mode hoists space and equal separated P1 flags", func(t *testing.T) {
		mock := &runtime.MockRuntime{}
		mfs := &config.MockFileSystem{
			Dirs:  map[string]bool{"/project": true},
			WD:    "/project",
			Files: map[string][]byte{},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		err := ExecuteContextWithOptions(ctx, []string{
			"cderun",
			"golang",
			"--cderun-image", "golang:1.22-alpine",
			"--cderun-workdir", "/workspace/src",
			"--cderun-env=DB_PASS=secret123",
			"test", "./...",
		}, func(o *rootOptions, cmd *cobra.Command) {
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
		assert.Equal(t, "golang:1.22-alpine", cfg.Image)
		assert.Equal(t, "/workspace/src", cfg.Workdir)
		assert.Contains(t, cfg.Env, "DB_PASS=secret123")
		assert.Equal(t, []string{"test", "./..."}, cfg.Command)
	})
}

func TestUnit_CommandScenarios_SymlinkMode_ExecutionWithOverrides(t *testing.T) {
	t.Parallel()

	t.Run("symlink mode parses tool configuration and applies P1 overrides", func(t *testing.T) {
		mock := &runtime.MockRuntime{}
		mfs := &config.MockFileSystem{
			Dirs: map[string]bool{"/project": true},
			WD:   "/project",
			Files: map[string][]byte{
				"/project/.tools.yaml": []byte("python:\n  image: python:3.9-alpine\n  env:\n    - PYTHONDONTWRITEBYTECODE=1\n"),
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		err := ExecuteContextWithOptions(ctx, []string{
			"python",
			"--cderun-image", "python:3.11-slim",
			"script.py",
			"--verbose",
		}, func(o *rootOptions, cmd *cobra.Command) {
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
		assert.Equal(t, "python:3.11-slim", cfg.Image)
		assert.Equal(t, []string{"script.py", "--verbose"}, cfg.Command)
		assert.Contains(t, cfg.Env, "PYTHONDONTWRITEBYTECODE=1")
	})
}

func TestUnit_CommandScenarios_AdHocMode_NoConfigFile(t *testing.T) {
	t.Parallel()

	t.Run("ad-hoc mode executes without tools config file when image specified", func(t *testing.T) {
		mock := &runtime.MockRuntime{}
		mfs := &config.MockFileSystem{
			Dirs:  map[string]bool{"/empty": true},
			WD:    "/empty",
			Files: map[string][]byte{},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		err := ExecuteContextWithOptions(ctx, []string{
			"cderun",
			"echo",
			"--cderun-image", "alpine:latest",
			"hello ad-hoc",
		}, func(o *rootOptions, cmd *cobra.Command) {
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
		assert.Equal(t, "alpine:latest", cfg.Image)
		assert.Equal(t, []string{"hello ad-hoc"}, cfg.Command)
	})
}
