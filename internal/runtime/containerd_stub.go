//go:build !linux

package runtime

import (
	"context"
	"fmt"
	"io"
	"time"

	"cderun/internal/container"
	"cderun/internal/logging"
)

// ContainerdRuntime is not supported on non-Linux platforms.
type ContainerdRuntime struct{}

// ContainerdRuntimeOption is a no-op option type for non-Linux platforms.
type ContainerdRuntimeOption func(*ContainerdRuntime)

// WithContainerdLogger is a no-op on non-Linux platforms.
func WithContainerdLogger(_ *logging.Logger) ContainerdRuntimeOption {
	return func(_ *ContainerdRuntime) {}
}

// NewContainerdRuntime always returns an error on non-Linux platforms.
func NewContainerdRuntime(_ string, _ ...ContainerdRuntimeOption) (*ContainerdRuntime, error) {
	return nil, fmt.Errorf("containerd runtime is only supported on Linux")
}

func (r *ContainerdRuntime) PullImage(_ context.Context, _ string, _ string, _ int, _ time.Duration) error {
	return fmt.Errorf("containerd runtime is only supported on Linux")
}

func (r *ContainerdRuntime) CreateContainer(_ context.Context, _ *container.ContainerConfig) (string, error) {
	return "", fmt.Errorf("containerd runtime is only supported on Linux")
}

func (r *ContainerdRuntime) StartContainer(_ context.Context, _ string) error {
	return fmt.Errorf("containerd runtime is only supported on Linux")
}

func (r *ContainerdRuntime) WaitContainer(_ context.Context, _ string) (int, error) {
	return 0, fmt.Errorf("containerd runtime is only supported on Linux")
}

func (r *ContainerdRuntime) RemoveContainer(_ context.Context, _ string) error {
	return fmt.Errorf("containerd runtime is only supported on Linux")
}

func (r *ContainerdRuntime) AttachContainer(_ context.Context, _ string, _ bool, _ io.Reader, _, _ io.Writer, _ chan<- struct{}) error {
	return fmt.Errorf("containerd runtime is only supported on Linux")
}

func (r *ContainerdRuntime) ResizeContainerTTY(_ context.Context, _ string, _, _ uint) error {
	return fmt.Errorf("containerd runtime is only supported on Linux")
}

func (r *ContainerdRuntime) SignalContainer(_ context.Context, _ string, _ string) error {
	return fmt.Errorf("containerd runtime is only supported on Linux")
}

func (r *ContainerdRuntime) Name() string { return "containerd" }

func (r *ContainerdRuntime) InspectContainer(_ context.Context, _ string) (bool, int, error) {
	return false, 0, fmt.Errorf("containerd runtime is only supported on Linux")
}

func (r *ContainerdRuntime) Close() error {
	return fmt.Errorf("containerd runtime is only supported on Linux")
}
