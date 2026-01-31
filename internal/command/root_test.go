package command

import (
	"bytes"
	"cderun/internal/runtime"
	"errors"
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
	// Reset flag variables and Changed state
	opts.tty = false
	opts.interactive = false
	opts.network = "bridge"
	opts.mountSocket = ""
	opts.mountCderun = false
	opts.image = ""
	opts.remove = true
	opts.cderunTTY = false
	opts.cderunInteractive = false
	opts.cderunImage = ""
	opts.cderunNetwork = ""
	opts.cderunRemove = true
	opts.cderunRuntime = ""
	opts.cderunMountSocket = ""
	opts.cderunWorkdir = ""
	opts.cderunVolumes = nil
	opts.cderunMountCderun = false
	opts.cderunMountTools = ""
	opts.cderunMountAllTools = false
	opts.runtimeName = "docker"
	opts.env = nil
	opts.cderunEnv = nil
	opts.workdir = ""
	opts.volumes = nil
	opts.mountTools = ""
	opts.mountAllTools = false
	opts.dryRun = false
	opts.dryRunFormat = "yaml"
	opts.cderunDryRun = false
	opts.cderunDryRunFormat = ""
	opts.logLevel = ""
	opts.logFile = ""
	opts.logFormat = "text"
	opts.logTee = false
	opts.logTimestamp = true
	opts.verbose = 0
	opts.cderunLogLevel = ""
	opts.cderunLogFile = ""
	opts.cderunLogFormat = ""
	opts.cderunLogTee = false
	opts.cderunVerbose = 0

	rootCmd.Flags().VisitAll(func(f *pflag.Flag) {
		f.Changed = false
		// Also reset default values in pflag if needed, but manual reset above is safer
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
			actual, err := preprocessArgs(tt.args)
			assert.NoError(t, err)
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
		oldTTY := opts.tty
		oldInteractive := opts.interactive
		oldNetwork := opts.network
		oldMountSocket := opts.mountSocket
		oldMountCderun := opts.mountCderun
		oldImage := opts.image
		oldRemove := opts.remove
		oldCderunTTY := opts.cderunTTY
		oldCderunInteractive := opts.cderunInteractive
		oldRuntimeName := opts.runtimeName
		oldFactory := runtimeFactory
		oldExit := exitFunc
		t.Cleanup(func() {
			opts.tty = oldTTY
			opts.interactive = oldInteractive
			opts.network = oldNetwork
			opts.mountSocket = oldMountSocket
			opts.mountCderun = oldMountCderun
			opts.image = oldImage
			opts.remove = oldRemove
			opts.cderunTTY = oldCderunTTY
			opts.cderunInteractive = oldCderunInteractive
			opts.runtimeName = oldRuntimeName
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
		oldTTY := opts.tty
		oldInteractive := opts.interactive
		oldNetwork := opts.network
		oldMountSocket := opts.mountSocket
		oldMountCderun := opts.mountCderun
		oldImage := opts.image
		oldRemove := opts.remove
		oldCderunTTY := opts.cderunTTY
		oldCderunInteractive := opts.cderunInteractive
		oldRuntimeName := opts.runtimeName
		oldFactory := runtimeFactory
		oldExit := exitFunc
		t.Cleanup(func() {
			opts.tty = oldTTY
			opts.interactive = oldInteractive
			opts.network = oldNetwork
			opts.mountSocket = oldMountSocket
			opts.mountCderun = oldMountCderun
			opts.image = oldImage
			opts.remove = oldRemove
			opts.cderunTTY = oldCderunTTY
			opts.cderunInteractive = oldCderunInteractive
			opts.runtimeName = oldRuntimeName
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
		oldTTY := opts.tty
		oldInteractive := opts.interactive
		oldNetwork := opts.network
		oldMountSocket := opts.mountSocket
		oldMountCderun := opts.mountCderun
		oldImage := opts.image
		oldRemove := opts.remove
		oldCderunTTY := opts.cderunTTY
		oldCderunInteractive := opts.cderunInteractive
		oldRuntimeName := opts.runtimeName
		oldFactory := runtimeFactory
		oldExit := exitFunc
		t.Cleanup(func() {
			opts.tty = oldTTY
			opts.interactive = oldInteractive
			opts.network = oldNetwork
			opts.mountSocket = oldMountSocket
			opts.mountCderun = oldMountCderun
			opts.image = oldImage
			opts.remove = oldRemove
			opts.cderunTTY = oldCderunTTY
			opts.cderunInteractive = oldCderunInteractive
			opts.runtimeName = oldRuntimeName
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
		oldRuntimeName := opts.runtimeName
		oldFactory := runtimeFactory
		oldExit := exitFunc
		t.Cleanup(func() {
			opts.runtimeName = oldRuntimeName
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
		oldRuntimeName := opts.runtimeName
		oldFactory := runtimeFactory
		oldExit := exitFunc
		t.Cleanup(func() {
			opts.runtimeName = oldRuntimeName
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
		oldRuntimeName := opts.runtimeName
		oldFactory := runtimeFactory
		oldExit := exitFunc
		t.Cleanup(func() {
			opts.runtimeName = oldRuntimeName
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
		oldRuntimeName := opts.runtimeName
		oldFactory := runtimeFactory
		oldExit := exitFunc
		t.Cleanup(func() {
			opts.runtimeName = oldRuntimeName
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
		oldRuntimeName := opts.runtimeName
		oldFactory := runtimeFactory
		oldExit := exitFunc
		t.Cleanup(func() {
			opts.runtimeName = oldRuntimeName
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

	t.Run("environment variable pass-through and P1 overrides", func(t *testing.T) {
		// Save and restore package-level state
		oldEnv := opts.env
		oldCderunEnv := opts.cderunEnv
		oldRuntimeName := opts.runtimeName
		oldFactory := runtimeFactory
		oldExit := exitFunc
		t.Cleanup(func() {
			opts.env = oldEnv
			opts.cderunEnv = oldCderunEnv
			opts.runtimeName = oldRuntimeName
			runtimeFactory = oldFactory
			exitFunc = oldExit
		})

		// Reset flags
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
  env:
    - TOOL_KEY=TOOL_VALUE
    - OVERRIDE_KEY=TOOL_VALUE
    - P1_OVERRIDE_KEY=TOOL_VALUE
    - HOST_KEY
`
		err = os.WriteFile(".tools.yaml", []byte(toolsContent), 0644)
		require.NoError(t, err)

		t.Setenv("HOST_KEY", "HOST_VALUE")
		t.Setenv("CLI_HOST_KEY", "CLI_HOST_VALUE")

		mockRuntime := &runtime.MockRuntime{}
		runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
			return mockRuntime, nil
		}
		exitFunc = func(code int) {}

		// Execute with CLI overrides and P1 overrides
		// Note: P1 overrides should use --cderun-flag=value format when placed after subcommand
		// to ensure preprocessArgs hoists them correctly as a single unit.
		_, err = executeCommand(
			"--env", "OVERRIDE_KEY=CLI_VALUE",
			"--env", "P1_OVERRIDE_KEY=CLI_VALUE",
			"--env", "CLI_KEY=CLI_VALUE",
			"--env", "CLI_HOST_KEY",
			"node",
			"--cderun-env=P1_OVERRIDE_KEY=P1_VALUE",
			"app.js",
		)
		assert.NoError(t, err)

		require.NotNil(t, mockRuntime.CreatedConfig)
		envs := mockRuntime.CreatedConfig.Env
		assert.Contains(t, envs, "TOOL_KEY=TOOL_VALUE")
		assert.Contains(t, envs, "OVERRIDE_KEY=CLI_VALUE")      // CLI overrides Tool
		assert.Contains(t, envs, "P1_OVERRIDE_KEY=P1_VALUE")    // P1 overrides CLI and Tool
		assert.Contains(t, envs, "HOST_KEY=HOST_VALUE")        // Resolved from host (via Tool config)
		assert.Contains(t, envs, "CLI_KEY=CLI_VALUE")          // CLI explicit
		assert.Contains(t, envs, "CLI_HOST_KEY=CLI_HOST_VALUE") // Resolved from host (via CLI flag)
	})

	t.Run("dry-run outputs configuration and skips execution", func(t *testing.T) {
		// Save and restore package-level state
		oldDryRun := opts.dryRun
		oldDryRunFormat := opts.dryRunFormat
		oldRuntimeName := opts.runtimeName
		oldFactory := runtimeFactory
		oldExit := exitFunc
		t.Cleanup(func() {
			opts.dryRun = oldDryRun
			opts.dryRunFormat = oldDryRunFormat
			opts.runtimeName = oldRuntimeName
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
		assert.NotContains(t, output, "Command: sh ") // Ensure no trailing space
		assert.Contains(t, output, "TTY: false")
		assert.Contains(t, output, "Interactive: false")
		assert.Contains(t, output, "Network: bridge")
		assert.Contains(t, output, "Remove: true")
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
	oldTTY := opts.tty
	oldCderunTTY := opts.cderunTTY
	oldFactory := runtimeFactory
	oldExit := exitFunc
	t.Cleanup(func() {
		opts.tty = oldTTY
		opts.cderunTTY = oldCderunTTY
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

	t.Run("cderun internal overrides before subcommand result in error", func(t *testing.T) {
		// Reset flags
		rootCmd.Flags().VisitAll(func(f *pflag.Flag) { f.Changed = false })
		rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) { f.Changed = false })

		// cderun --cderun-image=alpine:latest sh
		_, err := executeCommandRaw([]string{"cderun", "--cderun-image=alpine:latest", "sh"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be placed after the subcommand")
	})

	t.Run("cderun internal overrides after subcommand work correctly", func(t *testing.T) {
		// Reset flags
		rootCmd.Flags().VisitAll(func(f *pflag.Flag) { f.Changed = false })
		rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) { f.Changed = false })
		mockRuntime.CreatedConfig = nil

		// cderun --image=alpine:stable sh --cderun-image=alpine:latest
		_, err := executeCommandRaw([]string{"cderun", "--image=alpine:stable", "sh", "--cderun-image=alpine:latest"})
		assert.NoError(t, err)
		require.NotNil(t, mockRuntime.CreatedConfig)
		assert.Equal(t, "alpine:latest", mockRuntime.CreatedConfig.Image)
	})

	t.Run("cderun internal overrides for network, remove, workdir and volume", func(t *testing.T) {
		// Reset flags
		rootCmd.Flags().VisitAll(func(f *pflag.Flag) { f.Changed = false })
		rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) { f.Changed = false })
		mockRuntime.CreatedConfig = nil

		_, err := executeCommand("--image=alpine", "--network=bridge", "--remove=false", "--workdir=/old", "--volume=/h1:/c1", "sh", "--cderun-network=host", "--cderun-remove=true", "--cderun-workdir=/new", "--cderun-volume=/h2:/c2")
		assert.NoError(t, err)
		require.NotNil(t, mockRuntime.CreatedConfig)
		assert.Equal(t, "host", mockRuntime.CreatedConfig.Network)
		assert.True(t, mockRuntime.CreatedConfig.Remove)
		assert.Equal(t, "/new", mockRuntime.CreatedConfig.Workdir)

		// Volumes should be merged (P1 added after P2)
		assert.Len(t, mockRuntime.CreatedConfig.Volumes, 2)
		assert.Equal(t, "/h1", mockRuntime.CreatedConfig.Volumes[0].HostPath)
		assert.Equal(t, "/h2", mockRuntime.CreatedConfig.Volumes[1].HostPath)
	})

	t.Run("cderun internal overrides for runtime, socket and mounting", func(t *testing.T) {
		// Reset flags
		rootCmd.Flags().VisitAll(func(f *pflag.Flag) { f.Changed = false })
		rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) { f.Changed = false })
		mockRuntime.CreatedConfig = nil

		// Setup tools config for mount-tools
		oldWd, _ := os.Getwd()
		tmpDir := t.TempDir()
		os.Chdir(tmpDir)
		t.Cleanup(func() { os.Chdir(oldWd) })
		os.WriteFile(".tools.yaml", []byte("node:\n  image: node:20"), 0644)

		_, err := executeCommand("--image=alpine", "sh", "--cderun-runtime=docker", "--cderun-mount-socket=/var/run/custom.sock", "--cderun-mount-cderun=true", "--cderun-mount-tools=node")
		assert.NoError(t, err)
		require.NotNil(t, mockRuntime.CreatedConfig)

		// runtimeFactory is called with resolved runtime and socket
		// Wait, I need to check if runtimeFactory was called with correct args.
		// Actually I can't easily check runtimeFactory calls without a spy.
		// But I can check if volumes contain the custom socket.

		socketFound := false
		cderunFound := false
		nodeFound := false
		for _, v := range mockRuntime.CreatedConfig.Volumes {
			if v.HostPath == "/var/run/custom.sock" {
				socketFound = true
			}
			if v.ContainerPath == "/usr/local/bin/cderun" {
				cderunFound = true
			}
			if v.ContainerPath == "/usr/local/bin/node" {
				nodeFound = true
			}
		}
		assert.True(t, socketFound)
		assert.True(t, cderunFound)
		assert.True(t, nodeFound)
	})

	t.Run("cderun internal override can turn off remove", func(t *testing.T) {
		// Reset flags
		rootCmd.Flags().VisitAll(func(f *pflag.Flag) { f.Changed = false })
		rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) { f.Changed = false })
		mockRuntime.CreatedConfig = nil

		_, err := executeCommand("--image=alpine", "--remove=true", "sh", "--cderun-remove=false")
		assert.NoError(t, err)
		require.NotNil(t, mockRuntime.CreatedConfig)
		assert.False(t, mockRuntime.CreatedConfig.Remove)
	})

	t.Run("cderun internal overrides for dry-run", func(t *testing.T) {
		// Reset flags
		rootCmd.Flags().VisitAll(func(f *pflag.Flag) { f.Changed = false })
		rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) { f.Changed = false })
		mockRuntime.CreatedConfig = nil

		// cderun --image=alpine sh --cderun-dry-run --cderun-dry-run-format=simple
		output, err := executeCommandRaw([]string{"cderun", "--image=alpine", "sh", "--cderun-dry-run", "--cderun-dry-run-format=simple"})
		assert.NoError(t, err)
		assert.Nil(t, mockRuntime.CreatedConfig)
		assert.Contains(t, output, "Image: alpine")
		assert.Contains(t, output, "Command: sh")
	})
}

func TestPhase3Features(t *testing.T) {
	// Save and restore package-level state
	oldFactory := runtimeFactory
	oldExit := exitFunc
	t.Cleanup(func() {
		runtimeFactory = oldFactory
		exitFunc = oldExit
	})

	mockRuntime := &runtime.MockRuntime{}
	runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
		return mockRuntime, nil
	}
	exitFunc = func(code int) {}

	t.Run("workdir and volume flags", func(t *testing.T) {
		rootCmd.Flags().VisitAll(func(f *pflag.Flag) { f.Changed = false })
		rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) { f.Changed = false })
		mockRuntime.CreatedConfig = nil

		_, err := executeCommand("--image", "alpine", "--workdir", "/my/workdir", "--volume", "/h:/c:ro", "sh")
		assert.NoError(t, err)

		require.NotNil(t, mockRuntime.CreatedConfig)
		assert.Equal(t, "/my/workdir", mockRuntime.CreatedConfig.Workdir)
		require.Len(t, mockRuntime.CreatedConfig.Volumes, 1)
		assert.Equal(t, "/h", mockRuntime.CreatedConfig.Volumes[0].HostPath)
		assert.Equal(t, "/c", mockRuntime.CreatedConfig.Volumes[0].ContainerPath)
		assert.True(t, mockRuntime.CreatedConfig.Volumes[0].ReadOnly)
	})

	t.Run("mounting flags require explicit cderun socket settings", func(t *testing.T) {
		rootCmd.Flags().VisitAll(func(f *pflag.Flag) { f.Changed = false })
		rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) { f.Changed = false })

		// DOCKER_HOST should no longer be enough for SocketSet
		t.Setenv("DOCKER_HOST", "/var/run/docker.sock")
		t.Setenv("CDERUN_MOUNT_SOCKET", "")

		_, err := executeCommand("--image", "alpine", "--mount-cderun", "sh")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "requires --mount-socket")

		// CDERUN_MOUNT_SOCKET should work
		t.Setenv("CDERUN_MOUNT_SOCKET", "/var/run/docker.sock")
		_, err = executeCommand("--image", "alpine", "--mount-cderun", "sh")
		assert.NoError(t, err)
	})

	t.Run("mount-cderun logic", func(t *testing.T) {
		rootCmd.Flags().VisitAll(func(f *pflag.Flag) { f.Changed = false })
		rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) { f.Changed = false })
		mockRuntime.CreatedConfig = nil

		_, err := executeCommand("--image", "alpine", "--mount-cderun", "--mount-socket", "/socket", "sh")
		assert.NoError(t, err)

		require.NotNil(t, mockRuntime.CreatedConfig)
		exePath, _ := os.Executable()

		binaryFound := false
		socketFound := false
		for _, v := range mockRuntime.CreatedConfig.Volumes {
			if v.HostPath == exePath && v.ContainerPath == "/usr/local/bin/cderun" {
				binaryFound = true
			}
			if v.HostPath == "/socket" && v.ContainerPath == "/socket" {
				socketFound = true
			}
		}
		assert.True(t, binaryFound, "binary should be mounted")
		assert.True(t, socketFound, "socket should be mounted")
	})

	t.Run("mount-tools logic", func(t *testing.T) {
		rootCmd.Flags().VisitAll(func(f *pflag.Flag) { f.Changed = false })
		rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) { f.Changed = false })
		mockRuntime.CreatedConfig = nil

		// Setup tools config
		oldWd, _ := os.Getwd()
		tmpDir := t.TempDir()
		os.Chdir(tmpDir)
		t.Cleanup(func() { os.Chdir(oldWd) })

		toolsContent := `
node:
  image: node:20
python:
  image: python:3
sh:
  image: alpine
`
		os.WriteFile(".tools.yaml", []byte(toolsContent), 0644)

		_, err := executeCommand("--mount-tools", "node", "--mount-socket", "/socket", "sh")
		assert.NoError(t, err)

		require.NotNil(t, mockRuntime.CreatedConfig)
		exePath, _ := os.Executable()

		nodeFound := false
		pythonFound := false
		for _, v := range mockRuntime.CreatedConfig.Volumes {
			if v.HostPath == exePath && v.ContainerPath == "/usr/local/bin/node" {
				nodeFound = true
			}
			if v.HostPath == exePath && v.ContainerPath == "/usr/local/bin/python" {
				pythonFound = true
			}
		}
		assert.True(t, nodeFound, "node should be mounted")
		assert.False(t, pythonFound, "python should NOT be mounted")

		// Test mount-all-tools
		rootCmd.Flags().VisitAll(func(f *pflag.Flag) { f.Changed = false })
		rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) { f.Changed = false })
		mockRuntime.CreatedConfig = nil

		_, err = executeCommand("--mount-all-tools", "--mount-socket", "/socket", "sh")
		assert.NoError(t, err)

		nodeFound = false
		pythonFound = false
		for _, v := range mockRuntime.CreatedConfig.Volumes {
			if v.HostPath == exePath && v.ContainerPath == "/usr/local/bin/node" {
				nodeFound = true
			}
			if v.HostPath == exePath && v.ContainerPath == "/usr/local/bin/python" {
				pythonFound = true
			}
		}
		assert.True(t, nodeFound, "node should be mounted")
		assert.True(t, pythonFound, "python should be mounted")
	})

	t.Run("mount-all-tools with empty config shows warning", func(t *testing.T) {
		rootCmd.Flags().VisitAll(func(f *pflag.Flag) { f.Changed = false })
		rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) { f.Changed = false })
		mockRuntime.CreatedConfig = nil

		// Setup empty tools config
		oldWd, _ := os.Getwd()
		tmpDir := t.TempDir()
		os.Chdir(tmpDir)
		t.Cleanup(func() { os.Chdir(oldWd) })

		// No .tools.yaml created

		output, err := executeCommand("--mount-all-tools", "--mount-socket", "/socket", "--image", "alpine", "sh")
		assert.NoError(t, err)
		assert.Contains(t, output, "[WARN] --mount-all-tools specified but no tools defined in .tools.yaml")
	})
}

func TestRemoveContainerWarning(t *testing.T) {
	t.Run("prints warning if RemoveContainer fails", func(t *testing.T) {
		// Save and restore package-level state
		oldFactory := runtimeFactory
		oldExit := exitFunc
		t.Cleanup(func() {
			runtimeFactory = oldFactory
			exitFunc = oldExit
		})

		// Reset flags
		rootCmd.Flags().VisitAll(func(f *pflag.Flag) { f.Changed = false })
		rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) { f.Changed = false })

		mockRuntime := &runtime.MockRuntime{
			RemoveErr: errors.New("failed to remove"),
		}
		runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
			return mockRuntime, nil
		}
		exitFunc = func(code int) {}

		output, err := executeCommand("--image", "alpine", "sh")
		assert.NoError(t, err)
		assert.Contains(t, output, "Warning: failed to remove container (defer): failed to remove")
	})

	t.Run("does not print warning if RemoveContainer succeeds", func(t *testing.T) {
		// Save and restore package-level state
		oldFactory := runtimeFactory
		oldExit := exitFunc
		t.Cleanup(func() {
			runtimeFactory = oldFactory
			exitFunc = oldExit
		})

		// Reset flags
		rootCmd.Flags().VisitAll(func(f *pflag.Flag) { f.Changed = false })
		rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) { f.Changed = false })

		mockRuntime := &runtime.MockRuntime{
			RemoveErr: nil,
		}
		runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
			return mockRuntime, nil
		}
		exitFunc = func(code int) {}

		output, err := executeCommand("--image", "alpine", "sh")
		assert.NoError(t, err)
		assert.NotContains(t, output, "Warning: failed to remove container (defer)")
	})
}
