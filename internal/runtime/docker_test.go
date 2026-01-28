package runtime

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewDockerRuntime(t *testing.T) {
	// This should succeed even without docker daemon as it just creates the client
	runtime, err := NewDockerRuntime("/var/run/docker.sock")
	assert.NoError(t, err)
	assert.NotNil(t, runtime)
	assert.Equal(t, "docker", runtime.Name())
}
