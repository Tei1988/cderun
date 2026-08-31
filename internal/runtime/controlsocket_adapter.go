package runtime

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"cderun/internal/container"
	"cderun/internal/logging"
	"cderun/internal/runtime/controlsocket"
)

// ControlSocketRuntimeAdapter adapts a Control Socket client to the ContainerRuntime interface.
type ControlSocketRuntimeAdapter struct {
	underlying ContainerRuntime
	client     *controlsocket.Client
	logger     *logging.Logger
}

// NewControlSocketRuntimeAdapter wraps an underlying ContainerRuntime with a Control Socket client
// for dispatching container lifecycle calls over the control socket.
func NewControlSocketRuntimeAdapter(underlying ContainerRuntime, client *controlsocket.Client, logger *logging.Logger) *ControlSocketRuntimeAdapter {
	if logger == nil {
		logger = logging.GetGlobalLogger()
	}
	return &ControlSocketRuntimeAdapter{
		underlying: underlying,
		client:     client,
		logger:     logger,
	}
}

func (a *ControlSocketRuntimeAdapter) PullImage(ctx context.Context, image string, pullPolicy string, maxRetries int, backoffBase time.Duration) error {
	return a.underlying.PullImage(ctx, image, pullPolicy, maxRetries, backoffBase)
}

func (a *ControlSocketRuntimeAdapter) ValidateConfig(config *container.ContainerConfig) error {
	return a.underlying.ValidateConfig(config)
}

func (a *ControlSocketRuntimeAdapter) CreateContainer(ctx context.Context, config *container.ContainerConfig) (string, error) {
	a.logger.Debug("Dispatching CreateContainer via Control Socket client")
	return a.client.CreateContainer(ctx, config)
}

func (a *ControlSocketRuntimeAdapter) StartContainer(ctx context.Context, containerID string) error {
	a.logger.Debug("Dispatching StartContainer via Control Socket client")
	return a.client.StartContainer(ctx, containerID)
}

func (a *ControlSocketRuntimeAdapter) WaitContainer(ctx context.Context, containerID string) (int, error) {
	a.logger.Debug("Dispatching WaitContainer via Control Socket client")
	return a.client.WaitContainer(ctx, containerID)
}

func (a *ControlSocketRuntimeAdapter) RemoveContainer(ctx context.Context, containerID string) error {
	a.logger.Debug("Dispatching RemoveContainer via Control Socket client")
	return a.client.RemoveContainer(ctx, containerID)
}

func (a *ControlSocketRuntimeAdapter) AttachContainer(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error {
	a.logger.Debug("Dispatching AttachContainer via Control Socket client")
	return a.client.AttachContainer(ctx, containerID, tty, stdin, stdout, stderr, ready)
}

func (a *ControlSocketRuntimeAdapter) ResizeContainerTTY(ctx context.Context, containerID string, rows, cols uint) error {
	a.logger.Debug("Dispatching ResizeContainerTTY via Control Socket client")
	return a.client.ResizeContainerTTY(ctx, containerID, rows, cols)
}

func (a *ControlSocketRuntimeAdapter) SignalContainer(ctx context.Context, containerID string, sig string) error {
	a.logger.Debug("Dispatching SignalContainer via Control Socket client")
	return a.client.SignalContainer(ctx, containerID, sig)
}

func (a *ControlSocketRuntimeAdapter) Name() string {
	return fmt.Sprintf("%s-controlsocket", a.underlying.Name())
}

func (a *ControlSocketRuntimeAdapter) InspectContainer(ctx context.Context, containerID string) (bool, int, error) {
	return a.underlying.InspectContainer(ctx, containerID)
}

func (a *ControlSocketRuntimeAdapter) Close() error {
	var errs []error
	if err := a.client.Close(); err != nil {
		errs = append(errs, err)
	}
	if err := a.underlying.Close(); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}
