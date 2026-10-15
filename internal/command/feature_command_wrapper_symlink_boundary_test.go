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

func TestUnit_Command_WrapperSymlink_PreprocessArgs_P1OverridesAndPreSubcommandValidation(t *testing.T) {
	t.Parallel()

	t.Run("wrapper mode space and equal separated overrides hoisting", func(t *testing.T) {
		input := []string{
			"cderun",
			"python3",
			"--cderun-image", "python:3.12-alpine",
			"-m", "http.server",
			"--cderun-workdir=/app",
			"--cderun-read-only",
			"--cderun-env", "ENV_TYPE=staging",
			"8080",
		}

		opts := defaultOptions()
		cmd := newRootCmd(&opts)

		processed, err := preprocessArgs(cmd, input)
		require.NoError(t, err)

		expected := []string{
			"cderun",
			"--cderun-image", "python:3.12-alpine",
			"--cderun-workdir=/app",
			"--cderun-read-only",
			"--cderun-env", "ENV_TYPE=staging",
			"python3",
			"-m", "http.server",
			"8080",
		}

		assert.Equal(t, expected, processed)
	})

	t.Run("pre-subcommand override flag triggers explicit positioning error", func(t *testing.T) {
		badInputs := [][]string{
			{"cderun", "--cderun-image=alpine:3.19", "sh"},
			{"cderun", "--cderun-env", "KEY=VAL", "bash"},
			{"cderun", "--cderun-workdir", "/var/app", "node", "app.js"},
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
}

func TestUnit_Command_WrapperSymlink_TargetBinaryDeductionAndPolyglotIndexing(t *testing.T) {
	t.Parallel()

	t.Run("symlink mode resolves target binary name from symlink path", func(t *testing.T) {
		tmpDir := t.TempDir()
		binDir := filepath.Join(tmpDir, "bin")
		err := os.MkdirAll(binDir, 0755)
		require.NoError(t, err)

		cderunBinary := filepath.Join(binDir, "cderun")
		err = os.WriteFile(cderunBinary, []byte("#!/bin/sh\nexit 0\n"), 0755)
		require.NoError(t, err)

		symlinkPath := filepath.Join(tmpDir, "kubectl")
		err = os.Symlink(cderunBinary, symlinkPath)
		require.NoError(t, err)

		opts := defaultOptions()
		cmd := newRootCmd(&opts)

		args := []string{
			symlinkPath,
			"--cderun-image=bitnami/kubectl:latest",
			"get",
			"pods",
			"-n",
			"default",
		}

		processed, err := preprocessArgs(cmd, args)
		require.NoError(t, err)

		expected := []string{
			"cderun",
			"--cderun-image=bitnami/kubectl:latest",
			"kubectl",
			"get",
			"pods",
			"-n",
			"default",
		}

		assert.Equal(t, expected, processed)
	})

	t.Run("polyglot and wrapper mode subcommand index search behavior", func(t *testing.T) {
		opts := defaultOptions()
		cmd := newRootCmd(&opts)

		args := []string{
			"cderun",
			"--cderun-image=python:3.11",
			"script.py",
			"--arg1",
			"val1",
		}

		// Wrapper mode finds subcommand at index 2 ("script.py")
		wrapperIdx := findSubcommandIndex(cmd, args, false)
		assert.Equal(t, 2, wrapperIdx)
		assert.Equal(t, "script.py", args[wrapperIdx])

		// Polyglot mode treats index 0 as subcommand
		polyglotIdx := findSubcommandIndex(cmd, args, true)
		assert.Equal(t, 0, polyglotIdx)
		assert.Equal(t, "cderun", args[polyglotIdx])
	})
}

func TestUnit_Command_WrapperSymlink_ContainerConfigAssemblyInvariants(t *testing.T) {
	t.Parallel()

	t.Run("assembleContainerCommand short-circuits on empty passthrough args", func(t *testing.T) {
		cmdArgs, err := assembleContainerCommand(nil)
		require.NoError(t, err)
		assert.Nil(t, cmdArgs)

		cmdArgs, err = assembleContainerCommand([]string{})
		require.NoError(t, err)
		assert.Nil(t, cmdArgs)
	})

	t.Run("assembleContainerCommand rejects null bytes in arguments", func(t *testing.T) {
		_, err := assembleContainerCommand([]string{"echo", "hello\x00world"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "null byte")
	})

	t.Run("newBaseContainerConfig performs mount slice copy isolation", func(t *testing.T) {
		resolved := &config.ResolvedConfig{
			Image:      "alpine:3.19",
			User:       "1000:1000",
			Workdir:    "/workspace",
			Env:        []string{"A=1", "B=2"},
			Entrypoint: []string{"/bin/sh", "-c"},
			Mounts: []container.Mount{
				{Type: "bind", Source: "/host", Target: "/container", ReadOnly: true},
			},
		}

		cmdArgs := []string{"echo", "ready"}
		cc := newBaseContainerConfig(resolved, cmdArgs)

		require.NotNil(t, cc)
		assert.Equal(t, "alpine:3.19", cc.Image)
		assert.Equal(t, "1000:1000", cc.User)
		assert.Equal(t, "/workspace", cc.Workdir)
		assert.Equal(t, []string{"/bin/sh", "-c"}, cc.Entrypoint)
		assert.Equal(t, []string{"echo", "ready"}, cc.Command)

		// Verify Mounts slice cloning isolation
		resolved.Mounts[0].Source = "/mutated/host"
		assert.Equal(t, "/host", cc.Mounts[0].Source)
	})
}

func TestUnit_Command_WrapperSymlink_DryRunMaskingFormats(t *testing.T) {
	opts := defaultOptions()
	cmd := newRootCmd(&opts)

	resolvedCfg := &config.ResolvedConfig{
		Image:        "golang:1.22-alpine",
		Engine:       "docker",
		DryRun:       true,
		SensitiveEnv: []string{"*SECRET*", "*PASS*", "*KEY*"},
		Env: []string{
			"DB_PASSWORD=super_secret_db_pass",
			"API_KEY=key_abcdef123456",
			"APP_ENV=staging",
		},
	}

	cc := newBaseContainerConfig(resolvedCfg, []string{"go", "run", "main.go"})

	t.Run("simple dry-run format masks sensitive variables", func(t *testing.T) {
		var buf bytes.Buffer
		cmd.SetOut(&buf)

		resolvedCfg.DryRunFormat = "simple"
		err := opts.handleDryRun(cmd, cc, resolvedCfg)
		require.NoError(t, err)

		out := buf.String()
		assert.Contains(t, out, `"DB_PASSWORD"="[REDACTED]"`)
		assert.Contains(t, out, `"API_KEY"="[REDACTED]"`)
		assert.Contains(t, out, `"APP_ENV"="staging"`)
		assert.NotContains(t, out, "super_secret_db_pass")
		assert.NotContains(t, out, "key_abcdef123456")
	})

	t.Run("yaml dry-run format masks sensitive variables", func(t *testing.T) {
		var buf bytes.Buffer
		cmd.SetOut(&buf)

		resolvedCfg.DryRunFormat = "yaml"
		err := opts.handleDryRun(cmd, cc, resolvedCfg)
		require.NoError(t, err)

		out := buf.String()
		assert.Contains(t, out, "DB_PASSWORD=[REDACTED]")
		assert.Contains(t, out, "API_KEY=[REDACTED]")
		assert.Contains(t, out, "APP_ENV=staging")
		assert.NotContains(t, out, "super_secret_db_pass")
		assert.NotContains(t, out, "key_abcdef123456")
	})

	t.Run("json dry-run format masks sensitive variables", func(t *testing.T) {
		var buf bytes.Buffer
		cmd.SetOut(&buf)

		resolvedCfg.DryRunFormat = "json"
		err := opts.handleDryRun(cmd, cc, resolvedCfg)
		require.NoError(t, err)

		out := buf.String()
		assert.Contains(t, out, "DB_PASSWORD=[REDACTED]")
		assert.Contains(t, out, "API_KEY=[REDACTED]")
		assert.Contains(t, out, "APP_ENV=staging")
		assert.NotContains(t, out, "super_secret_db_pass")
		assert.NotContains(t, out, "key_abcdef123456")
	})
}

func TestUnit_Command_WrapperSymlink_ExecuteRuntimeErrorsAndExitCodeMapping(t *testing.T) {
	t.Parallel()

	t.Run("runtime create container error maps to exit code 125", func(t *testing.T) {
		mockRt := runtime.NewMockRuntime()
		mockRt.CreateErr = errors.New("failed to allocate network endpoint")

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
		assert.Contains(t, exitErr.Error(), "failed to allocate network endpoint")
	})

	t.Run("runtime pull image failure maps to exit code 125", func(t *testing.T) {
		mockRt := runtime.NewMockRuntime()
		mockRt.PullErr = errors.New("manifest for image:notfound not found")

		opts := defaultOptions()
		opts.runtimeFactory = func(eng string, sock string, l *logging.Logger) (runtime.ContainerRuntime, error) {
			return mockRt, nil
		}

		cmd := newRootCmd(&opts)
		cmd.SetContext(context.Background())

		resolvedCfg := &config.ResolvedConfig{
			Image:   "image:notfound",
			Runtime: "mock",
		}
		cc := newBaseContainerConfig(resolvedCfg, []string{"sh"})

		_, err := opts.execute(cmd, resolvedCfg, cc)
		require.Error(t, err)

		var exitErr *ExitCodeError
		require.True(t, errors.As(err, &exitErr))
		assert.Equal(t, 125, exitErr.Code)
		assert.Contains(t, exitErr.Error(), "manifest for image:notfound not found")
	})
}
