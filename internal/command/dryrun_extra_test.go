package command

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Root_DryRun_SimpleFormat_ExtraExhaustive(t *testing.T) {
	t.Parallel()

	output, err := executeCommand("--dry-run", "-f", "simple",
		"--image", "alpine",
		"--workdir", "/app",
		"--user", "nobody",
		"--publish-all",
		"--env", "FOO=BAR",
		"--mount", "type=bind,source=/host,target=/cont,readonly",
		"sh", "-c", "echo ok")

	require.NoError(t, err)
	assert.Contains(t, output, "Workdir: /app")
	assert.Contains(t, output, "User: nobody")
	assert.Contains(t, output, "PublishAll: true")
	assert.Contains(t, output, "Env: \"FOO\"=\"BAR\"")
	assert.Contains(t, output, "Mounts: type=bind,source=\"/host\",target=\"/cont\",readonly=true")
}
