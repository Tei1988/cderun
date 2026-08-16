package config

import (
	"bytes"
	"strings"
	"testing"

	"cderun/internal/container"
	"cderun/internal/logging"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecurityEnhancement_ResolveEnv_DefaultValueValidation(t *testing.T) {
	fs := &MockFileSystem{
		HomeDir: "/home/user",
		WD:      "/home/user/project",
	}

	t.Run("Env fallback default value with control character (newline) is rejected", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, fs)
		require.NoError(t, err)

		_, err = r.ResolveString("{{env:MISSING_VAR:-val\nwith_control}}")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "security validation failed for env directive default value")
	})

	t.Run("Env fallback default value with null byte is rejected", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, fs)
		require.NoError(t, err)

		_, err = r.ResolveString("{{env:MISSING_VAR:-val\x00with_null}}")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "security validation failed for env directive default value")
	})

	t.Run("Env fallback default value with safe chars succeeds", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, fs)
		require.NoError(t, err)

		res, err := r.ResolveString("{{env:MISSING_VAR:-safe-default_value}}")
		require.NoError(t, err)
		assert.Equal(t, "safe-default_value", res)
	})
}

func TestSecurityEnhancement_ResolveSysctls_ControlCharValidation(t *testing.T) {
	fs := &MockFileSystem{}

	t.Run("Sysctl key with control character is rejected", func(t *testing.T) {
		_, err := resolveSysctls([]string{"net.ipv4\r.ip_forward=1"}, nil, "test", nil, nil, nil, fs)
		var cfgErr *InvalidConfigError
		require.ErrorAs(t, err, &cfgErr)
		assert.Equal(t, "sysctl", cfgErr.Field)
		assert.Contains(t, cfgErr.Err.Error(), "security validation failed for sysctl key")
	})

	t.Run("Sysctl value with control character is rejected", func(t *testing.T) {
		_, err := resolveSysctls([]string{"net.ipv4.ip_forward=1\n"}, nil, "test", nil, nil, nil, fs)
		var cfgErr *InvalidConfigError
		require.ErrorAs(t, err, &cfgErr)
		assert.Equal(t, "sysctl", cfgErr.Field)
		assert.Contains(t, cfgErr.Err.Error(), "security validation failed for sysctl value")
	})

	t.Run("Sysctl value with null byte is rejected", func(t *testing.T) {
		_, err := resolveSysctls([]string{"net.ipv4.ip_forward=1\x00"}, nil, "test", nil, nil, nil, fs)
		var cfgErr *InvalidConfigError
		require.ErrorAs(t, err, &cfgErr)
		assert.Equal(t, "sysctl", cfgErr.Field)
		assert.Contains(t, cfgErr.Err.Error(), "security validation failed for sysctl value")
	})

	t.Run("Sysctl key with leading newline is rejected", func(t *testing.T) {
		_, err := resolveSysctls([]string{"\nnet.ipv4.ip_forward=1"}, nil, "test", nil, nil, nil, fs)
		var cfgErr *InvalidConfigError
		require.ErrorAs(t, err, &cfgErr)
		assert.Contains(t, cfgErr.Err.Error(), "security validation failed for sysctl key")
	})

	t.Run("Sysctl key with trailing carriage return is rejected", func(t *testing.T) {
		_, err := resolveSysctls([]string{"net.ipv4.ip_forward\r=1"}, nil, "test", nil, nil, nil, fs)
		var cfgErr *InvalidConfigError
		require.ErrorAs(t, err, &cfgErr)
		assert.Contains(t, cfgErr.Err.Error(), "security validation failed for sysctl key")
	})

	t.Run("Sysctl key with leading tab is rejected", func(t *testing.T) {
		_, err := resolveSysctls([]string{"\tnet.ipv4.ip_forward=1"}, nil, "test", nil, nil, nil, fs)
		var cfgErr *InvalidConfigError
		require.ErrorAs(t, err, &cfgErr)
		assert.Contains(t, cfgErr.Err.Error(), "security validation failed for sysctl key")
	})

	t.Run("Safe sysctls succeed", func(t *testing.T) {
		res, err := resolveSysctls([]string{"net.ipv4.ip_forward=1"}, nil, "test", nil, nil, nil, fs)
		require.NoError(t, err)
		assert.Equal(t, "1", res["net.ipv4.ip_forward"])
	})
}

