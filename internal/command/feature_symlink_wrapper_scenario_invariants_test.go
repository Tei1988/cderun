package command

import (
	"context"
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/config"
	"cderun/internal/logging"
	"cderun/internal/runtime"
)

// docs/features/polyglot-entry.md & docs/features/argument-parsing.md
// TestUnit_Command_SymlinkMode_BaseNameAndHoistingInvariants tests
// executable base name detection, relative and Windows paths, repeated P1 override hoisting,
// and preserving raw passthrough flags (including double-dash delimiters).
func TestUnit_Command_SymlinkMode_BaseNameAndHoistingInvariants(t *testing.T) {
	t.Parallel()

	t.Run("resolves basename for deep nested paths and relative paths", func(t *testing.T) {
		t.Parallel()
		opts := defaultOptions()
		cmd := newRootCmd(&opts)

		cases := []struct {
			name     string
			rawArgs  []string
			expected []string
		}{
			{
				name:     "nested unix path",
				rawArgs:  []string{"/opt/cderun/bin/python3", "-c", "print('hello')"},
				expected: []string{"cderun", "python3", "-c", "print('hello')"},
			},
			{
				name:     "relative dot-slash path",
				rawArgs:  []string{"./kubectl", "get", "pods", "-A"},
				expected: []string{"cderun", "kubectl", "get", "pods", "-A"},
			},
			{
				name:     "relative multi-parent traversal path",
				rawArgs:  []string{"../../tools/bin/terraform", "plan", "-out=tfplan"},
				expected: []string{"cderun", "terraform", "plan", "-out=tfplan"},
			},
			{
				name:     "standard cderun binary invocation is not treated as polyglot",
				rawArgs:  []string{"cderun", "python3", "-c", "print('hello')"},
				expected: []string{"cderun", "python3", "-c", "print('hello')"},
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				processed, err := preprocessArgs(cmd, tc.rawArgs)
				require.NoError(t, err)
				assert.Equal(t, tc.expected, processed)
			})
		}
	})

	t.Run("hoists multiple repeated and mixed format P1 overrides while keeping wrapped args unchanged", func(t *testing.T) {
		t.Parallel()
		opts := defaultOptions()
		cmd := newRootCmd(&opts)

		rawArgs := []string{
			"node",
			"--cderun-env=A=1",
			"-e", "console.log(process.env.A)",
			"--cderun-env", "B=2",
			"--cderun-mount=/host/data:/app/data:ro",
			"--cderun-shm-size", "512m",
			"--cderun-read-only",
			"--cderun-workdir=/app",
			"--max-old-space-size=4096",
			"--cderun-sysctl", "net.core.somaxconn=1024",
			"index.js",
		}

		processed, err := preprocessArgs(cmd, rawArgs)
		require.NoError(t, err)

		expected := []string{
			"cderun",
			"--cderun-env=A=1",
			"--cderun-env", "B=2",
			"--cderun-mount=/host/data:/app/data:ro",
			"--cderun-shm-size", "512m",
			"--cderun-read-only",
			"--cderun-workdir=/app",
			"--cderun-sysctl", "net.core.somaxconn=1024",
			"node",
			"-e", "console.log(process.env.A)",
			"--max-old-space-size=4096",
			"index.js",
		}

		assert.Equal(t, expected, processed)
	})

	t.Run("hoists P1 overrides positioned before and after double-dash divider in symlink mode", func(t *testing.T) {
		t.Parallel()
		opts := defaultOptions()
		cmd := newRootCmd(&opts)

		rawArgs := []string{
			"npm",
			"run", "test",
			"--cderun-image=node:20-alpine",
			"--",
			"--watch",
			"--cderun-env", "TEST_ENV=unit",
			"--runInBand",
		}

		processed, err := preprocessArgs(cmd, rawArgs)
		require.NoError(t, err)

		// Both --cderun-image and --cderun-env should be hoisted to the front of "cderun".
		// The double-dash and other flags remain in their exact sequence.
		expected := []string{
			"cderun",
			"--cderun-image=node:20-alpine",
			"--cderun-env", "TEST_ENV=unit",
			"npm",
			"run", "test",
			"--",
			"--watch",
			"--runInBand",
		}

		assert.Equal(t, expected, processed)
	})
}

