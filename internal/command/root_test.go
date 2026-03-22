package command

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/config"
	"cderun/internal/logging"
	"cderun/internal/runtime"
)

type terminationMockRuntime struct {
	*runtime.MockRuntime
	isRunning  bool
	inspectErr error
}

type safeBuffer struct {
	buf bytes.Buffer
	mu  sync.Mutex
}

func (s *safeBuffer) Write(p []byte) (n int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *safeBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

func (m *terminationMockRuntime) InspectContainer(ctx context.Context, containerID string) (bool, int, error) {
	return m.isRunning, m.ExitCode, m.inspectErr
}

func executeCommand(args ...string) (string, error) {
	return executeCommandContext(context.Background(), args...)
}

func executeCommandContext(ctx context.Context, args ...string) (string, error) {
	return executeCommandRawContext(ctx, append([]string{"cderun"}, args...))
}

func executeCommandRaw(args []string) (string, error) {
	return executeCommandRawContext(context.Background(), args)
}

func executeCommandRawContext(ctx context.Context, args []string) (string, error) {
	var buf bytes.Buffer

	execErr := ExecuteContextWithOptions(ctx, args, func(o *rootOptions, cmd *cobra.Command) {
		// Default to terminal mode for tests to avoid auto-detection of pipes
		// unless specifically overridden in a test.
		o.isTerminal = func(fd int) bool { return true }
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
	})

	return buf.String(), execErr
}

func TestUnit_Root_PreprocessArgs_HoistingAndPolyglot(t *testing.T) {
	t.Parallel()
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
			name:     "P1 flag with equals",
			args:     []string{"cderun", "sh", "--cderun-tty=true"},
			expected: []string{"cderun", "--cderun-tty=true", "sh"},
		},
		{
			name:     "P1 flag with argument following",
			args:     []string{"cderun", "node", "--cderun-image", "alpine", "app.js"},
			expected: []string{"cderun", "--cderun-image", "alpine", "node", "app.js"},
		},
		{
			name:     "complex hoisting with mixed flags",
			args:     []string{"cderun", "-t", "sh", "-c", "ls", "--cderun-image", "alpine", "--cderun-tty=false"},
			expected: []string{"cderun", "--cderun-image", "alpine", "--cderun-tty=false", "-t", "sh", "-c", "ls"},
		},
		{
			name:     "P1 flag before subcommand (error case)",
			args:     []string{"cderun", "--cderun-image", "alpine", "sh"},
			expected: nil,
		},
		{
			name:     "multiple shorthands, none take argument",
			args:     []string{"cderun", "-it", "sh", "--cderun-tty"},
			expected: []string{"cderun", "--cderun-tty", "-it", "sh"},
		},
		{
			name:     "polyglot with P1 and regular flags",
			args:     []string{"node", "--version", "--cderun-image", "alpine", "-t"},
			expected: []string{"cderun", "--cderun-image", "alpine", "node", "--version", "-t"},
		},
		{
			name:     "no subcommand found (diagnosis mode behavior in preprocess)",
			args:     []string{"cderun", "--diagnosis"},
			expected: []string{"cderun", "--diagnosis"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newRootCmd(&rootOptions{})
			actual, err := preprocessArgs(cmd, tt.args)
			if tt.expected == nil {
				require.Error(t, err)
				require.ErrorContains(t, err, "must be placed after the subcommand")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestUnit_Root_Execute_EmptyArgs(t *testing.T) {
	t.Parallel()
	// Should not panic
	_, err := executeCommandRaw([]string{})
	require.NoError(t, err)

	_, err = executeCommandRaw(nil)
	require.NoError(t, err)
}

func TestUnit_Root_Execution_CommandResolution(t *testing.T) {
	t.Parallel()
	t.Run("executes container correctly", func(t *testing.T) {
		mockRuntime := &runtime.MockRuntime{
			CreatedContainerID: "test-container-id",
			ExitCode:           42,
		}
		var capturedExitCode int

		// Use ExecuteContextWithOptions directly to capture exit code and use mock runtime
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "node:20-alpine", "--tty", "-i", "--network", "host", "node", "--version"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.exitFunc = func(code int) {
				capturedExitCode = code
			}
			o.isTerminal = func(fd int) bool { return true }
		})
		require.NoError(t, err)

		cfg := mockRuntime.GetCreatedConfig()
		require.NotNil(t, cfg)
		assert.Equal(t, "node:20-alpine", cfg.Image)
		assert.Equal(t, []string{"--version"}, cfg.Command)
		assert.True(t, cfg.TTY)
		assert.True(t, cfg.Interactive)
		assert.Equal(t, "host", cfg.Network)
		assert.Equal(t, "test-container-id", mockRuntime.GetStartedContainerID())
		assert.Equal(t, "test-container-id", mockRuntime.GetWaitedContainerID())
		assert.Equal(t, "test-container-id", mockRuntime.GetRemovedContainerID())
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
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "--tty=true", "--cderun-tty=false", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.isTerminal = func(fd int) bool { return true }
		})
		require.NoError(t, err)
		assert.False(t, mockRuntime.CreatedConfig.TTY)
	})

	t.Run("-t shorthand for --tty", func(t *testing.T) {
		mockRuntime := &runtime.MockRuntime{}
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "-t", "--image", "alpine", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.isTerminal = func(fd int) bool { return true }
		})
		require.NoError(t, err)
		assert.True(t, mockRuntime.CreatedConfig.TTY)
	})

	t.Run("returns error for unsupported runtime", func(t *testing.T) {
		_, err := executeCommand("--image", "alpine", "--runtime", "invalid", "sh")
		require.Error(t, err)
		require.ErrorContains(t, err, "unsupported runtime \"invalid\"")
	})

	t.Run("diagnosis mode works without subcommand", func(t *testing.T) {
		output, err := executeCommand("--diagnosis")
		require.NoError(t, err)
		assert.Contains(t, output, "runtime:")
		assert.Contains(t, output, "configs:")
	})

	t.Run("diagnosis mode works with subcommand and takes precedence", func(t *testing.T) {
		output, err := executeCommand("--diagnosis", "node", "--version")
		require.NoError(t, err)
		assert.Contains(t, output, "runtime:")
		assert.Contains(t, output, "configs:")
		assert.NotContains(t, output, "image: node") // Should not be container config dry-run
	})

	t.Run("dry-run requires a subcommand", func(t *testing.T) {
		_, err := executeCommand("--dry-run")
		require.Error(t, err)
		require.ErrorContains(t, err, "--dry-run requires a subcommand")
	})

	t.Run("dry-run outputs configuration and skips execution", func(t *testing.T) {
		// Dry-run with YAML (default)
		output, err := executeCommand("--dry-run", "--image", "alpine", "--env", "K=V", "--mount", "type=bind,source=/s,target=/t", "sh", "echo", "hello")
		require.NoError(t, err)
		assert.Contains(t, output, "image: alpine")
		assert.Contains(t, output, "command:")
		assert.Contains(t, output, "- echo")
		assert.Contains(t, output, "- hello")
		assert.Contains(t, output, "env:")
		assert.Contains(t, output, "- K=V")
		assert.Contains(t, output, "mounts:")
		assert.Contains(t, output, "target: /t")

		// Dry-run with JSON
		output, err = executeCommand("--dry-run", "--dry-run-format", "json", "--image", "alpine", "sh", "echo", "hello")
		require.NoError(t, err)
		assert.Contains(t, output, "\"image\": \"alpine\"")
		assert.Contains(t, output, "\"command\": [")

		// Dry-run with simple
		output, err = executeCommand("--dry-run", "-f", "simple", "--image", "alpine", "--env", "K=V", "--mount", "type=bind,source=/s,target=/t", "--device", "/dev/fuse", "--memory", "512MiB", "--cpus", "2", "sh", "echo", "hello")
		require.NoError(t, err)
		assert.Contains(t, output, "Image: alpine")
		assert.Contains(t, output, "Command: echo hello")
		assert.Contains(t, output, "Env: K=V")
		assert.Contains(t, output, "Mounts: type=bind,source=/s,target=/t,readonly=false")
		assert.Contains(t, output, "Devices: /dev/fuse")
		assert.Contains(t, output, "Memory: 512MiB") // go-units formatting
		assert.Contains(t, output, "CPUs: 2")
	})

	t.Run("returns error if AttachContainer fails", func(t *testing.T) {
		mockRuntime := &runtime.MockRuntime{
			AttachErr: errors.New("attach failed"),
		}
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.isTerminal = func(fd int) bool { return true }
		})
		require.Error(t, err)
		require.ErrorContains(t, err, "failed to attach to container: attach failed")
	})
}

