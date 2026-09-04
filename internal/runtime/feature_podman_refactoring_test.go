package runtime

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Podman_NewHTTPClient(t *testing.T) {
	socket := "/tmp/podman-test.sock"
	client := newPodmanHTTPClient(socket)
	require.NotNil(t, client)

	transport, ok := client.Transport.(*http.Transport)
	require.True(t, ok)
	assert.True(t, transport.DisableKeepAlives)
	assert.NotNil(t, transport.DialContext)
}

func TestUnit_Podman_NewPodmanRuntime_Refactored(t *testing.T) {
	socket := "/run/user/1000/podman/podman.sock"
	rt, err := NewPodmanRuntime(socket)
	require.NoError(t, err)
	require.NotNil(t, rt)
	assert.Equal(t, "podman", rt.Name())
}
