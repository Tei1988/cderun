package command

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDryRun(t *testing.T) {
	t.Run("default yaml output", func(t *testing.T) {
		// Reset flags
	dryRun = true
	dryRunFormat = "yaml"
	tty = false
	interactive = false
	defer func() {
		dryRun = false
		dryRunFormat = ""
	}()

		output, err := executeCommand("node", "--version")
		assert.NoError(t, err)

		assert.Contains(t, output, "image: alpine:latest")
		assert.Contains(t, output, "command:\n  - node")
		assert.Contains(t, output, "args:\n  - --version")
	})

	t.Run("json output", func(t *testing.T) {
		// Reset flags
	dryRun = true
	dryRunFormat = "json"
	defer func() {
		dryRun = false
		dryRunFormat = ""
	}()

		output, err := executeCommand("node", "--version")
		assert.NoError(t, err)

		assert.Contains(t, output, "\"image\": \"alpine:latest\"")
		assert.Contains(t, output, "\"command\": [")
		assert.Contains(t, output, "\"node\"")
	})

	t.Run("simple output", func(t *testing.T) {
		// Reset flags
	dryRun = true
	dryRunFormat = "simple"
	defer func() {
		dryRun = false
		dryRunFormat = ""
	}()

		output, err := executeCommand("node", "--version")
		assert.NoError(t, err)

		assert.Contains(t, output, "Image: alpine:latest")
		assert.Contains(t, output, "Command: node --version")
	})

	t.Run("unsupported format", func(t *testing.T) {
		// Reset flags
	dryRun = true
	dryRunFormat = "invalid"
	defer func() {
		dryRun = false
		dryRunFormat = ""
	}()

		_, err := executeCommand("node", "--version")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported format: invalid")
	})
}