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
	"cderun/internal/container"
	"cderun/internal/logging"
	"cderun/internal/runtime"
)

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

func TestUnit_PreprocessArgs_ShorthandWithArg(t *testing.T) {
	t.Parallel()
	cmd := &cobra.Command{}
	cmd.Flags().StringP("env", "e", "", "env")

	// -e value -> value should be skipped in subcmd search
	args := []string{"cderun", "-e", "K=V", "ls"}
	processed, err := preprocessArgs(cmd, args)
	require.NoError(t, err)
	assert.Equal(t, []string{"cderun", "-e", "K=V", "ls"}, processed)
}

func TestUnit_PreprocessArgs_HoistingAndPolyglot(t *testing.T) {
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

func TestUnit_Execute_EmptyArgs(t *testing.T) {
	t.Parallel()
	// Should not panic
	_, err := executeCommandRaw([]string{})
	require.NoError(t, err)

	_, err = executeCommandRaw(nil)
	require.NoError(t, err)
}

func TestUnit_Execution_CommandResolution(t *testing.T) {
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
		// Step 10.2: subcommand 'sh' is excluded from command
		output, err := executeCommand("--dry-run", "--image", "alpine", "sh", "echo", "hello")
		require.NoError(t, err)
		assert.Contains(t, output, "image: alpine")
		assert.Contains(t, output, "command:")
		assert.Contains(t, output, "- echo")
		assert.Contains(t, output, "- hello")
		assert.NotContains(t, output, "- sh")

		// Dry-run with JSON
		output, err = executeCommand("--dry-run", "--dry-run-format", "json", "--image", "alpine", "sh", "echo", "hello")
		require.NoError(t, err)
		assert.Contains(t, output, "\"image\": \"alpine\"")
		assert.Contains(t, output, "\"command\": [")

		// Dry-run with simple
		output, err = executeCommand("--dry-run", "-f", "simple", "--image", "alpine", "sh", "echo", "hello")
		require.NoError(t, err)
		assert.Contains(t, output, "Image: alpine")
		assert.Contains(t, output, "Command: echo hello")
		assert.NotContains(t, output, "Command: sh")
		assert.Contains(t, output, "TTY: false")
		assert.Contains(t, output, "Interactive: false")
		assert.Contains(t, output, "Network: bridge")
		assert.Contains(t, output, "Remove: true")

		// Dry-run with mount
		output, err = executeCommand("--dry-run", "-f", "simple", "--image", "alpine", "--mount", "type=bind,source=/h,target=/c", "sh")
		require.NoError(t, err)
		assert.Contains(t, output, "Mounts: type=bind,source=/h,target=/c,readonly=false")

		// Dry-run with device
		output, err = executeCommand("--dry-run", "-f", "simple", "--image", "alpine", "--device", "/dev/video0:/dev/video1:ro", "sh")
		require.NoError(t, err)
		assert.Contains(t, output, "Devices: /dev/video0:/dev/video1:ro")
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

	t.Run("comma in env value is preserved (StringArrayVar)", func(t *testing.T) {
		mockRuntime := &runtime.MockRuntime{}
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "--env", "MYVAR=a,b", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.isTerminal = func(fd int) bool { return true }
		})
		require.NoError(t, err)

		require.NotNil(t, mockRuntime.CreatedConfig)
		assert.Contains(t, mockRuntime.CreatedConfig.Env, "MYVAR=a,b")
	})
}

func TestUnit_Flags_Phase3Features(t *testing.T) {
	// Not safe for t.Parallel() because some subtests use t.Setenv
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
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "--mount-cderun", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.isTerminal = func(fd int) bool { return true }
		})
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
		mockRuntime = &runtime.MockRuntime{}
		err = ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "--mount-cderun", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.isTerminal = func(fd int) bool { return true }
		})
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
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "--mount-cderun", "--mount-socket", "--socket-path", "/socket", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.isTerminal = func(fd int) bool { return true }
		})
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
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "--mount-socket", "--socket-path", "/host/socket", "--mount-socket-path", "/container/socket", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.isTerminal = func(fd int) bool { return true }
		})
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

