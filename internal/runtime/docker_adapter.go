package runtime

import (
	"fmt"
	"maps"
	"strings"

	"cderun/internal/container"

	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
)

func toDockerContainerConfig(config *container.ContainerConfig) (
	*dockercontainer.Config, *dockercontainer.HostConfig, *network.NetworkingConfig, error,
) {
	containerConfig := &dockercontainer.Config{
		Image:        config.Image,
		Cmd:          config.Command,
		Tty:          config.TTY,
		StdinOnce:    config.Interactive,
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
			return nil, nil, nil, fmt.Errorf("invalid expose port %q: %w", port, err)
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
			return nil, nil, nil, fmt.Errorf("failed to parse port specs: %w", err)
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

	return containerConfig, hostConfig, nil, nil
}
