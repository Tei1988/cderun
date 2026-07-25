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

// TestUnit_Config_Resolver_InvalidEnvValues verifies that invalid bool, int, and float64
// values supplied via environment variables correctly trigger an InvalidConfigError during resolution.
func TestUnit_Config_Resolver_InvalidEnvValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		envKey        string
		envVal        string
		expectedField string
	}{
		{
			name:          "invalid boolean value for CDERUN_TTY",
			envKey:        "CDERUN_TTY",
			envVal:        "yes",
			expectedField: "CDERUN_TTY",
		},
		{
			name:          "invalid boolean value for CDERUN_INTERACTIVE",
			envKey:        "CDERUN_INTERACTIVE",
			envVal:        "invalid",
			expectedField: "CDERUN_INTERACTIVE",
		},
		{
			name:          "invalid integer value for CDERUN_PULL_MAX_RETRIES",
			envKey:        "CDERUN_PULL_MAX_RETRIES",
			envVal:        "abc",
			expectedField: "CDERUN_PULL_MAX_RETRIES",
		},
		{
			name:          "invalid float value for CDERUN_CPUS",
			envKey:        "CDERUN_CPUS",
			envVal:        "two",
			expectedField: "CDERUN_CPUS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mfs := &MockFileSystem{
				Env: map[string]string{
					tt.envKey: tt.envVal,
				},
			}
			cli := &CLIOptions{
				Image:    "alpine",
				ImageSet: true,
			}

			_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
			require.Error(t, err)

			var cfgErr *InvalidConfigError
			require.ErrorAs(t, err, &cfgErr)
			assert.Equal(t, tt.expectedField, cfgErr.Field)
			assert.Equal(t, tt.envVal, cfgErr.Value)
		})
	}
}

func TestUnit_Config_Resolver_ReadOnly(t *testing.T) {
	t.Run("resolved via CLI flag", func(t *testing.T) {
		mfs := &MockFileSystem{}
		cli := &CLIOptions{
			Image:       "alpine",
			ImageSet:    true,
			ReadOnly:    true,
			ReadOnlySet: true,
		}
		res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)
		assert.True(t, res.ReadOnly)
	})

	t.Run("resolved via Cderun CLI flag (priority)", func(t *testing.T) {
		mfs := &MockFileSystem{}
		cli := &CLIOptions{
			Image:             "alpine",
			ImageSet:          true,
			ReadOnly:          false,
			ReadOnlySet:       true,
			CderunReadOnly:    true,
			CderunReadOnlySet: true,
		}
		res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)
		assert.True(t, res.ReadOnly)
	})

	t.Run("resolved via environment variable", func(t *testing.T) {
		mfs := &MockFileSystem{
			Env: map[string]string{
				"CDERUN_READ_ONLY": "true",
			},
		}
		cli := &CLIOptions{
			Image:    "alpine",
			ImageSet: true,
		}
		res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)
		assert.True(t, res.ReadOnly)
	})

	t.Run("resolved via global defaults", func(t *testing.T) {
		mfs := &MockFileSystem{}
		cli := &CLIOptions{
			Image:    "alpine",
			ImageSet: true,
		}
		global := &CDERunConfig{}
		trueVal := true
		global.Defaults.ReadOnly = &trueVal

		res, err := ResolveWithFS("sh", cli, nil, global, mfs)
		require.NoError(t, err)
		assert.True(t, res.ReadOnly)
	})

	t.Run("resolved via tool config", func(t *testing.T) {
		mfs := &MockFileSystem{}
		cli := &CLIOptions{
			Image:    "alpine",
			ImageSet: true,
		}
		trueVal := true
		tools := ToolsConfig{
			"sh": ToolConfig{
				ReadOnly: &trueVal,
			},
		}

		res, err := ResolveWithFS("sh", cli, tools, nil, mfs)
		require.NoError(t, err)
		assert.True(t, res.ReadOnly)
	})
}

// TestUnit_Config_Resolver_RealFileSystemCaching verifies that the auto-detected
// container runtime and socket path are successfully cached on RealFileSystem.
func TestUnit_Config_Resolver_RealFileSystemCaching(t *testing.T) {
	// 1. Save existing cache globals to restore them after the test
	origRuntime := autoDetectedRuntime
	origSocketPath := autoDetectedSocketPath

	t.Cleanup(func() {
		autoDetectMu.Lock()
		autoDetectedRuntime = origRuntime
		autoDetectedSocketPath = origSocketPath
		autoDetectMu.Unlock()
	})

	// 2. Clear relevant CDERUN_SOCKET_PATH and runtime override environment variables
	t.Setenv("CDERUN_SOCKET_PATH", "")
	t.Setenv("CDERUN_RUNTIME", "")

	// 3. Ensure the cache is reset before ResolveWithFS runs
	autoDetectMu.Lock()
	autoDetectedRuntime = ""
	autoDetectedSocketPath = ""
	autoDetectMu.Unlock()

	cli := &CLIOptions{Image: "alpine", ImageSet: true}
	res, err := ResolveWithFS("sh", cli, nil, nil, RealFileSystem{})
	require.NoError(t, err)

	autoDetectMu.RLock()
	cachedRuntime := autoDetectedRuntime
	cachedSocketPath := autoDetectedSocketPath
	autoDetectMu.RUnlock()

	// 4. Check standard sockets using RealFileSystem to verify the correctness of the single observation.
	var socketExists bool
	var firstExistingRuntime string
	var firstExistingSocketPath string
	var rfs RealFileSystem

	for _, p := range []string{"/var/run/docker.sock", "/run/containerd/containerd.sock", "/run/podman/podman.sock"} {
		if _, err := rfs.Stat(p); err == nil {
			socketExists = true
			switch p {
			case "/var/run/docker.sock":
				firstExistingRuntime = "docker"
			case "/run/containerd/containerd.sock":
				firstExistingRuntime = "containerd"
			default:
				firstExistingRuntime = "podman"
			}
			firstExistingSocketPath = p
			break
		}
	}

	if socketExists {
		// Assert that ResolveWithFS successfully initialized both cache globals matching the host state.
		assert.NotEmpty(t, cachedRuntime, "Cache globals should be initialized since a socket exists")
		assert.NotEmpty(t, cachedSocketPath, "Cache globals should be initialized since a socket exists")
		assert.Equal(t, firstExistingRuntime, cachedRuntime, "Cached runtime should match first existing container socket")
		assert.Equal(t, firstExistingSocketPath, cachedSocketPath, "Cached socket path should match first existing container socket")
		assert.Equal(t, cachedRuntime, res.Runtime, "Resolved runtime should match cached runtime")
		assert.Equal(t, cachedSocketPath, res.SocketPath, "Resolved socket path should match cached socket path")
	} else {
		// Assert that cache remains empty if no socket exists, avoiding flaky failures in clean runner environments.
		assert.Empty(t, cachedRuntime, "Cache globals should remain empty if no socket exists")
		assert.Empty(t, cachedSocketPath, "Cache globals should remain empty if no socket exists")
		assert.Equal(t, "docker", res.Runtime, "Fell back to default runtime")
		assert.Equal(t, "/var/run/docker.sock", res.SocketPath, "Fell back to default socket path")
	}
}
