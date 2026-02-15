package command

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/config"
	"cderun/internal/runtime"
)

// setupTestOptions creates a rootOptions for testing with common defaults.
func setupTestOptions(t *testing.T) *rootOptions {
	t.Helper()
	o := defaultOptions()
	o.isTerminal = func(fd int) bool { return true }
	o.exitFunc = func(code int) {}
	return o
}

func executeCommand(args ...string) (string, error) {
	return executeCommandContext(context.Background(), args...)
}

func executeCommandContext(ctx context.Context, args ...string) (string, error) {
	return executeCommandRawContext(ctx, append([]string{"cderun"}, args...))
}

func setupMockRuntime(t *testing.T, o *rootOptions, rt runtime.ContainerRuntime) {
	t.Helper()
	o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
		return rt, nil
	}
}

func executeCommandRaw(args []string) (string, error) {
	return executeCommandRawContext(context.Background(), args)
}

func executeCommandRawContext(ctx context.Context, args []string) (string, error) {
	o := defaultOptions()
	// Default to terminal mode for tests to avoid auto-detection of pipes
	// unless specifically overridden in a test.
	o.isTerminal = func(fd int) bool { return true }
	o.exitFunc = func(code int) {}

	buf := &bytes.Buffer{}
	o.out = buf
	o.err = buf

	execErr := ExecuteWithOptions(ctx, args, o)
	return buf.String(), execErr
}

