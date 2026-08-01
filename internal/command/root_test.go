package command

import (
	"bytes"
	"context"
	"errors"
	"fmt"
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
	"golang.org/x/term"

	"cderun/internal/config"
	"cderun/internal/container"
	"cderun/internal/logging"
	"cderun/internal/runtime"
)

type safeBuffer struct {
	buf bytes.Buffer
	mu  sync.Mutex
}

var _ io.Writer = (*safeBuffer)(nil)
var _ fmt.Stringer = (*safeBuffer)(nil)

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

type mockFdWriter struct {
	io.Writer
}

func (m mockFdWriter) Fd() uintptr {
	return 1
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
		o.exitFunc = func(int) {}
		// Default to terminal mode for tests to avoid auto-detection of pipes
		// unless specifically overridden in a test.
		o.isTerminal = func(fd int) bool { return true }
		// Default to no-op for exitFunc to prevent tests from terminating the process.
		o.exitFunc = func(code int) {}
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
			name:     "P1 flag with argument following (equals-sign format required)",
			args:     []string{"cderun", "node", "--cderun-image=alpine", "app.js"},
			expected: []string{"cderun", "--cderun-image=alpine", "node", "app.js"},
		},
		{
			name:     "boolean P1 flag followed by an argument (should not eat next arg)",
			args:     []string{"cderun", "node", "--cderun-tty", "app.js"},
			expected: []string{"cderun", "--cderun-tty", "node", "app.js"},
		},
		{
			name:     "complex hoisting with mixed flags",
			args:     []string{"cderun", "-t", "sh", "-c", "ls", "--cderun-image=alpine", "--cderun-tty=false"},
			expected: []string{"cderun", "--cderun-image=alpine", "--cderun-tty=false", "-t", "sh", "-c", "ls"},
		},
		{
			name:     "P1 flag before subcommand (error case)",
			args:     []string{"cderun", "--cderun-image=alpine", "sh"},
			expected: nil,
		},
		{
			name:     "multiple shorthands, none take argument",
			args:     []string{"cderun", "-it", "sh", "--cderun-tty"},
			expected: []string{"cderun", "--cderun-tty", "-it", "sh"},
		},
		{
			name:     "polyglot with P1 and regular flags",
			args:     []string{"node", "--version", "--cderun-image=alpine", "-t"},
			expected: []string{"cderun", "--cderun-image=alpine", "node", "--version", "-t"},
		},
		{
			name:     "no subcommand found (diagnosis mode behavior in preprocess)",
			args:     []string{"cderun", "--diagnosis"},
			expected: []string{"cderun", "--diagnosis"},
		},
		{
			name:     "no subcommand found but has flags and arguments for flags",
			args:     []string{"cderun", "--config", "my.yaml"},
			expected: []string{"cderun", "--config", "my.yaml"},
		},
		{
			name:     "shorthand flag at the end",
			args:     []string{"cderun", "-t"},
			expected: []string{"cderun", "-t"},
		},
		{
			name:     "polyglot mode with executable only",
			args:     []string{"node"},
			expected: []string{"cderun", "node"},
		},
		{
			name:     "shorthand cluster with argument at the end",
			args:     []string{"cderun", "-itp", "8080", "node"},
			expected: []string{"cderun", "-itp", "8080", "node"},
		},
		{
			name:     "P1 flag at the end of standard mode",
			args:     []string{"cderun", "node", "--cderun-tty"},
			expected: []string{"cderun", "--cderun-tty", "node"},
		},
		{
			name:     "Polyglot mode with flags before and after",
			args:     []string{"node", "-v", "--cderun-tty", "app.js"},
			expected: []string{"cderun", "--cderun-tty", "node", "-v", "app.js"},
		},
		{
			name:     "Multiple P1 flags and mixed with others",
			args:     []string{"cderun", "sh", "-c", "echo", "--cderun-tty", "--cderun-interactive", "-x"},
			expected: []string{"cderun", "--cderun-tty", "--cderun-interactive", "sh", "-c", "echo", "-x"},
		},
		{
			name:     "P1 flag with value and more arguments",
			args:     []string{"cderun", "node", "--cderun-image=node:20", "app.js", "--foo"},
			expected: []string{"cderun", "--cderun-image=node:20", "node", "app.js", "--foo"},
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
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.isTerminal = func(fd int) bool { return true }
		})
		if err != nil {
			var exitErr *ExitCodeError
			if errors.As(err, &exitErr) {
				capturedExitCode = exitErr.Code
			} else {
				require.NoError(t, err)
			}
		} else {
			capturedExitCode = 0
		}

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
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "--tty=true", "sh", "--cderun-tty=false"}, func(o *rootOptions, cmd *cobra.Command) {
			o.exitFunc = func(int) {}
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
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
			o.exitFunc = func(int) {}
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
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
		require.ErrorContains(t, err, "unsupported runtime: \"invalid\"")
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
		// Note: env is masked as [REDACTED] by default because sensitive-env is unset.
		output, err := executeCommand("--dry-run", "--image", "alpine", "--env", "K=V", "--mount", "type=bind,source=/s,target=/t", "sh", "echo", "hello")
		require.NoError(t, err)
		assert.Contains(t, output, "image: alpine")
		assert.Contains(t, output, "command:")
		assert.Contains(t, output, "- echo")
		assert.Contains(t, output, "- hello")
		assert.Contains(t, output, "env:")
		assert.Contains(t, output, "- K=[REDACTED]")
		assert.Contains(t, output, "mounts:")
		assert.Contains(t, output, "target: /t")

		// Dry-run with JSON
		output, err = executeCommand("--dry-run", "--dry-run-format", "json", "--image", "alpine", "sh", "echo", "hello")
		require.NoError(t, err)
		assert.Contains(t, output, "\"image\": \"alpine\"")
		assert.Contains(t, output, "\"command\": [")

		// Dry-run with simple
		output, err = executeCommand("--dry-run", "-f", "simple", "--image", "alpine", "--env", "K=V", "--mount", "type=bind,source=/s,target=/t", "--device", "/dev/fuse", "--device", "/dev/snd:/dev/snd:rw", "--memory", "512MiB", "--cpus", "2", "sh", "echo", "hello")
		require.NoError(t, err)
		assert.Contains(t, output, "Image: alpine")
		assert.Contains(t, output, "Command: \"echo\" \"hello\"")
		assert.Contains(t, output, "Env: \"K\"=\"[REDACTED]\"")
		assert.Contains(t, output, "Mounts: type=bind,source=\"/s\",target=\"/t\",readonly=false")
		assert.Contains(t, output, "Devices: /dev/fuse, /dev/snd:/dev/snd:rw")
		assert.Contains(t, output, "Memory: 512MiB") // go-units formatting
		assert.Contains(t, output, "CPUs: 2")
	})

	t.Run("dry-run simple format exhaustive", func(t *testing.T) {
		output, err := executeCommand("--dry-run", "-f", "simple",
			"--image", "alpine",
			"--entrypoint", "/bin/sh",
			"--user", "1000:1000",
			"--hostname", "myhost",
			"--network", "bridge",
			"--dns", "8.8.8.8",
			"--add-host", "host:1.2.3.4",
			"--publish", "80:80",
			"--expose", "8080",
			"--privileged",
			"--cap-add", "SYS_ADMIN",
			"--cap-drop", "KILL",
			"sh", "echo", "hello")
		require.NoError(t, err)
		assert.Contains(t, output, "Entrypoint: \"/bin/sh\"")
		assert.Contains(t, output, "User: 1000:1000")
		assert.Contains(t, output, "Hostname: myhost")
		assert.Contains(t, output, "Network: bridge")
		assert.Contains(t, output, "DNS: 8.8.8.8")
		assert.Contains(t, output, "AddHosts: host:1.2.3.4")
		assert.Contains(t, output, "Ports: 80:80")
		assert.Contains(t, output, "Expose: 8080")
		assert.Contains(t, output, "Privileged: true")
		assert.Contains(t, output, "CapAdd: SYS_ADMIN")
		assert.Contains(t, output, "CapDrop: KILL")
	})

	t.Run("returns error if AttachContainer fails", func(t *testing.T) {
		mockRuntime := &runtime.MockRuntime{
			AttachErr: errors.New("attach failed"),
		}
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
			o.exitFunc = func(int) {}
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
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
			o.exitFunc = func(int) {}
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
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
			o.exitFunc = func(int) {}
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
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
		var imgErr *config.ImageNotFoundError
		require.ErrorAs(t, err, &imgErr)
		assert.Equal(t, "unknown-tool", imgErr.Tool)
	})

	t.Run("subcommand is excluded from CMD", func(t *testing.T) {
		mockRuntime := &runtime.MockRuntime{}
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "ls", "-l", "/tmp"}, func(o *rootOptions, cmd *cobra.Command) {
			o.exitFunc = func(int) {}
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
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

	t.Run("JSON format exhaustive", func(t *testing.T) {
		out := &bytes.Buffer{}
		mfs := &config.MockFileSystem{
			Files: map[string][]byte{"/var/run/docker.sock": {}},
		}
		opts := &rootOptions{fs: mfs}
		resolved := &config.ResolvedConfig{
			Runtime:         "docker",
			SocketPath:      "/var/run/docker.sock",
			Diagnosis:       true,
			DiagnosisFormat: "json",
		}
		cmd := &cobra.Command{}
		cmd.SetOut(out)

		err := opts.handleDiagnosis(cmd, resolved, config.ToolsConfig{"node": {}}, []string{"/etc/cderun.yaml"}, []string{".tools.yaml"})
		require.NoError(t, err)
		assert.Contains(t, out.String(), "\"name\": \"docker\"")
		assert.Contains(t, out.String(), "\"available_tools\":")
		assert.Contains(t, out.String(), "\"node\"")
		assert.Contains(t, out.String(), "\"global\":")
		assert.Contains(t, out.String(), "\"/etc/cderun.yaml\"")
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
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
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
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.exitFunc = func(code int) {}
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
		})
		require.Error(t, err)
		require.ErrorContains(t, err, "required environment variable not found: \"NONEXISTENT\"")
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
	// We wrap t.Context() with WithCancel to allow manual cancellation
	// for testing the behavior when the context is already cancelled.
	ctx, cancel := context.WithCancel(t.Context())

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

	// Case 3: Fallback to default (10s)
	assert.Equal(t, 10*time.Second, o.getHangTimeout(false, false, nil))

	// Case 4: Explicit 0
	resolved0 := &config.ResolvedConfig{HangTimeout: 0}
	assert.Equal(t, time.Duration(0), o.getHangTimeout(false, false, resolved0))
}

func TestUnit_Root_Execute_HangTimeout_Zero_InfiniteWait(t *testing.T) {
	t.Parallel()
	waitStarted := make(chan struct{})
	waitUnblock := make(chan struct{})
	mockRuntime := &hangTimeoutMockRuntime{
		MockRuntime: runtime.NewMockRuntime(),
		waitStarted: waitStarted,
		isRunning:   true,
	}
	mockRuntime.WaitFunc = func(ctx context.Context, id string) (int, error) {
		close(waitStarted)
		<-waitUnblock
		return 0, nil
	}

	var logBuf safeBuffer
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "sh", "--cderun-log-level=trace", "--cderun-hang-timeout=0"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(n, s string, l *logging.Logger) (runtime.ContainerRuntime, error) { return mockRuntime, nil }
			o.isTerminal = func(fd int) bool { return false }
			o.exitFunc = func(code int) {}
			cmd.SetErr(&logBuf)
		})
	}()

	// Wait for container to start (WaitContainer is called)
	select {
	case <-waitStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for WaitContainer to be called")
	}

	// Wait a bit to ensure it's not killing the container immediately
	select {
	case err := <-errCh:
		t.Fatalf("Execute finished prematurely: %v", err)
	case <-time.After(200 * time.Millisecond):
		// Still waiting, good.
	}

	// Unblock WaitContainer
	close(waitUnblock)

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for execution to finish")
	}

	assert.Contains(t, logBuf.String(), "IO finished, waiting indefinitely for container")
	assert.Empty(t, mockRuntime.Signal)
}

