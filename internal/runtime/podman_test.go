package runtime

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Podman_New(t *testing.T) {
	// This should succeed even without podman daemon as it just creates the client
	runtime, err := NewPodmanRuntime("/run/podman/podman.sock")
	require.NoError(t, err)
	assert.NotNil(t, runtime)
	assert.Equal(t, "podman", runtime.Name())
}

func TestUnit_Podman_New_Error(t *testing.T) {
	// Empty socket string should cause an error in NewClientWithOpts or internal/runtime/docker.go
	// In PodmanRuntime, it wraps NewDockerRuntimeWithOptions which uses "unix://" + socket
	// "unix://" is invalid and should fail.
	_, err := NewPodmanRuntime("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create docker client")
}
