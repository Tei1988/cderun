package runtime

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContainerdRuntime_Name(t *testing.T) {
	// We can't easily test the full containerd runtime without a running daemon,
	// but we can at least test some basic things if we had a way to mock the client.
	// For now, let's just ensure the Name() method returns the correct value.
	rt := &ContainerdRuntime{}
	assert.Equal(t, "containerd", rt.Name())
}
