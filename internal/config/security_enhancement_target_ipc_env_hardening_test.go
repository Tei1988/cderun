package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_SecurityEnhancement_Target_IPC_Env_Hardening(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD: "/app",
	}

	r, err := NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

	t.Run("MountConfig target parent traversal post resolution", func(t *testing.T) {
		mc := MountConfig{
			Type:   "bind",
			Source: ConfigPath{Raw: "/host/data"},
			Target: ConfigPath{Raw: "/container/../etc/passwd"},
		}
		_, err := mc.Resolve(r)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "mount target cannot contain parent directory references")
	})

	t.Run("DeviceConfig destination parent traversal post resolution", func(t *testing.T) {
		dc := DeviceConfig{
			Source:      ConfigPath{Raw: "/dev/ttyS0"},
			Destination: ConfigPath{Raw: "/dev/../etc/shadow"},
			Permissions: "rwm",
		}
		_, err := dc.Resolve(r)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "device destination cannot contain parent directory references")
	})

	t.Run("IPC container reference validation", func(t *testing.T) {
		invalidIPCs := []string{
			"container:..",
			"container:.",
			"container:foo/../bar",
			"container:invalid@name",
		}
		for _, ipc := range invalidIPCs {
			cli := &CLIOptions{
				Image: ptr("alpine"),
				IPC:   ptr(ipc),
			}
			_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
			require.Error(t, err)
			assert.True(t, assert.ObjectsAreEqual(true, strings.Contains(err.Error(), "unsupported ipc namespace") || strings.Contains(err.Error(), "invalid character in container ipc reference")))
		}
	})

	t.Run("Env value invalid UTF-8 rejection", func(t *testing.T) {
		invalidUTF8 := string([]byte{0xff, 0xfe, 0xfd})
		cli := &CLIOptions{
			Image: ptr("alpine"),
			Env:   []string{"BAD_UTF8=" + invalidUTF8},
		}
		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid UTF-8 encoding")
	})

	t.Run("ValidateDNSOption control character rejection", func(t *testing.T) {
		err := ValidateDNSOption("ndots:5\x00")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid character")
	})

	t.Run("ValidateSecurityOpt control character rejection", func(t *testing.T) {
		err := ValidateSecurityOpt("seccomp=unconfined\x07")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid character")
	})
}
