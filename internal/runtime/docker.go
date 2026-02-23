package runtime

import (
	"context"
	"fmt"
	"io"
	"maps"
	"regexp"
	"strings"
	"time"

	"cderun/internal/container"
	"cderun/internal/logging"

	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types"
	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-connections/nat"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

const pullMaxRetries = 3

var eofRegex = regexp.MustCompile(`\beof\b`)

type dockerClient interface {
	ImageInspect(ctx context.Context, imageID string, opts ...client.ImageInspectOption) (image.InspectResponse, error)
	ImagePull(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error)
	ContainerCreate(ctx context.Context, config *dockercontainer.Config, hostConfig *dockercontainer.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (dockercontainer.CreateResponse, error)
	ContainerStart(ctx context.Context, containerID string, options dockercontainer.StartOptions) error
	ContainerWait(ctx context.Context, containerID string, condition dockercontainer.WaitCondition) (<-chan dockercontainer.WaitResponse, <-chan error)
	ContainerRemove(ctx context.Context, containerID string, options dockercontainer.RemoveOptions) error
	ContainerResize(ctx context.Context, containerID string, options dockercontainer.ResizeOptions) error
	ContainerKill(ctx context.Context, containerID string, signal string) error
	ContainerAttach(ctx context.Context, container string, options dockercontainer.AttachOptions) (types.HijackedResponse, error)
}

// DockerRuntime implements ContainerRuntime using Docker Engine API.
type DockerRuntime struct {
	client    dockerClient
	socket    string
	name      string
	sleepFunc func(context.Context, time.Duration) error
}

// NewDockerRuntime creates a new DockerRuntime instance with name "docker".
func NewDockerRuntime(socket string) (*DockerRuntime, error) {
	return NewDockerRuntimeWithName(socket, "docker")
}

// NewDockerRuntimeWithName creates a new DockerRuntime instance with a specific name.
func NewDockerRuntimeWithName(socket string, name string) (*DockerRuntime, error) {
	cli, err := client.NewClientWithOpts(
		client.WithHost("unix://"+socket),
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	return &DockerRuntime{
		client: cli,
		socket: socket,
		name:   name,
		sleepFunc: func(ctx context.Context, d time.Duration) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(d):
				return nil
			}
		},
	}, nil
}

// PullImage pulls the specified image based on the pull policy.
func (d *DockerRuntime) PullImage(ctx context.Context, img string, pullPolicy string) error {
	switch pullPolicy {
	case "never":
		return nil
	case "missing":
		_, err := d.client.ImageInspect(ctx, img)
		if err == nil {
			return nil // Image exists locally
		}
		if !errdefs.IsNotFound(err) {
			return fmt.Errorf("failed to inspect image: %w", err)
		}
	}

	// Policy is "always" or "missing" (and not found locally)
	var lastErr error
	for i := range pullMaxRetries {
		if i > 0 {
			logging.Warn("Retrying image pull (%d/%d) for %s after error: %v", i, pullMaxRetries-1, img, lastErr)
			if err := d.sleepFunc(ctx, time.Duration(1<<i)*time.Second); err != nil {
				return err
			}
		}

		logging.Info("Pulling image %s...", img)
		reader, err := d.client.ImagePull(ctx, img, image.PullOptions{})
		if err != nil {
			lastErr = err
			if isRetryablePullError(err) {
				continue
			}
			return fmt.Errorf("failed to pull image: %w", err)
		}

		// Wait for pull to complete and check for errors in the stream
		err = jsonmessage.DisplayJSONMessagesStream(reader, io.Discard, 0, false, nil)
		_ = reader.Close() //nolint:errcheck
		if err != nil {
			lastErr = err
			if isRetryablePullError(err) {
				continue
			}
			return fmt.Errorf("failed to pull image (stream): %w", err)
		}

		return nil // Success
	}

	return fmt.Errorf("failed to pull image after %d attempts: %w", pullMaxRetries, lastErr)
}

// CreateContainer creates a new container based on the provided config.
func (d *DockerRuntime) CreateContainer(ctx context.Context, config *container.ContainerConfig) (string, error) {
	containerConfig := &dockercontainer.Config{
		Image:        config.Image,
		Cmd:          config.Command,
		Tty:          config.TTY,
		OpenStdin:    config.Interactive,
		Env:          config.Env,
		WorkingDir:   config.Workdir,
		User:         config.User,
		Hostname:     config.Hostname,
		Entrypoint:   config.Entrypoint,
		ExposedPorts: make(nat.PortSet),
	}

	// Handle ExposedPorts
	for _, port := range config.Expose {
		proto := "tcp"
		portNum := port
		if parts := strings.SplitN(port, "/", 2); len(parts) == 2 {
			portNum = parts[0]
			proto = parts[1]
		}
		p, err := nat.NewPort(proto, portNum)
		if err != nil {
			return "", fmt.Errorf("invalid expose port %q: %w", port, err)
		}
		containerConfig.ExposedPorts[p] = struct{}{}
	}

	hostConfig := &dockercontainer.HostConfig{
		AutoRemove:      config.Remove,
		NetworkMode:     dockercontainer.NetworkMode(config.Network),
		Privileged:      config.Privileged,
		CapAdd:          config.CapAdd,
		CapDrop:         config.CapDrop,
		DNS:             config.DNS,
		ExtraHosts:      config.AddHosts,
		PublishAllPorts: config.PublishAll,
		Tmpfs:           make(map[string]string),
		Resources: dockercontainer.Resources{
			Memory:   config.Memory,
			NanoCPUs: int64(config.CPUs * 1e9),
		},
	}

	// Handle PortBindings
	if len(config.Ports) > 0 {
		exposedPorts, bindings, err := nat.ParsePortSpecs(config.Ports)
		if err != nil {
			return "", fmt.Errorf("failed to parse port specs: %w", err)
		}
		hostConfig.PortBindings = bindings
		maps.Copy(containerConfig.ExposedPorts, exposedPorts)
	}

	// Handle Devices
	for _, dev := range config.Devices {
		dMapping := dockercontainer.DeviceMapping{
			PathOnHost:        dev.PathOnHost,
			PathInContainer:   dev.PathInContainer,
			CgroupPermissions: dev.CgroupPermissions,
		}
		hostConfig.Devices = append(hostConfig.Devices, dMapping)
	}

	for _, m := range config.Mounts {
		var mType mount.Type
		switch m.Type {
		case "bind":
			mType = mount.TypeBind
		case "volume":
			mType = mount.TypeVolume
		case "tmpfs":
			mType = mount.TypeTmpfs
		default:
			mType = mount.TypeBind
		}

		hostConfig.Mounts = append(hostConfig.Mounts, mount.Mount{
			Type:     mType,
			Source:   m.Source,
			Target:   m.Target,
			ReadOnly: m.ReadOnly,
		})
	}

	resp, err := d.client.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "")
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
	err := d.client.ContainerKill(ctx, containerID, sig)
	if err != nil {
		// Suppress errors if the container is already gone or not running.
		if errdefs.IsNotFound(err) || errdefs.IsConflict(err) {
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
		// Logs: true with Stream: true replays initial logs then switches to live stream for default
		// drivers (json-file/journald). While ignored by some external drivers, this ensures all
		// output is captured from the start, suitable for cderun's local development focus.
		Logs:   true,
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
			}
			if err := resp.CloseWrite(); err != nil {
				logging.Debug("STDIN CloseWrite to container %s failed: %v", containerID, err)
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
			n, err = io.Copy(stdout, resp.Reader)
		} else {
			// When TTY is disabled, the stream is multiplexed (stdout and stderr are separate).
			n, err = stdcopy.StdCopy(stdout, stderr, resp.Reader)
		}
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
		return err
	case <-stdinDone:
		if stdinErr != nil {
			return stdinErr
		}
		// If stdin is done, wait for the remaining output or context cancellation
		select {
		case err := <-outputDone:
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

// Name returns the name of the runtime.
func (d *DockerRuntime) Name() string {
	return d.name
}

func isRetryablePullError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "toomanyrequests") ||
		strings.Contains(msg, "rate exceeded") ||
		strings.Contains(msg, "rate limit") ||
		strings.Contains(msg, "data limit exceeded") ||
		strings.Contains(msg, "i/o timeout") ||
		strings.Contains(msg, "connection refused") ||
		eofRegex.MatchString(msg)
}