func TestUnit_Execution_StrictBehavior(t *testing.T) {
	t.Parallel()
	t.Run("fails when no image mapping found for tool (Step 10.1)", func(t *testing.T) {
		// No .tools.yaml created, and no --image flag
		_, err := executeCommand("unknown-tool", "--version")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no image mapping found for tool: unknown-tool")
	})

	t.Run("subcommand is excluded from CMD (Step 10.2)", func(t *testing.T) {
		mockRuntime := &runtime.MockRuntime{}
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "ls", "-l", "/tmp"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.isTerminal = func(fd int) bool { return true }
		})
		require.NoError(t, err)

		require.NotNil(t, mockRuntime.CreatedConfig)
		// 'ls' should be excluded, only '-l' and '/tmp' remain
		assert.Equal(t, []string{"-l", "/tmp"}, mockRuntime.CreatedConfig.Command)
	})
}

func TestUnit_Diagnosis_OutputFormats(t *testing.T) {
	t.Parallel()
	t.Run("JSON format", func(t *testing.T) {
		out := &bytes.Buffer{}
		opts := &rootOptions{
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

		err := opts.handleDiagnosis(cmd, resolved, nil, nil, nil)
		require.NoError(t, err)
		assert.Contains(t, out.String(), "\"name\": \"docker\"")
	})

	t.Run("Simple format", func(t *testing.T) {
		t.Parallel()
		out := &bytes.Buffer{}
		opts := &rootOptions{
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

		err := opts.handleDiagnosis(cmd, resolved, nil, nil, nil)
		require.NoError(t, err)
		assert.Contains(t, out.String(), "Runtime: podman")
	})

	t.Run("YAML format (default)", func(t *testing.T) {
		t.Parallel()
		out := &bytes.Buffer{}
		opts := &rootOptions{
			fs: config.RealFileSystem{},
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
		assert.Contains(t, out.String(), "runtime:")
		assert.Contains(t, out.String(), "name: docker")
	})

	t.Run("Socket missing", func(t *testing.T) {
		t.Parallel()
		out := &bytes.Buffer{}
		opts := &rootOptions{
			fs: &config.MockFileSystem{StatErr: errors.New("not found")},
		}
		resolved := &config.ResolvedConfig{
			Runtime:         "docker",
			SocketPath:      "/missing.sock",
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

func TestUnit_DryRun_OutputFormats(t *testing.T) {
	t.Parallel()
	cfg := &container.ContainerConfig{Image: "alpine", Command: []string{"ls"}}

	t.Run("JSON format", func(t *testing.T) {
		t.Parallel()
		out := &bytes.Buffer{}
		opts := &rootOptions{}
		resolved := &config.ResolvedConfig{DryRunFormat: "json"}
		cmd := &cobra.Command{}
		cmd.SetOut(out)
		err := opts.handleDryRun(cmd, cfg, resolved)
		require.NoError(t, err)
		assert.Contains(t, out.String(), "\"image\": \"alpine\"")
	})

	t.Run("Simple format", func(t *testing.T) {
		t.Parallel()
		out := &bytes.Buffer{}
		opts := &rootOptions{}
		resolved := &config.ResolvedConfig{DryRunFormat: "simple"}
		cmd := &cobra.Command{}
		cmd.SetOut(out)
		err := opts.handleDryRun(cmd, cfg, resolved)
		require.NoError(t, err)
		assert.Contains(t, out.String(), "Image: alpine")
		assert.Contains(t, out.String(), "Command: ls")
	})

	t.Run("YAML format (default)", func(t *testing.T) {
		t.Parallel()
		out := &bytes.Buffer{}
		opts := &rootOptions{}
		resolved := &config.ResolvedConfig{DryRunFormat: "yaml"}
		cmd := &cobra.Command{}
		cmd.SetOut(out)
		err := opts.handleDryRun(cmd, cfg, resolved)
		require.NoError(t, err)
		assert.Contains(t, out.String(), "image: alpine")
	})
}

func TestUnit_ContainerConfig_Nested(t *testing.T) {
	t.Parallel()
	mfs := &config.MockFileSystem{
		ExecPath: "/container/project/bin/cderun",
	}
	opts := defaultOptions()
	opts.fs = mfs
	opts.logger = logging.NewLogger()
	resolved := &config.ResolvedConfig{
		MountCderun: true,
		HostContext: &config.HostContext{
			Level: 1,
			Mounts: []config.MountMapping{
				{Source: "/host/project", Target: "/container/project", Level: 1},
			},
		},
	}

	cc, err := opts.buildContainerConfig(resolved, nil, nil)
	require.NoError(t, err)

	// Should resolve /container/project/bin/cderun to /host/project/bin/cderun for the next level
	found := false
	for _, m := range cc.Mounts {
		if m.Target == "/usr/local/bin/cderun" {
			assert.Equal(t, "/host/project/bin/cderun", m.Source)
			found = true
		}
	}
	assert.True(t, found)
}

func TestUnit_ContainerConfig_BuildFailures(t *testing.T) {
	t.Parallel()
	t.Run("fails when os.Executable fails", func(t *testing.T) {
		mfs := &config.MockFileSystem{
			ExecErr: errors.New("exec error"),
		}
		o := defaultOptions()
		o.fs = mfs

		// We need to trigger binary mount logic
		resolved := &config.ResolvedConfig{
			MountCderun: true,
		}
		_, err := o.buildContainerConfig(resolved, nil, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get executable path: exec error")
	})

	t.Run("fails when tool not found", func(t *testing.T) {
		t.Parallel()
		opts := &rootOptions{
			fs: &config.MockFileSystem{ExecPath: "/bin/cderun"},
		}
		resolved := &config.ResolvedConfig{
			MountTools: []string{"missing-tool"},
		}
		toolsCfg := config.ToolsConfig{"other": {Image: "alpine"}}
		_, err := opts.buildContainerConfig(resolved, nil, toolsCfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "tool \"missing-tool\" not found in .tools.yaml")
	})
}

func TestUnit_Execute_RuntimeErrors(t *testing.T) {
	t.Parallel()
	resolved := &config.ResolvedConfig{Runtime: "docker", SocketPath: "/var/run/docker.sock"}
	cfg := &container.ContainerConfig{Image: "alpine"}

	t.Run("runtime initialization failure", func(t *testing.T) {
		t.Parallel()
		opts := defaultOptions()
		opts.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
			return nil, errors.New("factory error")
		}
		cmd := &cobra.Command{}
		cmd.SetContext(context.Background())
		_, err := opts.execute(cmd, resolved, cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to initialize runtime")
	})

	t.Run("image pull failure", func(t *testing.T) {
		t.Parallel()
		mock := &runtime.MockRuntime{PullErr: errors.New("pull error")}
		opts := defaultOptions()
		opts.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
			return mock, nil
		}
		cmd := &cobra.Command{}
		cmd.SetContext(context.Background())
		_, err := opts.execute(cmd, resolved, cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to pull image")
	})

	t.Run("container creation failure", func(t *testing.T) {
		t.Parallel()
		mock := &runtime.MockRuntime{CreateErr: errors.New("create error")}
		opts := defaultOptions()
		opts.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
			return mock, nil
		}
		cmd := &cobra.Command{}
		cmd.SetContext(context.Background())
		_, err := opts.execute(cmd, resolved, cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create container")
	})

	t.Run("container start failure", func(t *testing.T) {
		t.Parallel()
		mock := &runtime.MockRuntime{StartErr: errors.New("start error")}
		opts := defaultOptions()
		opts.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
			return mock, nil
		}
		cmd := &cobra.Command{}
		cmd.SetContext(context.Background())
		_, err := opts.execute(cmd, resolved, cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to start container")
	})
}

func TestUnit_Cleanup_RemoveContainerWarning(t *testing.T) {
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

	t.Run("does not print warning if RemoveContainer succeeds", func(t *testing.T) {
		mockRuntime := &runtime.MockRuntime{
			RemoveErr: nil,
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

		assert.NotContains(t, stderrBuf.String(), "[WARN] failed to remove container (defer)")
	})
}

func TestUnit_Env_StrictEnvFlags(t *testing.T) {
	t.Parallel()
	t.Run("--strict-env flag", func(t *testing.T) {
		mockRuntime := &runtime.MockRuntime{}
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "--strict-env", "--env", "NONEXISTENT", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.exitFunc = func(code int) {}
		})
		// Should fail because NONEXISTENT env is not on host and strict-env is true
		require.Error(t, err)
		assert.Contains(t, err.Error(), "required environment variable not found: NONEXISTENT")
	})

	t.Run("--cderun-strict-env override", func(t *testing.T) {
		mockRuntime := &runtime.MockRuntime{}
		// Set global strictEnv to true via mock? No, just use flags.
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--strict-env", "--image", "alpine", "node", "--cderun-strict-env=false", "--env", "NONEXISTENT", "app.js"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.exitFunc = func(code int) {}
			o.fs = config.RealFileSystem{} // Ensure we use real FS to check env
		})
		// Should NOT fail because --cderun-strict-env=false overrides --strict-env
		require.NoError(t, err)
	})
}
