package command

import (
	"bytes"
	"cderun/internal/runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func executeCommand(args ...string) (string, error) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)

	err := Execute(append([]string{"cderun"}, args...))

	return buf.String(), err
}

func TestPreprocessArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected []string
	}{
		{
			name:     "cderun with args",
			args:     []string{"cderun", "node", "--version"},
			expected: []string{"cderun", "node", "--version"},
		},
		{
			name:     "cderun with path",
			args:     []string{"/usr/local/bin/cderun", "node", "--version"},
			expected: []string{"/usr/local/bin/cderun", "node", "--version"},
		},
		{
			name:     "symlink node",
			args:     []string{"node", "--version"},
			expected: []string{"cderun", "node", "--version"},
		},
		{
			name:     "symlink python with path",
			args:     []string{"/usr/bin/python", "-c", "print(1)"},
			expected: []string{"cderun", "python", "-c", "print(1)"},
		},
		{
			name:     "empty args",
			args:     []string{},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := preprocessArgs(tt.args)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestExecuteEmptyArgs(t *testing.T) {
	// Should not panic
	err := Execute([]string{})
	assert.NoError(t, err)

	err = Execute(nil)
	assert.NoError(t, err)
}

func TestRootCmd(t *testing.T) {
	t.Run("executes container correctly", func(t *testing.T) {
		// Save and restore package-level state
		oldTTY := tty
		oldInteractive := interactive
		oldNetwork := network
		oldMountSocket := mountSocket
		oldMountCderun := mountCderun
		oldFactory := runtimeFactory
		oldExit := exitFunc
		t.Cleanup(func() {
			tty = oldTTY
			interactive = oldInteractive
			network = oldNetwork
			mountSocket = oldMountSocket
			mountCderun = oldMountCderun
			runtimeFactory = oldFactory
			exitFunc = oldExit
		})

		// Prepare mock runtime
		mockRuntime := &runtime.MockRuntime{
			CreatedContainerID: "test-container-id",
			ExitCode:           0,
		}
		runtimeFactory = func(socket string) (runtime.ContainerRuntime, error) {
			return mockRuntime, nil
		}
		var capturedExitCode int
		exitFunc = func(code int) {
			capturedExitCode = code
		}

		_, err := executeCommand("--tty", "-i", "--network", "host", "node", "--version")
		assert.NoError(t, err)

		assert.NotNil(t, mockRuntime.CreatedConfig)
		assert.Equal(t, "alpine:latest", mockRuntime.CreatedConfig.Image)
		assert.Equal(t, []string{"node"}, mockRuntime.CreatedConfig.Command)
		assert.Equal(t, []string{"--version"}, mockRuntime.CreatedConfig.Args)
		assert.True(t, mockRuntime.CreatedConfig.TTY)
		assert.True(t, mockRuntime.CreatedConfig.Interactive)
		assert.Equal(t, "host", mockRuntime.CreatedConfig.Network)
		assert.Equal(t, "test-container-id", mockRuntime.StartedContainerID)
		assert.Equal(t, "test-container-id", mockRuntime.WaitedContainerID)
		assert.Equal(t, "test-container-id", mockRuntime.RemovedContainerID)
		assert.Equal(t, 0, capturedExitCode)
	})

	t.Run("shows help when no subcommand is provided", func(t *testing.T) {
		// Save and restore package-level state
		oldTTY := tty
		oldInteractive := interactive
		oldNetwork := network
		oldMountSocket := mountSocket
		oldMountCderun := mountCderun
		oldFactory := runtimeFactory
		oldExit := exitFunc
		t.Cleanup(func() {
			tty = oldTTY
			interactive = oldInteractive
			network = oldNetwork
			mountSocket = oldMountSocket
			mountCderun = oldMountCderun
			runtimeFactory = oldFactory
			exitFunc = oldExit
		})

		// Prepare mock runtime
		runtimeFactory = func(socket string) (runtime.ContainerRuntime, error) {
			return &runtime.MockRuntime{}, nil
		}
		exitFunc = func(code int) {}

		output, err := executeCommand("--tty")
		assert.NoError(t, err)

		assert.True(t, strings.HasPrefix(output, "cderun is a CLI wrapper tool"))
		assert.Contains(t, output, "Usage:")
	})

	t.Run("handles symlink execution via Execute", func(t *testing.T) {
		// Save and restore package-level state
		oldTTY := tty
		oldInteractive := interactive
		oldNetwork := network
		oldMountSocket := mountSocket
		oldMountCderun := mountCderun
		oldFactory := runtimeFactory
		oldExit := exitFunc
		t.Cleanup(func() {
			tty = oldTTY
			interactive = oldInteractive
			network = oldNetwork
			mountSocket = oldMountSocket
			mountCderun = oldMountCderun
			runtimeFactory = oldFactory
			exitFunc = oldExit
		})

		// Prepare mock runtime
		mockRuntime := &runtime.MockRuntime{
			CreatedContainerID: "test-container-id",
			ExitCode:           0,
		}
		runtimeFactory = func(socket string) (runtime.ContainerRuntime, error) {
			return mockRuntime, nil
		}
		exitFunc = func(code int) {}

		buf := new(bytes.Buffer)
		rootCmd.SetOut(buf)
		rootCmd.SetErr(buf)

		err := Execute([]string{"node", "--version"})

		assert.NoError(t, err)
		assert.Equal(t, []string{"node"}, mockRuntime.CreatedConfig.Command)
		assert.Equal(t, []string{"--version"}, mockRuntime.CreatedConfig.Args)
	})
}
