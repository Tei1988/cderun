package command

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"cderun/internal/runtime"
)

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

func TestIntegration_Root_AutoTermination_NonTTY(t *testing.T) {
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
		// It should finish after effectiveHangTimeout (100ms) because it is non-TTY
		require.NoError(t, err) // We handle the kill, so it should return nil error from Execute (exit code handled by exitFunc)
		if elapsed < 100*time.Millisecond {
			t.Errorf("Execution took too short (%v), expected at least effectiveHangTimeout", elapsed)
		}
		if elapsed > 1*time.Second {
			t.Errorf("Execution took too long (%v), expected short timeout for non-terminal", elapsed)
		}
	case <-time.After(11 * time.Second):
		t.Fatal("Test timed out completely")
	}
}

func TestIntegration_Root_AutoTermination_TTY_NoKill(t *testing.T) {
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
