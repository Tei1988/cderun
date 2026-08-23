package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_Path_ResolveDevice_TraversalValidation(t *testing.T) {
	t.Parallel()

	r, err := NewExpressionResolver(nil)
	require.NoError(t, err)

	t.Run("resolveDevicePath rejects host path parent traversal", func(t *testing.T) {
		tests := []string{
			"../dev/fuse:/dev/fuse",
			"/dev/../fuse:/dev/fuse",
			"..:/dev/fuse",
			"../dev/fuse",
		}

		for _, input := range tests {
			_, err := resolveDevicePath(input, "/work", r)
			require.Error(t, err, "input: %s", input)
			assert.Contains(t, err.Error(), "device source cannot contain parent directory references", "input: %s", input)
		}
	})

	t.Run("resolveDevicePath accepts valid host path", func(t *testing.T) {
		res, err := resolveDevicePath("/dev/fuse:/dev/fuse:rwm", "/work", r)
		require.NoError(t, err)
		assert.Equal(t, "/dev/fuse:/dev/fuse:rwm", res)
	})

	t.Run("DeviceConfig Resolve rejects host and container traversal", func(t *testing.T) {
		t.Run("host path traversal", func(t *testing.T) {
			dc := DeviceConfig{
				Source:      ConfigPath{Raw: "/dev/../fuse"},
				Destination: ConfigPath{Raw: "/dev/fuse"},
				Permissions: "rwm",
			}
			_, err := dc.Resolve(r)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "device source cannot contain parent directory references")
		})

		t.Run("destination path traversal", func(t *testing.T) {
			dc := DeviceConfig{
				Source:      ConfigPath{Raw: "/dev/fuse"},
				Destination: ConfigPath{Raw: "/dev/../fuse"},
				Permissions: "rwm",
			}
			_, err := dc.Resolve(r)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "device destination cannot contain parent directory references")
		})

		t.Run("valid device config resolves successfully", func(t *testing.T) {
			dc := DeviceConfig{
				Source:      ConfigPath{Raw: "/dev/fuse"},
				Destination: ConfigPath{Raw: "/dev/fuse"},
				Permissions: "rwm",
			}
			mapping, err := dc.Resolve(r)
			require.NoError(t, err)
			assert.Equal(t, "/dev/fuse", mapping.PathOnHost)
			assert.Equal(t, "/dev/fuse", mapping.PathInContainer)
			assert.Equal(t, "rwm", mapping.CgroupPermissions)
		})
	})

	t.Run("resolveDevices rejects device list with path traversal", func(t *testing.T) {
		mfs := RealFileSystem{}
		_, err := resolveDevices([]string{"/dev/../fuse:/dev/fuse"}, nil, "", nil, nil, r, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "device source cannot contain parent directory references")
	})
}
