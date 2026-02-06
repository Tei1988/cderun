package command

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testImage = "public.ecr.aws/docker/library/alpine:latest"

// runCderun runs the cderun command in-process for integration testing.
// It captures stdout and stderr and returns the exit code.
func runCderun(args ...string) (stdout, stderr string, exitCode int, err error) {
	// Re-use logic from root_test.go but simplified
	originalStdout := os.Stdout
	originalStderr := os.Stderr

	rOut, wOut, err := os.Pipe()
	if err != nil {
		return "", "", 0, err
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		_ = rOut.Close()
		_ = wOut.Close()
		return "", "", 0, err
	}

	os.Stdout = wOut
	os.Stderr = wErr

	stdoutChan := make(chan string)
	stderrChan := make(chan string)

	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, rOut)
		_ = rOut.Close()
		stdoutChan <- buf.String()
	}()

	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, rErr)
		_ = rErr.Close()
		stderrChan <- buf.String()
	}()

	// Reset global state
	opts = rootOptions{}
	rootCmd = newRootCmd()
	rootCmd.SetOut(wOut)
	rootCmd.SetErr(wErr)

	// Mock exitFunc to capture exit code
	capturedExitCode := 0
	originalExitFunc := exitFunc
	exitFunc = func(code int) {
		capturedExitCode = code
	}
	defer func() {
		exitFunc = originalExitFunc
	}()

	execErr := Execute(append([]string{"cderun"}, args...))

	_ = wOut.Close()
	_ = wErr.Close()

	stdout = <-stdoutChan
	stderr = <-stderrChan

	os.Stdout = originalStdout
	os.Stderr = originalStderr

	return stdout, stderr, capturedExitCode, execErr
}

func skipIfDockerBroken(t *testing.T, err error) {
	if err != nil && strings.Contains(err.Error(), "failed to mount") && strings.Contains(err.Error(), "invalid argument") {
		t.Skip("Skipping test due to Docker mount limitation in this environment (likely overlay-on-overlay)")
	}
}

