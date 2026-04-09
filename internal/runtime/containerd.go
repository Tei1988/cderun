package runtime

import (
	"context"
	"fmt"
	"io"
	"strings"
	"syscall"
	"time"

	"cderun/internal/container"
	"cderun/internal/logging"

	"github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/cio"
	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/containerd/errdefs"
	"github.com/google/uuid"
	"github.com/opencontainers/runtime-spec/specs-go"
)

const (
	containerdNamespace = "cderun"
)

// ContainerdRuntime implements ContainerRuntime using containerd client.
type ContainerdRuntime struct {
	client    *client.Client
	socket    string
	ioCreator cio.Creator
}

// NewContainerdRuntime creates a new ContainerdRuntime instance.
func NewContainerdRuntime(socket string) (*ContainerdRuntime, error) {
	c, err := client.New(socket, client.WithDefaultNamespace(containerdNamespace))
	if err != nil {
		return nil, fmt.Errorf("failed to create containerd client: %w", err)
	}
	return &ContainerdRuntime{
		client: c,
		socket: socket,
	}, nil
}

// Name returns the name of the runtime.
func (r *ContainerdRuntime) Name() string {
	return "containerd"
}

// PullImage pulls the specified image.
func (r *ContainerdRuntime) PullImage(ctx context.Context, img string, pullPolicy string, maxRetries int, backoffBase time.Duration) error {
	if pullPolicy == "never" {
		return nil
	}

	var lastErr error
	for i := range maxRetries {
		if i > 0 {
			logging.Warn("Retrying image pull (%d/%d) with exponential backoff for %s after error: %v", i, maxRetries-1, img, lastErr)
			timer := time.NewTimer(time.Duration(1<<i) * backoffBase)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}

		if pullPolicy == "missing" {
			_, err := r.client.GetImage(ctx, img)
			if err == nil {
				return nil // Image exists locally
			}
			if !errdefs.IsNotFound(err) {
				lastErr = err
				if isRetryablePullError(err) {
					continue
				}
				return fmt.Errorf("failed to get image: %w", err)
			}
		}

		// Policy is "always" or "missing" (and not found locally)
		logging.Info("Pulling image %s...", img)
		_, err := r.client.Pull(ctx, img, client.WithPullUnpack)
		if err != nil {
			lastErr = err
			if isRetryablePullError(err) {
				continue
			}
			return fmt.Errorf("failed to pull image: %w", err)
		}

		return nil // Success
	}

	return fmt.Errorf("failed to pull image after %d attempts: %w", maxRetries, lastErr)
}


// CreateContainer creates a new container.
func (r *ContainerdRuntime) CreateContainer(ctx context.Context, config *container.ContainerConfig) (string, error) {
	image, err := r.client.GetImage(ctx, config.Image)
	if err != nil {
		return "", fmt.Errorf("failed to get image: %w", err)
	}

	containerID := uuid.New().String()

	opts := []oci.SpecOpts{
		oci.WithImageConfig(image),
	}

	if len(config.Entrypoint) > 0 || len(config.Command) > 0 {
		var args []string
		if len(config.Entrypoint) > 0 {
			args = append([]string{}, config.Entrypoint...)
			args = append(args, config.Command...)
		} else {
			args = config.Command
		}
		opts = append(opts, oci.WithProcessArgs(args...))
	}

	if config.TTY {
		opts = append(opts, oci.WithTTY)
	}

	if config.User != "" {
		opts = append(opts, oci.WithUser(config.User))
	}

	if config.Workdir != "" {
		opts = append(opts, oci.WithProcessCwd(config.Workdir))
	}

	if len(config.Env) > 0 {
		opts = append(opts, oci.WithEnv(config.Env))
	}

	for _, m := range config.Mounts {
		var mountOptions []string
		if m.ReadOnly {
			mountOptions = append(mountOptions, "ro")
		}
		if m.Type == "bind" || m.Type == "" {
			mountOptions = append(mountOptions, "bind", "rbind")
		}
		opts = append(opts, oci.WithMounts([]specs.Mount{
			{
				Type:        m.Type,
				Source:      m.Source,
				Destination: m.Target,
				Options:     mountOptions,
			},
		}))
	}

	if config.Privileged {
		opts = append(opts, oci.WithPrivileged)
	}

	if len(config.CapAdd) > 0 {
		opts = append(opts, oci.WithCapabilities(config.CapAdd))
	}

	if config.Memory > 0 {
		opts = append(opts, oci.WithMemoryLimit(uint64(config.Memory)))
	}

	if config.CPUs > 0 {
		period := uint64(100000)
		quota := int64(config.CPUs * float64(period))
		opts = append(opts, oci.WithCPUCFS(quota, period))
	}

	for _, d := range config.Devices {
		opts = append(opts, oci.WithDevices(d.PathOnHost, d.PathInContainer, d.CgroupPermissions))
	}

	_, err = r.client.NewContainer(
		ctx,
		containerID,
		client.WithNewSpec(opts...),
		client.WithImage(image),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create containerd container: %w", err)
	}

	return containerID, nil
}