func TestUnit_Root_SignalKillIfRunning(t *testing.T) {
	t.Parallel()
	o := &rootOptions{logger: &logging.Logger{}}

	t.Run("Container already stopped", func(t *testing.T) {
		mockRuntime := &TerminationMockRuntime{
			MockRuntime: &runtime.MockRuntime{ExitCode: 123},
			IsRunning:   false,
		}
		exitCode, err := o.signalKillIfRunning(t.Context(), mockRuntime, "c1")
		require.NoError(t, err)
		assert.Equal(t, 123, exitCode)
		assert.Empty(t, mockRuntime.SignaledContainerID)
	})

	t.Run("Container running, signal success", func(t *testing.T) {
		mockRuntime := &TerminationMockRuntime{
			MockRuntime: runtime.NewMockRuntime(),
			IsRunning:   true,
		}
		_, err := o.signalKillIfRunning(t.Context(), mockRuntime, "c1")
		require.NoError(t, err)
		assert.Equal(t, "c1", mockRuntime.SignaledContainerID)
		assert.Equal(t, "SIGKILL", mockRuntime.Signal)
	})

	t.Run("Inspect fails, fallback to signal", func(t *testing.T) {
		mockRuntime := &TerminationMockRuntime{
			MockRuntime: runtime.NewMockRuntime(),
			InspectErr:  errors.New("inspect failed"),
		}
		_, err := o.signalKillIfRunning(t.Context(), mockRuntime, "c1")
		require.NoError(t, err)
		assert.Equal(t, "c1", mockRuntime.SignaledContainerID)
	})

	t.Run("Signal fails, propagate error", func(t *testing.T) {
		mockRuntime := &TerminationMockRuntime{
			MockRuntime: &runtime.MockRuntime{SignalErr: errors.New("signal failed")},
			IsRunning:   true,
		}
		_, err := o.signalKillIfRunning(t.Context(), mockRuntime, "c1")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "signal failed")
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
			WD:       "/app",
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
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.exitFunc = func(code int) {}
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
	var imgErr *config.ImageNotFoundError
	require.ErrorAs(t, err, &imgErr)
	assert.Equal(t, "sh", imgErr.Tool)
	assert.Empty(t, stdout)
	assert.Equal(t, 0, exitCode)
}

func TestUnit_Root_RunE_InvalidPullPolicy(t *testing.T) {
	t.Parallel()
	// Use --image to avoid configuration error about missing tool image mapping
	// And use a mock runtime to avoid actual image pulling
	err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "--pull", "invalid", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
		o.exitFunc = func(int) {}
		o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
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
		o.exitFunc = func(int) {}
		o.fs = mfs
		o.configLoader = config.NewConfigLoaderWithFS(mfs)
		o.isTerminal = func(fd int) bool { return true }
		o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
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
		o.exitFunc = func(int) {}
		o.fs = mfs
		o.configLoader = config.NewConfigLoaderWithFS(mfs)
		o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
			return &runtime.MockRuntime{}, nil
		}
		o.isTerminal = func(fd int) bool { return true }
	})
	require.NoError(t, err)
}

