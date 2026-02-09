package command

import (
	"bytes"
	"cderun/internal/runtime"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func executeCommand(args ...string) (string, error) {
	return executeCommandRaw(append([]string{"cderun"}, args...))
}

func executeCommandRaw(args []string) (string, error) {
	// Reset flag variables and Changed state
	rootCmd = newRootCmd()
	opts.tty = false
	opts.interactive = false
	opts.network = "bridge"
	opts.socketPath = ""
	opts.mountSocket = false
	opts.mountSocketPath = ""
	opts.mountCderun = false
	opts.image = ""
	opts.remove = true
	opts.cderunTTY = false
	opts.cderunInteractive = false
	opts.cderunImage = ""
	opts.cderunNetwork = ""
	opts.cderunRemove = true
	opts.cderunRuntime = ""
	opts.cderunSocketPath = ""
	opts.cderunMountSocket = false
	opts.cderunMountSocketPath = ""
	opts.cderunWorkdir = ""
	opts.cderunMounts = nil
	opts.cderunMountCderun = false
	opts.cderunMountTools = ""
	opts.cderunMountAllTools = false
	opts.runtimeName = "docker"
	opts.env = nil
	opts.cderunEnv = nil
	opts.workdir = ""
	opts.mounts = nil
	opts.mountTools = ""
	opts.mountAllTools = false
	opts.dryRun = false
	opts.dryRunFormat = "yaml"
	opts.cderunDryRun = false
	opts.cderunDryRunFormat = ""
	opts.diagnosis = false
	opts.diagnosisFormat = "yaml"
	opts.cderunDiagnosis = false
	opts.cderunDiagnosisFormat = ""
	opts.logLevel = ""
	opts.logFormat = "text"
	opts.logTimestamp = true
	opts.verbose = 0
	opts.cderunLogLevel = ""
	opts.cderunLogFormat = ""
	opts.cderunVerbose = 0

	opts.ports = nil
	opts.publishAll = false
	opts.expose = nil
	opts.hostname = ""
	opts.dns = nil
	opts.addHosts = nil
	opts.user = ""
	opts.privileged = false
	opts.capAdd = nil
	opts.capDrop = nil
	opts.entrypoint = nil
	opts.pull = "missing"
	opts.memory = ""
	opts.cpus = 0
	opts.devices = nil
	opts.cderunPorts = nil
	opts.cderunPublishAll = false
	opts.cderunExpose = nil
	opts.cderunHostname = ""
	opts.cderunDNS = nil
	opts.cderunAddHosts = nil
	opts.cderunUser = ""
	opts.cderunPrivileged = false
	opts.cderunCapAdd = nil
	opts.cderunCapDrop = nil
	opts.cderunEntrypoint = nil
	opts.cderunPull = ""
	opts.cderunMemory = ""
	opts.cderunCPUs = 0
	opts.cderunDevices = nil

	savedStdout := os.Stdout
	savedStderr := os.Stderr
	savedOut := rootCmd.OutOrStdout()
	savedErr := rootCmd.ErrOrStderr()
	defer func() {
		os.Stdout = savedStdout
		os.Stderr = savedStderr
		rootCmd.SetOut(savedOut)
		rootCmd.SetErr(savedErr)
	}()

	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}
	defer func() { _ = r.Close() }()
	defer func() { _ = w.Close() }()

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
	_, err := executeCommandRaw([]string{})
	assert.NoError(t, err)

	_, err = executeCommandRaw(nil)
	assert.NoError(t, err)
}

