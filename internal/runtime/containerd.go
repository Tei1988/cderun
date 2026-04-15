package runtime

import (
	"context"
	"fmt"
	"io"
	"sync"
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

type ContainerdRuntime struct {
	client *client.Client
	socket string

	// map from container ID to task info
	mu      sync.Mutex
	ioMap   map[string]cio.Creator
	taskMap map[string]client.Task
	exitMap map[string]chan uint32
}

func NewContainerdRuntime(socket string) (ContainerRuntime, error) {
	c, err := client.New(socket, client.WithDefaultNamespace("default"))
	if err != nil {
		return nil, fmt.Errorf("failed to create containerd client: %w", err)
	}
	return &ContainerdRuntime{
		client:  c,
		socket:  socket,
		ioMap:   make(map[string]cio.Creator),
		taskMap: make(map[string]client.Task),
		exitMap: make(map[string]chan uint32),
	}, nil
}

func (r *ContainerdRuntime) Name() string {
	return "containerd"
}

func (r *ContainerdRuntime) Close() error {
	return r.client.Close()
}

func (r *ContainerdRuntime) PullImage(ctx context.Context, img string, pullPolicy string, maxRetries int, backoffBase time.Duration) error {
	if pullPolicy == "never" {
		return nil
	}

	if pullPolicy == "missing" {
		_, err := r.client.GetImage(ctx, img)
		if err == nil {
			return nil
		}
		if !errdefs.IsNotFound(err) {
			return fmt.Errorf("failed to check image: %w", err)
		}
	}

	var lastErr error
	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			logging.Warn("Retrying image pull (%d/%d) for %s after error: %v", i, maxRetries-1, img, lastErr)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(1<<i) * backoffBase):
			}
		}

		logging.Info("Pulling image %s...", img)
		_, err := r.client.Pull(ctx, img, client.WithPullUnpack)
		if err == nil {
			return nil
		}
		lastErr = err
	}
	return fmt.Errorf("failed to pull image after %d attempts: %w", maxRetries, lastErr)
}

