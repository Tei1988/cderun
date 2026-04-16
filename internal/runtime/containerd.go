package runtime

import (
	"context"
	"fmt"
	"io"
	"syscall"
	"time"

	"cderun/internal/container"
	"cderun/internal/logging"

	"github.com/google/uuid"
	"github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/cio"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/containerd/errdefs"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// ContainerdRuntime implements ContainerRuntime using containerd client.
type ContainerdRuntime struct {
	client    *client.Client
	namespace string
}

// NewContainerdRuntime creates a new ContainerdRuntime instance.
func NewContainerdRuntime(socket string) (*ContainerdRuntime, error) {
	c, err := client.New(socket)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to containerd: %w", err)
	}

	return &ContainerdRuntime{
		client:    c,
		namespace: "default", // Default containerd namespace
	}, nil
}

// Name returns the name of the runtime.
func (c *ContainerdRuntime) Name() string {
	return "containerd"
}

// Close closes the containerd client.
func (c *ContainerdRuntime) Close() error {
	if c.client == nil {
		return nil
	}
	return c.client.Close()
}

// PullImage pulls the specified image.
func (c *ContainerdRuntime) PullImage(ctx context.Context, ref string, pullPolicy string, maxRetries int, backoffBase time.Duration) error {
	if pullPolicy == "never" {
		return nil
	}

	ctx = namespaces.WithNamespace(ctx, c.namespace)

	var lastErr error
	for i := range maxRetries {
		if i > 0 {
			logging.Warn("Retrying image pull (%d/%d) with exponential backoff for %s after error: %v", i, maxRetries-1, ref, lastErr)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(1<<i) * backoffBase):
			}
		}

		if pullPolicy == "missing" {
			_, err := c.client.ImageService().Get(ctx, ref)
			if err == nil {
				return nil // Image exists locally
			}
			if !errdefs.IsNotFound(err) {
				lastErr = err
				continue
			}
		}

		logging.Info("Pulling image %s...", ref)
		_, err := c.client.Pull(ctx, ref, client.WithPullUnpack)
		if err != nil {
			lastErr = err
			continue
		}

		return nil
	}

	return fmt.Errorf("failed to pull image after %d attempts: %w", maxRetries, lastErr)
}