func TestRootCmd(t *testing.T) {
	t.Run("executes container correctly", func(t *testing.T) {
		// Save and restore package-level state
		prevFactory := runtimeFactory
		prevExit := exitFunc
		t.Cleanup(func() {
			runtimeFactory = prevFactory
			exitFunc = prevExit
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
		assert.Equal(t, []string{"--version"}, mockRuntime.CreatedConfig.Command)
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
		prevFactory := runtimeFactory
		prevExit := exitFunc
		t.Cleanup(func() {
			runtimeFactory = prevFactory
			exitFunc = prevExit
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
		prevFactory := runtimeFactory
		prevExit := exitFunc
		t.Cleanup(func() {
			runtimeFactory = prevFactory
			exitFunc = prevExit
		})

		// Use a temporary directory for this test
		savedWd, err := os.Getwd()
		require.NoError(t, err)
		tmpDir := t.TempDir()
		require.NoError(t, os.Chdir(tmpDir))
		t.Cleanup(func() { _ = os.Chdir(savedWd) })

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
		assert.Equal(t, []string{"--version"}, mockRuntime.CreatedConfig.Command)
	})

	t.Run("resolves all settings from tools.yaml", func(t *testing.T) {
		// Save and restore package-level state
		prevFactory := runtimeFactory
		prevExit := exitFunc
		t.Cleanup(func() {
			runtimeFactory = prevFactory
			exitFunc = prevExit
		})

		// Use a temporary directory for this test
		savedWd, err := os.Getwd()
		require.NoError(t, err)
		tmpDir := t.TempDir()
		require.NoError(t, os.Chdir(tmpDir))
		t.Cleanup(func() { _ = os.Chdir(savedWd) })

		toolsContent := `
node:
  image: node:20-alpine
  tty: true
  network: host
  env:
    - KEY=VALUE
  mounts:
    - type: bind
      source: /host
      target: /container
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
		assert.Equal(t, []string{"app.js"}, mockRuntime.CreatedConfig.Command)
		assert.True(t, mockRuntime.CreatedConfig.TTY)
		assert.Equal(t, "host", mockRuntime.CreatedConfig.Network)
		assert.Contains(t, mockRuntime.CreatedConfig.Env, "KEY=VALUE")
		assert.Len(t, mockRuntime.CreatedConfig.Mounts, 1)
		assert.Equal(t, "bind", mockRuntime.CreatedConfig.Mounts[0].Type)
		assert.Equal(t, "/host", mockRuntime.CreatedConfig.Mounts[0].Source)
		assert.Equal(t, "/container", mockRuntime.CreatedConfig.Mounts[0].Target)
	})

	t.Run("P3 environment variable takes priority over tools.yaml (Step 10.1)", func(t *testing.T) {
		// Save and restore package-level state
		prevFactory := runtimeFactory
		prevExit := exitFunc
		t.Cleanup(func() {
			runtimeFactory = prevFactory
			exitFunc = prevExit
		})

		t.Setenv("CDERUN_IMAGE", "env-image:latest")

		// Use a temporary directory for this test
		savedWd, err := os.Getwd()
		require.NoError(t, err)
		tmpDir := t.TempDir()
		require.NoError(t, os.Chdir(tmpDir))
		t.Cleanup(func() { _ = os.Chdir(savedWd) })

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
		// Should use image from environment variable (P3 > P4)
		assert.Equal(t, "env-image:latest", mockRuntime.CreatedConfig.Image)
	})

	t.Run("resolves base command from tools.yaml", func(t *testing.T) {
		// Save and restore package-level state
		prevFactory := runtimeFactory
		prevExit := exitFunc
		t.Cleanup(func() {
			runtimeFactory = prevFactory
			exitFunc = prevExit
		})

		// Use a temporary directory for this test
		savedWd, err := os.Getwd()
		require.NoError(t, err)
		tmpDir := t.TempDir()
		require.NoError(t, os.Chdir(tmpDir))
		t.Cleanup(func() { _ = os.Chdir(savedWd) })

		toolsContent := `
node:
  image: node:20-alpine
  command: ["node", "--no-warnings"]
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
		assert.Equal(t, []string{"node", "--no-warnings", "app.js"}, mockRuntime.CreatedConfig.Command)
	})

	t.Run("P1 override takes priority over P2 CLI", func(t *testing.T) {
		// Save and restore package-level state
		prevFactory := runtimeFactory
		prevExit := exitFunc
		t.Cleanup(func() {
			runtimeFactory = prevFactory
			exitFunc = prevExit
		})

		mockRuntime := &runtime.MockRuntime{}
		runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
			return mockRuntime, nil
		}
		exitFunc = func(code int) {}

		_, err := executeCommand("--image", "alpine", "--tty=true", "--cderun-tty=false", "sh")
		assert.NoError(t, err)
		assert.False(t, mockRuntime.CreatedConfig.TTY)
	})

	t.Run("-t shorthand for --tty", func(t *testing.T) {
		// Save and restore package-level state
		prevFactory := runtimeFactory
		prevExit := exitFunc
		t.Cleanup(func() {
			runtimeFactory = prevFactory
			exitFunc = prevExit
		})

		mockRuntime := &runtime.MockRuntime{}
		runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
			return mockRuntime, nil
		}
		exitFunc = func(code int) {}

		_, err := executeCommand("-t", "--image", "alpine", "sh")
		assert.NoError(t, err)
		assert.True(t, mockRuntime.CreatedConfig.TTY)
	})

	t.Run("returns error for unsupported runtime", func(t *testing.T) {
		// Save and restore package-level state
		prevFactory := runtimeFactory
		prevExit := exitFunc
		t.Cleanup(func() {
			runtimeFactory = prevFactory
			exitFunc = prevExit
		})

		// Use the real runtimeFactory here to test the validation logic
		exitFunc = func(code int) {}

		_, err := executeCommand("--image", "alpine", "--runtime", "invalid", "sh")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported runtime \"invalid\"")
	})

	t.Run("environment variable pass-through and P1 overrides", func(t *testing.T) {
		// Save and restore package-level state
		prevFactory := runtimeFactory
		prevExit := exitFunc
		t.Cleanup(func() {
			runtimeFactory = prevFactory
			exitFunc = prevExit
		})

		// Use a temporary directory for this test
		savedWd, err := os.Getwd()
		require.NoError(t, err)
		tmpDir := t.TempDir()
		require.NoError(t, os.Chdir(tmpDir))
		t.Cleanup(func() { _ = os.Chdir(savedWd) })

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
		// Now using overwrite logic: P1 overrides everything else
		assert.Len(t, envs, 1)
		assert.Contains(t, envs, "P1_OVERRIDE_KEY=P1_VALUE")
		assert.NotContains(t, envs, "TOOL_KEY=TOOL_VALUE")
		assert.NotContains(t, envs, "OVERRIDE_KEY=CLI_VALUE")
		assert.NotContains(t, envs, "HOST_KEY=HOST_VALUE")
		assert.NotContains(t, envs, "CLI_KEY=CLI_VALUE")
		assert.NotContains(t, envs, "CLI_HOST_KEY=CLI_HOST_VALUE")
	})

	t.Run("diagnosis mode works without subcommand", func(t *testing.T) {
		// Save and restore package-level state
		prevFactory := runtimeFactory
		prevExit := exitFunc
		t.Cleanup(func() {
			runtimeFactory = prevFactory
			exitFunc = prevExit
		})

		mockRuntime := &runtime.MockRuntime{}
		runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
			return mockRuntime, nil
		}
		exitFunc = func(code int) {}

		output, err := executeCommand("--diagnosis")
		assert.NoError(t, err)
		assert.Contains(t, output, "runtime:")
		assert.Contains(t, output, "configs:")
		assert.Nil(t, mockRuntime.CreatedConfig)
	})

	t.Run("diagnosis mode works with subcommand and takes precedence", func(t *testing.T) {
		// Save and restore package-level state
		prevFactory := runtimeFactory
		prevExit := exitFunc
		t.Cleanup(func() {
			runtimeFactory = prevFactory
			exitFunc = prevExit
		})

		mockRuntime := &runtime.MockRuntime{}
		runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
			return mockRuntime, nil
		}
		exitFunc = func(code int) {}

		output, err := executeCommand("--diagnosis", "node", "--version")
		assert.NoError(t, err)
		assert.Contains(t, output, "runtime:")
		assert.Contains(t, output, "configs:")
		assert.NotContains(t, output, "image: node") // Should not be container config dry-run
		assert.Nil(t, mockRuntime.CreatedConfig)
	})

	t.Run("dry-run requires a subcommand", func(t *testing.T) {
		// Save and restore package-level state
		prevFactory := runtimeFactory
		prevExit := exitFunc
		t.Cleanup(func() {
			runtimeFactory = prevFactory
			exitFunc = prevExit
		})

		mockRuntime := &runtime.MockRuntime{}
		runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
			return mockRuntime, nil
		}
		exitFunc = func(code int) {}

		_, err := executeCommand("--dry-run")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--dry-run requires a subcommand")
	})

	t.Run("dry-run outputs configuration and skips execution", func(t *testing.T) {
		// Save and restore package-level state
		prevFactory := runtimeFactory
		prevExit := exitFunc
		t.Cleanup(func() {
			runtimeFactory = prevFactory
			exitFunc = prevExit
		})

		mockRuntime := &runtime.MockRuntime{}
		runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
			return mockRuntime, nil
		}
		exitFunc = func(code int) {}

		// Dry-run with YAML (default)
		// Step 10.2: subcommand 'sh' is excluded from command
		output, err := executeCommand("--dry-run", "--image", "alpine", "sh", "echo", "hello")
		assert.NoError(t, err)
		assert.Contains(t, output, "image: alpine")
		assert.Contains(t, output, "command:")
		assert.Contains(t, output, "- echo")
		assert.Contains(t, output, "- hello")
		assert.NotContains(t, output, "- sh")
		assert.Nil(t, mockRuntime.CreatedConfig, "Runtime should not be called in dry-run mode")

		// Dry-run with JSON
		output, err = executeCommand("--dry-run", "--dry-run-format", "json", "--image", "alpine", "sh", "echo", "hello")
		assert.NoError(t, err)
		assert.Contains(t, output, "\"image\": \"alpine\"")
		assert.Contains(t, output, "\"command\": [")

		// Dry-run with simple
		output, err = executeCommand("--dry-run", "-f", "simple", "--image", "alpine", "sh", "echo", "hello")
		assert.NoError(t, err)
		assert.Contains(t, output, "Image: alpine")
		assert.Contains(t, output, "Command: echo hello")
		assert.NotContains(t, output, "Command: sh")
		assert.Contains(t, output, "TTY: false")
		assert.Contains(t, output, "Interactive: false")
		assert.Contains(t, output, "Network: bridge")
		assert.Contains(t, output, "Remove: true")

		// Dry-run with mount
		output, err = executeCommand("--dry-run", "-f", "simple", "--image", "alpine", "--mount", "type=bind,source=/h,target=/c", "sh")
		assert.NoError(t, err)
		assert.Contains(t, output, "Mounts: type=bind,source=/h,target=/c,readonly=false")

		// Dry-run with device
		output, err = executeCommand("--dry-run", "-f", "simple", "--image", "alpine", "--device", "/dev/video0:/dev/video1:ro", "sh")
		assert.NoError(t, err)
		assert.Contains(t, output, "Devices: /dev/video0:/dev/video1:ro")
	})

	t.Run("returns error if AttachContainer fails", func(t *testing.T) {
		// Save and restore package-level state
		prevFactory := runtimeFactory
		prevExit := exitFunc
		t.Cleanup(func() {
			runtimeFactory = prevFactory
			exitFunc = prevExit
		})

		mockRuntime := &runtime.MockRuntime{
			AttachErr: errors.New("attach failed"),
		}
		runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
			return mockRuntime, nil
		}
		exitFunc = func(code int) {}

		_, err := executeCommand("--image", "alpine", "sh")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to attach to container: attach failed")
	})

	t.Run("comma in env value is preserved (StringArrayVar)", func(t *testing.T) {
		// Save and restore package-level state
		prevFactory := runtimeFactory
		prevExit := exitFunc
		t.Cleanup(func() {
			runtimeFactory = prevFactory
			exitFunc = prevExit
		})

		mockRuntime := &runtime.MockRuntime{}
		runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
			return mockRuntime, nil
		}
		exitFunc = func(code int) {}

		_, err := executeCommand("--image", "alpine", "--env", "MYVAR=a,b", "sh")
		assert.NoError(t, err)

		require.NotNil(t, mockRuntime.CreatedConfig)
		assert.Contains(t, mockRuntime.CreatedConfig.Env, "MYVAR=a,b")
	})

	t.Run("mount-tools not found error message includes available tools", func(t *testing.T) {
		// Save and restore package-level state
		prevFactory := runtimeFactory
		prevExit := exitFunc
		t.Cleanup(func() {
			runtimeFactory = prevFactory
			exitFunc = prevExit
		})

		// Setup tools config
		savedWd, err := os.Getwd()
		require.NoError(t, err)
		tmpDir := t.TempDir()
		require.NoError(t, os.Chdir(tmpDir))
		t.Cleanup(func() { _ = os.Chdir(savedWd) })

		toolsContent := `
node:
  image: node:20
python:
  image: python:3
`
		err = os.WriteFile(".tools.yaml", []byte(toolsContent), 0644)
		require.NoError(t, err)

		mockRuntime := &runtime.MockRuntime{}
		runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
			return mockRuntime, nil
		}
		exitFunc = func(code int) {}

		_, err = executeCommand("--mount-socket", "--mount-tools", "unknown", "--image", "alpine", "sh")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "tool \"unknown\" not found in .tools.yaml")
		assert.Contains(t, err.Error(), "available tools: node, python")
	})
}

