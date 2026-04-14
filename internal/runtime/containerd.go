package runtime

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"cderun/internal/container"
	"cderun/internal/logging"

	"github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/pkg/cio"
	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/google/uuid"
	"github.com/moby/sys/signal"
	"github.com/opencontainers/runtime-spec/specs-go"
)

type containerdClient interface {
	Pull(ctx context.Context, ref string, opts ...client.RemoteOpt) (client.Image, error)
	GetImage(ctx context.Context, ref string) (client.Image, error)
	NewContainer(ctx context.Context, id string, opts ...client.NewContainerOpts) (client.Container, error)
	LoadContainer(ctx context.Context, id string) (client.Container, error)
	ImageService() images.Store
	Close() error
}

// ContainerdRuntime implements ContainerRuntime using containerd Go client.
type ContainerdRuntime struct {
	client    containerdClient
	socket    string
	namespace string
	sleepFunc func(context.Context, time.Duration) error

	tasks     map[string]client.Task
	taskIOs   map[string]cio.IO
	ioWait    map[string]chan struct{}
	mu        sync.RWMutex
}

// NewContainerdRuntime creates a new ContainerdRuntime instance.
func NewContainerdRuntime(socket string) (*ContainerdRuntime, error) {
	c, err := client.New(socket, client.WithDefaultNamespace("default"))
	if err != nil {
		return nil, fmt.Errorf("failed to create containerd client: %w", err)
	}

	return &ContainerdRuntime{
		client:    c,
		socket:    socket,
		namespace: "default",
		sleepFunc: func(ctx context.Context, d time.Duration) error {
			t := time.NewTimer(d)
			defer t.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-t.C:
				return nil
			}
		},
		tasks:   make(map[string]client.Task),
		taskIOs: make(map[string]cio.IO),
		ioWait:  make(map[string]chan struct{}),
	}, nil
}

// Name returns the name of the runtime.
func (r *ContainerdRuntime) Name() string {
	return "containerd"
}

// InspectContainer inspects the container to get its status and exit code.
func (r *ContainerdRuntime) InspectContainer(ctx context.Context, containerID string) (bool, int, error) {
	r.mu.RLock()
	task, ok := r.tasks[containerID]
	r.mu.RUnlock()

	if !ok {
		return false, 0, fmt.Errorf("task not found for container %s", containerID)
	}

	status, err := task.Status(ctx)
	if err != nil {
		return false, 0, err
	}

	running := status.Status == client.Running
	exitCode := int(status.ExitStatus)
	return running, exitCode, nil
}

