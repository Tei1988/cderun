package runtime

import (
	"context"
	"fmt"
	"io"
	"time"

	"cderun/internal/container"
	"cderun/internal/logging"

	"github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/cio"
	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/google/uuid"
	"github.com/moby/sys/signal"
	"github.com/opencontainers/runtime-spec/specs-go"
)

const (
	defaultContainerdNamespace = "cderun"
)

// ContainerdRuntime implements ContainerRuntime using containerd client.
type ContainerdRuntime struct {
	client    *client.Client
	socket    string
	namespace string
}

// NewContainerdRuntime creates a new ContainerdRuntime instance.
func NewContainerdRuntime(socket string) (*ContainerdRuntime, error) {
	return NewContainerdRuntimeWithNamespace(socket, defaultContainerdNamespace)
}

// NewContainerdRuntimeWithNamespace creates a new ContainerdRuntime instance with a specific namespace.
func NewContainerdRuntimeWithNamespace(socket string, namespace string) (*ContainerdRuntime, error) {
	c, err := client.New(socket, client.WithDefaultNamespace(namespace))
	if err != nil {
		return nil, fmt.Errorf("failed to create containerd client: %w", err)
	}

	return &ContainerdRuntime{
		client:    c,
		socket:    socket,
		namespace: namespace,
	}, nil
}

// Name returns the name of the runtime.
func (r *ContainerdRuntime) Name() string {
	return "containerd"
}

// Close closes the underlying containerd client.
func (r *ContainerdRuntime) Close() error {
	return r.client.Close()
}

// PullImage pulls the specified image.
func (r *ContainerdRuntime) PullImage(ctx context.Context, img string, pullPolicy string, maxRetries int, backoffBase time.Duration) error {
	if pullPolicy == "never" {
		return nil
	}

	if pullPolicy == "missing" {
		imageService := r.client.ImageService()
		_, err := imageService.Get(ctx, img)
		if err == nil {
			return nil // Image exists
		}
	}

	var lastErr error
	for i := range maxRetries {
		if i > 0 {
			logging.Warn("Retrying image pull (%d/%d) with exponential backoff for %s after error: %v", i, maxRetries-1, img, lastErr)
			time.Sleep(time.Duration(1<<uint(i)) * backoffBase)
		}

		_, err := r.client.Pull(ctx, img, client.WithPullUnpack)
		if err == nil {
			return nil
		}
		lastErr = err
	}

	return fmt.Errorf("failed to pull image after %d attempts: %w", maxRetries, lastErr)
}

// CreateContainer creates a new container.
func (r *ContainerdRuntime) CreateContainer(ctx context.Context, config *container.ContainerConfig) (string, error) {
	img, err := r.client.GetImage(ctx, config.Image)
	if err != nil {
		return "", fmt.Errorf("failed to get image %q: %w", config.Image, err)
	}

	id := uuid.New().String()

	var opts []oci.SpecOpts
	opts = append(opts, oci.WithDefaultSpec(), oci.WithImageConfig(img))

	if len(config.Command) > 0 {
		opts = append(opts, oci.WithProcessArgs(config.Command...))
	}
	if config.Workdir != "" {
		opts = append(opts, oci.WithProcessCwd(config.Workdir))
	}
	if len(config.Env) > 0 {
		opts = append(opts, oci.WithEnv(config.Env))
	}
	if config.User != "" {
		opts = append(opts, oci.WithUser(config.User))
	}

	// TTY
	if config.TTY {
		opts = append(opts, oci.WithTTY)
	}

	// Mounts
	for _, m := range config.Mounts {
		if m.Type == "bind" || m.Type == "" {
			opts = append(opts, oci.WithMounts([]specs.Mount{
				{
					Type:        "bind",
					Source:      m.Source,
					Destination: m.Target,
					Options:     []string{"bind", "rbind"},
				},
			}))
		}
	}

	// Privileged
	if config.Privileged {
		opts = append(opts, oci.WithPrivileged)
	}

	// Caps
	if len(config.CapAdd) > 0 {
		opts = append(opts, oci.WithCapabilities(config.CapAdd))
	}

	container, err := r.client.NewContainer(
		ctx,
		id,
		client.WithNewSnapshot(id, img),
		client.WithNewSpec(opts...),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	return container.ID(), nil
}

// StartContainer starts a container by creating a task.
func (r *ContainerdRuntime) StartContainer(ctx context.Context, containerID string) error {
	// In containerd, "starting" means creating a task.
	// However, we usually create the task in AttachContainer or separately.
	// For the cderun model, we'll assume the task is created during Start if not already.
	// But WaitContainer and AttachContainer also need access to the task.
	return nil
}

// WaitContainer waits for a container's task to exit.
func (r *ContainerdRuntime) WaitContainer(ctx context.Context, containerID string) (int, error) {
	container, err := r.client.LoadContainer(ctx, containerID)
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
		return int(status.ExitCode()), status.Error()
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

// RemoveContainer removes a container and its snapshot.
func (r *ContainerdRuntime) RemoveContainer(ctx context.Context, containerID string) error {
	container, err := r.client.LoadContainer(ctx, containerID)
	if err != nil {
		return err
	}

	task, err := container.Task(ctx, nil)
	if err == nil {
		_, _ = task.Delete(ctx, client.WithProcessKill) //nolint:errcheck
	}

	if err := container.Delete(ctx, client.WithSnapshotCleanup); err != nil {
		return err
	}

	return nil
}

// AttachContainer attaches to a container's IO by creating a task.
func (r *ContainerdRuntime) AttachContainer(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error {
	container, err := r.client.LoadContainer(ctx, containerID)
	if err != nil {
		return err
	}

	var cioOpts cio.Opt
	if tty {
		cioOpts = cio.WithTerminal
	}

	task, err := container.NewTask(ctx, cio.NewCreator(cio.WithStreams(stdin, stdout, stderr), cioOpts))
	if err != nil {
		return err
	}

	if ready != nil {
		close(ready)
	}

	if err := task.Start(ctx); err != nil {
		return err
	}

	return nil
}

// ResizeContainerTTY resizes the terminal of a container's task.
func (r *ContainerdRuntime) ResizeContainerTTY(ctx context.Context, containerID string, rows, cols uint) error {
	container, err := r.client.LoadContainer(ctx, containerID)
	if err != nil {
		return err
	}

	task, err := container.Task(ctx, nil)
	if err != nil {
		return err
	}

	return task.Resize(ctx, uint32(cols), uint32(rows)) //nolint:gosec
}

// SignalContainer sends a signal to a container's task.
func (r *ContainerdRuntime) SignalContainer(ctx context.Context, containerID string, sig string) error {
	container, err := r.client.LoadContainer(ctx, containerID)
	if err != nil {
		return err
	}

	task, err := container.Task(ctx, nil)
	if err != nil {
		return err
	}

	s, err := signal.ParseSignal(sig)
	if err != nil {
		return fmt.Errorf("invalid signal %q: %w", sig, err)
	}

	return task.Kill(ctx, s)
}

// InspectContainer inspects the container status.
func (r *ContainerdRuntime) InspectContainer(ctx context.Context, containerID string) (bool, int, error) {
	container, err := r.client.LoadContainer(ctx, containerID)
	if err != nil {
		return false, 0, err
	}

	task, err := container.Task(ctx, nil)
	if err != nil {
		return false, 0, err
	}

	status, err := task.Status(ctx)
	if err != nil {
		return false, 0, err
	}

	isRunning := status.Status == client.Running
	return isRunning, int(status.ExitStatus), nil
}