func TestUnit_Root_EarlyLoggerInit_CderunLogLevel(t *testing.T) {
	t.Parallel()
	err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "sh", "--cderun-log-level=trace"}, func(o *rootOptions, cmd *cobra.Command) {
		o.exitFunc = func(int) {}
		o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
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
		o.exitFunc = func(int) {}
		o.fs = mfs
		o.configLoader = config.NewConfigLoaderWithFS(mfs)
		o.isTerminal = func(fd int) bool { return true }
		o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
			return &runtime.MockRuntime{}, nil
		}
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "container configuration error")
	require.ErrorContains(t, err, "tool \"nonexistent\" not found in .tools.yaml")
}

func TestUnit_Root_RunE_SnapshotCreationFailure(t *testing.T) {
	t.Parallel()
	var logBuf bytes.Buffer
	mfs := &RootErrorFS{
		MockFileSystem: &config.MockFileSystem{},
		MkdirErr:       os.ErrPermission,
	}
	// MountCderun triggers snapshot creation
	err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "--mount-cderun", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
		o.exitFunc = func(int) {}
		o.fs = mfs
		o.configLoader = config.NewConfigLoaderWithFS(mfs)
		o.isTerminal = func(fd int) bool { return true }
		o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
			return &runtime.MockRuntime{}, nil
		}
		cmd.SetErr(&logBuf)
	})
	// Now this should fail because --mount-cderun is an explicit request
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create snapshot for nested execution: failed to create snapshot directory: permission denied")
}

