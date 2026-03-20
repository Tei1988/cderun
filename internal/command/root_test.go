package command

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/config"
	"cderun/internal/runtime"
)

type failAttachRuntime struct {
	runtime.MockRuntime
}

func (m *failAttachRuntime) AttachContainer(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error {
	return errors.New("attach failed")
}

func setupTestMFS() *config.MockFileSystem {
	return &config.MockFileSystem{
		WD: "/project",
		Files: map[string][]byte{
			"/project/.tools.yaml": []byte("node:\n  image: node:20-alpine\n  network: host\nsh:\n  image: alpine\nls:\n  image: alpine"),
		},
	}
}

func TestUnit_Root_Execution_SubcommandMapping(t *testing.T) {
	t.Parallel()

	t.Run("successfully maps tool to image and executes", func(t *testing.T) {
		mfs := setupTestMFS()
		mockRuntime := runtime.NewMockRuntime()
		mockRuntime.CreatedContainerID = "test-container-id"
		mockRuntime.ExitCode = 42

		var capturedExitCode int

		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--tty", "node", "--version"}, func(o *rootOptions, cmd *cobra.Command) {
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
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

		assert.Eventually(t, func() bool {
			return mockRuntime.GetStartedContainerID() != ""
		}, 3*time.Second, 50*time.Millisecond)

		assert.Equal(t, 42, capturedExitCode)
	})

	t.Run("shows_help_when_no_subcommand_is_provided", func(t *testing.T) {
		var stdout bytes.Buffer
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--tty"}, func(o *rootOptions, cmd *cobra.Command) {
			cmd.SetOut(&stdout)
			o.exitFunc = func(code int) {}
		})
		require.NoError(t, err)
		output := stdout.String()

		assert.Contains(t, output, "Usage:")
	})
}

func TestUnit_Root_Flags_OverridePrecedence(t *testing.T) {
	t.Parallel()

	t.Run("P1_override_takes_priority_over_P2_CLI", func(t *testing.T) {
		mfs := setupTestMFS()
		mockRuntime := runtime.NewMockRuntime()
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "--tty=true", "--cderun-tty=false", "sh"}, withMockRuntime(mockRuntime, withMockFS(mfs), func(o *rootOptions, cmd *cobra.Command) {
			o.isTerminal = func(fd int) bool { return true }
		}))
		require.NoError(t, err)
		assert.False(t, mockRuntime.GetCreatedConfig().TTY)
	})

	t.Run("-t shorthand for --tty", func(t *testing.T) {
		mfs := setupTestMFS()
		mockRuntime := runtime.NewMockRuntime()
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "-t", "--image", "alpine", "sh"}, withMockRuntime(mockRuntime, withMockFS(mfs), func(o *rootOptions, cmd *cobra.Command) {
			o.isTerminal = func(fd int) bool { return true }
		}))
		require.NoError(t, err)
		assert.True(t, mockRuntime.GetCreatedConfig().TTY)
	})
}

func TestUnit_Root_Execution_ErrorHandling(t *testing.T) {
	t.Parallel()

	t.Run("returns error for unsupported runtime", func(t *testing.T) {
		mfs := setupTestMFS()
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "--runtime", "invalid", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
			o.exitFunc = func(code int) {}
			o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
				return nil, errors.New("unsupported runtime \"invalid\"")
			}
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported runtime \"invalid\"")
	})

	t.Run("returns error if AttachContainer fails", func(t *testing.T) {
		mfs := setupTestMFS()
		mockRuntime := &failAttachRuntime{MockRuntime: *runtime.NewMockRuntime()}

		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
			o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.isTerminal = func(fd int) bool { return true }
			o.exitFunc = func(code int) {}
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to attach to container: attach failed")
	})

	t.Run("fails when no image mapping found for tool", func(t *testing.T) {
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "unknown-tool", "--version"}, func(o *rootOptions, cmd *cobra.Command) {
			o.exitFunc = func(code int) {}
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no image mapping found for tool: unknown-tool")
	})
}

