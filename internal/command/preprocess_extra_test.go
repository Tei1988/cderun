package command

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Root_PreprocessArgs_Extra(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		args     []string
		expected []string
		wantErr  string
	}{
		{
			name:     "flags with values before subcommand",
			args:     []string{"cderun", "--image", "alpine", "--network", "host", "node", "--version"},
			expected: []string{"cderun", "--image", "alpine", "--network", "host", "node", "--version"},
		},
		{
			name:     "shorthand flags with values before subcommand",
			args:     []string{"cderun", "-t", "-u", "1000", "node", "-v"},
			expected: []string{"cderun", "-t", "-u", "1000", "node", "-v"},
		},
		{
			name:     "multiple P1 overrides after subcommand",
			args:     []string{"cderun", "node", "--cderun-tty", "--cderun-image", "node:20-alpine", "app.js"},
			expected: []string{"cderun", "--cderun-tty", "--cderun-image", "node:20-alpine", "node", "app.js"},
		},
		{
			name:     "P1 override with equals sign",
			args:     []string{"cderun", "node", "--cderun-image=node:20-alpine", "app.js"},
			expected: []string{"cderun", "--cderun-image=node:20-alpine", "node", "app.js"},
		},
		{
			name:     "P1 override with value in next arg",
			args:     []string{"cderun", "node", "--cderun-image", "node:20-alpine", "app.js"},
			expected: []string{"cderun", "--cderun-image", "node:20-alpine", "node", "app.js"},
		},
		{
			name:    "P1 override must be after subcommand in standard mode",
			args:    []string{"cderun", "--cderun-tty", "node", "app.js"},
			wantErr: "cderun internal override flag \"--cderun-tty\" must be placed after the subcommand",
		},
		{
			name:     "polyglot mode with P1 overrides and tool flags",
			args:     []string{"node", "--cderun-tty", "--version", "--cderun-image", "alpine"},
			expected: []string{"cderun", "--cderun-tty", "--cderun-image", "alpine", "node", "--version"},
		},
		{
			name:     "double dash -- currently does NOT stop hoisting (documenting T53)",
			args:     []string{"cderun", "echo", "--", "--cderun-tty"},
			expected: []string{"cderun", "--cderun-tty", "echo", "--"},
		},
		{
			name:     "no subcommand but has flags",
			args:     []string{"cderun", "--diagnosis", "--log-level", "debug"},
			expected: []string{"cderun", "--diagnosis", "--log-level", "debug"},
		},
		{
			name:     "complex interleaving (double dash behavior)",
			args:     []string{"cderun", "-t", "sh", "-c", "ls", "--cderun-image", "alpine", "--", "--cderun-literal"},
			expected: []string{"cderun", "--cderun-image", "alpine", "--cderun-literal", "-t", "sh", "-c", "ls", "--"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := &rootOptions{}
			cmd := newRootCmd(o)
			actual, err := preprocessArgs(cmd, tt.args)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, actual)
		})
	}
}
