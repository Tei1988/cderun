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

	t.Run("Mount target with relative path is rejected", func(t *testing.T) {
		cli := &CLIOptions{
			Image:  ptr("alpine"),
			Mounts: []string{"type=bind,source=/src,target=relative/path"},
		}

		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "mount target must be an absolute path")
	})

	t.Run("Mount target with empty target is rejected", func(t *testing.T) {
		cli := &CLIOptions{
			Image:  ptr("alpine"),
			Mounts: []string{"type=bind,source=/src,target="},
		}

		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "mount target is required")
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

	t.Run("Device destination with relative path is rejected", func(t *testing.T) {
		cli := &CLIOptions{
			Image:   ptr("alpine"),
			Devices: []string{"/dev/sda:relative/path"},
		}

		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "device destination must be an absolute path")
	})

	t.Run("Device destination with empty container path is rejected", func(t *testing.T) {
		cli := &CLIOptions{
			Image:   ptr("alpine"),
			Devices: []string{"/dev/sda:"},
		}

		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		// Since ParseDeviceConfig rejects "/dev/sda:" early (returning false), we assert on invalid device config
		assert.Contains(t, err.Error(), "invalid device config")
	})

	t.Run("Device destination with Windows-style traversal segments is rejected", func(t *testing.T) {
		cli := &CLIOptions{
			Image:    ptr("alpine"),
			Devices:  []string{"/dev/sda:/dev/..\\sda"},
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

func TestUnit_Config_Resolver_HostNetworkWarning(t *testing.T) {
	mfs := &MockFileSystem{WD: "/work"}

	origLevel := logging.GetGlobalLogger().GetLevel()
	defer logging.GetGlobalLogger().SetLevel(origLevel)

	origWriter := logging.GetGlobalLogger().GetWriter()
	defer logging.GetGlobalLogger().SetOutput(origWriter)

	t.Run("host network mode logs warning", func(t *testing.T) {
		var buf bytes.Buffer
		logging.GetGlobalLogger().SetLevel(logging.WarnLevel)
		logging.GetGlobalLogger().SetOutput(&buf)
		defer logging.GetGlobalLogger().SetOutput(origWriter)

		cli := &CLIOptions{
			Image:   ptr("alpine"),
			Network: ptr("host"),
		}

		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)

		logOutput := buf.String()
		assert.Contains(t, logOutput, "Container is running with host network mode enabled")
	})

	t.Run("bridge network mode does not log host network warning", func(t *testing.T) {
		var buf bytes.Buffer
		logging.GetGlobalLogger().SetLevel(logging.WarnLevel)
		logging.GetGlobalLogger().SetOutput(&buf)
		defer logging.GetGlobalLogger().SetOutput(origWriter)

		cli := &CLIOptions{
			Image:   ptr("alpine"),
			Network: ptr("bridge"),
		}

		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)

		logOutput := buf.String()
		assert.NotContains(t, logOutput, "Container is running with host network mode enabled")
	})
}

func TestUnit_Config_Resolver_SensitivePathMountWarnings(t *testing.T) {
	mfs := &MockFileSystem{WD: "/work"}

	origLevel := logging.GetGlobalLogger().GetLevel()
	defer logging.GetGlobalLogger().SetLevel(origLevel)

	origWriter := logging.GetGlobalLogger().GetWriter()
	defer logging.GetGlobalLogger().SetOutput(origWriter)

	tests := []struct {
		name        string
		mountSource string
		expectWarn  bool
	}{
		{"Mounting root path warns", "/", true},
		{"Mounting /etc warns", "/etc", true},
		{"Mounting subdirectory of /etc warns", "/etc/shadow", true},
		{"Mounting /proc warns", "/proc", true},
		{"Mounting /sys warns", "/sys", true},
		{"Mounting /boot warns", "/boot", true},
		{"Mounting /dev warns", "/dev", true},
		{"Mounting normal path does not warn", "/work/app", false},
		{"Mounting custom directory does not warn", "/etc-not-really", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logging.GetGlobalLogger().SetLevel(logging.WarnLevel)
			logging.GetGlobalLogger().SetOutput(&buf)
			defer logging.GetGlobalLogger().SetOutput(origWriter)

			cli := &CLIOptions{
				Image:  ptr("alpine"),
				Mounts: []string{"type=bind,source=" + tt.mountSource + ",target=/app"},
			}

			_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
			require.NoError(t, err)

			logOutput := buf.String()
			if tt.expectWarn {
				assert.Contains(t, logOutput, "Mounting highly sensitive host path")
			} else {
				assert.NotContains(t, logOutput, "Mounting highly sensitive host path")
			}
		})
	}
}