func TestUnit_Command_Root_PreprocessArgs(t *testing.T) {
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
		{
			name:     "shorthand with argument",
			args:     []string{"cderun", "-p", "80:80", "sh", "--cderun-tty"},
			expected: []string{"cderun", "--cderun-tty", "-p", "80:80", "sh"},
		},
		{
			name:     "multiple shorthands, last takes argument",
			args:     []string{"cderun", "-itp", "80:80", "sh", "--cderun-interactive"},
			expected: []string{"cderun", "--cderun-interactive", "-itp", "80:80", "sh"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := defaultOptions()
			cmd := newRootCmd(o)
			actual, err := preprocessArgs(cmd, tt.args)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestUnit_Command_Root_ExecuteEmptyArgs(t *testing.T) {
	// Should not panic
	_, err := executeCommandRaw([]string{})
	require.NoError(t, err)

	_, err = executeCommandRaw(nil)
	require.NoError(t, err)
}

func TestUnit_Command_Root_CommandResolution(t *testing.T) {
	t.Run("executes container correctly", func(t *testing.T) {
		mockRuntime := &runtime.MockRuntime{
			CreatedContainerID: "test-container-id",
			ExitCode:           42,
		}
		var capturedExitCode int
		o := setupTestOptions(t)
		setupMockRuntime(t, o, mockRuntime)
		o.exitFunc = func(code int) {
			capturedExitCode = code
		}

		buf := &bytes.Buffer{}
		o.out = buf
		o.err = buf

		err := ExecuteWithOptions(context.Background(), append([]string{"cderun"}, "--image", "node:20-alpine", "--tty", "-i", "--network", "host", "node", "--version"), o)
		require.NoError(t, err)

		assert.NotNil(t, mockRuntime.CreatedConfig)
		assert.Equal(t, "node:20-alpine", mockRuntime.CreatedConfig.Image)
		assert.Equal(t, []string{"--version"}, mockRuntime.CreatedConfig.Command)
		assert.True(t, mockRuntime.CreatedConfig.TTY)
		assert.True(t, mockRuntime.CreatedConfig.Interactive)
		assert.Equal(t, "host", mockRuntime.CreatedConfig.Network)
		assert.Equal(t, "test-container-id", mockRuntime.StartedContainerID)
		assert.Equal(t, "test-container-id", mockRuntime.WaitedContainerID)
		assert.Equal(t, "test-container-id", mockRuntime.RemovedContainerID)
		assert.Equal(t, 42, capturedExitCode)
	})

	t.Run("shows help when no subcommand is provided", func(t *testing.T) {
		output, err := executeCommand("--tty")
		require.NoError(t, err)

		assert.True(t, strings.HasPrefix(output, "cderun is a CLI wrapper tool"))
		assert.Contains(t, output, "Usage:")
	})

	t.Run("P1 override takes priority over P2 CLI", func(t *testing.T) {
		mockRuntime := &runtime.MockRuntime{}
		o := setupTestOptions(t)
		setupMockRuntime(t, o, mockRuntime)

		err := ExecuteWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "--tty=true", "--cderun-tty=false", "sh"}, o)
		require.NoError(t, err)
		assert.False(t, mockRuntime.CreatedConfig.TTY)
	})

	t.Run("-t shorthand for --tty", func(t *testing.T) {
		mockRuntime := &runtime.MockRuntime{}
		o := setupTestOptions(t)
		setupMockRuntime(t, o, mockRuntime)

		err := ExecuteWithOptions(context.Background(), []string{"cderun", "-t", "--image", "alpine", "sh"}, o)
		require.NoError(t, err)
		assert.True(t, mockRuntime.CreatedConfig.TTY)
	})

	t.Run("returns error for unsupported runtime", func(t *testing.T) {
		o := setupTestOptions(t)
		// Use real runtimeFactory logic but it will fail for "invalid"
		err := ExecuteWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "--runtime", "invalid", "sh"}, o)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported runtime \"invalid\"")
	})

	t.Run("diagnosis mode works without subcommand", func(t *testing.T) {
		mockRuntime := &runtime.MockRuntime{}
		o := setupTestOptions(t)
		setupMockRuntime(t, o, mockRuntime)
		buf := &bytes.Buffer{}
		o.out = buf

		err := ExecuteWithOptions(context.Background(), []string{"cderun", "--diagnosis"}, o)
		require.NoError(t, err)
		output := buf.String()
		assert.Contains(t, output, "runtime:")
		assert.Contains(t, output, "configs:")
		assert.Nil(t, mockRuntime.CreatedConfig)
	})

	t.Run("diagnosis mode works with subcommand and takes precedence", func(t *testing.T) {
		mockRuntime := &runtime.MockRuntime{}
		o := setupTestOptions(t)
		setupMockRuntime(t, o, mockRuntime)
		buf := &bytes.Buffer{}
		o.out = buf

		err := ExecuteWithOptions(context.Background(), []string{"cderun", "--diagnosis", "node", "--version"}, o)
		require.NoError(t, err)
		output := buf.String()
		assert.Contains(t, output, "runtime:")
		assert.Contains(t, output, "configs:")
		assert.NotContains(t, output, "image: node") // Should not be container config dry-run
		assert.Nil(t, mockRuntime.CreatedConfig)
	})

	t.Run("dry-run requires a subcommand", func(t *testing.T) {
		_, err := executeCommand("--dry-run")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--dry-run requires a subcommand")
	})

	t.Run("dry-run outputs configuration and skips execution", func(t *testing.T) {
		mockRuntime := &runtime.MockRuntime{}
		o := setupTestOptions(t)
		setupMockRuntime(t, o, mockRuntime)
		buf := &bytes.Buffer{}
		o.out = buf

		// Dry-run with YAML (default)
		err := ExecuteWithOptions(context.Background(), []string{"cderun", "--dry-run", "--image", "alpine", "sh", "echo", "hello"}, o)
		require.NoError(t, err)
		output := buf.String()
		assert.Contains(t, output, "image: alpine")
		assert.Contains(t, output, "command:")
		assert.Contains(t, output, "- echo")
		assert.Contains(t, output, "- hello")
		assert.NotContains(t, output, "- sh")
		assert.Nil(t, mockRuntime.CreatedConfig, "Runtime should not be called in dry-run mode")

		// Dry-run with JSON
		buf.Reset()
		err = ExecuteWithOptions(context.Background(), []string{"cderun", "--dry-run", "--dry-run-format", "json", "--image", "alpine", "sh", "echo", "hello"}, o)
		require.NoError(t, err)
		output = buf.String()
		assert.Contains(t, output, "\"image\": \"alpine\"")
		assert.Contains(t, output, "\"command\": [")

		// Dry-run with simple
		buf.Reset()
		err = ExecuteWithOptions(context.Background(), []string{"cderun", "--dry-run", "-f", "simple", "--image", "alpine", "sh", "echo", "hello"}, o)
		require.NoError(t, err)
		output = buf.String()
		assert.Contains(t, output, "Image: alpine")
		assert.Contains(t, output, "Command: echo hello")
		assert.NotContains(t, output, "Command: sh")
		assert.Contains(t, output, "TTY: false")
		assert.Contains(t, output, "Interactive: false")
		assert.Contains(t, output, "Network: bridge")
		assert.Contains(t, output, "Remove: true")

		// Dry-run with mount
		buf.Reset()
		err = ExecuteWithOptions(context.Background(), []string{"cderun", "--dry-run", "-f", "simple", "--image", "alpine", "--mount", "type=bind,source=/h,target=/c", "sh"}, o)
		require.NoError(t, err)
		output = buf.String()
		assert.Contains(t, output, "Mounts: type=bind,source=/h,target=/c,readonly=false")

		// Dry-run with device
		buf.Reset()
		err = ExecuteWithOptions(context.Background(), []string{"cderun", "--dry-run", "-f", "simple", "--image", "alpine", "--device", "/dev/video0:/dev/video1:ro", "sh"}, o)
		require.NoError(t, err)
		output = buf.String()
		assert.Contains(t, output, "Devices: /dev/video0:/dev/video1:ro")
	})

	t.Run("returns error if AttachContainer fails", func(t *testing.T) {
		mockRuntime := &runtime.MockRuntime{
			AttachErr: errors.New("attach failed"),
		}
		o := setupTestOptions(t)
		setupMockRuntime(t, o, mockRuntime)

		err := ExecuteWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "sh"}, o)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to attach to container: attach failed")
	})

	t.Run("comma in env value is preserved (StringArrayVar)", func(t *testing.T) {
		mockRuntime := &runtime.MockRuntime{}
		o := setupTestOptions(t)
		setupMockRuntime(t, o, mockRuntime)

		err := ExecuteWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "--env", "MYVAR=a,b", "sh"}, o)
		require.NoError(t, err)

		require.NotNil(t, mockRuntime.CreatedConfig)
		assert.Contains(t, mockRuntime.CreatedConfig.Env, "MYVAR=a,b")
	})
}

