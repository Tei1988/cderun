package runtime_test

import (
	"context"
	"errors"
	"testing"

	"cderun/internal/runtime"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Runtime_PruneAndInspect_MockRuntime(t *testing.T) {
	mock := runtime.NewMockRuntime()
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

func TestUnit_Runtime_PruneAndInspect_DockerRuntimeUnimplemented(t *testing.T) {
	dockerRt, err := runtime.NewDockerRuntime("/var/run/docker.sock")
	require.NoError(t, err)
	ctx := context.Background()

	// PruneContainers on DockerRuntime returns nil, nil for now
	pruned, err := dockerRt.PruneContainers(ctx)
	assert.NoError(t, err)
	assert.Nil(t, pruned)
}
