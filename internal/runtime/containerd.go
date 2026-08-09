//go:build linux

package runtime

import (
	"context"
	"fmt"
	"io"
	"maps"
	"math"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"cderun/internal/container"
	"cderun/internal/logging"

	"github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/containers"
	"github.com/containerd/containerd/v2/pkg/cio"
	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/containerd/errdefs"
	"github.com/google/uuid"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
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
	logger    *logging.Logger
	sleepFunc func(context.Context, time.Duration) error

	mu        sync.RWMutex
	ioMap     map[string]cio.Creator
	ioWait    map[string]chan error
	taskReady map[string]chan struct{}

	closeOnce sync.Once
	closeErr  error
}

// ContainerdRuntimeOption defines a functional option for ContainerdRuntime.
type ContainerdRuntimeOption func(*ContainerdRuntime)

// WithContainerdLogger sets the logger for the containerd runtime.
func WithContainerdLogger(l *logging.Logger) ContainerdRuntimeOption {
	return func(rt *ContainerdRuntime) {
		rt.logger = l
	}
}

// NewContainerdRuntime creates a new containerd runtime instance.
func NewContainerdRuntime(socket string, opts ...ContainerdRuntimeOption) (*ContainerdRuntime, error) {
	c, err := client.New(socket, client.WithDefaultNamespace(defaultNamespace))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to containerd: %w", err)
	}

	rt := &ContainerdRuntime{
		client:    c,
		socket:    socket,
		namespace: defaultNamespace,
		sleepFunc: SleepFunc,
		ioMap:     make(map[string]cio.Creator),
		ioWait:    make(map[string]chan error),
		taskReady: make(map[string]chan struct{}),
	}

	for _, opt := range opts {
		opt(rt)
	}

	if rt.logger == nil {
		rt.logger = logging.GetGlobalLogger()
	}

	return rt, nil
}

func (r *ContainerdRuntime) notifyWait(containerID string, err error) {
	r.mu.RLock()
	waitC, ok := r.ioWait[containerID]
	r.mu.RUnlock()
	if ok {
		// Non-blocking send
		select {
		case waitC <- err:
		default:
		}
	}
}

