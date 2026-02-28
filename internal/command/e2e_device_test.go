//go:build e2e

package command

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScenario_Device_MountNull(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	// We use 'alpine' as the mandatory subcommand.
	stdout, stderr, exitCode, err := runCderunE2E(
		[]string{
			"--image", "public.ecr.aws/docker/library/alpine:latest",
			"--device", "/dev/null:/dev/null2:rw",
		},
		[]string{"alpine", "sh", "-c", "ls -l /dev/null2 && echo 'test' > /dev/null2"},
	)

	// Handle environment-specific Docker issues
	skipIfDockerBroken(t, err)

	require.NoError(t, err, "stderr: %s", stderr)
	assert.Equal(t, 0, exitCode, "command failed, stderr: %s", stderr)
	assert.Contains(t, stdout, "/dev/null2", "stdout should contain /dev/null2")
}

func TestScenario_Stdin_Piped(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	stdinData := "hello e2e stdin"

	// Create a pipe for stdin
	pr, pw := io.Pipe()
	go func() {
		_, _ = pw.Write([]byte(stdinData))
		_ = pw.Close()
	}()

	// We use 'alpine' as the mandatory subcommand.
	stdout, stderr, exitCode, err := runCderunWithStdinE2E(pr,
		[]string{
			"--image", "public.ecr.aws/docker/library/alpine:latest",
			"--interactive",
		},
		[]string{"alpine", "cat"},
	)

	skipIfDockerBroken(t, err)

	require.NoError(t, err, "stderr: %s", stderr)
	assert.Equal(t, 0, exitCode, "command failed, stderr: %s", stderr)
	assert.Equal(t, stdinData, stdout)
}
