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

func TestUnit_PreprocessArgs_WrapperMode_SpaceAndEqualHoisting(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		input         []string
		expectedPasst []string
	}{
		{
			name: "interleaved space and equal separated p1 flags",
			input: []string{
				"--cderun-image", "ubuntu:22.04",
				"git",
				"commit",
				"--cderun-env=FOO=BAR",
				"-m", "feat: initial commit",
				"--cderun-workdir", "/workspace",
			},
			expectedPasst: []string{
				"cderun",
				"--cderun-env=FOO=BAR",
				"--cderun-workdir", "/workspace",
				"--cderun-image", "ubuntu:22.04",
				"git",
				"commit",
				"-m", "feat: initial commit",
			},
		},
		{
			name: "p1 flags after double dash delimiter are still hoisted per T81",
			input: []string{
				"git",
				"--",
				"--cderun-image=alpine:latest",
				"checkout",
				"main",
			},
			expectedPasst: []string{
				"cderun",
				"--cderun-image=alpine:latest",
				"git",
				"--",
				"checkout",
				"main",
			},
		},
		{
			name: "flag taking no value in space form followed by subcommand",
			input: []string{
				"--cderun-dry-run",
				"python3",
				"script.py",
			},
			expectedPasst: []string{
				"cderun",
				"--cderun-dry-run",
				"python3",
				"script.py",
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

func TestUnit_PreprocessArgs_SymlinkMode_TargetResolutionBoundary(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a real binary
	binaryPath := filepath.Join(tmpDir, "real_tool")
	err := os.WriteFile(binaryPath, []byte("#!/bin/sh\necho ok\n"), 0755)
	require.NoError(t, err)

	// Create symlink pointing to the real binary
	symlinkPath := filepath.Join(tmpDir, "kubectl")
	err = os.Symlink(binaryPath, symlinkPath)
	require.NoError(t, err)

	// Create broken symlink
	brokenSymlinkPath := filepath.Join(tmpDir, "helm")
	err = os.Symlink(filepath.Join(tmpDir, "nonexistent_target"), brokenSymlinkPath)
	require.NoError(t, err)

	opts := defaultOptions()
	cmd := newRootCmd(&opts)

	t.Run("valid symlink resolves target binary name", func(t *testing.T) {
		args := []string{symlinkPath, "get", "pods"}
		processed, err := preprocessArgs(cmd, args)
		require.NoError(t, err)
		assert.Equal(t, []string{"cderun", "kubectl", "get", "pods"}, processed)
	})

	t.Run("broken symlink falls back safely without panic", func(t *testing.T) {
		args := []string{brokenSymlinkPath, "version"}
		assert.NotPanics(t, func() {
			processed, err := preprocessArgs(cmd, args)
			require.NoError(t, err)
			assert.Equal(t, []string{"cderun", "helm", "version"}, processed)
		})
	})
}

func TestUnit_ContainerConfigBuilder_AssemblyInvariants(t *testing.T) {
	t.Parallel()

	t.Run("rejects null byte in passthrough command arguments", func(t *testing.T) {
		passthroughArgs := []string{"git", "log\x00--oneline"}
		cmdSlice, err := assembleContainerCommand(passthroughArgs)
		assert.Error(t, err)
		assert.Nil(t, cmdSlice)
		assert.Contains(t, err.Error(), "null byte")
	})

	t.Run("clones passthrough args without side effects on original slice", func(t *testing.T) {
		originalArgs := []string{"sh", "-c", "echo hello"}
		cmdSlice, err := assembleContainerCommand(originalArgs)
		require.NoError(t, err)
		require.NotNil(t, cmdSlice)

		// Mutate original slice
		originalArgs[0] = "bash"
		assert.Equal(t, []string{"sh", "-c", "echo hello"}, cmdSlice)
	})

	t.Run("newBaseContainerConfig maps fields correctly from resolved config", func(t *testing.T) {
		resolvedCfg := &config.ResolvedConfig{
			Image:    "alpine:3.18",
			Workdir:  "/workspace",
			User:     "1000:1000",
			Hostname: "cderun-container",
			Env:      []string{"ENV_A=VAL_A"},
		}

		cc := newBaseContainerConfig(resolvedCfg, []string{"echo", "hi"})
		require.NotNil(t, cc)
		assert.Equal(t, "alpine:3.18", cc.Image)
		assert.Equal(t, "/workspace", cc.Workdir)
		assert.Equal(t, "1000:1000", cc.User)
		assert.Equal(t, "cderun-container", cc.Hostname)
		assert.Equal(t, []string{"echo", "hi"}, cc.Command)
		assert.Equal(t, []string{"ENV_A=VAL_A"}, cc.Env)
	})
}

func TestUnit_DryRun_FormattingAndMaskingResilience(t *testing.T) {
	opts := defaultOptions()
	cmd := newRootCmd(&opts)

	resolvedCfg := &config.ResolvedConfig{
		Image:    "python:3.11-slim",
		Engine:   "docker",
		DryRun:   true,
		Env:      []string{"SECRET_TOKEN=super_secret_val", "PUBLIC_VAR=public_val"},
		Workdir:  "/app",
		User:     "1000:1000",
		Hostname: "cderun-host",
		Remove:   true,
	}

	cc := newBaseContainerConfig(resolvedCfg, []string{"python3", "-c", "print('hello')"})

	t.Run("dry run text format outputs expected core fields", func(t *testing.T) {
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

		out := buf.String()
		assert.Contains(t, out, "python:3.11-slim")
		assert.Contains(t, out, "SECRET_TOKEN=[REDACTED]")
		assert.Contains(t, out, "PUBLIC_VAR=[REDACTED]")
		assert.NotContains(t, out, "super_secret_val")
	})
}

func TestUnit_Execute_RuntimeInitializationError_Boundary(t *testing.T) {
	t.Parallel()

	opts := defaultOptions()
	cmd := newRootCmd(&opts)
	cmd.SetContext(context.Background())

	resolvedCfg := &config.ResolvedConfig{
		Image:   "alpine:latest",
		Engine:  "docker",
		Runtime: "docker",
	}
	cc := newBaseContainerConfig(resolvedCfg, []string{"echo", "hi"})

	// Intercept execution with runtime initialization error
	opts.ensureHooks()
	opts.runtimeFactory = func(eng string, sock string, l *logging.Logger) (runtime.ContainerRuntime, error) {
		return nil, errors.New("failed to connect to container daemon socket")
	}

	_, err := opts.execute(cmd, resolvedCfg, cc)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect to container daemon socket")

	var exitErr *ExitCodeError
	require.True(t, errors.As(err, &exitErr))
	assert.Equal(t, 125, exitErr.Code)
}
