package command

import (
	"bytes"
	"context"
	"io"
	"strings"
	"sync"
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

func TestUnit_Stdin_PipedInput(t *testing.T) {
	t.Run("piped stdin reaches container when interactive is true", func(t *testing.T) {
		mock := &pipeMockRuntime{MockRuntime: *runtime.NewMockRuntime()}
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

		errCh := make(chan error, 1)
		go func() {
			errCh <- ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "-i", "cat"}, func(o *rootOptions, cmd *cobra.Command) {
				o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
					return mock, nil
				}
				o.exitFunc = func(code int) {}
				cmd.SetIn(pr)
				cmd.SetOut(&stdout)
				cmd.SetErr(io.Discard)
			})
		}()

		testData := "hello from pipe\n"
		go func() {
			_, _ = pw.Write([]byte(testData))
			_ = pw.Close()
		}()

		select {
		case err := <-errCh:
			require.NoError(t, err)
		case <-ctx.Done():
			t.Fatal("Test timed out")
		}
		assert.Equal(t, testData, stdout.String())
	})

	t.Run("piped stdin does NOT reach container when interactive is false", func(t *testing.T) {
		mock := &pipeMockRuntime{MockRuntime: *runtime.NewMockRuntime()}
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

		errCh := make(chan error, 1)
		go func() {
			errCh <- ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "cat"}, func(o *rootOptions, cmd *cobra.Command) {
				o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
					return mock, nil
				}
				o.exitFunc = func(code int) {}
				cmd.SetIn(pr)
				cmd.SetOut(&stdout)
				cmd.SetErr(io.Discard)
			})
		}()

		// Use a goroutine to write to avoid blocking if cat doesn't read
		go func() {
			_, _ = pw.Write([]byte("hello\n"))
			_ = pw.Close()
		}()

		select {
		case err := <-errCh:
			require.NoError(t, err)
		case <-ctx.Done():
			t.Fatal("Test timed out")
		}

		assert.Empty(t, stdout.String())
	})
}

type syncMockRuntime struct {
	runtime.MockRuntime
	mu         sync.Mutex
	counter    int
	startOrder int
	readOrder  int
}

func (m *syncMockRuntime) StartContainer(ctx context.Context, containerID string) error {
	m.mu.Lock()
	m.counter++
	m.startOrder = m.counter
	m.mu.Unlock()
	return m.MockRuntime.StartContainer(ctx, containerID)
}

func (m *syncMockRuntime) AttachContainer(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error {
	// Call embedded mock for record keeping
	_ = m.MockRuntime.AttachContainer(ctx, containerID, tty, stdin, stdout, stderr, ready)

	if stdin != nil && stdout != nil {
		// Perform read+write synchronously to avoid races
		p := make([]byte, 1024)
		n, err := stdin.Read(p)
		if n > 0 {
			m.mu.Lock()
			if m.readOrder == 0 {
				m.counter++
				m.readOrder = m.counter
			}
			m.mu.Unlock()
			_, _ = stdout.Write(p[:n])
			_, _ = io.Copy(stdout, stdin)
		} else if err != nil && err != io.EOF {
			return err
		}
	}
	return nil
}

func TestUnit_Stdin_OrderOfExecution(t *testing.T) {
	t.Run("stdin is not read until container starts", func(t *testing.T) {
		mock := &syncMockRuntime{MockRuntime: *runtime.NewMockRuntime()}
		mock.CreatedContainerID = "test-sync-container"
		mock.ExitCode = 0

		stdinData := "sync-data\n"
		var stdout bytes.Buffer
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err := ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "-i", "cat"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
				return mock, nil
			}
			o.exitFunc = func(code int) {}
			cmd.SetIn(strings.NewReader(stdinData))
			cmd.SetOut(&stdout)
		})

		require.NoError(t, err)
		assert.Equal(t, stdinData, stdout.String())
		assert.Equal(t, "test-sync-container", mock.GetAttachedContainerID())

		mock.mu.Lock()
		defer mock.mu.Unlock()
		assert.Positive(t, mock.startOrder, "StartContainer should have been called")
		assert.Positive(t, mock.readOrder, "Stdin should have been read")
		assert.Less(t, mock.startOrder, mock.readOrder, "StartContainer should be called before stdin is read")
	})
}