func TestUnit_Root_RunE_LoadConfigFailure(t *testing.T) {
	t.Parallel()
	mfs := &config.MockFileSystem{
		Files: map[string][]byte{
			".cderun.yaml": []byte("invalid-yaml"),
		},
	}
	err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
		o.exitFunc = func(int) {}
		o.fs = mfs
		o.configLoader = config.NewConfigLoaderWithFS(mfs)
		o.isTerminal = func(fd int) bool { return true }
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "failed to load cderun config")
}

func TestUnit_Root_RunE_LoadToolsConfigFailure(t *testing.T) {
	t.Parallel()
	mfs := &config.MockFileSystem{
		Files: map[string][]byte{
			".tools.yaml": []byte("invalid-yaml: ["),
		},
	}
	err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
		o.exitFunc = func(int) {}
		o.fs = mfs
		o.configLoader = config.NewConfigLoaderWithFS(mfs)
		o.isTerminal = func(fd int) bool { return true }
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "failed to load tools config")
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
		o.exitFunc = func(int) {}
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
		rt, err := o.runtimeFactory("docker", "/tmp/docker.sock", o.logger)
		require.NoError(t, err)
		assert.NotNil(t, rt)
	})

	t.Run("podman runtime success", func(t *testing.T) {
		rt, err := o.runtimeFactory("podman", "/tmp/podman.sock", o.logger)
		require.NoError(t, err)
		assert.NotNil(t, rt)
	})

	t.Run("containerd runtime implemented", func(t *testing.T) {
		rt, err := o.runtimeFactory("containerd", "/run/containerd/containerd.sock", o.logger)
		require.NoError(t, err)
		assert.Equal(t, "containerd", rt.Name())
		rt.Close()
	})
	t.Run("unsupported runtime", func(t *testing.T) {
		rt, err := o.runtimeFactory("invalid", "", o.logger)
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
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "sh", "--cderun-diagnosis", "--cderun-config=/f1.yaml"}, func(o *rootOptions, cmd *cobra.Command) {
			o.exitFunc = func(int) {}
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
			o.exitFunc = func(int) {}
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
			o.exitFunc = func(int) {}
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
			name:     "PullImage fails",
			setup:    func(m *runtime.MockRuntime) { m.PullErr = errors.New("pull failed") },
			expected: "failed to pull image: pull failed",
		},
		{
			name:     "StartContainer fails",
			setup:    func(m *runtime.MockRuntime) { m.StartErr = errors.New("start failed") },
			expected: "failed to start container: start failed",
		},
		{
			name:     "WaitContainer fails",
			setup:    func(m *runtime.MockRuntime) { m.WaitErr = errors.New("wait failed") },
			expected: "failed to wait for container: wait failed",
		},
		{
			name:     "CreateContainer fails",
			setup:    func(m *runtime.MockRuntime) { m.CreateErr = errors.New("create failed") },
			expected: "failed to create container: create failed",
		},
		{
			name:     "AttachContainer fails",
			setup:    func(m *runtime.MockRuntime) { m.AttachErr = errors.New("attach failed") },
			expected: "failed to attach to container: attach failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRuntime := &runtime.MockRuntime{}
			tt.setup(mockRuntime)
			err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
				o.runtimeFactory = func(n, s string, l *logging.Logger) (runtime.ContainerRuntime, error) { return mockRuntime, nil }
				o.isTerminal = func(fd int) bool { return false }
				o.exitFunc = func(code int) {}
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
			args:     []string{"cderun", "node", "--cderun-image=alpine", "--version"},
			expected: []string{"cderun", "--cderun-image=alpine", "node", "--version"},
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
	assert.Equal(t, "SIGHUP", getSignalName(syscall.SIGHUP))
	assert.Equal(t, "SIGQUIT", getSignalName(syscall.SIGQUIT))

	// Just ensure they are consistent with what syscall returns on this platform
	killName := syscall.SIGKILL.String()
	assert.Equal(t, killName, getSignalName(syscall.SIGKILL))
}

func TestUnit_RunCderunCore_PreprocessError(t *testing.T) {
	t.Parallel()
	// Using a valid dummy reader to avoid potential panic if runCderunCore dereferences stdin.
	_, _, _, err := runCderunCore(strings.NewReader(""), "--cderun-image", "alpine", "sh")
	require.Error(t, err)
	require.ErrorContains(t, err, "must be placed after the subcommand")
}

func TestUnit_RunCderunCore_WithStdin(t *testing.T) {
	t.Parallel()
	// We want to verify that runCderunCore correctly propagates stdin to the executed command.
	// We use ExecuteContextWithOptions directly via runCderunCore.
	// To verify this without a real runtime, we mock the runtime to read from stdin and write to stdout.

	stdinContent := "hello from stdin"
	stdin := strings.NewReader(stdinContent)

	mockRuntime := &runtime.MockRuntime{
		AttachFunc: func(ctx context.Context, id string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error {
			close(ready)
			if stdin != nil {
				_, _ = io.Copy(stdout, stdin)
			}
			return nil
		},
	}

	var outBuf, errBuf bytes.Buffer
	ctx := context.Background()

	execErr := ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "-i", "cat"}, func(o *rootOptions, cmd *cobra.Command) {
		o.runtimeFactory = func(n, s string, l *logging.Logger) (runtime.ContainerRuntime, error) { return mockRuntime, nil }
		o.exitFunc = func(code int) {}
		cmd.SetIn(stdin)
		cmd.SetOut(&outBuf)
		cmd.SetErr(&errBuf)
	})

	require.NoError(t, execErr)
	assert.Equal(t, stdinContent, outBuf.String())
}

