package runtime

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Containerd_New(t *testing.T) {
	// containerd.New just validates the address format if it's not a valid dialer
	// "unix:///run/containerd/containerd.sock" is a valid address format.
	// Note: It might try to dial immediately, but we can't easily mock it without a real socket.
	// For unit tests, we'll just check if the constructor exists and handles basic errors.
	rt, err := NewContainerdRuntime("/run/containerd/containerd.sock")
	if err != nil {
		// If it fails because containerd is not running, we skip or just assert error.
		t.Skip("containerd not running or socket inaccessible")
		return
	}
	defer rt.Close()
	assert.Equal(t, "containerd", rt.Name())
}

func TestUnit_Containerd_ParseSignal(t *testing.T) {
	s, err := ParseSignal("SIGINT")
	require.NoError(t, err)
	assert.Equal(t, 2, int(s))

	s, err = ParseSignal("SIGKILL")
	require.NoError(t, err)
	assert.Equal(t, 9, int(s))

	s, err = ParseSignal("SIGTERM")
	require.NoError(t, err)
	assert.Equal(t, 15, int(s))

	s, err = ParseSignal("UNKNOWN")
	require.NoError(t, err)
	assert.Equal(t, 9, int(s)) // Default
}
