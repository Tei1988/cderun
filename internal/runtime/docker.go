package runtime

import (
	"cderun/internal/container"
	"context"
	"fmt"
	"io"
	"strings"

	"cderun/internal/logging"
	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-connections/nat"
)

// DockerRuntime implements ContainerRuntime using Docker Engine API.
type DockerRuntime struct {
	client *client.Client
	socket string
	name   string
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
	}, nil
}

// PullImage pulls the specified image based on the pull policy.
func (d *DockerRuntime) PullImage(ctx context.Context, img string, pullPolicy string) error {
	if pullPolicy == "never" {
		return nil
	}

	if pullPolicy == "missing" || pullPolicy == "" {
		_, _, err := d.client.ImageInspectWithRaw(ctx, img)
		if err == nil {
			return nil // Image exists locally
		}
		if !errdefs.IsNotFound(err) {
			return fmt.Errorf("failed to inspect image: %w", err)
		}
	}

	// Always or missing (but not found locally)
	logging.Info("Pulling image %s...", img)
	reader, err := d.client.ImagePull(ctx, img, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}
	defer reader.Close()

	// Wait for pull to complete and check for errors in the stream
	return jsonmessage.DisplayJSONMessagesStream(reader, io.Discard, 0, false, nil)
}

// CreateContainer creates a new container based on the provided config.
func (d *DockerRuntime) CreateContainer(ctx context.Context, config *container.ContainerConfig) (string, error) {
	containerConfig := &dockercontainer.Config{
		Image:        config.Image,
		Cmd:          append(config.Command, config.Args...),
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
		AutoRemove:     config.Remove,
		NetworkMode:    dockercontainer.NetworkMode(config.Network),
		Privileged:     config.Privileged,
		CapAdd:         config.CapAdd,
		CapDrop:        config.CapDrop,
		DNS:            config.DNS,
		ExtraHosts:     config.AddHosts,
		PublishAllPorts: config.PublishAll,
		Tmpfs:          make(map[string]string),
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
		for k, v := range exposedPorts {
			containerConfig.ExposedPorts[k] = v
		}
	}

	// Handle Tmpfs
	for _, t := range config.Tmpfs {
		parts := strings.SplitN(t, ":", 2)
		if len(parts) == 2 {
			hostConfig.Tmpfs[parts[0]] = parts[1]
		} else {
			hostConfig.Tmpfs[parts[0]] = ""
		}
	}

	// Handle Devices
	for _, dev := range config.Devices {
		parts := strings.SplitN(dev, ":", 3)
		dMapping := dockercontainer.DeviceMapping{
			PathOnHost:        parts[0],
			PathInContainer:   parts[0],
			CgroupPermissions: "rwm",
		}
		if len(parts) > 1 {
			dMapping.PathInContainer = parts[1]
		}
		if len(parts) > 2 {
			dMapping.CgroupPermissions = parts[2]
		}
		hostConfig.Resources.Devices = append(hostConfig.Resources.Devices, dMapping)
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
	return d.name
}
