package command

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Root_PreprocessArgs_ExtraCoverage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		args     []string
		expected []string
		wantErr  bool
	}{
		{
			name:     "standard mode - no subcommand, P1 flag at start",
			args:     []string{"cderun", "--cderun-tty"},
			expected: []string{"cderun", "--cderun-tty"},
		},
		{
			name:     "standard mode - unknown long flag skipping",
			args:     []string{"cderun", "--unknown", "subcmd"},
			expected: []string{"cderun", "--unknown", "subcmd"},
		},
		{
			name:     "standard mode - unknown shorthand flag skipping",
			args:     []string{"cderun", "-X", "subcmd"},
			expected: []string{"cderun", "-X", "subcmd"},
		},
		{
			name:     "standard mode - shorthand with value attached",
			args:     []string{"cderun", "-p8080", "subcmd"},
			expected: []string{"cderun", "-p8080", "subcmd"},
		},
		{
			name:     "hoisting - P1 flag with equals sign and value",
			args:     []string{"cderun", "node", "--cderun-image=alpine"},
			expected: []string{"cderun", "--cderun-image=alpine", "node"},
		},
		{
			name:     "hoisting - P1 flag that takes an argument (equals-sign format required)",
			args:     []string{"cderun", "node", "--cderun-image=alpine", "app.js"},
			expected: []string{"cderun", "--cderun-image=alpine", "node", "app.js"},
		},
		{
			name:     "hoisting - unknown P1 flag stays as is (one arg, value not hoisted)",
			args:     []string{"cderun", "node", "--cderun-unknown=val"},
			expected: []string{"cderun", "--cderun-unknown=val", "node"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newRootCmd(&rootOptions{})
			actual, err := preprocessArgs(cmd, tt.args)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, actual)
		})
	}
}
