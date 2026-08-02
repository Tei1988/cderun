package runtime

import (
	"context"
	"errors"
	"fmt"
	"github.com/containerd/errdefs"
	"io"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/stdcopy"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"cderun/internal/container"
	"cderun/internal/logging"
)

type dockerClient interface {
	Close() error
	ImageInspect(ctx context.Context, imageID string, opts ...client.ImageInspectOption) (image.InspectResponse, error)
	ImagePull(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error)
	ContainerCreate(ctx context.Context, config *dockercontainer.Config, hostConfig *dockercontainer.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (dockercontainer.CreateResponse, error)
	ContainerStart(ctx context.Context, containerID string, options dockercontainer.StartOptions) error
	ContainerWait(ctx context.Context, containerID string, condition dockercontainer.WaitCondition) (<-chan dockercontainer.WaitResponse, <-chan error)
	ContainerRemove(ctx context.Context, containerID string, options dockercontainer.RemoveOptions) error
	ContainerResize(ctx context.Context, containerID string, options dockercontainer.ResizeOptions) error
	ContainerInspect(ctx context.Context, containerID string) (dockercontainer.InspectResponse, error)
	ContainerKill(ctx context.Context, containerID string, signal string) error
	ContainerAttach(ctx context.Context, container string, options dockercontainer.AttachOptions) (types.HijackedResponse, error)
}

var signalRegex = regexp.MustCompile(`^(?i)[A-Z0-9]+$`)

// DockerRuntime implements ContainerRuntime using Docker Engine API.
type DockerRuntime struct {
	client                dockerClient
	socket                string
	name                  string
	logger                *logging.Logger
	sleepFunc             func(context.Context, time.Duration) error
	attachCloseWriteGrace time.Duration
	closeOnce             sync.Once
	closeErr              error
	mu                    sync.Mutex
	removeOnExit          map[string]bool
}

// DockerRuntimeOption defines a functional option for DockerRuntime.
type DockerRuntimeOption func(*DockerRuntime)

// WithLogger sets the logger for the Docker runtime.
func WithLogger(l *logging.Logger) DockerRuntimeOption {
	return func(rt *DockerRuntime) {
		rt.logger = l
	}
}

// WithAttachCloseWriteGrace sets the grace period before closing the write side of an attached connection.
func WithAttachCloseWriteGrace(d time.Duration) DockerRuntimeOption {
	return func(rt *DockerRuntime) {
		if d <= 0 {
			d = 1 * time.Millisecond
		}
		rt.attachCloseWriteGrace = d
	}
}

// NewDockerRuntime creates a new Docker runtime instance with name "docker".
func NewDockerRuntime(socket string) (*DockerRuntime, error) {
	return NewDockerRuntimeWithOptions(socket, "docker", []client.Opt{client.WithAPIVersionNegotiation()})
}

// NewDockerRuntimeWithName creates a new Docker runtime instance with a specific name.
func NewDockerRuntimeWithName(socket string, name string) (*DockerRuntime, error) {
	return NewDockerRuntimeWithOptions(socket, name, []client.Opt{client.WithAPIVersionNegotiation()})
}

// NewDockerRuntimeWithOptions creates a new Docker runtime instance with specific client options and internal options.
func NewDockerRuntimeWithOptions(socket string, name string, clientOpts []client.Opt, rtOpts ...DockerRuntimeOption) (*DockerRuntime, error) {
	if socket == "" {
		return nil, fmt.Errorf("creating docker client: empty socket path")
	}

	opts := append([]client.Opt{client.WithHost("unix://" + socket)}, clientOpts...)

	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, fmt.Errorf("creating docker client: %w", err)
	}

	rt := &DockerRuntime{
		client:                cli,
		socket:                socket,
		name:                  name,
		attachCloseWriteGrace: 100 * time.Millisecond,
		sleepFunc:             SleepFunc,
		removeOnExit:          make(map[string]bool),
	}

	for _, opt := range rtOpts {
		opt(rt)
	}

	if rt.logger == nil {
		rt.logger = logging.GetGlobalLogger()
	}

	return rt, nil
}

// Close closes the DockerRuntime and releases associated resources.
func (d *DockerRuntime) Close() error {
	d.closeOnce.Do(func() {
		d.closeErr = d.client.Close()
	})
	return d.closeErr
}

// ValidateConfig validates that the runtime supports the given container configuration.
// Docker runtime natively supports all existing features, so this is a no-op.
func (d *DockerRuntime) ValidateConfig(config *container.ContainerConfig) error {
	return nil
}

