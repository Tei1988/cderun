package config

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_ExpressionResolver_NestedFallbacksAndStickyError(t *testing.T) {
	mockFS := &MockFileSystem{
		HomeDir: "/home/testuser",
		WD:      "/workspace/project",
		Env:     map[string]string{"FALLBACK_ENV": "/opt/fallback"},
	}

	hostCtx := &HostContext{
		Level:      1,
		HomeDir:    "/base/home",
		WorkingDir: "/base/pwd",
		UID:        "1001",
		GID:        "1001",
	}

	resolver, err := NewExpressionResolverWithFS(hostCtx, mockFS)
	require.NoError(t, err)

	t.Run("Nested Expression Fallbacks - find_dir to env fallback", func(t *testing.T) {
		// {{find_dir:nonexistent:-{{env:FALLBACK_ENV:-/default/path}}}}
		expr := "{{find_dir:nonexistent:-{{env:FALLBACK_ENV:-/default/path}}}}"
		res, err := resolver.ResolveString(expr)
		assert.NoError(t, err)
		assert.Equal(t, "/opt/fallback", res)
		assert.NoError(t, resolver.Error())
	})

	t.Run("Nested Expression Fallbacks - env to default fallback", func(t *testing.T) {
		expr := "{{env:MISSING_ENV:-{{env:FALLBACK_ENV:-/default/path}}}}"
		res, err := resolver.ResolveString(expr)
		assert.NoError(t, err)
		assert.Equal(t, "/opt/fallback", res)
		assert.NoError(t, resolver.Error())
	})

	t.Run("Sticky Error Isolation - error in earlier evaluation persists", func(t *testing.T) {
		cleanResolver, err := NewExpressionResolverWithFS(hostCtx, mockFS)
		require.NoError(t, err)

		// Invalid directive triggers sticky error
		_, err = cleanResolver.ResolveString("{{UNKNOWN_DIRECTIVE:foo}}")
		assert.Error(t, err)
		assert.Error(t, cleanResolver.Error())

		// Subsequent resolution call returns input unmodified due to sticky error
		res, err2 := cleanResolver.ResolveString("{{HOME}}")
		assert.Error(t, err2)
		assert.Equal(t, "{{HOME}}", res)
	})
}

func TestUnit_ExpressionResolver_MagicWordsAndHostContext(t *testing.T) {
	uid := 1000
	gid := 1000
	mockFS := &MockFileSystem{
		HomeDir:  "/home/localuser",
		WD:       "/local/project",
		UIDValue: &uid,
		GIDValue: &gid,
	}

	hostCtx := &HostContext{
		Level:      2,
		HomeDir:    "/base/home/user",
		WorkingDir: "/base/project",
		UID:        "2000",
		GID:        "2000",
	}

	resolver, err := NewExpressionResolverWithFS(hostCtx, mockFS)
	require.NoError(t, err)

	testCases := []struct {
		name     string
		expr     string
		expected string
	}{
		{"HOME", "{{HOME}}", "/home/localuser"},
		{"PWD", "{{PWD}}", "/local/project"},
		{"BASE_HOME", "{{BASE_HOME}}", "/base/home/user"},
		{"BASE_PWD", "{{BASE_PWD}}", "/base/project"},
		{"UID", "{{UID}}", "1000"},
		{"GID", "{{GID}}", "1000"},
		{"BASE_UID", "{{BASE_UID}}", "2000"},
		{"BASE_GID", "{{BASE_GID}}", "2000"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := resolver.ResolveString(tc.expr)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, res)
		})
	}
}

