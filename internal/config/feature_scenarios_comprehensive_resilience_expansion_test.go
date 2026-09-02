package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/config"
)

// TestComprehensiveResilience_ParameterValidators tests boundary conditions and edge cases
// for input parameter validation functions in internal/config.
// Ref: docs/features/security-validations.md
func TestComprehensiveResilience_ParameterValidators(t *testing.T) {
	t.Parallel()

	t.Run("ValidateToolName", func(t *testing.T) {
		t.Parallel()
		validNames := []string{"node", "python3", "gcc-10", "tool_v1.2"}
		for _, name := range validNames {
			assert.NoError(t, config.ValidateToolName(name), "valid tool name: %s", name)
		}

		invalidNames := []string{
			"tool name",
			"tool/slash",
			"tool;cmd",
			"tool\x00null",
			"tool\nnewline",
			"../traversal",
		}
		for _, name := range invalidNames {
			assert.Error(t, config.ValidateToolName(name), "invalid tool name: %q", name)
		}
	})

	t.Run("ValidateHostname", func(t *testing.T) {
		t.Parallel()
		validHosts := []string{"localhost", "my-host", "host.domain.com", "node1"}
		for _, h := range validHosts {
			assert.NoError(t, config.ValidateHostname(h), "valid hostname: %s", h)
		}

		invalidHosts := []string{
			"host_name", // underscore not allowed in RFC1123 domain label
			"-host",
			"host-",
			"host..com",
			"host/slash",
			"host;cmd",
			"host\x00null",
		}
		for _, h := range invalidHosts {
			assert.Error(t, config.ValidateHostname(h), "invalid hostname: %q", h)
		}
	})

	t.Run("ValidateDNSOption", func(t *testing.T) {
		t.Parallel()
		validOpts := []string{"ndots:5", "timeout:1", "attempts:3", "rotate", "edns0", "use-vc"}
		for _, opt := range validOpts {
			assert.NoError(t, config.ValidateDNSOption(opt), "valid dns option: %s", opt)
		}

		invalidOpts := []string{
			"option;cmd",
			"ndots:5\x00null",
			"ndots:5\nnewline",
			"../../path",
		}
		for _, opt := range invalidOpts {
			assert.Error(t, config.ValidateDNSOption(opt), "invalid dns option: %q", opt)
		}
	})

	t.Run("ValidateSecurityOpt", func(t *testing.T) {
		t.Parallel()
		validOpts := []string{
			"no-new-privileges:true",
			"apparmor:unconfined",
			"seccomp=unconfined",
			"label:disable",
			"label=user:USER",
		}
		for _, opt := range validOpts {
			assert.NoError(t, config.ValidateSecurityOpt(opt), "valid security opt: %s", opt)
		}

		invalidOpts := []string{
			"seccomp;cmd",
			"label:disable\x00null",
			"apparmor\nnewline",
		}
		for _, opt := range invalidOpts {
			assert.Error(t, config.ValidateSecurityOpt(opt), "invalid security opt: %q", opt)
		}
	})

	t.Run("ValidateSysctlKeyAndValue", func(t *testing.T) {
		t.Parallel()
		validKeys := []string{"net.ipv4.ip_forward", "kernel.shmmax", "fs.file-max"}
		for _, k := range validKeys {
			assert.NoError(t, config.ValidateSysctlKey(k), "valid sysctl key: %s", k)
		}

		invalidKeys := []string{
			"net.ipv4..ip_forward",
			".net.ipv4",
			"net.ipv4.",
			"net/ipv4",
			"net;cmd",
			"net\x00null",
		}
		for _, k := range invalidKeys {
			assert.Error(t, config.ValidateSysctlKey(k), "invalid sysctl key: %q", k)
		}

		validValues := []string{"1", "0", "1 0 0 1", "65536"}
		for _, v := range validValues {
			assert.NoError(t, config.ValidateSysctlValue(v), "valid sysctl value: %s", v)
		}

		invalidValues := []string{
			"1;cmd",
			"1\x00null",
			"1\nnewline",
		}
		for _, v := range invalidValues {
			assert.Error(t, config.ValidateSysctlValue(v), "invalid sysctl value: %q", v)
		}
	})

	t.Run("ValidateGPUsAndCpuset", func(t *testing.T) {
		t.Parallel()
		validGPUs := []string{"all", "1", "0,1,2", "device=0,1"}
		for _, g := range validGPUs {
			assert.NoError(t, config.ValidateGPUs(g), "valid gpus: %s", g)
		}

		invalidGPUs := []string{
			"all;cmd",
			",1,2",
			"1,2,",
			"1,,2",
			"gpus\x00null",
		}
		for _, g := range invalidGPUs {
			assert.Error(t, config.ValidateGPUs(g), "invalid gpus: %q", g)
		}

		validCpusets := []string{"0", "0-3", "0,2-4", "0,1,2,3"}
		for _, c := range validCpusets {
			assert.NoError(t, config.ValidateCpuset(c), "valid cpuset: %s", c)
		}

		invalidCpusets := []string{
			"0-3-4",
			"0-",
			"-3",
			"0,,1",
			"0-3;cmd",
			"0\x00null",
		}
		for _, c := range invalidCpusets {
			assert.Error(t, config.ValidateCpuset(c), "invalid cpuset: %q", c)
		}
	})

	t.Run("ValidateAddHostAndImageName", func(t *testing.T) {
		t.Parallel()
		validAddHosts := []string{"myhost:127.0.0.1", "db.local:192.168.1.10", "ipv6host:::1"}
		for _, ah := range validAddHosts {
			assert.NoError(t, config.ValidateAddHost(ah), "valid add-host: %s", ah)
		}

		invalidAddHosts := []string{
			":127.0.0.1",
			"myhost:",
			"myhost;cmd:127.0.0.1",
			"myhost:127.0.0.1\x00null",
		}
		for _, ah := range invalidAddHosts {
			assert.Error(t, config.ValidateAddHost(ah), "invalid add-host: %q", ah)
		}

		validImages := []string{
			"ubuntu:latest",
			"golang:1.22-alpine",
			"ghcr.io/org/repo:v1.0.0",
			"my-registry.internal:5000/image:tag",
			"alpine@sha256:e4ed130b91e1d32a94488b39414f5e7141f2ff2b1a09d3ee978189c4a896a29d",
		}
		for _, img := range validImages {
			assert.NoError(t, config.ValidateImageName(img), "valid image name: %s", img)
		}

		invalidImages := []string{
			"ubuntu:",
			"ubuntu/",
			"ubuntu@",
			"ubuntu//tag",
			"ubuntu::tag",
			"ubuntu;cmd",
			"ubuntu\x00null",
		}
		for _, img := range invalidImages {
			assert.Error(t, config.ValidateImageName(img), "invalid image name: %q", img)
		}
	})

	t.Run("ValidateEnvKeyAndMountType", func(t *testing.T) {
		t.Parallel()
		validEnvKeys := []string{"FOO", "BAR_BAZ", "_KEY", "KEY_123"}
		for _, k := range validEnvKeys {
			assert.NoError(t, config.ValidateEnvKey(k), "valid env key: %s", k)
		}

		invalidEnvKeys := []string{
			"",
			"123KEY",
			"KEY-WITH-DASH",
			"KEY;CMD",
			"KEY\x00NULL",
		}
		for _, k := range invalidEnvKeys {
			assert.Error(t, config.ValidateEnvKey(k), "invalid env key: %q", k)
		}

		validMountTypes := []string{"bind", "volume", "tmpfs"}
		for _, m := range validMountTypes {
			assert.NoError(t, config.ValidateMountType(m), "valid mount type: %s", m)
		}

		invalidMountTypes := []string{
			"invalid",
			"bind;cmd",
			"volume\x00null",
		}
		for _, m := range invalidMountTypes {
			assert.Error(t, config.ValidateMountType(m), "invalid mount type: %q", m)
		}
	})
}

