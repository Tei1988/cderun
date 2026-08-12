package runtime

import (
	"fmt"
	"maps"
	"strconv"
	"strings"

	"cderun/internal/container"

	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	"github.com/docker/go-units"
)

func toDockerContainerConfig(config *container.ContainerConfig) (
	*dockercontainer.Config, *dockercontainer.HostConfig, *network.NetworkingConfig, error,
) {
	if config == nil {
		return nil, nil, nil, fmt.Errorf("nil container config")
	}

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
		PidMode:         dockercontainer.PidMode(config.Pid),
		IpcMode:         dockercontainer.IpcMode(config.IPC),
		SecurityOpt:     config.SecurityOpt,
		DNSSearch:       config.DNSSearch,
		DNSOptions:      config.DNSOptions,
		CgroupnsMode:    dockercontainer.CgroupnsMode(config.Cgroupns),
		CapAdd:          config.CapAdd,
		CapDrop:         config.CapDrop,
		DNS:             config.DNS,
		ExtraHosts:      config.AddHosts,
		PublishAllPorts: config.PublishAll,
		GroupAdd:        config.GroupAdd,
		ReadonlyRootfs:  config.ReadOnly,
		Init:            &config.Init,
		Sysctls:         config.Sysctls,
		Resources: dockercontainer.Resources{
			Memory:     config.Memory,
			NanoCPUs:   int64(config.CPUs * 1e9),
			CPUShares:  config.CPUShares,
			CpusetCpus: config.CpusetCpus,
			CpusetMems: config.CpusetMems,
		},
	}

	if config.PidsLimit != 0 {
		lim := config.PidsLimit
		hostConfig.Resources.PidsLimit = &lim
	}

	if config.Restart != "" {
		parts := strings.Split(config.Restart, ":")
		policy := parts[0]
		if policy != "no" && policy != "always" && policy != "on-failure" && policy != "unless-stopped" {
			return nil, nil, nil, fmt.Errorf("invalid restart policy: %q", config.Restart)
		}
		retries := 0
		if policy != "on-failure" {
			if len(parts) > 1 {
				return nil, nil, nil, fmt.Errorf("restart policy %q does not support retry suffix: %q", policy, config.Restart)
			}
		} else {
			if len(parts) > 2 {
				return nil, nil, nil, fmt.Errorf("restart policy on-failure supports at most one retry suffix: %q", config.Restart)
			}
			if len(parts) == 2 {
				suffix := parts[1]
				if suffix == "" {
					return nil, nil, nil, fmt.Errorf("restart policy on-failure has empty retry suffix: %q", config.Restart)
				}
				val, err := strconv.Atoi(suffix)
				if err != nil || val < 0 {
					return nil, nil, nil, fmt.Errorf("restart policy on-failure retry suffix must be a non-negative integer: %q", config.Restart)
				}
				retries = val
			}
		}

		rp := dockercontainer.RestartPolicy{
			Name:              dockercontainer.RestartPolicyMode(policy),
			MaximumRetryCount: retries,
		}
		hostConfig.RestartPolicy = rp
	}

	if config.GPUs != "" {
		reqs, err := parseGPUs(config.GPUs)
		if err != nil {
			return nil, nil, nil, err
		}
		hostConfig.Resources.DeviceRequests = reqs
	}

	if config.ShmSize != "" {
		shmBytes, err := units.RAMInBytes(config.ShmSize)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("invalid shm-size: %w", err)
		}
		hostConfig.ShmSize = shmBytes
	}

	if len(config.Ulimits) > 0 {
		ulimits := make([]*dockercontainer.Ulimit, len(config.Ulimits))
		for i, u := range config.Ulimits {
			ulimits[i] = &dockercontainer.Ulimit{
				Name: u.Name,
				Hard: u.Hard,
				Soft: u.Soft,
			}
		}
		hostConfig.Ulimits = ulimits
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
			return nil, nil, nil, fmt.Errorf("invalid mount type %q", m.Type)
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

func parseGPUs(gpus string) ([]dockercontainer.DeviceRequest, error) {
	if gpus == "" {
		return nil, nil
	}

	if gpus == "all" {
		req := dockercontainer.DeviceRequest{
			Driver:       "nvidia",
			Count:        -1, // Only use -1 for "all"
			Capabilities: [][]string{{"gpu"}},
		}
		return []dockercontainer.DeviceRequest{req}, nil
	}

	hasDevice := false
	hasCount := false
	var deviceIDs []string
	countVal := 0

	if strings.HasPrefix(gpus, "device=") {
		hasDevice = true
		idsStr := strings.TrimPrefix(gpus, "device=")
		if idsStr == "" {
			return nil, fmt.Errorf("empty device selector: %q", gpus)
		}
		deviceIDs = strings.Split(idsStr, ",")
		for _, id := range deviceIDs {
			id = strings.TrimSpace(id)
			if id == "" {
				return nil, fmt.Errorf("malformed device ID in %q", gpus)
			}
		}
	} else if strings.HasPrefix(gpus, "count=") {
		hasCount = true
		countStr := strings.TrimPrefix(gpus, "count=")
		if countStr == "" {
			return nil, fmt.Errorf("empty count selector: %q", gpus)
		}
		val, err := strconv.Atoi(countStr)
		if err != nil || (val < 1 && val != -1) {
			return nil, fmt.Errorf("invalid count value in %q", gpus)
		}
		countVal = val
	} else {
		return nil, fmt.Errorf("unknown gpu selector or malformed grammar: %q", gpus)
	}

	req := dockercontainer.DeviceRequest{
		Driver:       "nvidia",
		Capabilities: [][]string{{"gpu"}},
	}
	if hasDevice {
		req.DeviceIDs = deviceIDs
		req.Count = 0
	} else if hasCount {
		req.Count = countVal
	}

	return []dockercontainer.DeviceRequest{req}, nil
}
