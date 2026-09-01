package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeepCoverage_ConfigParameterValidators(t *testing.T) {
	t.Parallel()

	t.Run("ValidateToolName Boundary Cases", func(t *testing.T) {
		validTools := []string{"node", "python3", "cargo-clippy", "go_tool", "my.tool"}
		for _, name := range validTools {
			assert.NoError(t, ValidateToolName(name), "valid tool name should pass: %s", name)
		}

		invalidTools := []string{
			"",
			"tool/slash",
			"tool\\backslash",
			"tool:colon",
			"tool name",
			"tool\x00null",
			"tool\nnewline",
			"../relative",
		}
		for _, name := range invalidTools {
			assert.Error(t, ValidateToolName(name), "invalid tool name should fail: %q", name)
		}
	})

	t.Run("ValidateHostname & ValidateNetworkName", func(t *testing.T) {
		assert.NoError(t, ValidateHostname("my-host-123"))
		assert.NoError(t, ValidateHostname("sub.domain.local"))
		assert.NoError(t, ValidateHostname(""))
		assert.Error(t, ValidateHostname("host_name"))
		assert.Error(t, ValidateHostname("host\x01name"))

		assert.NoError(t, ValidateNetworkName("bridge"))
		assert.NoError(t, ValidateNetworkName("host"))
		assert.NoError(t, ValidateNetworkName("none"))
		assert.NoError(t, ValidateNetworkName("container:12345"))
		assert.NoError(t, ValidateNetworkName("ns:/proc/1/ns/net"))
		assert.Error(t, ValidateNetworkName("ns:/proc/../etc/passwd"))
		assert.Error(t, ValidateNetworkName("invalid_net\n"))
	})

	t.Run("ValidateDNSOption & ValidateSecurityOpt", func(t *testing.T) {
		assert.NoError(t, ValidateDNSOption("ndots:5"))
		assert.NoError(t, ValidateDNSOption("timeout:2"))
		assert.NoError(t, ValidateDNSOption("attempts:3"))
		assert.NoError(t, ValidateDNSOption(""))
		assert.Error(t, ValidateDNSOption("ndots:5\x00"))
		require.Error(t, ValidateDNSOption("ndots:5;bad"))

		assert.NoError(t, ValidateSecurityOpt("no-new-privileges:true"))
		assert.NoError(t, ValidateSecurityOpt("seccomp=unconfined"))
		assert.NoError(t, ValidateSecurityOpt(""))
		assert.Error(t, ValidateSecurityOpt("invalid\x08opt"))
		assert.Error(t, ValidateSecurityOpt("../etc/passwd"))
	})

	t.Run("ValidateSysctlKey & ValidateSysctlValue", func(t *testing.T) {
		assert.NoError(t, ValidateSysctlKey("net.ipv4.ip_forward"))
		assert.NoError(t, ValidateSysctlKey("kernel.shmmax"))
		assert.Error(t, ValidateSysctlKey(""))
		assert.Error(t, ValidateSysctlKey("net.ipv4.ip_forward; rm -rf /"))

		assert.NoError(t, ValidateSysctlValue("1"))
		assert.NoError(t, ValidateSysctlValue("268435456"))
		assert.Error(t, ValidateSysctlValue("val\n"))
	})

	t.Run("ValidateGPUs & ValidateCpuset", func(t *testing.T) {
		assert.NoError(t, ValidateGPUs("all"))
		assert.NoError(t, ValidateGPUs("device=0,1"))
		assert.NoError(t, ValidateGPUs("count=2"))
		assert.NoError(t, ValidateGPUs(""))
		assert.Error(t, ValidateGPUs("device=0,1;bad"))
		assert.Error(t, ValidateGPUs(",all"))

		assert.NoError(t, ValidateCpuset("0-3"))
		assert.NoError(t, ValidateCpuset("0,2,4"))
		assert.NoError(t, ValidateCpuset("0-3,5"))
		assert.NoError(t, ValidateCpuset(""))
		assert.Error(t, ValidateCpuset("0-3-4"))
		assert.Error(t, ValidateCpuset("0,,"))
		assert.Error(t, ValidateCpuset("-0-3"))
	})

	t.Run("ValidateAddHost & ValidateImageName", func(t *testing.T) {
		assert.NoError(t, ValidateAddHost("myhost:127.0.0.1"))
		assert.NoError(t, ValidateAddHost("db.local:10.0.0.5"))
		assert.Error(t, ValidateAddHost(":127.0.0.1"))
		assert.Error(t, ValidateAddHost("myhost:"))
		assert.Error(t, ValidateAddHost("invalid_host"))

		assert.NoError(t, ValidateImageName("alpine:latest"))
		assert.NoError(t, ValidateImageName("ubuntu@sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"))
		assert.NoError(t, ValidateImageName("ghcr.io/org/repo:v1.0.0"))
		assert.NoError(t, ValidateImageName(""))
		assert.Error(t, ValidateImageName("alpine:"))
		assert.Error(t, ValidateImageName("ubuntu/"))
		assert.Error(t, ValidateImageName("ubuntu::latest"))
		assert.Error(t, ValidateImageName("../alpine:latest"))
	})

	t.Run("ValidateEnvKey & ValidateMountType", func(t *testing.T) {
		assert.NoError(t, ValidateEnvKey("PATH"))
		assert.NoError(t, ValidateEnvKey("_MY_VAR_1"))
		assert.Error(t, ValidateEnvKey(""))
		assert.Error(t, ValidateEnvKey("123NUM"))
		assert.Error(t, ValidateEnvKey("VAR-WITH-HYPHEN"))

		assert.NoError(t, ValidateMountType("bind"))
		assert.NoError(t, ValidateMountType("volume"))
		assert.NoError(t, ValidateMountType("tmpfs"))
		assert.Error(t, ValidateMountType("invalid"))
	})

	t.Run("ValidateWorkdir & ValidateUserName & ValidateGroupAdd & ValidatePort", func(t *testing.T) {
		assert.NoError(t, ValidateWorkdir("/app"))
		assert.Error(t, ValidateWorkdir("app\x00relative"))

		assert.NoError(t, ValidateUserName("root"))
		assert.NoError(t, ValidateUserName("1000:1000"))
		assert.Error(t, ValidateUserName("user\nname"))

		assert.NoError(t, ValidateGroupAdd("sudo"))
		assert.NoError(t, ValidateGroupAdd("1001"))
		assert.Error(t, ValidateGroupAdd("group\x00bad"))

		assert.NoError(t, ValidatePort("8080:80"))
		assert.NoError(t, ValidatePort("8080"))
		assert.Error(t, ValidatePort("invalid_port"))
	})
}

