package runtime

import (
	"context"
	"net"
	"net/http"

	"github.com/docker/docker/client"
)

// newPodmanHTTPClient creates a custom HTTP client configured for Podman Unix domain sockets.
// DisableKeepAlives: true is a known workaround for EOF issues with some
// container runtime proxies (especially Podman's system service).
func newPodmanHTTPClient(socket string) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "unix", socket)
			},
			DisableKeepAlives: true,
		},
	}
}

// NewPodmanRuntime creates a new Podman runtime instance.
// It uses the Docker-compatible API provided by Podman.
func NewPodmanRuntime(socket string, opts ...DockerRuntimeOption) (*DockerRuntime, error) {
	httpClient := newPodmanHTTPClient(socket)

	return NewDockerRuntimeWithOptions(
		socket,
		"podman",
		[]client.Opt{
			client.WithHTTPClient(httpClient),
			// Podman sometimes has issues with version negotiation or higher API versions.
			// Explicitly using 1.41 (compatible with Podman 4.0+) is more stable.
			client.WithVersion("1.41"),
		},
		opts...,
	)
}
