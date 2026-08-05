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
			Image:  ptr("alpine"),
			Mounts: []string{"type=bind,source=/src,target=/app/../etc"},
		}

		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "mount target cannot contain parent directory references")
	})

	t.Run("Mount target with Windows-style traversal segments is rejected", func(t *testing.T) {
		cli := &CLIOptions{
			Image:  ptr("alpine"),
			Mounts: []string{"type=bind,source=/src,target=/app\\..\\etc"},
		}

		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "mount target cannot contain parent directory references")
	})

	t.Run("Mount target without traversal segments is accepted", func(t *testing.T) {
		cli := &CLIOptions{
			Image:  ptr("alpine"),
			Mounts: []string{"type=bind,source=/src,target=/app/etc"},
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
			Image:   ptr("alpine"),
			Devices: []string{"/dev/sda:/dev/../sda"},
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
			Image:   ptr("alpine"),
			Devices: []string{"/dev/sda:/dev/..\\sda"},
		}

		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "device destination cannot contain parent directory references")
	})

	t.Run("Device destination without traversal segments is accepted", func(t *testing.T) {
		cli := &CLIOptions{
			Image:   ptr("alpine"),
			Devices: []string{"/dev/sda:/dev/sda"},
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
			Image:       ptr("alpine"),
			MountSocket: ptr(true),
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
			Image:       ptr("alpine"),
			MountSocket: ptr(true),
			GroupAdd:    []string{"1001"},
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
			Image:       ptr("alpine"),
			MountSocket: ptr(true),
			GroupAdd:    []string{"docker"},
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

func TestUnit_Config_Loader_ConfigPathSecurity(t *testing.T) {
	t.Parallel()
	mfs := &MockFileSystem{WD: "/work"}
	loader := NewConfigLoaderWithFS(mfs)

	t.Run("CDERun config loading rejects path with control characters", func(t *testing.T) {
		_, _, err := loader.LoadCDERunConfigFromPath("/etc/cderun\x01.yaml")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid character in configuration path")
	})

	t.Run("CDERun config loading rejects path with null bytes", func(t *testing.T) {
		_, _, err := loader.LoadCDERunConfigFromPath("/etc/cderun\x00.yaml")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid character in configuration path")
	})

	t.Run("Tools config loading rejects path with control characters", func(t *testing.T) {
		_, _, err := loader.LoadToolsConfigFromPath("/etc/tools\x01.yaml")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid character in configuration path")
	})

	t.Run("Tools config loading rejects path with null bytes", func(t *testing.T) {
		_, _, err := loader.LoadToolsConfigFromPath("/etc/tools\x00.yaml")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid character in configuration path")
	})
}

func TestUnit_Config_Resolver_HighlyPrivilegedCapsWarnings(t *testing.T) {
	mfs := &MockFileSystem{WD: "/work"}

	origLevel := logging.GetGlobalLogger().GetLevel()
	defer logging.GetGlobalLogger().SetLevel(origLevel)

	origWriter := logging.GetGlobalLogger().GetWriter()
	defer logging.GetGlobalLogger().SetOutput(origWriter)

	tests := []struct {
		name string
		cap  string
	}{
		{"SYS_CHROOT warns", "SYS_CHROOT"},
		{"CAP_SYS_CHROOT warns", "CAP_SYS_CHROOT"},
		{"SYS_BOOT warns", "SYS_BOOT"},
		{"CAP_SYS_BOOT warns", "CAP_SYS_BOOT"},
		{"SYS_TIME warns", "SYS_TIME"},
		{"CAP_SYS_TIME warns", "CAP_SYS_TIME"},
		{"SYSLOG warns", "SYSLOG"},
		{"CAP_SYSLOG warns", "CAP_SYSLOG"},
		{"DAC_OVERRIDE warns", "DAC_OVERRIDE"},
		{"CAP_DAC_OVERRIDE warns", "CAP_DAC_OVERRIDE"},
		{"DAC_READ_SEARCH warns", "DAC_READ_SEARCH"},
		{"CAP_DAC_READ_SEARCH warns", "CAP_DAC_READ_SEARCH"},
		{"LINUX_IMMUTABLE warns", "LINUX_IMMUTABLE"},
		{"CAP_LINUX_IMMUTABLE warns", "CAP_LINUX_IMMUTABLE"},
		{"IPC_LOCK warns", "IPC_LOCK"},
		{"CAP_IPC_LOCK warns", "CAP_IPC_LOCK"},
		{"IPC_OWNER warns", "IPC_OWNER"},
		{"CAP_IPC_OWNER warns", "CAP_IPC_OWNER"},
		{"SYS_TTY_CONFIG warns", "SYS_TTY_CONFIG"},
		{"CAP_SYS_TTY_CONFIG warns", "CAP_SYS_TTY_CONFIG"},
		{"LEASE warns", "LEASE"},
		{"CAP_LEASE warns", "CAP_LEASE"},
		{"AUDIT_CONTROL warns", "AUDIT_CONTROL"},
		{"CAP_AUDIT_CONTROL warns", "CAP_AUDIT_CONTROL"},
		{"MAC_ADMIN warns", "MAC_ADMIN"},
		{"CAP_MAC_ADMIN warns", "CAP_MAC_ADMIN"},
		{"MAC_OVERRIDE warns", "MAC_OVERRIDE"},
		{"CAP_MAC_OVERRIDE warns", "CAP_MAC_OVERRIDE"},
		{"BPF warns", "BPF"},
		{"CAP_BPF warns", "CAP_BPF"},
		{"PERFMON warns", "PERFMON"},
		{"CAP_PERFMON warns", "CAP_PERFMON"},
		{"CHECKPOINT_RESTORE warns", "CHECKPOINT_RESTORE"},
		{"CAP_CHECKPOINT_RESTORE warns", "CAP_CHECKPOINT_RESTORE"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logging.GetGlobalLogger().SetLevel(logging.WarnLevel)
			logging.GetGlobalLogger().SetOutput(&buf)
			defer logging.GetGlobalLogger().SetOutput(origWriter)

			cli := &CLIOptions{
				Image:  ptr("alpine"),
				CapAdd: []string{tt.cap},
			}

			_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
			require.NoError(t, err)

			logOutput := buf.String()
			assert.Contains(t, logOutput, "Highly privileged capability")
			assert.Contains(t, logOutput, tt.cap)
		})
	}
}

func TestUnit_Config_Expression_HardenedValidations(t *testing.T) {
	t.Parallel()
	mfs := &MockFileSystem{WD: "/work"}

	t.Run("resolveFile rejects control characters", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)
		_, err = r.ResolveString("{{file:some\x01file}}")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid character in file directive parameter")
	})

	t.Run("resolveFindDir rejects control characters", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)
		_, err = r.ResolveString("{{find_dir:some\x01dir}}")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid character in find_dir directive parameter")
	})

	t.Run("resolveEnv rejects invalid environment keys", func(t *testing.T) {
		// Env key containing an invalid character like '=' or special symbols
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)
		_, err = r.ResolveString("{{env:KEY=VAL}}")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "security validation failed for env directive key")

		// Env key with null byte
		r, err = NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)
		_, err = r.ResolveString("{{env:KEY\x00INJECTION}}")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "security validation failed for env directive key")

		// Env key with control characters
		r, err = NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)
		_, err = r.ResolveString("{{env:KEY\x1fKEY}}")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "security validation failed for env directive key")

		// Empty env key
		r, err = NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)
		_, err = r.ResolveString("{{env:}}")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "security validation failed for env directive key")
	})
}

