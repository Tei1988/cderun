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

	// Subcommand 'alpine' is mandatory.
	stdout, stderr, exitCode, err := runCderunE2E(
		[]string{
			"--image", "public.ecr.aws/docker/library/alpine:latest",
			"--device", "/dev/null:/dev/null2:rw",
		},
		"alpine",
		[]string{"sh", "-c", "ls -l /dev/null2 && echo 'test' > /dev/null2"},
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

	// Ensure proper cleanup of resources
	t.Cleanup(func() {
		_ = pw.Close()
		_ = pr.Close()
	})

	go func() {
		_, writeErr := pw.Write([]byte(stdinData))
		// We close the writer to signal EOF to the reader (container stdin)
		_ = pw.Close()
		if writeErr != nil {
			// If the test environment is extremely slow or broken,
			// this might be logged but typically shouldn't fail the test
			// unless the reader is already closed.
			return
		}
	}()

	// Subcommand 'alpine' is mandatory.
	stdout, stderr, exitCode, err := runCderunWithStdinE2E(pr,
		[]string{
			"--image", "public.ecr.aws/docker/library/alpine:latest",
			"--interactive", "--cderun-tty=false", "--cderun-memory=512m",
		},
		"alpine",
		[]string{"sh", "-c", "cat"},
	)

	skipIfDockerBroken(t, err)

	require.NoError(t, err, "stderr: %s", stderr)
	assert.Equal(t, 0, exitCode, "command failed, stderr: %s", stderr)
	assert.Equal(t, stdinData, stdout)
}
