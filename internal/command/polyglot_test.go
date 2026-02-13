package command

import (
	"bytes"
	"cderun/internal/config"
	"cderun/internal/runtime"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestUnit_Command_Root_PolyglotFlags(t *testing.T) {
	t.Run("flags without cderun-prefix ARE NOT picked up in polyglot mode (specification)", func(t *testing.T) {
		mock := &pipeMockRuntime{}
		mock.CreatedContainerID = "test-container"
		setupMockRuntime(t, &mock.MockRuntime)
		runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
			return mock, nil
		}

		// Use MockFileSystem
		mfs := &config.MockFileSystem{
			Dirs: map[string]bool{"/project": true},
			WD:   "/project",
		}
		opts.fs = mfs
		opts.configLoader = config.NewConfigLoaderWithFS(mfs)

		// Simulate symlink execution: node --interactive=true --image alpine cat
		// Specification: only --cderun- prefixed flags are hoisted.

		var stdout bytes.Buffer
		rootCmd = newRootCmd()
		rootCmd.SetOut(&stdout)
		rootCmd.SetErr(io.Discard)

		done := make(chan struct{})
		var execErr error
		go func() {
			// Simulating "node --interactive=true --image alpine cat"
			execErr = Execute([]string{"node", "--interactive=true", "--image", "alpine", "cat"})
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("Test timed out")
		}

		// It should fail because no image mapping for 'node' exists, and --image was not hoisted.
		assert.Error(t, execErr)
		assert.Contains(t, execErr.Error(), "no image mapping found for tool: node")

		requireConfig := mock.GetCreatedConfig()
		assert.Nil(t, requireConfig)
	})

	t.Run("flags WITH cderun-prefix ARE picked up in polyglot mode", func(t *testing.T) {
		mock := &pipeMockRuntime{}
		mock.CreatedContainerID = "test-container"
		setupMockRuntime(t, &mock.MockRuntime)
		runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
			return mock, nil
		}

		// Use MockFileSystem
		mfs := &config.MockFileSystem{
			Dirs: map[string]bool{"/project": true},
			WD:   "/project",
			Files: map[string][]byte{
				"/project/.tools.yaml": []byte("node:\n  image: node:20"),
			},
		}
		opts.fs = mfs
		opts.configLoader = config.NewConfigLoaderWithFS(mfs)

		rootCmd = newRootCmd()
		rootCmd.SetOut(io.Discard)
		rootCmd.SetErr(io.Discard)

		done := make(chan struct{})
		var execErr error
		go func() {
			// Simulating "node --cderun-interactive=true --cderun-image=alpine cat"
			execErr = Execute([]string{"node", "--cderun-interactive=true", "--cderun-image=alpine", "cat"})
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("Test timed out")
		}

		assert.NoError(t, execErr)

		requireConfig := mock.GetCreatedConfig()
		assert.NotNil(t, requireConfig)
		assert.True(t, requireConfig.Interactive)
		assert.Equal(t, "alpine", requireConfig.Image)
	})
}
