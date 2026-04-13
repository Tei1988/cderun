//go:build runtime

package command

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testImage = "public.ecr.aws/docker/library/alpine:latest"

func TestScenario_Execution_AlpineEcho(t *testing.T) {
	t.Run("echo hello", func(t *testing.T) {
		setupTestDir(t)

		err := os.WriteFile(".tools.yaml", []byte("echo:\n  image: "+testImage+"\n  entrypoint: [\"echo\"]"), 0o644)
		require.NoError(t, err)

		stdout, _, exitCode, err := runCderun("echo", "hello-cderun")
		checkRuntimeResult(t, stdout, "", exitCode, err)
		assert.Equal(t, 0, exitCode)
		assert.Contains(t, stdout, "hello-cderun")
	})

	t.Run("volume mounting", func(t *testing.T) {
		tmpDir := setupTestDir(t)

		err := os.WriteFile(".tools.yaml", []byte("cat:\n  image: "+testImage+"\n  entrypoint: [\"cat\"]"), 0o644)
		require.NoError(t, err)

		hostFile := filepath.Join(tmpDir, "hello.txt")
		err = os.WriteFile(hostFile, []byte("hello-from-host"), 0o644)
		require.NoError(t, err)

		stdout, stderr, exitCode, err := runCderun("--mount", "type=bind,source="+hostFile+",target=/hello.txt", "cat", "/hello.txt")
		checkRuntimeResult(t, stdout, stderr, exitCode, err)
		assert.Equal(t, 0, exitCode, "stderr: %s", stderr)
		assert.Contains(t, stdout, "hello-from-host")
	})

	t.Run("environment variables", func(t *testing.T) {
		setupTestDir(t)

		err := os.WriteFile(".tools.yaml", []byte("env:\n  image: "+testImage+"\n  entrypoint: [\"env\"]"), 0o644)
		require.NoError(t, err)

		t.Setenv("HOST_VAR", "host-value")
		stdout, _, exitCode, err := runCderun("-e", "EXPLICIT_VAR=explicit-value", "-e", "HOST_VAR", "env")
		checkRuntimeResult(t, stdout, "", exitCode, err)
		assert.Equal(t, 0, exitCode)
		assert.Contains(t, stdout, "EXPLICIT_VAR=explicit-value")
		assert.Contains(t, stdout, "HOST_VAR=host-value")
	})

	t.Run("port mapping", func(t *testing.T) {
		_, _, exitCode, err := runCderun("--image", testImage, "-p", "8081:8000", "--entrypoint", "echo", "echo", "port-test")
		checkRuntimeResult(t, "", "", exitCode, err)
		assert.Equal(t, 0, exitCode)
	})

	t.Run("cderun expressions", func(t *testing.T) {
		tmpDir := setupTestDir(t)

		err := os.WriteFile(".tools.yaml", []byte("mytool:\n  image: "+testImage+"\n  env:\n    - MY_PWD={{PWD}}"), 0o644)
		require.NoError(t, err)

		stdout, _, exitCode, err := runCderun("mytool", "env")
		checkRuntimeResult(t, stdout, "", exitCode, err)
		assert.Equal(t, 0, exitCode)
		assert.Contains(t, stdout, "MY_PWD="+tmpDir)
	})

	t.Run("relative path and tilde expansion", func(t *testing.T) {
		tmpDir := setupTestDir(t)

		subDir := filepath.Join(tmpDir, "subdir")
		err := os.MkdirAll(subDir, 0o755)
		require.NoError(t, err)

		err = os.WriteFile(".tools.yaml", []byte("mytool:\n  image: "+testImage+"\n  mounts:\n    - type: bind\n      source: ./subdir\n      target: /mnt"), 0o644)
		require.NoError(t, err)

		stdout, _, exitCode, err := runCderun("mytool", "ls", "-d", "/mnt")
		checkRuntimeResult(t, stdout, "", exitCode, err)
		assert.Equal(t, 0, exitCode)
		assert.Contains(t, stdout, "/mnt")
	})
}
