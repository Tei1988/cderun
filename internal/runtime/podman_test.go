package runtime

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Podman_NewRuntime(t *testing.T) {
	// This should succeed even without podman daemon as it just creates the client
	runtime, err := NewPodmanRuntime("/run/podman/podman.sock")
	require.NoError(t, err)
	assert.NotNil(t, runtime)
	assert.Equal(t, "podman", runtime.Name())
}
