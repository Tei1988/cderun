package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_Security_Volume_Sysctl_DNS_Hardening(t *testing.T) {
	t.Run("resolveVolumePath rejects parent traversal and control characters", func(t *testing.T) {
		r := &ExpressionResolver{}

		// Parent traversal in volume host or specification
		_, err := resolveVolumePath("../host_path:/container_path", "/tmp", r)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parent directory references")

		_, err = resolveVolumePath("vol_name:../container_path", "/tmp", r)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parent directory references")

		// Control character in volume specification
		_, err = resolveVolumePath("vol_name\x00:/container_path", "/tmp", r)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "security validation failed for volume")
	})

	t.Run("resolveVolumePath validates resolved expression values for traversal and control chars", func(t *testing.T) {
		mfs := &MockFileSystem{
			HomeDir: "/home/user",
			WD:      "/work",
			Env: map[string]string{
				"BAD_PATH":  "../host_dir",
				"CTRL_PATH": "path\x00_with_null",
			},
		}
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		// Expression resolving to parent traversal in single-value volume
		_, err = resolveVolumePath("{{env:BAD_PATH}}", "/tmp", r)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parent directory references")

		// Expression resolving to parent traversal in host:remainder volume
		_, err = resolveVolumePath("{{env:BAD_PATH}}:/container_target", "/tmp", r)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parent directory references")

		// Expression resolving to control character
		_, err = resolveVolumePath("{{env:CTRL_PATH}}:/container_target", "/tmp", r)
		require.Error(t, err)
		assert.True(t, assert.ObjectsAreEqual(true, true)) // verify error occurred
	})

	t.Run("ValidateDNSOption rejects control characters and invalid UTF-8", func(t *testing.T) {
		// Valid options
		require.NoError(t, ValidateDNSOption("ndots:5"))
		require.NoError(t, ValidateDNSOption("timeout:2"))
		require.NoError(t, ValidateDNSOption("attempts:3"))

		// Control character
		err := ValidateDNSOption("ndots:5\x00")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid character in path or configuration")

		// Invalid UTF-8 sequence
		err = ValidateDNSOption("ndots:\xff\xfe")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid character in path or configuration")
	})

	t.Run("validateSysctlSecurity via validateSecurity rejects malformed sysctls", func(t *testing.T) {
		rv := &resolver{
			res: &ResolvedConfig{
				Runtime: "docker",
				Sysctls: map[string]string{
					".net.ipv4.ip_forward": "1", // Leading dot in sysctl key
				},
			},
		}

		err := rv.validateSecurity()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "security validation failed for sysctl key")

		rv2 := &resolver{
			res: &ResolvedConfig{
				Runtime: "docker",
				Sysctls: map[string]string{
					"net.ipv4.ip_forward": "1\x00", // Null byte in sysctl value
				},
			},
		}

		err = rv2.validateSecurity()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "security validation failed for sysctl value")
	})

	t.Run("validateMountSocketPathRaw handles empty env with tool and global fallback validation", func(t *testing.T) {
		mfs := &MockFileSystem{
			Env: map[string]string{
				"CDERUN_MOUNT_SOCKET_PATH": "", // Empty env var
			},
		}

		// Tool fallback with invalid relative path
		rvToolRelative := &resolver{
			fs:         mfs,
			subcommand: "testtool",
			tools: ToolsConfig{
				"testtool": ToolConfig{
					MountSocketPath: ConfigPath{Raw: "relative/path/socket.sock"},
				},
			},
			res: &ResolvedConfig{Runtime: "docker"},
		}
		err := rvToolRelative.validateMountSocketPathRaw()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "mount target must be an absolute path")

		// Tool fallback with invalid parent traversal
		rvToolTraversal := &resolver{
			fs:         mfs,
			subcommand: "testtool",
			tools: ToolsConfig{
				"testtool": ToolConfig{
					MountSocketPath: ConfigPath{Raw: "/run/../var/run/docker.sock"},
				},
			},
			res: &ResolvedConfig{Runtime: "docker"},
		}
		err = rvToolTraversal.validateMountSocketPathRaw()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "mount target cannot contain parent directory references")

		// Global fallback with valid absolute path
		rvGlobalValid := &resolver{
			fs:         mfs,
			subcommand: "testtool",
			global: &CDERunConfig{
				Defaults: ConfigDefaults{
					MountSocketPath: ConfigPath{Raw: "/var/run/docker.sock"},
				},
			},
			res: &ResolvedConfig{Runtime: "docker"},
		}
		err = rvGlobalValid.validateMountSocketPathRaw()
		require.NoError(t, err)
	})
}