func (r *ContainerdRuntime) CreateContainer(ctx context.Context, config *container.ContainerConfig) (string, error) {
	img, err := r.client.GetImage(ctx, config.Image)
	if err != nil {
		return "", fmt.Errorf("failed to get image: %w", err)
	}

	id := uuid.New().String()

	var opts []oci.SpecOpts
	opts = append(opts, oci.WithImageConfig(img))
	opts = append(opts, oci.WithProcessArgs(config.Command...))

	if config.Workdir != "" {
		opts = append(opts, oci.WithProcessCwd(config.Workdir))
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

	if config.Privileged {
		opts = append(opts, oci.WithPrivileged)
	}

	for _, m := range config.Mounts {
		if m.Type == "bind" {
			var mountOpts []string
			if m.ReadOnly {
				mountOpts = append(mountOpts, "ro")
			} else {
				mountOpts = append(mountOpts, "rw")
			}
			mountOpts = append(mountOpts, "rbind")
			opts = append(opts, oci.WithMounts([]specs.Mount{
				{
					Source:      m.Source,
					Destination: m.Target,
					Type:        "bind",
					Options:     mountOpts,
				},
			}))
		}
	}

	// containerd implementation explicitly rejects non-empty network config as unsupported
	if config.Network != "" && config.Network != "bridge" && config.Network != "host" && config.Network != "none" {
		return "", fmt.Errorf("custom network %q is not supported by containerd runtime", config.Network)
	}

	// Default to host network if specified, but containerd needs more setup for bridge usually.
	// For simplicity and matching memory, we might just use Host namespace if requested.
	if config.Network == "host" {
		opts = append(opts, oci.WithHostNamespace(specs.NetworkNamespace))
	}

	c, err := r.client.NewContainer(ctx, id,
		client.WithImage(img),
		client.WithNewSpec(opts...),
		client.WithNewSnapshot(id, img),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	return c.ID(), nil
}

func (r *ContainerdRuntime) StartContainer(ctx context.Context, containerID string) error {
	c, err := r.client.LoadContainer(ctx, containerID)
	if err != nil {
		return fmt.Errorf("failed to load container: %w", err)
	}

	r.mu.Lock()
	creator, ok := r.ioMap[containerID]
	if !ok {
		creator = cio.NewCreator(cio.WithStdio)
	}
	r.mu.Unlock()

	task, err := c.NewTask(ctx, creator)
	if err != nil {
		return fmt.Errorf("failed to create task: %w", err)
	}

	r.mu.Lock()
	r.taskMap[containerID] = task
	exitChan := make(chan uint32, 1)
	r.exitMap[containerID] = exitChan
	r.mu.Unlock()

	exitStatusC, err := task.Wait(ctx)
	if err != nil {
		// Clean up if wait fails
		r.mu.Lock()
		delete(r.taskMap, containerID)
		delete(r.exitMap, containerID)
		r.mu.Unlock()
		task.Delete(ctx)
		return fmt.Errorf("failed to wait for task: %w", err)
	}

	go func() {
		select {
		case status := <-exitStatusC:
			exitChan <- status.ExitCode()
		case <-ctx.Done():
		}
	}()

	if err := task.Start(ctx); err != nil {
		return fmt.Errorf("failed to start task: %w", err)
	}

	return nil
}

func (r *ContainerdRuntime) WaitContainer(ctx context.Context, containerID string) (int, error) {
	r.mu.Lock()
	exitChan, ok := r.exitMap[containerID]
	r.mu.Unlock()

	if !ok {
		return 0, fmt.Errorf("container task not found or not started")
	}

	select {
	case code := <-exitChan:
		return int(code), nil
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

func (r *ContainerdRuntime) RemoveContainer(ctx context.Context, containerID string) error {
	c, err := r.client.LoadContainer(ctx, containerID)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to load container for removal: %w", err)
	}

	r.mu.Lock()
	task, hasTask := r.taskMap[containerID]
	r.mu.Unlock()

	if hasTask {
		// Try to delete task first
		_, _ = task.Delete(ctx, client.WithProcessKill)
	}

	err = c.Delete(ctx, client.WithSnapshotCleanup)
	if err != nil {
		if !errdefs.IsNotFound(err) {
			return fmt.Errorf("failed to delete container: %w", err)
		}
	}

	r.mu.Lock()
	delete(r.ioMap, containerID)
	delete(r.taskMap, containerID)
	delete(r.exitMap, containerID)
	r.mu.Unlock()

	return nil
}

func (r *ContainerdRuntime) AttachContainer(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error {
	// Prepare IO creator for NewTask
	var creator cio.Creator
	if tty {
		creator = cio.NewCreator(cio.WithStreams(stdin, stdout, stderr), cio.WithTerminal)
	} else {
		creator = cio.NewCreator(cio.WithStreams(stdin, stdout, stderr))
	}

	r.mu.Lock()
	r.ioMap[containerID] = creator
	r.mu.Unlock()

	if ready != nil {
		close(ready)
	}

	return nil
}

func (r *ContainerdRuntime) ResizeContainerTTY(ctx context.Context, containerID string, rows, cols uint) error {
	r.mu.Lock()
	task, ok := r.taskMap[containerID]
	r.mu.Unlock()

	if !ok {
		return fmt.Errorf("task not found for container %s", containerID)
	}

	return task.Resize(ctx, uint32(cols), uint32(rows))
}

func (r *ContainerdRuntime) SignalContainer(ctx context.Context, containerID string, sig string) error {
	r.mu.Lock()
	task, ok := r.taskMap[containerID]
	r.mu.Unlock()

	if !ok {
		return nil // Container might already be gone
	}

	s, err := parseSignal(sig)
	if err != nil {
		return err
	}

	err = task.Kill(ctx, s)
	if err != nil {
		if errdefs.IsNotFound(err) || errdefs.IsConflict(err) {
			return nil
		}
		return err
	}
	return nil
}

func (r *ContainerdRuntime) InspectContainer(ctx context.Context, containerID string) (bool, int, error) {
	c, err := r.client.LoadContainer(ctx, containerID)
	if err != nil {
		return false, 0, err
	}

	task, err := c.Task(ctx, nil)
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

	running := status.Status == client.Running || status.Status == client.Created
	return running, int(status.ExitStatus), nil
}

func parseSignal(sig string) (syscall.Signal, error) {
	switch sig {
	case "SIGKILL", "KILL", "9":
		return syscall.SIGKILL, nil
	case "SIGTERM", "TERM", "15":
		return syscall.SIGTERM, nil
	case "SIGINT", "INT", "2":
		return syscall.SIGINT, nil
	case "SIGHUP", "HUP", "1":
		return syscall.SIGHUP, nil
	case "SIGQUIT", "QUIT", "3":
		return syscall.SIGQUIT, nil
	case "":
		return syscall.SIGTERM, nil
	default:
		return 0, fmt.Errorf("unsupported signal: %q", sig)
	}
}
