package runtime

import (
	"context"
	"fmt"
	"io"
	"math"
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
	defaultNamespace = "cderun"
)

// ContainerdRuntime implements the ContainerRuntime interface using containerd.
type ContainerdRuntime struct {
	client    *client.Client
	socket    string
	namespace string
	sleepFunc func(context.Context, time.Duration) error

	mu     sync.RWMutex
	ioMap  map[string]cio.Creator
	ioWait map[string]chan error

	closeOnce sync.Once
	closeErr  error
}

// NewContainerdRuntime creates a new containerd runtime instance.
func NewContainerdRuntime(socket string) (*ContainerdRuntime, error) {
	c, err := client.New(socket, client.WithDefaultNamespace(defaultNamespace))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to containerd: %w", err)
	}

	return &ContainerdRuntime{
		client:    c,
		socket:    socket,
		namespace: defaultNamespace,
		sleepFunc: SleepFunc,
		ioMap:     make(map[string]cio.Creator),
		ioWait:    make(map[string]chan error),
	}, nil
}

func (r *ContainerdRuntime) Close() error {
	r.closeOnce.Do(func() {
		r.closeErr = r.client.Close()
	})
	return r.closeErr
}

func (r *ContainerdRuntime) Name() string {
	return "containerd"
}

func (r *ContainerdRuntime) PullImage(ctx context.Context, img string, pullPolicy string, maxRetries int, backoffBase time.Duration) error {
	if pullPolicy == "never" {
		return nil
	}

	var lastErr error
	attempts := maxRetries
	for i := range attempts {
		if i > 0 {
			logging.Warn("Retrying image pull (%d/%d) with exponential backoff for %s after error: %v", i+1, maxRetries, img, lastErr)
			if err := r.sleepFunc(ctx, time.Duration(1<<uint(i))*backoffBase); err != nil {
				return err
			}
		}

		if pullPolicy == "missing" {
			is := r.client.ImageService()
			_, err := is.Get(ctx, img)
			if err == nil {
				return nil
			}
			if !errdefs.IsNotFound(err) {
				lastErr = err
				if IsRetryablePullError(err) {
					continue
				}
				return fmt.Errorf("failed to check image: %w", err)
			}
		}

		logging.Info("Pulling image %s...", img)
		_, err := r.client.Pull(ctx, img, client.WithPullUnpack)
		if err != nil {
			lastErr = err
			if IsRetryablePullError(err) {
				continue
			}
			return fmt.Errorf("failed to pull image: %w", err)
		}
		return nil
	}
	return fmt.Errorf("failed to pull image after %d attempts: %w", maxRetries, lastErr)
}

func (r *ContainerdRuntime) CreateContainer(ctx context.Context, config *container.ContainerConfig) (string, error) {
	if config.Memory < 0 {
		return "", fmt.Errorf("containerd runtime: negative memory limit %d is not supported", config.Memory)
	}
	if config.CPUs < 0 {
		return "", fmt.Errorf("containerd runtime: negative CPU limit %f is not supported", config.CPUs)
	}

	var quota int64
	var period uint64
	if config.CPUs > 0 {
		period = 100000
		quota = int64(config.CPUs * float64(period))
		if quota <= 0 {
			return "", fmt.Errorf("containerd runtime: CPU quota %d derived from CPUs %f is too small", quota, config.CPUs)
		}
	}

	if config.Network != "" && config.Network != "host" && config.Network != "bridge" {
		return "", fmt.Errorf("containerd runtime: Network %q is not supported yet", config.Network)
	}
	if len(config.Ports) > 0 || config.PublishAll || len(config.Expose) > 0 {
		return "", fmt.Errorf("containerd runtime: port mapping is not supported yet")
	}

	img, err := r.client.GetImage(ctx, config.Image)
	if err != nil {
		return "", fmt.Errorf("failed to get image: %w", err)
	}

	id := uuid.New().String()
	var args []string
	if len(config.Entrypoint) > 0 {
		args = append([]string{}, config.Entrypoint...)
		args = append(args, config.Command...)
	} else {
		args = config.Command
	}

	opts := []oci.SpecOpts{
		oci.WithDefaultSpec(),
		oci.WithImageConfig(img),
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
		mountType := m.Type
		if mountType == "" {
			mountType = "bind"
		}
		var mountOptions []string
		if m.ReadOnly {
			mountOptions = append(mountOptions, "ro")
		} else {
			mountOptions = append(mountOptions, "rw")
		}
		if mountType == "bind" {
			mountOptions = append(mountOptions, "rbind")
		}
		opts = append(opts, oci.WithMounts([]specs.Mount{
			{
				Type:        mountType,
				Source:      m.Source,
				Destination: m.Target,
				Options:     mountOptions,
			},
		}))
	}

	if config.Privileged {
		opts = append(opts, oci.WithPrivileged)
	}
	if config.Memory > 0 {
		opts = append(opts, oci.WithMemoryLimit(uint64(config.Memory)))
	}
	if config.CPUs > 0 {
		opts = append(opts, oci.WithCPUCFS(quota, period))
	}
	if config.Hostname != "" {
		opts = append(opts, oci.WithHostname(config.Hostname))
	}
	if config.Network == "host" {
		opts = append(opts, oci.WithHostNamespace(specs.NetworkNamespace))
	}

	_, err = r.client.NewContainer(ctx, id,
		client.WithImage(img),
		client.WithNewSnapshot(id, img),
		client.WithNewSpec(opts...),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create: %w", err)
	}
	return id, nil
}