type fakeFdReader struct {
	io.Reader
}
func (f fakeFdReader) Fd() uintptr { return 0 }

func TestUnit_Stdin_QuickExit(t *testing.T) {
	t.Run("piped stdin exits quickly after IO finished", func(t *testing.T) {
		mock := &pipeMockRuntime{MockRuntime: *runtime.NewMockRuntime()}
		mock.CreatedContainerID = "test-quick-exit"
		mock.ExitCode = 0
		mock.WaitDelay = 1 * time.Second

		var outBuf bytes.Buffer
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		start := time.Now()
		err := ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "-i", "cat"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
				return mock, nil
			}
			o.exitFunc = func(code int) {}
			cmd.SetIn(strings.NewReader("quick data"))
			cmd.SetOut(&outBuf)
			// Explicitly set isTerminal to false to ensure non-TTY behavior
			o.isTerminal = func(fd int) bool { return false }
		})
		duration := time.Since(start)

		require.NoError(t, err)
		assert.Less(t, duration, 4000*time.Millisecond, "Should exit quickly due to piped stdin")
	})
}

func TestUnit_Stdin_TTYWaitBehavior(t *testing.T) {
	t.Run("TTY stdin waits for original timeout", func(t *testing.T) {
		mock := &pipeMockRuntime{MockRuntime: *runtime.NewMockRuntime()}
		mock.CreatedContainerID = "test-tty-wait"
		mock.ExitCode = 0
		mock.WaitDelay = 100 * time.Millisecond

		var outBuf bytes.Buffer
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		start := time.Now()
		err := ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "-i", "cat"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
				return mock, nil
			}
			o.exitFunc = func(code int) {}
			cmd.SetIn(fakeFdReader{strings.NewReader("tty data")})
			cmd.SetOut(&outBuf)
			o.isTerminal = func(fd int) bool { return true }
		})
		duration := time.Since(start)

		require.NoError(t, err)
		assert.GreaterOrEqual(t, duration, 100*time.Millisecond, "Should wait for container to exit naturally since host is TTY")
	})
}

// blockingReader blocks Read until the block channel is closed.
// It is used to simulate an open pipe (like tail -f).
type blockingReader struct {
	io.Reader
	block chan struct{}
}

func (b blockingReader) Read(p []byte) (int, error) {
	<-b.block
	// After unblocking, we return io.EOF to simulate the end of input.
	// The embedded io.Reader is intentionally not used to simplify simulation
	// of IO completion/attachment lifecycle.
	return 0, io.EOF
}

func TestUnit_Stdin_ContinuousInput(t *testing.T) {
	t.Run("piped stdin does not exit while pipe is open (like tail -f)", func(t *testing.T) {
		mock := &pipeMockRuntime{MockRuntime: *runtime.NewMockRuntime()}
		mock.CreatedContainerID = "test-piped-logs"
		mock.ExitCode = 0
		mock.WaitDelay = 10 * time.Second

		var outBuf bytes.Buffer
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		block := make(chan struct{})
		errCh := make(chan error, 1)
		go func() {
			errCh <- ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "-i", "cat"}, func(o *rootOptions, cmd *cobra.Command) {
				o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
					return mock, nil
				}
				o.exitFunc = func(code int) {}
				cmd.SetIn(blockingReader{Reader: strings.NewReader(""), block: block})
				cmd.SetOut(&outBuf)
				o.isTerminal = func(fd int) bool { return false }
			})
		}()

		// Replace brittle Sleep with state-polling
		assert.Eventually(t, func() bool {
			return mock.GetAttachedContainerID() != ""
		}, 5*time.Second, 10*time.Millisecond, "Container did not attach in time")

		// Verify it hasn't exited yet
		select {
		case err := <-errCh:
			t.Fatalf("Process exited prematurely: %v", err)
		default:
		}

		close(block)
		select {
		case err := <-errCh:
			require.NoError(t, err)
		case <-time.After(5 * time.Second):
			t.Fatal("Process did not exit after pipe closed")
		}
	})
}
