//go:build linux

package runtime

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Containerd_RemoveContainer_NoIoWaitDelete(t *testing.T) {
	t.Parallel()

	waitC := make(chan error, 1)
	rt := &ContainerdRuntime{
		ioWait: map[string]chan error{"c1": waitC},
	}

	// Because rt.client is nil, calling RemoveContainer should immediately return
	// an explicit initialization error without notifying or deleting from ioWait.
	err := rt.RemoveContainer(context.Background(), "c1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client is not initialized")

	// Verify that waitC was NOT written to (as it should bypass notification on early errors)
	select {
	case notifiedErr := <-waitC:
		t.Fatalf("unexpected notification in waitC: %v", notifiedErr)
	default:
		// Success: no notification was sent
	}

	// Verify that "c1" is still in the map (it is deleted in AttachContainer's defer, not RemoveContainer)
	rt.mu.RLock()
	_, exists := rt.ioWait["c1"]
	rt.mu.RUnlock()
	assert.True(t, exists, "ioWait entry should not be deleted by RemoveContainer directly")
}

func TestUnit_Containerd_AttachContainer_ClientNilError(t *testing.T) {
	t.Parallel()

	rt := &ContainerdRuntime{
		ioWait: map[string]chan error{},
	}

	err := rt.AttachContainer(context.Background(), "c1", false, nil, nil, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client is not initialized")
}
