package runtime

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/container"

	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
)

func TestUnit_DockerAdapter_ToDockerContainerConfig_NilConfig(t *testing.T) {
	t.Parallel()
	cfg, hostCfg, netCfg, err := toDockerContainerConfig(nil)
	assert.Nil(t, cfg)
	assert.Nil(t, hostCfg)
	assert.Nil(t, netCfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil container config")
}

func TestUnit_DockerAdapter_ToDockerContainerConfig_ValidFullConfig(t *testing.T) {
	t.Parallel()
	input := &container.ContainerConfig{
		Image:       "alpine:latest",
		Command:     []string{"echo", "hello"},
		TTY:         true,
		Interactive: true,
		Remove:      true,
		Network:     "host",
		Expose:      []string{"8080/tcp", "9090"},
		Ports:       []string{"80:80/tcp"},
		Env:         []string{"ENV_VAR=value"},
		Workdir:     "/workspace",
		User:        "1000:1000",
		Hostname:    "myhost",
		Entrypoint:  []string{"/bin/sh", "-c"},
		Privileged:  true,
		Pid:         "host",
		IPC:         "host",
		Cgroupns:    "host",
		ShmSize:     "256m",
		CapAdd:      []string{"SYS_ADMIN"},
		CapDrop:     []string{"NET_RAW"},
		Memory:      512 * 1024 * 1024,
		CPUs:        1.5,
		PidsLimit:   100,
		Restart:     "on-failure:3",
		GPUs:        "all",
		Init:        true,
		Ulimits: []container.Ulimit{
			{Name: "nofile", Soft: 1024, Hard: 2048},
		},
		Devices: []container.DeviceMapping{
			{PathOnHost: "/dev/kvm", PathInContainer: "/dev/kvm", CgroupPermissions: "rwm"},
		},
		Mounts: []container.Mount{
			{Type: "bind", Source: "/host", Target: "/container", ReadOnly: true},
			{Type: "volume", Source: "myvol", Target: "/vol"},
			{Type: "tmpfs", Target: "/tmp"},
		},
	}

	cfg, hostCfg, _, err := toDockerContainerConfig(input)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.NotNil(t, hostCfg)

	assert.Equal(t, "alpine:latest", cfg.Image)
	assert.EqualValues(t, []string{"echo", "hello"}, cfg.Cmd)
	assert.True(t, cfg.Tty)
	assert.True(t, cfg.OpenStdin)

	assert.True(t, hostCfg.AutoRemove)
	assert.Equal(t, dockercontainer.NetworkMode("host"), hostCfg.NetworkMode)
	assert.True(t, hostCfg.Privileged)
	assert.Equal(t, dockercontainer.PidMode("host"), hostCfg.PidMode)
	assert.Equal(t, dockercontainer.IpcMode("host"), hostCfg.IpcMode)
	assert.Equal(t, dockercontainer.CgroupnsMode("host"), hostCfg.CgroupnsMode)
	assert.Equal(t, int64(256*1024*1024), hostCfg.ShmSize)
	require.NotNil(t, hostCfg.PidsLimit)
	assert.Equal(t, int64(100), *hostCfg.PidsLimit)
	assert.Equal(t, dockercontainer.RestartPolicyMode("on-failure"), hostCfg.RestartPolicy.Name)
	assert.Equal(t, 3, hostCfg.RestartPolicy.MaximumRetryCount)

	assert.Len(t, hostCfg.Ulimits, 1)
	assert.Equal(t, "nofile", hostCfg.Ulimits[0].Name)

	assert.Len(t, hostCfg.Devices, 1)
	assert.Equal(t, "/dev/kvm", hostCfg.Devices[0].PathOnHost)

	assert.Len(t, hostCfg.Mounts, 3)
	assert.Equal(t, mount.TypeBind, hostCfg.Mounts[0].Type)
	assert.Equal(t, mount.TypeVolume, hostCfg.Mounts[1].Type)
	assert.Equal(t, mount.TypeTmpfs, hostCfg.Mounts[2].Type)
}

func TestUnit_DockerAdapter_BuildDockerExposedPorts_Error(t *testing.T) {
	t.Parallel()
	_, err := buildDockerExposedPorts([]string{"invalid-port/tcp"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid expose port")
}

func TestUnit_DockerAdapter_ParseDockerRestartPolicy_Errors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		restart string
		errMsg  string
	}{
		{
			name:    "invalid policy name",
			restart: "unknown-policy",
			errMsg:  "invalid restart policy",
		},
		{
			name:    "always with retry suffix",
			restart: "always:5",
			errMsg:  "does not support retry suffix",
		},
		{
			name:    "on-failure with too many colons",
			restart: "on-failure:3:5",
			errMsg:  "supports at most one retry suffix",
		},
		{
			name:    "on-failure with empty suffix",
			restart: "on-failure:",
			errMsg:  "has empty retry suffix",
		},
		{
			name:    "on-failure with non-numeric suffix",
			restart: "on-failure:abc",
			errMsg:  "retry suffix must be a non-negative integer",
		},
		{
			name:    "on-failure with negative suffix",
			restart: "on-failure:-1",
			errMsg:  "retry suffix must be a non-negative integer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseDockerRestartPolicy(tt.restart)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}
}

func TestUnit_DockerAdapter_BuildDockerUlimits_And_Devices_Empty(t *testing.T) {
	t.Parallel()
	assert.Nil(t, buildDockerUlimits(nil))
	assert.Nil(t, buildDockerDevices(nil))
	mounts, err := buildDockerMounts(nil)
	require.NoError(t, err)
	assert.Nil(t, mounts)
}

func TestUnit_DockerAdapter_BuildDockerMounts_InvalidType(t *testing.T) {
	t.Parallel()
	_, err := buildDockerMounts([]container.Mount{
		{Type: "invalid-mount-type", Source: "/s", Target: "/t"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid mount type")
}

func TestUnit_DockerAdapter_ToDockerContainerConfig_ShmSizeAndPortBindingErrors(t *testing.T) {
	t.Parallel()
	t.Run("invalid shm-size", func(t *testing.T) {
		input := &container.ContainerConfig{
			Image:   "alpine",
			ShmSize: "invalid-size",
		}
		_, _, _, err := toDockerContainerConfig(input)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid shm-size")
	})

	t.Run("invalid ports spec", func(t *testing.T) {
		input := &container.ContainerConfig{
			Image: "alpine",
			Ports: []string{"invalid:ports:spec:extra"},
		}
		_, _, _, err := toDockerContainerConfig(input)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse port specs")
	})
}