// docs/features/argument-parsing.md & docs/features/argument-priority-logic.md
// TestUnit_Command_WrapperMode_SubcommandDetectionAndHoistingInvariants tests
// complex P2 flags preceding subcommands (including value-taking shorthands like -p 80:80, -e K=V, -u 1000:1000)
// and error detection for misplaced P1 overrides.
func TestUnit_Command_WrapperMode_SubcommandDetectionAndHoistingInvariants(t *testing.T) {
	t.Parallel()

	t.Run("accurately skips complex P2 flags and shorthands with space-separated values to locate subcommand", func(t *testing.T) {
		t.Parallel()
		opts := defaultOptions()
		cmd := newRootCmd(&opts)

		// Here cderun is invoked with various P2 flags taking arguments:
		// -e APP_ENV=production (shorthand takes arg)
		// -p 8080:80 (shorthand takes arg)
		// -u 1000:1000 (shorthand takes arg)
		// -i (shorthand bool flag)
		// --network host (long flag takes arg)
		// Subcommand is "python3", followed by script and hoisted P1 override
		rawArgs := []string{
			"cderun",
			"-e", "APP_ENV=production",
			"-p", "8080:80",
			"-u", "1000:1000",
			"-i",
			"--network", "host",
			"python3",
			"server.py",
			"--cderun-image", "python:3.12-slim",
		}

		subcmdIdx := findSubcommandIndex(cmd, rawArgs, false)
		assert.Equal(t, 10, subcmdIdx)
		assert.Equal(t, "python3", rawArgs[subcmdIdx])

		processed, err := preprocessArgs(cmd, rawArgs)
		require.NoError(t, err)

		expected := []string{
			"cderun",
			"--cderun-image", "python:3.12-slim",
			"-e", "APP_ENV=production",
			"-p", "8080:80",
			"-u", "1000:1000",
			"-i",
			"--network", "host",
			"python3",
			"server.py",
		}

		assert.Equal(t, expected, processed)
	})

	t.Run("rejects P1 override flag placed before subcommand even when mixed with valid P2 flags", func(t *testing.T) {
		t.Parallel()
		opts := defaultOptions()
		cmd := newRootCmd(&opts)

		invalidCombinations := [][]string{
			{
				"cderun",
				"--network", "bridge",
				"--cderun-read-only",
				"node", "app.js",
			},
			{
				"cderun",
				"-i",
				"--cderun-image=alpine:latest",
				"-e", "FOO=BAR",
				"sh",
			},
			{
				"cderun",
				"-p", "3000:3000",
				"--cderun-env", "ENV=test",
				"npm", "start",
			},
		}

		for _, invalidArgs := range invalidCombinations {
			_, err := preprocessArgs(cmd, invalidArgs)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "cderun internal override flag")
			assert.Contains(t, err.Error(), "must be placed after the subcommand")
		}
	})

	t.Run("supports empty arguments after subcommand", func(t *testing.T) {
		t.Parallel()
		opts := defaultOptions()
		cmd := newRootCmd(&opts)

		rawArgs := []string{"cderun", "sh"}
		processed, err := preprocessArgs(cmd, rawArgs)
		require.NoError(t, err)
		assert.Equal(t, []string{"cderun", "sh"}, processed)
	})
}

