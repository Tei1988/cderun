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
	// Subcommand is mandatory. For diagnosis, we use 'diagnosis' as the tool name.
	stdout, stderr, exitCode, err := runCderunE2E([]string{"--diagnosis", "--diagnosis-format", "json"}, "diagnosis", nil)
	skipIfDockerBroken(t, err)
	require.NoError(t, err, "stderr: %s", stderr)
	assert.Equal(t, 0, exitCode)

	// Log docker version for visibility in CI matrix
	fmt.Printf("Detected Docker Version in E2E: %s\n", stdout)
}

func TestE2E_StandardExecution(t *testing.T) {
	// Mandatory subcommand 'alpine' identifies the tool/image context.
	stdout, stderr, exitCode, err := runCderunE2E(
		[]string{"--image", "public.ecr.aws/docker/library/alpine:latest"},
		"alpine",
		[]string{"echo", "hello-cderun-e2e"},
	)
	skipIfDockerBroken(t, err)
	require.NoError(t, err, "stderr: %s", stderr)
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, stdout, "hello-cderun-e2e")
}

func TestE2E_VolumeMount(t *testing.T) {
	var baseDir string
	if envDir := os.Getenv("TEST_HOST_TMP_DIR"); envDir != "" {
		err := os.MkdirAll(envDir, 0755)
		require.NoError(t, err)

		baseDir, err = os.MkdirTemp(envDir, "test-volume-")
		require.NoError(t, err)
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
		},
		"alpine",
		[]string{"cat", "/mnt/test/test.txt"},
	)
	skipIfDockerBroken(t, err)
	require.NoError(t, err, "stderr: %s", stderr)
	assert.Equal(t, 0, exitCode)
	assert.Equal(t, content, strings.TrimSpace(stdout))
}

func TestE2E_NestedExecution(t *testing.T) {
	// Host -> Container A -> Container B

	exePath, err := findCderunBinary()
	require.NoError(t, err, "failed to resolve cderun binary path")

	dockerSocket := "/var/run/docker.sock"
	if host := os.Getenv("DOCKER_HOST"); strings.HasPrefix(host, "unix://") {
		dockerSocket = strings.TrimPrefix(host, "unix://")
	}

	cderunFlags := []string{
		"--image", "public.ecr.aws/docker/library/alpine:latest",
		"--mount-cderun",
		"--mount-cderun-path", exePath,
	}

	if os.Getenv("DOCKER_HOST") != "" && !strings.HasPrefix(os.Getenv("DOCKER_HOST"), "unix://") {
		cderunFlags = append(cderunFlags, "--env", fmt.Sprintf("DOCKER_HOST=%s", os.Getenv("DOCKER_HOST")))
	} else {
		cderunFlags = append(cderunFlags, "--mount-socket", "--mount-socket-path", dockerSocket)
	}

	// Host-side Call: cderun [flags] alpine [container command]
	// Container Command: cderun --image ... alpine echo nested-success
	commandOptions := []string{
		"cderun", "--image", "public.ecr.aws/docker/library/alpine:latest", "alpine", "echo", "nested-success",
	}

	stdout, stderr, exitCode, err := runCderunE2E(cderunFlags, "alpine", commandOptions)
	skipIfDockerBroken(t, err)
	require.NoError(t, err, "stderr: %s", stderr)
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, stdout, "nested-success")
}

func TestE2E_DryRun(t *testing.T) {
	stdout, stderr, exitCode, err := runCderunE2E(
		[]string{"--image", "public.ecr.aws/docker/library/alpine:latest", "--dry-run", "--dry-run-format", "json"},
		"alpine",
		[]string{"echo", "dry-run-test"},
	)
	skipIfDockerBroken(t, err)
	require.NoError(t, err, "stderr: %s", stderr)
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, stdout, "\"image\": \"public.ecr.aws/docker/library/alpine:latest\"")
	assert.Contains(t, stdout, "dry-run-test")
}