func (r *ContainerdRuntime) notifyTaskReady(containerID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if readyC, ok := r.taskReady[containerID]; ok {
		select {
		case <-readyC:
			// Already closed
		default:
			close(readyC)
		}
		delete(r.taskReady, containerID)
	}
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

// ValidateConfig validates that the runtime supports the given container configuration.
func (r *ContainerdRuntime) ValidateConfig(config *container.ContainerConfig) error {
	if config.Init {
		return fmt.Errorf("containerd runtime: init is not supported yet")
	}

	if math.IsNaN(config.CPUs) || math.IsInf(config.CPUs, 0) {
		return fmt.Errorf("containerd runtime: non-finite CPU limit %f is not supported", config.CPUs)
	}

	if config.Memory < 0 {
		return fmt.Errorf("containerd runtime: negative memory limit %d is not supported", config.Memory)
	}
	if config.CPUs < 0 {
		return fmt.Errorf("containerd runtime: negative CPU limit %f is not supported", config.CPUs)
	}

	if config.CPUs > 0 {
		period := uint64(100000)
		quota := int64(config.CPUs * float64(period))
		if quota <= 0 {
			return fmt.Errorf("containerd runtime: CPU quota %d derived from CPUs %f is too small", quota, config.CPUs)
		}
	}

	if config.Network != "" && config.Network != "host" {
		return fmt.Errorf("containerd runtime: Network %q is not supported yet (only \"host\" is supported for containerd)", config.Network)
	}
	if len(config.Ports) > 0 || config.PublishAll || len(config.Expose) > 0 {
		return fmt.Errorf("containerd runtime: port mapping is not supported yet")
	}
	if len(config.DNS) > 0 {
		return fmt.Errorf("containerd runtime: DNS setting is not supported yet")
	}
	if len(config.AddHosts) > 0 {
		return fmt.Errorf("containerd runtime: add-host is not supported yet")
	}

	for _, m := range config.Mounts {
		if m.Type == "volume" {
			return fmt.Errorf("containerd runtime: volume mount type is not supported")
		}
		if m.Type != "" && m.Type != "bind" && m.Type != "tmpfs" {
			return fmt.Errorf("containerd runtime: unsupported mount type %q", m.Type)
		}
	}

	if len(config.GroupAdd) > 0 {
		for _, g := range config.GroupAdd {
			_, err := strconv.ParseUint(g, 10, 32)
			if err != nil {
				return fmt.Errorf("containerd runtime: non-numeric GroupAdd GID %q is not supported: %w", g, err)
			}
		}
	}

	return nil
}

func (r *ContainerdRuntime) PullImage(ctx context.Context, img string, pullPolicy string, maxRetries int, backoffBase time.Duration) error {
	if maxRetries < 0 {
		maxRetries = 0
	}
	if pullPolicy == "never" {
		return nil
	}

	var lastErr error
	attempts := maxRetries + 1
	for i := range attempts {
		if i > 0 {
			r.logger.Warn("Retrying image pull %d/%d with exponential backoff for %s after error: %v", i, maxRetries, img, lastErr)
			if err := r.sleepFunc(ctx, time.Duration(1<<uint(i-1))*backoffBase); err != nil {
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

		r.logger.Debug("Pulling image %s...", img)
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
	return fmt.Errorf("failed to pull image after %d attempts: %w", attempts, lastErr)
}

func (r *ContainerdRuntime) CreateContainer(ctx context.Context, config *container.ContainerConfig) (string, error) {
	if err := r.ValidateConfig(config); err != nil {
		return "", err
	}

	var quota int64
	var period uint64
	if config.CPUs > 0 {
		period = 100000
		quota = int64(config.CPUs * float64(period))
	}

	var validatedGids []uint32
	if len(config.GroupAdd) > 0 {
		validatedGids = make([]uint32, 0, len(config.GroupAdd))
		for _, g := range config.GroupAdd {
			gid64, err := strconv.ParseUint(g, 10, 32)
			if err != nil {
				return "", fmt.Errorf("containerd runtime: invalid GroupAdd GID %q: %w", g, err)
			}
			validatedGids = append(validatedGids, uint32(gid64))
		}
	}

	img, err := r.client.GetImage(ctx, config.Image)
	if err != nil {
		return "", fmt.Errorf("failed to get image: %w", err)
	}

	id := uuid.New().String()
	args, err := resolveProcessArgs(ctx, config, func(ctx context.Context) (ocispec.Image, error) {
		return img.Spec(ctx)
	})
	if err != nil {
		return "", err
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

		source := m.Source
		var mountOptions []string
		if m.ReadOnly {
			mountOptions = append(mountOptions, "ro")
		} else {
			mountOptions = append(mountOptions, "rw")
		}

		switch mountType {
		case "bind":
			mountOptions = append(mountOptions, "rbind")
		case "tmpfs":
			source = "tmpfs"
		}

		opts = append(opts, oci.WithMounts([]specs.Mount{
			{
				Type:        mountType,
				Source:      source,
				Destination: m.Target,
				Options:     mountOptions,
			},
		}))
	}

	if config.Privileged {
		opts = append(opts, oci.WithPrivileged)
	}
	if len(config.CapAdd) > 0 {
		opts = append(opts, oci.WithAddedCapabilities(normalizeCapabilities(config.CapAdd)))
	}
	if len(config.CapDrop) > 0 {
		opts = append(opts, oci.WithDroppedCapabilities(normalizeCapabilities(config.CapDrop)))
	}
	if config.Memory > 0 {
		opts = append(opts, oci.WithMemoryLimit(uint64(config.Memory)))
	}
	if config.CPUs > 0 {
		opts = append(opts, oci.WithCPUCFS(quota, period))
	}

	if config.Devices != nil {
		for _, d := range config.Devices {
			opts = append(opts, oci.WithDevices(d.PathOnHost, d.PathInContainer, d.CgroupPermissions))
		}
	}

	if config.Hostname != "" {
		opts = append(opts, oci.WithHostname(config.Hostname))
	}

	if config.ReadOnly {
		opts = append(opts, func(ctx context.Context, _ oci.Client, _ *containers.Container, s *specs.Spec) error {
			if s.Root == nil {
				s.Root = &specs.Root{}
			}
			s.Root.Readonly = true
			return nil
		})
	}
	if len(config.Ulimits) > 0 {
		opts = append(opts, func(ctx context.Context, _ oci.Client, _ *containers.Container, s *specs.Spec) error {
			if s.Process == nil {
				s.Process = &specs.Process{}
			}
			for _, u := range config.Ulimits {
				rlimitType := strings.ToUpper(u.Name)
				if !strings.HasPrefix(rlimitType, "RLIMIT_") {
					rlimitType = "RLIMIT_" + rlimitType
				}
				var hardVal uint64
				if u.Hard < 0 {
					hardVal = math.MaxUint64
				} else {
					hardVal = uint64(u.Hard)
				}
				var softVal uint64
				if u.Soft < 0 {
					softVal = math.MaxUint64
				} else {
					softVal = uint64(u.Soft)
				}
				s.Process.Rlimits = append(s.Process.Rlimits, specs.POSIXRlimit{
					Type: rlimitType,
					Hard: hardVal,
					Soft: softVal,
				})
			}
			return nil
		})
	}
	if config.Network == "host" {
		opts = append(opts, oci.WithHostNamespace(specs.NetworkNamespace))
	}
	if config.Pid == "host" {
		opts = append(opts, oci.WithHostNamespace(specs.PIDNamespace))
	}

	if len(config.GroupAdd) > 0 {
		opts = append(opts, func(ctx context.Context, _ oci.Client, _ *containers.Container, s *specs.Spec) error {
			if s.Process == nil {
				s.Process = &specs.Process{}
			}
			s.Process.User.AdditionalGids = append(s.Process.User.AdditionalGids, validatedGids...)
			return nil
		})
	}

	if len(config.Sysctls) > 0 {
		opts = append(opts, func(ctx context.Context, _ oci.Client, _ *containers.Container, s *specs.Spec) error {
			if s.Linux == nil {
				s.Linux = &specs.Linux{}
			}
			if s.Linux.Sysctl == nil {
				s.Linux.Sysctl = make(map[string]string)
			}
			maps.Copy(s.Linux.Sysctl, config.Sysctls)
			return nil
		})
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
		r.notifyWait(containerID, err)
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
		r.notifyWait(containerID, err)
		return fmt.Errorf("failed to create task: %w", err)
	}

	r.notifyTaskReady(containerID)

	started := false
	defer func() {
		if !started {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if _, delErr := task.Delete(cleanupCtx, client.WithProcessKill); delErr != nil && !errdefs.IsNotFound(delErr) {
				r.logger.Warn("failed to cleanup task for container %s after start failure: %v", containerID, delErr)
			}
		}
	}()

	if err := task.Start(ctx); err != nil {
		r.notifyWait(containerID, err)
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
		r.notifyWait(containerID, status.Error())
		return int(status.ExitCode()), status.Error()
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

func (r *ContainerdRuntime) RemoveContainer(ctx context.Context, containerID string) error {
	cCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
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
			r.logger.Warn("failed to delete task during container removal (best-effort): %v", delErr)
		}
	}

	return container.Delete(cCtx, client.WithSnapshotCleanup)
}

func (r *ContainerdRuntime) SignalContainer(ctx context.Context, containerID string, sig string) error {
	if sig != "" && !signalRegex.MatchString(sig) {
		return fmt.Errorf("invalid signal: %q", sig)
	}

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
	if r.taskReady == nil {
		r.taskReady = make(map[string]chan struct{})
	}
	readyC, ok := r.taskReady[containerID]
	if !ok {
		readyC = make(chan struct{})
		r.taskReady[containerID] = readyC
	}
	r.mu.Unlock()

	// Check if task is already ready (e.g., if StartContainer was called first)
	taskAlreadyReady := false
	if container, err := r.client.LoadContainer(ctx, containerID); err == nil {
		if _, err := container.Task(ctx, nil); err == nil {
			taskAlreadyReady = true
		}
	}

	attachCtx, cancelAttach := context.WithCancel(ctx)

	go func() {
		if !taskAlreadyReady {
			select {
			case <-readyC:
			case <-attachCtx.Done():
				return
			}
		}

		container, err := r.client.LoadContainer(attachCtx, containerID)
		if err != nil {
			if !errdefs.IsNotFound(err) {
				r.notifyWait(containerID, err)
			}
			return
		}
		task, err := container.Task(attachCtx, nil)
		if err != nil {
			if !errdefs.IsNotFound(err) {
				r.notifyWait(containerID, err)
			}
			return
		}
		exitStatusC, err := task.Wait(attachCtx)
		if err != nil {
			r.notifyWait(containerID, err)
			return
		}
		select {
		case status := <-exitStatusC:
			r.notifyWait(containerID, status.Error())
			return
		case <-attachCtx.Done():
			return
		}
	}()

	defer func() {
		cancelAttach()
		r.mu.Lock()
		delete(r.ioMap, containerID)
		delete(r.ioWait, containerID)
		if readyC, ok := r.taskReady[containerID]; ok {
			select {
			case <-readyC:
				// Already closed
			default:
				close(readyC)
			}
			delete(r.taskReady, containerID)
		}
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

// normalizeCapabilities converts Docker-style capability names (e.g. "SYS_ADMIN")
// to the CAP_-prefixed form the OCI spec expects. Already-prefixed values are preserved.
func normalizeCapabilities(caps []string) []string {
	normalized := make([]string, len(caps))
	for i, c := range caps {
		name := strings.ToUpper(strings.TrimSpace(c))
		if !strings.HasPrefix(name, "CAP_") {
			name = "CAP_" + name
		}
		normalized[i] = name
	}
	return normalized
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

func resolveProcessArgs(ctx context.Context, config *container.ContainerConfig, getSpec func(context.Context) (ocispec.Image, error)) ([]string, error) {
	var args []string
	if len(config.Entrypoint) > 0 {
		args = append([]string{}, config.Entrypoint...)
		args = append(args, config.Command...)
	} else if len(config.Command) > 0 {
		imageSpec, err := getSpec(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get image spec: %w", err)
		}
		args = append([]string{}, imageSpec.Config.Entrypoint...)
		args = append(args, config.Command...)
	}
	return args, nil
}
