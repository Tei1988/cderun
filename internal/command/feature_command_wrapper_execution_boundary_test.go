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

func TestUnit_Command_WrapperMode_PreprocessArgs_SpaceAndEqualHoisting(t *testing.T) {
	t.Parallel()

	t.Run("hoists space separated and equal separated P1 flags after subcommand", func(t *testing.T) {
		input := []string{
			"cderun",
			"kubectl",
			"--cderun-image", "bitnami/kubectl:1.28",
			"--cderun-engine=docker",
			"--cderun-workdir", "/workspace",
			"--cderun-shm-size=1g",
			"get", "pods", "-A",
		}

		opts := defaultOptions()
		cmd := newRootCmd(&opts)

		processed, err := preprocessArgs(cmd, input)
		require.NoError(t, err)

		expected := []string{
			"cderun",
			"--cderun-image", "bitnami/kubectl:1.28",
			"--cderun-engine=docker",
			"--cderun-workdir", "/workspace",
			"--cderun-shm-size=1g",
			"kubectl",
			"get", "pods", "-A",
		}
		assert.Equal(t, expected, processed)
	})

	t.Run("space separated P1 flag missing value at end returns error", func(t *testing.T) {
		input := []string{
			"cderun",
			"python3",
			"main.py",
			"--cderun-image",
		}

		opts := defaultOptions()
		cmd := newRootCmd(&opts)

		_, err := preprocessArgs(cmd, input)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `flag "--cderun-image" requires a value`)
	})

	t.Run("pre-subcommand P1 flag returns placement error", func(t *testing.T) {
		input := []string{
			"cderun",
			"--cderun-image=alpine:latest",
			"sh",
		}

		opts := defaultOptions()
		cmd := newRootCmd(&opts)

		_, err := preprocessArgs(cmd, input)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `cderun internal override flag "--cderun-image=alpine:latest" must be placed after the subcommand`)
	})
}

func TestUnit_Command_SymlinkMode_DirtyPathAndUnicodeResolution(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "bin")
	err := os.MkdirAll(binDir, 0755)
	require.NoError(t, err)

	realBinary := filepath.Join(binDir, "cderun")
	err = os.WriteFile(realBinary, []byte("#!/bin/sh\nexit 0\n"), 0755)
	require.NoError(t, err)

	symlinkPath := filepath.Join(binDir, "kubectl")
	err = os.Symlink(realBinary, symlinkPath)
	require.NoError(t, err)

	t.Run("dirty relative symlink path resolves to base binary name", func(t *testing.T) {
		dirtySymlink := filepath.Join(tmpDir, "bin", "..", "bin", ".", "kubectl")

		opts := defaultOptions()
		cmd := newRootCmd(&opts)

		args := []string{
			dirtySymlink,
			"--cderun-image", "bitnami/kubectl:latest",
			"cluster-info",
		}

		processed, err := preprocessArgs(cmd, args)
		require.NoError(t, err)

		expected := []string{
			"cderun",
			"--cderun-image", "bitnami/kubectl:latest",
			"kubectl",
			"cluster-info",
		}
		assert.Equal(t, expected, processed)
	})

	t.Run("symlink execution with CJK unicode arguments preserves non-ASCII parameters", func(t *testing.T) {
		opts := defaultOptions()
		cmd := newRootCmd(&opts)

		args := []string{
			symlinkPath,
			"--cderun-workdir=/作業ディレクトリ",
			"apply",
			"-f", "マニフェスト.yaml",
		}

		processed, err := preprocessArgs(cmd, args)
		require.NoError(t, err)

		expected := []string{
			"cderun",
			"--cderun-workdir=/作業ディレクトリ",
			"kubectl",
			"apply",
			"-f", "マニフェスト.yaml",
		}
		assert.Equal(t, expected, processed)
	})
}

