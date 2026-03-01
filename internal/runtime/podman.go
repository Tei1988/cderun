package runtime

import (
	"cderun/internal/logging"
)

// NewPodmanRuntime creates a new Podman runtime instance.
// It uses the Docker-compatible API provided by Podman.
func NewPodmanRuntime(socket string, logger *logging.Logger) (ContainerRuntime, error) {
	return NewDockerRuntimeWithName(socket, "podman", logger)
}
