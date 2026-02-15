package command

import (
	"testing"

	"cderun/internal/runtime"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Command_Root_DryRun(t *testing.T) {
	t.Run("dry-run requires a subcommand", func(t *testing.T) {
		setupTestOptions(t)
		mockRuntime := &runtime.MockRuntime{}
		setupMockRuntime(t, mockRuntime)

		_, _, err := executeCommand("--dry-run")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--dry-run requires a subcommand")
	})

	t.Run("dry-run outputs configuration and skips execution", func(t *testing.T) {
		setupTestOptions(t)
		mockRuntime := &runtime.MockRuntime{}
		setupMockRuntime(t, mockRuntime)

		// Dry-run with YAML (default)
		// Step 10.2: subcommand 'sh' is excluded from command
		output, _, err := executeCommand("--dry-run", "--image", "alpine", "sh", "echo", "hello")
		require.NoError(t, err)
		assert.Contains(t, output, "image: alpine")
		assert.Contains(t, output, "command:")
		assert.Contains(t, output, "- echo")
		assert.Contains(t, output, "- hello")
		assert.NotContains(t, output, "- sh")
		assert.Nil(t, mockRuntime.CreatedConfig, "Runtime should not be called in dry-run mode")

		// Dry-run with JSON
		output, _, err = executeCommand("--dry-run", "--dry-run-format", "json", "--image", "alpine", "sh", "echo", "hello")
		require.NoError(t, err)
		assert.Contains(t, output, "\"image\": \"alpine\"")
		assert.Contains(t, output, "\"command\": [")

		// Dry-run with simple
		output, _, err = executeCommand("--dry-run", "-f", "simple", "--image", "alpine", "sh", "echo", "hello")
		require.NoError(t, err)
		assert.Contains(t, output, "Image: alpine")
		assert.Contains(t, output, "Command: echo hello")
		assert.NotContains(t, output, "Command: sh")
		assert.Contains(t, output, "TTY: false")
		assert.Contains(t, output, "Interactive: false")
		assert.Contains(t, output, "Network: bridge")
		assert.Contains(t, output, "Remove: true")

		// Dry-run with mount
		output, _, err = executeCommand("--dry-run", "-f", "simple", "--image", "alpine", "--mount", "type=bind,source=/h,target=/c", "sh")
		require.NoError(t, err)
		assert.Contains(t, output, "Mounts: type=bind,source=/h,target=/c,readonly=false")

		// Dry-run with device
		output, _, err = executeCommand("--dry-run", "-f", "simple", "--image", "alpine", "--device", "/dev/video0:/dev/video1:ro", "sh")
		require.NoError(t, err)
		assert.Contains(t, output, "Devices: /dev/video0:/dev/video1:ro")
	})
}
