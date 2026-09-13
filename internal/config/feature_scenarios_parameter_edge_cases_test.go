package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFeature_Config_ParameterValidations(t *testing.T) {
	t.Parallel()

	t.Run("ValidateCpuset boundary checks", func(t *testing.T) {
		t.Parallel()
		validCpusets := []string{"0", "0-3", "0,1,2", "0-3,6,7-10", ""}
		for _, cpuset := range validCpusets {
			assert.NoError(t, ValidateCpuset(cpuset), "cpuset %q should be valid", cpuset)
		}

		invalidCpusets := []string{"0;1", "0-3|4", "cpuset0", "0,1,2;ls"}
		for _, cpuset := range invalidCpusets {
			assert.Error(t, ValidateCpuset(cpuset), "cpuset %q should be invalid", cpuset)
		}
	})

	t.Run("ValidateGPUs boundary checks", func(t *testing.T) {
		t.Parallel()
		validGPUs := []string{"all", "1", "2,3", "device=0,1", "count=2", "driver=nvidia", ""}
		for _, gpu := range validGPUs {
			assert.NoError(t, ValidateGPUs(gpu), "gpu %q should be valid", gpu)
		}

		invalidGPUs := []string{"all;rm -rf /", "device=0&1", "count=2|sh", "driver=`id`"}
		for _, gpu := range invalidGPUs {
			assert.Error(t, ValidateGPUs(gpu), "gpu %q should be invalid", gpu)
		}
	})

	t.Run("ValidateDNSOption boundary checks", func(t *testing.T) {
		t.Parallel()
		validDNSOptions := []string{"ndots:5", "timeout:2", "attempts:3", "rotate", "single-request", ""}
		for _, opt := range validDNSOptions {
			assert.NoError(t, ValidateDNSOption(opt), "dns option %q should be valid", opt)
		}

		invalidDNSOptions := []string{"ndots:5;cat /etc/passwd", "timeout:2\n", "attempts:3\x00", "rotate|sh"}
		for _, opt := range invalidDNSOptions {
			assert.Error(t, ValidateDNSOption(opt), "dns option %q should be invalid", opt)
		}
	})

	t.Run("ValidateSysctlKey and ValidateSysctlValue boundary checks", func(t *testing.T) {
		t.Parallel()
		validKeys := []string{"net.ipv4.ip_forward", "kernel.shmmax", "fs.file-max"}
		for _, key := range validKeys {
			assert.NoError(t, ValidateSysctlKey(key), "sysctl key %q should be valid", key)
		}

		invalidKeys := []string{"", "net.ipv4;ls", "kernel.shmmax\x00"}
		for _, key := range invalidKeys {
			assert.Error(t, ValidateSysctlKey(key), "sysctl key %q should be invalid", key)
		}

		validValues := []string{"1", "0", "65536 131072", "4096 87380 16777216", ""}
		for _, val := range validValues {
			assert.NoError(t, ValidateSysctlValue(val), "sysctl value %q should be valid", val)
		}

		invalidValues := []string{"1;rm -rf /", "1\n2", "65536\x00", "4096|echo"}
		for _, val := range invalidValues {
			assert.Error(t, ValidateSysctlValue(val), "sysctl value %q should be invalid", val)
		}
	})

	t.Run("HasParentTraversal boundary checks", func(t *testing.T) {
		t.Parallel()
		traversalPaths := []string{"../foo", "foo/../bar", "foo/bar/..", "..", "/a/b/../../c"}
		for _, p := range traversalPaths {
			assert.True(t, HasParentTraversal(p), "path %q should be detected as parent traversal", p)
		}

		normalPaths := []string{"foo/bar", "foo..bar", ".foo", "foo/bar.", "/usr/local/bin", ""}
		for _, p := range normalPaths {
			assert.False(t, HasParentTraversal(p), "path %q should not be parent traversal", p)
		}
	})
}

func TestFeature_Config_ExpressionResolutionEdgeCases(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		Files: map[string][]byte{"/app/projects/myproj/config.json": []byte(`{"key": "value"}`)},
		Dirs:  map[string]bool{"/app/projects/myproj": true},
		WD:    "/app/projects/myproj",
	}

	resolver, err := NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

	t.Run("Complex nested expressions and fallbacks", func(t *testing.T) {
		t.Parallel()
		// Test env variable fallback
		resolved, err := resolver.ResolveString("{{env:NON_EXISTENT_VAR_XYZ:-default_value}}")
		require.NoError(t, err)
		assert.Equal(t, "default_value", resolved)

		// Test file directive
		resolvedFile, err := resolver.ResolveString("{{file:config.json}}")
		require.NoError(t, err)
		assert.Equal(t, `{"key": "value"}`, resolvedFile)

		// Test double brace escaping
		resolvedEscaped, err := resolver.ResolveString("{{ {{HOME}} }}")
		require.NoError(t, err)
		assert.Equal(t, "{{HOME}}", resolvedEscaped)
	})
}

func TestFeature_Config_SensitiveEnvMasking(t *testing.T) {
	t.Parallel()

	envVars := []string{
		"MY_API_KEY=secret123",
		"USER_TOKEN=tok_abc124",
		"PUBLIC_VAR=hello_world",
		"SECRET_PASSWORD=pass",
	}

	patterns := []string{"*API*", "USER_*", "*PASSWORD"}
	masked := MaskSensitiveEnvList(envVars, patterns)

	maskedMap := make(map[string]string)
	for _, entry := range masked {
		for i := 0; i < len(entry); i++ {
			if entry[i] == '=' {
				maskedMap[entry[:i]] = entry[i+1:]
				break
			}
		}
	}

	assert.Equal(t, "[REDACTED]", maskedMap["MY_API_KEY"])
	assert.Equal(t, "[REDACTED]", maskedMap["USER_TOKEN"])
	assert.Equal(t, "[REDACTED]", maskedMap["SECRET_PASSWORD"])
	assert.Equal(t, "hello_world", maskedMap["PUBLIC_VAR"])
}
