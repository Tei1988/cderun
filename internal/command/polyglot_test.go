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
	t.Run("flags without cderun-prefix ARE picked up in polyglot mode (hoisting)", func(t *testing.T) {
		mock := &pipeMockRuntime{}
		mock.CreatedContainerID = "test-container"
		setupMockRuntime(t, &mock.MockRuntime)
		runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
			return mock, nil
		}

		// Simulate symlink execution: node --interactive=true --image alpine cat
		// preprocessArgs should now hoist these even without prefix.

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

		assert.NoError(t, execErr)

		requireConfig := mock.GetCreatedConfig()
		assert.NotNil(t, requireConfig)
		assert.True(t, requireConfig.Interactive)
		assert.Equal(t, "alpine", requireConfig.Image)
		assert.Equal(t, []string{"cat"}, requireConfig.Command)
	})

	t.Run("flags WITH cderun-prefix ARE picked up in polyglot mode", func(t *testing.T) {
		mock := &pipeMockRuntime{}
		mock.CreatedContainerID = "test-container"
		setupMockRuntime(t, &mock.MockRuntime)
		runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
			return mock, nil
		}

		// Setup a tool mapping for 'node' so it doesn't fail
		err := os.WriteFile(".tools.yaml", []byte("node:\n  image: node:20"), 0644)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Remove(".tools.yaml") })

		rootCmd = newRootCmd()
		rootCmd.SetOut(io.Discard)
		rootCmd.SetErr(io.Discard)

		done := make(chan struct{})
		go func() {
			// Simulating "node --cderun-interactive=true --cderun-image=alpine cat"
			_ = Execute([]string{"node", "--cderun-interactive=true", "--cderun-image=alpine", "cat"})
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("Test timed out")
		}

		requireConfig := mock.GetCreatedConfig()
		assert.NotNil(t, requireConfig)
		assert.True(t, requireConfig.Interactive)
		assert.Equal(t, "alpine", requireConfig.Image)
	})
}
