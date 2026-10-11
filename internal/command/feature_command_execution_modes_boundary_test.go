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

func TestUnit_Command_PreprocessArgs_PreSubcommandOverrideError(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		input   []string
		wantErr string
	}{
		{
			name: "single P1 flag before subcommand in standard wrapper mode",
			input: []string{
				"cderun",
				"--cderun-image=alpine:latest",
				"sh",
			},
			wantErr: "cderun internal override flag \"--cderun-image=alpine:latest\" must be placed after the subcommand",
		},
		{
			name: "space-separated P1 flag before subcommand in standard wrapper mode",
			input: []string{
				"cderun",
				"--cderun-workdir",
				"/app",
				"python3",
				"main.py",
			},
			wantErr: "cderun internal override flag \"--cderun-workdir\" must be placed after the subcommand",
		},
		{
			name: "boolean P1 flag before subcommand in standard wrapper mode",
			input: []string{
				"cderun",
				"--cderun-read-only",
				"git",
				"status",
			},
			wantErr: "cderun internal override flag \"--cderun-read-only\" must be placed after the subcommand",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			opts := defaultOptions()
			cmd := newRootCmd(&opts)

			_, err := preprocessArgs(cmd, tc.input)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestUnit_Command_PreprocessArgs_WrapperMode_MixedFlagsAndSeparators(t *testing.T) {
	t.Parallel()

	input := []string{
		"cderun",
		"node",
		"--cderun-image", "node:20-alpine",
		"--cderun-read-only",
		"--cderun-env=NODE_ENV=production",
		"--cderun-sysctl", "net.core.somaxconn=1024",
		"--cderun-user", "1000:1000",
		"server.js",
		"--port=8080",
	}

	opts := defaultOptions()
	cmd := newRootCmd(&opts)

	processed, err := preprocessArgs(cmd, input)
	require.NoError(t, err)

	expected := []string{
		"cderun",
		"--cderun-image", "node:20-alpine",
		"--cderun-read-only",
		"--cderun-env=NODE_ENV=production",
		"--cderun-sysctl", "net.core.somaxconn=1024",
		"--cderun-user", "1000:1000",
		"node",
		"server.js",
		"--port=8080",
	}

	assert.Equal(t, expected, processed)
}

func TestUnit_Command_SymlinkMode_ComplexPathTargetResolution(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	nestedBin := filepath.Join(tmpDir, "usr", "local", "bin")
	err := os.MkdirAll(nestedBin, 0755)
	require.NoError(t, err)

	targetExe := filepath.Join(nestedBin, "cderun")
	err = os.WriteFile(targetExe, []byte("#!/bin/sh\nexit 0\n"), 0755)
	require.NoError(t, err)

	symlinkPath := filepath.Join(tmpDir, "go")
	err = os.Symlink(targetExe, symlinkPath)
	require.NoError(t, err)

	opts := defaultOptions()
	cmd := newRootCmd(&opts)

	args := []string{
		symlinkPath,
		"--cderun-image=golang:1.22-alpine",
		"test",
		"./...",
		"-v",
	}

	processed, err := preprocessArgs(cmd, args)
	require.NoError(t, err)

	expected := []string{
		"cderun",
		"--cderun-image=golang:1.22-alpine",
		"go",
		"test",
		"./...",
		"-v",
	}

	assert.Equal(t, expected, processed)
}

func TestUnit_Command_BuildContainerConfig_MountToolsValidation(t *testing.T) {
	t.Parallel()

	opts := defaultOptions()
	opts.fs = config.RealFileSystem{}

	toolsCfg := config.ToolsConfig{
		"python3": config.ToolConfig{Image: "python:3.11"},
		"node":    config.ToolConfig{Image: "node:20"},
	}

	t.Run("mount specified tool succeeds", func(t *testing.T) {
		resolved := &config.ResolvedConfig{
			Image:      "alpine:latest",
			MountTools: []string{"python3"},
		}

		cc, err := opts.buildContainerConfig(resolved, []string{"python3", "app.py"}, toolsCfg)
		require.NoError(t, err)
		require.NotNil(t, cc)

		var toolMounts []container.Mount
		for _, m := range cc.Mounts {
			if m.Target == "/usr/local/bin/python3" {
				toolMounts = append(toolMounts, m)
			}
		}
		require.Len(t, toolMounts, 1)
		assert.Equal(t, "bind", toolMounts[0].Type)
		assert.True(t, toolMounts[0].ReadOnly)
	})

	t.Run("mount missing tool returns descriptive error with available tools", func(t *testing.T) {
		resolved := &config.ResolvedConfig{
			Image:      "alpine:latest",
			MountTools: []string{"ruby"},
		}

		_, err := opts.buildContainerConfig(resolved, []string{"ruby", "app.rb"}, toolsCfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `tool "ruby" not found in .tools.yaml`)
		assert.Contains(t, err.Error(), "available tools: node, python3")
	})

	t.Run("mount invalid tool name returns validation error", func(t *testing.T) {
		resolved := &config.ResolvedConfig{
			Image:      "alpine:latest",
			MountTools: []string{"../../invalid/tool"},
		}

		_, err := opts.buildContainerConfig(resolved, []string{"tool"}, toolsCfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid tool name in mount-tools")
	})
}

func TestUnit_Command_DryRun_SecretMaskingInYAMLAndJSON(t *testing.T) {
	opts := defaultOptions()
	cmd := newRootCmd(&opts)

	resolvedCfg := &config.ResolvedConfig{
		Image:        "python:3.11-slim",
		Engine:       "docker",
		DryRun:       true,
		SensitiveEnv: []string{"*SECRET*", "*TOKEN*", "*KEY*"},
		Env: []string{
			"AWS_SECRET_ACCESS_KEY=secret_aws_key_12345",
			"GITHUB_TOKEN=ghp_secret_token_val",
			"API_KEY=key_98765",
			"APP_MODE=production",
		},
	}

	cc := newBaseContainerConfig(resolvedCfg, []string{"python3", "-c", "import sys"})

	t.Run("yaml output masks sensitive variables", func(t *testing.T) {
		var buf bytes.Buffer
		cmd.SetOut(&buf)

		resolvedCfg.DryRunFormat = "yaml"
		err := opts.handleDryRun(cmd, cc, resolvedCfg)
		require.NoError(t, err)

		out := buf.String()
		assert.Contains(t, out, "AWS_SECRET_ACCESS_KEY=[REDACTED]")
		assert.Contains(t, out, "GITHUB_TOKEN=[REDACTED]")
		assert.Contains(t, out, "API_KEY=[REDACTED]")
		assert.Contains(t, out, "APP_MODE=production")
		assert.NotContains(t, out, "secret_aws_key_12345")
		assert.NotContains(t, out, "ghp_secret_token_val")
	})

	t.Run("json output masks sensitive variables", func(t *testing.T) {
		var buf bytes.Buffer
		cmd.SetOut(&buf)

		resolvedCfg.DryRunFormat = "json"
		err := opts.handleDryRun(cmd, cc, resolvedCfg)
		require.NoError(t, err)

		out := buf.String()
		assert.Contains(t, out, "AWS_SECRET_ACCESS_KEY=[REDACTED]")
		assert.Contains(t, out, "GITHUB_TOKEN=[REDACTED]")
		assert.Contains(t, out, "API_KEY=[REDACTED]")
		assert.Contains(t, out, "APP_MODE=production")
		assert.NotContains(t, out, "secret_aws_key_12345")
		assert.NotContains(t, out, "ghp_secret_token_val")
	})

	t.Run("original container config env slice remains unmutated", func(t *testing.T) {
		assert.Equal(t, "AWS_SECRET_ACCESS_KEY=secret_aws_key_12345", cc.Env[0])
		assert.Equal(t, "GITHUB_TOKEN=ghp_secret_token_val", cc.Env[1])
	})
}

func TestUnit_Command_Execute_MockRuntimeLifecycleAndExitCodes(t *testing.T) {
	t.Parallel()

	t.Run("successful execution returns exit code 0", func(t *testing.T) {
		mockRt := runtime.NewMockRuntime()
		mockRt.CreatedContainerID = "container-123"
		mockRt.ExitCode = 0

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
		cc := newBaseContainerConfig(resolvedCfg, []string{"echo", "ok"})

		code, err := opts.execute(cmd, resolvedCfg, cc)
		require.NoError(t, err)
		assert.Equal(t, 0, code)
	})

	t.Run("container non-zero exit code propagated cleanly", func(t *testing.T) {
		mockRt := runtime.NewMockRuntime()
		mockRt.CreatedContainerID = "container-456"
		mockRt.ExitCode = 137 // OOM killed

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
		cc := newBaseContainerConfig(resolvedCfg, []string{"stress", "--vm", "1"})

		code, err := opts.execute(cmd, resolvedCfg, cc)
		require.NoError(t, err)
		assert.Equal(t, 137, code)
	})

	t.Run("container start error wrapped into ExitCodeError code 125", func(t *testing.T) {
		mockRt := runtime.NewMockRuntime()
		mockRt.CreatedContainerID = "container-789"
		mockRt.StartErr = errors.New("executable file not found in $PATH")

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
		cc := newBaseContainerConfig(resolvedCfg, []string{"nonexistent_cmd"})

		_, err := opts.execute(cmd, resolvedCfg, cc)
		require.Error(t, err)

		var exitErr *ExitCodeError
		require.True(t, errors.As(err, &exitErr))
		assert.Equal(t, 125, exitErr.Code)
		assert.Contains(t, exitErr.Error(), "executable file not found")
	})
}
