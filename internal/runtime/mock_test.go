package runtime

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/container"
)

func TestUnit_Mock_Methods(t *testing.T) {
	mock := NewMockRuntime()
	ctx := context.Background()

	// PullImage
	_ = mock.PullImage(ctx, "img", "always")
	assert.Equal(t, "img", mock.GetPulledImage())

	// CreateContainer
	config := &container.ContainerConfig{Image: "img"}
	mock.CreatedContainerID = "c1"
	id, _ := mock.CreateContainer(ctx, config)
	assert.Equal(t, "c1", id)
	assert.Equal(t, config, mock.GetCreatedConfig())

	// StartContainer
	_ = mock.StartContainer(ctx, "c1")
	assert.Equal(t, "c1", mock.GetStartedContainerID())

	// WaitContainer
	mock.ExitCode = 42
	code, _ := mock.WaitContainer(ctx, "c1")
	assert.Equal(t, 42, code)
	assert.Equal(t, "c1", mock.GetWaitedContainerID())

	// RemoveContainer
	_ = mock.RemoveContainer(ctx, "c1")
	assert.Equal(t, "c1", mock.GetRemovedContainerID())

	// AttachContainer
	_ = mock.AttachContainer(ctx, "c1", false, nil, nil, nil, nil)
	assert.Equal(t, "c1", mock.GetAttachedContainerID())

	// ResizeContainerTTY
	_ = mock.ResizeContainerTTY(ctx, "c1", 10, 20)
	assert.Equal(t, "c1", mock.ResizedContainerID)
	r, c := mock.GetTTYSize()
	assert.Equal(t, uint(10), r)
	assert.Equal(t, uint(20), c)

	// SignalContainer
	_ = mock.SignalContainer(ctx, "c1", "SIGINT")
	assert.Equal(t, "c1", mock.SignaledContainerID)
	assert.Equal(t, "SIGINT", mock.Signal)

	// Name
	assert.Equal(t, "mock", mock.Name())

	// ResetCreatedConfig
	mock.ResetCreatedConfig()
	assert.Nil(t, mock.GetCreatedConfig())
}

func TestUnit_Mock_New(t *testing.T) {
	mock := NewMockRuntime()
	assert.NotNil(t, mock)
}

func TestUnit_Mock_ConcurrentAccess(t *testing.T) {
	mock := NewMockRuntime()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const goroutines = 10
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := range goroutines {
		go func() {
			defer wg.Done()
			for j := range iterations {
				_ = mock.PullImage(ctx, "img", "always")
				_ = mock.GetPulledImage()
				_, _ = mock.CreateContainer(ctx, &container.ContainerConfig{})
				_ = mock.GetCreatedConfig()
				_ = mock.ResizeContainerTTY(ctx, "c", uint(i), uint(j))
				r, c := mock.GetTTYSize()
				assert.Less(t, r, uint(goroutines))
				assert.Less(t, c, uint(iterations))
			}
		}()
	}

	wg.Wait()
}

func TestUnit_Mock_WithLockedMock(t *testing.T) {
	mock := NewMockRuntime()
	mock.WithLockedMock(func(m *MockRuntime) {
		m.ExitCode = 123
	})
	assert.Equal(t, 123, mock.GetExitCode())
}

func TestUnit_Mock_WaitContainer_Advanced(t *testing.T) {
	ctx := context.Background()

	t.Run("WaitDelay with SIGINT", func(t *testing.T) {
		mock := NewMockRuntime()
		mock.WaitDelay = 200 * time.Millisecond

		go func() {
			time.Sleep(20 * time.Millisecond)
			_ = mock.SignalContainer(ctx, "c1", "SIGINT")
		}()

		start := time.Now()
		code, err := mock.WaitContainer(ctx, "c1")
		require.NoError(t, err)
		assert.Equal(t, 0, code)
		assert.Less(t, time.Since(start), 400*time.Millisecond)
	})

	t.Run("Context Cancelled during WaitDelay", func(t *testing.T) {
		mock := NewMockRuntime()
		mock.WaitDelay = 1 * time.Second

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(20 * time.Millisecond)
			cancel()
		}()

		_, err := mock.WaitContainer(ctx, "c1")
		require.ErrorIs(t, err, context.Canceled)
	})
}

func TestUnit_Mock_InspectContainer(t *testing.T) {
	mock := NewMockRuntime()
	mock.ExitCode = 7
	running, code, err := mock.InspectContainer(context.Background(), "c1")
	require.NoError(t, err)
	assert.False(t, running)
	assert.Equal(t, 7, code)
}

func TestUnit_Mock_GetExitCode(t *testing.T) {
	mock := NewMockRuntime()
	mock.ExitCode = 99
	assert.Equal(t, 99, mock.GetExitCode())
}

func TestUnit_Mock_AttachContainer_Func(t *testing.T) {
	mock := NewMockRuntime()
	mock.AttachFunc = func(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error {
		if ready != nil {
			close(ready)
		}
		return errors.New("custom attach error")
	}

	err := mock.AttachContainer(context.Background(), "c1", false, nil, nil, nil, nil)
	require.Error(t, err)
	assert.Equal(t, "custom attach error", err.Error())
}

func TestUnit_Mock_InspectContainer_Func(t *testing.T) {
	mock := NewMockRuntime()
	mock.InspectFunc = func(ctx context.Context, containerID string) (bool, int, error) {
		return true, 123, nil
	}

	running, code, err := mock.InspectContainer(context.Background(), "c1")
	require.NoError(t, err)
	assert.True(t, running)
	assert.Equal(t, 123, code)
}

func TestUnit_Mock_WaitContainer_TimerStop(t *testing.T) {
	mock := NewMockRuntime()
	mock.WaitDelay = 10 * time.Millisecond

	// This test exercises the !t.Stop() path in WaitContainer when the timer has already fired or stopped.
	// Since we can't easily race the timer and the Stop call precisely,
	// we use a very short delay and wait for it to finish.
	time.Sleep(20 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel context to enter the select block with context cancelled

	_, err := mock.WaitContainer(ctx, "c1")
	require.ErrorIs(t, err, context.Canceled)
}