func TestCderunInternalOverrides(t *testing.T) {
	// Use a temporary directory for this test
	savedWd, err := os.Getwd()
	require.NoError(t, err)
	tmpDir := t.TempDir()
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() { _ = os.Chdir(savedWd) })

	// Create a temporary .tools.yaml for image mapping
	toolsContent := `
node:
  image: node:20-alpine
`
	err = os.WriteFile(".tools.yaml", []byte(toolsContent), 0644)
	require.NoError(t, err)

	// Save and restore package-level state
	savedTTY := opts.tty
	savedCderunTTY := opts.cderunTTY
	prevFactory := runtimeFactory
	prevExit := exitFunc
	t.Cleanup(func() {
		opts.tty = savedTTY
		opts.cderunTTY = savedCderunTTY
		runtimeFactory = prevFactory
		exitFunc = prevExit
	})

	mockRuntime := &runtime.MockRuntime{}
	runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
		return mockRuntime, nil
	}
	exitFunc = func(code int) {}

	t.Run("cderun-tty overrides tty even if placed after subcommand", func(t *testing.T) {
		// cderun --tty=true node --cderun-tty=false --version
		// We use a path that doesn't end in "cderun" for polyglot test later,
		// but here we use "cderun" explicitly.
		_, err := executeCommandRaw([]string{"cderun", "--tty=true", "node", "--cderun-tty=false", "--version"})
		assert.NoError(t, err)

		require.NotNil(t, mockRuntime.CreatedConfig)
		assert.False(t, mockRuntime.CreatedConfig.TTY, "TTY should be false because --cderun-tty=false overrides --tty=true")
	})

	t.Run("cderun-tty works in polyglot mode", func(t *testing.T) {
		mockRuntime.CreatedConfig = nil

		// node --cderun-tty=true --version
		_, err := executeCommandRaw([]string{"node", "--cderun-tty=true", "--version"})
		assert.NoError(t, err)

		require.NotNil(t, mockRuntime.CreatedConfig)
		assert.True(t, mockRuntime.CreatedConfig.TTY, "TTY should be true because --cderun-tty=true was provided")
	})

	t.Run("cderun internal overrides before subcommand result in error", func(t *testing.T) {
		// cderun --cderun-image=alpine:latest sh
		_, err := executeCommandRaw([]string{"cderun", "--cderun-image=alpine:latest", "sh"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be placed after the subcommand")
	})

	t.Run("cderun internal overrides after subcommand work correctly", func(t *testing.T) {
		mockRuntime.CreatedConfig = nil

		// cderun --image=alpine:stable sh --cderun-image=alpine:latest
		_, err := executeCommandRaw([]string{"cderun", "--image=alpine:stable", "sh", "--cderun-image=alpine:latest"})
		assert.NoError(t, err)
		require.NotNil(t, mockRuntime.CreatedConfig)
		assert.Equal(t, "alpine:latest", mockRuntime.CreatedConfig.Image)
	})

	t.Run("cderun internal overrides for network, remove, workdir and mount", func(t *testing.T) {
		mockRuntime.CreatedConfig = nil

		_, err := executeCommand("--image=alpine", "--network=bridge", "--remove=false", "--workdir=/initial", "--mount=type=bind,source=/h1,target=/c1", "sh", "--cderun-network=host", "--cderun-remove=true", "--cderun-workdir=/override", "--cderun-mount=type=bind,source=/h2,target=/c2")
		assert.NoError(t, err)
		require.NotNil(t, mockRuntime.CreatedConfig)
		assert.Equal(t, "host", mockRuntime.CreatedConfig.Network)
		assert.True(t, mockRuntime.CreatedConfig.Remove)
		assert.Equal(t, "/override", mockRuntime.CreatedConfig.Workdir)

		// Mounts should be overwritten (P1 replaces P2)
		assert.Len(t, mockRuntime.CreatedConfig.Mounts, 1)
		assert.Equal(t, "/h2", mockRuntime.CreatedConfig.Mounts[0].Source)
	})

	t.Run("cderun internal overrides for runtime, socket and mounting", func(t *testing.T) {
		mockRuntime.CreatedConfig = nil

		// Setup tools config for mount-tools
		savedWd, err := os.Getwd()
		require.NoError(t, err)
		tmpDir := t.TempDir()
		require.NoError(t, os.Chdir(tmpDir))
		t.Cleanup(func() { _ = os.Chdir(savedWd) })
		err = os.WriteFile(".tools.yaml", []byte("node:\n  image: node:20"), 0644)
		require.NoError(t, err)

		_, err = executeCommand("--image=alpine", "sh", "--cderun-runtime=docker", "--cderun-socket-path=/var/run/custom.sock", "--cderun-mount-socket=true", "--cderun-mount-cderun=true", "--cderun-mount-tools=node")
		assert.NoError(t, err)
		require.NotNil(t, mockRuntime.CreatedConfig)

		socketFound := false
		cderunFound := false
		nodeFound := false
		for _, v := range mockRuntime.CreatedConfig.Mounts {
			if v.Source == "/var/run/custom.sock" {
				socketFound = true
			}
			if v.Target == "/usr/local/bin/cderun" {
				cderunFound = true
			}
			if v.Target == "/usr/local/bin/node" {
				nodeFound = true
			}
		}
		assert.True(t, socketFound)
		assert.True(t, cderunFound)
		assert.True(t, nodeFound)
	})

	t.Run("cderun internal override can turn off remove", func(t *testing.T) {
		mockRuntime.CreatedConfig = nil

		_, err := executeCommand("--image=alpine", "--remove=true", "sh", "--cderun-remove=false")
		assert.NoError(t, err)
		require.NotNil(t, mockRuntime.CreatedConfig)
		assert.False(t, mockRuntime.CreatedConfig.Remove)
	})

	t.Run("cderun internal overrides for dry-run", func(t *testing.T) {
		mockRuntime.CreatedConfig = nil

		// cderun --image=alpine sh echo hello --cderun-dry-run --cderun-dry-run-format=simple
		output, err := executeCommandRaw([]string{"cderun", "--image=alpine", "sh", "echo", "hello", "--cderun-dry-run", "--cderun-dry-run-format=simple"})
		assert.NoError(t, err)
		assert.Nil(t, mockRuntime.CreatedConfig)
		assert.Contains(t, output, "Image: alpine")
		assert.Contains(t, output, "Command: echo hello")
	})
}

func TestPhase3Features(t *testing.T) {
	// Save and restore package-level state
	prevFactory := runtimeFactory
	prevExit := exitFunc
	t.Cleanup(func() {
		runtimeFactory = prevFactory
		exitFunc = prevExit
	})

	mockRuntime := &runtime.MockRuntime{}
	runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
		return mockRuntime, nil
	}
	exitFunc = func(code int) {}

	t.Run("workdir, mount and device flags", func(t *testing.T) {
		mockRuntime.CreatedConfig = nil

		_, err := executeCommand("--image", "alpine", "--workdir", "/my/workdir", "--mount", "type=bind,source=/h,target=/c,readonly", "--device", "/dev/fuse:/dev/fuse:rm", "sh")
		assert.NoError(t, err)

		require.NotNil(t, mockRuntime.CreatedConfig)
		assert.Equal(t, "/my/workdir", mockRuntime.CreatedConfig.Workdir)
		require.Len(t, mockRuntime.CreatedConfig.Mounts, 1)
		assert.Equal(t, "/h", mockRuntime.CreatedConfig.Mounts[0].Source)
		assert.Equal(t, "/c", mockRuntime.CreatedConfig.Mounts[0].Target)
		assert.True(t, mockRuntime.CreatedConfig.Mounts[0].ReadOnly)

		require.Len(t, mockRuntime.CreatedConfig.Devices, 1)
		assert.Equal(t, "/dev/fuse", mockRuntime.CreatedConfig.Devices[0].PathOnHost)
		assert.Equal(t, "/dev/fuse", mockRuntime.CreatedConfig.Devices[0].PathInContainer)
		assert.Equal(t, "rm", mockRuntime.CreatedConfig.Devices[0].CgroupPermissions)
	})

	t.Run("mounting flags require explicit cderun socket settings", func(t *testing.T) {
		t.Setenv("CDERUN_SOCKET_PATH", "/var/run/docker.sock")
		t.Setenv("CDERUN_MOUNT_SOCKET", "false")

		_, err := executeCommand("--image", "alpine", "--mount-cderun", "sh")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "requires --mount-socket")

		// CDERUN_MOUNT_SOCKET should work
		t.Setenv("CDERUN_MOUNT_SOCKET", "true")
		_, err = executeCommand("--image", "alpine", "--mount-cderun", "sh")
		assert.NoError(t, err)
	})

	t.Run("mount-cderun logic", func(t *testing.T) {
		mockRuntime.CreatedConfig = nil

		_, err := executeCommand("--image", "alpine", "--mount-cderun", "--mount-socket", "--socket-path", "/socket", "sh")
		assert.NoError(t, err)

		require.NotNil(t, mockRuntime.CreatedConfig)
		exePath, _ := os.Executable()

		binaryFound := false
		socketFound := false
		for _, v := range mockRuntime.CreatedConfig.Mounts {
			if v.Source == exePath && v.Target == "/usr/local/bin/cderun" {
				binaryFound = true
			}
			if v.Source == "/socket" && v.Target == "/socket" {
				socketFound = true
			}
		}
		assert.True(t, binaryFound, "binary should be mounted")
		assert.True(t, socketFound, "socket should be mounted")
	})

	t.Run("mount-socket-path logic", func(t *testing.T) {
		mockRuntime.CreatedConfig = nil

		_, err := executeCommand("--image", "alpine", "--mount-socket", "--socket-path", "/host/socket", "--mount-socket-path", "/container/socket", "sh")
		assert.NoError(t, err)

		require.NotNil(t, mockRuntime.CreatedConfig)
		socketFound := false
		for _, v := range mockRuntime.CreatedConfig.Mounts {
			if v.Source == "/host/socket" && v.Target == "/container/socket" {
				socketFound = true
			}
		}
		assert.True(t, socketFound, "socket should be mounted to custom path")
	})

	t.Run("mount-tools logic", func(t *testing.T) {
		mockRuntime.CreatedConfig = nil

		// Setup tools config
		savedWd, err := os.Getwd()
		require.NoError(t, err)
		tmpDir := t.TempDir()
		require.NoError(t, os.Chdir(tmpDir))
		t.Cleanup(func() { _ = os.Chdir(savedWd) })

		toolsContent := `
node:
  image: node:20
python:
  image: python:3
sh:
  image: alpine
`
		err = os.WriteFile(".tools.yaml", []byte(toolsContent), 0644)
		require.NoError(t, err)

		_, err = executeCommand("--mount-tools", "node", "--mount-socket", "--socket-path", "/socket", "sh")
		assert.NoError(t, err)

		require.NotNil(t, mockRuntime.CreatedConfig)
		exePath, _ := os.Executable()

		nodeFound := false
		pythonFound := false
		for _, v := range mockRuntime.CreatedConfig.Mounts {
			if v.Source == exePath && v.Target == "/usr/local/bin/node" {
				nodeFound = true
			}
			if v.Source == exePath && v.Target == "/usr/local/bin/python" {
				pythonFound = true
			}
		}
		assert.True(t, nodeFound, "node should be mounted")
		assert.False(t, pythonFound, "python should NOT be mounted")

		// Test mount-all-tools
		mockRuntime.CreatedConfig = nil

		_, err = executeCommand("--mount-all-tools", "--mount-socket", "--socket-path", "/socket", "sh")
		assert.NoError(t, err)

		nodeFound = false
		pythonFound = false
		for _, v := range mockRuntime.CreatedConfig.Mounts {
			if v.Source == exePath && v.Target == "/usr/local/bin/node" {
				nodeFound = true
			}
			if v.Source == exePath && v.Target == "/usr/local/bin/python" {
				pythonFound = true
			}
		}
		assert.True(t, nodeFound, "node should be mounted")
		assert.True(t, pythonFound, "python should be mounted")
	})

	t.Run("mount-all-tools with empty config shows warning", func(t *testing.T) {
		mockRuntime.CreatedConfig = nil

		// Setup empty tools config
		savedWd, err := os.Getwd()
		require.NoError(t, err)
		tmpDir := t.TempDir()
		require.NoError(t, os.Chdir(tmpDir))
		t.Cleanup(func() { _ = os.Chdir(savedWd) })

		// No .tools.yaml created

		output, err := executeCommand("--mount-all-tools", "--mount-socket", "--socket-path", "/socket", "--image", "alpine", "sh")
		assert.NoError(t, err)
		assert.Contains(t, output, "[WARN] --mount-all-tools specified but no tools defined in .tools.yaml")
	})
}

