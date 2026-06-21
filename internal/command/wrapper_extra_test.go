package command

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_PreprocessArgs_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		execName string
		args     []string
		want     []string
		wantErr  string
	}{
		{
			name:     "Standard mode - no subcommand",
			execName: "cderun",
			args:     []string{"cderun", "--log-level", "debug"},
			want:     []string{"cderun", "--log-level", "debug"},
		},
		{
			name:     "Standard mode - P1 before subcommand (error)",
			execName: "cderun",
			args:     []string{"cderun", "--cderun-log-level", "debug", "ls"},
			wantErr:  "cderun internal override flag \"--cderun-log-level\" must be placed after the subcommand",
		},
		{
			name:     "Standard mode - P1 after subcommand (hoisted)",
			execName: "cderun",
			args:     []string{"cderun", "ls", "--cderun-log-level", "debug"},
			want:     []string{"cderun", "--cderun-log-level", "debug", "ls"},
		},
		{
			name:     "Standard mode - P2 after subcommand (passthrough)",
			execName: "cderun",
			args:     []string{"cderun", "ls", "-l", "--image", "alpine"},
			want:     []string{"cderun", "ls", "-l", "--image", "alpine"},
		},
		{
			name:     "Polyglot mode - P1 after tool",
			execName: "node",
			args:     []string{"node", "app.js", "--cderun-image", "node:alpine"},
			want:     []string{"cderun", "--cderun-image", "node:alpine", "node", "app.js"},
		},
		{
			name:     "Polyglot mode - P2 after tool (passthrough)",
			execName: "node",
			args:     []string{"node", "app.js", "--image", "node:alpine"},
			want:     []string{"cderun", "node", "app.js", "--image", "node:alpine"},
		},
		{
			name:     "Standard mode - complicated flags before subcommand",
			execName: "cderun",
			args:     []string{"cderun", "--log-level", "debug", "-i", "ls", "-l"},
			want:     []string{"cderun", "--log-level", "debug", "-i", "ls", "-l"},
		},
		{
			name:     "Standard mode - flag with = before subcommand",
			execName: "cderun",
			args:     []string{"cderun", "--log-level=info", "ls"},
			want:     []string{"cderun", "--log-level=info", "ls"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := defaultOptions()
			cmd := newRootCmd(&opts)

			// Mock the execution environment for preprocessArgs
			args := make([]string, len(tt.args))
			copy(args, tt.args)
			if tt.execName != "cderun" {
				args[0] = "/usr/local/bin/" + tt.execName
			}

			got, err := preprocessArgs(cmd, args)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
