package command

import (
	"bytes"
	"cderun/internal/runtime"
	"io"
	"os"
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

		// Use a temporary directory for this test
		restoreWd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		tmpDir := t.TempDir()
		if err := os.Chdir(tmpDir); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chdir(restoreWd) })

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

		// Use a temporary directory for this test
		restoreWd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		tmpDir := t.TempDir()
		if err := os.Chdir(tmpDir); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chdir(restoreWd) })

		// Setup a tool mapping for 'node' so it doesn't fail
		err = os.WriteFile(".tools.yaml", []byte("node:\n  image: node:20"), 0644)
		if err != nil {
			t.Fatal(err)
		}

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
