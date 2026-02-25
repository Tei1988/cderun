package command

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/config"
	"cderun/internal/runtime"
)

func TestUnit_Polyglot_Flags(t *testing.T) {
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
				o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
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
		require.Error(t, execErr)
		assert.Contains(t, execErr.Error(), "no image mapping found for tool: node")

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
			// Simulating "node --cderun-interactive=true --cderun-image=alpine cat"
			execErr = ExecuteContextWithOptions(ctx, []string{"node", "--cderun-interactive=true", "--cderun-image=alpine", "cat"}, func(o *rootOptions, cmd *cobra.Command) {
				o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
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
		assert.Equal(t, "alpine", requireConfig.Image)
	})
}

func TestIntegration_Polyglot_Symlink(t *testing.T) {
	setupTestDir(t)

	err := os.WriteFile(".tools.yaml", []byte("node:\n  image: node:20-alpine"), 0o644)
	require.NoError(t, err)

	mockRuntime := &runtime.MockRuntime{
		CreatedContainerID: "test-container-id",
		ExitCode:           0,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = ExecuteContextWithOptions(ctx, []string{"node", "--version"}, withMockRuntime(mockRuntime))

	require.NoError(t, err)
	cfg := mockRuntime.GetCreatedConfig()
	require.NotNil(t, cfg)
	assert.Equal(t, "node:20-alpine", cfg.Image)
	assert.Equal(t, []string{"--version"}, cfg.Command)
}
