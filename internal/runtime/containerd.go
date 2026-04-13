package runtime

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
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

const (
	containerdNamespace = "cderun"
)

// ContainerdRuntime implements ContainerRuntime using containerd client.
type ContainerdRuntime struct {
	client   *client.Client
	socket   string
	mu       sync.RWMutex
	creators map[string]cio.Creator
	waiters  map[string]chan error
}

// NewContainerdRuntime creates a new ContainerdRuntime instance.
func NewContainerdRuntime(socket string) (*ContainerdRuntime, error) {
	c, err := client.New(socket, client.WithDefaultNamespace(containerdNamespace))
	if err != nil {
		return nil, fmt.Errorf("failed to create containerd client: %w", err)
	}
	return &ContainerdRuntime{
		client:   c,
		creators: make(map[string]cio.Creator),
		waiters:  make(map[string]chan error),
		socket:   socket,
	}, nil
}

// Name returns the name of the runtime.
func (r *ContainerdRuntime) Name() string {
	return "containerd"
}

// Close closes the containerd client connection.
func (r *ContainerdRuntime) Close() error {
	if r.client == nil {
		return nil
	}
	return r.client.Close()
}

// PullImage pulls the specified image.
func (r *ContainerdRuntime) PullImage(ctx context.Context, img string, pullPolicy string, maxRetries int, backoffBase time.Duration) error {
	if pullPolicy == "never" {
		return nil
	}

	var lastErr error
	// The requirement is "up to PullMaxRetries retries", so total attempts = 1 + maxRetries.
	for i := 0; i <= maxRetries; i++ {
		if i > 0 {
			logging.Warn("Retrying image pull (%d/%d) with exponential backoff for %s after error: %v", i, maxRetries, img, lastErr)
			timer := time.NewTimer(time.Duration(1<<(i-1)) * backoffBase)
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

	return fmt.Errorf("failed to pull image after %d retries: %w", maxRetries, lastErr)
}

// CreateContainer creates a new container.
func (r *ContainerdRuntime) CreateContainer(ctx context.Context, config *container.ContainerConfig) (string, error) {

	if len(config.CapDrop) > 0 {
		return "", errors.New("containerd runtime: CapDrop is not supported yet")
	}
	if config.Network != "" {
		return "", fmt.Errorf("containerd runtime: Network %q is not supported yet", config.Network)
	}
	if config.Hostname != "" {
		return "", errors.New("containerd runtime: Hostname is not supported yet")
	}
	if len(config.DNS) > 0 {
		return "", errors.New("containerd runtime: DNS is not supported yet")
	}
	if len(config.AddHosts) > 0 {
		return "", errors.New("containerd runtime: AddHosts is not supported yet")
	}
	if len(config.Ports) > 0 || len(config.Expose) > 0 || config.PublishAll {
		return "", errors.New("containerd runtime: Port mapping is not supported yet")
	}

	image, err := r.client.GetImage(ctx, config.Image)
	if err != nil {
		return "", fmt.Errorf("failed to get image: %w", err)
	}

	containerID := uuid.New().String()

	var args []string
	if len(config.Entrypoint) > 0 {
		args = make([]string, 0, len(config.Entrypoint)+len(config.Command))
		args = append(args, config.Entrypoint...)
		args = append(args, config.Command...)
	} else {
		args = config.Command
	}

	opts := []oci.SpecOpts{
		oci.WithDefaultSpec(),
		oci.WithImageConfig(image),
	}

	if len(args) > 0 {
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
		resolvedType := m.Type
		if resolvedType == "" {
			resolvedType = "bind"
		}
		if resolvedType == "bind" {
			// rbind is preferred for cderun to ensure nested mounts are included
			mountOptions = append(mountOptions, "rbind")
		}
		opts = append(opts, oci.WithMounts([]specs.Mount{
			{
				Type:        resolvedType,
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
		opts = append(opts, oci.WithAddedCapabilities(config.CapAdd))
	}

	if config.Memory != 0 {
		if config.Memory < 0 {
			return "", fmt.Errorf("invalid memory limit: %d", config.Memory)
		}
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

	snapshotter := os.Getenv("CDERUN_CONTAINERD_SNAPSHOTTER")
	if snapshotter == "" {
		snapshotter = "overlayfs"
	}

	_, err = r.client.NewContainer(
		ctx,
		containerID,
		client.WithImage(image),
		client.WithSnapshotter(snapshotter),
		client.WithNewSnapshot(containerID, image),
		client.WithNewSpec(opts...),
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

	r.mu.RLock()
	creator, ok := r.creators[containerID]
	r.mu.RUnlock()
	if !ok {
		creator = cio.NewCreator(cio.WithStdio)
	}

	task, err := container.NewTask(ctx, creator)
	if err != nil {
		return fmt.Errorf("failed to create task: %w", err)
	}

	started := false
	defer func() {
		if !started {
			// Use background context for cleanup to ensure it runs even if original ctx was cancelled
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if _, delErr := task.Delete(cleanupCtx, client.WithProcessKill); delErr != nil {
				logging.Debug("failed to delete task on start failure: %v", delErr)
			}
		}
	}()

	if err := task.Start(ctx); err != nil {
		return fmt.Errorf("failed to start task: %w", err)
	}
	started = true

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

	r.mu.RLock()
	waiter := r.waiters[containerID]
	r.mu.RUnlock()

	select {
	case status := <-exitStatusC:
		if waiter != nil {
			waiter <- status.Error()
		}
		return int(status.ExitCode()), status.Error()
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

// RemoveContainer removes a container.
func (r *ContainerdRuntime) RemoveContainer(ctx context.Context, containerID string) error {
	// Use background context for cleanup to ensure it completes even if original ctx is canceled
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	container, err := r.client.LoadContainer(cleanupCtx, containerID)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to load container: %w", err)
	}

	task, err := container.Task(cleanupCtx, nil)
	if err == nil {
		if _, err := task.Delete(cleanupCtx, client.WithProcessKill); err != nil {
			if !errdefs.IsNotFound(err) {
				return fmt.Errorf("failed to delete task: %w", err)
			}
		}
	}

	if err := container.Delete(cleanupCtx, client.WithSnapshotCleanup); err != nil {
		if !errdefs.IsNotFound(err) {
			return fmt.Errorf("failed to delete container: %w", err)
		}
	}

	r.mu.Lock()
	delete(r.creators, containerID)
	delete(r.waiters, containerID)
	r.mu.Unlock()

	return nil
}

// AttachContainer attaches to a container's IO streams.
// Note: Unlike Docker's implementation, containerd uses a cio.Creator pattern.
// The I/O streams are actually connected when NewTask is called in StartContainer,
// so AttachContainer MUST be called before StartContainer.
func (r *ContainerdRuntime) AttachContainer(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error {
	ioOpts := []cio.Opt{
		cio.WithStreams(stdin, stdout, stderr),
	}
	if tty {
		ioOpts = append(ioOpts, cio.WithTerminal)
	}

	creator := cio.NewCreator(ioOpts...)
	waiter := make(chan error, 1)

	r.mu.Lock()
	r.creators[containerID] = creator
	r.waiters[containerID] = waiter
	r.mu.Unlock()

	if ready != nil {
		close(ready)
	}

	// Wait for the task to finish (signaled by WaitContainer)
	select {
	case err := <-waiter:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ResizeContainerTTY resizes the terminal of a container.
func (r *ContainerdRuntime) ResizeContainerTTY(ctx context.Context, containerID string, rows, cols uint) error {
	if rows > math.MaxUint32 || cols > math.MaxUint32 {
		return errors.New("terminal size overflow")
	}

	container, err := r.client.LoadContainer(ctx, containerID)
	if err != nil {
		return fmt.Errorf("failed to load container: %w", err)
	}

	task, err := container.Task(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to load task: %w", err)
	}

	return task.Resize(ctx, uint32(cols), uint32(rows))
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

var signalMap = map[string]syscall.Signal{
	"HUP":    syscall.SIGHUP,
	"INT":    syscall.SIGINT,
	"QUIT":   syscall.SIGQUIT,
	"ILL":    syscall.SIGILL,
	"TRAP":   syscall.SIGTRAP,
	"ABRT":   syscall.SIGABRT,
	"BUS":    syscall.SIGBUS,
	"FPE":    syscall.SIGFPE,
	"KILL":   syscall.SIGKILL,
	"USR1":   syscall.SIGUSR1,
	"SEGV":   syscall.SIGSEGV,
	"USR2":   syscall.SIGUSR2,
	"PIPE":   syscall.SIGPIPE,
	"ALRM":   syscall.SIGALRM,
	"TERM":   syscall.SIGTERM,
	"CHLD":   syscall.SIGCHLD,
	"CONT":   syscall.SIGCONT,
	"STOP":   syscall.SIGSTOP,
	"TSTP":   syscall.SIGTSTP,
	"TTIN":   syscall.SIGTTIN,
	"TTOU":   syscall.SIGTTOU,
	"URG":    syscall.SIGURG,
	"XCPU":   syscall.SIGXCPU,
	"XFSZ":   syscall.SIGXFSZ,
	"VTALRM": syscall.SIGVTALRM,
	"PROF":   syscall.SIGPROF,
	"WINCH":  syscall.SIGWINCH,
	"IO":     syscall.SIGIO,
	"PWR":    syscall.SIGPWR,
	"SYS":    syscall.SIGSYS,
}

func parseSignal(sig string) (syscall.Signal, error) {
	if sig == "" {
		return syscall.SIGTERM, nil
	}
	if n, err := strconv.Atoi(sig); err == nil {
		return syscall.Signal(n), nil
	}
	name := strings.TrimPrefix(strings.ToUpper(sig), "SIG")
	if s, ok := signalMap[name]; ok {
		return s, nil
	}
	return 0, fmt.Errorf("unsupported signal: %s", sig)
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
