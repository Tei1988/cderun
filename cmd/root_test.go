package cmd

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func executeCommand(args ...string) (string, error) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = append([]string{"cderun"}, args...)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	rootCmd.SetOut(w)
	rootCmd.SetErr(w)
	err := Execute()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)

	return buf.String(), err
}

func TestPreprocessArgs(t *testing.T) {
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := preprocessArgs(tt.args)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestRootCmd(t *testing.T) {
	t.Run("parses flags and subcommand correctly", func(t *testing.T) {
		output, err := executeCommand("--tty", "-i", "--network", "host", "docker", "run", "--rm", "-it", "--tty", "ubuntu:latest", "bash")
		assert.NoError(t, err)

		assert.Contains(t, output, "TTY: true")
		assert.Contains(t, output, "Interactive: true")
		assert.Contains(t, output, "Network: host")
		assert.Contains(t, output, "Subcommand: docker")
		assert.Contains(t, output, "Passthrough Args: [run --rm -it --tty ubuntu:latest bash]")
	})

	t.Run("handles boundary case with --tty flag", func(t *testing.T) {
		output, err := executeCommand("--tty", "docker", "--tty")
		assert.NoError(t, err)

		assert.Contains(t, output, "TTY: true")
		assert.Contains(t, output, "Subcommand: docker")
		assert.Contains(t, output, "Passthrough Args: [--tty]")
	})

	t.Run("shows help when no subcommand is provided", func(t *testing.T) {
		output, err := executeCommand("--tty")
		assert.NoError(t, err)

		assert.True(t, strings.HasPrefix(output, "cderun is a CLI wrapper tool"))
		assert.Contains(t, output, "Usage:")
	})

	t.Run("handles symlink execution via Execute", func(t *testing.T) {
		oldArgs := os.Args
		defer func() { os.Args = oldArgs }()
		os.Args = []string{"node", "--version"}

		r, w, _ := os.Pipe()
		rootCmd.SetOut(w)
		rootCmd.SetErr(w)

		err := Execute()
		w.Close()

		var buf bytes.Buffer
		io.Copy(&buf, r)
		output := buf.String()

		assert.NoError(t, err)
		assert.Contains(t, output, "Subcommand: node")
		assert.Contains(t, output, "Passthrough Args: [--version]")
	})
}
