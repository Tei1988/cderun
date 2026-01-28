package runtime

import (
	"cderun/internal/container"
	"context"
	"io"
)

// ContainerRuntime defines the interface for interacting with container runtimes.
type ContainerRuntime interface {
	// Container lifecycle
	CreateContainer(ctx context.Context, config *container.ContainerConfig) (string, error)
	StartContainer(ctx context.Context, containerID string) error
	WaitContainer(ctx context.Context, containerID string) (int, error)
	RemoveContainer(ctx context.Context, containerID string) error

	// Container communication
	AttachContainer(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer) error

	// Information
	Name() string
}
