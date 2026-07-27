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

func TestUnit_Polyglot_ExtraScenarios(t *testing.T) {
	t.Parallel()

	t.Run("polyglot mode - complex flag mix", func(t *testing.T) {
		mock := &runtime.MockRuntime{
			CreatedContainerID: "test-container",
		}
		mfs := &config.MockFileSystem{
			Dirs: map[string]bool{"/project": true},
			WD:   "/project",
			Files: map[string][]byte{
				"/project/.tools.yaml": []byte("node:\n  image: node:20"),
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		err := ExecuteContextWithOptions(ctx, []string{"node", "--cderun-tty", "-v", "--cderun-image=node:alpine", "app.js"}, func(o *rootOptions, cmd *cobra.Command) {
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
		assert.True(t, cfg.TTY)
		assert.Equal(t, "node:alpine", cfg.Image)
		assert.Equal(t, []string{"-v", "app.js"}, cfg.Command)
	})

	t.Run("polyglot mode - executable as path", func(t *testing.T) {
		mock := &runtime.MockRuntime{
			CreatedContainerID: "test-container",
		}
		mfs := &config.MockFileSystem{
			WD: "/project",
			Files: map[string][]byte{
				"/project/.tools.yaml": []byte("node:\n  image: node:20"),
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		err := ExecuteContextWithOptions(ctx, []string{"/usr/local/bin/node", "--cderun-tty", "-v"}, func(o *rootOptions, cmd *cobra.Command) {
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
		assert.Equal(t, "node:20", cfg.Image)
		assert.Equal(t, []string{"-v"}, cfg.Command)
	})
}

func TestUnit_Wrapper_P1Positions(t *testing.T) {
	t.Parallel()

	mock := &runtime.MockRuntime{}
	mfs := &config.MockFileSystem{
		WD: "/project",
		Files: map[string][]byte{
			"/project/.tools.yaml": []byte("node:\n  image: node:20"),
		},
	}

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "P1 immediately after subcmd",
			args: []string{"cderun", "node", "--cderun-tty", "-v"},
		},
		{
			name: "P1 at the end",
			args: []string{"cderun", "node", "-v", "--cderun-tty"},
		},
		{
			name: "Multiple P1 flags",
			args: []string{"cderun", "node", "--cderun-tty", "-v", "--cderun-image=node:alpine"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			err := ExecuteContextWithOptions(ctx, tt.args, func(o *rootOptions, cmd *cobra.Command) {
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
			assert.True(t, cfg.TTY)
		})
	}
}