// TestScenario_Command_SymlinkMode_EndToEndExecution verifies end-to-end container execution
// through symlink invocation using ExecuteContextWithOptions, validating ContainerConfig assembly,
// environment variables, mounts, and tool command line parameters.
func TestScenario_Command_SymlinkMode_EndToEndExecution(t *testing.T) {
	t.Parallel()

	t.Run("executes symlink command and applies configuration from tools config with hoisted overrides", func(t *testing.T) {
		mock := &runtime.MockRuntime{}
		mfs := &config.MockFileSystem{
			WD: "/workspace",
			Files: map[string][]byte{
				"/workspace/.tools.yaml": []byte(`
go:
  image: golang:1.22-alpine
  workdir: /workspace
`),
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		symlinkPath := filepath.Join("/usr/local/bin", "go")
		args := []string{
			symlinkPath,
			"test",
			"--cderun-env=CUSTOM_FLAG=1",
			"--cderun-read-only",
			"./...",
			"-v",
		}

		execErr := ExecuteContextWithOptions(ctx, args, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mock, nil
			}
			o.exitFunc = func(code int) {}
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
		})

		require.NoError(t, execErr)

		cfg := mock.GetCreatedConfig()
		require.NotNil(t, cfg)
		assert.Equal(t, "golang:1.22-alpine", cfg.Image)
		assert.True(t, cfg.ReadOnly)
		assert.Equal(t, "/workspace", cfg.Workdir)

		// Assert that go receives passthrough args in order: ["test", "./...", "-v"]
		assert.Equal(t, []string{"test", "./...", "-v"}, cfg.Command)

		// Assert environments contain hoisted P1 setting (P1 collection replaces lower layer collection)
		assert.Contains(t, cfg.Env, "CUSTOM_FLAG=1")
	})

	t.Run("fails when tool has no configured image in symlink mode without P1 override", func(t *testing.T) {
		mock := &runtime.MockRuntime{}
		mfs := &config.MockFileSystem{
			WD:    "/workspace",
			Files: map[string][]byte{},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		args := []string{"unconfigured-tool", "run"}

		execErr := ExecuteContextWithOptions(ctx, args, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mock, nil
			}
			o.exitFunc = func(code int) {}
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
		})

		var imgErr *config.ImageNotFoundError
		require.ErrorAs(t, execErr, &imgErr)
		assert.Equal(t, "unconfigured-tool", imgErr.Tool)
		assert.Nil(t, mock.GetCreatedConfig())
	})
}

// TestScenario_Command_WrapperMode_EndToEndExecution verifies end-to-end container execution
// through standard Wrapper Mode with complex hoisted P1 overrides, verifying that ContainerConfig
// correctly captures all options.
func TestScenario_Command_WrapperMode_EndToEndExecution(t *testing.T) {
	t.Parallel()

	t.Run("executes wrapper mode command with hoisted resource limits and sysctl", func(t *testing.T) {
		mock := &runtime.MockRuntime{}
		mfs := &config.MockFileSystem{
			WD: "/project",
			Files: map[string][]byte{
				"/project/.tools.yaml": []byte(`
redis-server:
  image: redis:7-alpine
`),
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		args := []string{
			"cderun",
			"redis-server",
			"--port", "6379",
			"--cderun-memory=512m",
			"--cderun-cpus=2.0",
			"--cderun-shm-size=128m",
			"--cderun-sysctl=net.core.somaxconn=512",
			"--cderun-env", "REDIS_MAXMEMORY=256mb",
		}

		execErr := ExecuteContextWithOptions(ctx, args, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mock, nil
			}
			o.exitFunc = func(code int) {}
			o.fs = mfs
			o.configLoader = config.NewConfigLoaderWithFS(mfs)
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
		})

		require.NoError(t, execErr)

		cfg := mock.GetCreatedConfig()
		require.NotNil(t, cfg)
		assert.Equal(t, "redis:7-alpine", cfg.Image)
		assert.Equal(t, []string{"--port", "6379"}, cfg.Command)
		assert.Equal(t, int64(512*1024*1024), cfg.Memory)
		assert.InDelta(t, 2.0, cfg.CPUs, 1e-9)
		assert.Equal(t, "128m", cfg.ShmSize)
		assert.Equal(t, map[string]string{"net.core.somaxconn": "512"}, cfg.Sysctls)
		assert.Contains(t, cfg.Env, "REDIS_MAXMEMORY=256mb")
	})
}