func TestUnit_Root_Flags_MountingAndDevices(t *testing.T) {
	t.Parallel()
	t.Run("workdir, mount and device flags", func(t *testing.T) {
		mockRuntime := &runtime.MockRuntime{}
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "--workdir", "/my/workdir", "--mount", "type=bind,source=/h,target=/c,readonly", "--device", "/dev/fuse:/dev/fuse:rm", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.isTerminal = func(fd int) bool { return true }
		})
		require.NoError(t, err)

		require.NotNil(t, mockRuntime.CreatedConfig)
		assert.Equal(t, "/my/workdir", mockRuntime.CreatedConfig.Workdir)
		require.Len(t, mockRuntime.CreatedConfig.Mounts, 1)
		assert.Equal(t, "/h", mockRuntime.CreatedConfig.Mounts[0].Source)

		require.Len(t, mockRuntime.CreatedConfig.Devices, 1)
		assert.Equal(t, "/dev/fuse", mockRuntime.CreatedConfig.Devices[0].PathOnHost)
	})

	t.Run("mounting flags auto-enable socket mounting", func(t *testing.T) {
		mockRuntime := &runtime.MockRuntime{}
		mfs := &config.MockFileSystem{
			Env: map[string]string{
				"CDERUN_SOCKET_PATH": "/var/run/docker.sock",
			},
		}
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "--mount-cderun", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.isTerminal = func(fd int) bool { return true }
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
		})
		require.NoError(t, err)
		require.NotNil(t, mockRuntime.CreatedConfig)

		socketFound := false
		for _, v := range mockRuntime.CreatedConfig.Mounts {
			if v.Target == "/var/run/docker.sock" {
				socketFound = true
			}
		}
		assert.True(t, socketFound, "Socket should be automatically mounted")
	})
}

func TestUnit_Root_Execution_StrictBehavior(t *testing.T) {
	t.Parallel()
	t.Run("fails when no image mapping found for tool", func(t *testing.T) {
		_, err := executeCommand("unknown-tool", "--version")
		require.Error(t, err)
		require.ErrorContains(t, err, "no image mapping found for tool: unknown-tool")
	})

	t.Run("subcommand is excluded from CMD", func(t *testing.T) {
		mockRuntime := &runtime.MockRuntime{}
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "ls", "-l", "/tmp"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.isTerminal = func(fd int) bool { return true }
		})
		require.NoError(t, err)

		require.NotNil(t, mockRuntime.CreatedConfig)
		assert.Equal(t, []string{"-l", "/tmp"}, mockRuntime.CreatedConfig.Command)
	})
}