// PullImage pulls the specified image based on the pull policy.
func (d *DockerRuntime) PullImage(ctx context.Context, img string, pullPolicy string, maxRetries int, backoffBase time.Duration) error {
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
			d.logger.Warn("Retrying image pull %d/%d with exponential backoff for %s after error: %v", i, maxRetries, img, lastErr)
			if err := d.sleepFunc(ctx, time.Duration(1<<uint(i-1))*backoffBase); err != nil {
				return err
			}
		}

		if pullPolicy == "missing" {
			_, err := d.client.ImageInspect(ctx, img)
			if err == nil {
				return nil // Image exists locally
			}
			if !errdefs.IsNotFound(err) {
				lastErr = err
				if IsRetryablePullError(err) {
					continue
				}
				return fmt.Errorf("failed to inspect image: %w", err)
			}
		}

		// Policy is "always" or "missing" (and not found locally)
		d.logger.Info("Pulling image %s...", img)
		reader, err := d.client.ImagePull(ctx, img, image.PullOptions{})
		if err != nil {
			lastErr = err
			if IsRetryablePullError(err) {
				continue
			}
			return fmt.Errorf("failed to pull image: %w", err)
		}

		// Wait for pull to complete and check for errors in the stream
		err = jsonmessage.DisplayJSONMessagesStream(reader, io.Discard, 0, false, nil)
		_ = reader.Close() //nolint:errcheck
		if err != nil {
			lastErr = err
			if IsRetryablePullError(err) {
				continue
			}
			return fmt.Errorf("failed to pull image (stream): %w", err)
		}

		return nil // Success
	}

	return fmt.Errorf("failed to pull image after %d attempts: %w", attempts, lastErr)
}

// CreateContainer creates a new container based on the provided config.
func (d *DockerRuntime) CreateContainer(ctx context.Context, config *container.ContainerConfig) (string, error) {
	containerConfig, hostConfig, networkingConfig, err := toDockerContainerConfig(config)
	if err != nil {
		return "", err
	}

	resp, err := d.client.ContainerCreate(ctx, containerConfig, hostConfig, networkingConfig, nil, "")
	if err != nil {
		return "", err
	}

	if config.Remove {
		d.mu.Lock()
		d.removeOnExit[resp.ID] = true
		d.mu.Unlock()
	}

	return resp.ID, nil
}

// StartContainer starts a created container.
func (d *DockerRuntime) StartContainer(ctx context.Context, containerID string) error {
	return d.client.ContainerStart(ctx, containerID, dockercontainer.StartOptions{})
}

// WaitContainer waits for a container to exit and returns its exit code.
func (d *DockerRuntime) WaitContainer(ctx context.Context, containerID string) (int, error) {
	d.logger.Trace("Waiting for container %s to stop...", containerID)

	d.mu.Lock()
	remove := d.removeOnExit[containerID]
	d.mu.Unlock()

	condition := dockercontainer.WaitConditionNotRunning
	if remove {
		condition = dockercontainer.WaitConditionRemoved
	}

	resultC, errC := d.client.ContainerWait(ctx, containerID, condition)
	select {
	case err := <-errC:
		d.logger.Trace("ContainerWait for %s returned error: %v", containerID, err)
		if errdefs.IsNotFound(err) || strings.Contains(err.Error(), "No such container") {
			d.logger.Debug("Container %s not found during wait (possibly already exited and auto-removed), returning 0", containerID)
			return 0, nil
		}
		return 0, err
	case result := <-resultC:
		d.logger.Trace("ContainerWait for %s returned status: %d", containerID, result.StatusCode)
		return int(result.StatusCode), nil
	}
}

// RemoveContainer removes a container.
func (d *DockerRuntime) RemoveContainer(ctx context.Context, containerID string) error {
	d.mu.Lock()
	delete(d.removeOnExit, containerID)
	d.mu.Unlock()

	err := d.client.ContainerRemove(ctx, containerID, dockercontainer.RemoveOptions{
		Force:         true,
		RemoveVolumes: true,
	})
	if err != nil {
		if errdefs.IsNotFound(err) || errdefs.IsConflict(err) {
			return nil
		}
	}
	return err
}

// ResizeContainerTTY resizes the terminal of a container.
func (d *DockerRuntime) ResizeContainerTTY(ctx context.Context, containerID string, rows, cols uint) error {
	return d.client.ContainerResize(ctx, containerID, dockercontainer.ResizeOptions{
		Height: rows,
		Width:  cols,
	})
}

