package command

import (
	"bytes"
	"cderun/internal/runtime"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func executeCommand(args ...string) (string, error) {
	return executeCommandRaw(append([]string{"cderun"}, args...))
}

func executeCommandRaw(args []string) (string, error) {
	// Reset flags Changed state
	rootCmd.Flags().VisitAll(func(f *pflag.Flag) {
		f.Changed = false
	})
	rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		f.Changed = false
	})

	oldStdout := os.Stdout
	oldStderr := os.Stderr
	oldOut := rootCmd.OutOrStdout()
	oldErr := rootCmd.ErrOrStderr()
	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
		rootCmd.SetOut(oldOut)
		rootCmd.SetErr(oldErr)
	}()

	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}
	defer r.Close()
	defer w.Close()

	os.Stdout = w
	os.Stderr = w
	rootCmd.SetOut(w)
	rootCmd.SetErr(w)

	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(&buf, r)
		close(done)
	}()

	execErr := Execute(args)

	_ = w.Close()
	<-done

	return buf.String(), execErr
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
		oldImage := image
		oldRemove := remove
		oldCderunTTY := cderunTTY
		oldCderunInteractive := cderunInteractive
		oldRuntimeName := runtimeName
		oldFactory := runtimeFactory
		oldExit := exitFunc
		t.Cleanup(func() {
			tty = oldTTY
			interactive = oldInteractive
			network = oldNetwork
			mountSocket = oldMountSocket
			mountCderun = oldMountCderun
			image = oldImage
			remove = oldRemove
			cderunTTY = oldCderunTTY
			cderunInteractive = oldCderunInteractive
			runtimeName = oldRuntimeName
			runtimeFactory = oldFactory
			exitFunc = oldExit
		})

		// Prepare mock runtime
		mockRuntime := &runtime.MockRuntime{
			CreatedContainerID: "test-container-id",
			ExitCode:           0,
		}
		runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
			return mockRuntime, nil
		}
		var capturedExitCode int
		exitFunc = func(code int) {
			capturedExitCode = code
		}

		_, err := executeCommand("--image", "node:20-alpine", "--tty", "-i", "--network", "host", "node", "--version")
		assert.NoError(t, err)

		assert.NotNil(t, mockRuntime.CreatedConfig)
		assert.Equal(t, "node:20-alpine", mockRuntime.CreatedConfig.Image)
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
		oldImage := image
		oldRemove := remove
		oldCderunTTY := cderunTTY
		oldCderunInteractive := cderunInteractive
		oldRuntimeName := runtimeName
		oldFactory := runtimeFactory
		oldExit := exitFunc
		t.Cleanup(func() {
			tty = oldTTY
			interactive = oldInteractive
			network = oldNetwork
			mountSocket = oldMountSocket
			mountCderun = oldMountCderun
			image = oldImage
			remove = oldRemove
			cderunTTY = oldCderunTTY
			cderunInteractive = oldCderunInteractive
			runtimeName = oldRuntimeName
			runtimeFactory = oldFactory
			exitFunc = oldExit
		})

		// Prepare mock runtime
		runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
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
		oldImage := image
		oldRemove := remove
		oldCderunTTY := cderunTTY
		oldCderunInteractive := cderunInteractive
		oldRuntimeName := runtimeName
		oldFactory := runtimeFactory
		oldExit := exitFunc
		t.Cleanup(func() {
			tty = oldTTY
			interactive = oldInteractive
			network = oldNetwork
			mountSocket = oldMountSocket
			mountCderun = oldMountCderun
			image = oldImage
			remove = oldRemove
			cderunTTY = oldCderunTTY
			cderunInteractive = oldCderunInteractive
			runtimeName = oldRuntimeName
			runtimeFactory = oldFactory
			exitFunc = oldExit
		})

		// Use a temporary directory for this test
		oldWd, err := os.Getwd()
		require.NoError(t, err)
		tmpDir := t.TempDir()
		require.NoError(t, os.Chdir(tmpDir))
		t.Cleanup(func() { os.Chdir(oldWd) })

		// Create a temporary .tools.yaml for image mapping
		toolsContent := `
node:
  image: node:20-alpine
`
		err = os.WriteFile(".tools.yaml", []byte(toolsContent), 0644)
		require.NoError(t, err)

		// Prepare mock runtime
		mockRuntime := &runtime.MockRuntime{
			CreatedContainerID: "test-container-id",
			ExitCode:           0,
		}
		runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
			return mockRuntime, nil
		}
		exitFunc = func(code int) {}

		_, err = executeCommandRaw([]string{"node", "--version"})

		assert.NoError(t, err)
		assert.Equal(t, "node:20-alpine", mockRuntime.CreatedConfig.Image)
		assert.Equal(t, []string{"node"}, mockRuntime.CreatedConfig.Command)
		assert.Equal(t, []string{"--version"}, mockRuntime.CreatedConfig.Args)
	})

	t.Run("resolves all settings from tools.yaml", func(t *testing.T) {
		// Save and restore package-level state
		oldRuntimeName := runtimeName
		oldFactory := runtimeFactory
		oldExit := exitFunc
		t.Cleanup(func() {
			runtimeName = oldRuntimeName
			runtimeFactory = oldFactory
			exitFunc = oldExit
		})

		// Reset flags Changed state
		rootCmd.Flags().VisitAll(func(f *pflag.Flag) { f.Changed = false })
		rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) { f.Changed = false })

		// Use a temporary directory for this test
		oldWd, err := os.Getwd()
		require.NoError(t, err)
		tmpDir := t.TempDir()
		require.NoError(t, os.Chdir(tmpDir))
		t.Cleanup(func() { os.Chdir(oldWd) })

		toolsContent := `
node:
  image: node:20-alpine
  tty: true
  network: host
  env:
    - KEY=VALUE
  volumes:
    - /host:/container
`
		err = os.WriteFile(".tools.yaml", []byte(toolsContent), 0644)
		require.NoError(t, err)

		mockRuntime := &runtime.MockRuntime{}
		runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
			return mockRuntime, nil
		}
		exitFunc = func(code int) {}

		_, err = executeCommand("node", "app.js")
		assert.NoError(t, err)

		require.NotNil(t, mockRuntime.CreatedConfig)
		assert.Equal(t, "node:20-alpine", mockRuntime.CreatedConfig.Image)
		assert.True(t, mockRuntime.CreatedConfig.TTY)
		assert.Equal(t, "host", mockRuntime.CreatedConfig.Network)
		assert.Contains(t, mockRuntime.CreatedConfig.Env, "KEY=VALUE")
		assert.Len(t, mockRuntime.CreatedConfig.Volumes, 1)
		assert.Equal(t, "/host", mockRuntime.CreatedConfig.Volumes[0].HostPath)
		assert.Equal(t, "/container", mockRuntime.CreatedConfig.Volumes[0].ContainerPath)
	})

	t.Run("P3 environment variable takes priority over tools.yaml", func(t *testing.T) {
		// Save and restore package-level state
		oldRuntimeName := runtimeName
		oldFactory := runtimeFactory
		oldExit := exitFunc
		t.Cleanup(func() {
			runtimeName = oldRuntimeName
			runtimeFactory = oldFactory
			exitFunc = oldExit
		})

		// Reset flags
		rootCmd.Flags().VisitAll(func(f *pflag.Flag) { f.Changed = false })
		rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) { f.Changed = false })

		t.Setenv("CDERUN_IMAGE", "env-image:latest")

		// Use a temporary directory for this test
		oldWd, err := os.Getwd()
		require.NoError(t, err)
		tmpDir := t.TempDir()
		require.NoError(t, os.Chdir(tmpDir))
		t.Cleanup(func() { os.Chdir(oldWd) })

		toolsContent := `
node:
  image: node:20-alpine
`
		err = os.WriteFile(".tools.yaml", []byte(toolsContent), 0644)
		require.NoError(t, err)

		mockRuntime := &runtime.MockRuntime{}
		runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
			return mockRuntime, nil
		}
		exitFunc = func(code int) {}

		_, err = executeCommand("node", "app.js")
		assert.NoError(t, err)
		assert.Equal(t, "env-image:latest", mockRuntime.CreatedConfig.Image)
	})

	t.Run("P1 override takes priority over P2 CLI", func(t *testing.T) {
		// Save and restore package-level state
		oldRuntimeName := runtimeName
		oldFactory := runtimeFactory
		oldExit := exitFunc
		t.Cleanup(func() {
			runtimeName = oldRuntimeName
			runtimeFactory = oldFactory
			exitFunc = oldExit
		})

		// Reset flags
		rootCmd.Flags().VisitAll(func(f *pflag.Flag) { f.Changed = false })
		rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) { f.Changed = false })

		mockRuntime := &runtime.MockRuntime{}
		runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
			return mockRuntime, nil
		}
		exitFunc = func(code int) {}

		_, err := executeCommand("--image", "alpine", "--tty=true", "--cderun-tty=false", "sh")
		assert.NoError(t, err)
		assert.False(t, mockRuntime.CreatedConfig.TTY)
	})

	t.Run("returns error for unsupported runtime", func(t *testing.T) {
		// Save and restore package-level state
		oldRuntimeName := runtimeName
		oldFactory := runtimeFactory
		oldExit := exitFunc
		t.Cleanup(func() {
			runtimeName = oldRuntimeName
			runtimeFactory = oldFactory
			exitFunc = oldExit
		})

		// Reset flags
		rootCmd.Flags().VisitAll(func(f *pflag.Flag) { f.Changed = false })
		rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) { f.Changed = false })

		// Use the real runtimeFactory here to test the validation logic
		exitFunc = func(code int) {}

		_, err := executeCommand("--image", "alpine", "--runtime", "invalid", "sh")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported runtime \"invalid\"")
	})

	t.Run("returns error for podman (not implemented yet)", func(t *testing.T) {
		// Save and restore package-level state
		oldRuntimeName := runtimeName
		oldFactory := runtimeFactory
		oldExit := exitFunc
		t.Cleanup(func() {
			runtimeName = oldRuntimeName
			runtimeFactory = oldFactory
			exitFunc = oldExit
		})

		// Reset flags
		rootCmd.Flags().VisitAll(func(f *pflag.Flag) { f.Changed = false })
		rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) { f.Changed = false })

		// Use the real runtimeFactory
		exitFunc = func(code int) {}

		_, err := executeCommand("--image", "alpine", "--runtime", "podman", "sh")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "podman runtime is not implemented yet")
	})

	t.Run("dry-run outputs configuration and skips execution", func(t *testing.T) {
		// Save and restore package-level state
		oldDryRun := dryRun
		oldDryRunFormat := dryRunFormat
		oldRuntimeName := runtimeName
		oldFactory := runtimeFactory
		oldExit := exitFunc
		t.Cleanup(func() {
			dryRun = oldDryRun
			dryRunFormat = oldDryRunFormat
			runtimeName = oldRuntimeName
			runtimeFactory = oldFactory
			exitFunc = oldExit
		})

		// Reset flags
		rootCmd.Flags().VisitAll(func(f *pflag.Flag) { f.Changed = false })
		rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) { f.Changed = false })

		mockRuntime := &runtime.MockRuntime{}
		runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
			return mockRuntime, nil
		}
		exitFunc = func(code int) {}

		// Dry-run with YAML (default)
		output, err := executeCommand("--dry-run", "--image", "alpine", "sh")
		assert.NoError(t, err)
		assert.Contains(t, output, "image: alpine")
		assert.Contains(t, output, "command:")
		assert.Contains(t, output, "- sh")
		assert.Nil(t, mockRuntime.CreatedConfig, "Runtime should not be called in dry-run mode")

		// Dry-run with JSON
		output, err = executeCommand("--dry-run", "--dry-run-format", "json", "--image", "alpine", "sh")
		assert.NoError(t, err)
		assert.Contains(t, output, "\"image\": \"alpine\"")
		assert.Contains(t, output, "\"command\": [")

		// Dry-run with simple
		output, err = executeCommand("--dry-run", "-f", "simple", "--image", "alpine", "sh")
		assert.NoError(t, err)
		assert.Contains(t, output, "Image: alpine")
		assert.Contains(t, output, "Command: sh")
	})
}