func TestUnit_Root_Diagnosis_OutputFormats(t *testing.T) {
	t.Parallel()
	t.Run("JSON format", func(t *testing.T) {
		out := &bytes.Buffer{}
		mfs := &config.MockFileSystem{
			Files: map[string][]byte{
				"/var/run/docker.sock": {},
			},
		}
		opts := &rootOptions{
			fs: mfs,
		}
		resolved := &config.ResolvedConfig{
			Runtime:         "docker",
			SocketPath:      "/var/run/docker.sock",
			Diagnosis:       true,
			DiagnosisFormat: "json",
		}
		cmd := &cobra.Command{}
		cmd.SetOut(out)

		err := opts.handleDiagnosis(cmd, resolved, nil, nil, nil)
		require.NoError(t, err)
		assert.Contains(t, out.String(), "\"name\": \"docker\"")
		assert.Contains(t, out.String(), "\"status\": \"accessible\"")
	})

	t.Run("YAML format (default)", func(t *testing.T) {
		out := &bytes.Buffer{}
		mfs := &config.MockFileSystem{
			Files: map[string][]byte{
				"/var/run/docker.sock": {},
			},
		}
		opts := &rootOptions{
			fs: mfs,
		}
		resolved := &config.ResolvedConfig{
			Runtime:         "docker",
			SocketPath:      "/var/run/docker.sock",
			Diagnosis:       true,
			DiagnosisFormat: "yaml",
		}
		cmd := &cobra.Command{}
		cmd.SetOut(out)

		err := opts.handleDiagnosis(cmd, resolved, nil, nil, nil)
		require.NoError(t, err)
		assert.Contains(t, out.String(), "name: docker")
		assert.Contains(t, out.String(), "status: accessible")
	})

	t.Run("Simple format", func(t *testing.T) {
		out := &bytes.Buffer{}
		mfs := &config.MockFileSystem{
			Files: map[string][]byte{
				"/var/run/docker.sock": {},
			},
		}
		opts := &rootOptions{
			fs: mfs,
		}
		resolved := &config.ResolvedConfig{
			Runtime:         "docker",
			SocketPath:      "/var/run/docker.sock",
			Diagnosis:       true,
			DiagnosisFormat: "simple",
		}
		cmd := &cobra.Command{}
		cmd.SetOut(out)

		err := opts.handleDiagnosis(cmd, resolved, config.ToolsConfig{"node": {}}, []string{"/etc/cderun.yaml"}, []string{".tools.yaml"})
		require.NoError(t, err)
		assert.Contains(t, out.String(), "Runtime: docker (/var/run/docker.sock)")
		assert.Contains(t, out.String(), "Runtime Status: accessible")
		assert.Contains(t, out.String(), "Global Config: /etc/cderun.yaml")
		assert.Contains(t, out.String(), "Tools Config: .tools.yaml")
		assert.Contains(t, out.String(), "Available Tools: node")
	})

	t.Run("Socket not found", func(t *testing.T) {
		out := &bytes.Buffer{}
		mfs := &config.MockFileSystem{} // Empty filesystem
		opts := &rootOptions{
			fs: mfs,
		}
		resolved := &config.ResolvedConfig{
			Runtime:         "docker",
			SocketPath:      "/nonexistent/socket",
			Diagnosis:       true,
			DiagnosisFormat: "simple",
		}
		cmd := &cobra.Command{}
		cmd.SetOut(out)

		err := opts.handleDiagnosis(cmd, resolved, nil, nil, nil)
		require.NoError(t, err)
		assert.Contains(t, out.String(), "Runtime Status: not found or inaccessible")
	})
}

func TestUnit_Root_Cleanup_RemoveContainerWarning(t *testing.T) {
	t.Parallel()
	t.Run("prints warning if RemoveContainer fails", func(t *testing.T) {
		mockRuntime := &runtime.MockRuntime{
			RemoveErr: errors.New("failed to remove"),
		}

		var stderrBuf bytes.Buffer
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.exitFunc = func(code int) {}
			o.isTerminal = func(fd int) bool { return true }
			cmd.SetErr(&stderrBuf)
		})
		require.NoError(t, err)

		assert.Contains(t, stderrBuf.String(), "[WARN] failed to remove container (defer): failed to remove")
	})
}

func TestUnit_Root_Env_StrictEnvFlags(t *testing.T) {
	t.Parallel()
	t.Run("--strict-env flag", func(t *testing.T) {
		mockRuntime := &runtime.MockRuntime{}
		mfs := &config.MockFileSystem{} // Empty environment
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "--strict-env", "--env", "NONEXISTENT", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.exitFunc = func(code int) {}
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
		})
		require.Error(t, err)
		require.ErrorContains(t, err, "required environment variable not found: NONEXISTENT")
	})
}

func TestUnit_Root_DefaultOptions(t *testing.T) {
	t.Parallel()
	o := defaultOptions()
	assert.NotNil(t, o.fs)
	assert.NotNil(t, o.exitFunc)
	assert.NotNil(t, o.isTerminal)
	assert.NotNil(t, o.termGetSize)
	assert.NotNil(t, o.makeRaw)
	assert.NotNil(t, o.restore)
}

func TestUnit_Root_GetFd(t *testing.T) {
	t.Parallel()

	// Test os.Stdin
	fd, ok := getFd(os.Stdin)
	assert.True(t, ok)
	assert.Equal(t, int(os.Stdin.Fd()), fd)

	// Test non-file reader
	buf := bytes.NewBuffer(nil)
	_, ok = getFd(buf)
	assert.False(t, ok)
}

type syncBlockingReader struct {
	entered chan struct{}
	unblock chan struct{}
}

func (r *syncBlockingReader) Read(p []byte) (n int, err error) {
	close(r.entered)
	<-r.unblock
	copy(p, "hello")
	return 5, nil
}

func TestUnit_Root_SyncReader(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	ready := make(chan struct{})
	entered := make(chan struct{})
	unblock := make(chan struct{})
	inner := &syncBlockingReader{entered: entered, unblock: unblock}

	sr := &syncReader{
		inner: inner,
		ready: ready,
		ctx:   ctx,
	}

	// Test before ready
		type result struct {
			n   int
			err error
			p   []byte
		}
		resCh := make(chan result)
		go func() {
			buf := make([]byte, 5)
			n, err := sr.Read(buf)
			resCh <- result{n: n, err: err, p: buf}
		}()

	// Ensure Read has reached the select block but is waiting for ready
	select {
	case <-resCh:
		t.Fatal("Read should have blocked on ready")
	case <-time.After(10 * time.Millisecond):
		// OK
	}

	close(ready)

	// Now ensure it reaches the inner reader's Read method
	<-entered

	// And verify it's still blocked on unblock
	select {
	case <-resCh:
		t.Fatal("Read should have blocked on inner reader")
	case <-time.After(10 * time.Millisecond):
		// OK
	}

	close(unblock)
	res := <-resCh
	assert.Equal(t, 5, res.n)
	require.NoError(t, res.err)
	assert.Equal(t, "hello", string(res.p))
}