func TestSecurityEnhancement_ValidateSecurity_ManualSocketBindWarning(t *testing.T) {
	origLevel := logging.GetGlobalLogger().GetLevel()
	defer logging.GetGlobalLogger().SetLevel(origLevel)
	origWriter := logging.GetGlobalLogger().GetWriter()
	defer logging.GetGlobalLogger().SetOutput(origWriter)

	fs := &MockFileSystem{}

	// Initialize field info registry so option lookups succeed
	fieldOnce.Do(initFieldInfo)

	t.Run("Manual bind mount of docker socket emits warning when MountSocket is false", func(t *testing.T) {
		var buf bytes.Buffer
		logging.GetGlobalLogger().SetLevel(logging.WarnLevel)
		logging.GetGlobalLogger().SetOutput(&buf)

		cli := &CLIOptions{}
		resolved := &ResolvedConfig{
			Image:      "alpine",
			Engine:     "docker",
			Runtime:    "docker",
			SocketPath: "/var/run/docker.sock",
			Mounts: []container.Mount{
				{
					Type:     "bind",
					Source:   "/var/run/docker.sock",
					Target:   "/var/run/docker.sock",
					ReadOnly: false,
				},
			},
		}

		rv := &resolver{
			cli: cli,
			res: resolved,
			fs:  fs,
		}

		err := rv.validateSecurity()
		require.NoError(t, err)

		logOutput := buf.String()
		assert.Contains(t, logOutput, "Container socket mounting is enabled. Granting access to the container runtime socket is highly privileged")
		// Check that it only warned once
		assert.Equal(t, 1, strings.Count(logOutput, "Container socket mounting is enabled"))
	})

	t.Run("Manual bind mount of docker socket and MountSocket=true emits warning exactly once", func(t *testing.T) {
		var buf bytes.Buffer
		logging.GetGlobalLogger().SetLevel(logging.WarnLevel)
		logging.GetGlobalLogger().SetOutput(&buf)

		cli := &CLIOptions{}
		resolved := &ResolvedConfig{
			Image:       "alpine",
			Engine:      "docker",
			Runtime:     "docker",
			SocketPath:  "/var/run/docker.sock",
			MountSocket: true,
			Mounts: []container.Mount{
				{
					Type:     "bind",
					Source:   "/var/run/docker.sock",
					Target:   "/var/run/docker.sock",
					ReadOnly: false,
				},
			},
		}

		rv := &resolver{
			cli: cli,
			res: resolved,
			fs:  fs,
		}

		err := rv.validateSecurity()
		require.NoError(t, err)

		logOutput := buf.String()
		assert.Contains(t, logOutput, "Container socket mounting is enabled. Granting access to the container runtime socket is highly privileged")
		// Ensure exactly ONE warning is logged, i.e., no duplicates from MountSocket and the mounts loop
		assert.Equal(t, 1, strings.Count(logOutput, "Container socket mounting is enabled"))
	})

	t.Run("Manual bind mount of docker socket with numeric group GID warning check", func(t *testing.T) {
		var buf bytes.Buffer
		logging.GetGlobalLogger().SetLevel(logging.WarnLevel)
		logging.GetGlobalLogger().SetOutput(&buf)

		cli := &CLIOptions{}
		resolved := &ResolvedConfig{
			Image:      "alpine",
			Engine:     "docker",
			Runtime:    "docker",
			SocketPath: "/var/run/docker.sock",
			GroupAdd:   []string{"1001"}, // Numeric group GID
			Mounts: []container.Mount{
				{
					Type:     "bind",
					Source:   "/var/run/docker.sock",
					Target:   "/var/run/docker.sock",
					ReadOnly: false,
				},
			},
		}

		rv := &resolver{
			cli: cli,
			res: resolved,
			fs:  fs,
		}

		err := rv.validateSecurity()
		require.NoError(t, err)

		logOutput := buf.String()
		assert.Contains(t, logOutput, "Granting container socket permissions through a numeric VM socket GID allows socket access but is highly privileged.")
	})
}
