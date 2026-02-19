package command

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"cderun/internal/config"
	"cderun/internal/runtime"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Command_Root_PolyglotFlags(t *testing.T) {
	t.Run("flags without cderun-prefix ARE NOT picked up in polyglot mode (specification)", func(t *testing.T) {
		mock := &pipeMockRuntime{}
		mock.CreatedContainerID = "test-container"

		mfs := &config.MockFileSystem{
			Dirs: map[string]bool{"/project": true},
			WD:   "/project",
		}

		var stdout bytes.Buffer
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		done := make(chan struct{})
		var execErr error
		go func() {
			execErr = ExecuteContextWithOptions(ctx, []string{"node", "--interactive=true", "--image", "alpine", "cat"}, func(o *rootOptions) {
				o.fs = mfs
				o.configLoader = config.NewConfigLoaderWithFS(mfs)
				o.out = &stdout
				o.err = io.Discard
				o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
					return mock, nil
				}
				o.exitFunc = func(code int) {}
			})
			close(done)
		}()

		select {
		case <-done:
		case <-ctx.Done():
			t.Fatal("Test timed out")
		}

		require.Error(t, execErr)
		assert.Contains(t, execErr.Error(), "no image mapping found for tool: node")
		assert.Nil(t, mock.GetCreatedConfig())
	})

	t.Run("flags WITH cderun-prefix ARE picked up in polyglot mode", func(t *testing.T) {
		mock := &pipeMockRuntime{}
		mock.CreatedContainerID = "test-container"

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
			execErr = ExecuteContextWithOptions(ctx, []string{"node", "--cderun-interactive=true", "--cderun-image=alpine", "cat"}, func(o *rootOptions) {
				o.fs = mfs
				o.configLoader = config.NewConfigLoaderWithFS(mfs)
				o.out = io.Discard
				o.err = io.Discard
				o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
					return mock, nil
				}
				o.exitFunc = func(code int) {}
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
