package runtime

import (
	"cderun/internal/container"
	"context"
	"fmt"
	"io"
)

// NewPodmanRuntime creates a new Podman runtime instance.
// Currently, Podman is not implemented and this function returns an error.
func NewPodmanRuntime(socket string) (ContainerRuntime, error) {
	return nil, fmt.Errorf("podman runtime is not implemented yet")
}

// PodmanRuntime stub for interface compatibility.
type PodmanRuntime struct{}

func (p *PodmanRuntime) CreateContainer(ctx context.Context, config *container.ContainerConfig) (string, error) {
	return "", fmt.Errorf("podman runtime not implemented")
}
func (p *PodmanRuntime) StartContainer(ctx context.Context, containerID string) error {
	return fmt.Errorf("podman runtime not implemented")
}
func (p *PodmanRuntime) WaitContainer(ctx context.Context, containerID string) (int, error) {
	return 0, fmt.Errorf("podman runtime not implemented")
}
func (p *PodmanRuntime) RemoveContainer(ctx context.Context, containerID string) error {
	return fmt.Errorf("podman runtime not implemented")
}
func (p *PodmanRuntime) AttachContainer(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer) error {
	return fmt.Errorf("podman runtime not implemented")
}
func (p *PodmanRuntime) ResizeContainerTTY(ctx context.Context, containerID string, rows, cols uint) error {
	return fmt.Errorf("podman runtime not implemented")
}
func (p *PodmanRuntime) SignalContainer(ctx context.Context, containerID string, sig string) error {
	return fmt.Errorf("podman runtime not implemented")
}
func (p *PodmanRuntime) Name() string { return "podman" }
