package command

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/config"
	"cderun/internal/container"
	"cderun/internal/logging"
)

// TestScenarios_PreprocessArgs_WrapperHoisting_Resilience tests argument hoisting
// in Wrapper Mode across space and equals flag syntax and double-dash '--' delimiters.
// Reference: docs/features/argument-parsing.md & docs/features/argument-priority-logic.md
func TestScenarios_PreprocessArgs_WrapperHoisting_Resilience(t *testing.T) {
	t.Parallel()

	cmd := newRootCmd(&rootOptions{})

	t.Run("Space and Equals Flag Hoisting Across Double Dash", func(t *testing.T) {
		args := []string{
			"node",
			"server.js",
			"--",
			"--cderun-runtime", "docker",
			"--cderun-pid=host",
			"--cderun-shm-size", "2g",
			"--app-flag=value",
		}

		processed, err := preprocessArgs(cmd, args)
		require.NoError(t, err)

		// Internal overrides (--cderun-*) should be hoisted to the front after executable "cderun"
		expectedPrefix := []string{
			"cderun",
			"--cderun-runtime", "docker",
			"--cderun-pid=host",
			"--cderun-shm-size", "2g",
			"node",
			"server.js",
			"--",
			"--app-flag=value",
		}

		assert.Equal(t, expectedPrefix, processed)
	})

	t.Run("Flag Hoisting with Missing Value Errors Handled", func(t *testing.T) {
		args := []string{
			"python3",
			"script.py",
			"--cderun-runtime", // Missing argument value
		}

		_, err := preprocessArgs(cmd, args)
		assert.Error(t, err, "missing argument for string flag should produce error")
	})
}

// TestScenarios_SymlinkMode_ExecutionAndMapping verifies Symlink Mode tool invocation,
// argument passthrough, and polyglot container config preparation via buildContainerConfig.
// Reference: docs/features/polyglot-entry.md & docs/features/direct-container-execution.md
func TestScenarios_SymlinkMode_ExecutionAndMapping(t *testing.T) {
	t.Parallel()

	opts := &rootOptions{}
	resolved := &config.ResolvedConfig{
		Image:   "node:20-alpine",
		Workdir: "/workspace",
		Env:     []string{"NODE_ENV=production"},
		Pull:    "missing",
		Network: "bridge",
	}

	toolsCfg := config.ToolsConfig{
		"node": config.ToolConfig{
			Image: "node:20-alpine",
		},
	}

	passthroughArgs := []string{"index.js", "--port", "3000"}
	cc, err := opts.buildContainerConfig(resolved, passthroughArgs, toolsCfg)
	require.NoError(t, err)

	assert.Equal(t, "node:20-alpine", cc.Image)
	assert.Equal(t, []string{"index.js", "--port", "3000"}, cc.Command)
	assert.Equal(t, "/workspace", cc.Workdir)
	assert.Contains(t, cc.Env, "NODE_ENV=production")
}

// TestScenarios_DryRun_JSONFormattingAndMasking verifies dry-run JSON payload formatting via handleDryRun,
// ensuring sensitive environment variables are masked in JSON container configuration output.
// Reference: docs/features/command-line-options.md & docs/testing/strategy.md
func TestScenarios_DryRun_JSONFormattingAndMasking(t *testing.T) {
	t.Parallel()

	opts := &rootOptions{}
	cmd := newRootCmd(opts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)

	cc := &container.ContainerConfig{
		Image: "python:3.11-slim",
		Command: []string{
			"python3",
			"main.py",
		},
		Env: []string{
			"PUBLIC_ENV=development",
			"SECRET_API_KEY=super_secret_token_123",
			"DATABASE_PASSWORD=my_db_password",
		},
	}

	resolved := &config.ResolvedConfig{
		DryRunFormat: "json",
		SensitiveEnv: []string{"*KEY*", "*PASSWORD*"},
	}

	err := opts.handleDryRun(cmd, cc, resolved)
	require.NoError(t, err)

	outputJSON := buf.String()
	assert.Contains(t, outputJSON, "PUBLIC_ENV=development")
	assert.Contains(t, outputJSON, "SECRET_API_KEY=[REDACTED]")
	assert.Contains(t, outputJSON, "DATABASE_PASSWORD=[REDACTED]")
	assert.NotContains(t, outputJSON, "super_secret_token_123")
	assert.NotContains(t, outputJSON, "my_db_password")
}

// TestScenarios_Snapshot_TempDirResolution tests snapshot creation via createSnapshot.
// Reference: docs/features/nested-execution-control-socket.md
func TestScenarios_Snapshot_TempDirResolution(t *testing.T) {
	t.Parallel()

	mfs := &config.MockFileSystem{}
	logger := logging.NewLogger()
	globalCfg := &config.CDERunConfig{
		HostContext: &config.HostContext{
			Level: 0,
		},
	}

	snapDir, ctrlPath, srv, err := createSnapshot(logger, mfs, globalCfg, nil, nil, nil, false)
	require.NoError(t, err)
	defer func() {
		if srv != nil {
			_ = srv.Close()
		}
	}()

	assert.NotEmpty(t, snapDir)
	assert.NotEmpty(t, ctrlPath)
	_, errCderun := mfs.Stat(filepath.Join(snapDir, ".cderun.yaml"))
	require.NoError(t, errCderun, ".cderun.yaml should be created inside snapshot directory")
	_, errTools := mfs.Stat(filepath.Join(snapDir, ".tools.yaml"))
	require.NoError(t, errTools, ".tools.yaml should be created inside snapshot directory")
}
