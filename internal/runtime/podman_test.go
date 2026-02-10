package runtime

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnit_Runtime_Podman_New(t *testing.T) {
	// This should succeed even without podman daemon as it just creates the client
	runtime, err := NewPodmanRuntime("/run/podman/podman.sock")
	assert.NoError(t, err)
	assert.NotNil(t, runtime)
	assert.Equal(t, "podman", runtime.Name())
}
