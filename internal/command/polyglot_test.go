package command

import (
	"context"
	"io"
	"testing"
	"time"

	"cderun/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Command_Root_PolyglotFlags(t *testing.T) {
	t.Run("flags without cderun-prefix ARE NOT picked up in polyglot mode (specification)", func(t *testing.T) {
		mock := &pipeMockRuntime{}
		mock.CreatedContainerID = "test-container"
		o := setupTestOptions(t)
		setupMockRuntime(t, o, mock)

		// Use MockFileSystem
		mfs := &config.MockFileSystem{
			Dirs: map[string]bool{"/project": true},
			WD:   "/project",
		}
		o.fs = mfs
		o.configLoader = config.NewConfigLoaderWithFS(mfs)

		o.out = io.Discard
		o.err = io.Discard

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		done := make(chan struct{})
		var execErr error
		go func() {
			// Simulating "node --interactive=true --image alpine cat"
			execErr = ExecuteWithOptions(ctx, []string{"node", "--interactive=true", "--image", "alpine", "cat"}, o)
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
		o := setupTestOptions(t)
		setupMockRuntime(t, o, mock)

		// Use MockFileSystem
		mfs := &config.MockFileSystem{
			Dirs: map[string]bool{"/project": true},
			WD:   "/project",
			Files: map[string][]byte{
				"/project/.tools.yaml": []byte("node:\n  image: node:20"),
			},
		}
		o.fs = mfs
		o.configLoader = config.NewConfigLoaderWithFS(mfs)

		o.out = io.Discard
		o.err = io.Discard

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		done := make(chan struct{})
		var execErr error
		go func() {
			// Simulating "node --cderun-interactive=true --cderun-image=alpine cat"
			execErr = ExecuteWithOptions(ctx, []string{"node", "--cderun-interactive=true", "--cderun-image=alpine", "cat"}, o)
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
