package runtime

import (
	"cderun/internal/container"
	"context"
	"fmt"
	"io"

	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

// DockerRuntime implements ContainerRuntime using Docker Engine API.
type DockerRuntime struct {
	client *client.Client
	socket string
}

// NewDockerRuntime creates a new DockerRuntime instance.
func NewDockerRuntime(socket string) (*DockerRuntime, error) {
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
	}, nil
}

// CreateContainer creates a new container based on the provided config.
func (d *DockerRuntime) CreateContainer(ctx context.Context, config *container.ContainerConfig) (string, error) {
	containerConfig := &dockercontainer.Config{
		Image:      config.Image,
		Cmd:        append(config.Command, config.Args...),
		Tty:        config.TTY,
		OpenStdin:  config.Interactive,
		Env:        config.Env,
		WorkingDir: config.Workdir,
		User:       config.User,
	}

	hostConfig := &dockercontainer.HostConfig{
		AutoRemove:  config.Remove,
		NetworkMode: dockercontainer.NetworkMode(config.Network),
	}

	for _, vol := range config.Volumes {
		m := mount.Mount{
			Type:     mount.TypeBind,
			Source:   vol.HostPath,
			Target:   vol.ContainerPath,
			ReadOnly: vol.ReadOnly,
		}
		hostConfig.Mounts = append(hostConfig.Mounts, m)
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
	resultC, errC := d.client.ContainerWait(ctx, containerID, dockercontainer.WaitConditionNotRunning)
	select {
	case err := <-errC:
		return 0, err
	case result := <-resultC:
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
func (d *DockerRuntime) AttachContainer(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer) error {
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}

	resp, err := d.client.ContainerAttach(ctx, containerID, dockercontainer.AttachOptions{
		Stream: true,
		Logs:   true,
		Stdin:  stdin != nil,
		Stdout: true,
		Stderr: true,
	})
	if err != nil {
		return err
	}
	defer resp.Close()

	var stdinErr error
	stdinDone := make(chan struct{})

	if stdin != nil {
		go func() {
			_, stdinErr = io.Copy(resp.Conn, stdin)
			if err := resp.CloseWrite(); err != nil {
				// Logging the error could be useful but we are limited in where to log.
				// For now we just ensure EOF is signaled.
			}
			close(stdinDone)
		}()
	} else {
		close(stdinDone)
	}

	outputDone := make(chan error, 1)
	go func() {
		var err error
		if tty {
			// When TTY is enabled, the stream is raw (not multiplexed).
			_, err = io.Copy(stdout, resp.Reader)
		} else {
			// When TTY is disabled, the stream is multiplexed (stdout and stderr are separate).
			_, err = stdcopy.StdCopy(stdout, stderr, resp.Reader)
		}
		outputDone <- err
	}()

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
		// Explicitly close the connection to unblock any pending I/O
		resp.Close()
		return ctx.Err()
	}
}

// Name returns the name of the runtime.
func (d *DockerRuntime) Name() string {
	return "docker"
}
