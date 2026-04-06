package command

import (
	"context"
	"io"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/runtime"
)

type blockingMockRuntime struct {
	runtime.MockRuntime
	blockAttach chan struct{}
}

func (m *blockingMockRuntime) AttachContainer(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error {
	m.WithLockedMock(func(base *runtime.MockRuntime) {
		base.AttachedContainerID = containerID
	})
	if ready != nil {
		close(ready)
	}
	select {
	case <-m.blockAttach:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestRobustness_SignalHandling_ContainerInteractions(t *testing.T) {
	t.Run("unblocks hanging AttachContainer after WaitContainer finishes", func(t *testing.T) {
		mock := &blockingMockRuntime{
			blockAttach: make(chan struct{}),
		}
		mock.CreatedContainerID = "test-container"
		mock.ExitCode = 0

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		done := make(chan struct{})
		go func() {
			_ = ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "ls"}, func(o *rootOptions, cmd *cobra.Command) {
				o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
					return mock, nil
				}
				o.exitFunc = func(code int) {}
			})
			close(done)
		}()

		assert.Eventually(t, func() bool {
			return mock.GetAttachedContainerID() != ""
		}, 5*time.Second, 10*time.Millisecond, "AttachContainer did not start in time")

		select {
		case <-done:
		case <-ctx.Done():
			t.Fatal("executeCommand did not finish even though WaitContainer should have completed")
		}
	})

	t.Run("DoubleSIGINT", func(t *testing.T) {
		mock := &blockingMockRuntime{
			blockAttach: make(chan struct{}),
		}
		mock.CreatedContainerID = "test-container"
		blockWait := make(chan struct{})

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		done := make(chan struct{})
		go func() {
			_ = ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "sleep", "60"}, func(o *rootOptions, cmd *cobra.Command) {
				o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
					return &waitBlockingMock{
						blockingMockRuntime: mock,
						blockWait:           blockWait,
					}, nil
				}
				o.exitFunc = func(code int) {}
			})
			close(done)
		}()

		assert.Eventually(t, func() bool {
			return mock.GetAttachedContainerID() != ""
		}, 5*time.Second, 10*time.Millisecond, "AttachContainer did not start in time")

		_ = syscall.Kill(os.Getpid(), syscall.SIGINT)
		time.Sleep(100 * time.Millisecond)

		select {
		case <-done:
			t.Fatal("Process exited after first SIGINT, expected it to stay running")
		default:
		}

		_ = syscall.Kill(os.Getpid(), syscall.SIGINT)

		select {
		case <-done:
		case <-ctx.Done():
			t.Fatal("Process did not exit after second SIGINT or timeout")
		}
	})

	t.Run("TTYResize", func(t *testing.T) {
		var mu sync.Mutex
		currentRows, currentCols := 24, 80

		mock := &blockingMockRuntime{
			blockAttach: make(chan struct{}),
		}
		mock.CreatedContainerID = "test-container"
		blockWait := make(chan struct{})

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		done := make(chan struct{})
		go func() {
			_ = ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "--tty", "sleep", "60"}, func(o *rootOptions, cmd *cobra.Command) {
				o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
					return &waitBlockingMock{
						blockingMockRuntime: mock,
						blockWait:           blockWait,
					}, nil
				}
				o.exitFunc = func(code int) {}
				o.isTerminal = func(fd int) bool { return true }
				o.termGetSize = func(fd int) (int, int, error) {
					mu.Lock()
					defer mu.Unlock()
					return currentCols, currentRows, nil
				}
			})
			close(done)
		}()

		assert.Eventually(t, func() bool {
			return mock.GetWaitedContainerID() != ""
		}, 5*time.Second, 10*time.Millisecond, "Container did not start wait in time")

		mu.Lock()
		currentRows, currentCols = 30, 100
		mu.Unlock()

		_ = syscall.Kill(os.Getpid(), syscall.SIGWINCH)

		assert.Eventually(t, func() bool {
			actualRows, actualCols := mock.GetTTYSize()
			return actualRows == uint(30) && actualCols == uint(100)
		}, 1*time.Second, 20*time.Millisecond)

		close(blockWait)
		close(mock.blockAttach)
		<-done
	})

	t.Run("ExitCode", func(t *testing.T) {
		mock := &blockingMockRuntime{
			blockAttach: make(chan struct{}),
		}
		mock.CreatedContainerID = "test-container"
		mock.ExitCode = 42

		var capturedExitCode int
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err := ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "false"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
				return mock, nil
			}
			o.exitFunc = func(code int) {
				capturedExitCode = code
			}
		})
		require.NoError(t, err)
		assert.Equal(t, 42, capturedExitCode)
	})
}

