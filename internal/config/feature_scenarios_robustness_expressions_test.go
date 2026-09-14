package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_Robustness_DynamicExpressionsFallback(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD:      "/workspace",
		HomeDir: "/home/user",
		Env: map[string]string{
			"EXISTING_ENV": "active_value",
		},
	}

	resolver, err := NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

	// Fallback expression testing
	val := resolver.Resolve("{{env:NONEXISTENT_VAR:-fallback_value}}")
	assert.Equal(t, "fallback_value", val)

	valActive := resolver.Resolve("{{env:EXISTING_ENV:-fallback_value}}")
	assert.Equal(t, "active_value", valActive)
}

func TestUnit_Config_Robustness_SecurityValidators(t *testing.T) {
	t.Parallel()

	// ValidateCpuset
	require.NoError(t, ValidateCpuset("0-3"))
	require.NoError(t, ValidateCpuset("0,1,2"))
	require.Error(t, ValidateCpuset("0-3; rm -rf /"))
	require.Error(t, ValidateCpuset("cpuset0"))

	// ValidateGPUs
	require.NoError(t, ValidateGPUs("all"))
	require.NoError(t, ValidateGPUs("device=0,1"))
	require.NoError(t, ValidateGPUs("count=2"))
	require.Error(t, ValidateGPUs("all; drop database"))
	require.Error(t, ValidateGPUs("device=0&"))

	// ValidateSysctlKey
	require.NoError(t, ValidateSysctlKey("net.ipv4.ip_forward"))
	require.NoError(t, ValidateSysctlKey("kernel.shmmax"))
	require.Error(t, ValidateSysctlKey(".net.ipv4"))
	require.Error(t, ValidateSysctlKey("net..ipv4"))
	require.Error(t, ValidateSysctlKey("net.ipv4."))
	require.Error(t, ValidateSysctlKey("net/ipv4"))
}

func TestUnit_Config_Robustness_HostContextMountResolution(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD:      "/workspace/project/sub",
		HomeDir: "/home/user",
	}

	hc := &HostContext{
		Level: 1,
		Mounts: []MountMapping{
			{
				Source: "/workspace/project",
				Target: "/app",
			},
		},
	}

	resolver, err := NewExpressionResolverWithFS(hc, mfs)
	require.NoError(t, err)

	mc := MountConfig{
		Source: ConfigPath{Raw: "/app/sub/file.txt"},
		Target: ConfigPath{Raw: "/container/file.txt"},
	}

	resolvedMount, err := mc.Resolve(resolver)
	require.NoError(t, err)
	assert.Equal(t, "/workspace/project/sub/file.txt", resolvedMount.Source)
}
