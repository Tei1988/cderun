package command

import (
	"context"
	"testing"

	"cderun/internal/config"
	"cderun/internal/runtime"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScenario_BDD_ArgumentHoisting(t *testing.T) {
	t.Parallel()

	// Given: A command line with P1 internal flags placed after the subcommand
	// When: preprocessArgs is called
	// Then: The P1 flags should be hoisted to follow the main 'cderun' executable name

	tests := []struct {
		name     string
		args     []string
		expected []string
	}{
		{
			name:     "Hoist single P1 flag",
			args:     []string{"cderun", "ls", "--cderun-image", "alpine"},
			expected: []string{"cderun", "--cderun-image", "alpine", "ls"},
		},
		{
			name:     "Hoist multiple P1 flags and preserve subcommand args",
			args:     []string{"cderun", "node", "app.js", "--cderun-tty", "--cderun-log-level", "debug"},
			expected: []string{"cderun", "--cderun-tty", "--cderun-log-level", "debug", "node", "app.js"},
		},
		{
			name:     "Handle polyglot mode (node symlink)",
			args:     []string{"node", "app.js", "--cderun-image", "node:20"},
			expected: []string{"cderun", "--cderun-image", "node:20", "node", "app.js"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			localOpts := defaultOptions()
			cmd := newRootCmd(&localOpts)
			actual, err := preprocessArgs(cmd, tt.args)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestScenario_BDD_ConfigurationPrecedence(t *testing.T) {
	t.Parallel()

	// Given: Multiple configuration sources (CLI, Env, Tool, Global)
	// When: The command is executed
	// Then: The resolved configuration should follow the P1-P5 precedence rules

	t.Run("P1 > P2 > P3 > P4 > P5 for TTY", func(t *testing.T) {
		mockRuntime := &runtime.MockRuntime{}

		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "--tty=true", "--cderun-tty=false", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
			o.fs = &config.MockFileSystem{
				Env: map[string]string{
					"CDERUN_TTY": "true", // P3
				},
				Files: map[string][]byte{
					"/.tools.yaml": []byte("sh:\n  tty: true\n"), // P4
					"/.cderun.yaml": []byte("defaults:\n  tty: true\n"), // P5
				},
				WD: "/",
			}
			o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
			o.isTerminal = func(fd int) bool { return true }
		})
		require.NoError(t, err)
		assert.False(t, mockRuntime.CreatedConfig.TTY, "P1 (--cderun-tty=false) should win")
	})

	t.Run("P2 > P3 > P4 > P5 for Network", func(t *testing.T) {
		mockRuntime := &runtime.MockRuntime{}

		err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "--network", "host", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
			o.fs = &config.MockFileSystem{
				Env: map[string]string{
					"CDERUN_NETWORK": "none",
				},
				Files: map[string][]byte{
					"/.tools.yaml": []byte("sh:\n  network: bridge\n"),
				},
				WD: "/",
			}
			o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
				return mockRuntime, nil
			}
		})
		require.NoError(t, err)
		assert.Equal(t, "host", mockRuntime.CreatedConfig.Network, "P2 (--network host) should win")
	})
}
