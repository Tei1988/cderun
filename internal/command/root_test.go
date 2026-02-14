package command

import (
	"bytes"
	"cderun/internal/config"
	"cderun/internal/runtime"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testOptions *rootOptions

func executeCommand(args ...string) (string, error) {
	return executeCommandContext(context.Background(), args...)
}

func executeCommandContext(ctx context.Context, args ...string) (string, error) {
	return executeCommandRawContext(ctx, append([]string{"cderun"}, args...))
}

func setupTestOptions(t *testing.T) {
	t.Helper()
	testOptions = newDefaultOptions()
	t.Cleanup(func() {
		testOptions = nil
	})
}

func setupMockRuntime(t *testing.T, mock *runtime.MockRuntime) {
	t.Helper()
	if testOptions == nil {
		setupTestOptions(t)
	}
	testOptions.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
		return mock, nil
	}
	testOptions.exitFunc = func(code int) {}
}

func executeCommandRaw(args []string) (string, error) {
	return executeCommandRawContext(context.Background(), args)
}

func executeCommandRawContext(ctx context.Context, args []string) (string, error) {
	// Re-initialize options for each call if not set by test
	// Note: for multiple calls in the same test, the caller should
	// ensure testOptions is preserved or re-initialized as needed.
	o := testOptions
	if o == nil {
		o = newDefaultOptions()
	}

	// Default to terminal mode for tests to avoid auto-detection of pipes
	// unless specifically overridden in a test.
	if o.isTerminal == nil {
		o.isTerminal = func(fd int) bool { return true }
	}

	cmd := newRootCmd(o)

	savedStdout := os.Stdout
	savedStderr := os.Stderr
	savedOut := cmd.OutOrStdout()
	savedErr := cmd.ErrOrStderr()
	defer func() {
		os.Stdout = savedStdout
		os.Stderr = savedStderr
		cmd.SetOut(savedOut)
		cmd.SetErr(savedErr)
	}()

	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}
	defer func() { _ = r.Close() }()
	defer func() { _ = w.Close() }()

	os.Stdout = w
	os.Stderr = w
	cmd.SetOut(w)
	cmd.SetErr(w)

	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(&buf, r)
		close(done)
	}()

	// Since we are bypassng the standard ExecuteContext for finer control
	// we perform the preprocessing and argument setting here.
	processedArgs, err := preprocessArgs(cmd, args)
	var execErr error
	if err == nil {
		if len(processedArgs) >= 1 {
			cmd.SetArgs(processedArgs[1:])
		} else {
			cmd.SetArgs([]string{})
		}
		execErr = cmd.ExecuteContext(ctx)
	} else {
		execErr = err
	}

	_ = w.Close()
	<-done

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
		{
			name:     "flag with = is not skipped by hoist",
			args:     []string{"cderun", "sh", "--cderun-log-level=debug", "-l"},
			expected: []string{"cderun", "--cderun-log-level=debug", "sh", "-l"},
		},
		{
			name:     "flag without = takes next argument as value during hoist",
			args:     []string{"cderun", "sh", "--cderun-log-level", "debug", "-l"},
			expected: []string{"cderun", "--cderun-log-level", "debug", "sh", "-l"},
		},
		{
			name:     "multiple hoisting",
			args:     []string{"cderun", "sh", "--cderun-tty", "ls", "--cderun-image", "alpine"},
			expected: []string{"cderun", "--cderun-tty", "--cderun-image", "alpine", "sh", "ls"},
		},
	}

	cmd := newRootCmd(newDefaultOptions())
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
		setupTestOptions(t)
		mockRuntime := &runtime.MockRuntime{
			CreatedContainerID: "test-container-id",
			ExitCode:           42,
		}
		var capturedExitCode int
		setupMockRuntime(t, mockRuntime)
		testOptions.exitFunc = func(code int) {
			capturedExitCode = code
		}

		_, err := executeCommand("--image", "node:20-alpine", "--tty", "-i", "--network", "host", "node", "--version")
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
		setupTestOptions(t)
		setupMockRuntime(t, &runtime.MockRuntime{})

		output, err := executeCommand("--tty")
		require.NoError(t, err)

		assert.True(t, strings.HasPrefix(output, "cderun is a CLI wrapper tool"))
		assert.Contains(t, output, "Usage:")
	})

	t.Run("P1 override takes priority over P2 CLI", func(t *testing.T) {
		setupTestOptions(t)
		mockRuntime := &runtime.MockRuntime{}
		setupMockRuntime(t, mockRuntime)

		_, err := executeCommand("--image", "alpine", "--tty=true", "--cderun-tty=false", "sh")
		require.NoError(t, err)
		assert.False(t, mockRuntime.CreatedConfig.TTY)
	})

	t.Run("-t shorthand for --tty", func(t *testing.T) {
		setupTestOptions(t)
		mockRuntime := &runtime.MockRuntime{}
		setupMockRuntime(t, mockRuntime)

		_, err := executeCommand("-t", "--image", "alpine", "sh")
		require.NoError(t, err)
		assert.True(t, mockRuntime.CreatedConfig.TTY)
	})

	t.Run("returns error for unsupported runtime", func(t *testing.T) {
		setupTestOptions(t)
		testOptions.exitFunc = func(code int) {}

		_, err := executeCommand("--image", "alpine", "--runtime", "invalid", "sh")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported runtime \"invalid\"")
	})

	t.Run("diagnosis mode works without subcommand", func(t *testing.T) {
		setupTestOptions(t)
		mockRuntime := &runtime.MockRuntime{}
		setupMockRuntime(t, mockRuntime)

		output, err := executeCommand("--diagnosis")
		require.NoError(t, err)
		assert.Contains(t, output, "runtime:")
		assert.Contains(t, output, "configs:")
		assert.Nil(t, mockRuntime.CreatedConfig)
	})

	t.Run("diagnosis mode works with subcommand and takes precedence", func(t *testing.T) {
		setupTestOptions(t)
		mockRuntime := &runtime.MockRuntime{}
		setupMockRuntime(t, mockRuntime)

		output, err := executeCommand("--diagnosis", "node", "--version")
		require.NoError(t, err)
		assert.Contains(t, output, "runtime:")
		assert.Contains(t, output, "configs:")
		assert.NotContains(t, output, "image: node") // Should not be container config dry-run
		assert.Nil(t, mockRuntime.CreatedConfig)
	})

	t.Run("returns error if AttachContainer fails", func(t *testing.T) {
		setupTestOptions(t)
		mockRuntime := &runtime.MockRuntime{
			AttachErr: errors.New("attach failed"),
		}
		setupMockRuntime(t, mockRuntime)

		_, err := executeCommand("--image", "alpine", "sh")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to attach to container: attach failed")
	})

	t.Run("comma in env value is preserved (StringArrayVar)", func(t *testing.T) {
		setupTestOptions(t)
		mockRuntime := &runtime.MockRuntime{}
		setupMockRuntime(t, mockRuntime)

		_, err := executeCommand("--image", "alpine", "--env", "MYVAR=a,b", "sh")
		require.NoError(t, err)

		require.NotNil(t, mockRuntime.CreatedConfig)
		assert.Contains(t, mockRuntime.CreatedConfig.Env, "MYVAR=a,b")
	})
}

