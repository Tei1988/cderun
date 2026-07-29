package config

import (
	"bytes"
	"testing"

	"cderun/internal/logging"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_Resolver_MountTargetTraversalSecurity(t *testing.T) {
	t.Parallel()
	mfs := &MockFileSystem{WD: "/work"}

	t.Run("Mount target with traversal segments is rejected", func(t *testing.T) {
		cli := &CLIOptions{
			Image:    ptr("alpine"),
			Mounts:   []string{"type=bind,source=/src,target=/app/../etc"},
		}

		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "mount target cannot contain parent directory references")
	})

	t.Run("Mount target with Windows-style traversal segments is rejected", func(t *testing.T) {
		cli := &CLIOptions{
			Image:    ptr("alpine"),
			Mounts:   []string{"type=bind,source=/src,target=/app\\..\\etc"},
		}

		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "mount target cannot contain parent directory references")
	})

	t.Run("Mount target without traversal segments is accepted", func(t *testing.T) {
		cli := &CLIOptions{
			Image:    ptr("alpine"),
			Mounts:   []string{"type=bind,source=/src,target=/app/etc"},
		}

		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)
	})
}

func TestUnit_Config_Resolver_DeviceDestinationTraversalSecurity(t *testing.T) {
	t.Parallel()
	mfs := &MockFileSystem{WD: "/work"}

	t.Run("Device destination with traversal segments is rejected", func(t *testing.T) {
		cli := &CLIOptions{
			Image:    ptr("alpine"),
			Devices:  []string{"/dev/sda:/dev/../sda"},
		}

		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "device destination cannot contain parent directory references")
	})

	t.Run("Device destination with Windows-style traversal segments is rejected", func(t *testing.T) {
		cli := &CLIOptions{
			Image:    ptr("alpine"),
			Devices:  []string{"/dev/sda:\\dev\\..\\sda"},
		}

		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "device destination cannot contain parent directory references")
	})

	t.Run("Device destination without traversal segments is accepted", func(t *testing.T) {
		cli := &CLIOptions{
			Image:    ptr("alpine"),
			Devices:  []string{"/dev/sda:/dev/sda"},
		}

		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)
	})
}

func TestUnit_Config_Resolver_SocketMountWarnings(t *testing.T) {
	// Not Parallel because it manipulates global logger output and level
	mfs := &MockFileSystem{WD: "/work"}

	origLevel := logging.GetGlobalLogger().GetLevel()
	defer logging.GetGlobalLogger().SetLevel(origLevel)

	origWriter := logging.GetGlobalLogger().GetWriter()
	defer logging.GetGlobalLogger().SetOutput(origWriter)

	t.Run("MountSocket true triggers socket warning", func(t *testing.T) {
		var buf bytes.Buffer
		logging.GetGlobalLogger().SetLevel(logging.WarnLevel)
		logging.GetGlobalLogger().SetOutput(&buf)
		defer logging.GetGlobalLogger().SetOutput(origWriter)

		cli := &CLIOptions{
			Image:          ptr("alpine"),
			MountSocket:    ptr(true),
		}

		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)

		logOutput := buf.String()
		assert.Contains(t, logOutput, "Container socket mounting is enabled")
		assert.NotContains(t, logOutput, "Granting container socket permissions through a numeric VM socket GID")
	})

	t.Run("MountSocket true with numeric GID triggers GID warning", func(t *testing.T) {
		var buf bytes.Buffer
		logging.GetGlobalLogger().SetLevel(logging.WarnLevel)
		logging.GetGlobalLogger().SetOutput(&buf)
		defer logging.GetGlobalLogger().SetOutput(origWriter)

		cli := &CLIOptions{
			Image:          ptr("alpine"),
			MountSocket:    ptr(true),
			GroupAdd:       []string{"1001"},
		}

		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)

		logOutput := buf.String()
		assert.Contains(t, logOutput, "Container socket mounting is enabled")
		assert.Contains(t, logOutput, "Granting container socket permissions through a numeric VM socket GID")
	})

	t.Run("MountSocket true with non-numeric group does not trigger GID warning", func(t *testing.T) {
		var buf bytes.Buffer
		logging.GetGlobalLogger().SetLevel(logging.WarnLevel)
		logging.GetGlobalLogger().SetOutput(&buf)
		defer logging.GetGlobalLogger().SetOutput(origWriter)

		cli := &CLIOptions{
			Image:          ptr("alpine"),
			MountSocket:    ptr(true),
			GroupAdd:       []string{"docker"},
		}

		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)

		logOutput := buf.String()
		assert.Contains(t, logOutput, "Container socket mounting is enabled")
		assert.NotContains(t, logOutput, "Granting container socket permissions through a numeric VM socket GID")
	})
}

func TestUnit_Config_Resolver_PrivilegedCapWarnings_NonPrivileged(t *testing.T) {
	mfs := &MockFileSystem{WD: "/work"}

	origLevel := logging.GetGlobalLogger().GetLevel()
	defer logging.GetGlobalLogger().SetLevel(origLevel)

	origWriter := logging.GetGlobalLogger().GetWriter()
	defer logging.GetGlobalLogger().SetOutput(origWriter)

	t.Run("highly privileged capabilities in CapAdd with Privileged false logs warning", func(t *testing.T) {
		var buf bytes.Buffer
		logging.GetGlobalLogger().SetLevel(logging.WarnLevel)
		logging.GetGlobalLogger().SetOutput(&buf)
		defer logging.GetGlobalLogger().SetOutput(origWriter)

		cli := &CLIOptions{
			Image:      ptr("alpine"),
			Privileged: ptr(false),
			CapAdd:     []string{"SYS_ADMIN"},
		}

		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)

		logOutput := buf.String()
		assert.Contains(t, logOutput, "Highly privileged capability")
		assert.Contains(t, logOutput, "SYS_ADMIN")
		assert.NotContains(t, logOutput, "Container is running in privileged mode")
	})
}

func TestUnit_Config_Resolver_EnvNullByteSecurity(t *testing.T) {
	t.Parallel()
	mfs := &MockFileSystem{WD: "/work"}

	t.Run("environment variable value with null byte is rejected", func(t *testing.T) {
		cli := &CLIOptions{
			Image: ptr("alpine"),
			Env:   []string{"SECRET=value\x00injection"},
		}

		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "null byte injection detected")
	})
}
