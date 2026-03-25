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

func TestUnit_Stdin_PipedInputFlow(t *testing.T) {
	t.Parallel()
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

func TestUnit_Stdin_PipedFlowExtended(t *testing.T) {
	t.Parallel()
	t.Run("container echoes stdin with pipe-like reader", func(t *testing.T) {
		mock := &pipeMockRuntime{MockRuntime: *runtime.NewMockRuntime()}
		mock.CreatedContainerID = "test-container"
		mock.ExitCode = 0

		var stdout bytes.Buffer
		stdinData := "hello extended\n"
		pr, pw := io.Pipe()
		defer pr.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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
		assert.Equal(t, "test-container", mock.GetAttachedContainerID())
	})
}

func TestUnit_Stdin_MockedInput(t *testing.T) {
	t.Parallel()
	mock := &pipeMockRuntime{MockRuntime: *runtime.NewMockRuntime()}
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

func TestUnit_Stdin_AttachSynchronization(t *testing.T) {
	t.Parallel()
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

func TestUnit_Stdin_QuickExitWithPipedInput(t *testing.T) {
	t.Parallel()
	t.Run("piped stdin exits quickly after IO finished", func(t *testing.T) {
		mock := &pipeMockRuntime{MockRuntime: *runtime.NewMockRuntime()}
		mock.CreatedContainerID = "test-quick-exit"
		mock.ExitCode = 0
		mock.WaitDelay = 1 * time.Second

		var outBuf bytes.Buffer
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		start := time.Now()
		err := ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "-i", "cat", "--cderun-hang-timeout", "2s"}, func(o *rootOptions, cmd *cobra.Command) {
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
	t.Parallel()
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

func TestUnit_Stdin_NonInteractiveQuickExitBehavior(t *testing.T) {
	t.Parallel()
	t.Run("non-interactive exits quickly even if host is TTY", func(t *testing.T) {
		mock := &pipeMockRuntime{MockRuntime: *runtime.NewMockRuntime()}
		mock.CreatedContainerID = "test-non-interactive-quick"
		mock.ExitCode = 0
		mock.WaitDelay = 1 * time.Second

		var outBuf bytes.Buffer
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		start := time.Now()
		err := ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "cat", "--cderun-hang-timeout", "2s"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
				return mock, nil
			}
			o.exitFunc = func(code int) {}
			cmd.SetIn(fakeFdReader{strings.NewReader("some data")})
			cmd.SetOut(&outBuf)
			o.isTerminal = func(fd int) bool { return true }
		})
		duration := time.Since(start)

		require.NoError(t, err)
		assert.Less(t, duration, 4000*time.Millisecond, "Should exit quickly since it is non-interactive")
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

func TestUnit_Stdin_PipedContinuousLogOutput(t *testing.T) {
	t.Parallel()
	t.Run("piped stdin does not exit while pipe is open (like tail -f)", func(t *testing.T) {
		mock := &pipeMockRuntime{MockRuntime: *runtime.NewMockRuntime()}
		mock.CreatedContainerID = "test-piped-logs"
		mock.ExitCode = 0
		mock.WaitDelay = 10 * time.Second

		var outBuf bytes.Buffer
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		block := make(chan struct{})
		done := make(chan error, 1)
		go func() {
			done <- ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "-i", "cat", "--cderun-hang-timeout", "2s"}, func(o *rootOptions, cmd *cobra.Command) {
				o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
					return mock, nil
				}
				o.exitFunc = func(code int) {}
				cmd.SetIn(blockingReader{Reader: strings.NewReader(""), block: block})
				cmd.SetOut(&outBuf)
				o.isTerminal = func(fd int) bool { return false }
			})
		}()

		time.Sleep(200 * time.Millisecond)
		select {
		case err := <-done:
			t.Fatalf("Process exited prematurely: %v", err)
		case <-time.After(500 * time.Millisecond):
		}

		close(block)
		select {
		case err := <-done:
			require.NoError(t, err)
		case <-time.After(5 * time.Second):
			t.Fatal("Process did not exit after pipe closed")
		}
	})
}