func TestUnit_Command_ContainerConfigAssembly_InvariantsAndValidation(t *testing.T) {
	t.Parallel()

	opts := defaultOptions()
	opts.fs = config.RealFileSystem{}

	t.Run("rejects null bytes in container arguments", func(t *testing.T) {
		resolvedCfg := &config.ResolvedConfig{
			Image: "alpine:latest",
		}

		_, err := opts.buildContainerConfig(resolvedCfg, []string{"sh", "-c", "echo \x00 bad"}, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "null byte")
	})

	t.Run("mounts slice mutation in resolved config does not affect assembled container config", func(t *testing.T) {
		resolvedCfg := &config.ResolvedConfig{
			Image: "python:3.11",
			Mounts: []container.Mount{
				{Type: "bind", Source: "/host/path", Target: "/container/path"},
			},
		}

		cc, err := opts.buildContainerConfig(resolvedCfg, []string{"python3", "-v"}, nil)
		require.NoError(t, err)

		// Mutate original resolved config mounts slice
		resolvedCfg.Mounts[0].Source = "/mutated/path"

		// Assert container config internal mounts slice remains unchanged
		assert.Equal(t, "/host/path", cc.Mounts[0].Source)
	})

	t.Run("prefetch tool determination and image resolution", func(t *testing.T) {
		toolsCfg := config.ToolsConfig{
			"node":   config.ToolConfig{Image: "node:20-alpine"},
			"python": config.ToolConfig{Image: "python:3.11-alpine"},
		}

		resolvedCfg := &config.ResolvedConfig{
			Prefetch: "node, python",
		}

		tools, err := determineToolsToPrefetch(resolvedCfg, toolsCfg)
		require.NoError(t, err)
		assert.Equal(t, []string{"node", "python"}, tools)

		exprResolver, err := config.NewExpressionResolver(nil)
		require.NoError(t, err)

		images, err := resolvePrefetchImages(exprResolver, tools, toolsCfg)
		require.NoError(t, err)
		assert.Equal(t, []string{"node:20-alpine", "python:3.11-alpine"}, images)
	})
}

func TestUnit_Command_DryRun_SensitiveDataMaskingAcrossFormats(t *testing.T) {
	t.Parallel()

	opts := defaultOptions()
	cmd := newRootCmd(&opts)

	resolvedCfg := &config.ResolvedConfig{
		Image:        "alpine:latest",
		Engine:       "docker",
		DryRun:       true,
		SensitiveEnv: []string{"*TOKEN*", "*SECRET*", "*PASSWORD*"},
		Env: []string{
			"AUTH_TOKEN=super_secret_token_val",
			"DATABASE_PASSWORD=p@ssword123",
			"LOG_LEVEL=info",
			"API_SECRET_KEY=sec_key_5678",
		},
	}

	cc := newBaseContainerConfig(resolvedCfg, []string{"sh", "-c", "env"})

	t.Run("simple output masks all sensitive variables", func(t *testing.T) {
		var buf bytes.Buffer
		cmd.SetOut(&buf)

		resolvedCfg.DryRunFormat = "simple"
		err := opts.handleDryRun(cmd, cc, resolvedCfg)
		require.NoError(t, err)

		out := buf.String()
		assert.Contains(t, out, `"AUTH_TOKEN"="[REDACTED]"`)
		assert.Contains(t, out, `"DATABASE_PASSWORD"="[REDACTED]"`)
		assert.Contains(t, out, `"API_SECRET_KEY"="[REDACTED]"`)
		assert.Contains(t, out, `"LOG_LEVEL"="info"`)
		assert.NotContains(t, out, "super_secret_token_val")
		assert.NotContains(t, out, "p@ssword123")
		assert.NotContains(t, out, "sec_key_5678")
	})

	t.Run("json format masks all sensitive variables and outputs valid json", func(t *testing.T) {
		var buf bytes.Buffer
		cmd.SetOut(&buf)

		resolvedCfg.DryRunFormat = "json"
		err := opts.handleDryRun(cmd, cc, resolvedCfg)
		require.NoError(t, err)

		out := buf.String()
		assert.Contains(t, out, "AUTH_TOKEN=[REDACTED]")
		assert.Contains(t, out, "DATABASE_PASSWORD=[REDACTED]")
		assert.Contains(t, out, "API_SECRET_KEY=[REDACTED]")
		assert.Contains(t, out, "LOG_LEVEL=info")
		assert.NotContains(t, out, "super_secret_token_val")
	})

	t.Run("yaml format masks all sensitive variables and outputs valid yaml", func(t *testing.T) {
		var buf bytes.Buffer
		cmd.SetOut(&buf)

		resolvedCfg.DryRunFormat = "yaml"
		err := opts.handleDryRun(cmd, cc, resolvedCfg)
		require.NoError(t, err)

		out := buf.String()
		assert.Contains(t, out, "AUTH_TOKEN=[REDACTED]")
		assert.Contains(t, out, "DATABASE_PASSWORD=[REDACTED]")
		assert.Contains(t, out, "API_SECRET_KEY=[REDACTED]")
		assert.Contains(t, out, "LOG_LEVEL=info")
		assert.NotContains(t, out, "p@ssword123")
	})
}