func TestUnit_Root_SyncReader_ContextCancel(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	// In Go 1.24+, we should ideally use t.Context().
	// But here we need a manual cancel to test the behavior before calling Read.

	sr := &syncReader{
		inner: strings.NewReader("hello"),
		ready: make(chan struct{}),
		ctx:   ctx,
	}

	cancel()
	p := make([]byte, 5)
	n, err := sr.Read(p)
	assert.Equal(t, 0, n)
	require.ErrorIs(t, err, context.Canceled)
}

func TestUnit_Root_GetHangTimeout(t *testing.T) {
	t.Parallel()
	o := &rootOptions{logger: &logging.Logger{}}

	// Case 1: TTY + Interactive -> Timeout 0
	assert.Equal(t, time.Duration(0), o.getHangTimeout(true, true, nil))

	// Case 2: Resolved HangTimeout > 0
	resolved := &config.ResolvedConfig{HangTimeout: 5 * time.Second}
	assert.Equal(t, 5*time.Second, o.getHangTimeout(false, true, resolved))

	// Case 3: Fallback to default (2s)
	assert.Equal(t, 2*time.Second, o.getHangTimeout(false, false, nil))
}

func TestUnit_Root_ForceTerminateIfRunning(t *testing.T) {
	t.Parallel()
	o := &rootOptions{logger: &logging.Logger{}}

	t.Run("Container already stopped", func(t *testing.T) {
		mockRuntime := &terminationMockRuntime{
			MockRuntime: &runtime.MockRuntime{ExitCode: 123},
			isRunning:   false,
		}
		exitCode, err := o.forceTerminateIfRunning(t.Context(), mockRuntime, "c1")
		require.NoError(t, err)
		assert.Equal(t, 123, exitCode)
		assert.Empty(t, mockRuntime.SignaledContainerID)
	})

	t.Run("Container running, signal success", func(t *testing.T) {
		mockRuntime := &terminationMockRuntime{
			MockRuntime: runtime.NewMockRuntime(),
			isRunning:   true,
		}
		_, err := o.forceTerminateIfRunning(t.Context(), mockRuntime, "c1")
		require.NoError(t, err)
		assert.Equal(t, "c1", mockRuntime.SignaledContainerID)
		assert.Equal(t, "SIGKILL", mockRuntime.Signal)
	})

	t.Run("Inspect fails, fallback to signal", func(t *testing.T) {
		mockRuntime := &terminationMockRuntime{
			MockRuntime: runtime.NewMockRuntime(),
			inspectErr:  errors.New("inspect failed"),
		}
		_, err := o.forceTerminateIfRunning(t.Context(), mockRuntime, "c1")
		require.NoError(t, err)
		assert.Equal(t, "c1", mockRuntime.SignaledContainerID)
	})

	t.Run("Signal fails, ignore error", func(t *testing.T) {
		mockRuntime := &terminationMockRuntime{
			MockRuntime: &runtime.MockRuntime{SignalErr: errors.New("signal failed")},
			isRunning:   true,
		}
		_, err := o.forceTerminateIfRunning(t.Context(), mockRuntime, "c1")
		require.NoError(t, err)
		assert.Equal(t, "c1", mockRuntime.SignaledContainerID)
		assert.Equal(t, "SIGKILL", mockRuntime.Signal)
	})
}

func TestUnit_Root_BuildContainerConfig_Additions(t *testing.T) {
	t.Parallel()

	t.Run("MountCderun without MountCderunPath uses fs.Executable", func(t *testing.T) {
		mfs := &config.MockFileSystem{
			ExecPath: "/usr/bin/cderun-real",
		}
		o := &rootOptions{
			fs:     mfs,
			logger: logging.NewLogger(),
		}
		resolved := &config.ResolvedConfig{
			MountCderun: true,
		}
		cfg, err := o.buildContainerConfig(resolved, nil, nil)
		require.NoError(t, err)

		assertMountSourceEquals(t, cfg.Mounts, "/usr/local/bin/cderun", "/usr/bin/cderun-real")
	})

	t.Run("MountAllTools with empty toolsCfg", func(t *testing.T) {
		var logBuf bytes.Buffer
		logger := logging.NewLogger()
		logger.Init("warn", "text", false)
		logger.SetOutput(&logBuf)

		mfs := &config.MockFileSystem{ExecPath: "/usr/bin/cderun"}
		o := &rootOptions{
			fs:     mfs,
			logger: logger,
		}
		resolved := &config.ResolvedConfig{
			MountAllTools: true,
		}
		_, err := o.buildContainerConfig(resolved, nil, nil)
		require.NoError(t, err)
		assert.Contains(t, logBuf.String(), "--mount-all-tools specified but no tools defined in .tools.yaml")
	})

	t.Run("MountTools with invalid tool name", func(t *testing.T) {
		mfs := &config.MockFileSystem{ExecPath: "/usr/bin/cderun"}
		o := &rootOptions{
			fs:     mfs,
			logger: logging.NewLogger(),
		}
		resolved := &config.ResolvedConfig{
			MountTools: []string{"nonexistent"},
		}
		toolsCfg := config.ToolsConfig{
			"node": {},
		}
		_, err := o.buildContainerConfig(resolved, nil, toolsCfg)
		require.Error(t, err)
		require.ErrorContains(t, err, "tool \"nonexistent\" not found in .tools.yaml")
		require.ErrorContains(t, err, "available tools: node")
	})
}

