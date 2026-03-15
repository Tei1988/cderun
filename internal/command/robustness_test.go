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

func TestRobustness_Signal_UnblockAttach(t *testing.T) {
	t.Parallel()
	mock := &blockingMockRuntime{
		blockAttach: make(chan struct{}),
	}
	mock.CreatedContainerID = "test-container"
	mock.ExitCode = 0

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Run execute in a goroutine because we want to check if it finishes
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

	// Wait for attach to start
	assert.Eventually(t, func() bool {
		return mock.GetAttachedContainerID() != ""
	}, 5*time.Second, 10*time.Millisecond, "AttachContainer did not start in time")

	// executeCommand should eventually finish because WaitContainer returns immediately
	// and AttachContainer will be canceled after 500ms grace period.
	select {
	case <-done:
		// Success
	case <-ctx.Done():
		t.Fatal("executeCommand did not finish even though WaitContainer should have completed")
	}
}

func TestRobustness_Signal_DoubleCtrlC(t *testing.T) {
	// Signal tests often cannot be run in parallel if they rely on process-wide signal handling,
	// but here we are sending signals to our own PID. Still, they might interfere if multiple
	// tests do this simultaneously.
	// However, the application uses context and separate goroutines for signal handling.
	// To be safe, we don't use t.Parallel() for tests that kill the current process.

	// Use a mock that blocks in WaitContainer to simulate long running process
	mock := &blockingMockRuntime{
		blockAttach: make(chan struct{}),
	}
	mock.CreatedContainerID = "test-container"

	// Custom WaitContainer that blocks
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

	// Wait for attach to start
	assert.Eventually(t, func() bool {
		return mock.GetAttachedContainerID() != ""
	}, 5*time.Second, 10*time.Millisecond, "AttachContainer did not start in time")

	// Send first SIGINT
	_ = syscall.Kill(os.Getpid(), syscall.SIGINT)

	// Wait a bit
	time.Sleep(100 * time.Millisecond)

	// Ensure it hasn't finished yet
	select {
	case <-done:
		t.Fatal("Process exited after first SIGINT, expected it to stay running")
	default:
		// Still running, good
	}

	// Send second SIGINT
	_ = syscall.Kill(os.Getpid(), syscall.SIGINT)

	// Now it should finish
	select {
	case <-done:
		// Success
	case <-ctx.Done():
		t.Fatal("Process did not exit after second SIGINT or timeout")
	}
}

func TestRobustness_Signal_TTYResize(t *testing.T) {
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

	// Wait for wait to start
	assert.Eventually(t, func() bool {
		return mock.GetWaitedContainerID() != ""
	}, 5*time.Second, 10*time.Millisecond, "Container did not start wait in time")

	// Update terminal size for simulation
	mu.Lock()
	currentRows, currentCols = 30, 100
	mu.Unlock()

	// Send SIGWINCH
	_ = syscall.Kill(os.Getpid(), syscall.SIGWINCH)

	// Poll for resize with timeout
	expectedRows, expectedCols := 30, 100
	assert.Eventually(t, func() bool {
		actualRows, actualCols := mock.GetTTYSize()
		return actualRows == uint(expectedRows) && actualCols == uint(expectedCols)
	}, 1*time.Second, 20*time.Millisecond, "expected resize to %dx%d", expectedRows, expectedCols)

	// Cleanup
	close(blockWait)
	close(mock.blockAttach)
	<-done
}

func TestRobustness_Runtime_ExitCode(t *testing.T) {
	t.Parallel()
	mock := &blockingMockRuntime{
		blockAttach: make(chan struct{}),
	}
	mock.CreatedContainerID = "test-container"
	mock.ExitCode = 42 // Non-zero exit code

	var capturedExitCode int
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// executeCommand calls Execute -> RunE which calls exitFunc(exitCode)
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
}

func TestRobustness_HangRecovery_AutoTerminationNonTTY(t *testing.T) {
	t.Parallel()
	mock := &hangMockRuntime{MockRuntime: *runtime.NewMockRuntime(),
		waitStarted: make(chan struct{}),
		killed:      make(chan struct{}),
	}
	mock.CreatedContainerID = "hang-container"

	// Longer timeout for the whole test
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	start := time.Now()
	done := make(chan error, 1)
	go func() {
		done <- ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "-i", "cat"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
				return mock, nil
			}
			o.exitFunc = func(code int) {}
			o.isTerminal = func(fd int) bool { return false }
		})
	}()

	select {
	case err := <-done:
		elapsed := time.Since(start)
		t.Logf("Execution finished in %v", elapsed)
		// It should finish after effectiveHangTimeout (2s) because it is non-TTY
		require.NoError(t, err) // We handle the kill, so it should return nil error from Execute (exit code handled by exitFunc)
		if elapsed < 2*time.Second {
			t.Errorf("Execution took too short (%v), expected at least effectiveHangTimeout", elapsed)
		}
		if elapsed > 4*time.Second {
			t.Errorf("Execution took too long (%v), expected short timeout for non-terminal", elapsed)
		}
	case <-time.After(11 * time.Second):
		t.Fatal("Test timed out completely")
	}
}

func TestRobustness_HangRecovery_AutoTerminationTTYNoKill(t *testing.T) {
	t.Parallel()
	mock := &hangMockRuntime{MockRuntime: *runtime.NewMockRuntime(),
		waitStarted: make(chan struct{}),
		killed:      make(chan struct{}),
	}
	mock.CreatedContainerID = "tty-container"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	done := make(chan error, 1)
	go func() {
		done <- ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "-i", "-t", "cat"}, func(o *rootOptions, cmd *cobra.Command) {
			o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
				return mock, nil
			}
			o.exitFunc = func(code int) {}
			o.isTerminal = func(fd int) bool { return true }
		})
	}()

	select {
	case err := <-done:
		elapsed := time.Since(start)
		t.Logf("Execution finished in %v", elapsed)
		// In TTY mode, it should wait for the context timeout (5s) because no auto-kill
		require.Error(t, err)
		if elapsed < 4*time.Second {
			t.Errorf("Execution finished too early (%v), expected to wait for context", elapsed)
		}
	case <-time.After(6 * time.Second):
		t.Fatal("Test timed out")
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

type hangMockRuntime struct {
	runtime.MockRuntime
	waitStarted chan struct{}
	killed      chan struct{}
	killedOnce  sync.Once
	waitStartedOnce sync.Once
}

func (m *hangMockRuntime) WaitContainer(ctx context.Context, containerID string) (int, error) {
	_, _ = m.MockRuntime.WaitContainer(ctx, containerID)
	m.waitStartedOnce.Do(func() {
		close(m.waitStarted)
	})

	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case <-m.killed:
		return 137, nil // SIGKILL exit code
	}
}


func (m *hangMockRuntime) SignalContainer(ctx context.Context, containerID string, sig string) error {
	_ = m.MockRuntime.SignalContainer(ctx, containerID, sig)
	if sig == "SIGKILL" {
		m.killedOnce.Do(func() {
			close(m.killed)
		})
	}
	return nil
}

func (m *hangMockRuntime) AttachContainer(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error {
	_ = m.MockRuntime.AttachContainer(ctx, containerID, tty, stdin, stdout, stderr, ready)
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