// CreateContainer creates a new container.
func (c *ContainerdRuntime) CreateContainer(ctx context.Context, config *container.ContainerConfig) (string, error) {
	ctx = namespaces.WithNamespace(ctx, c.namespace)

	img, err := c.client.GetImage(ctx, config.Image)
	if err != nil {
		return "", fmt.Errorf("failed to get image: %w", err)
	}

	id := "cderun-" + uuid.NewString()

	// Basic OCI spec
	opts := []oci.SpecOpts{
		oci.WithDefaultSpec(),
		oci.WithImageConfig(img),
	}

	if config.TTY {
		opts = append(opts, oci.WithTTY)
	}

	if config.Workdir != "" {
		opts = append(opts, oci.WithProcessCwd(config.Workdir))
	}

	if len(config.Command) > 0 {
		opts = append(opts, oci.WithProcessArgs(config.Command...))
	}

	if len(config.Env) > 0 {
		opts = append(opts, oci.WithEnv(config.Env))
	}

	// Add mounts
	if len(config.Mounts) > 0 {
		var mnts []specs.Mount
		for _, m := range config.Mounts {
			mnts = append(mnts, specs.Mount{
				Type:        m.Type,
				Source:      m.Source,
				Destination: m.Target,
				Options:     []string{"rbind", "rw"}, // Default options
			})
		}
		opts = append(opts, oci.WithMounts(mnts))
	}

	container, err := c.client.NewContainer(
		ctx,
		id,
		client.WithImage(img),
		client.WithNewSpec(opts...),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	return container.ID(), nil
}

// StartContainer starts the container by creating a task.
func (c *ContainerdRuntime) StartContainer(ctx context.Context, containerID string) error {
	ctx = namespaces.WithNamespace(ctx, c.namespace)
	container, err := c.client.LoadContainer(ctx, containerID)
	if err != nil {
		return err
	}

	task, err := container.Task(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to get task: %w", err)
	}

	return task.Start(ctx)
}

// WaitContainer waits for the container task to exit.
func (c *ContainerdRuntime) WaitContainer(ctx context.Context, containerID string) (int, error) {
	ctx = namespaces.WithNamespace(ctx, c.namespace)
	container, err := c.client.LoadContainer(ctx, containerID)
	if err != nil {
		return 0, err
	}

	task, err := container.Task(ctx, nil)
	if err != nil {
		return 0, err
	}

	exitStatusC, err := task.Wait(ctx)
	if err != nil {
		return 0, err
	}
	select {
	case status := <-exitStatusC:
		return int(status.ExitCode()), nil
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

// RemoveContainer removes the container and its task.
func (c *ContainerdRuntime) RemoveContainer(ctx context.Context, containerID string) error {
	ctx = namespaces.WithNamespace(ctx, c.namespace)
	container, err := c.client.LoadContainer(ctx, containerID)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return nil
		}
		return err
	}

	task, err := container.Task(ctx, nil)
	if err == nil {
		// Kill and delete task if it exists
		_ = task.Kill(ctx, syscall.SIGKILL)
		_, _ = task.Delete(ctx)
	}

	return container.Delete(ctx, client.WithSnapshotCleanup)
}

// SignalContainer sends a signal to the container task.
func (c *ContainerdRuntime) SignalContainer(ctx context.Context, containerID string, sig string) error {
	ctx = namespaces.WithNamespace(ctx, c.namespace)
	container, err := c.client.LoadContainer(ctx, containerID)
	if err != nil {
		return err
	}

	task, err := container.Task(ctx, nil)
	if err != nil {
		return err
	}

	s, err := ParseSignal(sig)
	if err != nil {
		return err
	}

	return task.Kill(ctx, s)
}

// ResizeContainerTTY resizes the terminal.
func (c *ContainerdRuntime) ResizeContainerTTY(ctx context.Context, containerID string, rows, cols uint) error {
	ctx = namespaces.WithNamespace(ctx, c.namespace)
	container, err := c.client.LoadContainer(ctx, containerID)
	if err != nil {
		return err
	}

	task, err := container.Task(ctx, nil)
	if err != nil {
		return err
	}

	return task.Resize(ctx, uint32(cols), uint32(rows))
}

// AttachContainer attaches I/O to the container task.
func (c *ContainerdRuntime) AttachContainer(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error {
	ctx = namespaces.WithNamespace(ctx, c.namespace)
	container, err := c.client.LoadContainer(ctx, containerID)
	if err != nil {
		if ready != nil {
			close(ready)
		}
		return err
	}

	// For containerd, I/O is specified during task creation.
	// Since cderun calls AttachContainer BEFORE StartContainer,
	// we should create the task here.

	ioOpts := []cio.Opt{
		cio.WithStreams(stdin, stdout, stderr),
	}
	if tty {
		ioOpts = append(ioOpts, cio.WithTerminal)
	}

	task, err := container.NewTask(ctx, cio.NewCreator(ioOpts...))
	if err != nil {
		if ready != nil {
			close(ready)
		}
		return fmt.Errorf("failed to create task: %w", err)
	}

	if ready != nil {
		close(ready)
	}

	// Wait for task to finish or context to be cancelled
	exitStatusC, err := task.Wait(ctx)
	if err != nil {
		return err
	}
	select {
	case <-exitStatusC:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// InspectContainer returns the container status.
func (c *ContainerdRuntime) InspectContainer(ctx context.Context, containerID string) (bool, int, error) {
	ctx = namespaces.WithNamespace(ctx, c.namespace)
	container, err := c.client.LoadContainer(ctx, containerID)
	if err != nil {
		return false, 0, err
	}

	task, err := container.Task(ctx, nil)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return false, 0, nil
		}
		return false, 0, err
	}

	status, err := task.Status(ctx)
	if err != nil {
		return false, 0, err
	}

	isRunning := status.Status == client.Running
	return isRunning, int(status.ExitStatus), nil
}

// ParseSignal converts string signal to syscall.Signal.
func ParseSignal(sig string) (syscall.Signal, error) {
	switch sig {
	case "SIGINT", "2":
		return syscall.SIGINT, nil
	case "SIGKILL", "9":
		return syscall.SIGKILL, nil
	case "SIGTERM", "15":
		return syscall.SIGTERM, nil
	// Add more as needed
	default:
		return syscall.SIGKILL, nil // Default
	}
}
