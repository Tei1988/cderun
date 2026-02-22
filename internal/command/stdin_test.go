package command

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/runtime"
)

type pipeMockRuntime struct {
	runtime.MockRuntime
}

func (m *pipeMockRuntime) AttachContainer(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error {
	// Call embedded mock for record keeping
	_ = m.MockRuntime.AttachContainer(ctx, containerID, tty, stdin, stdout, stderr, ready)

	if stdin != nil && stdout != nil {
		// Simulate container that echoes back stdin to stdout
		_, err := io.Copy(stdout, stdin)
		return err
	}
	return nil
}

func TestUnit_Stdin_Piped(t *testing.T) {
	t.Run("piped stdin reaches container when interactive is true", func(t *testing.T) {
		mock := &pipeMockRuntime{}
		mock.CreatedContainerID = "test-container"
		mock.ExitCode = 0

		pr, pw := io.Pipe()
		defer pr.Close()
		var stdout bytes.Buffer

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		go func() {
			<-ctx.Done()
			_ = pr.Close()
		}()

		done := make(chan struct{})
		var execErr error
		go func() {
			execErr = ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "-i", "cat"}, func(o *rootOptions, cmd *cobra.Command) {
				o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
					return mock, nil
				}
				o.exitFunc = func(code int) {}
				cmd.SetIn(pr)
				cmd.SetOut(&stdout)
				cmd.SetErr(io.Discard)
			})
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
			t.Fatal("Test timed out")
		}

		require.NoError(t, execErr)
		assert.Equal(t, testData, stdout.String())
	})

	t.Run("piped stdin does NOT reach container when interactive is false", func(t *testing.T) {
		mock := &pipeMockRuntime{}
		mock.CreatedContainerID = "test-container"
		mock.ExitCode = 0

		pr, pw := io.Pipe()
		defer pr.Close()
		var stdout bytes.Buffer

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		go func() {
			<-ctx.Done()
			_ = pr.Close()
		}()

		done := make(chan struct{})
		go func() {
			_ = ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "cat"}, func(o *rootOptions, cmd *cobra.Command) {
				o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
					return mock, nil
				}
				o.exitFunc = func(code int) {}
				cmd.SetIn(pr)
				cmd.SetOut(&stdout)
				cmd.SetErr(io.Discard)
			})
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
			t.Fatal("Test timed out")
		}

		assert.Empty(t, stdout.String())
	})
}

func TestUnit_Stdin_FlowExtended(t *testing.T) {
	t.Run("container echoes stdin with pipe-like reader", func(t *testing.T) {
		mock := &pipeMockRuntime{}
		mock.CreatedContainerID = "test-container"
		mock.ExitCode = 0

		var stdout bytes.Buffer
		stdinData := "hello extended\n"
		pr, pw := io.Pipe()
		defer pr.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		go func() {
			<-ctx.Done()
			_ = pr.Close()
		}()

		go func() {
			_, _ = pw.Write([]byte(stdinData))
			_ = pw.Close()
		}()

		err := ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "-i", "cat"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
				return mock, nil
			}
			o.exitFunc = func(code int) {}
			cmd.SetIn(pr)
			cmd.SetOut(&stdout)
		})
		require.NoError(t, err)
		assert.Equal(t, stdinData, stdout.String())
	})
}

func TestIntegration_Command_Stdin_Mocked(t *testing.T) {
	mock := &pipeMockRuntime{}
	mock.CreatedContainerID = "test-integration-container"
	mock.ExitCode = 0

	stdinData := "integration test data"
	pr, pw := io.Pipe()
	defer pr.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		<-ctx.Done()
		_ = pr.Close()
	}()

	go func() {
		_, _ = pw.Write([]byte(stdinData))
		_ = pw.Close()
	}()

	var outBuf bytes.Buffer
	var exitCode int

	err := ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "-i", "cat"}, withMockRuntime(mock, func(o *rootOptions, cmd *cobra.Command) {
		o.exitFunc = func(code int) {
			exitCode = code
		}
		cmd.SetIn(pr)
		cmd.SetOut(&outBuf)
	}))

	require.NoError(t, err)
	assert.Equal(t, 0, exitCode)
	assert.Equal(t, stdinData, outBuf.String())
}