func TestUnit_Root_ResolveSettings_Coverage(t *testing.T) {
	t.Parallel()
	mfs := &config.MockFileSystem{}
	var buf bytes.Buffer
	err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "--tty", "--dry-run", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
		o.exitFunc = func(int) {}
		o.fs = mfs
		o.isTerminal = func(fd int) bool { return true }
		o.runtimeFactory = func(n, s string, l *logging.Logger) (runtime.ContainerRuntime, error) {
			return &runtime.MockRuntime{}, nil
		}
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
		o.runtimeFactory = func(n, s string, l *logging.Logger) (runtime.ContainerRuntime, error) { return mockRuntime, nil }
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
		o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
			return mockRuntime, nil
		}
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
	if m.attached != nil {
		close(m.attached)
	}
	<-ctx.Done() // Block until context is canceled by the timeout logic
	return ctx.Err()
}

type syncWaitMockRuntime struct {
	*runtime.MockRuntime
	waitStarted chan struct{}
}

func (m *syncWaitMockRuntime) WaitContainer(ctx context.Context, containerID string) (int, error) {
	close(m.waitStarted)
	return m.MockRuntime.WaitContainer(ctx, containerID)
}

func TestUnit_Root_Execute_SignalForwardingFailure_Warning(t *testing.T) {
	t.Parallel()
	waitStarted := make(chan struct{})
	mockRuntime := &syncWaitMockRuntime{
		MockRuntime: &runtime.MockRuntime{
			SignalErr: errors.New("signal failed"),
			WaitDelay: 1 * time.Second, // Give time to send signal
		},
		waitStarted: waitStarted,
	}
	var errBuf safeBuffer
	ctx := t.Context()

	sigChanHolder := make(chan chan os.Signal, 1)
	setupSignalsMock := func(sigChan chan os.Signal) {
		sigChanHolder <- sigChan
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(n, s string, l *logging.Logger) (runtime.ContainerRuntime, error) { return mockRuntime, nil }
			o.isTerminal = func(fd int) bool { return false }
			o.exitFunc = func(code int) {}
			o.setupSignals = setupSignalsMock
			o.stopSignalHandling = func(chan os.Signal) {} // No-op
			cmd.SetErr(&errBuf)
		})
	}()

	// Wait for container to start (WaitContainer is called)
	select {
	case <-waitStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for WaitContainer to be called")
	}

	// Trigger the signal manually via the captured channel
	var triggerSignal chan os.Signal
	select {
	case triggerSignal = <-sigChanHolder:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for signal channel registration")
	}
	require.NotNil(t, triggerSignal)
	triggerSignal <- syscall.SIGINT

	// Wait for execution to finish
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for execution")
	}

	assert.Contains(t, errBuf.String(), "[WARN] failed to forward signal")
	assert.Contains(t, errBuf.String(), "signal failed")
}

func TestUnit_Root_Execute_AttachGracePeriodTimeout_DebugLog(t *testing.T) {
	t.Parallel()
	attached := make(chan struct{})
	mockRuntime := &blockingAttachMockRuntime{
		MockRuntime: runtime.NewMockRuntime(),
		attached:    attached,
	}

	var logBuf safeBuffer
	ctx := t.Context()

	errCh := make(chan error, 1)
	go func() {
		errCh <- ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "sh", "--cderun-log-level=debug"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(n, s string, l *logging.Logger) (runtime.ContainerRuntime, error) { return mockRuntime, nil }
			o.isTerminal = func(fd int) bool { return false }
			o.exitFunc = func(code int) {}
			o.attachGracePeriod = 100 * time.Millisecond
			cmd.SetErr(&logBuf)
		})
	}()

	// Wait for attachment to be established
	select {
	case <-attached:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for attachment")
	}

	// Wait for execution to finish (it should wait 100ms for the grace period)
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for execution")
	}

	assert.Contains(t, logBuf.String(), "AttachContainer timed out after container exit")
}

