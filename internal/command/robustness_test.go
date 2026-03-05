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

func TestRobustness_Root_SignalHanging(t *testing.T) {
	t.Run("unblocks hanging AttachContainer after WaitContainer finishes", func(t *testing.T) {
		mock := &blockingMockRuntime{
			blockAttach: make(chan struct{}),
		}
		mock.CreatedContainerID = "test-container"
		mock.ExitCode = 0

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Run execute in a goroutine because we want to check if it finishes
		errCh := make(chan error, 1)
		go func() {
			errCh <- ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "ls"}, func(o *rootOptions, cmd *cobra.Command) {
				o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
					return mock, nil
				}
				o.exitFunc = func(code int) {}
			})
		}()

		// Wait for attach to start
		assert.Eventually(t, func() bool {
			return mock.GetAttachedContainerID() != ""
		}, 5*time.Second, 10*time.Millisecond, "AttachContainer did not start in time")

		// executeCommand should eventually finish because WaitContainer returns immediately
		// and AttachContainer will be canceled after grace period.
		select {
		case err := <-errCh:
			require.NoError(t, err)
		case <-ctx.Done():
			t.Fatal("executeCommand did not finish even though WaitContainer should have completed")
		}
	})
}

func TestRobustness_Root_DoubleSIGINT(t *testing.T) {
	// Use a mock that blocks in WaitContainer to simulate long running process
	mock := &blockingMockRuntime{
		blockAttach: make(chan struct{}),
	}
	mock.CreatedContainerID = "test-container"

	// Custom WaitContainer that blocks
	blockWait := make(chan struct{})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	errCh := make(chan error, 1)
	go func() {
		errCh <- ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "sleep", "60"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
				return &waitBlockingMock{
					blockingMockRuntime: mock,
					blockWait:           blockWait,
				}, nil
			}
			o.exitFunc = func(code int) {}
			o.setupSignals = func(c chan os.Signal) {
				go func() {
					for s := range sigChan {
						c <- s
					}
				}()
			}
		})
	}()

	// Wait for attach to start
	assert.Eventually(t, func() bool {
		return mock.GetAttachedContainerID() != ""
	}, 5*time.Second, 10*time.Millisecond, "AttachContainer did not start in time")

	// Send first SIGINT
	sigChan <- syscall.SIGINT

	// Wait a bit
	time.Sleep(100 * time.Millisecond)

	// Ensure it hasn't finished yet
	select {
	case err := <-errCh:
		t.Fatalf("Process exited after first SIGINT with error %v, expected it to stay running", err)
	default:
		// Still running, good
	}

	// Send second SIGINT
	close(blockWait)
	sigChan <- syscall.SIGINT

	// Now it should finish
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal("Process did not exit after second SIGINT or timeout")
	}
}

func TestRobustness_Root_TTYResize(t *testing.T) {
	var mu sync.Mutex
	// Mock terminal size
	currentRows, currentCols := 24, 80

	// Use a mock that blocks in WaitContainer so we have time to send signal
	mock := &blockingMockRuntime{
		blockAttach: make(chan struct{}),
	}
	mock.CreatedContainerID = "test-container"

	blockWait := make(chan struct{})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resizeChan := make(chan os.Signal, 1)
	errCh := make(chan error, 1)
	go func() {
		errCh <- ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "--tty", "sleep", "60"}, func(o *rootOptions, cmd *cobra.Command) {
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
			o.setupResizeSignal = func(c chan os.Signal) {
				go func() {
					for s := range resizeChan {
						c <- s
					}
				}()
			}
		})
	}()

	// Wait for wait to start
	assert.Eventually(t, func() bool {
		return mock.GetWaitedContainerID() != ""
	}, 5*time.Second, 10*time.Millisecond, "Container did not start wait in time")

	// Update terminal size for simulation
	mu.Lock()
	currentRows, currentCols = 30, 100
	mu.Unlock()

	// Send SIGWINCH
	resizeChan <- syscall.SIGWINCH

	// Poll for resize with timeout
	expectedRows, expectedCols := 30, 100
	assert.Eventually(t, func() bool {
		actualRows, actualCols := mock.GetTTYSize()
		return actualRows == uint(expectedRows) && actualCols == uint(expectedCols)
	}, 1*time.Second, 20*time.Millisecond, "expected resize to %dx%d", expectedRows, expectedCols)

	// Cleanup
	close(blockWait)
	close(mock.blockAttach)
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for TTYResize test completion")
	}
}

func TestRobustness_Root_ExitCodeCapture(t *testing.T) {
	mock := &blockingMockRuntime{
		blockAttach: make(chan struct{}),
	}
	mock.CreatedContainerID = "test-container"
	mock.ExitCode = 42 // Non-zero exit code

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
	if err != nil {
		t.Fatalf("executeCommand failed: %v", err)
	}

	if capturedExitCode != 42 {
		t.Errorf("expected exit code 42, got %d", capturedExitCode)
	}
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