func TestUnit_Command_Execute_MockRuntimeLifecycleAndExitCodeMapping(t *testing.T) {
	t.Parallel()

	t.Run("successful container run propagates exit code 0", func(t *testing.T) {
		mockRt := runtime.NewMockRuntime()
		mockRt.CreatedContainerID = "cid-001"
		mockRt.ExitCode = 0

		opts := defaultOptions()
		opts.runtimeFactory = func(eng string, sock string, l *logging.Logger) (runtime.ContainerRuntime, error) {
			return mockRt, nil
		}

		cmd := newRootCmd(&opts)
		cmd.SetContext(context.Background())

		resolvedCfg := &config.ResolvedConfig{Image: "alpine:latest", Runtime: "mock"}
		cc := newBaseContainerConfig(resolvedCfg, []string{"true"})

		code, err := opts.execute(cmd, resolvedCfg, cc)
		require.NoError(t, err)
		assert.Equal(t, 0, code)
	})

	t.Run("container script failure propagates exit code 127", func(t *testing.T) {
		mockRt := runtime.NewMockRuntime()
		mockRt.CreatedContainerID = "cid-002"
		mockRt.ExitCode = 127

		opts := defaultOptions()
		opts.runtimeFactory = func(eng string, sock string, l *logging.Logger) (runtime.ContainerRuntime, error) {
			return mockRt, nil
		}

		cmd := newRootCmd(&opts)
		cmd.SetContext(context.Background())

		resolvedCfg := &config.ResolvedConfig{Image: "alpine:latest", Runtime: "mock"}
		cc := newBaseContainerConfig(resolvedCfg, []string{"invalid_command"})

		code, err := opts.execute(cmd, resolvedCfg, cc)
		require.NoError(t, err)
		assert.Equal(t, 127, code)
	})

	t.Run("runtime container create failure maps to ExitCodeError 125", func(t *testing.T) {
		mockRt := runtime.NewMockRuntime()
		mockRt.CreateErr = errors.New("failed to resolve image reference")

		opts := defaultOptions()
		opts.runtimeFactory = func(eng string, sock string, l *logging.Logger) (runtime.ContainerRuntime, error) {
			return mockRt, nil
		}

		cmd := newRootCmd(&opts)
		cmd.SetContext(context.Background())

		resolvedCfg := &config.ResolvedConfig{Image: "invalid/image:tag", Runtime: "mock"}
		cc := newBaseContainerConfig(resolvedCfg, []string{"echo"})

		_, err := opts.execute(cmd, resolvedCfg, cc)
		require.Error(t, err)

		var exitErr *ExitCodeError
		require.True(t, errors.As(err, &exitErr))
		assert.Equal(t, 125, exitErr.Code)
		assert.Contains(t, exitErr.Error(), "failed to resolve image reference")
	})

	t.Run("container removal error logs warning without failing non-zero container exit code", func(t *testing.T) {
		mockRt := runtime.NewMockRuntime()
		mockRt.CreatedContainerID = "cid-003"
		mockRt.ExitCode = 1
		mockRt.RemoveErr = errors.New("container removal temporarily locked")

		opts := defaultOptions()
		opts.runtimeFactory = func(eng string, sock string, l *logging.Logger) (runtime.ContainerRuntime, error) {
			return mockRt, nil
		}

		cmd := newRootCmd(&opts)
		cmd.SetContext(context.Background())

		resolvedCfg := &config.ResolvedConfig{Image: "alpine:latest", Runtime: "mock"}
		cc := newBaseContainerConfig(resolvedCfg, []string{"false"})

		code, err := opts.execute(cmd, resolvedCfg, cc)
		require.NoError(t, err)
		assert.Equal(t, 1, code)
	})
}
