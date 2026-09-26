package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_ExpressionResolution_FallbackAndMagicWords(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD:      "/workspace/app",
		HomeDir: "/home/testuser",
		Files: map[string][]byte{
			"/workspace/app/version.txt": []byte("1.2.3\n"),
		},
	}

	hostCtx := &HostContext{
		Level:      0,
		WorkingDir: "/workspace/app",
		HomeDir:    "/home/testuser",
		UID:        "1000",
		GID:        "1000",
	}

	t.Run("nested_fallback_with_magic_words", func(t *testing.T) {
		t.Parallel()

		resolver, err := NewExpressionResolverWithFS(hostCtx, mfs)
		require.NoError(t, err)

		// Test env directive fallback to {{HOME}}
		res, err := resolver.ResolveString("{{env:UNSET_ENV_VAR:-{{HOME}}}}")
		require.NoError(t, err)
		assert.Equal(t, "/home/testuser", res)

		// Test file directive fallback to {{PWD}}
		res, err = resolver.ResolveString("{{file:nonexistent.txt:-{{PWD}}}}")
		require.NoError(t, err)
		assert.Equal(t, "/workspace/app", res)

		// Test magic word resolution for UID and GID
		res, err = resolver.ResolveString("{{UID}}:{{GID}}")
		require.NoError(t, err)
		assert.Equal(t, "1000:1000", res)
	})

	t.Run("existing_file_directive_resolution", func(t *testing.T) {
		t.Parallel()

		resolver, err := NewExpressionResolverWithFS(hostCtx, mfs)
		require.NoError(t, err)

		res, err := resolver.ResolveString("{{file:version.txt:-default}}")
		require.NoError(t, err)
		assert.Equal(t, "1.2.3", res)
	})
}

func TestUnit_Config_ResolverValidation_BoundaryCases(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD:      "/workspace",
		HomeDir: "/home/user",
	}

	img := "alpine:latest"

	t.Run("valid_and_invalid_cpuset_cpus", func(t *testing.T) {
		t.Parallel()

		assert.NoError(t, ValidateCpuset("0-3,5"))
		assert.NoError(t, ValidateCpuset("0"))
		assert.Error(t, ValidateCpuset("invalid-cpuset!"))
	})

	t.Run("valid_and_invalid_sysctl_keys", func(t *testing.T) {
		t.Parallel()

		opts := &CLIOptions{
			Image:   &img,
			Sysctls: []string{"net.ipv4.ip_forward=1", "kernel.shmmax=18446744073709551615"},
		}

		resolved, err := ResolveWithFS("python3", opts, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, "1", resolved.Sysctls["net.ipv4.ip_forward"])
		assert.Equal(t, "18446744073709551615", resolved.Sysctls["kernel.shmmax"])
	})

	t.Run("resource_limits_boundary_values", func(t *testing.T) {
		t.Parallel()

		mem := "512m"
		pids := 100
		cpuShares := 1024

		opts := &CLIOptions{
			Image:     &img,
			Memory:    &mem,
			PidsLimit: &pids,
			CPUShares: &cpuShares,
		}

		resolved, err := ResolveWithFS("node", opts, nil, nil, mfs)
		require.NoError(t, err)

		assert.Equal(t, int64(512*1024*1024), resolved.Memory)
		assert.Equal(t, 100, resolved.PidsLimit)
		assert.Equal(t, 1024, resolved.CPUShares)
	})
}
