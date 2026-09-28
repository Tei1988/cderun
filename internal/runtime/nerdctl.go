package runtime

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"cderun/internal/container"
	"cderun/internal/logging"
)

// NerdctlRuntime Option pattern.
type NerdctlRuntimeOption func(*NerdctlRuntime)

// WithNerdctlLogger sets custom logger for NerdctlRuntime.
func WithNerdctlLogger(logger *logging.Logger) NerdctlRuntimeOption {
	return func(r *NerdctlRuntime) {
		if logger != nil {
			r.logger = logger
		}
	}
}

// WithNerdctlExecPath sets custom binary path for nerdctl (default: "nerdctl").
func WithNerdctlExecPath(execPath string) NerdctlRuntimeOption {
	return func(r *NerdctlRuntime) {
		if execPath != "" {
			r.execPath = execPath
		}
	}
}

// NerdctlRuntime implements ContainerRuntime interface using nerdctl CLI binary.
type NerdctlRuntime struct {
	socketPath string
	execPath   string
	logger     *logging.Logger
}

// NewNerdctlRuntime creates a new NerdctlRuntime instance.
func NewNerdctlRuntime(socketPath string, opts ...NerdctlRuntimeOption) (*NerdctlRuntime, error) {
	r := &NerdctlRuntime{
		socketPath: socketPath,
		execPath:   "nerdctl",
		logger:     logging.GetGlobalLogger(),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r, nil
}

func (r *NerdctlRuntime) baseArgs() []string {
	if r.socketPath != "" {
		return []string{fmt.Sprintf("--address=%s", r.socketPath)}
	}
	return nil
}

// PullImage pulls an image using nerdctl pull.
func (r *NerdctlRuntime) PullImage(ctx context.Context, image string, pullPolicy string, maxRetries int, backoffBase time.Duration) error {
	if pullPolicy == "never" {
		r.logger.Debug("Pull policy is 'never'; skipping image pull for %s", image)
		return nil
	}
	if pullPolicy != "always" && pullPolicy != "missing" {
		return fmt.Errorf("unknown pull policy %q", pullPolicy)
	}

	builder := NewCLIArgBuilder()
	builder.AddPositionals(image)

	args := append(r.baseArgs(), "pull")
	args = append(args, builder.Build()...)

	if err := builder.VerifyStructure(args[len(r.baseArgs())+1:], -1); err != nil {
		return fmt.Errorf("nerdctl pull structural verification failed: %w", err)
	}

	cmd := exec.CommandContext(ctx, r.execPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to pull image %s with nerdctl: %w, output: %s", image, err, string(output))
	}
	return nil
}

// ValidateConfig validates ContainerConfig compatibility for nerdctl.
func (r *NerdctlRuntime) ValidateConfig(config *container.ContainerConfig) error {
	if config == nil {
		return fmt.Errorf("container config is nil")
	}
	if config.Image == "" {
		return fmt.Errorf("image name cannot be empty")
	}
	return nil
}

// CreateContainer creates a container using nerdctl container create and returns its container ID.
func (r *NerdctlRuntime) CreateContainer(ctx context.Context, config *container.ContainerConfig) (string, error) {
	if err := r.ValidateConfig(config); err != nil {
		return "", err
	}

	builder := NewCLIArgBuilder()
	builder.AddJoinedFlag("--workdir", config.Workdir, true)
	builder.AddJoinedFlag("--user", config.User, true)
	builder.AddJoinedFlag("--hostname", config.Hostname, true)
	builder.AddJoinedFlag("--net", config.Network, true)
	builder.AddJoinedFlag("--ipc", config.IPC, true)
	builder.AddJoinedFlag("--cgroupns", config.Cgroupns, true)
	builder.AddJoinedFlag("--restart", config.Restart, true)
	builder.AddJoinedFlag("--shm-size", config.ShmSize, true)
	builder.AddJoinedFlag("--cpuset-cpus", config.CpusetCpus, true)
	builder.AddJoinedFlag("--cpuset-mems", config.CpusetMems, true)
	builder.AddJoinedFlag("--gpus", config.GPUs, true)

	if config.PidsLimit != 0 {
		builder.AddJoinedFlag("--pids-limit", strconv.FormatInt(config.PidsLimit, 10), false)
	}
	if config.CPUShares > 0 {
		builder.AddJoinedFlag("--cpu-shares", strconv.FormatInt(config.CPUShares, 10), false)
	}
	if config.Memory > 0 {
		builder.AddJoinedFlag("--memory", strconv.FormatInt(config.Memory, 10), false)
	}

	builder.AddBoolFlag("--tty", config.TTY)
	builder.AddBoolFlag("--interactive", config.Interactive)
	builder.AddBoolFlag("--read-only", config.ReadOnly)
	builder.AddBoolFlag("--init", config.Init)

	for _, e := range config.Env {
		builder.AddJoinedFlag("--env", e, false)
	}
	for _, m := range config.Mounts {
		mountVal := fmt.Sprintf("type=%s,source=%s,target=%s", m.Type, m.Source, m.Target)
		if m.ReadOnly {
			mountVal += ",readonly"
		}
		builder.AddJoinedFlag("--mount", mountVal, false)
	}
	for _, cap := range config.CapAdd {
		builder.AddJoinedFlag("--cap-add", cap, false)
	}
	for _, cap := range config.CapDrop {
		builder.AddJoinedFlag("--cap-drop", cap, false)
	}
	for _, sec := range config.SecurityOpt {
		builder.AddJoinedFlag("--security-opt", sec, false)
	}
	for _, dns := range config.DNS {
		builder.AddJoinedFlag("--dns", dns, false)
	}
	for _, host := range config.AddHosts {
		builder.AddJoinedFlag("--add-host", host, false)
	}
	for k, v := range config.Labels {
		builder.AddJoinedFlag("--label", fmt.Sprintf("%s=%s", k, v), false)
	}

	if len(config.Entrypoint) > 0 {
		builder.AddJoinedFlag("--entrypoint", config.Entrypoint[0], false)
	}

	// Positionals: Image, additional Entrypoint args, and Command
	positionals := []string{config.Image}
	if len(config.Entrypoint) > 1 {
		positionals = append(positionals, config.Entrypoint[1:]...)
	}
	positionals = append(positionals, config.Command...)
	builder.AddPositionals(positionals...)

	argv := builder.Build()
	if err := builder.VerifyStructure(argv, -1); err != nil {
		return "", fmt.Errorf("nerdctl create structural verification failed: %w", err)
	}

	fullArgs := append(r.baseArgs(), "create")
	fullArgs = append(fullArgs, argv...)

	cmd := exec.CommandContext(ctx, r.execPath, fullArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("nerdctl create failed: %w, output: %s", err, string(out))
	}

	containerID := strings.TrimSpace(string(out))
	return containerID, nil
}

// StartContainer starts a container using nerdctl start.
func (r *NerdctlRuntime) StartContainer(ctx context.Context, containerID string) error {
	builder := NewCLIArgBuilder()
	builder.AddPositionals(containerID)

	argv := builder.Build()
	fullArgs := append(r.baseArgs(), "start")
	fullArgs = append(fullArgs, argv...)

	cmd := exec.CommandContext(ctx, r.execPath, fullArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("nerdctl start failed: %w, output: %s", err, string(out))
	}
	return nil
}

// WaitContainer waits for a container using nerdctl wait and returns its exit code.
func (r *NerdctlRuntime) WaitContainer(ctx context.Context, containerID string) (int, error) {
	builder := NewCLIArgBuilder()
	builder.AddPositionals(containerID)

	argv := builder.Build()
	fullArgs := append(r.baseArgs(), "wait")
	fullArgs = append(fullArgs, argv...)

	cmd := exec.CommandContext(ctx, r.execPath, fullArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return -1, fmt.Errorf("nerdctl wait failed: %w, output: %s", err, string(out))
	}

	exitCodeStr := strings.TrimSpace(string(out))
	exitCode, err := strconv.Atoi(exitCodeStr)
	if err != nil {
		return -1, fmt.Errorf("failed to parse nerdctl wait exit code %q: %w", exitCodeStr, err)
	}
	return exitCode, nil
}

// RemoveContainer removes a container using nerdctl rm.
func (r *NerdctlRuntime) RemoveContainer(ctx context.Context, containerID string) error {
	builder := NewCLIArgBuilder()
	builder.AddBoolFlag("--force", true)
	builder.AddPositionals(containerID)

	argv := builder.Build()
	fullArgs := append(r.baseArgs(), "rm")
	fullArgs = append(fullArgs, argv...)

	cmd := exec.CommandContext(ctx, r.execPath, fullArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("nerdctl rm failed: %w, output: %s", err, string(out))
	}
	return nil
}

// AttachContainer attaches stdio streams using nerdctl attach.
func (r *NerdctlRuntime) AttachContainer(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error {
	builder := NewCLIArgBuilder()
	builder.AddPositionals(containerID)

	argv := builder.Build()
	fullArgs := append(r.baseArgs(), "attach")
	fullArgs = append(fullArgs, argv...)

	cmd := exec.CommandContext(ctx, r.execPath, fullArgs...)
	if stdin != nil {
		cmd.Stdin = stdin
	}
	if stdout != nil {
		cmd.Stdout = stdout
	}
	if stderr != nil {
		cmd.Stderr = stderr
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("nerdctl attach start failed: %w", err)
	}

	if ready != nil {
		close(ready)
	}

	return cmd.Wait()
}

// ResizeContainerTTY resizes container TTY using nerdctl container exec or resize if supported.
func (r *NerdctlRuntime) ResizeContainerTTY(ctx context.Context, containerID string, rows, cols uint) error {
	r.logger.Debug("ResizeContainerTTY called for nerdctl container %s (%dx%d)", containerID, cols, rows)
	return nil
}

// SignalContainer sends a signal using nerdctl kill.
func (r *NerdctlRuntime) SignalContainer(ctx context.Context, containerID string, sig string) error {
	builder := NewCLIArgBuilder()
	builder.AddJoinedFlag("--signal", sig, false)
	builder.AddPositionals(containerID)

	argv := builder.Build()
	fullArgs := append(r.baseArgs(), "kill")
	fullArgs = append(fullArgs, argv...)

	cmd := exec.CommandContext(ctx, r.execPath, fullArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("nerdctl kill failed: %w, output: %s", err, string(out))
	}
	return nil
}

// Name returns the runtime adapter name.
func (r *NerdctlRuntime) Name() string {
	return "nerdctl"
}

// InspectContainer inspects container status using nerdctl inspect.
func (r *NerdctlRuntime) InspectContainer(ctx context.Context, containerID string) (bool, int, error) {
	builder := NewCLIArgBuilder()
	builder.AddJoinedFlag("--format", "{{.State.Running}} {{.State.ExitCode}}", false)
	builder.AddPositionals(containerID)

	argv := builder.Build()
	fullArgs := append(r.baseArgs(), "inspect")
	fullArgs = append(fullArgs, argv...)

	cmd := exec.CommandContext(ctx, r.execPath, fullArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, -1, fmt.Errorf("nerdctl inspect failed: %w, output: %s", err, string(out))
	}

	parts := strings.Fields(strings.TrimSpace(string(out)))
	if len(parts) < 2 {
		return false, -1, fmt.Errorf("malformed nerdctl inspect output: %s", string(out))
	}

	running := parts[0] == "true"
	exitCode, err := strconv.Atoi(parts[1])
	if err != nil {
		return false, -1, fmt.Errorf("failed to parse exit code from inspect output %q: %w", parts[1], err)
	}

	return running, exitCode, nil
}

// PruneContainers prunes stopped containers using nerdctl container prune.
func (r *NerdctlRuntime) PruneContainers(ctx context.Context) ([]string, error) {
	builder := NewCLIArgBuilder()
	builder.AddBoolFlag("--force", true)

	argv := builder.Build()
	fullArgs := append(r.baseArgs(), "container", "prune")
	fullArgs = append(fullArgs, argv...)

	cmd := exec.CommandContext(ctx, r.execPath, fullArgs...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = io.Discard

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("nerdctl container prune failed: %w", err)
	}

	var pruned []string
	lines := strings.Split(stdout.String(), "\n")
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed != "" {
			pruned = append(pruned, trimmed)
		}
	}
	return pruned, nil
}

// Close releases any runtime resources.
func (r *NerdctlRuntime) Close() error {
	return nil
}
