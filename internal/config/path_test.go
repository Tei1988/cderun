package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPathResolution(t *testing.T) {
	home, _ := os.UserHomeDir()
	baseDir := "/abs/path"
	r, err := NewExpressionResolver()
	require.NoError(t, err)

	t.Run("resolvePath", func(t *testing.T) {
		assert.Equal(t, "/abs/path/file", resolvePath("./file", baseDir))
		assert.Equal(t, "/abs/file", resolvePath("../file", baseDir))
		assert.Equal(t, filepath.Join(home, ".ssh"), resolvePath("~/.ssh", baseDir))
		assert.Equal(t, "/other/abs/path", resolvePath("/other/abs/path", baseDir))
		assert.Equal(t, "just-name", resolvePath("just-name", baseDir)) // No ./ prefix, no resolution
	})

	t.Run("ConfigPath.Resolve", func(t *testing.T) {
		cp := ConfigPath{Raw: "./data", BaseDir: baseDir}
		assert.Equal(t, "/abs/path/data", cp.Resolve(r))

		cp = ConfigPath{Raw: "{{HOME}}/config", BaseDir: baseDir}
		assert.Equal(t, filepath.Join(home, "config"), cp.Resolve(r))
	})

	t.Run("VolumeConfig.Resolve", func(t *testing.T) {
		vc := VolumeConfig{
			Source:      ConfigPath{Raw: "./data", BaseDir: baseDir},
			Destination: ConfigPath{Raw: "/app/data", BaseDir: baseDir},
			ReadOnly:    false,
		}
		mount := vc.Resolve(r)
		assert.Equal(t, "/abs/path/data", mount.HostPath)
		assert.Equal(t, "/app/data", mount.ContainerPath)
		assert.False(t, mount.ReadOnly)

		vc = VolumeConfig{
			Source:      ConfigPath{Raw: "~/config", BaseDir: baseDir},
			Destination: ConfigPath{Raw: "/root/config", BaseDir: baseDir},
			ReadOnly:    true,
		}
		mount = vc.Resolve(r)
		assert.Equal(t, filepath.Join(home, "config"), mount.HostPath)
		assert.Equal(t, "/root/config", mount.ContainerPath)
		assert.True(t, mount.ReadOnly)
	})

	t.Run("DeviceConfig.Resolve", func(t *testing.T) {
		dc := DeviceConfig{
			Source:      ConfigPath{Raw: "/dev/video0", BaseDir: baseDir},
			Destination: ConfigPath{Raw: "/dev/video0", BaseDir: baseDir},
			Permissions: "rwm",
		}
		mapping := dc.Resolve(r)
		assert.Equal(t, "/dev/video0", mapping.PathOnHost)
		assert.Equal(t, "/dev/video0", mapping.PathInContainer)
		assert.Equal(t, "rwm", mapping.CgroupPermissions)
	})

	t.Run("ParseVolumeConfig", func(t *testing.T) {
		vc, ok := ParseVolumeConfig("./data:/app/data:ro")
		assert.True(t, ok)
		assert.Equal(t, "./data", vc.Source.Raw)
		assert.Equal(t, "/app/data", vc.Destination.Raw)
		assert.True(t, vc.ReadOnly)

		vc, ok = ParseVolumeConfig("/host/path:/container/path")
		assert.True(t, ok)
		assert.Equal(t, "/host/path", vc.Source.Raw)
		assert.Equal(t, "/container/path", vc.Destination.Raw)
		assert.False(t, vc.ReadOnly)

		_, ok = ParseVolumeConfig("invalid-format")
		assert.False(t, ok)
	})

	t.Run("Windows Paths", func(t *testing.T) {
		vc, ok := ParseVolumeConfig(`C:\host\path:/container`)
		assert.True(t, ok)
		assert.Equal(t, `C:\host\path`, vc.Source.Raw)
		assert.Equal(t, `/container`, vc.Destination.Raw)

		dc, ok := ParseDeviceConfig(`E:\dev\path:/dev/path:rwm`)
		assert.True(t, ok)
		assert.Equal(t, `E:\dev\path`, dc.Source.Raw)
		assert.Equal(t, `/dev/path`, dc.Destination.Raw)
		assert.Equal(t, "rwm", dc.Permissions)
	})

	t.Run("Scheme Preservation", func(t *testing.T) {
		assert.Equal(t, "unix:///var/run/docker.sock", resolvePath("unix:///var/run/docker.sock", baseDir))
		assert.Equal(t, "unix:///var/run/docker.sock", resolvePath("unix:////var/run/docker.sock", baseDir))
		assert.Equal(t, "http://example.com/path", resolvePath("http://example.com/path", baseDir))
	})
}