// StartContainer starts a created container.
func (r *ContainerdRuntime) StartContainer(ctx context.Context, containerID string) error {
	container, err := r.client.LoadContainer(ctx, containerID)
	if err != nil {
		return fmt.Errorf("failed to load container: %w", err)
	}

	creator := r.ioCreator
	if creator == nil {
		creator = cio.NewCreator(cio.WithStdio)
	}

	task, err := container.NewTask(ctx, creator)
	if err != nil {
		return fmt.Errorf("failed to create task: %w", err)
	}

	if err := task.Start(ctx); err != nil {
		return fmt.Errorf("failed to start task: %w", err)
	}

	return nil
}

// WaitContainer waits for a container to exit and returns its exit code.
func (r *ContainerdRuntime) WaitContainer(ctx context.Context, containerID string) (int, error) {
	container, err := r.client.LoadContainer(ctx, containerID)
	if err != nil {
		return 0, fmt.Errorf("failed to load container: %w", err)
	}

	task, err := container.Task(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to load task: %w", err)
	}

	exitStatusC, err := task.Wait(ctx)
	if err != nil {
		return 0, err
	}
	status := <-exitStatusC
	return int(status.ExitCode()), status.Error()
}

// RemoveContainer removes a container.
func (r *ContainerdRuntime) RemoveContainer(ctx context.Context, containerID string) error {
	container, err := r.client.LoadContainer(ctx, containerID)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to load container: %w", err)
	}

	task, err := container.Task(ctx, nil)
	if err == nil {
		if _, err := task.Delete(ctx, client.WithProcessKill); err != nil {
			if !errdefs.IsNotFound(err) {
				return fmt.Errorf("failed to delete task: %w", err)
			}
		}
	}

	if err := container.Delete(ctx); err != nil {
		if !errdefs.IsNotFound(err) {
			return fmt.Errorf("failed to delete container: %w", err)
		}
	}

	return nil
}

// AttachContainer attaches to a container's IO streams.
func (r *ContainerdRuntime) AttachContainer(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error {
	ioOpts := []cio.Opt{
		cio.WithStreams(stdin, stdout, stderr),
	}
	if tty {
		ioOpts = append(ioOpts, cio.WithTerminal)
	}

	r.ioCreator = cio.NewCreator(ioOpts...)

	if ready != nil {
		close(ready)
	}

	return nil
}

// ResizeContainerTTY resizes the terminal of a container.
func (r *ContainerdRuntime) ResizeContainerTTY(ctx context.Context, containerID string, rows, cols uint) error {
	container, err := r.client.LoadContainer(ctx, containerID)
	if err != nil {
		return fmt.Errorf("failed to load container: %w", err)
	}

	task, err := container.Task(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to load task: %w", err)
	}

	return task.Resize(ctx, uint32(cols), uint32(rows)) //nolint:gosec
}

// SignalContainer sends a signal to a container.
func (r *ContainerdRuntime) SignalContainer(ctx context.Context, containerID string, sig string) error {
	container, err := r.client.LoadContainer(ctx, containerID)
	if err != nil {
		return fmt.Errorf("failed to load container: %w", err)
	}

	task, err := container.Task(ctx, nil)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to load task: %w", err)
	}

	s, err := parseSignal(sig)
	if err != nil {
		return err
	}

	return task.Kill(ctx, s)
}

func parseSignal(sig string) (syscall.Signal, error) {
	switch strings.ToUpper(sig) {
	case "SIGTERM", "15":
		return syscall.SIGTERM, nil
	case "SIGKILL", "9":
		return syscall.SIGKILL, nil
	case "SIGINT", "2":
		return syscall.SIGINT, nil
	case "SIGHUP", "1":
		return syscall.SIGHUP, nil
	case "SIGQUIT", "3":
		return syscall.SIGQUIT, nil
	case "":
		return syscall.SIGTERM, nil
	default:
		return 0, fmt.Errorf("unsupported signal: %s", sig)
	}
}

// InspectContainer inspects the container to get its status and exit code.
func (r *ContainerdRuntime) InspectContainer(ctx context.Context, containerID string) (bool, int, error) {
	container, err := r.client.LoadContainer(ctx, containerID)
	if err != nil {
		return false, 0, fmt.Errorf("failed to load container: %w", err)
	}

	task, err := container.Task(ctx, nil)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return false, 0, nil
		}
		return false, 0, fmt.Errorf("failed to load task: %w", err)
	}

	status, err := task.Status(ctx)
	if err != nil {
		return false, 0, fmt.Errorf("failed to get task status: %w", err)
	}

	isRunning := status.Status == client.Running
	return isRunning, int(status.ExitStatus), nil
}