type waitBlockingMock struct {
	*blockingMockRuntime
	blockWait chan struct{}
}

func (m *waitBlockingMock) WaitContainer(ctx context.Context, containerID string) (int, error) {
	var exitCode int
	m.WithLockedMock(func(base *runtime.MockRuntime) {
		base.WaitedContainerID = containerID
		exitCode = base.ExitCode
	})
	select {
	case <-m.blockWait:
		return exitCode, nil
	case <-ctx.Done():
		return exitCode, ctx.Err()
	}
}

type hangMockRuntime struct {
	runtime.MockRuntime
	waitStarted     chan struct{}
	killed          chan struct{}
	killedOnce      sync.Once
	waitStartedOnce sync.Once
}

func (m *hangMockRuntime) WaitContainer(ctx context.Context, containerID string) (int, error) {
	// SA5007: Use m.MockRuntime.WaitContainer to avoid infinite recursion
	// and capture configured error scenarios.
	_, err := m.MockRuntime.WaitContainer(ctx, containerID)
	if err != nil {
		return 0, err
	}
	m.waitStartedOnce.Do(func() {
		if m.waitStarted != nil {
			close(m.waitStarted)
		}
	})

	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case <-m.killed:
		return 137, nil // SIGKILL exit code
	}
}

func (m *hangMockRuntime) SignalContainer(ctx context.Context, containerID string, sig string) error {
	// SA5007: Use m.MockRuntime.SignalContainer to avoid infinite recursion
	// and capture configured error scenarios.
	err := m.MockRuntime.SignalContainer(ctx, containerID, sig)
	if err != nil {
		return err
	}
	if sig == "SIGKILL" {
		m.killedOnce.Do(func() {
			if m.killed != nil {
				close(m.killed)
			}
		})
	}
	return nil
}

func (m *hangMockRuntime) AttachContainer(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error {
	// SA5007: Use m.MockRuntime.AttachContainer to avoid infinite recursion
	// and capture configured error scenarios.
	err := m.MockRuntime.AttachContainer(ctx, containerID, tty, stdin, stdout, stderr, ready)
	if err != nil {
		return err
	}
	// Return immediately to simulate IO finished
	return nil
}

func (m *hangMockRuntime) InspectContainer(ctx context.Context, containerID string) (bool, int, error) {
	select {
	case <-m.killed:
		return false, 137, nil
	default:
		return true, 0, nil
	}
}

func TestRobustness_HangRecovery_AutoTerminationNonTTY(t *testing.T) {
	mock := &hangMockRuntime{MockRuntime: *runtime.NewMockRuntime(),
		waitStarted: make(chan struct{}),
		killed:      make(chan struct{}),
	}
	mock.CreatedContainerID = "hang-container"

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var capturedExitCode int
	done := make(chan error, 1)
	go func() {
		done <- ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "-i", "--cderun-hang-timeout=100ms", "cat"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
				return mock, nil
			}
			o.exitFunc = func(code int) {
				capturedExitCode = code
			}
			o.isTerminal = func(fd int) bool { return false }
		})
	}()

	select {
	case err := <-done:
		require.NoError(t, err)
		assert.Equal(t, 137, capturedExitCode, "Should capture SIGKILL exit code")
		select {
		case <-mock.killed:
			// Success
		default:
			t.Error("SIGKILL was not sent")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Test timed out")
	}
}

func TestRobustness_HangRecovery_AutoTerminationTTYNoKill(t *testing.T) {
	mock := &hangMockRuntime{MockRuntime: *runtime.NewMockRuntime(),
		waitStarted: make(chan struct{}),
		killed:      make(chan struct{}),
	}
	mock.CreatedContainerID = "tty-container"

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	var capturedExitCode int
	done := make(chan error, 1)
	go func() {
		done <- ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "-i", "-t", "cat"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
				return mock, nil
			}
			o.exitFunc = func(code int) {
				capturedExitCode = code
			}
			o.isTerminal = func(fd int) bool { return true }
		})
	}()

	select {
	case err := <-done:
		require.Error(t, err)
		require.ErrorIs(t, err, context.DeadlineExceeded)
		// capturedExitCode exists to satisfy the exitFunc signature in this TTY timeout test.
		_ = capturedExitCode
		select {
		case <-mock.killed:
			t.Error("SIGKILL should NOT be sent")
		default:
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Test timed out")
	}
}