func TestUnit_Command_Root_Phase3Features(t *testing.T) {
	t.Run("workdir, mount and device flags", func(t *testing.T) {
		mockRuntime := &runtime.MockRuntime{}
		o := setupTestOptions(t)
		setupMockRuntime(t, o, mockRuntime)

		err := ExecuteWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "--workdir", "/my/workdir", "--mount", "type=bind,source=/h,target=/c,readonly", "--device", "/dev/fuse:/dev/fuse:rm", "sh"}, o)
		require.NoError(t, err)

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

	t.Run("mounting flags no longer require explicit cderun socket settings (auto-enabled if unspecified)", func(t *testing.T) {
		t.Setenv("CDERUN_SOCKET_PATH", "/var/run/docker.sock")
		// If unspecified, --mount-cderun should auto-enable --mount-socket
		t.Setenv("CDERUN_MOUNT_SOCKET", "")

		mockRuntime := &runtime.MockRuntime{}
		o := setupTestOptions(t)
		setupMockRuntime(t, o, mockRuntime)

		err := ExecuteWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "--mount-cderun", "sh"}, o)
		require.NoError(t, err)
		require.NotNil(t, mockRuntime.CreatedConfig)

		socketFound := false
		for _, v := range mockRuntime.CreatedConfig.Mounts {
			if v.Target == "/var/run/docker.sock" {
				assert.Equal(t, "/var/run/docker.sock", v.Source)
				socketFound = true
			}
		}
		assert.True(t, socketFound, "Socket should be automatically mounted")

		// If explicitly set to false, it should NOT mount the socket but NOT fail
		t.Setenv("CDERUN_MOUNT_SOCKET", "false")
		mockRuntime.CreatedConfig = nil
		err = ExecuteWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "--mount-cderun", "sh"}, o)
		require.NoError(t, err)
		require.NotNil(t, mockRuntime.CreatedConfig)

		socketFound = false
		for _, v := range mockRuntime.CreatedConfig.Mounts {
			if strings.Contains(v.Source, "docker.sock") || strings.Contains(v.Target, "docker.sock") {
				socketFound = true
			}
		}
		assert.False(t, socketFound, "Socket should NOT be mounted when CDERUN_MOUNT_SOCKET=false")
	})

	t.Run("mount-cderun logic", func(t *testing.T) {
		mockRuntime := &runtime.MockRuntime{}
		o := setupTestOptions(t)
		setupMockRuntime(t, o, mockRuntime)

		err := ExecuteWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "--mount-cderun", "--mount-socket", "--socket-path", "/socket", "sh"}, o)
		require.NoError(t, err)

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
		mockRuntime := &runtime.MockRuntime{}
		o := setupTestOptions(t)
		setupMockRuntime(t, o, mockRuntime)

		err := ExecuteWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "--mount-socket", "--socket-path", "/host/socket", "--mount-socket-path", "/container/socket", "sh"}, o)
		require.NoError(t, err)

		require.NotNil(t, mockRuntime.CreatedConfig)
		socketFound := false
		for _, v := range mockRuntime.CreatedConfig.Mounts {
			if v.Source == "/host/socket" && v.Target == "/container/socket" {
				socketFound = true
			}
		}
		assert.True(t, socketFound, "socket should be mounted to custom path")
	})
}