func TestIntegrationBasic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	t.Run("cderun alpine echo hello", func(t *testing.T) {
		// Note: since alpine has no entrypoint, the subcommand 'echo' is stripped
		// and the container command becomes 'hello-cderun'.
		// This will fail unless the image has 'echo' as entrypoint.
		// To fix this, we'll use a tool definition in .tools.yaml or just test that it fails/executes correctly.
		// Actually, the requirement is to use alpine as image and echo hello.
		// Let's use a temporary .tools.yaml for this test to be realistic.
		originalWd, err := os.Getwd()
		require.NoError(t, err)
		tmpDir := t.TempDir()
		require.NoError(t, os.Chmod(tmpDir, 0755))
		require.NoError(t, os.Chdir(tmpDir))
		t.Cleanup(func() { _ = os.Chdir(originalWd) })

		err = os.WriteFile(".tools.yaml", []byte("echo:\n  image: "+testImage+"\n  entrypoint: [\"echo\"]"), 0644)
		require.NoError(t, err)

		stdout, _, exitCode, err := runCderun("echo", "hello-cderun")
		skipIfDockerBroken(t, err)
		assert.NoError(t, err)
		assert.Equal(t, 0, exitCode)
		assert.Contains(t, stdout, "hello-cderun")
	})

	t.Run("volume mounting", func(t *testing.T) {
		originalWd, err := os.Getwd()
		require.NoError(t, err)
		tmpDir := t.TempDir()
		require.NoError(t, os.Chmod(tmpDir, 0755))
		require.NoError(t, os.Chdir(tmpDir))
		t.Cleanup(func() { _ = os.Chdir(originalWd) })

		err = os.WriteFile(".tools.yaml", []byte("cat:\n  image: "+testImage+"\n  entrypoint: [\"cat\"]"), 0644)
		require.NoError(t, err)

		hostFile := filepath.Join(tmpDir, "hello.txt")
		err = os.WriteFile(hostFile, []byte("hello-from-host"), 0644)
		require.NoError(t, err)

		stdout, stderr, exitCode, err := runCderun("--mount", "type=bind,source="+hostFile+",target=/hello.txt", "cat", "/hello.txt")
		skipIfDockerBroken(t, err)
		assert.NoError(t, err, "stderr: %s", stderr)
		assert.Equal(t, 0, exitCode, "stderr: %s", stderr)
		assert.Contains(t, stdout, "hello-from-host", "stdout: %s, stderr: %s", stdout, stderr)
	})

	t.Run("environment variables", func(t *testing.T) {
		originalWd, err := os.Getwd()
		require.NoError(t, err)
		tmpDir := t.TempDir()
		require.NoError(t, os.Chmod(tmpDir, 0755))
		require.NoError(t, os.Chdir(tmpDir))
		t.Cleanup(func() { _ = os.Chdir(originalWd) })

		err = os.WriteFile(".tools.yaml", []byte("env:\n  image: "+testImage+"\n  entrypoint: [\"env\"]"), 0644)
		require.NoError(t, err)

		t.Setenv("HOST_VAR", "host-value")
		stdout, _, exitCode, err := runCderun("-e", "EXPLICIT_VAR=explicit-value", "-e", "HOST_VAR", "env")
		skipIfDockerBroken(t, err)
		assert.NoError(t, err)
		assert.Equal(t, 0, exitCode)
		assert.Contains(t, stdout, "EXPLICIT_VAR=explicit-value")
		assert.Contains(t, stdout, "HOST_VAR=host-value")
	})

	t.Run("port mapping", func(t *testing.T) {
		_, _, exitCode, err := runCderun("--image", testImage, "-p", "8081:8000", "--entrypoint", "echo", "echo", "port-test")
		skipIfDockerBroken(t, err)
		assert.NoError(t, err)
		assert.Equal(t, 0, exitCode)
	})

	t.Run("cderun expressions", func(t *testing.T) {
		originalWd, err := os.Getwd()
		require.NoError(t, err)
		tmpDir := t.TempDir()
		require.NoError(t, os.Chmod(tmpDir, 0755))
		require.NoError(t, os.Chdir(tmpDir))
		t.Cleanup(func() {
			require.NoError(t, os.Chdir(originalWd))
		})

		err = os.WriteFile(".tools.yaml", []byte("mytool:\n  image: "+testImage+"\n  env:\n    - MY_PWD={{PWD}}"), 0644)
		require.NoError(t, err)

		stdout, _, exitCode, err := runCderun("mytool", "env")
		skipIfDockerBroken(t, err)
		assert.NoError(t, err)
		assert.Equal(t, 0, exitCode)
		assert.Contains(t, stdout, "MY_PWD="+tmpDir)
	})

	t.Run("relative path and tilde expansion", func(t *testing.T) {
		originalWd, err := os.Getwd()
		require.NoError(t, err)
		tmpDir := t.TempDir()
		require.NoError(t, os.Chmod(tmpDir, 0755))
		require.NoError(t, os.Chdir(tmpDir))
		t.Cleanup(func() {
			require.NoError(t, os.Chdir(originalWd))
		})

		subDir := filepath.Join(tmpDir, "subdir")
		err = os.MkdirAll(subDir, 0755)
		require.NoError(t, err)

		err = os.WriteFile(".tools.yaml", []byte("mytool:\n  image: "+testImage+"\n  mounts:\n    - type: bind\n      source: ./subdir\n      target: /mnt"), 0644)
		require.NoError(t, err)

		stdout, _, exitCode, err := runCderun("mytool", "ls", "-d", "/mnt")
		skipIfDockerBroken(t, err)
		assert.NoError(t, err)
		assert.Equal(t, 0, exitCode)
		assert.Contains(t, stdout, "/mnt")
	})

	t.Run("mount-cderun-path", func(t *testing.T) {
		customPath := "/tmp/custom-cderun"
		stdout, _, exitCode, err := runCderun("--image", testImage, "--mount-socket", "--mount-cderun", "--mount-cderun-path", customPath, "--dry-run", "--dry-run-format", "simple", "echo", "hello")
		assert.NoError(t, err)
		assert.Equal(t, 0, exitCode)
		assert.Contains(t, stdout, "source="+customPath+",target=/usr/local/bin/cderun")
	})
}