func TestUnit_Root_BuildContainerConfig_Nested_Additions(t *testing.T) {
	t.Parallel()

	t.Run("nested execution path resolution success", func(t *testing.T) {
		mfs := &config.MockFileSystem{
			ExecPath: "/app/cderun",
		}
		o := &rootOptions{
			fs:     mfs,
			logger: logging.NewLogger(),
		}
		// Simulate nested execution: Level 1.
		resolved := &config.ResolvedConfig{
			MountCderun: true,
			HostContext: &config.HostContext{
				Level: 1,
				Mounts: []config.MountMapping{
					{Source: "/host/app", Target: "/app"},
				},
			},
		}

		cfg, err := o.buildContainerConfig(resolved, nil, nil)
		require.NoError(t, err)

		assertMountSourceEquals(t, cfg.Mounts, "/usr/local/bin/cderun", "/host/app/cderun")
	})

	t.Run("nested execution path resolution respects o.fs", func(t *testing.T) {
		// Use a path that requires o.fs to be used in the ExpressionResolver.
		// {{HOME}} will be resolved using o.fs.
		mfs := &config.MockFileSystem{
			ExecPath: "/app/{{HOME}}/cderun",
			HomeDir:  "myhome",
		}
		o := &rootOptions{
			fs:     mfs,
			logger: logging.NewLogger(),
		}
		// Simulate nested execution: Level 1.
		resolved := &config.ResolvedConfig{
			MountCderun: true,
			HostContext: &config.HostContext{
				Level: 1,
				Mounts: []config.MountMapping{
					{Source: "/host/app", Target: "/app"},
				},
			},
		}

		cfg, err := o.buildContainerConfig(resolved, nil, nil)
		require.NoError(t, err)

		// If o.fs is used, "{{HOME}}" resolves to "myhome".
		// Then "/app/myhome/cderun" is reverse-resolved to "/host/app/myhome/cderun".
		assertMountSourceEquals(t, cfg.Mounts, "/usr/local/bin/cderun", "/host/app/myhome/cderun")
	})
}

func TestUnit_Root_NewRootCmd_PersistentPreRun_Additions(t *testing.T) {
	t.Parallel()

	t.Run("initializes fs and configLoader if nil", func(t *testing.T) {
		o := &rootOptions{}
		cmd := newRootCmd(o)

		// Before PersistentPreRun
		assert.Nil(t, o.fs)
		assert.Nil(t, o.configLoader)

		// Execute PersistentPreRun
		if cmd.PersistentPreRun != nil {
			cmd.PersistentPreRun(cmd, []string{})
		}

		assert.NotNil(t, o.fs)
		assert.NotNil(t, o.configLoader)
	})
}

func TestUnit_RunCderunCore_Errors_Additions(t *testing.T) {
	t.Parallel()

	t.Run("container creation failure", func(t *testing.T) {
		mockRuntime := &runtime.MockRuntime{
			CreateErr: errors.New("creation failed"),
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		var errBuf bytes.Buffer
		err := ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.exitFunc = func(code int) { }
			o.isTerminal = func(fd int) bool { return true }
			cmd.SetErr(&errBuf)
		})

		require.Error(t, err)
		require.ErrorContains(t, err, "creation failed")
	})
}

func TestUnit_Root_NewRootCmd_PersistentPreRun_RespectsExisting(t *testing.T) {
	t.Parallel()

	mfs := &config.MockFileSystem{}
	mcl := config.NewConfigLoaderWithFS(mfs)
	o := &rootOptions{
		fs:           mfs,
		configLoader: mcl,
	}
	cmd := newRootCmd(o)

	// Execute PersistentPreRun
	if cmd.PersistentPreRun != nil {
		cmd.PersistentPreRun(cmd, []string{})
	}

	assert.Same(t, mfs, o.fs)
	assert.Same(t, mcl, o.configLoader)
}

func TestUnit_Root_BuildContainerConfig_Errors(t *testing.T) {
	t.Parallel()

	t.Run("fs.Executable failure", func(t *testing.T) {
		mfs := &config.MockFileSystem{
			ExecErr: errors.New("exec error"),
		}
		o := &rootOptions{
			fs:     mfs,
			logger: logging.NewLogger(),
		}
		resolved := &config.ResolvedConfig{
			MountCderun: true,
		}
		_, err := o.buildContainerConfig(resolved, nil, nil)
		require.Error(t, err)
		require.ErrorContains(t, err, "failed to get executable path: exec error")
	})
}

func TestUnit_Root_BuildContainerConfig_UnresolvedPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		execPath       string
		hostContext    *config.HostContext
		expectedSource string
	}{
		{
			name:     "resolve error in nested level",
			execPath: "/app/{{file:nonexistent}}",
			hostContext: &config.HostContext{
				Level: 1,
			},
			expectedSource: "/app/{{file:nonexistent}}",
		},
		{
			name:     "no resolution in Level 0",
			execPath: "/app/{{HOME}}",
			hostContext: &config.HostContext{
				Level: 0,
			},
			expectedSource: "/app/{{HOME}}",
		},
		{
			name:     "fs.Executable path with unresolved template",
			execPath: "/app/{{file:nonexistent-file-that-should-not-exist}}",
			hostContext: &config.HostContext{
				Level: 1,
			},
			expectedSource: "/app/{{file:nonexistent-file-that-should-not-exist}}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mfs := &config.MockFileSystem{
				ExecPath: tt.execPath,
			}
			o := &rootOptions{
				fs:     mfs,
				logger: logging.NewLogger(),
			}
			resolved := &config.ResolvedConfig{
				MountCderun: true,
				HostContext: tt.hostContext,
			}
			cfg, err := o.buildContainerConfig(resolved, nil, nil)
			require.NoError(t, err)
			require.NotNil(t, cfg)

			assertMountSourceEquals(t, cfg.Mounts, "/usr/local/bin/cderun", tt.expectedSource)
		})
	}
}

func TestUnit_RunCderunCore_ExecuteFailure(t *testing.T) {
	t.Parallel()
	stdout, _, exitCode, err := runCderunCore(nil, "sh")
	require.Error(t, err)
	require.ErrorContains(t, err, "no image mapping found for tool: sh")
	assert.Empty(t, stdout)
	assert.Equal(t, 0, exitCode)
}

func TestUnit_Root_RunE_InvalidPullPolicy(t *testing.T) {
	t.Parallel()
	// Use --image to avoid configuration error about missing tool image mapping
	// And use a mock runtime to avoid actual image pulling
	err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "--pull", "invalid", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
		o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
			return &runtime.MockRuntime{}, nil
		}
		o.isTerminal = func(fd int) bool { return true }
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "invalid pull policy \"invalid\"")
}