func TestDeepCoverage_TemplateExpressionFallbacksAndEscaping(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{WD: "/base", HomeDir: "/base/home"}
	r, err := NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

	t.Run("Env Fallback Directive", func(t *testing.T) {
		val, err := r.ResolveString("{{env:CDERUN_TEST_NONEXISTENT_KEY:-default_value}}")
		require.NoError(t, err)
		assert.Equal(t, "default_value", val)
	})

	t.Run("File Fallback Directive Nonexistent File", func(t *testing.T) {
		val, err := r.ResolveString("{{file:nonexistent.txt:-fallback_content}}")
		require.NoError(t, err)
		assert.Equal(t, "fallback_content", val)
	})

	t.Run("FindDir Fallback Directive Nonexistent Item", func(t *testing.T) {
		val, err := r.ResolveString("{{find_dir:nonexistent_marker_dir:-/fallback/path}}")
		require.NoError(t, err)
		assert.Equal(t, "/fallback/path", val)
	})

	t.Run("Double Brace Escaping", func(t *testing.T) {
		val, err := r.ResolveString("{{{{HOME}}}}")
		require.NoError(t, err)
		assert.Contains(t, val, "{{")
	})
}

func TestDeepCoverage_SensitiveEnvMaskingInvariants(t *testing.T) {
	t.Parallel()

	envList := []string{
		"PATH=/usr/bin:/bin",
		"API_KEY=secret_key_12345",
		"AWS_SECRET_ACCESS_KEY=supersecret",
		"DB_PASSWORD=my_password",
		"NORMAL_VAR=hello",
	}

	// Default mask-all behavior when patterns is nil
	masked := MaskSensitiveEnvList(envList, nil)
	require.Len(t, masked, len(envList))
	for _, item := range masked {
		assert.Contains(t, item, "[REDACTED]")
	}

	// Keyword-based pattern matching when explicit patterns provided
	customKeywords := []string{"*KEY*", "*PASSWORD*"}
	customMasked := MaskSensitiveEnvList(envList, customKeywords)
	assert.Contains(t, customMasked[1], "[REDACTED]") // API_KEY
	assert.Contains(t, customMasked[2], "[REDACTED]") // AWS_SECRET_ACCESS_KEY
	assert.Contains(t, customMasked[3], "[REDACTED]") // DB_PASSWORD
	assert.Equal(t, "PATH=/usr/bin:/bin", customMasked[0])
	assert.Equal(t, "NORMAL_VAR=hello", customMasked[4])
}

func TestDeepCoverage_FullPrecedenceResolutionMatrix(t *testing.T) {
	t.Parallel()

	p1Val := "p1-image"

	mfs := &MockFileSystem{WD: "/base", Env: map[string]string{"CDERUN_IMAGE": "p2-image"}}

	cliOpts := &CLIOptions{
		Image: &p1Val,
	}

	resolved, err := ResolveWithFS("node", cliOpts, nil, nil, mfs)
	require.NoError(t, err)
	assert.Equal(t, "p1-image", resolved.Image, "P1 CLI options should override env vars")
}
