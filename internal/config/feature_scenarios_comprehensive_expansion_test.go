package config

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/container"
	"cderun/internal/logging"
)

// docs/features/security-validations.md: Least Privilege Capabilities
// Test that highly privileged capabilities are correctly recognized.
func TestUnit_Config_IsHighlyPrivilegedCapability(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		capName  string
		expected bool
	}{
		{"SYS_ADMIN", true},
		{"sys_admin", true},
		{" CAP_SYS_ADMIN ", true},
		{"ALL", true},
		{"cap_all", true},
		{"CHROOT", false}, // not highly privileged or not standard
		{"SYS_CHROOT", true},
		{"CAP_SYS_CHROOT", true},
		{"NET_RAW", false}, // not standard in highlyPrivilegedCaps
		{"AUDIT_CONTROL", true},
		{"CAP_AUDIT_CONTROL", true},
		{"PERFMON", true},
		{"CAP_PERFMON", true},
		{"SYS_RESOURCE", true},
		{"CAP_SYS_RESOURCE", true},
	}

	for _, tc := range testCases {
		t.Run(tc.capName, func(t *testing.T) {
			assert.Equal(t, tc.expected, isHighlyPrivilegedCapability(tc.capName))
		})
	}
}

// docs/features/security-validations.md: Device and Path Security
// Test that highly sensitive host devices are correctly recognized.
func TestUnit_Config_IsHighlySensitiveDevice(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		devicePath string
		expected   bool
	}{
		{"/dev/mem", true},
		{"/dev/kmem", true},
		{"/dev/port", true},
		{"/dev/sda", true},
		{"/dev/sdb1", true},
		{"/dev/nvme0n1", true},
		{"/dev/loop0", true},
		{"/dev/mapper/crypt", true},
		{"/etc/passwd", false},
		{"/dev/null", false},
		{"/dev/zero", false},
		{"/dev/random", false},
	}

	for _, tc := range testCases {
		t.Run(tc.devicePath, func(t *testing.T) {
			assert.Equal(t, tc.expected, isHighlySensitiveDevice(tc.devicePath))
		})
	}
}

// docs/features/security-validations.md: Security Warnings and Privilege Minimization
// Test that validateSecurity emits appropriate warnings under various risk conditions.
func TestUnit_Config_ValidateSecurity_Warnings(t *testing.T) {
	// Do not run in parallel because we hijack the global logger's output.
	oldWriter := logging.GetGlobalLogger().GetWriter()
	defer func() {
		logging.SetOutput(oldWriter)
		_ = logging.Init("error", "text", true)
	}()

	buf := &bytes.Buffer{}
	logging.SetOutput(buf)
	err := logging.Init("warn", "text", false)
	require.NoError(t, err)

	mfs := &MockFileSystem{
		WD: "/work",
	}

	res := &ResolvedConfig{
		Image:        "alpine",
		Runtime:      "docker",
		Network:      "host",
		Pid:          "host",
		Privileged:   true,
		CapAdd:       []string{"SYS_ADMIN", "NET_ADMIN"},
		MountSocket:  true,
		GroupAdd:     []string{"1001"}, // Contains numeric VM socket GID
		Mounts: []container.Mount{
			{Type: "bind", Source: "/etc", Target: "/etc_in_container"},
			{Type: "bind", Source: "/", Target: "/host_root"},
		},
		Devices: []container.DeviceMapping{
			{PathOnHost: "/dev/mem", PathInContainer: "/dev/mem"},
		},
	}

	// Always initialize FieldInfo once to prevent reflection panics in fetchFieldAndParams
	fieldOnce.Do(initFieldInfo)

	rv := &resolver{
		subcommand: "sh",
		fs:         mfs,
		cli:        &CLIOptions{},
		res:        res,
	}

	err = rv.validateSecurity()
	require.NoError(t, err)

	output := buf.String()

	// Verify all expected warnings are present
	assert.Contains(t, output, "Container is running in privileged mode")
	assert.Contains(t, output, "Highly privileged capability [SYS_ADMIN NET_ADMIN] detected in CapAdd while running in privileged mode")
	assert.Contains(t, output, "Container is running with host network mode enabled")
	assert.Contains(t, output, "Container is running with host PID namespace enabled")
	assert.Contains(t, output, "Mounting highly sensitive host path \"/etc\" into the container")
	assert.Contains(t, output, "Mounting highly sensitive host path \"/\" into the container")
	assert.Contains(t, output, "Mounting highly sensitive host device \"/dev/mem\"")
	assert.Contains(t, output, "Container socket mounting is enabled")
	assert.Contains(t, output, "Granting container socket permissions through a numeric VM socket GID")
}

