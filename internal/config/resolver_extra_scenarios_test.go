package config

import (
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUnit_Config_Resolver_AdvancedDurationResolutionErrors validates that
// duration options check for resolution errors before trying to parse the duration.
func TestUnit_Config_Resolver_AdvancedDurationResolutionErrors(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{}
	cli := &CLIOptions{
		Image:          "alpine",
		ImageSet:       true,
		HangTimeout:    "{{file:nonexistent}}",
		HangTimeoutSet: true,
	}

	_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "file not found: \"nonexistent\"")
}

// TestUnit_Config_Resolver_MemoryQuotingAndParsingBorderCases validates memory resolution
// with various border values and invalid expressions.
func TestUnit_Config_Resolver_MemoryQuotingAndParsingBorderCases(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{}

	t.Run("valid extremely large memory limit 128TiB", func(t *testing.T) {
		cli := &CLIOptions{
			Image:     "alpine",
			ImageSet:  true,
			Memory:    "128TiB",
			MemorySet: true,
		}
		res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, int64(128*1024*1024*1024*1024), res.Memory)
	})

	t.Run("invalid memory format negative value", func(t *testing.T) {
		cli := &CLIOptions{
			Image:     "alpine",
			ImageSet:  true,
			Memory:    "-500MB",
			MemorySet: true,
		}
		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		var cfgErr *InvalidConfigError
		require.ErrorAs(t, err, &cfgErr)
		assert.Equal(t, "memory", cfgErr.Field)
	})

	t.Run("invalid memory format unrecognized unit", func(t *testing.T) {
		cli := &CLIOptions{
			Image:     "alpine",
			ImageSet:  true,
			Memory:    "500invalid",
			MemorySet: true,
		}
		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		var cfgErr *InvalidConfigError
		require.ErrorAs(t, err, &cfgErr)
		assert.Equal(t, "memory", cfgErr.Field)
	})

	t.Run("zero memory limit", func(t *testing.T) {
		cli := &CLIOptions{
			Image:     "alpine",
			ImageSet:  true,
			Memory:    "0",
			MemorySet: true,
		}
		res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, int64(0), res.Memory)
	})
}

// TestUnit_Config_Resolver_RegistryMismatchErrorDetails validates structured error returns
// of validateImageRegistryMatch for different matching combinations.
func TestUnit_Config_Resolver_RegistryMismatchErrorDetails(t *testing.T) {
	t.Parallel()

	t.Run("registry mismatch error verification", func(t *testing.T) {
		err := validateImageRegistryMatch("ghcr.io/user/app:1.0", "docker.io/library/app:2.0")
		require.Error(t, err)
		var regErr *RegistryMismatchError
		require.ErrorAs(t, err, &regErr)
		assert.Equal(t, "docker.io/library/app", regErr.ExpectedRegistry)
		assert.Equal(t, "ghcr.io/user/app", regErr.ActualRegistry)
		assert.Contains(t, regErr.Error(), "container registry mismatch")
	})

	t.Run("nested repo registry matches", func(t *testing.T) {
		err := validateImageRegistryMatch("my-reg.com/nested/path/to/img:v1", "my-reg.com/nested/path/to/img:latest")
		require.NoError(t, err)
	})

	t.Run("localhost with custom port matches", func(t *testing.T) {
		err := validateImageRegistryMatch("localhost:5000/app:v1", "localhost:5000/app:v2")
		require.NoError(t, err)
	})
}

// TestUnit_Config_Resolver_DevicePermissionsSecurityValidation validates device validation rules.
func TestUnit_Config_Resolver_DevicePermissionsSecurityValidation(t *testing.T) {
	t.Parallel()

	t.Run("valid permissions rwm", func(t *testing.T) {
		dc, ok := ParseDeviceConfig("/dev/null:/dev/null:rwm")
		assert.True(t, ok)
		assert.Equal(t, "rwm", dc.Permissions)
	})

	t.Run("valid permissions rw", func(t *testing.T) {
		dc, ok := ParseDeviceConfig("/dev/null:/dev/null:rw")
		assert.True(t, ok)
		assert.Equal(t, "rw", dc.Permissions)
	})

	t.Run("invalid permissions rwx", func(t *testing.T) {
		_, ok := ParseDeviceConfig("/dev/null:/dev/null:rwx")
		assert.False(t, ok)
	})

	t.Run("invalid permission character a", func(t *testing.T) {
		_, ok := ParseDeviceConfig("/dev/null:/dev/null:a")
		assert.False(t, ok)
	})

	t.Run("malicious device parameter injection block", func(t *testing.T) {
		_, ok := ParseDeviceConfig("/dev/null:/dev/null:rwm;rm -rf /")
		assert.False(t, ok)
	})
}

// TestUnit_Config_Resolver_SocketPathAutoDetectionBaseNameOnly validates that auto-detection
// matches base name only (e.g. /my-podman-dir/docker.sock should detect docker).
func TestUnit_Config_Resolver_SocketPathAutoDetectionBaseNameOnly(t *testing.T) {
	t.Parallel()

	t.Run("podman directory with docker.sock detects docker", func(t *testing.T) {
		mfs := &MockFileSystem{
			Dirs:  map[string]bool{"/my-podman-dir": true},
			Files: map[string][]byte{"/my-podman-dir/docker.sock": {}},
		}
		cli := &CLIOptions{
			Image:         "alpine",
			ImageSet:      true,
			SocketPath:    "/my-podman-dir/docker.sock",
			SocketPathSet: true,
		}

		res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, "docker", res.Runtime)
		assert.Equal(t, "/my-podman-dir/docker.sock", res.SocketPath)
	})

	t.Run("docker directory with podman.sock detects podman", func(t *testing.T) {
		mfs := &MockFileSystem{
			Dirs:  map[string]bool{"/my-docker-dir": true},
			Files: map[string][]byte{"/my-docker-dir/podman.sock": {}},
		}
		cli := &CLIOptions{
			Image:         "alpine",
			ImageSet:      true,
			SocketPath:    "/my-docker-dir/podman.sock",
			SocketPathSet: true,
		}

		res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, "podman", res.Runtime)
		assert.Equal(t, "/my-docker-dir/podman.sock", res.SocketPath)
	})

	t.Run("base name logic verification", func(t *testing.T) {
		assert.Equal(t, "docker.sock", path.Base("/var/run/docker.sock"))
		assert.Equal(t, "podman.sock", path.Base("/run/podman/podman.sock"))
	})
}
