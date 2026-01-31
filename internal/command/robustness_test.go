package command

import (
	"cderun/internal/runtime"
	"context"
	"io"
	"os"
	"syscall"
	"testing"
	"time"

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

func TestExecuteRobustness(t *testing.T) {
	t.Run("unblocks hanging AttachContainer after WaitContainer finishes", func(t *testing.T) {
		oldFactory := runtimeFactory
		oldExit := exitFunc
		defer func() {
			runtimeFactory = oldFactory
			exitFunc = oldExit
		}()

		mock := &blockingMockRuntime{
			attachStarted: make(chan struct{}),
			blockAttach:   make(chan struct{}),
		}
		mock.CreatedContainerID = "test-container"
		mock.ExitCode = 0

		runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
			return mock, nil
		}
		exitFunc = func(code int) {}

		// Run execute in a goroutine because we want to check if it finishes
		done := make(chan struct{})
		go func() {
			_, _ = executeCommand("--image", "alpine", "ls")
			close(done)
		}()

		// Wait for attach to start
		select {
		case <-mock.attachStarted:
			// attach started and is blocking
		case <-time.After(2 * time.Second):
			t.Fatal("AttachContainer did not start in time")
		}

		// executeCommand should eventually finish because WaitContainer returns immediately
		// and AttachContainer will be canceled after 500ms grace period.
		select {
		case <-done:
			// Success
		case <-time.After(2 * time.Second):
			t.Fatal("executeCommand did not finish even though WaitContainer should have completed")
		}
	})

	t.Run("handles double Ctrl+C to terminate", func(t *testing.T) {
		oldFactory := runtimeFactory
		oldExit := exitFunc
		defer func() {
			runtimeFactory = oldFactory
			exitFunc = oldExit
		}()

		// Use a mock that blocks in WaitContainer to simulate long running process
		mock := &blockingMockRuntime{
			attachStarted: make(chan struct{}),
			blockAttach:   make(chan struct{}),
		}
		mock.CreatedContainerID = "test-container"

		// Custom WaitContainer that blocks
		waitStarted := make(chan struct{})
		blockWait := make(chan struct{})

		// We need a way to override WaitContainer.
		// Since blockingMockRuntime embeds MockRuntime, we can't easily override just one method
		// if we want to use the embedded one's fields, but here we can just define it.

		runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
			return &waitBlockingMock{
				blockingMockRuntime: mock,
				waitStarted:         waitStarted,
				blockWait:           blockWait,
			}, nil
		}

		exitFunc = func(code int) {}

		done := make(chan struct{})
		go func() {
			_, _ = executeCommand("--image", "alpine", "sleep", "60")
			close(done)
		}()

		// Wait for attach to start
		<-mock.attachStarted

		// Send first SIGINT
		syscall.Kill(os.Getpid(), syscall.SIGINT)

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
		syscall.Kill(os.Getpid(), syscall.SIGINT)

		// Now it should finish
		select {
		case <-done:
			// Success
		case <-time.After(2 * time.Second):
			t.Fatal("Process did not exit after second SIGINT")
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