// docs/features/security-validations.md: Critical Fields Validation
// Test that validateCriticalFields strictly validates and rejects unsupported inputs.
func TestUnit_Config_ValidateCriticalFields_Errors(t *testing.T) {
	t.Parallel()

	// Always initialize FieldInfo once to prevent reflection panics in fetchFieldAndParams
	fieldOnce.Do(initFieldInfo)

	mfs := &MockFileSystem{
		WD: "/work",
	}

	t.Run("unsupported PID namespace", func(t *testing.T) {
		res := &ResolvedConfig{
			Image:   "alpine",
			Pid:     "container:foo", // unsupported
			Runtime: "docker",
		}
		rv := &resolver{res: res, fs: mfs, cli: &CLIOptions{}}
		err := rv.validateCriticalFields()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported pid namespace")
	})

	t.Run("unsupported runtime", func(t *testing.T) {
		res := &ResolvedConfig{
			Image:   "alpine",
			Runtime: "unsupported_runtime",
		}
		rv := &resolver{res: res, fs: mfs, cli: &CLIOptions{}}
		err := rv.validateCriticalFields()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported runtime")
	})

	t.Run("unsupported pull policy", func(t *testing.T) {
		res := &ResolvedConfig{
			Image:   "alpine",
			Runtime: "docker",
			Pull:    "always-on-failure", // unsupported
		}
		rv := &resolver{res: res, fs: mfs, cli: &CLIOptions{}}
		err := rv.validateCriticalFields()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid pull policy")
	})

	t.Run("negative Memory", func(t *testing.T) {
		res := &ResolvedConfig{
			Image:   "alpine",
			Runtime: "docker",
			Memory:  -500,
		}
		rv := &resolver{res: res, fs: mfs, cli: &CLIOptions{}}
		err := rv.validateCriticalFields()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "memory limit cannot be negative")
	})

	t.Run("negative CPUs", func(t *testing.T) {
		res := &ResolvedConfig{
			Image:   "alpine",
			Runtime: "docker",
			CPUs:    -1.5,
		}
		rv := &resolver{res: res, fs: mfs, cli: &CLIOptions{}}
		err := rv.validateCriticalFields()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "CPU limit cannot be negative")
	})

	t.Run("invalid image name", func(t *testing.T) {
		res := &ResolvedConfig{
			Image:   "illegal_chars_#_not_allowed",
			Runtime: "docker",
		}
		rv := &resolver{res: res, fs: mfs, cli: &CLIOptions{}}
		err := rv.validateCriticalFields()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid image name")
	})

	t.Run("invalid username", func(t *testing.T) {
		res := &ResolvedConfig{
			Image:   "alpine",
			Runtime: "docker",
			User:    "user*invalid",
		}
		rv := &resolver{res: res, fs: mfs, cli: &CLIOptions{}}
		err := rv.validateCriticalFields()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid user or group identifier")
	})

	t.Run("invalid hostname", func(t *testing.T) {
		res := &ResolvedConfig{
			Image:    "alpine",
			Runtime:  "docker",
			Hostname: "host_name_invalid", // underscores not allowed in hostname
		}
		rv := &resolver{res: res, fs: mfs, cli: &CLIOptions{}}
		err := rv.validateCriticalFields()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid hostname")
	})

	t.Run("invalid workdir", func(t *testing.T) {
		res := &ResolvedConfig{
			Image:   "alpine",
			Runtime: "docker",
			Workdir: "relative/path", // workdir must be absolute
		}
		rv := &resolver{res: res, fs: mfs, cli: &CLIOptions{}}
		err := rv.validateCriticalFields()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "working directory must be an absolute path")
	})

	t.Run("invalid mount-socket-path", func(t *testing.T) {
		res := &ResolvedConfig{
			Image:           "alpine",
			Runtime:         "docker",
			MountSocketPath: "relative/path", // must be absolute
		}
		rv := &resolver{res: res, fs: mfs, cli: &CLIOptions{}}
		err := rv.validateCriticalFields()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "mount target must be an absolute path")
	})
}

