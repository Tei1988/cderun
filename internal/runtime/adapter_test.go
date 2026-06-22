package runtime

import (
	"testing"

	"cderun/internal/container"

	"github.com/docker/docker/api/types/mount"
	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Adapter_ToDockerContainerConfig(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		c, h, n, err := toDockerContainerConfig(nil)
		assert.Nil(t, c)
		assert.Nil(t, h)
		assert.Nil(t, n)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "nil container config")
	})

	t.Run("basic config", func(t *testing.T) {
		config := &container.ContainerConfig{
			Image: "alpine",
			Command: []string{"echo", "hello"},
			Env: []string{"FOO=BAR"},
			Workdir: "/app",
			User: "1000",
			Hostname: "testhost",
			Entrypoint: []string{"/bin/sh", "-c"},
		}
		c, h, n, err := toDockerContainerConfig(config)
		require.NoError(t, err)
		assert.NotNil(t, c)
		assert.NotNil(t, h)
		assert.Nil(t, n)

		assert.Equal(t, "alpine", c.Image)
		assert.Equal(t, []string{"echo", "hello"}, []string(c.Cmd))
		assert.Equal(t, []string{"FOO=BAR"}, c.Env)
		assert.Equal(t, "/app", c.WorkingDir)
		assert.Equal(t, "1000", c.User)
		assert.Equal(t, "testhost", c.Hostname)
		assert.Equal(t, []string{"/bin/sh", "-c"}, []string(c.Entrypoint))
	})

	t.Run("ports and expose", func(t *testing.T) {
		config := &container.ContainerConfig{
			Expose: []string{"80/tcp", "53/udp", "8080"},
			Ports: []string{"80:80", "53:53/udp"},
		}
		c, h, _, err := toDockerContainerConfig(config)
		require.NoError(t, err)
		assert.Contains(t, c.ExposedPorts, natPort("80/tcp"))
		assert.Contains(t, c.ExposedPorts, natPort("53/udp"))
		assert.Contains(t, c.ExposedPorts, natPort("8080/tcp"))
		assert.Equal(t, 2, len(h.PortBindings))
	})

	t.Run("invalid expose port", func(t *testing.T) {
		config := &container.ContainerConfig{
			Expose: []string{"invalid"},
		}
		_, _, _, err := toDockerContainerConfig(config)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid expose port")
	})

	t.Run("invalid port spec", func(t *testing.T) {
		config := &container.ContainerConfig{
			Ports: []string{"invalid"},
		}
		_, _, _, err := toDockerContainerConfig(config)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse port specs")
	})

	t.Run("mounts", func(t *testing.T) {
		config := &container.ContainerConfig{
			Mounts: []container.Mount{
				{Type: "bind", Source: "/src", Target: "/dst", ReadOnly: true},
				{Type: "volume", Source: "myvol", Target: "/data"},
				{Type: "tmpfs", Target: "/cache"},
			},
		}
		_, h, _, err := toDockerContainerConfig(config)
		require.NoError(t, err)
		require.Equal(t, 3, len(h.Mounts))
		assert.Equal(t, mount.TypeBind, h.Mounts[0].Type)
		assert.Equal(t, "/src", h.Mounts[0].Source)
		assert.Equal(t, "/dst", h.Mounts[0].Target)
		assert.True(t, h.Mounts[0].ReadOnly)

		assert.Equal(t, mount.TypeVolume, h.Mounts[1].Type)
		assert.Equal(t, "myvol", h.Mounts[1].Source)

		assert.Equal(t, mount.TypeTmpfs, h.Mounts[2].Type)
		assert.Equal(t, "/cache", h.Mounts[2].Target)
	})

	t.Run("invalid mount type", func(t *testing.T) {
		config := &container.ContainerConfig{
			Mounts: []container.Mount{{Type: "invalid"}},
		}
		_, _, _, err := toDockerContainerConfig(config)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid mount type")
	})

	t.Run("resources", func(t *testing.T) {
		config := &container.ContainerConfig{
			Memory: 512 * 1024 * 1024,
			CPUs: 2.5,
		}
		_, h, _, err := toDockerContainerConfig(config)
		require.NoError(t, err)
		assert.Equal(t, int64(512*1024*1024), h.Resources.Memory)
		assert.Equal(t, int64(2.5*1e9), h.Resources.NanoCPUs)
	})

	t.Run("devices", func(t *testing.T) {
		config := &container.ContainerConfig{
			Devices: []container.DeviceMapping{
				{PathOnHost: "/dev/video0", PathInContainer: "/dev/video0", CgroupPermissions: "r"},
			},
		}
		_, h, _, err := toDockerContainerConfig(config)
		require.NoError(t, err)
		require.Equal(t, 1, len(h.Devices))
		assert.Equal(t, "/dev/video0", h.Devices[0].PathOnHost)
		assert.Equal(t, "r", h.Devices[0].CgroupPermissions)
	})
}

func natPort(s string) nat.Port {
	p, _ := nat.NewPort(nat.SplitProtoPort(s))
	return p
}
