package runtime

import (
	"context"
	"errors"
	"testing"

	"cderun/internal/logging"

	"github.com/docker/docker/api/types"
	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockDockerClientForPrune struct {
	dockerClient
	containers []types.Container
	listErr    error
	removed    []string
	removeErr  error
}

func (m *mockDockerClientForPrune) Close() error {
	return nil
}

func (m *mockDockerClientForPrune) ContainerList(ctx context.Context, options dockercontainer.ListOptions) ([]types.Container, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.containers, nil
}

func (m *mockDockerClientForPrune) ContainerRemove(ctx context.Context, containerID string, options dockercontainer.RemoveOptions) error {
	if m.removeErr != nil {
		return m.removeErr
	}
	m.removed = append(m.removed, containerID)
	return nil
}

func TestUnit_Runtime_PruneAndInspect_MockRuntime(t *testing.T) {
	mock := NewMockRuntime()
	ctx := context.Background()

	// Test default PruneContainers
	pruned, err := mock.PruneContainers(ctx)
	assert.NoError(t, err)
	assert.Nil(t, pruned)

	// Test PruneContainers with configured pruned containers and error
	mock.PrunedContainers = []string{"c1", "c2"}
	mock.PruneErr = errors.New("prune failure")

	pruned, err = mock.PruneContainers(ctx)
	assert.Error(t, err)
	assert.Equal(t, []string{"c1", "c2"}, pruned)

	// Test PruneContainers with custom hook
	mock.PruneFunc = func(ctx context.Context) ([]string, error) {
		return []string{"c3"}, nil
	}
	pruned, err = mock.PruneContainers(ctx)
	assert.NoError(t, err)
	assert.Equal(t, []string{"c3"}, pruned)

	// Test default InspectContainer
	mock.ExitCode = 42
	exists, exitCode, err := mock.InspectContainer(ctx, "c123")
	assert.NoError(t, err)
	assert.False(t, exists)
	assert.Equal(t, 42, exitCode)

	// Test InspectContainer with custom hook
	mock.InspectFunc = func(ctx context.Context, containerID string) (bool, int, error) {
		if containerID == "active-c" {
			return true, 0, nil
		}
		return false, 127, errors.New("container not found")
	}

	exists, exitCode, err = mock.InspectContainer(ctx, "active-c")
	assert.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, 0, exitCode)

	exists, exitCode, err = mock.InspectContainer(ctx, "missing-c")
	assert.Error(t, err)
	assert.False(t, exists)
	assert.Equal(t, 127, exitCode)
}

func TestUnit_Runtime_PruneAndInspect_DockerRuntimePrune(t *testing.T) {
	ctx := context.Background()
	logger := logging.GetGlobalLogger()

	// Case 1: Empty container list
	mock1 := &mockDockerClientForPrune{containers: nil}
	rt1 := &DockerRuntime{client: mock1, logger: logger}
	pruned, err := rt1.PruneContainers(ctx)
	assert.NoError(t, err)
	assert.Nil(t, pruned)

	// Case 2: Active containers skipped, stopped containers removed
	mock2 := &mockDockerClientForPrune{
		containers: []types.Container{
			{ID: "c-active-1", State: "running"},
			{ID: "c-active-2", State: "restarting"},
			{ID: "c-stopped-1", State: "exited"},
		},
	}
	rt2 := &DockerRuntime{client: mock2, logger: logger}
	pruned, err = rt2.PruneContainers(ctx)
	assert.NoError(t, err)
	assert.Equal(t, []string{"c-stopped-1"}, pruned)
	assert.Equal(t, []string{"c-stopped-1"}, mock2.removed)

	// Case 3: List error
	mock3 := &mockDockerClientForPrune{listErr: errors.New("list failed")}
	rt3 := &DockerRuntime{client: mock3, logger: logger}
	_, err = rt3.PruneContainers(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to list containers for prune")
}
