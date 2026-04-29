package runtime

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"sync"
	"time"

	dockererrdefs "github.com/docker/docker/errdefs"
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

var signalRegex = regexp.MustCompile(`^(?i)(SIG[A-Z0-9]+|[A-Z0-9]+|[0-9]+)$`)

// DockerRuntime implements ContainerRuntime using Docker Engine API.
type DockerRuntime struct {
	client                dockerClient
	socket                string
	name                  string
	sleepFunc             func(context.Context, time.Duration) error
	attachCloseWriteGrace time.Duration
	closeOnce             sync.Once
	closeErr              error
}

// DockerRuntimeOption defines a functional option for DockerRuntime.
type DockerRuntimeOption func(*DockerRuntime)

// WithAttachCloseWriteGrace sets the grace period before closing the write side of an attached connection.
// This is useful in slow or high-latency environments where the default 100ms might be too short
// for the daemon to process the EOF. If d is non-positive, a minimum duration of 1ms is used.
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
	}

	for _, opt := range rtOpts {
		opt(rt)
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

// PullImage pulls the specified image based on the pull policy.
func (d *DockerRuntime) PullImage(ctx context.Context, img string, pullPolicy string, maxRetries int, backoffBase time.Duration) error {
	if pullPolicy == "never" {
		return nil
	}

	var lastErr error
	attempts := maxRetries
	for i := range attempts {
		if i > 0 {
			logging.Warn("Retrying image pull (%d/%d) with exponential backoff for %s after error: %v", i+1, maxRetries, img, lastErr)
			if err := d.sleepFunc(ctx, time.Duration(1<<uint(i))*backoffBase); err != nil {
				return err
			}
		}

		if pullPolicy == "missing" {
			_, err := d.client.ImageInspect(ctx, img)
			if err == nil {
				return nil // Image exists locally
			}
			if !dockererrdefs.IsNotFound(err) {
				lastErr = err
				if IsRetryablePullError(err) {
					continue
				}
				return fmt.Errorf("failed to inspect image: %w", err)
			}
		}

		// Policy is "always" or "missing" (and not found locally)
		logging.Info("Pulling image %s...", img)
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

	return fmt.Errorf("failed to pull image after %d attempts: %w", maxRetries, lastErr)

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

	return resp.ID, nil
}

// StartContainer starts a created container.
func (d *DockerRuntime) StartContainer(ctx context.Context, containerID string) error {
	return d.client.ContainerStart(ctx, containerID, dockercontainer.StartOptions{})
}

// WaitContainer waits for a container to exit and returns its exit code.
func (d *DockerRuntime) WaitContainer(ctx context.Context, containerID string) (int, error) {
	logging.Trace("Waiting for container %s to stop...", containerID)
	resultC, errC := d.client.ContainerWait(ctx, containerID, dockercontainer.WaitConditionNotRunning)
	select {
	case err := <-errC:
		logging.Trace("ContainerWait for %s returned error: %v", containerID, err)
		return 0, err
	case result := <-resultC:
		logging.Trace("ContainerWait for %s returned status: %d", containerID, result.StatusCode)
		return int(result.StatusCode), nil
	}
}

// RemoveContainer removes a container.
func (d *DockerRuntime) RemoveContainer(ctx context.Context, containerID string) error {
	err := d.client.ContainerRemove(ctx, containerID, dockercontainer.RemoveOptions{
		Force: true,
	})
	if err != nil {
		// Suppress errors if the container is already gone or removal is already in progress.
		// This can happen when AutoRemove is enabled and the container finishes before the defer block runs.
		if dockererrdefs.IsNotFound(err) || dockererrdefs.IsConflict(err) {
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
		// Suppress errors if the container is already gone or not running.
		if dockererrdefs.IsNotFound(err) || dockererrdefs.IsConflict(err) {
			return nil
		}
	}
	return err
}

// AttachContainer attaches to a container's IO streams.
func (d *DockerRuntime) AttachContainer(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error {
	logging.Debug("Attaching to container %s (tty=%v, stdin=%v)", containerID, tty, stdin != nil)
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}

	resp, err := d.client.ContainerAttach(ctx, containerID, dockercontainer.AttachOptions{
		Stream: true,
		// Logs: false is used here because we ensure the container is started only after
		// attachment is established (via the ready channel). Using Logs: true can cause
		// early EOF in some Docker versions if the container hasn't started yet.
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
	stdinDone := make(chan struct{})

	if stdin != nil {
		go func() {
			logging.Debug("Starting to copy STDIN to container %s", containerID)
			var n int64
			n, stdinErr = io.Copy(resp.Conn, stdin)
			if stdinErr != nil {
				logging.Debug("STDIN copy to container %s finished with error: %v", containerID, stdinErr)
			} else {
				logging.Debug("STDIN copy to container %s finished: %d bytes", containerID, n)

				// Give a small grace period before closing the write side.
				// In some Docker versions, calling CloseWrite immediately after io.Copy
				// can cause the entire connection to be closed or the EOF to be processed
				// before the data has been fully consumed by the daemon.
				// Using d.sleepFunc ensures we respect context cancellation; if ctx is cancelled,
				// sleepFunc returns an error and we skip CloseWrite to avoid redundant or late calls.
				if err := d.sleepFunc(ctx, d.attachCloseWriteGrace); err == nil {
					logging.Trace("Calling CloseWrite on container %s connection", containerID)
					if err := resp.CloseWrite(); err != nil {
						logging.Debug("STDIN CloseWrite to container %s failed: %v", containerID, err)
					}
				}
			}
			close(stdinDone)
		}()
	} else {
		close(stdinDone)
	}

	outputDone := make(chan error, 1)
	logging.Debug("Starting to copy output from container %s", containerID)
	go func() {
		var err error
		var n int64
		logging.Debug("Output goroutine started")
		if tty {
			// When TTY is enabled, the stream is raw (not multiplexed).
			logging.Trace("Starting raw IO copy (TTY=true)")
			n, err = io.Copy(stdout, resp.Reader)
		} else {
			// When TTY is disabled, the stream is multiplexed (stdout and stderr are separate).
			logging.Trace("Starting multiplexed StdCopy (TTY=false)")
			n, err = stdcopy.StdCopy(stdout, stderr, resp.Reader)
		}
		logging.Trace("Output copy from container %s finished: n=%d, err=%v", containerID, n, err)

		if err != nil {
			logging.Debug("Output copy from container %s finished after %d bytes with error: %v", containerID, n, err)
		} else {
			logging.Debug("Output copy from container %s finished: %d bytes", containerID, n)
		}
		outputDone <- err
	}()

	// Signal that goroutines are started and we are ready for the container to start.
	if ready != nil {
		close(ready)
	}

	select {
	case err := <-outputDone:
		logging.Trace("AttachContainer: output goroutine finished")
		if err == nil {
			// If output finished successfully, check if there was a pending stdin error.
			select {
			case <-stdinDone:
				if stdinErr != nil {
					logging.Trace("AttachContainer: output finished but returning pending stdin error")
					return stdinErr
				}
			default:
			}
		}
		return err
	case <-stdinDone:
		if stdinErr != nil {
			logging.Trace("AttachContainer: stdin goroutine finished with error")
			return stdinErr
		}
		// If stdin is done, wait for the remaining output or context cancellation
		select {
		case err := <-outputDone:
			logging.Trace("AttachContainer: output goroutine finished after stdin done")
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	case <-ctx.Done():
		// Explicitly close the connection to force-unblock pending I/O on context cancellation.
		// Double-closing is acceptable as resp.Close() error is intentionally ignored here or by defer.
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