func TestUnit_Root_RunE_CleanupSnapshotWarning(t *testing.T) {
	t.Parallel()
	var logBuf bytes.Buffer
	mfs := &config.MockFileSystem{
		RemoveAllErr: errors.New("remove failed"),
	}
	err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "--mount-cderun", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
		o.fs = mfs
		o.configLoader = config.NewConfigLoaderWithFS(mfs)
		o.isTerminal = func(fd int) bool { return true }
		o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
			return &runtime.MockRuntime{}, nil
		}
		cmd.SetErr(&logBuf)
	})
	require.NoError(t, err)
	assert.Contains(t, logBuf.String(), "failed to cleanup snapshot: remove failed")
}

func TestUnit_Root_EarlyLoggerInit_LogLevel(t *testing.T) {
	t.Parallel()
	mfs := &config.MockFileSystem{
		Env: map[string]string{"CDERUN_LOG_LEVEL": "debug"},
	}
	err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
		o.fs = mfs
		o.configLoader = config.NewConfigLoaderWithFS(mfs)
		o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
			return &runtime.MockRuntime{}, nil
		}
		o.isTerminal = func(fd int) bool { return true }
	})
	require.NoError(t, err)
}

func TestUnit_Root_EarlyLoggerInit_CderunLogLevel(t *testing.T) {
	t.Parallel()
	err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "sh", "--cderun-log-level", "trace"}, func(o *rootOptions, cmd *cobra.Command) {
		o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
			return &runtime.MockRuntime{}, nil
		}
		o.isTerminal = func(fd int) bool { return true }
	})
	require.NoError(t, err)
}

func TestUnit_Root_ExecuteWrappers(t *testing.T) {
	t.Parallel()
	// These are just to cover the simple wrapper functions
	require.NoError(t, Execute(nil))
	require.NoError(t, ExecuteContext(context.Background(), nil))
}

func TestUnit_Root_RunE_BuildContainerConfigFailure(t *testing.T) {
	t.Parallel()
	// Trigger buildContainerConfig failure by using --mount-tools with a tool not in .tools.yaml.
	// We need a tool that exists (to pass resolveSettings) but fails in buildContainerConfig.
	// Wait, resolveSettings also checks tool existence if it's the subcommand.
	// If we use --mount-tools=nonexistent, buildContainerConfig will fail.

	mfs := &config.MockFileSystem{}
	err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "--mount-tools", "nonexistent", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
		o.fs = mfs
		o.configLoader = config.NewConfigLoaderWithFS(mfs)
		o.isTerminal = func(fd int) bool { return true }
		o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
			return &runtime.MockRuntime{}, nil
		}
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "container configuration error")
	require.ErrorContains(t, err, "tool \"nonexistent\" not found in .tools.yaml")
}

type rootErrorFS struct {
	wfFunc func(path string, data []byte, perm os.FileMode) error
	*config.MockFileSystem
	mkdirErr error
	writeErr error
}

func (fs *rootErrorFS) MkdirAll(path string, perm os.FileMode) error {
	if fs.mkdirErr != nil {
		return fs.mkdirErr
	}
	return fs.MockFileSystem.MkdirAll(path, perm)
}

func (fs *rootErrorFS) WriteFile(path string, data []byte, perm os.FileMode) error {
	if fs.wfFunc != nil {
		return fs.wfFunc(path, data, perm)
	}
	if fs.writeErr != nil {
		return fs.writeErr
	}
	return fs.MockFileSystem.WriteFile(path, data, perm)
}

func TestUnit_Root_RunE_SnapshotCreationFailure(t *testing.T) {
	t.Parallel()
	var logBuf bytes.Buffer
	mfs := &rootErrorFS{
		MockFileSystem: &config.MockFileSystem{},
		mkdirErr:       os.ErrPermission,
	}
	// MountCderun triggers snapshot creation
	err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "--mount-cderun", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
		o.fs = mfs
		o.configLoader = config.NewConfigLoaderWithFS(mfs)
		o.isTerminal = func(fd int) bool { return true }
		o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
			return &runtime.MockRuntime{}, nil
		}
		cmd.SetErr(&logBuf)
	})
	require.NoError(t, err)
	assert.Contains(t, logBuf.String(), "failed to create snapshot: failed to create snapshot directory")
}

func TestUnit_Root_RunE_LoadConfigFailure(t *testing.T) {
	t.Parallel()
	mfs := &config.MockFileSystem{
		Files: map[string][]byte{
			".cderun.yaml": []byte("invalid-yaml"),
		},
	}
	err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
		o.fs = mfs
		o.configLoader = config.NewConfigLoaderWithFS(mfs)
		o.isTerminal = func(fd int) bool { return true }
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "failed to load cderun config")
}

func TestUnit_Root_Diagnosis_MalformedConfig(t *testing.T) {
	t.Parallel()
	// diagnosis mode with malformed config
	// resolveSettings might fail if config is technically valid YAML but semantically incorrect for resolution.
	// But let's test if it handles it.
	mfs := &config.MockFileSystem{
		Files: map[string][]byte{
			".cderun.yaml": []byte("invalid: ["), // Invalid YAML
		},
	}
	err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--diagnosis"}, func(o *rootOptions, cmd *cobra.Command) {
		o.fs = mfs
		o.configLoader = config.NewConfigLoaderWithFS(mfs)
		o.isTerminal = func(fd int) bool { return true }
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "failed to load cderun config")
}

func TestUnit_Root_DefaultOptions_RuntimeFactory(t *testing.T) {
	t.Parallel()
	o := defaultOptions()

	t.Run("docker runtime success", func(t *testing.T) {
		rt, err := o.runtimeFactory("docker", "/tmp/docker.sock")
		require.NoError(t, err)
		assert.NotNil(t, rt)
	})

	t.Run("podman runtime success", func(t *testing.T) {
		rt, err := o.runtimeFactory("podman", "/tmp/podman.sock")
		require.NoError(t, err)
		assert.NotNil(t, rt)
	})

	t.Run("unsupported runtime", func(t *testing.T) {
		rt, err := o.runtimeFactory("invalid", "")
		require.Error(t, err)
		assert.Nil(t, rt)
		require.ErrorContains(t, err, "unsupported runtime \"invalid\"")
	})
}

