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

func TestUnit_PreprocessArgs_WrapperAndSymlink_ExecutionScenarios(t *testing.T) {
	t.Parallel()

	t.Run("complex interleaved P1 flags with space and equal forms", func(t *testing.T) {
		input := []string{
			"cderun",
			"npm",
			"--cderun-image", "node:20-alpine",
			"run", "build",
			"--cderun-env=NODE_ENV=production",
			"--cderun-workdir", "/app",
		}
		expected := []string{
			"cderun",
			"--cderun-image", "node:20-alpine",
			"--cderun-env=NODE_ENV=production",
			"--cderun-workdir", "/app",
			"npm",
			"run", "build",
		}

		opts := defaultOptions()
		cmd := newRootCmd(&opts)

		processed, err := preprocessArgs(cmd, input)
		require.NoError(t, err)
		assert.Equal(t, expected, processed)
	})

	t.Run("symlink resolution target binary in temporary directory", func(t *testing.T) {
		tmpDir := t.TempDir()

		targetBin := filepath.Join(tmpDir, "real_tool")
		err := os.WriteFile(targetBin, []byte("#!/bin/sh\necho ok\n"), 0755)
		require.NoError(t, err)

		symlinkPath := filepath.Join(tmpDir, "docker-compose")
		err = os.Symlink(targetBin, symlinkPath)
		require.NoError(t, err)

		opts := defaultOptions()
		cmd := newRootCmd(&opts)

		args := []string{symlinkPath, "up", "-d"}
		processed, err := preprocessArgs(cmd, args)
		require.NoError(t, err)
		assert.Equal(t, []string{"cderun", "docker-compose", "up", "-d"}, processed)
	})
}

func TestUnit_ContainerConfig_CommandAssemblyAndCloning(t *testing.T) {
	t.Parallel()

	t.Run("rejects null byte in command argument list", func(t *testing.T) {
		passthrough := []string{"git", "status\x00-s"}
		cmdSlice, err := assembleContainerCommand(passthrough)
		assert.Error(t, err)
		assert.Nil(t, cmdSlice)
		assert.Contains(t, err.Error(), "null byte")
	})

	t.Run("clones passthrough args without side effects on original slice", func(t *testing.T) {
		original := []string{"go", "test", "./..."}
		cmdSlice, err := assembleContainerCommand(original)
		require.NoError(t, err)
		require.NotNil(t, cmdSlice)

		original[0] = "cargo"
		assert.Equal(t, []string{"go", "test", "./..."}, cmdSlice)
	})

	t.Run("newBaseContainerConfig constructs isolated container config", func(t *testing.T) {
		resolvedCfg := &config.ResolvedConfig{
			Image:    "golang:1.22",
			Workdir:  "/go/src/app",
			User:     "1001:1001",
			Hostname: "builder-host",
			Env:      []string{"CGO_ENABLED=0"},
		}

		cc := newBaseContainerConfig(resolvedCfg, []string{"go", "build", "-o", "bin/app"})
		require.NotNil(t, cc)
		assert.Equal(t, "golang:1.22", cc.Image)
		assert.Equal(t, "/go/src/app", cc.Workdir)
		assert.Equal(t, "1001:1001", cc.User)
		assert.Equal(t, "builder-host", cc.Hostname)
		assert.Equal(t, []string{"go", "build", "-o", "bin/app"}, cc.Command)
		assert.Equal(t, []string{"CGO_ENABLED=0"}, cc.Env)
	})
}

func TestUnit_DryRun_ExecutionFormattingAndSecretRedaction(t *testing.T) {
	opts := defaultOptions()
	cmd := newRootCmd(&opts)

	resolvedCfg := &config.ResolvedConfig{
		Image:    "postgres:15-alpine",
		Engine:   "docker",
		DryRun:   true,
		Env:      []string{"POSTGRES_PASSWORD=secretpass", "POSTGRES_USER=admin"},
		Workdir:  "/var/lib/postgresql",
		User:     "999:999",
		Hostname: "db-host",
		Remove:   true,
	}

	cc := newBaseContainerConfig(resolvedCfg, []string{"postgres"})

	t.Run("text dry-run outputs config fields and redacts secrets", func(t *testing.T) {
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
		assert.Contains(t, out, "postgres:15-alpine")
		assert.Contains(t, out, "POSTGRES_PASSWORD=[REDACTED]")
		assert.Contains(t, out, "POSTGRES_USER=[REDACTED]")
		assert.NotContains(t, out, "secretpass")
	})
}

func TestUnit_Execute_RuntimeInitFailure_ExitCode125Mapping(t *testing.T) {
	t.Parallel()

	opts := defaultOptions()
	cmd := newRootCmd(&opts)
	cmd.SetContext(context.Background())

	resolvedCfg := &config.ResolvedConfig{
		Image:   "alpine:latest",
		Engine:  "docker",
		Runtime: "docker",
	}
	cc := newBaseContainerConfig(resolvedCfg, []string{"echo", "test"})

	opts.ensureHooks()
	opts.runtimeFactory = func(eng string, sock string, l *logging.Logger) (runtime.ContainerRuntime, error) {
		return nil, errors.New("cannot communicate with docker daemon")
	}

	_, err := opts.execute(cmd, resolvedCfg, cc)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot communicate with docker daemon")

	var exitErr *ExitCodeError
	require.True(t, errors.As(err, &exitErr))
	assert.Equal(t, 125, exitErr.Code)
}
