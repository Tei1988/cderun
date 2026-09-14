package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/config"
)

func TestUnit_Config_ParameterValidators_ComprehensiveResilience(t *testing.T) {
	t.Parallel()

	t.Run("ValidateToolName", func(t *testing.T) {
		t.Parallel()
		validNames := []string{"git", "python3", "my-tool_v1", "tool.v2"}
		for _, name := range validNames {
			assert.NoError(t, config.ValidateToolName(name), "valid tool name: %s", name)
		}
		invalidNames := []string{"", "../tool", "tool/sub", "tool\x00", "tool\x1f", "tool name"}
		for _, name := range invalidNames {
			assert.Error(t, config.ValidateToolName(name), "invalid tool name: %s", name)
		}
	})

	t.Run("ValidateSysctlKey", func(t *testing.T) {
		t.Parallel()
		validKeys := []string{"net.ipv4.ip_forward", "kernel.pid_max", "net.core.somaxconn"}
		for _, key := range validKeys {
			assert.NoError(t, config.ValidateSysctlKey(key), "valid sysctl key: %s", key)
		}
		invalidKeys := []string{"", ".net.ipv4", "net..ipv4", "net.ipv4.", "net/ipv4", "net.ipv4\x07"}
		for _, key := range invalidKeys {
			assert.Error(t, config.ValidateSysctlKey(key), "invalid sysctl key: %s", key)
		}
	})

	t.Run("ValidateSysctlValue", func(t *testing.T) {
		t.Parallel()
		validVals := []string{"1", "65536", "0 0 0", "enabled"}
		for _, val := range validVals {
			assert.NoError(t, config.ValidateSysctlValue(val), "valid sysctl value: %s", val)
		}
		invalidVals := []string{"val\x00", "val\x1f"}
		for _, val := range invalidVals {
			assert.Error(t, config.ValidateSysctlValue(val), "invalid sysctl value: %s", val)
		}
	})

	t.Run("ValidateDNSOption", func(t *testing.T) {
		t.Parallel()
		validOptions := []string{"", "ndots:5", "timeout:2", "attempts:3", "edns0", "use-vc", "single-request"}
		for _, opt := range validOptions {
			assert.NoError(t, config.ValidateDNSOption(opt), "valid dns option: %s", opt)
		}
		invalidOptions := []string{"ndots:5!", "ndots:5;drop", "../traversal", "ndots\x00"}
		for _, opt := range invalidOptions {
			assert.Error(t, config.ValidateDNSOption(opt), "invalid dns option: %s", opt)
		}
	})

	t.Run("ValidateSecurityOpt", func(t *testing.T) {
		t.Parallel()
		validOpts := []string{"", "no-new-privileges:true", "seccomp=unconfined", "apparmor=unconfined", "label=disable"}
		for _, opt := range validOpts {
			assert.NoError(t, config.ValidateSecurityOpt(opt), "valid security opt: %s", opt)
		}
		invalidOpts := []string{"invalid_opt!", "no-new-privileges\x00", "../traversal"}
		for _, opt := range invalidOpts {
			assert.Error(t, config.ValidateSecurityOpt(opt), "invalid security opt: %s", opt)
		}
	})

	t.Run("ValidateGPUs", func(t *testing.T) {
		t.Parallel()
		validGPUs := []string{"", "all", "0", "1,2", "device=0,1", "count=2", "driver=nvidia"}
		for _, g := range validGPUs {
			assert.NoError(t, config.ValidateGPUs(g), "valid gpus: %s", g)
		}
		invalidGPUs := []string{",all", "all,", "all,,1", "gpus\x00", "device=0;1"}
		for _, g := range invalidGPUs {
			assert.Error(t, config.ValidateGPUs(g), "invalid gpus: %s", g)
		}
	})

	t.Run("ValidateCpuset", func(t *testing.T) {
		t.Parallel()
		validCpusets := []string{"", "0", "0-3", "0,2,4", "0-3,6-7"}
		for _, c := range validCpusets {
			assert.NoError(t, config.ValidateCpuset(c), "valid cpuset: %s", c)
		}
		invalidCpusets := []string{"-0", "0-", "0,,1", "0-3-4", "abc", "0-3\x00"}
		for _, c := range invalidCpusets {
			assert.Error(t, config.ValidateCpuset(c), "invalid cpuset: %s", c)
		}
	})

	t.Run("ValidateHostname", func(t *testing.T) {
		t.Parallel()
		validHostnames := []string{"", "my-host", "web-server-1", "host.domain.com"}
		for _, h := range validHostnames {
			assert.NoError(t, config.ValidateHostname(h), "valid hostname: %s", h)
		}
		invalidHostnames := []string{"-my-host", "my-host-", "host..domain", "host_name", "host\x00"}
		for _, h := range invalidHostnames {
			assert.Error(t, config.ValidateHostname(h), "invalid hostname: %s", h)
		}
	})

	t.Run("ValidateMountType", func(t *testing.T) {
		t.Parallel()
		validTypes := []string{"", "bind", "volume", "tmpfs"}
		for _, m := range validTypes {
			assert.NoError(t, config.ValidateMountType(m), "valid mount type: %s", m)
		}
		invalidTypes := []string{"nfs", "overlay", "bind\x00"}
		for _, m := range invalidTypes {
			assert.Error(t, config.ValidateMountType(m), "invalid mount type: %s", m)
		}
	})

	t.Run("ValidateAddHost", func(t *testing.T) {
		t.Parallel()
		validAddHosts := []string{"", "host.docker.internal:127.0.0.1", "myhost:10.0.0.1"}
		for _, ah := range validAddHosts {
			assert.NoError(t, config.ValidateAddHost(ah), "valid add host: %s", ah)
		}
		invalidAddHosts := []string{":127.0.0.1", "myhost:", "myhost:invalid_ip", "myhost\x00:127.0.0.1"}
		for _, ah := range invalidAddHosts {
			assert.Error(t, config.ValidateAddHost(ah), "invalid add host: %s", ah)
		}
	})

	t.Run("ValidateImageName", func(t *testing.T) {
		t.Parallel()
		validImages := []string{"", "alpine", "ubuntu:22.04", "registry.example.com/repo/image:tag@sha256:1234567890123456789012345678901234567890123456789012345678901234"}
		for _, img := range validImages {
			assert.NoError(t, config.ValidateImageName(img), "valid image: %s", img)
		}
		invalidImages := []string{"ubuntu/", "ubuntu:", "ubuntu@", "ubuntu//tag", "ubuntu::tag", "ubuntu@@tag", "ubuntu\x00"}
		for _, img := range invalidImages {
			assert.Error(t, config.ValidateImageName(img), "invalid image: %s", img)
		}
	})

	t.Run("ValidateEnvKey", func(t *testing.T) {
		t.Parallel()
		validKeys := []string{"FOO", "_FOO", "FOO_BAR_123"}
		for _, key := range validKeys {
			assert.NoError(t, config.ValidateEnvKey(key), "valid env key: %s", key)
		}
		invalidKeys := []string{"", "123FOO", "FOO-BAR", "FOO.BAR", "FOO\x00"}
		for _, key := range invalidKeys {
			assert.Error(t, config.ValidateEnvKey(key), "invalid env key: %s", key)
		}
	})
}