func TestUnit_ParameterSafetyValidators_Invariants(t *testing.T) {
	t.Run("ValidateNetworkName", func(t *testing.T) {
		assert.NoError(t, ValidateNetworkName("bridge"))
		assert.NoError(t, ValidateNetworkName("my-net_123"))
		assert.Error(t, ValidateNetworkName("net; rm -rf /"))
		assert.Error(t, ValidateNetworkName("net\x00null"))
	})

	t.Run("ValidateUserName", func(t *testing.T) {
		assert.NoError(t, ValidateUserName("1000:1000"))
		assert.NoError(t, ValidateUserName("root"))
		assert.Error(t, ValidateUserName("user; id"))
		assert.Error(t, ValidateUserName("user\nnewline"))
	})

	t.Run("ValidateAddHost", func(t *testing.T) {
		assert.NoError(t, ValidateAddHost("host.docker.internal:127.0.0.1"))
		assert.NoError(t, ValidateAddHost("myhost:10.0.0.1"))
		assert.Error(t, ValidateAddHost("invalid_format_no_colon"))
		assert.Error(t, ValidateAddHost("host:127.0.0.1; evil"))
	})

	t.Run("ValidateImageName", func(t *testing.T) {
		assert.NoError(t, ValidateImageName("alpine:latest"))
		assert.NoError(t, ValidateImageName("registry.example.com/org/repo:v1.0.0"))
		assert.Error(t, ValidateImageName("alpine:latest; touch /tmp/pwned"))
		assert.Error(t, ValidateImageName("alpine\x00:latest"))
	})

	t.Run("ValidatePort", func(t *testing.T) {
		assert.NoError(t, ValidatePort("8080:80"))
		assert.NoError(t, ValidatePort("127.0.0.1:8080:80/tcp"))
		assert.Error(t, ValidatePort("8080:80; echo pwned"))
	})

	t.Run("ValidateWorkdir", func(t *testing.T) {
		assert.NoError(t, ValidateWorkdir("/app/src"))
		assert.NoError(t, ValidateWorkdir("/workspace"))
		assert.Error(t, ValidateWorkdir("relative/dir"))
		assert.Error(t, ValidateWorkdir("/app; rm -rf /"))
	})

	t.Run("ValidateDNSOption", func(t *testing.T) {
		assert.NoError(t, ValidateDNSOption("ndots:5"))
		assert.NoError(t, ValidateDNSOption("timeout:2"))
		assert.Error(t, ValidateDNSOption("ndots:5; evil"))
	})

	t.Run("ValidateSecurityOpt", func(t *testing.T) {
		assert.NoError(t, ValidateSecurityOpt("seccomp=unconfined"))
		assert.NoError(t, ValidateSecurityOpt("no-new-privileges:true"))
		assert.Error(t, ValidateSecurityOpt("seccomp=unconfined; evil"))
	})

	t.Run("ValidateCpuset", func(t *testing.T) {
		assert.NoError(t, ValidateCpuset("0-3,6"))
		assert.NoError(t, ValidateCpuset("0,1"))
		assert.Error(t, ValidateCpuset("0-3; rm -rf /"))
	})

	t.Run("ValidateGPUs", func(t *testing.T) {
		assert.NoError(t, ValidateGPUs("all"))
		assert.NoError(t, ValidateGPUs("device=0,1"))
		assert.Error(t, ValidateGPUs("all; malicious"))
	})

	t.Run("ParseDeviceConfig", func(t *testing.T) {
		dev, ok := ParseDeviceConfig("/dev/sda:/dev/xda:r")
		assert.True(t, ok)
		assert.Equal(t, "/dev/sda", dev.Source.Raw)
		assert.Equal(t, "/dev/xda", dev.Destination.Raw)
		assert.Equal(t, "r", dev.Permissions)

		_, ok2 := ParseDeviceConfig("/dev/sda:/dev/xda:invalid_perms")
		assert.False(t, ok2)
	})
}

func TestUnit_ResolveWithFS_PrecedenceAndMasking(t *testing.T) {
	mockFS := &MockFileSystem{
		HomeDir: "/home/user",
		WD:      "/app",
		Env: map[string]string{
			"CDERUN_IMAGE": "alpine:3.18", // P2 Env var
		},
	}

	cliOpts := &CLIOptions{
		CderunImage: makeStrPtr("ubuntu:22.04"), // P1 CLI override
	}

	globalCfg := &CDERunConfig{
		Engine: "docker",
	}

	res, err := ResolveWithFS("python", cliOpts, nil, globalCfg, mockFS)
	require.NoError(t, err)
	assert.Equal(t, "ubuntu:22.04", res.Image, "CLI option (P1) must beat Env (P2)")

	t.Run("Sensitive env masking list", func(t *testing.T) {
		masked := MaskSensitiveEnvList([]string{
			"SECRET_KEY=supersecret",
			"API_TOKEN=123456",
			"PATH=/usr/bin:/bin",
		}, []string{"*KEY*", "*TOKEN*"})

		assert.Contains(t, masked, "SECRET_KEY=[REDACTED]")
		assert.Contains(t, masked, "API_TOKEN=[REDACTED]")
		assert.Contains(t, masked, "PATH=/usr/bin:/bin")
	})
}

func makeStrPtr(s string) *string {
	return &s
}

// Ensure unused import warning is suppressed if any
var _ = errors.New
