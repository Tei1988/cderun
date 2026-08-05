package runtime

import (
	"context"
	"io"
	"time"

	"cderun/internal/container"
)

// ContainerRuntime defines the interface for interacting with container runtimes.
type ContainerRuntime interface {
	// Image management
	PullImage(ctx context.Context, image string, pullPolicy string, maxRetries int, backoffBase time.Duration) error

	// Container lifecycle
	ValidateConfig(config *container.ContainerConfig) error
	CreateContainer(ctx context.Context, config *container.ContainerConfig) (string, error)
	StartContainer(ctx context.Context, containerID string) error
	WaitContainer(ctx context.Context, containerID string) (int, error)
	RemoveContainer(ctx context.Context, containerID string) error

	// Container communication
	// AttachContainer attaches to the container's standard streams (stdin, stdout, stderr).
	// For some runtimes (e.g., containerd), AttachContainer must be called BEFORE StartContainer,
	// otherwise the process will fall back to NullIO and all standard I/O streams will be silently discarded.
	AttachContainer(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error
	ResizeContainerTTY(ctx context.Context, containerID string, rows, cols uint) error
	SignalContainer(ctx context.Context, containerID string, sig string) error

	// Information
	Name() string
	InspectContainer(ctx context.Context, containerID string) (isRunning bool, exitCode int, err error)

	// Cleanup
	Close() error
}
