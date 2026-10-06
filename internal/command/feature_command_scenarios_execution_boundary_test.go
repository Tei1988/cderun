package command

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/config"
	"cderun/internal/logging"
	"cderun/internal/runtime"
)

// Specification Reference: docs/features/argument-parsing.md
// Section: "Wrapper Mode & Argument Hoisting"
func TestUnit_PreprocessArgs_WrapperMode_ComplexHoistingInvariants(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		input         []string
		expectedPasst []string
	}{
		{
			name: "multi-flag hoisting with space and equal delimiters interleaved after subcommand",
			input: []string{
				"cderun",
				"npm",
				"test",
				"--cderun-image", "node:20-alpine",
				"--cderun-env=NODE_ENV=test",
				"--cderun-workdir", "/app",
				"--cderun-sysctl", "net.core.somaxconn=1024",
				"--",
				"--runInBand",
				"--cderun-env=EXTRA_FLAG=1",
			},
			expectedPasst: []string{
				"cderun",
				"--cderun-image", "node:20-alpine",
				"--cderun-env=NODE_ENV=test",
				"--cderun-workdir", "/app",
				"--cderun-sysctl", "net.core.somaxconn=1024",
				"--cderun-env=EXTRA_FLAG=1",
				"npm",
				"test",
				"--",
				"--runInBand",
			},
		},
		{
			name: "subcommand with leading options and hoisted cderun overrides",
			input: []string{
				"cderun",
				"cargo",
				"build",
				"--release",
				"--cderun-image=rust:1.75",
				"--cderun-cpu-shares", "512",
			},
			expectedPasst: []string{
				"cderun",
				"--cderun-image=rust:1.75",
				"--cderun-cpu-shares", "512",
				"cargo",
				"build",
				"--release",
			},
		},
		{
			name: "polyglot symlink executable invocation with embedded space flags",
			input: []string{
				"/usr/local/bin/python3",
				"-m", "unittest",
				"--cderun-image", "python:3.11",
				"--cderun-read-only",
			},
			expectedPasst: []string{
				"cderun",
				"--cderun-image", "python:3.11",
				"--cderun-read-only",
				"python3",
				"-m", "unittest",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			opts := defaultOptions()
			cmd := newRootCmd(&opts)

			processed, err := preprocessArgs(cmd, tc.input)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedPasst, processed)
		})
	}
}

// Specification Reference: docs/features/argument-parsing.md
// Section: "Symlink Mode & Polyglot Entry Point"
func TestUnit_PreprocessArgs_SymlinkMode_SymlinkPathResolution(t *testing.T) {
	tmpDir := t.TempDir()

	targetBin := filepath.Join(tmpDir, "cderun_real_binary")
	err := os.WriteFile(targetBin, []byte("#!/bin/sh\nexit 0\n"), 0755)
	require.NoError(t, err)

	symlinkPath := filepath.Join(tmpDir, "docker-compose")
	err = os.Symlink(targetBin, symlinkPath)
	require.NoError(t, err)

	opts := defaultOptions()
	cmd := newRootCmd(&opts)

	args := []string{symlinkPath, "up", "-d"}
	processed, err := preprocessArgs(cmd, args)
	require.NoError(t, err)

	expected := []string{"cderun", "docker-compose", "up", "-d"}
	assert.Equal(t, expected, processed)
}

func TestUnit_ContainerConfigBuilder_NullByteAndSliceIsolation(t *testing.T) {
	t.Parallel()

	t.Run("null byte detection in subcommand name", func(t *testing.T) {
		passthroughArgs := []string{"git\x00", "status"}
		cmdSlice, err := assembleContainerCommand(passthroughArgs)
		assert.Error(t, err)
		assert.Nil(t, cmdSlice)
		assert.Contains(t, err.Error(), "null byte")
	})

	t.Run("empty passthrough arguments returns nil slice without error", func(t *testing.T) {
		cmdSlice, err := assembleContainerCommand(nil)
		require.NoError(t, err)
		assert.Nil(t, cmdSlice)

		cmdSliceEmpty, err := assembleContainerCommand([]string{})
		require.NoError(t, err)
		assert.Nil(t, cmdSliceEmpty)
	})

	t.Run("slice isolation in assembled container config command", func(t *testing.T) {
		inputArgs := []string{"go", "test", "./..."}
		cmdSlice, err := assembleContainerCommand(inputArgs)
		require.NoError(t, err)
		require.Equal(t, inputArgs, cmdSlice)

		// Mutate original slice to ensure cmdSlice is an isolated clone
		inputArgs[1] = "build"
		assert.Equal(t, "test", cmdSlice[1])
	})
}

func TestUnit_DryRun_JSONAndYAMLFormattingResilience(t *testing.T) {
	opts := defaultOptions()
	cmd := newRootCmd(&opts)

	resolvedCfg := &config.ResolvedConfig{
		Image:    "redis:7-alpine",
		Engine:   "podman",
		DryRun:   true,
		Env:      []string{"REDIS_PASSWORD=supersecret", "APP_ENV=prod"},
		Workdir:  "/data",
		User:     "999:999",
		Hostname: "redis-cache",
		Remove:   true,
	}

	cc := newBaseContainerConfig(resolvedCfg, []string{"redis-server", "--save", "60", "1"})

	t.Run("dry run formatted output redaction verification", func(t *testing.T) {
		oldStdout := os.Stdout
		r, w, err := os.Pipe()
		require.NoError(t, err)
		os.Stdout = w

		err = opts.handleDryRun(cmd, cc, resolvedCfg)
		w.Close()
		os.Stdout = oldStdout
		require.NoError(t, err)

		var buf bytes.Buffer
		_, err = buf.ReadFrom(r)
		require.NoError(t, err)

		outputStr := buf.String()
		assert.Contains(t, outputStr, "redis:7-alpine")
		assert.Contains(t, outputStr, "REDIS_PASSWORD=[REDACTED]")
		assert.Contains(t, outputStr, "APP_ENV=[REDACTED]")
		assert.NotContains(t, outputStr, "supersecret")
	})
}

func TestUnit_Execute_RuntimeFactoryError_Handling(t *testing.T) {
	t.Parallel()

	opts := defaultOptions()
	cmd := newRootCmd(&opts)
	cmd.SetContext(context.Background())

	resolvedCfg := &config.ResolvedConfig{
		Image:   "busybox:latest",
		Engine:  "podman",
		Runtime: "podman",
	}
	cc := newBaseContainerConfig(resolvedCfg, []string{"sh", "-c", "echo hello"})

	opts.ensureHooks()
	opts.runtimeFactory = func(eng string, sock string, l *logging.Logger) (runtime.ContainerRuntime, error) {
		return nil, errors.New("podman socket not found at /run/user/1000/podman/podman.sock")
	}

	_, err := opts.execute(cmd, resolvedCfg, cc)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "podman socket not found")

	var exitErr *ExitCodeError
	require.True(t, errors.As(err, &exitErr))
	assert.Equal(t, 125, exitErr.Code)
}
