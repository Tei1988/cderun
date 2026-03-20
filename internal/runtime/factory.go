package runtime

func NewRuntime(name, socket string) (ContainerRuntime, error) {
	if name == "podman" {
		return NewPodmanRuntime(socket)
	}
	return NewDockerRuntime(socket)
}
