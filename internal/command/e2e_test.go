//go:build e2e

package command

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestE2E_DockerVersion(t *testing.T) {
	// --diagnosis does not take a subcommand, so we pass empty targetCommand.
	stdout, stderr, exitCode, err := runCderunE2E([]string{"--diagnosis", "--diagnosis-format", "json"}, nil)
	skipIfDockerBroken(t, err)
	require.NoError(t, err, "stderr: %s", stderr)
	assert.Equal(t, 0, exitCode)

	// Log docker version for visibility in CI matrix
	fmt.Printf("Detected Docker Version in E2E: %s\n", stdout)
}

func TestE2E_StandardExecution(t *testing.T) {
	// Use --entrypoint sh to avoid OCI runtime "executable file not found" errors
	// when trying to execute "sh -c ..." directly.
	stdout, stderr, exitCode, err := runCderunE2E(
		[]string{"--image", "public.ecr.aws/docker/library/alpine:latest", "--entrypoint", "sh"},
		[]string{"sh", "-c", "echo hello-cderun-e2e"},
	)
	skipIfDockerBroken(t, err)
	require.NoError(t, err, "stderr: %s", stderr)
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, stdout, "hello-cderun-e2e")
}

func TestE2E_VolumeMount(t *testing.T) {
	// Use TEST_HOST_TMP_DIR if set, otherwise t.TempDir()
	var baseDir string
	if envDir := os.Getenv("TEST_HOST_TMP_DIR"); envDir != "" {
		// Ensure the directory exists
		err := os.MkdirAll(envDir, 0755)
		require.NoError(t, err)

		// Create a unique subdirectory for this test to avoid conflicts
		baseDir, err = os.MkdirTemp(envDir, "test-volume-")
		require.NoError(t, err)
		// Ensure cleanup for the created subdirectory
		t.Cleanup(func() {
			_ = os.RemoveAll(baseDir)
		})
	} else {
		baseDir = t.TempDir()
	}

	testFile := filepath.Join(baseDir, "test.txt")
	content := "cderun e2e volume test"
	err := os.WriteFile(testFile, []byte(content), 0644)
	require.NoError(t, err)

	stdout, stderr, exitCode, err := runCderunE2E(
		[]string{
			"--image", "public.ecr.aws/docker/library/alpine:latest",
			"--mount", fmt.Sprintf("source=%s,target=/mnt/test,readonly", baseDir),
			"--entrypoint", "sh",
		},
		[]string{"sh", "-c", "cat /mnt/test/test.txt"},
	)
	skipIfDockerBroken(t, err)
	require.NoError(t, err, "stderr: %s", stderr)
	assert.Equal(t, 0, exitCode)
	assert.Equal(t, content, strings.TrimSpace(stdout))
}

func TestE2E_NestedExecution(t *testing.T) {
	// Host -> Container A -> Container B
	// Container A needs Docker socket and cderun binary.

	// Use robust search for cderun binary
	exePath, err := findCderunBinary()
	require.NoError(t, err, "failed to resolve cderun binary path")

	// Docker socket resolution
	dockerSocket := "/var/run/docker.sock"
	if host := os.Getenv("DOCKER_HOST"); strings.HasPrefix(host, "unix://") {
		dockerSocket = strings.TrimPrefix(host, "unix://")
	}

	cderunFlags := []string{
		"--image", "public.ecr.aws/docker/library/alpine:latest",
		"--mount-cderun",
		"--mount-cderun-path", exePath,
	}

	// Handle docker socket or DOCKER_HOST passthrough
	if os.Getenv("DOCKER_HOST") != "" && !strings.HasPrefix(os.Getenv("DOCKER_HOST"), "unix://") {
		cderunFlags = append(cderunFlags, "--env", fmt.Sprintf("DOCKER_HOST=%s", os.Getenv("DOCKER_HOST")))
	} else {
		cderunFlags = append(cderunFlags, "--mount-socket", "--mount-socket-path", dockerSocket)
	}

	// Command in Container A: run cderun to start Container B
	// Using sh -c for robustness and explicit -- separator for the nested call
	targetCommand := []string{
		"sh", "-c",
		"cderun --image public.ecr.aws/docker/library/alpine:latest --entrypoint sh -- sh -c 'echo nested-success'",
	}

	stdout, stderr, exitCode, err := runCderunE2E(cderunFlags, targetCommand)
	skipIfDockerBroken(t, err)
	require.NoError(t, err, "stderr: %s", stderr)
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, stdout, "nested-success")
}

func TestE2E_DryRun(t *testing.T) {
	// Added skipIfDockerBroken for consistency and guard
	// Dry-run normally doesn't require a running Docker daemon, but we keep it for consistency.
	stdout, stderr, exitCode, err := runCderunE2E(
		[]string{"--image", "public.ecr.aws/docker/library/alpine:latest", "--dry-run", "--dry-run-format", "json"},
		[]string{"echo", "dry-run-test"},
	)
	skipIfDockerBroken(t, err)
	require.NoError(t, err, "stderr: %s", stderr)
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, stdout, "\"image\": \"public.ecr.aws/docker/library/alpine:latest\"")
	assert.Contains(t, stdout, "dry-run-test")
}
