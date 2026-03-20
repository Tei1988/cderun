package command

import (
	"cderun/internal/logging"
	"time"
	"os"
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/config"
	"cderun/internal/runtime"
)
type terminationMockRuntime struct {
	*runtime.MockRuntime
	isRunning  bool
	inspectErr error
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
				assert.Contains(t, err.Error(), "must be placed after the subcommand")
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
		assert.Contains(t, err.Error(), "unsupported runtime \"invalid\"")
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
		assert.Contains(t, err.Error(), "--dry-run requires a subcommand")
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
		assert.Contains(t, err.Error(), "failed to attach to container: attach failed")
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
		assert.Contains(t, err.Error(), "no image mapping found for tool: unknown-tool")
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
		assert.Contains(t, err.Error(), "required environment variable not found: NONEXISTENT")
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

func TestUnit_Root_SyncReader(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ready := make(chan struct{})
	inner := strings.NewReader("hello")

	sr := &syncReader{
		inner: inner,
		ready: ready,
		ctx:   ctx,
	}

	// Test before ready
	p := make([]byte, 5)
	done := make(chan bool)
	go func() {
		n, err := sr.Read(p)
		assert.Equal(t, 5, n)
		assert.NoError(t, err)
		assert.Equal(t, "hello", string(p))
		done <- true
	}()

	select {
	case <-done:
		t.Fatal("Read should have blocked")
	case <-time.After(10 * time.Millisecond):
		// OK
	}

	close(ready)
	<-done
}

func TestUnit_Root_SyncReader_ContextCancel(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())

	sr := &syncReader{
		inner: strings.NewReader("hello"),
		ready: make(chan struct{}),
		ctx:   ctx,
	}

	cancel()
	p := make([]byte, 5)
	n, err := sr.Read(p)
	assert.Equal(t, 0, n)
	assert.ErrorIs(t, err, context.Canceled)
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
		exitCode, err := o.forceTerminateIfRunning(context.Background(), mockRuntime, "c1")
		require.NoError(t, err)
		assert.Equal(t, 123, exitCode)
		assert.Equal(t, "", mockRuntime.SignaledContainerID)
	})

	t.Run("Container running, signal success", func(t *testing.T) {
		mockRuntime := &terminationMockRuntime{
			MockRuntime: runtime.NewMockRuntime(),
			isRunning:   true,
		}
		_, err := o.forceTerminateIfRunning(context.Background(), mockRuntime, "c1")
		require.NoError(t, err)
		assert.Equal(t, "c1", mockRuntime.SignaledContainerID)
		assert.Equal(t, "SIGKILL", mockRuntime.Signal)
	})

	t.Run("Inspect fails, fallback to signal", func(t *testing.T) {
		mockRuntime := &terminationMockRuntime{
			MockRuntime: runtime.NewMockRuntime(),
			inspectErr:  errors.New("inspect failed"),
		}
		_, err := o.forceTerminateIfRunning(context.Background(), mockRuntime, "c1")
		require.NoError(t, err)
		assert.Equal(t, "c1", mockRuntime.SignaledContainerID)
	})

	t.Run("Signal fails, ignore error", func(t *testing.T) {
		mockRuntime := &terminationMockRuntime{
			MockRuntime: &runtime.MockRuntime{SignalErr: errors.New("signal failed")},
			isRunning:   true,
		}
		_, err := o.forceTerminateIfRunning(context.Background(), mockRuntime, "c1")
		require.NoError(t, err)
	})
}
