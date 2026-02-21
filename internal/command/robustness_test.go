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

	"cderun/internal/runtime"
)

type blockingMockRuntime struct {
	runtime.MockRuntime
	attachStarted chan struct{}
	blockAttach   chan struct{}
}

func (m *blockingMockRuntime) AttachContainer(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer) error {
	m.Lock()
	m.AttachedContainerID = containerID
	m.Unlock()
	close(m.attachStarted)
	select {
	case <-m.blockAttach:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestRobustness_Command_Root_SignalHandling(t *testing.T) {
	t.Run("unblocks hanging AttachContainer after WaitContainer finishes", func(t *testing.T) {
		mock := &blockingMockRuntime{
			attachStarted: make(chan struct{}),
			blockAttach:   make(chan struct{}),
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
		select {
		case <-mock.attachStarted:
			// attach started and is blocking
		case <-ctx.Done():
			t.Fatal("AttachContainer did not start in time or timeout")
		}

		// executeCommand should eventually finish because WaitContainer returns immediately
		// and AttachContainer will be canceled after 500ms grace period.
		select {
		case <-done:
			// Success
		case <-ctx.Done():
			t.Fatal("executeCommand did not finish even though WaitContainer should have completed")
		}
	})

	t.Run("handles double Ctrl+C to terminate", func(t *testing.T) {
		// Use a mock that blocks in WaitContainer to simulate long running process
		mock := &blockingMockRuntime{
			attachStarted: make(chan struct{}),
			blockAttach:   make(chan struct{}),
		}
		mock.CreatedContainerID = "test-container"

		// Custom WaitContainer that blocks
		waitStarted := make(chan struct{})
		blockWait := make(chan struct{})

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		done := make(chan struct{})
		go func() {
			_ = ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "sleep", "60"}, func(o *rootOptions, cmd *cobra.Command) {
				o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
					return &waitBlockingMock{
						blockingMockRuntime: mock,
						waitStarted:         waitStarted,
						blockWait:           blockWait,
					}, nil
				}
				o.exitFunc = func(code int) {}
			})
			close(done)
		}()

		// Wait for attach to start
		select {
		case <-mock.attachStarted:
			// attach started
		case <-ctx.Done():
			t.Fatal("AttachContainer did not start in time or timeout")
		}

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
	})

	t.Run("handles TTY resize via SIGWINCH", func(t *testing.T) {
		var mu sync.Mutex
		// Mock terminal size
		currentRows, currentCols := 24, 80

		// Use a mock that blocks in WaitContainer so we have time to send signal
		mock := &blockingMockRuntime{
			attachStarted: make(chan struct{}),
			blockAttach:   make(chan struct{}),
		}
		mock.CreatedContainerID = "test-container"

		waitStarted := make(chan struct{})
		blockWait := make(chan struct{})

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		done := make(chan struct{})
		go func() {
			_ = ExecuteContextWithOptions(ctx, []string{"cderun", "--image", "alpine", "--tty", "sleep", "60"}, func(o *rootOptions, cmd *cobra.Command) {
				o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
					return &waitBlockingMock{
						blockingMockRuntime: mock,
						waitStarted:         waitStarted,
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

		// Wait for start
		select {
		case <-waitStarted:
			// container started
		case <-ctx.Done():
			t.Fatal("Container did not start in time or timeout")
		}

		// Update terminal size for simulation
		mu.Lock()
		currentRows, currentCols = 30, 100
		mu.Unlock()

		// Send SIGWINCH
		_ = syscall.Kill(os.Getpid(), syscall.SIGWINCH)

		// Poll for resize with timeout
		expectedRows, expectedCols := 30, 100
		deadline := time.Now().Add(1 * time.Second)
		success := false
		for time.Now().Before(deadline) {
			actualRows, actualCols := mock.GetTTYSize()
			if actualRows == uint(expectedRows) && actualCols == uint(expectedCols) {
				success = true
				break
			}
			time.Sleep(20 * time.Millisecond)
		}

		if !success {
			actualRows, actualCols := mock.GetTTYSize()
			t.Errorf("expected resize to %dx%d, got %dx%d (timed out)", expectedRows, expectedCols, actualRows, actualCols)
		}

		// Cleanup
		close(blockWait)
		close(mock.blockAttach)
		<-done
	})

	t.Run("returns non-zero exit code correctly", func(t *testing.T) {
		mock := &blockingMockRuntime{
			attachStarted: make(chan struct{}),
			blockAttach:   make(chan struct{}),
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
		if err != nil {
			t.Fatalf("executeCommand failed: %v", err)
		}

		if capturedExitCode != 42 {
			t.Errorf("expected exit code 42, got %d", capturedExitCode)
		}
	})
}

type waitBlockingMock struct {
	*blockingMockRuntime
	waitStarted chan struct{}
	blockWait   chan struct{}
}

func (m *waitBlockingMock) WaitContainer(ctx context.Context, containerID string) (int, error) {
	m.Lock()
	m.WaitedContainerID = containerID
	exitCode := m.ExitCode
	m.Unlock()
	close(m.waitStarted)
	select {
	case <-m.blockWait:
		return exitCode, nil
	case <-ctx.Done():
		return exitCode, ctx.Err()
	}
}