func TestUnit_Root_Execute_AttachFailureAfterExit(t *testing.T) {
	t.Parallel()
	// To reach the error check for attachDone after waitDone:
	// 1. WaitContainer must finish.
	// 2. AttachContainer must return an error (not Canceled) within the grace period.

	attachStarted := make(chan struct{})
	waitDone := make(chan struct{})

	mockRuntime := &runtime.MockRuntime{
		AttachFunc: func(ctx context.Context, id string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error {
			close(ready)
			close(attachStarted)
			// Block until WaitContainer finishes
			select {
			case <-waitDone:
				return errors.New("attach error after exit")
			case <-ctx.Done():
				return ctx.Err()
			}
		},
		WaitFunc: func(ctx context.Context, id string) (int, error) {
			// Signal that WaitContainer is finishing
			defer close(waitDone)
			return 77, nil
		},
	}

	err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
		o.runtimeFactory = func(n, s string, l *logging.Logger) (runtime.ContainerRuntime, error) { return mockRuntime, nil }
		o.isTerminal = func(fd int) bool { return false }
		o.exitFunc = func(code int) {}
		o.attachGracePeriod = 2 * time.Second
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to attach to container: attach error after exit")
}

func TestUnit_Root_Execute_MakeRawFailure_Warning(t *testing.T) {
	t.Parallel()
	mockRuntime := &runtime.MockRuntime{}
	var stderrBuf bytes.Buffer
	err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "--tty", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
		o.runtimeFactory = func(n, s string, l *logging.Logger) (runtime.ContainerRuntime, error) { return mockRuntime, nil }
		o.isTerminal = func(fd int) bool { return true }
		o.makeRaw = func(fd int) (*term.State, error) { return nil, errors.New("makeRaw failed") }
		o.exitFunc = func(code int) {}
		cmd.SetErr(&stderrBuf)
	})
	require.NoError(t, err)
	assert.Contains(t, stderrBuf.String(), "failed to set terminal to raw mode: makeRaw failed")
}

type resizeMockRuntime struct {
	*runtime.MockRuntime
	resizeCalled chan struct{}
}

func (m *resizeMockRuntime) ResizeContainerTTY(ctx context.Context, id string, rows, cols uint) error {
	m.resizeCalled <- struct{}{}
	return m.MockRuntime.ResizeContainerTTY(ctx, id, rows, cols)
}

func TestUnit_Root_Execute_ResizeContainerTTY(t *testing.T) {
	// Not t.Parallel() because it depends on signals

	t.Run("Initial resize and signal resize", func(t *testing.T) {
		resizeCalled := make(chan struct{}, 10)
		mockRuntime := &resizeMockRuntime{
			MockRuntime:  runtime.NewMockRuntime(),
			resizeCalled: resizeCalled,
		}

		resizeChanHolder := make(chan chan os.Signal, 1)
		setupResizeSignalMock := func(c chan os.Signal) {
			resizeChanHolder <- c
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		errCh := make(chan error, 1)
		go func() {
			errCh <- ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "--tty", "sh", "--cderun-log-level=debug"}, func(o *rootOptions, cmd *cobra.Command) {
				o.runtimeFactory = func(n, s string, l *logging.Logger) (runtime.ContainerRuntime, error) { return mockRuntime, nil }
				o.isTerminal = func(fd int) bool { return true }
				o.termGetSize = func(fd int) (int, int, error) { return 80, 24, nil }
				o.setupResizeSignal = setupResizeSignalMock
				o.stopSignalHandling = func(c chan os.Signal) {}
				o.exitFunc = func(code int) {}
				cmd.SetOut(mockFdWriter{io.Discard})
				// Block WaitContainer to keep the command running
				mockRuntime.WaitDelay = 10 * time.Second
			})
		}()

		// Initial resize
		select {
		case <-resizeCalled:
		case <-time.After(5 * time.Second):
			t.Fatal("initial resize not called")
		}

		// Signal resize
		var rc chan os.Signal
		select {
		case rc = <-resizeChanHolder:
		case <-time.After(5 * time.Second):
			t.Fatal("resize signal channel not registered")
		}

		// In root.go, the resize goroutine waits on <-resizeChan.
		// We need to send a signal to the channel.
		// We use os.Interrupt for portability across platforms in this test.

		// Send a signal and wait for the handler to be called.
		timer := time.NewTimer(5 * time.Second)
		defer timer.Stop()

		rc <- os.Interrupt
		select {
		case <-resizeCalled:
			// Success
		case <-timer.C:
			t.Fatal("signal resize not called (timeout)")
		}

		cancel()
		<-errCh
	})
}

type hangTimeoutMockRuntime struct {
	*runtime.MockRuntime
	waitStarted chan struct{}
	isRunning   bool
}

func (m *hangTimeoutMockRuntime) WaitContainer(ctx context.Context, id string) (int, error) {
	if m.WaitFunc != nil {
		return m.WaitFunc(ctx, id)
	}
	close(m.waitStarted)
	select {
	case <-m.SigChan:
		return 137, nil
	case <-ctx.Done():
		return 137, ctx.Err()
	}
}

func (m *hangTimeoutMockRuntime) InspectContainer(ctx context.Context, id string) (bool, int, error) {
	return m.isRunning, 0, nil
}

func (m *hangTimeoutMockRuntime) AttachContainer(ctx context.Context, id string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error {
	close(ready)
	return nil
}

func TestUnit_Root_Execute_HangTimeoutForceTermination(t *testing.T) {
	t.Parallel()
	waitStarted := make(chan struct{})
	mockRuntime := &hangTimeoutMockRuntime{
		MockRuntime: runtime.NewMockRuntime(),
		waitStarted: waitStarted,
		isRunning:   true,
	}

	var logBuf safeBuffer
	err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "sh", "--cderun-log-level=trace", "--cderun-hang-timeout=100ms"}, func(o *rootOptions, cmd *cobra.Command) {
		o.runtimeFactory = func(n, s string, l *logging.Logger) (runtime.ContainerRuntime, error) { return mockRuntime, nil }
		o.isTerminal = func(fd int) bool { return false } // Non-terminal
		o.exitFunc = func(code int) {}
		cmd.SetErr(&logBuf)
	})

	// docs/features/hang-timeout.md: The exit code (137) of a container forced to terminate (SIGKILL) is
	// always propagated to the caller as an ExitCodeError. err == nil would mean a regression of this propagation.
	var exitErr *ExitCodeError
	require.ErrorAs(t, err, &exitErr)
	assert.Equal(t, 137, exitErr.Code)
	assert.Contains(t, logBuf.String(), "IO finished, waiting up to 100ms for container")
	assert.Contains(t, logBuf.String(), "forcing termination")
	assert.Equal(t, "SIGKILL", mockRuntime.Signal)
}

