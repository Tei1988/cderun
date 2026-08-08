//go:build linux

package runtime

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnit_Containerd_RemoveContainer_NoIoWaitDelete(t *testing.T) {
	t.Parallel()

	waitC := make(chan error, 1)
	rt := &ContainerdRuntime{
		ioWait: map[string]chan error{"c1": waitC},
	}

	// We expect a panic because rt.client is nil, but before the panic,
	// RemoveContainer should notify the ioWait channel with an error,
	// and NOT delete it directly.
	defer func() {
		_ = recover()
		// Verify that waitC received the "removed" notification error
		select {
		case err := <-waitC:
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "was removed")
		default:
			t.Error("expected waitC to receive container removal notification")
		}

		// Verify that "c1" is still in the map (it is deleted in AttachContainer's defer, not RemoveContainer)
		rt.mu.RLock()
		_, exists := rt.ioWait["c1"]
		rt.mu.RUnlock()
		assert.True(t, exists, "ioWait entry should not be deleted by RemoveContainer directly")
	}()

	_ = rt.RemoveContainer(context.Background(), "c1")
}

func TestUnit_Containerd_AttachContainer_ClientNilError(t *testing.T) {
	t.Parallel()

	rt := &ContainerdRuntime{
		ioWait: map[string]chan error{},
	}

	err := rt.AttachContainer(context.Background(), "c1", false, nil, nil, nil, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "client is not initialized")
}