// PullImage pulls the specified image based on the pull policy.
func (r *ContainerdRuntime) PullImage(ctx context.Context, img string, pullPolicy string, maxRetries int, backoffBase time.Duration) error {
	if pullPolicy == "never" {
		return nil
	}

	var lastErr error
	for i := range maxRetries {
		if i > 0 {
			logging.Warn("Retrying image pull (%d/%d) with exponential backoff for %s after error: %v", i, maxRetries-1, img, lastErr)
			if err := r.sleepFunc(ctx, time.Duration(1<<uint(i))*backoffBase); err != nil {
				return err
			}
		}

		if pullPolicy == "missing" {
			// In containerd, there's no direct "ImageInspect" like Docker.
			// We check if image exists in the image store.
			is := r.client.ImageService()
			_, err := is.Get(ctx, img)
			if err == nil {
				return nil // Image exists locally
			}
			// If not found, continue to pull
		}

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

// CreateContainer creates a new container based on the provided config.
func (r *ContainerdRuntime) CreateContainer(ctx context.Context, config *container.ContainerConfig) (string, error) {
	if config.Network != "" && config.Network != "none" && config.Network != "bridge" {
		return "", fmt.Errorf("network configuration %q is not supported yet in containerd runtime", config.Network)
	}

	img, err := r.client.GetImage(ctx, config.Image)
	if err != nil {
		return "", fmt.Errorf("failed to get image: %w", err)
	}

	id := uuid.New().String()

	var opts []oci.SpecOpts
	opts = append(opts, oci.WithImageConfig(img))
	if config.Workdir != "" {
		opts = append(opts, oci.WithProcessCwd(config.Workdir))
	}
	if len(config.Command) > 0 {
		opts = append(opts, oci.WithProcessArgs(config.Command...))
	}
	if len(config.Env) > 0 {
		opts = append(opts, oci.WithEnv(config.Env))
	}
	if config.User != "" {
		opts = append(opts, oci.WithUser(config.User))
	}
	if config.TTY {
		opts = append(opts, oci.WithTTY)
	}

	for _, m := range config.Mounts {
		mOpts := []string{"bind"}
		if m.ReadOnly {
			mOpts = append(mOpts, "ro")
		} else {
			mOpts = append(mOpts, "rw")
		}
		opts = append(opts, oci.WithMounts([]specs.Mount{
			{
				Type:        m.Type,
				Source:      m.Source,
				Destination: m.Target,
				Options:     mOpts,
			},
		}))
	}

	if config.Privileged {
		opts = append(opts, oci.WithPrivileged)
	}
	if len(config.CapAdd) > 0 {
		opts = append(opts, oci.WithAddedCapabilities(config.CapAdd))
	}
	if len(config.CapDrop) > 0 {
		opts = append(opts, oci.WithDroppedCapabilities(config.CapDrop))
	}

	container, err := r.client.NewContainer(
		ctx,
		id,
		client.WithImage(img),
		client.WithNewSnapshot(id, img),
		client.WithNewSpec(opts...),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	return container.ID(), nil
}

// StartContainer starts a created container by creating a task.
// Attachment must happen before StartContainer for cderun to capture IO.
// However, the interface has StartContainer separate.
// In containerd, Start actually starts the task.
func (r *ContainerdRuntime) StartContainer(ctx context.Context, containerID string) error {
	r.mu.Lock()
	task, ok := r.tasks[containerID]
	r.mu.Unlock()

	if !ok {
		return fmt.Errorf("task not found for container %s. Attachment must happen before Start", containerID)
	}

	return task.Start(ctx)
}

// WaitContainer waits for a container to exit and returns its exit code.
func (r *ContainerdRuntime) WaitContainer(ctx context.Context, containerID string) (int, error) {
	r.mu.RLock()
	task, ok := r.tasks[containerID]
	r.mu.RUnlock()

	if !ok {
		return 0, fmt.Errorf("task not found for container %s", containerID)
	}

	exitStatusC, err := task.Wait(ctx)
	if err != nil {
		return 0, err
	}
	status := <-exitStatusC
	return int(status.ExitCode()), status.Error()
}

// RemoveContainer removes a container and its task.
func (r *ContainerdRuntime) RemoveContainer(ctx context.Context, containerID string) error {
	r.mu.Lock()
	task, hasTask := r.tasks[containerID]
	io := r.taskIOs[containerID]
	delete(r.tasks, containerID)
	delete(r.taskIOs, containerID)
	delete(r.ioWait, containerID)
	r.mu.Unlock()

	if hasTask {
		_, _ = task.Delete(ctx) //nolint:errcheck
	}
	if io != nil {
		_ = io.Close() //nolint:errcheck
	}

	container, err := r.client.LoadContainer(ctx, containerID)
	if err != nil {
		return err
	}

	return container.Delete(ctx, client.WithSnapshotCleanup)
}

// AttachContainer attaches to a container's IO streams by creating a task.
func (r *ContainerdRuntime) AttachContainer(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error {
	container, err := r.client.LoadContainer(ctx, containerID)
	if err != nil {
		return err
	}

	ioWait := make(chan struct{})
	r.mu.Lock()
	r.ioWait[containerID] = ioWait
	r.mu.Unlock()

	var ioCreator cio.Creator
	if tty {
		ioCreator = cio.NewCreator(cio.WithStreams(stdin, stdout, stderr), cio.WithTerminal)
	} else {
		ioCreator = cio.NewCreator(cio.WithStreams(stdin, stdout, stderr))
	}

	task, err := container.NewTask(ctx, ioCreator)
	if err != nil {
		if ready != nil {
			close(ready)
		}
		return err
	}

	r.mu.Lock()
	r.tasks[containerID] = task
	r.taskIOs[containerID] = task.IO()
	r.mu.Unlock()

	if ready != nil {
		close(ready)
	}

	// Wait for task IO to finish or context to be cancelled
	waitC, err := task.Wait(ctx)
	if err != nil {
		return err
	}

	select {
	case <-waitC:
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}

// ResizeContainerTTY resizes the terminal of a container task.
func (r *ContainerdRuntime) ResizeContainerTTY(ctx context.Context, containerID string, rows, cols uint) error {
	r.mu.RLock()
	task, ok := r.tasks[containerID]
	r.mu.RUnlock()

	if !ok {
		return fmt.Errorf("task not found for container %s", containerID)
	}

	return task.Resize(ctx, uint32(cols), uint32(rows)) //nolint:gosec
}

// SignalContainer sends a signal to a container task.
func (r *ContainerdRuntime) SignalContainer(ctx context.Context, containerID string, sig string) error {
	r.mu.RLock()
	task, ok := r.tasks[containerID]
	r.mu.RUnlock()

	if !ok {
		return fmt.Errorf("task not found for container %s", containerID)
	}

	if sig == "" {
		sig = "SIGTERM"
	}

	s, err := signal.ParseSignal(sig)
	if err != nil {
		return fmt.Errorf("invalid signal %q: %w", sig, err)
	}

	return task.Kill(ctx, s)
}