func (r *ContainerdRuntime) StartContainer(ctx context.Context, containerID string) error {
	container, err := r.client.LoadContainer(ctx, containerID)
	if err != nil {
		return err
	}

	r.mu.Lock()
	creator, ok := r.ioMap[containerID]
	delete(r.ioMap, containerID)
	r.mu.Unlock()

	if !ok {
		creator = cio.NullIO
	}

	task, err := container.NewTask(ctx, creator)
	if err != nil {
		return fmt.Errorf("failed to create task: %w", err)
	}

	started := false
	defer func() {
		if !started {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if _, delErr := task.Delete(cleanupCtx, client.WithProcessKill); delErr != nil && !errdefs.IsNotFound(delErr) {
				logging.Warn("failed to cleanup task for container %s after start failure: %v", containerID, delErr)
			}
		}
	}()

	if err := task.Start(ctx); err != nil {
		return fmt.Errorf("failed to start task: %w", err)
	}
	started = true
	return nil
}

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
		r.mu.RLock()
		waitC, ok := r.ioWait[containerID]
		r.mu.RUnlock()
		if ok {
			waitC <- status.Error()
		}
		return int(status.ExitCode()), status.Error()
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

func (r *ContainerdRuntime) RemoveContainer(ctx context.Context, containerID string) error {
	cCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	r.mu.Lock()
	delete(r.ioWait, containerID)
	r.mu.Unlock()

	container, err := r.client.LoadContainer(cCtx, containerID)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return nil
		}
		return err
	}

	task, err := container.Task(cCtx, nil)
	if err == nil {
		if _, delErr := task.Delete(cCtx, client.WithProcessKill); delErr != nil && !errdefs.IsNotFound(delErr) {
			logging.Warn("failed to delete task during container removal (best-effort): %v", delErr)
		}
	}

	return container.Delete(cCtx, client.WithSnapshotCleanup)
}

func (r *ContainerdRuntime) SignalContainer(ctx context.Context, containerID string, sig string) error {
	container, err := r.client.LoadContainer(ctx, containerID)
	if err != nil {
		return err
	}
	task, err := container.Task(ctx, nil)
	if err != nil {
		return err
	}

	parsedSig, err := parseSignal(sig)
	if err != nil {
		return err
	}

	return task.Kill(ctx, parsedSig)
}

func (r *ContainerdRuntime) ResizeContainerTTY(ctx context.Context, containerID string, rows, cols uint) error {
	if rows > math.MaxUint32 || cols > math.MaxUint32 {
		return fmt.Errorf("terminal size exceeds maximum: rows=%d, cols=%d", rows, cols)
	}

	container, err := r.client.LoadContainer(ctx, containerID)
	if err != nil {
		return err
	}
	task, err := container.Task(ctx, nil)
	if err != nil {
		return err
	}
	return task.Resize(ctx, uint32(cols), uint32(rows))
}

func (r *ContainerdRuntime) AttachContainer(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error {
	var creator cio.Creator
	if tty {
		creator = cio.NewCreator(cio.WithStreams(stdin, stdout, stderr), cio.WithTerminal)
	} else {
		creator = cio.NewCreator(cio.WithStreams(stdin, stdout, stderr))
	}

	waitC := make(chan error, 1)
	r.mu.Lock()
	r.ioMap[containerID] = creator
	r.ioWait[containerID] = waitC
	r.mu.Unlock()

	defer func() {
		r.mu.Lock()
		delete(r.ioMap, containerID)
		delete(r.ioWait, containerID)
		r.mu.Unlock()
	}()

	if ready != nil {
		close(ready)
	}

	select {
	case err := <-waitC:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *ContainerdRuntime) InspectContainer(ctx context.Context, containerID string) (bool, int, error) {
	container, err := r.client.LoadContainer(ctx, containerID)
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
	return status.Status == client.Running, int(status.ExitStatus), nil
}

func parseSignal(sig string) (syscall.Signal, error) {
	if n, err := strconv.Atoi(sig); err == nil {
		if n <= 0 || n > 64 {
			return 0, fmt.Errorf("invalid signal number: %d", n)
		}
		return syscall.Signal(n), nil
	}

	name := strings.ToUpper(sig)
	if stripped, ok := strings.CutPrefix(name, "SIG"); ok {
		name = stripped
	}

	signals := map[string]syscall.Signal{
		"HUP": syscall.SIGHUP, "INT": syscall.SIGINT, "QUIT": syscall.SIGQUIT, "ILL": syscall.SIGILL,
		"TRAP": syscall.SIGTRAP, "ABRT": syscall.SIGABRT, "BUS": syscall.SIGBUS, "FPE": syscall.SIGFPE,
		"KILL": syscall.SIGKILL, "USR1": syscall.SIGUSR1, "SEGV": syscall.SIGSEGV, "USR2": syscall.SIGUSR2,
		"PIPE": syscall.SIGPIPE, "ALRM": syscall.SIGALRM, "TERM": syscall.SIGTERM, "CHLD": syscall.SIGCHLD,
		"CONT": syscall.SIGCONT, "STOP": syscall.SIGSTOP, "TSTP": syscall.SIGTSTP, "TTIN": syscall.SIGTTIN,
		"TTOU": syscall.SIGTTOU, "URG": syscall.SIGURG, "XCPU": syscall.SIGXCPU, "XFSZ": syscall.SIGXFSZ,
		"VTALRM": syscall.SIGVTALRM, "PROF": syscall.SIGPROF, "WINCH": syscall.SIGWINCH, "IO": syscall.SIGIO,
		"PWR": syscall.SIGPWR, "SYS": syscall.SIGSYS,
	}

	if s, ok := signals[name]; ok {
		return s, nil
	}
	return 0, fmt.Errorf("unsupported signal: %q", sig)
}
