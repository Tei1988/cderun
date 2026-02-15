package command

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/runtime"
)

type pipeMockRuntime struct {
	runtime.MockRuntime
}

func (m *pipeMockRuntime) AttachContainer(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer) error {
	// Call embedded mock for record keeping
	_ = m.MockRuntime.AttachContainer(ctx, containerID, tty, stdin, stdout, stderr)

	if stdin != nil && stdout != nil {
		// Simulate container that echoes back stdin to stdout
		_, err := io.Copy(stdout, stdin)
		return err
	}
	return nil
}

func TestUnit_Command_Root_PipedStdin(t *testing.T) {
	t.Run("piped stdin reaches container when interactive is true", func(t *testing.T) {
		mock := &pipeMockRuntime{}
		mock.CreatedContainerID = "test-container"
		mock.ExitCode = 0

		setupTestOptions(t)
		o := testOptions
		o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
			return mock, nil
		}
		o.exitFunc = func(code int) {}

		pr, pw := io.Pipe()

		o.stdin = pr

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		done := make(chan struct{})
		var execErr error
		var capturedStdout string
		go func() {
			capturedStdout, _, _, execErr = runCderunWithOptions(ctx, o, "--image", "alpine", "-i", "cat")
			close(done)
		}()

		testData := "hello from pipe\n"
		go func() {
			_, _ = pw.Write([]byte(testData))
			_ = pw.Close()
		}()

		select {
		case <-done:
		case <-ctx.Done():
			_ = pr.Close()
			t.Fatal("Test timed out")
		}

		_ = pr.Close()
		require.NoError(t, execErr)
		assert.Equal(t, testData, capturedStdout)
	})

	t.Run("piped stdin does NOT reach container when interactive is false", func(t *testing.T) {
		mock := &pipeMockRuntime{}
		mock.CreatedContainerID = "test-container"
		mock.ExitCode = 0

		setupTestOptions(t)
		o := testOptions
		o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
			return mock, nil
		}
		o.exitFunc = func(code int) {}

		pr, pw := io.Pipe()

		o.stdin = pr

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		done := make(chan struct{})
		var capturedStdout string
		go func() {
			capturedStdout, _, _, _ = runCderunWithOptions(ctx, o, "--image", "alpine", "cat")
			close(done)
		}()

		// Use a goroutine to write to avoid blocking if cat doesn't read
		go func() {
			_, _ = pw.Write([]byte("hello\n"))
			_ = pw.Close()
		}()

		select {
		case <-done:
		case <-ctx.Done():
			_ = pr.Close() // Unblock writer if it's still running
			t.Fatal("Test timed out")
		}

		_ = pr.Close() // Unblock writer to allow it to finish
		assert.Empty(t, capturedStdout)
	})
}
