package command

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Command_Init_DryRun(t *testing.T) {
	t.Parallel()

	t.Run("dry-run with --init outputs init: true in yaml", func(t *testing.T) {
		output, err := executeCommandContext(context.Background(), "--dry-run", "--init", "--image", "alpine", "sh", "echo", "hello")
		require.NoError(t, err)
		assert.Contains(t, output, "init: true")
	})

	t.Run("dry-run with --cderun-init overrides --init in yaml", func(t *testing.T) {
		output, err := executeCommandContext(context.Background(), "--dry-run", "--init=false", "--image", "alpine", "sh", "--cderun-init=true", "echo", "hello")
		require.NoError(t, err)
		assert.Contains(t, output, "init: true")
	})

	t.Run("dry-run simple format outputs Init: true", func(t *testing.T) {
		output, err := executeCommandContext(context.Background(), "--dry-run", "--dry-run-format", "simple", "--init", "--image", "alpine", "sh", "echo", "hello")
		require.NoError(t, err)
		assert.Contains(t, output, "Init: true")
	})

	t.Run("dry-run simple format outputs Init: false when disabled", func(t *testing.T) {
		output, err := executeCommandContext(context.Background(), "--dry-run", "--dry-run-format", "simple", "--init=false", "--image", "alpine", "sh", "echo", "hello")
		require.NoError(t, err)
		assert.Contains(t, output, "Init: false")
	})
}
