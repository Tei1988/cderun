package command

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/config"
	"cderun/internal/container"
)

func TestUnit_AssembleContainerCommand_SuccessAndValidation(t *testing.T) {
	t.Parallel()

	t.Run("empty passthrough arguments returns nil slice", func(t *testing.T) {
		cmd, err := assembleContainerCommand(nil)
		require.NoError(t, err)
		assert.Nil(t, cmd)

		cmdEmpty, err := assembleContainerCommand([]string{})
		require.NoError(t, err)
		assert.Nil(t, cmdEmpty)
	})

	t.Run("valid passthrough arguments cloned successfully", func(t *testing.T) {
		input := []string{"echo", "hello", "world"}
		cmd, err := assembleContainerCommand(input)
		require.NoError(t, err)
		assert.Equal(t, input, cmd)

		// Verify deep copy
		input[0] = "mutated"
		assert.Equal(t, "echo", cmd[0])
	})

	t.Run("null byte in argument triggers security validation error", func(t *testing.T) {
		input := []string{"echo", "hello\x00world"}
		cmd, err := assembleContainerCommand(input)
		require.Error(t, err)
		assert.Nil(t, cmd)
		assert.Contains(t, err.Error(), "security validation failed: command argument [1] contains null byte")
	})
}

func TestUnit_NewBaseContainerConfig_FieldMapping(t *testing.T) {
	t.Parallel()

	resolved := &config.ResolvedConfig{
		Image:       "alpine:latest",
		TTY:         true,
		Interactive: true,
		Network:     "bridge",
		Remove:      true,
		ReadOnly:    true,
		Init:        true,
		Workdir:     "/app",
		User:        "1000:1000",
		Ports:       []string{"8080:8080"},
		PublishAll:  true,
		Expose:      []string{"8080/tcp"},
		Hostname:    "myhost",
		DNS:         []string{"8.8.8.8"},
		AddHosts:    []string{"customhost:127.0.0.1"},
		Privileged:  true,
		Pid:         "host",
		ShmSize:     "64m",
		CapAdd:      []string{"SYS_PTRACE"},
		CapDrop:     []string{"NET_RAW"},
		Entrypoint:  []string{"/entrypoint.sh"},
		Pull:        "always",
		Memory:      1024 * 1024 * 512,
		CPUs:        2.0,
		IPC:         "host",
		SecurityOpt: []string{"no-new-privileges:true"},
		DNSSearch:   []string{"example.com"},
		DNSOptions:  []string{"ndots:2"},
		GPUs:        "all",
		Cgroupns:    "private",
		PidsLimit:   500,
		CPUShares:   1024,
		CpusetCpus:  "0-3",
		CpusetMems:  "0",
		Restart:     "unless-stopped",
		OciRuntime:  "runc",
		Devices: []container.DeviceMapping{
			{PathOnHost: "/dev/kvm", PathInContainer: "/dev/kvm", CgroupPermissions: "rwm"},
		},
		GroupAdd: []string{"1001"},
		Ulimits: []container.Ulimit{
			{Name: "nofile", Soft: 1024, Hard: 2048},
		},
		Sysctls: map[string]string{
			"net.core.somaxconn": "1024",
		},
		Env: []string{"ENV_VAR=value"},
		Mounts: []container.Mount{
			{Type: "bind", Source: "/host", Target: "/container", ReadOnly: true},
		},
	}

	cmdArgs := []string{"ls", "-la"}
	cfg := newBaseContainerConfig(resolved, cmdArgs)

	require.NotNil(t, cfg)
	assert.Equal(t, "alpine:latest", cfg.Image)
	assert.Equal(t, cmdArgs, cfg.Command)
	assert.True(t, cfg.TTY)
	assert.True(t, cfg.Interactive)
	assert.Equal(t, "bridge", cfg.Network)
	assert.True(t, cfg.Remove)
	assert.True(t, cfg.ReadOnly)
	assert.True(t, cfg.Init)
	assert.Equal(t, "/app", cfg.Workdir)
	assert.Equal(t, "1000:1000", cfg.User)
	assert.Equal(t, []string{"8080:8080"}, cfg.Ports)
	assert.True(t, cfg.PublishAll)
	assert.Equal(t, []string{"8080/tcp"}, cfg.Expose)
	assert.Equal(t, "myhost", cfg.Hostname)
	assert.Equal(t, []string{"8.8.8.8"}, cfg.DNS)
	assert.Equal(t, []string{"customhost:127.0.0.1"}, cfg.AddHosts)
	assert.True(t, cfg.Privileged)
	assert.Equal(t, "host", cfg.Pid)
	assert.Equal(t, "64m", cfg.ShmSize)
	assert.Equal(t, []string{"SYS_PTRACE"}, cfg.CapAdd)
	assert.Equal(t, []string{"NET_RAW"}, cfg.CapDrop)
	assert.Equal(t, []string{"/entrypoint.sh"}, cfg.Entrypoint)
	assert.Equal(t, "always", cfg.Pull)
	assert.Equal(t, int64(1024*1024*512), cfg.Memory)
	assert.Equal(t, 2.0, cfg.CPUs)
	assert.Equal(t, "host", cfg.IPC)
	assert.Equal(t, []string{"no-new-privileges:true"}, cfg.SecurityOpt)
	assert.Equal(t, []string{"example.com"}, cfg.DNSSearch)
	assert.Equal(t, []string{"ndots:2"}, cfg.DNSOptions)
	assert.Equal(t, "all", cfg.GPUs)
	assert.Equal(t, "private", cfg.Cgroupns)
	assert.Equal(t, int64(500), cfg.PidsLimit)
	assert.Equal(t, int64(1024), cfg.CPUShares)
	assert.Equal(t, "0-3", cfg.CpusetCpus)
	assert.Equal(t, "0", cfg.CpusetMems)
	assert.Equal(t, "unless-stopped", cfg.Restart)
	assert.Equal(t, "runc", cfg.OciRuntime)
}

func TestUnit_BuildContainerConfig_MountsAndSocketIntegration(t *testing.T) {
	t.Parallel()

	opts := defaultOptions()
	mockFS := &config.MockFileSystem{}
	opts.fs = mockFS
	opts.socketGIDGetter = func(fs config.FileSystem, path string) (string, error) {
		return "999", nil
	}

	resolved := &config.ResolvedConfig{
		Image:           "alpine:latest",
		MountSocket:     true,
		SocketPath:      "/var/run/docker.sock",
		MountSocketPath: "/var/run/docker.sock",
	}

	cfg, err := opts.buildContainerConfig(resolved, []string{"echo", "hi"}, nil)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Check command
	assert.Equal(t, []string{"echo", "hi"}, cfg.Command)

	// Check socket mount and auto-added GID
	require.Len(t, cfg.Mounts, 1)
	assert.Equal(t, "/var/run/docker.sock", cfg.Mounts[0].Source)
	assert.Equal(t, "/var/run/docker.sock", cfg.Mounts[0].Target)
	assert.Contains(t, cfg.GroupAdd, "999")
}
