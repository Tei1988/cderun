package command

import (
	"context"
	"testing"

	"cderun/internal/config"
	"cderun/internal/logging"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Command_PassthroughArgumentsNullByteHardening(t *testing.T) {
	t.Parallel()

	t.Run("Passthrough argument with null byte is rejected", func(t *testing.T) {
		ctx := context.Background()
		err := ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "echo", "hello\x00world"}, func(o *rootOptions, cmd *cobra.Command) {
			// Mock or extra settings
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "security validation failed: command argument")
		assert.Contains(t, err.Error(), "contains null byte")
	})

	t.Run("Valid passthrough arguments are accepted", func(t *testing.T) {
		opts := &rootOptions{
			logger: logging.NewLogger(),
		}
		resolved := &config.ResolvedConfig{
			Image: "alpine",
		}

		cc, err := opts.buildContainerConfig(resolved, []string{"echo", "hello world"}, nil)
		require.NoError(t, err)
		require.NotNil(t, cc)
		assert.Equal(t, []string{"echo", "hello world"}, cc.Command)
	})
}
