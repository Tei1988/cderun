package runtime

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestUnit_Containerd_NotifyTaskReady(t *testing.T) {
	t.Parallel()

	rt := &ContainerdRuntime{
		taskReady: make(map[string]chan struct{}),
	}

	containerID := "test-container-id"
	readyC := make(chan struct{})
	rt.taskReady[containerID] = readyC

	// 1. Verify notifyTaskReady closes the channel and deletes it from the map
	rt.notifyTaskReady(containerID)

	select {
	case <-readyC:
		// Success, channel closed
	default:
		t.Fatal("readyC was not closed by notifyTaskReady")
	}

	rt.mu.RLock()
	_, ok := rt.taskReady[containerID]
	rt.mu.RUnlock()
	assert.False(t, ok, "containerID should be deleted from taskReady map")

	// 2. Verify subsequent notifyTaskReady calls are safe and do not panic
	assert.NotPanics(t, func() {
		rt.notifyTaskReady(containerID)
	})
}

func TestUnit_Containerd_TaskReady_Concurrent(t *testing.T) {
	t.Parallel()

	rt := &ContainerdRuntime{
		taskReady: make(map[string]chan struct{}),
	}

	containerID := "test-container-id-concurrent"
	readyC := make(chan struct{})
	rt.taskReady[containerID] = readyC

	var wg sync.WaitGroup
	// Spin up multiple goroutines waiting for taskReady
	for range 10 {
		wg.Add(1)
		//nolint:modernize
		go func() {
			defer wg.Done()
			<-readyC
		}()
	}

	// Trigger notification
	rt.notifyTaskReady(containerID)

	// Wait with timeout to verify no hangs
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for concurrent waiters to unblock")
	}
}