// TestComprehensiveResilience_ExpressionFallbackAndResolution tests expression parsing,
// fallback handling (:-default), and nested expression resolution.
// Ref: docs/features/value-resolution.md
func TestComprehensiveResilience_ExpressionFallbackAndResolution(t *testing.T) {
	t.Run("env expression fallback", func(t *testing.T) {
		r, err := config.NewExpressionResolver(nil)
		require.NoError(t, err)

		t.Setenv("TEST_RESILIENCE_VAR_EXISTING", "existing_val")

		res1, err := r.ResolveString("{{env:TEST_RESILIENCE_VAR_EXISTING:-default_val}}")
		require.NoError(t, err)
		assert.Equal(t, "existing_val", res1)

		res2, err := r.ResolveString("{{env:TEST_RESILIENCE_VAR_NON_EXISTING_XYZ:-default_val}}")
		require.NoError(t, err)
		assert.Equal(t, "default_val", res2)
	})

	t.Run("file expression with missing file and fallback", func(t *testing.T) {
		r, err := config.NewExpressionResolver(nil)
		require.NoError(t, err)

		res, err := r.ResolveString("{{file:non_existent_file_xyz.txt:-default_val}}")
		require.NoError(t, err)
		assert.Equal(t, "default_val", res)
	})

	t.Run("find_dir expression with missing dir and fallback", func(t *testing.T) {
		r, err := config.NewExpressionResolver(nil)
		require.NoError(t, err)

		res, err := r.ResolveString("{{find_dir:.non_existent_dir_12345:-fallback_dir}}")
		require.NoError(t, err)
		assert.Equal(t, "fallback_dir", res)
	})
}