func TestPhase10StrictBehavior(t *testing.T) {
	// Save and restore package-level state
	prevFactory := runtimeFactory
	prevExit := exitFunc
	t.Cleanup(func() {
		runtimeFactory = prevFactory
		exitFunc = prevExit
	})

	mockRuntime := &runtime.MockRuntime{}
	runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
		return mockRuntime, nil
	}
	exitFunc = func(code int) {}

	t.Run("fails when no image mapping found for tool (Step 10.1)", func(t *testing.T) {
		// No .tools.yaml created, and no --image flag
		_, err := executeCommand("unknown-tool", "--version")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no image mapping found for tool: unknown-tool")
	})

	t.Run("subcommand is excluded from CMD (Step 10.2)", func(t *testing.T) {
		mockRuntime.CreatedConfig = nil
		_, err := executeCommand("--image", "alpine", "ls", "-l", "/tmp")
		assert.NoError(t, err)

		require.NotNil(t, mockRuntime.CreatedConfig)
		// 'ls' should be excluded, only '-l' and '/tmp' remain
		assert.Equal(t, []string{"-l", "/tmp"}, mockRuntime.CreatedConfig.Command)
	})

	t.Run("subcommand is excluded even if it is a tool (Step 10.2)", func(t *testing.T) {
		// Setup tools config
		savedWd, err := os.Getwd()
		require.NoError(t, err)
		tmpDir := t.TempDir()
		require.NoError(t, os.Chdir(tmpDir))
		t.Cleanup(func() { _ = os.Chdir(savedWd) })

		toolsContent := `
node:
  image: node:20
`
		err = os.WriteFile(".tools.yaml", []byte(toolsContent), 0644)
		require.NoError(t, err)

		mockRuntime.CreatedConfig = nil
		_, err = executeCommand("node", "app.js")
		assert.NoError(t, err)

		require.NotNil(t, mockRuntime.CreatedConfig)
		// 'node' is excluded from CMD because it's not defined in tool's 'command' field
		assert.Equal(t, []string{"app.js"}, mockRuntime.CreatedConfig.Command)
	})

	t.Run("subcommand is included if explicitly in tool's command field", func(t *testing.T) {
		// Setup tools config
		savedWd, err := os.Getwd()
		require.NoError(t, err)
		tmpDir := t.TempDir()
		require.NoError(t, os.Chdir(tmpDir))
		t.Cleanup(func() { _ = os.Chdir(savedWd) })

		toolsContent := `
node:
  image: node:20
  command: ["node", "--no-warnings"]
`
		err = os.WriteFile(".tools.yaml", []byte(toolsContent), 0644)
		require.NoError(t, err)

		mockRuntime.CreatedConfig = nil
		_, err = executeCommand("node", "app.js")
		assert.NoError(t, err)

		require.NotNil(t, mockRuntime.CreatedConfig)
		// 'node' and '--no-warnings' come from tool config, 'app.js' from passthrough
		assert.Equal(t, []string{"node", "--no-warnings", "app.js"}, mockRuntime.CreatedConfig.Command)
	})
}

