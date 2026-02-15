package command

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"cderun/internal/runtime"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

		o := setupTestOptions(t)
		setupMockRuntime(t, o, mock)

		pr, pw := io.Pipe()
		var stdout bytes.Buffer

		o.in = pr
		o.out = &stdout
		o.err = io.Discard

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		done := make(chan struct{})
		var execErr error
		go func() {
			execErr = ExecuteWithOptions(ctx, []string{"cderun", "--image", "alpine", "-i", "cat"}, o)
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
		assert.Equal(t, testData, stdout.String())
	})

	t.Run("piped stdin does NOT reach container when interactive is false", func(t *testing.T) {
		mock := &pipeMockRuntime{}
		mock.CreatedContainerID = "test-container"
		mock.ExitCode = 0

		o := setupTestOptions(t)
		setupMockRuntime(t, o, mock)

		pr, pw := io.Pipe()
		var stdout bytes.Buffer

		o.in = pr
		o.out = &stdout
		o.err = io.Discard

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		done := make(chan struct{})
		go func() {
			_ = ExecuteWithOptions(ctx, []string{"cderun", "--image", "alpine", "cat"}, o)
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
		assert.Empty(t, stdout.String())
	})
}
