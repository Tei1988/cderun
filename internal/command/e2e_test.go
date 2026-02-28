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
	stdout, stderr, exitCode, err := runCderun("--diagnosis", "--diagnosis-format", "json")
	skipIfDockerBroken(t, err)
	require.NoError(t, err, "stderr: %s", stderr)
	assert.Equal(t, 0, exitCode)

	// Log docker version for visibility in CI matrix
	fmt.Printf("Detected Docker Version in E2E: %s\n", stdout)
}

func TestE2E_StandardExecution(t *testing.T) {
	stdout, stderr, exitCode, err := runCderun(
		"--image", "public.ecr.aws/docker/library/alpine:latest",
		"echo", "hello-cderun-e2e",
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
	} else {
		baseDir = t.TempDir()
	}

	testFile := filepath.Join(baseDir, "test.txt")
	content := "cderun e2e volume test"
	err := os.WriteFile(testFile, []byte(content), 0644)
	require.NoError(t, err)

	stdout, stderr, exitCode, err := runCderun(
		"--image", "public.ecr.aws/docker/library/alpine:latest",
		"--mount", fmt.Sprintf("source=%s,target=/mnt/test,readonly", baseDir),
		"cat", "/mnt/test/test.txt",
	)
	skipIfDockerBroken(t, err)
	require.NoError(t, err, "stderr: %s", stderr)
	assert.Equal(t, 0, exitCode)
	assert.Equal(t, content, strings.TrimSpace(stdout))
}

func TestE2E_NestedExecution(t *testing.T) {
	// Host -> Container A -> Container B
	// Container A needs Docker socket and cderun binary.

	// Use absolute path for cderun binary (built in GHA workflow or present in local)
	exePath, err := os.Executable()
	if strings.Contains(exePath, "go-build") {
		// If running via go test, look for built binary in current dir
		if _, err := os.Stat("./cderun"); err == nil {
			exePath, _ = filepath.Abs("./cderun")
		}
	}
	require.NoError(t, err)

	// We need to know where the docker socket is. Default to /var/run/docker.sock
	dockerSocket := "/var/run/docker.sock"
	if host := os.Getenv("DOCKER_HOST"); strings.HasPrefix(host, "unix://") {
		dockerSocket = strings.TrimPrefix(host, "unix://")
	}

	// In DinD environment, DOCKER_HOST might be tcp.
	// For nested execution (DooD) to work, the INNER container needs to talk to the SAME daemon.
	// If DOCKER_HOST is tcp, we pass it to the inner container.

	args := []string{
		"--image", "public.ecr.aws/docker/library/alpine:latest",
		"--mount-cderun",
		"--mount-cderun-path", exePath,
	}

	// Handle docker socket or DOCKER_HOST passthrough
	if os.Getenv("DOCKER_HOST") != "" && !strings.HasPrefix(os.Getenv("DOCKER_HOST"), "unix://") {
		// If TCP, pass the environment variable
		args = append(args, "--env", fmt.Sprintf("DOCKER_HOST=%s", os.Getenv("DOCKER_HOST")))
	} else {
		// If Unix socket, mount it
		args = append(args, "--mount-socket", "--mount-socket-path", dockerSocket)
	}

	// Command in Container A: run cderun to start Container B
	args = append(args, "cderun", "--image", "public.ecr.aws/docker/library/alpine:latest", "echo", "nested-success")

	stdout, stderr, exitCode, err := runCderun(args...)
	skipIfDockerBroken(t, err)
	require.NoError(t, err, "stderr: %s", stderr)
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, stdout, "nested-success")
}

func TestE2E_DryRun(t *testing.T) {
	stdout, stderr, exitCode, err := runCderun(
		"--image", "public.ecr.aws/docker/library/alpine:latest",
		"--dry-run",
		"--dry-run-format", "json",
		"echo", "dry-run-test",
	)
	require.NoError(t, err, "stderr: %s", stderr)
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, stdout, "\"image\": \"public.ecr.aws/docker/library/alpine:latest\"")
	assert.Contains(t, stdout, "dry-run-test")
}
