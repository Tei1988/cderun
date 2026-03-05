package command

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/config"
)

func executeCommandRaw(args []string) (string, error) {
	return executeCommandRawContext(context.Background(), args)
}

func executeCommandRawContext(ctx context.Context, args []string) (string, error) {
	var buf bytes.Buffer

	execErr := ExecuteContextWithOptions(ctx, args, func(o *rootOptions, cmd *cobra.Command) {
		// Default to terminal mode for tests to avoid auto-detection of pipes
		// unless specifically overridden in a test.
		o.isTerminal = func(fd int) bool { return true }
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
	})

	return buf.String(), execErr
}

func TestUnit_Root_PreprocessArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected []string
	}{
		{
			name:     "cderun with args",
			args:     []string{"cderun", "node", "--version"},
			expected: []string{"cderun", "node", "--version"},
		},
		{
			name:     "cderun with path",
			args:     []string{"/usr/local/bin/cderun", "node", "--version"},
			expected: []string{"/usr/local/bin/cderun", "node", "--version"},
		},
		{
			name:     "symlink node",
			args:     []string{"node", "--version"},
			expected: []string{"cderun", "node", "--version"},
		},
		{
			name:     "symlink python with path",
			args:     []string{"/usr/bin/python", "-c", "print(1)"},
			expected: []string{"cderun", "python", "-c", "print(1)"},
		},
		{
			name:     "empty args",
			args:     []string{},
			expected: []string{},
		},
		{
			name:     "shorthand with argument",
			args:     []string{"cderun", "-p", "80:80", "sh", "--cderun-tty"},
			expected: []string{"cderun", "--cderun-tty", "-p", "80:80", "sh"},
		},
		{
			name:     "multiple shorthands, last takes argument",
			args:     []string{"cderun", "-itp", "80:80", "sh", "--cderun-interactive"},
			expected: []string{"cderun", "--cderun-interactive", "-itp", "80:80", "sh"},
		},
		{
			name:     "P1 flag with equals",
			args:     []string{"cderun", "sh", "--cderun-tty=true"},
			expected: []string{"cderun", "--cderun-tty=true", "sh"},
		},
		{
			name:     "P1 flag with argument following",
			args:     []string{"cderun", "node", "--cderun-image", "alpine", "app.js"},
			expected: []string{"cderun", "--cderun-image", "alpine", "node", "app.js"},
		},
		{
			name:     "complex hoisting with mixed flags",
			args:     []string{"cderun", "-t", "sh", "-c", "ls", "--cderun-image", "alpine", "--cderun-tty=false"},
			expected: []string{"cderun", "--cderun-image", "alpine", "--cderun-tty=false", "-t", "sh", "-c", "ls"},
		},
		{
			name:     "no subcommand but with P1 flags",
			args:     []string{"cderun", "--cderun-image", "alpine"},
			expected: []string{"cderun", "--cderun-image", "alpine"},
		},
		{
			name:     "polyglot with multiple arguments",
			args:     []string{"node", "app.js", "--port", "8080"},
			expected: []string{"cderun", "node", "app.js", "--port", "8080"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newRootCmd(&rootOptions{})
			actual, err := preprocessArgs(cmd, tt.args)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestUnit_Root_PreprocessArgs_Errors(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectedErr string
	}{
		{
			name:        "P1 flag before subcommand",
			args:        []string{"cderun", "--cderun-image", "alpine", "sh"},
			expectedErr: "must be placed after the subcommand",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newRootCmd(&rootOptions{})
			_, err := preprocessArgs(cmd, tt.args)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedErr)
		})
	}
}

func TestUnit_Root_UnknownFlag(t *testing.T) {
	err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--unknown-flag", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
		o.isTerminal = func(fd int) bool { return true }
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown flag: --unknown-flag")
}

func TestUnit_Root_ExecuteEmptyArgs(t *testing.T) {
	// Should not panic
	_, err := executeCommandRaw([]string{})
	require.NoError(t, err)

	_, err = executeCommandRaw(nil)
	require.NoError(t, err)
}

func TestUnit_Root_HelpWithoutSubcommand(t *testing.T) {
	var buf bytes.Buffer
	err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--tty"}, func(o *rootOptions, cmd *cobra.Command) {
		o.isTerminal = func(fd int) bool { return true }
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
	})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "cderun is a CLI wrapper tool")
	assert.Contains(t, output, "Usage:")
}

func TestUnit_Root_UnsupportedRuntime(t *testing.T) {
	err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "--runtime", "invalid", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
		o.isTerminal = func(fd int) bool { return true }
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported runtime \"invalid\"")
}

func TestUnit_Root_DryRunRequiresSubcommand(t *testing.T) {
	err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--dry-run"}, func(o *rootOptions, cmd *cobra.Command) {
		o.isTerminal = func(fd int) bool { return true }
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--dry-run requires a subcommand")
}

func TestUnit_Root_HandleDiagnosis_JSON(t *testing.T) {
	out := &bytes.Buffer{}
	opts := &rootOptions{
		fs: config.RealFileSystem{},
	}
	resolved := &config.ResolvedConfig{
		Runtime:         "docker",
		SocketPath:      "/var/run/docker.sock",
		Diagnosis:       true,
		DiagnosisFormat: "json",
	}
	cmd := &cobra.Command{}
	cmd.SetOut(out)

	err := opts.handleDiagnosis(cmd, resolved, nil, nil, nil)
	require.NoError(t, err)
	assert.Contains(t, out.String(), "\"name\": \"docker\"")
}

func TestUnit_Root_HandleDiagnosis_Simple(t *testing.T) {
	out := &bytes.Buffer{}
	opts := &rootOptions{
		fs: config.RealFileSystem{},
	}
	resolved := &config.ResolvedConfig{
		Runtime:         "podman",
		SocketPath:      "/run/podman/podman.sock",
		Diagnosis:       true,
		DiagnosisFormat: "simple",
	}
	cmd := &cobra.Command{}
	cmd.SetOut(out)

	err := opts.handleDiagnosis(cmd, resolved, nil, nil, nil)
	require.NoError(t, err)
	assert.Contains(t, out.String(), "Runtime: podman")
}

func TestUnit_Root_BuildContainerConfig_ExecFailure(t *testing.T) {
	mfs := &config.MockFileSystem{
		ExecErr: errors.New("exec error"),
	}
	o := defaultOptions()
	o.fs = mfs

	// We need to trigger binary mount logic
	resolved := &config.ResolvedConfig{
		MountCderun: true,
	}
	_, err := o.buildContainerConfig(resolved, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get executable path: exec error")
}
