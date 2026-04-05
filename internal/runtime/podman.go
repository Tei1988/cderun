package runtime

import (
	"context"
	"net"
	"net/http"

	"github.com/docker/docker/client"
)

// NewPodmanRuntime creates a new Podman runtime instance.
// It uses the Docker-compatible API provided by Podman.
func NewPodmanRuntime(socket string) (ContainerRuntime, error) {
	// Create a custom transport to handle unix sockets and disable keep-alives.
	// DisableKeepAlives: true is a known workaround for EOF issues with some
	// container runtime proxies (especially Podman's system service).
	httpClient := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "unix", socket)
			},
			DisableKeepAlives: true,
		},
	}

	rt, err := NewDockerRuntimeWithOptions(
		socket,
		"podman",
		client.WithHTTPClient(httpClient),
		// Podman sometimes has issues with version negotiation or higher API versions.
		// Explicitly using 1.41 (compatible with Podman 4.0+) is more stable.
		client.WithVersion("1.41"),
	)
	if err != nil {
		return nil, err // NewDockerRuntimeWithOptions already wraps it in RuntimeInitError
	}
	return rt, nil
}