func TestUnit_Command_Root_Phase3Features(t *testing.T) {
	setupTestOptions(t)
	mockRuntime := &runtime.MockRuntime{}
	setupMockRuntime(t, mockRuntime)

	t.Run("workdir, mount and device flags", func(t *testing.T) {
		mockRuntime.CreatedConfig = nil

		_, err := executeCommand("--image", "alpine", "--workdir", "/my/workdir", "--mount", "type=bind,source=/h,target=/c,readonly", "--device", "/dev/fuse:/dev/fuse:rm", "sh")
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

		mockRuntime.CreatedConfig = nil
		_, err := executeCommand("--image", "alpine", "--mount-cderun", "sh")
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
		_, err = executeCommand("--image", "alpine", "--mount-cderun", "sh")
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
		mockRuntime.CreatedConfig = nil

		_, err := executeCommand("--image", "alpine", "--mount-cderun", "--mount-socket", "--socket-path", "/socket", "sh")
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
		mockRuntime.CreatedConfig = nil

		_, err := executeCommand("--image", "alpine", "--mount-socket", "--socket-path", "/host/socket", "--mount-socket-path", "/container/socket", "sh")
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
	setupTestOptions(t)
	mockRuntime := &runtime.MockRuntime{}
	setupMockRuntime(t, mockRuntime)

	t.Run("fails when no image mapping found for tool (Step 10.1)", func(t *testing.T) {
		// No .tools.yaml created, and no --image flag
		_, err := executeCommand("unknown-tool", "--version")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no image mapping found for tool: unknown-tool")
	})

	t.Run("subcommand is excluded from CMD (Step 10.2)", func(t *testing.T) {
		mockRuntime.CreatedConfig = nil
		_, err := executeCommand("--image", "alpine", "ls", "-l", "/tmp")
		require.NoError(t, err)

		require.NotNil(t, mockRuntime.CreatedConfig)
		// 'ls' should be excluded, only '-l' and '/tmp' remain
		assert.Equal(t, []string{"-l", "/tmp"}, mockRuntime.CreatedConfig.Command)
	})
}

func TestUnit_Command_Root_BuildContainerConfig_Failures(t *testing.T) {
	t.Run("fails when os.Executable fails", func(t *testing.T) {
		mfs := &config.MockFileSystem{
			ExecErr: errors.New("exec error"),
		}
		opts := &rootOptions{
			fs: mfs,
		}

		// We need to trigger binary mount logic
		resolved := &config.ResolvedConfig{
			MountCderun: true,
		}
		_, err := opts.buildContainerConfig(resolved, nil, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get executable path: exec error")
	})
}

func TestUnit_Command_Root_RemoveContainerWarning(t *testing.T) {
	t.Run("prints warning if RemoveContainer fails", func(t *testing.T) {
		setupTestOptions(t)
		mockRuntime := &runtime.MockRuntime{
			RemoveErr: errors.New("failed to remove"),
		}
		setupMockRuntime(t, mockRuntime)

		output, err := executeCommand("--image", "alpine", "sh")
		require.NoError(t, err)
		assert.Contains(t, output, "[WARN] failed to remove container (defer): failed to remove")
	})

	t.Run("does not print warning if RemoveContainer succeeds", func(t *testing.T) {
		setupTestOptions(t)
		mockRuntime := &runtime.MockRuntime{
			RemoveErr: nil,
		}
		setupMockRuntime(t, mockRuntime)

		output, err := executeCommand("--image", "alpine", "sh")
		require.NoError(t, err)
		assert.NotContains(t, output, "[WARN] failed to remove container (defer)")
	})
}
