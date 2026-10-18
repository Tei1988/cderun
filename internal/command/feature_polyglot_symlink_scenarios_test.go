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
	"cderun/internal/container"
	"cderun/internal/logging"
	"cderun/internal/runtime"
)

// docs/features/argument-parsing.md: Wrapper Mode & Argument Hoisting
// TestUnit_Command_WrapperMode_ComplexHoistingAndPreSubcommandValidation verifies that
// internal P1 override flags (--cderun-*) placed after subcommands are hoisted,
// while pre-subcommand P1 overrides trigger explicit positioning errors.
func TestUnit_Command_WrapperMode_ComplexHoistingAndPreSubcommandValidation(t *testing.T) {
	t.Parallel()

	t.Run("hoists space and equal separated P1 internal overrides after subcommand", func(t *testing.T) {
		input := []string{
			"cderun",
			"python3",
			"--cderun-image", "python:3.12-alpine",
			"-m", "http.server",
			"--cderun-workdir=/workspace",
			"--cderun-read-only",
			"--cderun-env", "ENV_TYPE=production",
			"--cderun-sysctl", "net.ipv4.ip_forward=1",
			"8080",
		}

		opts := defaultOptions()
		cmd := newRootCmd(&opts)

		processed, err := preprocessArgs(cmd, input)
		require.NoError(t, err)

		expected := []string{
			"cderun",
			"--cderun-image", "python:3.12-alpine",
			"--cderun-workdir=/workspace",
			"--cderun-read-only",
			"--cderun-env", "ENV_TYPE=production",
			"--cderun-sysctl", "net.ipv4.ip_forward=1",
			"python3",
			"-m", "http.server",
			"8080",
		}

		assert.Equal(t, expected, processed)
	})

	t.Run("returns error when P1 override flag precedes subcommand in wrapper mode", func(t *testing.T) {
		badInputs := [][]string{
			{"cderun", "--cderun-image=alpine:3.19", "sh"},
			{"cderun", "--cderun-env", "DEBUG=1", "node", "server.js"},
			{"cderun", "--cderun-workdir", "/app", "go", "run", "main.go"},
		}

		for _, input := range badInputs {
			opts := defaultOptions()
			cmd := newRootCmd(&opts)

			_, err := preprocessArgs(cmd, input)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "cderun internal override flag")
			assert.Contains(t, err.Error(), "must be placed after the subcommand")
		}
	})

	t.Run("returns error when space-separated P1 flag lacks value or precedes another flag", func(t *testing.T) {
		// Space-separated P1 flag at end of args without value
		inputMissingValue := []string{"cderun", "node", "app.js", "--cderun-image"}
		opts1 := defaultOptions()
		cmd1 := newRootCmd(&opts1)
		_, err1 := preprocessArgs(cmd1, inputMissingValue)
		require.Error(t, err1)
		assert.Contains(t, err1.Error(), "cderun internal override flag")
		assert.Contains(t, err1.Error(), "requires a value")

		// Space-separated P1 flag followed directly by another P1 flag
		inputAdjacentFlags := []string{"cderun", "node", "app.js", "--cderun-image", "--cderun-read-only"}
		opts2 := defaultOptions()
		cmd2 := newRootCmd(&opts2)
		_, err2 := preprocessArgs(cmd2, inputAdjacentFlags)
		require.Error(t, err2)
		assert.Contains(t, err2.Error(), "cderun internal override flag")
		assert.Contains(t, err2.Error(), "requires a value")
	})
}