func TestUnit_Config_ExpressionResolver_ComprehensiveResilience(t *testing.T) {
	t.Setenv("RESILIENCE_TEST_VAR", "hello-world")

	r, err := config.NewExpressionResolver(nil)
	require.NoError(t, err)

	t.Run("Nested Fallbacks", func(t *testing.T) {
		res, err := r.ResolveString("{{env:UNSET_VAR_1:-{{env:UNSET_VAR_2:-fallback-value}}}}")
		require.NoError(t, err)
		assert.Equal(t, "fallback-value", res)
	})

	t.Run("Double Brace Escaping", func(t *testing.T) {
		res, err := r.ResolveString("{{ {{env:RESILIENCE_TEST_VAR}} }}")
		require.NoError(t, err)
		assert.Equal(t, "{{env:RESILIENCE_TEST_VAR}}", res)
	})
}

func TestUnit_Config_Masking_ComprehensiveResilience(t *testing.T) {
	t.Parallel()

	envs := []string{
		"API_KEY=secret-key-123",
		"DB_PASSWORD=super-secret",
		"USER_NAME=john_doe",
		"PUBLIC_VAR=public-value",
	}

	t.Run("Mode 1 Mask All (nil patterns)", func(t *testing.T) {
		t.Parallel()
		masked := config.MaskSensitiveEnvList(envs, nil)
		for _, env := range masked {
			assert.Contains(t, env, "=[REDACTED]")
		}
	})

	t.Run("Mode 2 Unmasked (empty pattern slice)", func(t *testing.T) {
		t.Parallel()
		unmasked := config.MaskSensitiveEnvList(envs, []string{})
		assert.Equal(t, envs, unmasked)
	})

	t.Run("Custom Glob Patterns", func(t *testing.T) {
		t.Parallel()
		patterns := []string{"*KEY*", "*PASSWORD*"}
		masked := config.MaskSensitiveEnvList(envs, patterns)
		assert.Contains(t, masked, "API_KEY=[REDACTED]")
		assert.Contains(t, masked, "DB_PASSWORD=[REDACTED]")
		assert.Contains(t, masked, "USER_NAME=john_doe")
		assert.Contains(t, masked, "PUBLIC_VAR=public-value")
	})
}