func TestRemoveContainerWarning(t *testing.T) {
	t.Run("prints warning if RemoveContainer fails", func(t *testing.T) {
		// Save and restore package-level state
		prevFactory := runtimeFactory
		prevExit := exitFunc
		t.Cleanup(func() {
			runtimeFactory = prevFactory
			exitFunc = prevExit
		})

		mockRuntime := &runtime.MockRuntime{
			RemoveErr: errors.New("failed to remove"),
		}
		runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
			return mockRuntime, nil
		}
		exitFunc = func(code int) {}

		output, err := executeCommand("--image", "alpine", "sh")
		assert.NoError(t, err)
		assert.Contains(t, output, "[WARN] failed to remove container (defer): failed to remove")
	})

	t.Run("does not print warning if RemoveContainer succeeds", func(t *testing.T) {
		// Save and restore package-level state
		prevFactory := runtimeFactory
		prevExit := exitFunc
		t.Cleanup(func() {
			runtimeFactory = prevFactory
			exitFunc = prevExit
		})

		mockRuntime := &runtime.MockRuntime{
			RemoveErr: nil,
		}
		runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
			return mockRuntime, nil
		}
		exitFunc = func(code int) {}

		output, err := executeCommand("--image", "alpine", "sh")
		assert.NoError(t, err)
		assert.NotContains(t, output, "[WARN] failed to remove container (defer)")
	})
}