// docs/features/security-validations.md: Slice Field Element Validation
// Test that slice fields such as expose, publish, DNS, add-host, cap-add, and group-add are validated.
func TestUnit_Config_ValidateSlices_Errors(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD: "/work",
	}

	t.Run("invalid expose port format", func(t *testing.T) {
		res := &ResolvedConfig{
			Image:   "alpine",
			Runtime: "docker",
			Expose:  []string{"999999"}, // out of port range
		}
		rv := &resolver{res: res, fs: mfs, cli: &CLIOptions{}}
		err := rv.validateSlices()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid port")
	})

	t.Run("invalid publish port format", func(t *testing.T) {
		res := &ResolvedConfig{
			Image:   "alpine",
			Runtime: "docker",
			Ports:   []string{"abc:123"},
		}
		rv := &resolver{res: res, fs: mfs, cli: &CLIOptions{}}
		err := rv.validateSlices()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid host port")
	})

	t.Run("invalid DNS server", func(t *testing.T) {
		res := &ResolvedConfig{
			Image:   "alpine",
			Runtime: "docker",
			DNS:     []string{"999.999.999.999"},
		}
		rv := &resolver{res: res, fs: mfs, cli: &CLIOptions{}}
		err := rv.validateSlices()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid DNS IP")
	})

	t.Run("invalid add-host entry", func(t *testing.T) {
		res := &ResolvedConfig{
			Image:    "alpine",
			Runtime:  "docker",
			AddHosts: []string{"host_invalid_format"},
		}
		rv := &resolver{res: res, fs: mfs, cli: &CLIOptions{}}
		err := rv.validateSlices()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid add-host format")
	})

	t.Run("invalid cap-add capability", func(t *testing.T) {
		res := &ResolvedConfig{
			Image:   "alpine",
			Runtime: "docker",
			CapAdd:  []string{"INVALID_CAP_NAME*"},
		}
		rv := &resolver{res: res, fs: mfs, cli: &CLIOptions{}}
		err := rv.validateSlices()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid Linux capability")
	})

	t.Run("invalid group-add supplementary group", func(t *testing.T) {
		res := &ResolvedConfig{
			Image:    "alpine",
			Runtime:  "docker",
			GroupAdd: []string{"group*with*illegal*chars"},
		}
		rv := &resolver{res: res, fs: mfs, cli: &CLIOptions{}}
		err := rv.validateSlices()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid supplementary group name")
	})
}

// docs/features/security-validations.md: Path Character Protection
// Test validatePathChars with various strings.
func TestUnit_Config_ValidatePathChars_Direct(t *testing.T) {
	t.Parallel()

	valid := []string{
		"hello_world",
		"foo/bar/baz.txt",
		"C:\\Windows\\System32",
		"~/.config",
		"123-456",
		"日本語", // Unicode is valid
	}

	invalid := []string{
		"null\x00byte",
		"control\x01char",
		"control\x1fchar",
		"delete\x7fchar",
	}

	for _, s := range valid {
		t.Run(fmt.Sprintf("valid_%s", s), func(t *testing.T) {
			assert.NoError(t, validatePathChars(s))
		})
	}

	for _, s := range invalid {
		t.Run(fmt.Sprintf("invalid_%x", s), func(t *testing.T) {
			err := validatePathChars(s)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid character")
		})
	}
}

// docs/features/security-validations.md: Sysctl Parameters Security
// Test resolveSysctls with malformed patterns and null byte injections.
func TestUnit_Config_ResolveSysctls_EdgeCases(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD: "/work",
	}

	t.Run("missing value format", func(t *testing.T) {
		_, err := resolveSysctls([]string{"net.ipv4.ip_forward"}, nil, "sh", nil, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be in key=value format")
	})

	t.Run("empty key format", func(t *testing.T) {
		_, err := resolveSysctls([]string{"=1"}, nil, "sh", nil, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be in key=value format")
	})

	t.Run("null byte key", func(t *testing.T) {
		_, err := resolveSysctls([]string{"net.ipv4\x00.ip_forward=1"}, nil, "sh", nil, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "null byte injection detected")
	})

	t.Run("null byte value", func(t *testing.T) {
		_, err := resolveSysctls([]string{"net.ipv4.ip_forward=1\x00"}, nil, "sh", nil, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "null byte injection detected")
	})
}

// docs/features/security-validations.md: Ulimit Configuration Validation
// Test resolveUlimits with invalid limit boundaries.
func TestUnit_Config_ResolveUlimits_EdgeCases(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD: "/work",
	}

	t.Run("invalid ulimit format", func(t *testing.T) {
		_, err := resolveUlimits([]string{"nofile=abc"}, nil, "sh", nil, nil, mfs)
		require.Error(t, err)
	})

	t.Run("ulimit values below -1", func(t *testing.T) {
		_, err := resolveUlimits([]string{"nofile=-2"}, nil, "sh", nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "limit values must be at least -1")
	})
}
