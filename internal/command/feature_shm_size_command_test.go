package command

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Command_ShmSize_DryRun(t *testing.T) {
	t.Parallel()

	t.Run("dry-run with --shm-size outputs shm_size in yaml", func(t *testing.T) {
		output, err := executeCommandContext(context.Background(), "--dry-run", "--shm-size", "512m", "--image", "alpine", "sh", "echo", "hello")
		require.NoError(t, err)
		assert.Contains(t, output, "shm_size: 512m")
	})

	t.Run("dry-run with --cderun-shm-size overrides --shm-size in yaml", func(t *testing.T) {
		output, err := executeCommandContext(context.Background(), "--dry-run", "--shm-size", "128m", "--image", "alpine", "sh", "--cderun-shm-size=1g", "echo", "hello")
		require.NoError(t, err)
		assert.Contains(t, output, "shm_size: 1g")
	})

	t.Run("dry-run simple format outputs ShmSize: ...", func(t *testing.T) {
		output, err := executeCommandContext(context.Background(), "--dry-run", "--dry-run-format", "simple", "--shm-size", "256m", "--image", "alpine", "sh", "echo", "hello")
		require.NoError(t, err)
		assert.Contains(t, output, "ShmSize: 256m")
	})
}