func TestUnit_Root_PreprocessArgs_UnknownP1Flag(t *testing.T) {
	t.Parallel()
	cmd := newRootCmd(&rootOptions{})
	// Hoisted flag that is not in the flag set (should still hoist with = format)
	args := []string{"cderun", "sh", "--cderun-unknown=value"}
	expected := []string{"cderun", "--cderun-unknown=value", "sh"}
	actual, err := preprocessArgs(cmd, args)
	require.NoError(t, err)
	assert.Equal(t, expected, actual)
}

func TestUnit_Root_MarshalingErrors(t *testing.T) {
	t.Parallel()

	t.Run("handleDiagnosis JSON error", func(t *testing.T) {
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--diagnosis", "--diagnosis-format", "json"}, func(o *rootOptions, cmd *cobra.Command) {
			o.exitFunc = func(int) {}
			o.jsonMarshalIndent = func(v any, prefix, indent string) ([]byte, error) {
				return nil, errors.New("json error")
			}
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to marshal JSON: json error")
	})

	t.Run("handleDiagnosis YAML error", func(t *testing.T) {
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--diagnosis", "--diagnosis-format", "yaml"}, func(o *rootOptions, cmd *cobra.Command) {
			o.exitFunc = func(int) {}
			o.yamlMarshal = func(v any) ([]byte, error) {
				return nil, errors.New("yaml error")
			}
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to marshal YAML: yaml error")
	})

	t.Run("handleDryRun JSON error", func(t *testing.T) {
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "sh", "--cderun-dry-run", "--cderun-dry-run-format=json"}, func(o *rootOptions, cmd *cobra.Command) {
			o.exitFunc = func(int) {}
			o.jsonMarshalIndent = func(v any, prefix, indent string) ([]byte, error) {
				return nil, errors.New("json dry-run error")
			}
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to marshal JSON: json dry-run error")
	})

	t.Run("handleDryRun YAML error", func(t *testing.T) {
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "sh", "--cderun-dry-run", "--cderun-dry-run-format=yaml"}, func(o *rootOptions, cmd *cobra.Command) {
			o.exitFunc = func(int) {}
			o.yamlMarshal = func(v any) ([]byte, error) {
				return nil, errors.New("yaml dry-run error")
			}
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to marshal YAML: yaml dry-run error")
	})
}

func TestUnit_Root_DryRun_Safety(t *testing.T) {
	t.Parallel()
	t.Run("Quotes command and environment", func(t *testing.T) {
		out := &bytes.Buffer{}
		opts := &rootOptions{}
		resolved := &config.ResolvedConfig{
			DryRun:       true,
			DryRunFormat: "simple",
		}
		containerConfig := &container.ContainerConfig{
			Image:   "alpine",
			Command: []string{"sh", "-c", "echo 'hello world'"},
			Env:     []string{"SECRET_TOKEN=top-secret", "PLAIN_VAR=value", "VAR_WITH_SPACE=val with space"},
		}
		cmd := &cobra.Command{}
		cmd.SetOut(out)

		err := opts.handleDryRun(cmd, containerConfig, resolved)
		require.NoError(t, err)
		output := out.String()

		// Verify Command quoting
		assert.Contains(t, output, "Command: \"sh\" \"-c\" \"echo 'hello world'\"")

		// Verify Env masking and quoting
		// Note: env is masked as [REDACTED] by default because sensitive-env is unset.
		assert.Contains(t, output, "Env: \"SECRET_TOKEN\"=\"[REDACTED]\", \"PLAIN_VAR\"=\"[REDACTED]\", \"VAR_WITH_SPACE\"=\"[REDACTED]\"")
	})

	t.Run("Handles Entrypoint quoting", func(t *testing.T) {
		out := &bytes.Buffer{}
		opts := &rootOptions{}
		resolved := &config.ResolvedConfig{
			DryRun:       true,
			DryRunFormat: "simple",
		}
		containerConfig := &container.ContainerConfig{
			Image:      "alpine",
			Entrypoint: []string{"/usr/bin/env", "bash"},
		}
		cmd := &cobra.Command{}
		cmd.SetOut(out)

		err := opts.handleDryRun(cmd, containerConfig, resolved)
		require.NoError(t, err)
		output := out.String()

		assert.Contains(t, output, "Entrypoint: \"/usr/bin/env\" \"bash\"")
	})
}

type hangingMockRuntime struct {
	*runtime.MockRuntime
	started chan struct{}
}

func (m *hangingMockRuntime) RemoveContainer(ctx context.Context, containerID string) error {
	close(m.started)
	<-ctx.Done() // Block until context is cancelled (via timeout)
	return ctx.Err()
}