func TestUnit_Root_LoadConfigs_Priority(t *testing.T) {
	t.Parallel()

	t.Run("cderun-config flag takes precedence over env", func(t *testing.T) {
		mfs := &config.MockFileSystem{
			Files: map[string][]byte{
				"/f1.yaml": []byte("runtime: podman\n"),
				"/f2.yaml": []byte("runtime: docker\n"),
			},
			Env: map[string]string{
				"CDERUN_CONFIG": "/f2.yaml",
			},
		}
		var buf bytes.Buffer
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "sh", "--cderun-diagnosis", "--cderun-config", "/f1.yaml"}, func(o *rootOptions, cmd *cobra.Command) {
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
			cmd.SetOut(&buf)
		})
		require.NoError(t, err)
		assert.Contains(t, buf.String(), "name: podman")
	})

	t.Run("config flag takes precedence over env", func(t *testing.T) {
		mfs := &config.MockFileSystem{
			Files: map[string][]byte{
				"/f1.yaml": []byte("runtime: podman\n"),
				"/f2.yaml": []byte("runtime: docker\n"),
			},
			Env: map[string]string{
				"CDERUN_CONFIG": "/f2.yaml",
			},
		}
		var buf bytes.Buffer
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--diagnosis", "--config", "/f1.yaml"}, func(o *rootOptions, cmd *cobra.Command) {
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
			cmd.SetOut(&buf)
		})
		require.NoError(t, err)
		assert.Contains(t, buf.String(), "name: podman")
	})

	t.Run("tool-config flag priority", func(t *testing.T) {
		mfs := &config.MockFileSystem{
			Files: map[string][]byte{
				"/t1.yaml": []byte("node: { image: node1 }"),
				"/t2.yaml": []byte("node: { image: node2 }"),
			},
			Env: map[string]string{
				"CDERUN_TOOL_CONFIG": "/t2.yaml",
			},
		}
		var buf bytes.Buffer
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--diagnosis", "--tool-config", "/t1.yaml"}, func(o *rootOptions, cmd *cobra.Command) {
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
			cmd.SetOut(&buf)
		})
		require.NoError(t, err)
		assert.Contains(t, buf.String(), "- node")
		// Verify tool-config specifically selected t1
		assert.Contains(t, buf.String(), "/t1.yaml")
	})
}

func TestUnit_Root_Execute_ErrorPropagation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		setup    func(*runtime.MockRuntime)
		expected string
	}{
		{
			name: "PullImage fails",
			setup: func(m *runtime.MockRuntime) { m.PullErr = errors.New("pull failed") },
			expected: "failed to pull image: pull failed",
		},
		{
			name: "StartContainer fails",
			setup: func(m *runtime.MockRuntime) { m.StartErr = errors.New("start failed") },
			expected: "failed to start container: start failed",
		},
		{
			name: "WaitContainer fails",
			setup: func(m *runtime.MockRuntime) { m.WaitErr = errors.New("wait failed") },
			expected: "failed to wait for container: wait failed",
		},
		{
			name: "CreateContainer fails",
			setup: func(m *runtime.MockRuntime) { m.CreateErr = errors.New("create failed") },
			expected: "failed to create container: create failed",
		},
		{
			name: "AttachContainer fails",
			setup: func(m *runtime.MockRuntime) { m.AttachErr = errors.New("attach failed") },
			expected: "failed to attach to container: attach failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRuntime := &runtime.MockRuntime{}
			tt.setup(mockRuntime)
			err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
				o.runtimeFactory = func(n, s string) (runtime.ContainerRuntime, error) { return mockRuntime, nil }
				o.isTerminal = func(fd int) bool { return false }
				o.exitFunc = func(code int) { }
			})
			require.Error(t, err)
			assert.ErrorContains(t, err, tt.expected)
		})
	}
}

func TestUnit_Root_PreprocessArgs_FlagArguments(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		args     []string
		expected []string
	}{
		{
			name:     "cderun with flag that takes argument and subcommand",
			args:     []string{"cderun", "--config", "myconfig.yaml", "node", "--version"},
			expected: []string{"cderun", "--config", "myconfig.yaml", "node", "--version"},
		},
		{
			name:     "cderun with shorthand flag that takes argument and subcommand",
			args:     []string{"cderun", "-p", "80:80", "node", "--version"},
			expected: []string{"cderun", "-p", "80:80", "node", "--version"},
		},
		{
			name:     "cderun with multiple shorthands and argument for the last one",
			args:     []string{"cderun", "-itp", "80:80", "node", "--version"},
			expected: []string{"cderun", "-itp", "80:80", "node", "--version"},
		},
		{
			name:     "hoisting P1 flag with argument",
			args:     []string{"cderun", "node", "--cderun-image", "alpine", "--version"},
			expected: []string{"cderun", "--cderun-image", "alpine", "node", "--version"},
		},
		{
			name:     "no subcommand found with flag arg",
			args:     []string{"cderun", "-p", "80:80"},
			expected: []string{"cderun", "-p", "80:80"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newRootCmd(&rootOptions{})
			actual, err := preprocessArgs(cmd, tt.args)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestUnit_Signals_Unix_AllSignals(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "SIGINT", getSignalName(syscall.SIGINT))
	assert.Equal(t, "SIGTERM", getSignalName(syscall.SIGTERM))

	// Just ensure they are consistent with what syscall returns on this platform
	killName := syscall.SIGKILL.String()
	quitName := syscall.SIGQUIT.String()
	assert.Equal(t, killName, getSignalName(syscall.SIGKILL))
	assert.Equal(t, quitName, getSignalName(syscall.SIGQUIT))
}

func TestUnit_RunCderunCore_PreprocessError(t *testing.T) {
	t.Parallel()
	// Using a valid dummy reader to avoid potential panic if runCderunCore dereferences stdin.
	_, _, _, err := runCderunCore(strings.NewReader(""), "--cderun-image", "alpine", "sh")
	require.Error(t, err)
	require.ErrorContains(t, err, "must be placed after the subcommand")
}

func TestUnit_Root_ResolveSettings_Coverage(t *testing.T) {
	t.Parallel()
	mfs := &config.MockFileSystem{}
	var buf bytes.Buffer
	err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "--tty", "--dry-run", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
		o.fs = mfs
		o.isTerminal = func(fd int) bool { return true }
		o.runtimeFactory = func(n, s string) (runtime.ContainerRuntime, error) { return &runtime.MockRuntime{}, nil }
		cmd.SetOut(&buf)
	})
	require.NoError(t, err)
	// Check YAML output fields directly as handleDryRun marshals containerConfig
	assert.Contains(t, buf.String(), "image: alpine")
	assert.Contains(t, buf.String(), "tty: true")
}