// docs/features/polyglot-entry.md: Symlink Mode (Polyglot Entry Point)
// TestUnit_Command_SymlinkMode_PathResolutionAndPassthroughInvariants tests
// executable path cleaning, dirty path resolution, Unicode/CJK arguments, and non-prefixed option passthrough.
func TestUnit_Command_SymlinkMode_PathResolutionAndPassthroughInvariants(t *testing.T) {
	t.Parallel()

	t.Run("resolves target executable name from relative and dirty symlink paths", func(t *testing.T) {
		tmpDir := t.TempDir()
		binDir := filepath.Join(tmpDir, "bin")
		require.NoError(t, os.MkdirAll(binDir, 0755))

		cderunBinary := filepath.Join(binDir, "cderun")
		require.NoError(t, os.WriteFile(cderunBinary, []byte("#!/bin/sh\nexit 0\n"), 0755))

		symlinkPath := filepath.Join(tmpDir, "dirty", "..", "kubectl")
		require.NoError(t, os.MkdirAll(filepath.Dir(symlinkPath), 0755))
		require.NoError(t, os.Symlink(cderunBinary, symlinkPath))

		opts := defaultOptions()
		cmd := newRootCmd(&opts)

		args := []string{
			symlinkPath,
			"--cderun-image=bitnami/kubectl:1.29",
			"--namespace", "kube-system",
			"get", "pods",
			"--cderun-env", "KUBECONFIG=/root/.kube/config",
		}

		processed, err := preprocessArgs(cmd, args)
		require.NoError(t, err)

		expected := []string{
			"cderun",
			"--cderun-image=bitnami/kubectl:1.29",
			"--cderun-env", "KUBECONFIG=/root/.kube/config",
			"kubectl",
			"--namespace", "kube-system",
			"get", "pods",
		}

		assert.Equal(t, expected, processed)
	})

	t.Run("preserves non-prefixed options and CJK Unicode arguments in symlink mode", func(t *testing.T) {
		opts := defaultOptions()
		cmd := newRootCmd(&opts)

		// Symlink invocation of "git" tool with CJK commit message and non-prefixed flags
		args := []string{
			"git",
			"--cderun-image=alpine/git:latest",
			"commit",
			"-m", "機能追加: Symlinkモードの引数ハンドリング拡張",
			"--author", "開発者 <dev@example.com>",
		}

		processed, err := preprocessArgs(cmd, args)
		require.NoError(t, err)

		expected := []string{
			"cderun",
			"--cderun-image=alpine/git:latest",
			"git",
			"commit",
			"-m", "機能追加: Symlinkモードの引数ハンドリング拡張",
			"--author", "開発者 <dev@example.com>",
		}

		assert.Equal(t, expected, processed)
	})
}