// Reference: docs/features/command-line-options.md (deferred cleanup option '--remove')
// Verifies that deferred container removal cleanup is bounded by a timeout when the runtime hangs.
func TestUnit_Root_Cleanup_Timeout(t *testing.T) {
	t.Parallel()
	t.Run("cleanup terminates within timeout when RemoveContainer hangs", func(t *testing.T) {
		started := make(chan struct{})
		mockRuntime := &hangingMockRuntime{
			MockRuntime: &runtime.MockRuntime{},
			started:     started,
		}

		var stderrBuf bytes.Buffer
		start := time.Now()
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.exitFunc = func(code int) {}
			o.isTerminal = func(fd int) bool { return true }
			o.cleanupTimeout = 50 * time.Millisecond
			cmd.SetErr(&stderrBuf)
		})
		elapsed := time.Since(start)
		require.NoError(t, err)

		// Verify that RemoveContainer was actually invoked
		select {
		case <-started:
		default:
			t.Fatal("expected RemoveContainer to be called")
		}

		// Ensure it returned after the configured cleanupTimeout but within a short upper bound
		assert.GreaterOrEqual(t, elapsed, 50 * time.Millisecond)
		assert.Less(t, elapsed, 500 * time.Millisecond)
	})
}

// Reference: docs/features/command-line-options.md (logging options: '--log-level', '--log-format', '--log-timestamp')
// Verifies that early logger initialization correctly resolves, validates and errors out on invalid formats or levels.
func TestUnit_Root_EarlyLogger_Validation(t *testing.T) {
	t.Parallel()

	t.Run("invalid early log level", func(t *testing.T) {
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--log-level", "invalid-level", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
			o.exitFunc = func(code int) {}
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported log level: \"invalid-level\"")
	})

	t.Run("invalid early cderun log level", func(t *testing.T) {
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "sh", "--cderun-log-level=invalid-level"}, func(o *rootOptions, cmd *cobra.Command) {
			o.exitFunc = func(code int) {}
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported log level: \"invalid-level\"")
	})

	t.Run("invalid early log format", func(t *testing.T) {
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--log-format", "invalid-format", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
			o.exitFunc = func(code int) {}
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported log format: \"invalid-format\"")
	})

	t.Run("invalid early cderun log format", func(t *testing.T) {
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "sh", "--cderun-log-format=invalid-format"}, func(o *rootOptions, cmd *cobra.Command) {
			o.exitFunc = func(code int) {}
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported log format: \"invalid-format\"")
	})

	t.Run("invalid environment log level", func(t *testing.T) {
		mfs := &config.MockFileSystem{
			Env: map[string]string{"CDERUN_LOG_LEVEL": "invalid-env-level"},
		}
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
			o.exitFunc = func(code int) {}
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported log level: \"invalid-env-level\"")
	})

	t.Run("invalid environment log timestamp", func(t *testing.T) {
		mfs := &config.MockFileSystem{
			Env: map[string]string{"CDERUN_LOG_TIMESTAMP": "not-a-bool"},
		}
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
			o.exitFunc = func(code int) {}
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid boolean value for log-timestamp: \"not-a-bool\"")
	})

	t.Run("valid early log timestamp true", func(t *testing.T) {
		mfs := &config.MockFileSystem{
			Env: map[string]string{"CDERUN_LOG_TIMESTAMP": "false"},
		}
		var capturedLogger *logging.Logger
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "--log-timestamp=true", "--dry-run", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
			o.exitFunc = func(code int) {}
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
			capturedLogger = o.logger
		})
		require.NoError(t, err)
		assert.True(t, capturedLogger.GetTimestamp())
	})

	t.Run("valid early log timestamp false", func(t *testing.T) {
		var capturedLogger *logging.Logger
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "--log-timestamp=false", "--dry-run", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
			o.exitFunc = func(code int) {}
			capturedLogger = o.logger
		})
		require.NoError(t, err)
		assert.False(t, capturedLogger.GetTimestamp())
	})

	t.Run("valid early cderun log timestamp false", func(t *testing.T) {
		var capturedLogger *logging.Logger
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "--dry-run", "sh", "--cderun-log-timestamp=false"}, func(o *rootOptions, cmd *cobra.Command) {
			o.exitFunc = func(code int) {}
			capturedLogger = o.logger
		})
		require.NoError(t, err)
		assert.False(t, capturedLogger.GetTimestamp())
	})

	t.Run("valid environment log timestamp false", func(t *testing.T) {
		mfs := &config.MockFileSystem{
			Env: map[string]string{"CDERUN_LOG_TIMESTAMP": "false"},
		}
		var capturedLogger *logging.Logger
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "--dry-run", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
			o.exitFunc = func(code int) {}
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
			capturedLogger = o.logger
		})
		require.NoError(t, err)
		assert.False(t, capturedLogger.GetTimestamp())
	})
}

func TestUnit_Root_ConfigPath_SecurityValidation(t *testing.T) {
	t.Parallel()

	t.Run("CDERUN_CONFIG with control character is rejected", func(t *testing.T) {
		mfs := &config.MockFileSystem{
			Env: map[string]string{"CDERUN_CONFIG": "/etc/cderun\x00.yaml"},
		}
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
			o.exitFunc = func(code int) {}
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "security validation failed for config path")
	})

	t.Run("--config with control character is rejected", func(t *testing.T) {
		mfs := &config.MockFileSystem{}
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--config", "/some/path/with/\n/newline.yaml", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
			o.exitFunc = func(code int) {}
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "security validation failed for config path")
	})

	t.Run("--tool-config with control character is rejected", func(t *testing.T) {
		mfs := &config.MockFileSystem{}
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--tool-config", "/some/path/with/\t/tab.yaml", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
			o.exitFunc = func(code int) {}
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "security validation failed for tool config path")
	})
}