func TestCderunInternalOverrides(t *testing.T) {
	// Use a temporary directory for this test
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	tmpDir := t.TempDir()
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() { os.Chdir(oldWd) })

	// Create a temporary .tools.yaml for image mapping
	toolsContent := `
node:
  image: node:20-alpine
`
	err = os.WriteFile(".tools.yaml", []byte(toolsContent), 0644)
	require.NoError(t, err)

	// Save and restore package-level state
	oldTTY := tty
	oldCderunTTY := cderunTTY
	oldFactory := runtimeFactory
	oldExit := exitFunc
	t.Cleanup(func() {
		tty = oldTTY
		cderunTTY = oldCderunTTY
		runtimeFactory = oldFactory
		exitFunc = oldExit
	})

	mockRuntime := &runtime.MockRuntime{}
	runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
		return mockRuntime, nil
	}
	exitFunc = func(code int) {}

	t.Run("cderun-tty overrides tty even if placed after subcommand", func(t *testing.T) {
		// Reset flags Changed state
		rootCmd.Flags().VisitAll(func(f *pflag.Flag) { f.Changed = false })
		rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) { f.Changed = false })

		// cderun --tty=true node --cderun-tty=false --version
		// We use a path that doesn't end in "cderun" for polyglot test later,
		// but here we use "cderun" explicitly.
		_, err := executeCommandRaw([]string{"cderun", "--tty=true", "node", "--cderun-tty=false", "--version"})
		assert.NoError(t, err)

		require.NotNil(t, mockRuntime.CreatedConfig)
		assert.False(t, mockRuntime.CreatedConfig.TTY, "TTY should be false because --cderun-tty=false overrides --tty=true")
	})

	t.Run("cderun-tty works in polyglot mode", func(t *testing.T) {
		// Reset flags Changed state
		rootCmd.Flags().VisitAll(func(f *pflag.Flag) { f.Changed = false })
		rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) { f.Changed = false })
		mockRuntime.CreatedConfig = nil

		// node --cderun-tty=true --version
		_, err := executeCommandRaw([]string{"node", "--cderun-tty=true", "--version"})
		assert.NoError(t, err)

		require.NotNil(t, mockRuntime.CreatedConfig)
		assert.True(t, mockRuntime.CreatedConfig.TTY, "TTY should be true because --cderun-tty=true was provided")
	})
}