// TestComprehensiveResilience_SensitiveEnvMasking tests sensitive env list masking logic.
// Ref: docs/features/sensitive-data-protection.md
func TestComprehensiveResilience_SensitiveEnvMasking(t *testing.T) {
	t.Parallel()

	envs := []string{
		"NORMAL_VAR=hello",
		"API_KEY=secret_123",
		"DB_PASSWORD=topsecret",
		"AWS_SECRET_ACCESS_KEY=aws_key",
		"PUBLIC_PORT=8080",
	}

	patterns := []string{"*KEY*", "*PASSWORD*", "*SECRET*"}
	masked := config.MaskSensitiveEnvList(envs, patterns)
	require.Len(t, masked, 5)

	assert.Equal(t, "NORMAL_VAR=hello", masked[0])
	assert.Equal(t, "API_KEY=[REDACTED]", masked[1])
	assert.Equal(t, "DB_PASSWORD=[REDACTED]", masked[2])
	assert.Equal(t, "AWS_SECRET_ACCESS_KEY=[REDACTED]", masked[3])
	assert.Equal(t, "PUBLIC_PORT=8080", masked[4])

	// Mask-all behavior with nil patterns
	maskedAll := config.MaskSensitiveEnvList(envs, nil)
	assert.Equal(t, "NORMAL_VAR=[REDACTED]", maskedAll[0])
	assert.Equal(t, "PUBLIC_PORT=[REDACTED]", maskedAll[4])
}

// TestComprehensiveResilience_PrecedenceMatrix tests resolution order across configuration sources.
// Ref: docs/features/argument-priority-logic.md
func TestComprehensiveResilience_PrecedenceMatrix(t *testing.T) {
	t.Parallel()

	t.Run("P1 CLI override precedence over P3 config file", func(t *testing.T) {
		t.Parallel()

		// P1 CLI explicitly specifies "ubuntu:22.04"
		cliOpts := config.CLIOptions{
			Image: strPtr("ubuntu:22.04"),
		}

		// P3 configuration specifies a distinct image tag "ubuntu:20.04" (same registry/repository)
		tools := config.ToolsConfig{
			"run": config.ToolConfig{
				Image: "ubuntu:20.04",
			},
		}
		global := &config.CDERunConfig{
			Defaults: config.ConfigDefaults{
				Network: "bridge",
			},
		}

		res, err := config.Resolve("run", &cliOpts, tools, global)
		require.NoError(t, err)

		// Assert P1 CLI override takes precedence over configured P3 value
		assert.Equal(t, "ubuntu:22.04", res.Image, "P1 CLI image override must take precedence over P3 config file image")
		assert.NotEqual(t, "ubuntu:20.04", res.Image, "Configured P3 image value must not override P1 CLI image")
	})
}

func strPtr(s string) *string {
	return &s
}