func TestUnit_Command_Root_Phase10StrictBehavior(t *testing.T) {
	t.Run("fails when no image mapping found for tool (Step 10.1)", func(t *testing.T) {
		// No .tools.yaml created, and no --image flag
		_, err := executeCommand("unknown-tool", "--version")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no image mapping found for tool: unknown-tool")
	})

	t.Run("subcommand is excluded from CMD (Step 10.2)", func(t *testing.T) {
		mockRuntime := &runtime.MockRuntime{}
		o := setupTestOptions(t)
		setupMockRuntime(t, o, mockRuntime)

		err := ExecuteWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "ls", "-l", "/tmp"}, o)
		require.NoError(t, err)

		require.NotNil(t, mockRuntime.CreatedConfig)
		// 'ls' should be excluded, only '-l' and '/tmp' remain
		assert.Equal(t, []string{"-l", "/tmp"}, mockRuntime.CreatedConfig.Command)
	})
}

func TestUnit_Command_Root_HandleDiagnosis(t *testing.T) {
	t.Run("JSON format", func(t *testing.T) {
		out := &bytes.Buffer{}
		o := &rootOptions{
			fs: config.RealFileSystem{},
		}
		resolved := &config.ResolvedConfig{
			Runtime:         "docker",
			SocketPath:      "/var/run/docker.sock",
			Diagnosis:       true,
			DiagnosisFormat: "json",
		}
		cmd := &cobra.Command{}
		cmd.SetOut(out)

		err := o.handleDiagnosis(cmd, resolved, nil, nil, nil)
		require.NoError(t, err)
		assert.Contains(t, out.String(), "\"name\": \"docker\"")
	})

	t.Run("Simple format", func(t *testing.T) {
		out := &bytes.Buffer{}
		o := &rootOptions{
			fs: config.RealFileSystem{},
		}
		resolved := &config.ResolvedConfig{
			Runtime:         "podman",
			SocketPath:      "/run/podman/podman.sock",
			Diagnosis:       true,
			DiagnosisFormat: "simple",
		}
		cmd := &cobra.Command{}
		cmd.SetOut(out)

		err := o.handleDiagnosis(cmd, resolved, nil, nil, nil)
		require.NoError(t, err)
		assert.Contains(t, out.String(), "Runtime: podman")
	})
}

func TestUnit_Command_Root_BuildContainerConfig_Failures(t *testing.T) {
	t.Run("fails when os.Executable fails", func(t *testing.T) {
		mfs := &config.MockFileSystem{
			ExecErr: errors.New("exec error"),
		}
		o := setupTestOptions(t)
		o.fs = mfs

		// We need to trigger binary mount logic
		resolved := &config.ResolvedConfig{
			MountCderun: true,
		}
		_, err := o.buildContainerConfig(resolved, nil, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get executable path: exec error")
	})
}

func TestUnit_Command_Root_RemoveContainerWarning(t *testing.T) {
	t.Run("prints warning if RemoveContainer fails", func(t *testing.T) {
		mockRuntime := &runtime.MockRuntime{
			RemoveErr: errors.New("failed to remove"),
		}
		o := setupTestOptions(t)
		setupMockRuntime(t, o, mockRuntime)
		buf := &bytes.Buffer{}
		o.err = buf

		err := ExecuteWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "sh"}, o)
		require.NoError(t, err)
		assert.Contains(t, buf.String(), "[WARN] failed to remove container (defer): failed to remove")
	})

	t.Run("does not print warning if RemoveContainer succeeds", func(t *testing.T) {
		mockRuntime := &runtime.MockRuntime{
			RemoveErr: nil,
		}
		o := setupTestOptions(t)
		setupMockRuntime(t, o, mockRuntime)
		buf := &bytes.Buffer{}
		o.err = buf

		err := ExecuteWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "sh"}, o)
		require.NoError(t, err)
		assert.NotContains(t, buf.String(), "[WARN] failed to remove container (defer)")
	})
}