// SignalContainer sends a signal to a container.
func (d *DockerRuntime) SignalContainer(ctx context.Context, containerID string, sig string) error {
	if sig != "" && !signalRegex.MatchString(sig) {
		return fmt.Errorf("invalid signal: %q", sig)
	}
	err := d.client.ContainerKill(ctx, containerID, sig)
	if err != nil {
		if errdefs.IsNotFound(err) || errdefs.IsConflict(err) {
			return nil
		}
	}
	return err
}

// AttachContainer attaches to a container's IO streams.
func (d *DockerRuntime) AttachContainer(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error {
	d.logger.Debug("Attaching to container %s (tty=%v, stdin=%v)", containerID, tty, stdin != nil)
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}

	resp, err := d.client.ContainerAttach(ctx, containerID, dockercontainer.AttachOptions{
		Stream: true,
		Logs:   false,
		Stdin:  stdin != nil,
		Stdout: true,
		Stderr: true,
	})
	if err != nil {
		if ready != nil {
			close(ready)
		}
		return err
	}
	defer resp.Close()

	var stdinErr error
	var stdinMu sync.Mutex
	stdinDone := make(chan struct{})

	if stdin != nil {
		go func() {
			d.logger.Debug("Starting to copy STDIN to container %s", containerID)
			var n int64
			var err error
			n, err = io.Copy(resp.Conn, stdin)
			if err != nil {
				if !errors.Is(err, context.Canceled) {
					stdinMu.Lock()
					stdinErr = err
					stdinMu.Unlock()
				}
				d.logger.Debug("STDIN copy to container %s finished with error: %v", containerID, err)
			} else {
				d.logger.Debug("STDIN copy to container %s finished: %d bytes", containerID, n)
				if err := d.sleepFunc(ctx, d.attachCloseWriteGrace); err == nil {
					d.logger.Trace("Calling CloseWrite on container %s connection", containerID)
					if err := resp.CloseWrite(); err != nil {
						d.logger.Debug("STDIN CloseWrite to container %s failed: %v", containerID, err)
					}
				}
			}
			close(stdinDone)
		}()
	} else {
		close(stdinDone)
	}

	outputDone := make(chan error, 1)
	d.logger.Debug("Starting to copy output from container %s", containerID)
	go func() {
		var err error
		var n int64
		d.logger.Debug("Output goroutine started")
		if tty {
			d.logger.Trace("Starting raw IO copy (TTY=true)")
			n, err = io.Copy(stdout, resp.Reader)
		} else {
			d.logger.Trace("Starting multiplexed StdCopy (TTY=false)")
			n, err = stdcopy.StdCopy(stdout, stderr, resp.Reader)
		}
		d.logger.Trace("Output copy from container %s finished: n=%d, err=%v", containerID, n, err)

		if err != nil {
			d.logger.Debug("Output copy from container %s finished after %d bytes with error: %v", containerID, n, err)
		} else {
			d.logger.Debug("Output copy from container %s finished: %d bytes", containerID, n)
		}
		outputDone <- err
	}()

	if ready != nil {
		close(ready)
	}

	select {
	case err := <-outputDone:
		d.logger.Trace("AttachContainer: output goroutine finished")
		if err == nil {
			stdinMu.Lock()
			sErr := stdinErr
			stdinMu.Unlock()
			if sErr != nil {
				d.logger.Trace("AttachContainer: output finished but returning pending stdin error")
				return sErr
			}
		}
		return err
	case <-stdinDone:
		stdinMu.Lock()
		sErr := stdinErr
		stdinMu.Unlock()
		if sErr != nil {
			d.logger.Trace("AttachContainer: stdin goroutine finished with error")
			return sErr
		}
		select {
		case err := <-outputDone:
			d.logger.Trace("AttachContainer: output goroutine finished after stdin done")
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	case <-ctx.Done():
		resp.Close()
		return ctx.Err()
	}
}

// InspectContainer inspects the container to get its status and exit code.
func (d *DockerRuntime) InspectContainer(ctx context.Context, containerID string) (bool, int, error) {
	resp, err := d.client.ContainerInspect(ctx, containerID)
	if err != nil {
		return false, 0, err
	}
	return resp.State.Running, resp.State.ExitCode, nil
}

// Name returns the name of the runtime.
func (d *DockerRuntime) Name() string {
	return d.name
}
