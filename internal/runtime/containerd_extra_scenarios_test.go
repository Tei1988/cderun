//go:build linux

package runtime

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"cderun/internal/logging"
	"github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/cio"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockContainerdClient mocks containerdClient interface for testing.
type mockContainerdClient struct {
	containerdClient
	loadContainerFunc func(ctx context.Context, id string) (client.Container, error)
}

func (m *mockContainerdClient) LoadContainer(ctx context.Context, id string) (client.Container, error) {
	if m.loadContainerFunc != nil {
		return m.loadContainerFunc(ctx, id)
	}
	return nil, fmt.Errorf("not implemented")
}

// mockContainer mocks client.Container interface for testing.
type mockContainer struct {
	client.Container
	newTaskFunc func(ctx context.Context, creator cio.Creator, opts ...client.NewTaskOpts) (client.Task, error)
}

func (m *mockContainer) NewTask(ctx context.Context, creator cio.Creator, opts ...client.NewTaskOpts) (client.Task, error) {
	if m.newTaskFunc != nil {
		return m.newTaskFunc(ctx, creator, opts...)
	}
	return nil, fmt.Errorf("not implemented")
}

func TestUnit_Containerd_StartContainer_NullIO_Warning(t *testing.T) {
	t.Parallel()

	// Create custom logger to capture logs
	var logBuf bytes.Buffer
	logger := logging.NewLogger()
	logger.SetLevel(logging.WarnLevel)
	logger.SetOutput(&logBuf)

	// Create containerd adapter with mocked client
	mockCl := &mockContainerdClient{}
	rt := &ContainerdRuntime{
		client: mockCl,
		logger: logger,
		ioMap:  make(map[string]cio.Creator),
	}

	// Mock LoadContainer to return a mock container
	mockCont := &mockContainer{
		newTaskFunc: func(ctx context.Context, creator cio.Creator, opts ...client.NewTaskOpts) (client.Task, error) {
			// Return error so we don't need to mock task lifecycle
			return nil, fmt.Errorf("simulated new task error")
		},
	}
	mockCl.loadContainerFunc = func(ctx context.Context, id string) (client.Container, error) {
		return mockCont, nil
	}

	// Call StartContainer with no registered I/O creator in ioMap (triggering NullIO fallback)
	err := rt.StartContainer(context.Background(), "test-container-id")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "simulated new task error")

	// Verify warning was logged to buffer
	logOutput := logBuf.String()
	assert.Contains(t, logOutput, "StartContainer: no registered I/O creator found for container test-container-id, falling back to NullIO")
}

func TestUnit_Containerd_RemoveContainer_NoIoWaitDelete(t *testing.T) {
	t.Parallel()

	rt := &ContainerdRuntime{
		ioWait: map[string]chan error{"c1": make(chan error, 1)},
	}

	// It will panic on r.client.LoadContainer because client is nil.
	assert.Panics(t, func() {
		_ = rt.RemoveContainer(context.Background(), "c1")
	})

	// Assert that "c1" is still in rt.ioWait because RemoveContainer no longer deletes it.
	rt.mu.RLock()
	_, exists := rt.ioWait["c1"]
	rt.mu.RUnlock()
	assert.True(t, exists, "ioWait entry should not be deleted by RemoveContainer")
}