func TestUnit_Root_Diagnosis_Behaviors(t *testing.T) {
	t.Parallel()

	t.Run("diagnosis mode works without subcommand", func(t *testing.T) {
		var stdout bytes.Buffer
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--diagnosis"}, func(o *rootOptions, cmd *cobra.Command) {
			cmd.SetOut(&stdout)
			o.exitFunc = func(code int) {}
		})
		require.NoError(t, err)
		output := stdout.String()
		assert.Contains(t, output, "resolved:")
	})

	t.Run("diagnosis mode works with subcommand and takes precedence", func(t *testing.T) {
		mfs := setupTestMFS()
		var stdout bytes.Buffer
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--diagnosis", "node", "--version"}, func(o *rootOptions, cmd *cobra.Command) {
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
			cmd.SetOut(&stdout)
			o.exitFunc = func(code int) {}
		})
		require.NoError(t, err)
		output := stdout.String()
		assert.Contains(t, output, "resolved:")
	})

	t.Run("JSON_format", func(t *testing.T) {
		var out bytes.Buffer
		mfs := &config.MockFileSystem{
			Files: map[string][]byte{
				"/var/run/docker.sock": {},
			},
		}
		opts := &rootOptions{
			fs: mfs,
			exitFunc: func(code int) {},
		}
		resolved := &config.ResolvedConfig{
			Runtime:         "docker",
			SocketPath:      "/var/run/docker.sock",
			Diagnosis:       true,
			DiagnosisFormat: "json",
		}
		cmd := &cobra.Command{}
		cmd.SetOut(&out)

		err := opts.handleDiagnosis(cmd, resolved, nil, nil, nil)
		require.NoError(t, err)
		assert.Contains(t, out.String(), "\"resolved\":")
	})
}

func TestUnit_Root_DryRun_Behaviors(t *testing.T) {
	t.Parallel()

	t.Run("dry-run requires a subcommand", func(t *testing.T) {
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--dry-run"}, func(o *rootOptions, cmd *cobra.Command) {
			o.exitFunc = func(code int) {}
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--dry-run requires a subcommand")
	})

	t.Run("dry-run outputs configuration and skips execution", func(t *testing.T) {
		mfs := setupTestMFS()
		// Dry-run format defaults to YAML in resolver, so it returns 'yaml' string.
		// handleDryRun in root.go check for 'json' or 'yaml' and falls back to 'simple'.

		// Simple Format Test
		var out1 bytes.Buffer
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--dry-run", "--dry-run-format", "simple", "--image", "alpine", "sh", "echo", "hello"}, func(o *rootOptions, cmd *cobra.Command) {
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
			cmd.SetOut(&out1)
			o.exitFunc = func(code int) {}
		})
		require.NoError(t, err)
		assert.Contains(t, out1.String(), "Image: alpine")
		assert.Contains(t, out1.String(), "Command: echo hello")

		// JSON Format Test
		var out2 bytes.Buffer
		err = ExecuteContextWithOptions(context.Background(), []string{"cderun", "--dry-run", "--dry-run-format", "json", "--image", "alpine", "sh", "echo", "hello"}, func(o *rootOptions, cmd *cobra.Command) {
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
			cmd.SetOut(&out2)
			o.exitFunc = func(code int) {}
		})
		require.NoError(t, err)
		assert.Contains(t, out2.String(), "\"image\": \"alpine\"")

		// YAML Format Test
		var out3 bytes.Buffer
		err = ExecuteContextWithOptions(context.Background(), []string{"cderun", "--dry-run", "--dry-run-format", "yaml", "--image", "alpine", "sh", "echo", "hello"}, func(o *rootOptions, cmd *cobra.Command) {
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
			cmd.SetOut(&out3)
			o.exitFunc = func(code int) {}
		})
		require.NoError(t, err)
		assert.Contains(t, out3.String(), "image: alpine")
		assert.Contains(t, out3.String(), "command:")
	})
}

