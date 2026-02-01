package runtime

// NewPodmanRuntime creates a new Podman runtime instance.
// It uses the Docker-compatible API provided by Podman.
func NewPodmanRuntime(socket string) (ContainerRuntime, error) {
	return NewDockerRuntimeWithName(socket, "podman")
}