// TestUnit_Command_ContainerConfigAssembly_NullByteAndSliceIsolation verifies
// ContainerConfig assembly invariants and null-byte rejection.
func TestUnit_Command_ContainerConfigAssembly_NullByteAndSliceIsolation(t *testing.T) {
	t.Parallel()

	t.Run("assembleContainerCommand short-circuits on empty arguments", func(t *testing.T) {
		cmdArgs, err := assembleContainerCommand(nil)
		require.NoError(t, err)
		assert.Nil(t, cmdArgs)

		cmdArgs, err = assembleContainerCommand([]string{})
		require.NoError(t, err)
		assert.Nil(t, cmdArgs)
	})

	t.Run("assembleContainerCommand rejects null bytes in passthrough arguments", func(t *testing.T) {
		_, err := assembleContainerCommand([]string{"sh", "-c", "echo \x00 corrupted"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "null byte")
	})

	t.Run("newBaseContainerConfig performs deep copy isolation for mounts and slices", func(t *testing.T) {
		resolved := &config.ResolvedConfig{
			Image:      "node:20-alpine",
			User:       "1000:1000",
			Workdir:    "/app",
			Env:        []string{"NODE_ENV=production", "PORT=3000"},
			Entrypoint: []string{"node"},
			Mounts: []container.Mount{
				{Type: "bind", Source: "/host/src", Target: "/app/src", ReadOnly: true},
			},
		}

		passthroughArgs := []string{"server.js", "--port", "3000"}
		cc := newBaseContainerConfig(resolved, passthroughArgs)

		require.NotNil(t, cc)
		assert.Equal(t, "node:20-alpine", cc.Image)
		assert.Equal(t, []string{"node"}, cc.Entrypoint)
		assert.Equal(t, passthroughArgs, cc.Command)

		// Mutate resolved.Mounts to verify slice copy isolation
		resolved.Mounts[0].Source = "/mutated/host/src"
		assert.Equal(t, "/host/src", cc.Mounts[0].Source)
	})
}

// TestUnit_Command_DryRunMasking_MultiFormatOutputs verifies sensitive data masking in simple, JSON, and YAML dry-run outputs.
func TestUnit_Command_DryRunMasking_MultiFormatOutputs(t *testing.T) {
	opts := defaultOptions()
	cmd := newRootCmd(&opts)

	resolvedCfg := &config.ResolvedConfig{
		Image:        "postgres:16-alpine",
		Engine:       "docker",
		DryRun:       true,
		SensitiveEnv: []string{"*PASS*", "*TOKEN*", "*SECRET*"},
		Env: []string{
			"POSTGRES_PASSWORD=super_secret_db_pass",
			"AUTH_TOKEN=secret_token_12345",
			"APP_ENV=production",
		},
	}

	cc := newBaseContainerConfig(resolvedCfg, []string{"postgres"})

	t.Run("simple dry-run format masks sensitive variables", func(t *testing.T) {
		var buf bytes.Buffer
		cmd.SetOut(&buf)

		resolvedCfg.DryRunFormat = "simple"
		err := opts.handleDryRun(cmd, cc, resolvedCfg)
		require.NoError(t, err)

		out := buf.String()
		assert.Contains(t, out, `"POSTGRES_PASSWORD"="[REDACTED]"`)
		assert.Contains(t, out, `"AUTH_TOKEN"="[REDACTED]"`)
		assert.Contains(t, out, `"APP_ENV"="production"`)
		assert.NotContains(t, out, "super_secret_db_pass")
		assert.NotContains(t, out, "secret_token_12345")
	})

	t.Run("yaml dry-run format masks sensitive variables", func(t *testing.T) {
		var buf bytes.Buffer
		cmd.SetOut(&buf)

		resolvedCfg.DryRunFormat = "yaml"
		err := opts.handleDryRun(cmd, cc, resolvedCfg)
		require.NoError(t, err)

		out := buf.String()
		assert.Contains(t, out, "POSTGRES_PASSWORD=[REDACTED]")
		assert.Contains(t, out, "AUTH_TOKEN=[REDACTED]")
		assert.Contains(t, out, "APP_ENV=production")
		assert.NotContains(t, out, "super_secret_db_pass")
		assert.NotContains(t, out, "secret_token_12345")
	})

	t.Run("json dry-run format masks sensitive variables", func(t *testing.T) {
		var buf bytes.Buffer
		cmd.SetOut(&buf)

		resolvedCfg.DryRunFormat = "json"
		err := opts.handleDryRun(cmd, cc, resolvedCfg)
		require.NoError(t, err)

		out := buf.String()
		assert.Contains(t, out, "POSTGRES_PASSWORD=[REDACTED]")
		assert.Contains(t, out, "AUTH_TOKEN=[REDACTED]")
		assert.Contains(t, out, "APP_ENV=production")
		assert.NotContains(t, out, "super_secret_db_pass")
		assert.NotContains(t, out, "secret_token_12345")
	})
}

// TestUnit_Command_ExecuteRuntimeErrors_ExitCode125Mapping verifies that runtime container creation and pull image failures map to exit code 125.
func TestUnit_Command_ExecuteRuntimeErrors_ExitCode125Mapping(t *testing.T) {
	t.Parallel()

	t.Run("container creation failure maps to exit code 125", func(t *testing.T) {
		mockRt := runtime.NewMockRuntime()
		mockRt.CreateErr = errors.New("cannot mount volume: permission denied")

		opts := defaultOptions()
		opts.runtimeFactory = func(eng string, sock string, l *logging.Logger) (runtime.ContainerRuntime, error) {
			return mockRt, nil
		}

		cmd := newRootCmd(&opts)
		cmd.SetContext(context.Background())

		resolvedCfg := &config.ResolvedConfig{
			Image:   "alpine:latest",
			Runtime: "mock",
		}
		cc := newBaseContainerConfig(resolvedCfg, []string{"sh"})

		_, err := opts.execute(cmd, resolvedCfg, cc)
		require.Error(t, err)

		var exitErr *ExitCodeError
		require.True(t, errors.As(err, &exitErr))
		assert.Equal(t, 125, exitErr.Code)
		assert.Contains(t, exitErr.Error(), "cannot mount volume: permission denied")
	})

	t.Run("image pull failure maps to exit code 125", func(t *testing.T) {
		mockRt := runtime.NewMockRuntime()
		mockRt.PullErr = errors.New("image pull failed: repository does not exist")

		opts := defaultOptions()
		opts.runtimeFactory = func(eng string, sock string, l *logging.Logger) (runtime.ContainerRuntime, error) {
			return mockRt, nil
		}

		cmd := newRootCmd(&opts)
		cmd.SetContext(context.Background())

		resolvedCfg := &config.ResolvedConfig{
			Image:   "nonexistent/image:latest",
			Runtime: "mock",
		}
		cc := newBaseContainerConfig(resolvedCfg, []string{"sh"})

		_, err := opts.execute(cmd, resolvedCfg, cc)
		require.Error(t, err)

		var exitErr *ExitCodeError
		require.True(t, errors.As(err, &exitErr))
		assert.Equal(t, 125, exitErr.Code)
		assert.Contains(t, exitErr.Error(), "image pull failed: repository does not exist")
	})
}