func TestUnit_Root_Execute_WaitContainer_Interrupted(t *testing.T) {
	t.Parallel()
	mockRuntime := &runtime.MockRuntime{
		WaitErr: errors.New("wait interrupted"),
	}
	err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
		o.runtimeFactory = func(n, s string) (runtime.ContainerRuntime, error) { return mockRuntime, nil }
		o.isTerminal = func(fd int) bool { return false }
		o.exitFunc = func(code int) {}
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "failed to wait for container: wait interrupted")
}

func TestUnit_Root_Execute_AttachEarlyFailure_Error(t *testing.T) {
	t.Parallel()
	// NOTE: MockRuntime.AttachContainer returns AttachErr immediately if it's not nil,
	// which exercises the early-attach-failure path in ExecuteContextWithOptions (via execute).
	mockRuntime := &runtime.MockRuntime{
		AttachErr: errors.New("attach failed early"),
		ExitCode:  42,
	}
	err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
		o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) { return mockRuntime, nil }
		o.isTerminal = func(fd int) bool { return false }
		o.exitFunc = func(code int) {}
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "failed to attach to container: attach failed early")
}

func TestUnit_Root_PreprocessArgs_NoSubcommandHoisting(t *testing.T) {
	t.Parallel()
	cmd := newRootCmd(&rootOptions{})
	// Standard mode, no subcommand found, but has P1 flag after some other flag.
	// preprocessArgs should still work and hoist if it's after where it *thinks* a subcommand might be,
	// but actually subcmdIdx will be -1 if no non-flag arg is found.
	// If subcmdIdx == -1, startIdx is 1.
	args := []string{"cderun", "--config", "c.yaml", "--cderun-tty"}
	expected := []string{"cderun", "--cderun-tty", "--config", "c.yaml"}
	actual, err := preprocessArgs(cmd, args)
	require.NoError(t, err)
	assert.Equal(t, expected, actual)
}

type blockingAttachMockRuntime struct {
	*runtime.MockRuntime
	attached chan struct{}
}

func (m *blockingAttachMockRuntime) AttachContainer(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error {
	close(ready)
	close(m.attached)
	<-ctx.Done() // Block until context is canceled by the timeout logic
	return ctx.Err()
}

func TestUnit_Root_Execute_SignalForwardingFailure_Warning(t *testing.T) {
	t.Parallel()
	mockRuntime := &runtime.MockRuntime{
		SignalErr: errors.New("signal failed"),
		WaitDelay: 1 * time.Second, // Give time to send signal
	}
	var errBuf safeBuffer
	ctx := t.Context()

	var triggerSignal chan<- os.Signal
	setupSignalsMock := func(sigChan chan os.Signal) {
		triggerSignal = sigChan
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(n, s string) (runtime.ContainerRuntime, error) { return mockRuntime, nil }
			o.isTerminal = func(fd int) bool { return false }
			o.exitFunc = func(code int) {}
			o.setupSignals = setupSignalsMock
			o.stopSignalHandling = func(chan os.Signal) {} // No-op
			cmd.SetErr(&errBuf)
		})
	}()

	// Wait for container to start (WaitContainer is called)
	time.Sleep(100 * time.Millisecond)

	// Trigger the signal manually via the captured channel
	require.NotNil(t, triggerSignal)
	triggerSignal <- syscall.SIGINT

	// Wait for execution to finish
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for execution")
	}

	assert.Contains(t, errBuf.String(), "[WARN] failed to forward signal")
	assert.Contains(t, errBuf.String(), "signal failed")
}

func TestUnit_Root_Execute_AttachGracePeriodTimeout_DebugLog(t *testing.T) {
	// Cannot use t.Parallel() because it depends on the 5s constant
	// and we want to capture the specific debug log.
	attached := make(chan struct{})
	mockRuntime := &blockingAttachMockRuntime{
		MockRuntime: runtime.NewMockRuntime(),
		attached:    attached,
	}

	var logBuf safeBuffer
	// We MUST NOT use a fresh logger if we want to bypass the Init calls in RunE.
	// Actually, ExecuteContextWithOptions(..., setup) allows us to inject things.
	// But RunE calls o.logger.Init twice.
	// The second one uses resolved.LogLevel.
	// So we should just set --cderun-log-level=debug in args.

	ctx := t.Context()

	errCh := make(chan error, 1)
	go func() {
		errCh <- ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "--cderun-log-level", "debug", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(n, s string) (runtime.ContainerRuntime, error) { return mockRuntime, nil }
			o.isTerminal = func(fd int) bool { return false }
			o.exitFunc = func(code int) {}
			cmd.SetErr(&logBuf)
		})
	}()

	// Wait for attachment to be established
	select {
	case <-attached:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for attachment")
	}

	// Wait for execution to finish (it should wait 5s for the grace period)
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for execution")
	}

	assert.Contains(t, logBuf.String(), "AttachContainer timed out after container exit")
}

func TestUnit_Root_PreprocessArgs_UnknownP1Flag(t *testing.T) {
	t.Parallel()
	cmd := newRootCmd(&rootOptions{})
	// Hoisted flag that is not in the flag set (coverage for f == nil)
	args := []string{"cderun", "sh", "--cderun-unknown", "value"}
	expected := []string{"cderun", "--cderun-unknown", "sh", "value"}
	actual, err := preprocessArgs(cmd, args)
	require.NoError(t, err)
	assert.Equal(t, expected, actual)
}
