package command

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Command_WrapperMode_SpaceSeparatedMultipleOverrides(t *testing.T) {
	t.Parallel()

	args := []string{
		"cderun",
		"python3",
		"script.py",
		"--cderun-image", "python:3.11-slim",
		"--cderun-workdir", "/workspace",
		"--cderun-user", "1001:1001",
		"--cderun-read-only",
		"--verbose",
		"arg1",
	}

	o := &rootOptions{}
	cmd := newRootCmd(o)
	processed, err := preprocessArgs(cmd, args)
	require.NoError(t, err)

	expected := []string{
		"cderun",
		"--cderun-image", "python:3.11-slim",
		"--cderun-workdir", "/workspace",
		"--cderun-user", "1001:1001",
		"--cderun-read-only",
		"python3",
		"script.py",
		"--verbose",
		"arg1",
	}

	assert.Equal(t, expected, processed)
}

func TestUnit_Command_SymlinkMode_SymlinkTargetResolutionInvariants(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	symlinkPath := filepath.Join(tmpDir, "node")
	err := os.Symlink("/usr/local/bin/cderun", symlinkPath)
	require.NoError(t, err)

	args := []string{
		symlinkPath,
		"--cderun-image", "node:20-alpine",
		"--cderun-tty",
		"--version",
	}

	o := &rootOptions{}
	cmd := newRootCmd(o)
	processed, err := preprocessArgs(cmd, args)
	require.NoError(t, err)

	expected := []string{
		"cderun",
		"--cderun-image", "node:20-alpine",
		"--cderun-tty",
		"node",
		"--version",
	}

	assert.Equal(t, expected, processed)
}

func TestUnit_Command_ContainerConfigBuilder_AssemblyInvariants(t *testing.T) {
	t.Parallel()

	originalCmd := []string{"echo", "hello", "world"}
	assembled, err := assembleContainerCommand(originalCmd)
	require.NoError(t, err)

	assert.Equal(t, originalCmd, assembled)

	// Verify deep-copy slice isolation
	assembled[1] = "modified"
	assert.Equal(t, "hello", originalCmd[1])

	// Verify null-byte rejection in command arguments
	invalidCmd := []string{"sh", "-c", "echo \x00 malicious"}
	_, err = assembleContainerCommand(invalidCmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "null byte")
}

func TestUnit_Command_PreprocessArgs_FlagOrderAndInterleavingInvariants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		args     []string
		expected []string
		wantErr  string
	}{
		{
			name: "interleaved P1 space and equal overrides after subcommand",
			args: []string{
				"cderun", "go", "test", "./...",
				"--cderun-engine=podman",
				"--cderun-image", "golang:1.22",
				"-v",
			},
			expected: []string{
				"cderun",
				"--cderun-engine=podman",
				"--cderun-image", "golang:1.22",
				"go",
				"test",
				"./...",
				"-v",
			},
		},
		{
			name: "double dash before P1 override hoists flag regardless",
			args: []string{
				"cderun", "kubectl", "exec", "-it", "pod", "--",
				"--cderun-engine", "docker",
			},
			expected: []string{
				"cderun",
				"--cderun-engine", "docker",
				"kubectl",
				"exec",
				"-it",
				"pod",
				"--",
			},
		},
		{
			name: "value-taking P1 flag missing argument at end of args",
			args: []string{
				"cderun", "node", "index.js",
				"--cderun-image",
			},
			wantErr: "cderun internal override flag \"--cderun-image\" requires a value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			o := &rootOptions{}
			cmd := newRootCmd(o)
			processed, err := preprocessArgs(cmd, tt.args)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, processed)
			}
		})
	}
}