func TestUnit_Config_Resolver_MountSocketPathSecurity(t *testing.T) {
	t.Parallel()
	mfs := &MockFileSystem{WD: "/work"}

	t.Run("MountSocketPath relative is rejected", func(t *testing.T) {
		cli := &CLIOptions{
			Image:           ptr("alpine"),
			MountSocketPath: ptr("relative/path"),
		}
		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "mount target must be an absolute path")
	})

	t.Run("MountSocketPath with parent traversal is rejected", func(t *testing.T) {
		cli := &CLIOptions{
			Image:           ptr("alpine"),
			MountSocketPath: ptr("/app/../etc"),
		}
		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "mount target cannot contain parent directory references")
	})

	t.Run("MountSocketPath absolute is accepted", func(t *testing.T) {
		cli := &CLIOptions{
			Image:           ptr("alpine"),
			MountSocketPath: ptr("/var/run/docker.sock"),
		}
		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)
	})
}

func TestUnit_Config_Resolver_SensitiveDeviceWarnings(t *testing.T) {
	mfs := &MockFileSystem{WD: "/work"}

	origLevel := logging.GetGlobalLogger().GetLevel()
	defer logging.GetGlobalLogger().SetLevel(origLevel)

	origWriter := logging.GetGlobalLogger().GetWriter()
	defer logging.GetGlobalLogger().SetOutput(origWriter)

	tests := []struct {
		name       string
		devicePath string
		expectWarn bool
	}{
		{"/dev/mem warns", "/dev/mem", true},
		{"/dev/kmem warns", "/dev/kmem", true},
		{"/dev/port warns", "/dev/port", true},
		{"/dev/sda warns", "/dev/sda", true},
		{"/dev/nvme0n1 warns", "/dev/nvme0n1", true},
		{"/dev/loop0 warns", "/dev/loop0", true},
		{"/dev/mapper/vol warns", "/dev/mapper/vol", true},
		{"/dev/fuse does not warn", "/dev/fuse", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logging.GetGlobalLogger().SetLevel(logging.WarnLevel)
			logging.GetGlobalLogger().SetOutput(&buf)
			defer logging.GetGlobalLogger().SetOutput(origWriter)

			cli := &CLIOptions{
				Image:   ptr("alpine"),
				Devices: []string{tt.devicePath + ":" + tt.devicePath},
			}

			_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
			require.NoError(t, err)

			logOutput := buf.String()
			if tt.expectWarn {
				assert.Contains(t, logOutput, "Mounting highly sensitive host device")
			} else {
				assert.NotContains(t, logOutput, "Mounting highly sensitive host device")
			}
		})
	}
}
