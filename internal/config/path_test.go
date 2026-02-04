package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
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

	t.Run("MountConfig.Resolve", func(t *testing.T) {
		mc := MountConfig{
			Type:     "bind",
			Source:   ConfigPath{Raw: "./data", BaseDir: baseDir},
			Target:   ConfigPath{Raw: "/app/data", BaseDir: baseDir},
			ReadOnly: false,
		}
		mount := mc.Resolve(r)
		assert.Equal(t, "bind", mount.Type)
		assert.Equal(t, "/abs/path/data", mount.Source)
		assert.Equal(t, "/app/data", mount.Target)
		assert.False(t, mount.ReadOnly)

		mc = MountConfig{
			Type:     "bind",
			Source:   ConfigPath{Raw: "~/config", BaseDir: baseDir},
			Target:   ConfigPath{Raw: "/root/config", BaseDir: baseDir},
			ReadOnly: true,
		}
		mount = mc.Resolve(r)
		assert.Equal(t, filepath.Join(home, "config"), mount.Source)
		assert.Equal(t, "/root/config", mount.Target)
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

	t.Run("ParseMountFlag", func(t *testing.T) {
		mc, err := ParseMountFlag("type=bind,source=./data,target=/app/data,readonly")
		assert.NoError(t, err)
		assert.Equal(t, "bind", mc.Type)
		assert.Equal(t, "./data", mc.Source.Raw)
		assert.Equal(t, "/app/data", mc.Target.Raw)
		assert.True(t, mc.ReadOnly)

		mc, err = ParseMountFlag("source=/host/path,target=/container/path")
		assert.NoError(t, err)
		assert.Equal(t, "bind", mc.Type)
		assert.Equal(t, "/host/path", mc.Source.Raw)
		assert.Equal(t, "/container/path", mc.Target.Raw)
		assert.False(t, mc.ReadOnly)

		_, err = ParseMountFlag("invalid-format")
		assert.Error(t, err)
	})

	t.Run("Windows Paths", func(t *testing.T) {
		mc, err := ParseMountFlag(`type=bind,source=C:\host\path,target=/container`)
		assert.NoError(t, err)
		assert.Equal(t, `C:\host\path`, mc.Source.Raw)
		assert.Equal(t, `/container`, mc.Target.Raw)

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

func TestUnmarshalYAML_Errors(t *testing.T) {
	t.Run("MountConfig", func(t *testing.T) {
		var mc MountConfig

		// Valid (structure)
		yamlStr := `
type: bind
source: ./data
target: /app/data
read_only: true
`
		err := yaml.Unmarshal([]byte(yamlStr), &mc)
		assert.NoError(t, err)
		assert.Equal(t, "bind", mc.Type)
		assert.Equal(t, "./data", mc.Source.Raw)

		// Implicit type (default to bind)
		yamlStr = `
source: ./implicit
target: /app/implicit
`
		err = yaml.Unmarshal([]byte(yamlStr), &mc)
		assert.NoError(t, err)
		assert.Equal(t, "bind", mc.Type)
		assert.Equal(t, "./implicit", mc.Source.Raw)

		// Invalid
		err = yaml.Unmarshal([]byte("invalid-mount"), &mc)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid mount config")

		// Missing target
		yamlStr = `
type: bind
source: ./data
`
		err = yaml.Unmarshal([]byte(yamlStr), &mc)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "mount target is required")
	})

	t.Run("DeviceConfig", func(t *testing.T) {
		var dc DeviceConfig

		// Valid
		err := yaml.Unmarshal([]byte("/dev/video0:/dev/video0:rwm"), &dc)
		assert.NoError(t, err)
		assert.Equal(t, "/dev/video0", dc.Source.Raw)

		// Invalid
		err = yaml.Unmarshal([]byte(":/container:rwm"), &dc)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid device config")
	})
}
