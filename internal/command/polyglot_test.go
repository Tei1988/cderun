package command

import (
	"bytes"
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

func TestUnit_Polyglot_InternalOverridesHoisting(t *testing.T) {
	t.Run("flags without cderun-prefix ARE NOT picked up in polyglot mode (specification)", func(t *testing.T) {
		mock := &pipeMockRuntime{}
		mock.CreatedContainerID = "test-container"

		// Use MockFileSystem
		mfs := &config.MockFileSystem{
			Dirs: map[string]bool{"/project": true},
			WD:   "/project",
		}

		// Simulate symlink execution: node --interactive=true --image alpine cat
		// Specification: only --cderun- prefixed flags are hoisted.

		var stdout bytes.Buffer

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		done := make(chan struct{})
		var execErr error
		go func() {
			// Simulating "node --interactive=true --image alpine cat"
			execErr = ExecuteContextWithOptions(ctx, []string{"node", "--interactive=true", "--image", "alpine", "cat"}, func(o *rootOptions, cmd *cobra.Command) {
				o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
					return mock, nil
				}
				o.exitFunc = func(code int) {}
				o.fs = mfs
				o.configLoader = config.NewConfigLoaderWithFS(mfs)
				cmd.SetOut(&stdout)
				cmd.SetErr(io.Discard)
			})
			close(done)
		}()

		select {
		case <-done:
		case <-ctx.Done():
			t.Fatal("Test timed out")
		}

		// It should fail because no image mapping for 'node' exists, and --image was not hoisted.
		var imgErr *config.ImageNotFoundError
		require.ErrorAs(t, execErr, &imgErr)
		assert.Equal(t, "node", imgErr.Tool)

		requireConfig := mock.GetCreatedConfig()
		assert.Nil(t, requireConfig)
	})

	t.Run("flags WITH cderun-prefix ARE picked up in polyglot mode", func(t *testing.T) {
		mock := &pipeMockRuntime{}
		mock.CreatedContainerID = "test-container"

		// Use MockFileSystem
		mfs := &config.MockFileSystem{
			Dirs: map[string]bool{"/project": true},
			WD:   "/project",
			Files: map[string][]byte{
				"/project/.tools.yaml": []byte("node:\n  image: node:20"),
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		done := make(chan struct{})
		var execErr error
		go func() {
			// Simulating "node --cderun-interactive=true --cderun-image=node:alpine cat"
			execErr = ExecuteContextWithOptions(ctx, []string{"node", "--cderun-interactive=true", "--cderun-image=node:alpine", "cat"}, func(o *rootOptions, cmd *cobra.Command) {
				o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
					return mock, nil
				}
				o.exitFunc = func(code int) {}
				o.fs = mfs
				o.configLoader = config.NewConfigLoaderWithFS(mfs)
				cmd.SetOut(io.Discard)
				cmd.SetErr(io.Discard)
			})
			close(done)
		}()

		select {
		case <-done:
		case <-ctx.Done():
			t.Fatal("Test timed out")
		}

		require.NoError(t, execErr)

		requireConfig := mock.GetCreatedConfig()
		assert.NotNil(t, requireConfig)
		assert.True(t, requireConfig.Interactive)
		assert.Equal(t, "node:alpine", requireConfig.Image)
	})
}

func TestUnit_Polyglot_PreprocessArgs_Symlinks_Additions(t *testing.T) {
	t.Parallel()

	t.Run("symlink to absolute path", func(t *testing.T) {
		args := []string{"/usr/local/bin/node", "--version"}

		cmd := newRootCmd(&rootOptions{})
		processed, err := preprocessArgs(cmd, args)
		require.NoError(t, err)

		assert.Equal(t, []string{"cderun", "node", "--version"}, processed)
	})
}

func TestUnit_Polyglot_PreprocessArgs_MultiTool(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		args     []string
		expected []string
	}{
		{
			name:     "node symlink",
			args:     []string{"node", "-v"},
			expected: []string{"cderun", "node", "-v"},
		},
		{
			name:     "python3 symlink",
			args:     []string{"python3", "--version"},
			expected: []string{"cderun", "python3", "--version"},
		},
		{
			name:     "git symlink with absolute path",
			args:     []string{"/usr/bin/git", "status"},
			expected: []string{"cderun", "git", "status"},
		},
		{
			name:     "cderun standard mode",
			args:     []string{"cderun", "run", "--image", "alpine"},
			expected: []string{"cderun", "run", "--image", "alpine"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newRootCmd(&rootOptions{})
			processed, err := preprocessArgs(cmd, tc.args)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, processed)
		})
	}
}
