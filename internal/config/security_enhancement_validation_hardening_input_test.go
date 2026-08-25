package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_SecurityEnhancements_ValidateHostname_ControlChars(t *testing.T) {
	t.Parallel()

	t.Run("valid hostname passes", func(t *testing.T) {
		err := ValidateHostname("my-host.example.com")
		assert.NoError(t, err)
	})

	t.Run("hostname with control character rejected", func(t *testing.T) {
		err := ValidateHostname("host\x01name.com")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid character in path or configuration")
	})

	t.Run("hostname with null byte rejected", func(t *testing.T) {
		err := ValidateHostname("host\x00name.com")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid character in path or configuration")
	})
}

func TestUnit_Config_SecurityEnhancements_ValidateNetworkName_ControlChars(t *testing.T) {
	t.Parallel()

	t.Run("valid network name passes", func(t *testing.T) {
		err := ValidateNetworkName("my-network")
		assert.NoError(t, err)
	})

	t.Run("network name with control character rejected", func(t *testing.T) {
		err := ValidateNetworkName("net\x07work")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid character in path or configuration")
	})

	t.Run("network name with null byte rejected", func(t *testing.T) {
		err := ValidateNetworkName("net\x00work")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid character in path or configuration")
	})
}

func TestUnit_Config_SecurityEnhancements_ValidateUserName_ControlChars(t *testing.T) {
	t.Parallel()

	t.Run("valid username passes", func(t *testing.T) {
		err := ValidateUserName("user123:group456")
		assert.NoError(t, err)
	})

	t.Run("username with control character rejected", func(t *testing.T) {
		err := ValidateUserName("user\x08name")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid character in path or configuration")
	})

	t.Run("group with control character rejected", func(t *testing.T) {
		err := ValidateUserName("user:group\x1f")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid character in path or configuration")
	})
}

func TestUnit_Config_SecurityEnhancements_ValidateWorkdir_ControlChars(t *testing.T) {
	t.Parallel()

	t.Run("valid workdir passes", func(t *testing.T) {
		err := ValidateWorkdir("/app/workspace")
		assert.NoError(t, err)
	})

	t.Run("workdir with control character rejected", func(t *testing.T) {
		err := ValidateWorkdir("/app/\x03dir")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid character in path or configuration")
	})
}

func TestUnit_Config_SecurityEnhancements_ResolveDevicePath_DestinationTraversal(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{WD: "/workspace", HomeDir: "/workspace/home"}
	r, err := NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

	t.Run("device with valid destination passes", func(t *testing.T) {
		res, err := resolveDevicePath("/dev/sda:/dev/sda", "/workspace", r)
		require.NoError(t, err)
		assert.Equal(t, "/dev/sda:/dev/sda", res)
	})

	t.Run("device with destination parent traversal rejected", func(t *testing.T) {
		_, err := resolveDevicePath("/dev/sda:/dev/../etc/passwd", "/workspace", r)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "device destination cannot contain parent directory references")
	})

	t.Run("device single path with parent traversal rejected", func(t *testing.T) {
		_, err := resolveDevicePath("/dev/../etc/passwd", "/workspace", r)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "device source cannot contain parent directory references")
	})
}
