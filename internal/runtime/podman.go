package runtime

import (
	"fmt"
)

// NewPodmanRuntime creates a new Podman runtime instance.
// Currently, Podman is not implemented and this function returns an error.
func NewPodmanRuntime(socket string) (ContainerRuntime, error) {
	return nil, fmt.Errorf("podman runtime is not implemented yet")
}
