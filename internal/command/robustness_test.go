package command

import (
	"cderun/internal/runtime"
	"context"
	"io"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type blockingMockRuntime struct {
	runtime.MockRuntime
	attachStarted chan struct{}
	blockAttach   chan struct{}
}

func (m *blockingMockRuntime) AttachContainer(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer) error {
	m.AttachedContainerID = containerID
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

		done := make(chan struct{})
		go func() {
			_, _ = executeCommandWithOptions(ctx, nil, func(o *rootOptions) {
				o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
					return mock, nil
				}
				o.exitFunc = func(code int) {}
			}, "--image", "alpine", "ls")
			close(done)
		}()

		select {
		case <-mock.attachStarted:
		case <-ctx.Done():
			t.Fatal("AttachContainer did not start in time or timeout")
		}

		select {
		case <-done:
		case <-ctx.Done():
			t.Fatal("executeCommand did not finish even though WaitContainer should have completed")
		}
	})

	t.Run("handles double Ctrl+C to terminate", func(t *testing.T) {
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
			_, _ = executeCommandWithOptions(ctx, nil, func(o *rootOptions) {
				o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
					return &waitBlockingMock{
						blockingMockRuntime: mock,
						waitStarted:         waitStarted,
						blockWait:           blockWait,
					}, nil
				}
				o.exitFunc = func(code int) {}
			}, "--image", "alpine", "sleep", "60")
			close(done)
		}()

		select {
		case <-mock.attachStarted:
		case <-ctx.Done():
			t.Fatal("AttachContainer did not start in time or timeout")
		}

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

	t.Run("handles TTY resize via SIGWINCH", func(t *testing.T) {
		var mu sync.Mutex
		currentRows, currentCols := 24, 80

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
			_, _ = executeCommandWithOptions(ctx, nil, func(o *rootOptions) {
				o.isTerminal = func(fd int) bool { return true }
				o.termGetSize = func(fd int) (int, int, error) {
					mu.Lock()
					defer mu.Unlock()
					return currentCols, currentRows, nil
				}
				o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
					return &waitBlockingMock{
						blockingMockRuntime: mock,
						waitStarted:         waitStarted,
						blockWait:           blockWait,
					}, nil
				}
				o.exitFunc = func(code int) {}
			}, "--image", "alpine", "--tty", "sleep", "60")
			close(done)
		}()

		select {
		case <-waitStarted:
		case <-ctx.Done():
			t.Fatal("Container did not start in time or timeout")
		}

		mu.Lock()
		currentRows, currentCols = 30, 100
		mu.Unlock()

		_ = syscall.Kill(os.Getpid(), syscall.SIGWINCH)
		time.Sleep(200 * time.Millisecond)

		mu.Lock()
		expectedRows, expectedCols := currentRows, currentCols
		mu.Unlock()

		actualRows, actualCols := mock.GetTTYSize()
		if actualRows != uint(expectedRows) || actualCols != uint(expectedCols) {
			t.Errorf("expected resize to %dx%d, got %dx%d", expectedRows, expectedCols, actualRows, actualCols)
		}

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
		mock.ExitCode = 42

		var capturedExitCode int

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_, err := executeCommandWithOptions(ctx, nil, func(o *rootOptions) {
			o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
				return mock, nil
			}
			o.exitFunc = func(code int) {
				capturedExitCode = code
			}
		}, "--image", "alpine", "false")
		require.NoError(t, err)

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
	m.WaitedContainerID = containerID
	close(m.waitStarted)
	select {
	case <-m.blockWait:
		return 0, nil
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}