func TestUnit_Root_Flags_MountingAndDevices(t *testing.T) {
	t.Parallel()
	t.Run("workdir_mount_and_device_flags", func(t *testing.T) {
		mfs := setupTestMFS()
		mockRuntime := runtime.NewMockRuntime()
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "--workdir", "/my/workdir", "--mount", "type=bind,source=/h,target=/c,readonly", "--device", "/dev/fuse:/dev/fuse:rm", "sh"}, withMockRuntime(mockRuntime, withMockFS(mfs), func(o *rootOptions, cmd *cobra.Command) {
			o.isTerminal = func(fd int) bool { return true }
		}))
		require.NoError(t, err)

		cfg := mockRuntime.GetCreatedConfig()
		require.NotNil(t, cfg)
		assert.Equal(t, "/my/workdir", cfg.Workdir)
		require.Len(t, cfg.Mounts, 1)
		assert.Equal(t, "/h", cfg.Mounts[0].Source)

		require.Len(t, cfg.Devices, 1)
		assert.Equal(t, "/dev/fuse", cfg.Devices[0].PathOnHost)
	})

	t.Run("mounting flags auto-enable socket mounting", func(t *testing.T) {
		mockRuntime := runtime.NewMockRuntime()
		mockRuntime.CreatedContainerID = "test-id"
		mfs := setupTestMFS()
		mfs.Files["/var/run/docker.sock"] = []byte{}
		mfs.Env = map[string]string{
			"CDERUN_SOCKET_PATH": "/var/run/docker.sock",
		}

		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "--mount-socket", "sh"}, withMockRuntime(mockRuntime, withMockFS(mfs), func(o *rootOptions, cmd *cobra.Command) {
			o.isTerminal = func(fd int) bool { return true }
		}))
		require.NoError(t, err)
		cfg := mockRuntime.GetCreatedConfig()
		require.NotNil(t, cfg)

		socketFound := false
		for _, v := range cfg.Mounts {
			if v.Target == "/var/run/docker.sock" {
				socketFound = true
			}
		}
		assert.True(t, socketFound, "Socket should be automatically mounted")
	})
}

func TestUnit_Root_Execution_SubcommandArgs(t *testing.T) {
	t.Parallel()
	t.Run("subcommand_is_excluded_from_CMD", func(t *testing.T) {
		mfs := setupTestMFS()
		mockRuntime := runtime.NewMockRuntime()
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "ls", "-l", "/tmp"}, withMockRuntime(mockRuntime, withMockFS(mfs), func(o *rootOptions, cmd *cobra.Command) {
			o.isTerminal = func(fd int) bool { return true }
		}))
		require.NoError(t, err)

		cfg := mockRuntime.GetCreatedConfig()
		require.NotNil(t, cfg)
		assert.Equal(t, []string{"-l", "/tmp"}, cfg.Command)
	})
}

func TestUnit_Root_Cleanup_RemoveContainerWarning(t *testing.T) {
	t.Parallel()
	t.Run("prints_warning_if_RemoveContainer_fails", func(t *testing.T) {
		mfs := setupTestMFS()
		mockRuntime := runtime.NewMockRuntime()
		mockRuntime.CreatedContainerID = "test-id"
		mockRuntime.RemoveErr = errors.New("failed to remove")

		var stderrBuf bytes.Buffer
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "sh"}, withMockRuntime(mockRuntime, withMockFS(mfs), func(o *rootOptions, cmd *cobra.Command) {
			o.isTerminal = func(fd int) bool { return true }
			cmd.SetErr(&stderrBuf)
		}))
		require.NoError(t, err)

		assert.Contains(t, stderrBuf.String(), "failed to remove container: failed to remove")
	})
}

func TestUnit_Root_Env_StrictEnvFlags(t *testing.T) {
	t.Parallel()
	t.Run("--strict-env flag", func(t *testing.T) {
		mockRuntime := runtime.NewMockRuntime()
		mfs := setupTestMFS()
		mfs.Env = map[string]string{} // Clear env
		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "--strict-env", "--env", "NONEXISTENT", "sh"}, withMockRuntime(mockRuntime, withMockFS(mfs)))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "required environment variable not found: NONEXISTENT")
	})
}
