package runtime

import (
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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

func TestUnit_Podman_DialContext(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "podman.sock")
	l, err := net.Listen("unix", socket)
	require.NoError(t, err)
	defer l.Close()

	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Mock response for ContainerInspect (which is usually /v1.41/containers/test/json)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"State":{"Running":true,"ExitCode":0}}`))
		}),
	}
	go func() {
		_ = srv.Serve(l)
	}()
	defer func() {
		_ = srv.Shutdown(context.Background())
	}()

	rt, err := NewPodmanRuntime(socket)
	require.NoError(t, err)

	// Trigger a request to exercise DialContext
	running, code, err := rt.InspectContainer(context.Background(), "test")
	require.NoError(t, err)
	assert.True(t, running)
	assert.Equal(t, 0, code)
}

func TestUnit_Podman_DialContext_Error(t *testing.T) {
	// Use a path that doesn't exist to trigger dial error
	socket := filepath.Join(t.TempDir(), "nonexistent.sock")
	rt, err := NewPodmanRuntime(socket)
	require.NoError(t, err)

	_, _, err = rt.InspectContainer(context.Background(), "test")
	require.Error(t, err)
	// Error should be related to connection failure
	assert.True(t, os.IsNotExist(err) ||
		containsAny(err.Error(), "no such file", "connect: connection refused", "context canceled", "cannot connect to the docker daemon"),
		"expected connection error, got: %v", err)
}

func containsAny(s string, keywords ...string) bool {
	s = strings.ToLower(s)
	for _, kw := range keywords {
		if strings.Contains(s, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}
