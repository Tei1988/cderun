package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFeatureScenarios_AdvancedResilienceRefinementExpansion_Validators(t *testing.T) {
	t.Parallel()

	t.Run("ValidateDNSOption", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, ValidateDNSOption("ndots:5"))
		require.NoError(t, ValidateDNSOption("timeout:2"))
		require.NoError(t, ValidateDNSOption("attempts:3"))
		require.Error(t, ValidateDNSOption("invalid\x00opt"))
	})

	t.Run("ValidateSecurityOpt", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, ValidateSecurityOpt("no-new-privileges:true"))
		require.NoError(t, ValidateSecurityOpt("seccomp=unconfined"))
		require.NoError(t, ValidateSecurityOpt("apparmor=unconfined"))
		require.Error(t, ValidateSecurityOpt("invalid\x00sec"))
	})

	t.Run("ValidatePort", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, ValidatePort("8080:80"))
		require.NoError(t, ValidatePort("127.0.0.1:8080:80"))
		require.NoError(t, ValidatePort("8080-8085:80-85"))
		require.Error(t, ValidatePort("invalid"))
		require.Error(t, ValidatePort("70000:80"))
		require.Error(t, ValidatePort("8080:0"))
	})

	t.Run("ValidateWorkdir", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, ValidateWorkdir("/app"))
		require.NoError(t, ValidateWorkdir("/node_modules/.pnpm/esbuild@0.25.12"))
		require.NoError(t, ValidateWorkdir("/workspace/@scope/pkg+dir"))
		require.Error(t, ValidateWorkdir("relative/path"))
		require.Error(t, ValidateWorkdir("/app\x00null"))
	})

	t.Run("ValidateNetworkName", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, ValidateNetworkName("bridge"))
		require.NoError(t, ValidateNetworkName("host"))
		require.NoError(t, ValidateNetworkName("custom-net_1"))
		require.Error(t, ValidateNetworkName("invalid\x00net"))
	})

	t.Run("ValidateUserName", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, ValidateUserName("root"))
		require.NoError(t, ValidateUserName("1000:1000"))
		require.NoError(t, ValidateUserName("node"))
		require.Error(t, ValidateUserName("user\x00name"))
	})

	t.Run("ValidateCapability", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, ValidateCapability("SYS_ADMIN"))
		require.NoError(t, ValidateCapability("CAP_NET_BIND_SERVICE"))
		require.NoError(t, ValidateCapability("ALL"))
		require.Error(t, ValidateCapability("CAP_"))
		require.Error(t, ValidateCapability("SYS__ADMIN"))
	})
}

func TestFeatureScenarios_AdvancedResilienceRefinementExpansion_ExpressionEngine(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		Files: make(map[string][]byte),
	}
	resolver, err := NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

	t.Run("Expression fallbacks and escaping", func(t *testing.T) {
		t.Parallel()

		// Unset env with fallback
		resolved, err := resolver.ResolveString("{{env:NON_EXISTENT_VAR_EXPANSION:-/default/path}}")
		require.NoError(t, err)
		assert.Equal(t, "/default/path", resolved)

		// Non-existent file with fallback
		resolved, err = resolver.ResolveString("{{file:nonexistent_file.txt:-fallback_content}}")
		require.NoError(t, err)
		assert.Equal(t, "fallback_content", resolved)

		// Literal string without expressions
		resolved, err = resolver.ResolveString("literal path /tmp/test")
		require.NoError(t, err)
		assert.Equal(t, "literal path /tmp/test", resolved)
	})
}

func TestFeatureScenarios_AdvancedResilienceRefinementExpansion_SensitiveMasking(t *testing.T) {
	t.Parallel()

	t.Run("MaskSensitiveEnv", func(t *testing.T) {
		t.Parallel()

		// Mask all when pattern is nil / empty
		masked := MaskSensitiveEnv("SECRET_KEY", "secret", nil)
		assert.Equal(t, "[REDACTED]", masked)

		// Custom pattern matching
		masked = MaskSensitiveEnv("PUBLIC_VAR", "hello", []string{"*KEY*"})
		assert.Equal(t, "hello", masked)

		// Already masked fast path
		masked = MaskSensitiveEnv("KEY", "[REDACTED]", nil)
		assert.Equal(t, "[REDACTED]", masked)
	})

	t.Run("MaskSensitiveEnvList", func(t *testing.T) {
		t.Parallel()

		env := []string{"API_TOKEN=xyz", "APP_ENV=prod", "DB_PASS=p@ss"}
		masked := MaskSensitiveEnvList(env, []string{"*TOKEN*", "*PASS*"})
		assert.Equal(t, []string{"API_TOKEN=[REDACTED]", "APP_ENV=prod", "DB_PASS=[REDACTED]"}, masked)
	})
}

func TestFeatureScenarios_AdvancedResilienceRefinementExpansion_PrecedenceMatrix(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		Files: map[string][]byte{},
		WD:    "/app",
	}

	cliOpts := &CLIOptions{
		Image:   optAdv(true, "alpine:cli"),
		Network: optAdv(false, ""),
	}

	cfg := &CDERunConfig{
		Defaults: ConfigDefaults{
			Network: "host",
			Workdir: "/yaml/workdir",
		},
	}

	res, err := ResolveWithFS("sh", cliOpts, nil, cfg, mfs)
	require.NoError(t, err)

	// P1 CLI override image
	assert.Equal(t, "alpine:cli", res.Image)
	// P5 YAML defaults for Network and Workdir
	assert.Equal(t, "host", res.Network)
	assert.Equal(t, "/yaml/workdir", res.Workdir)
}

func optAdv[T any](set bool, val T) *T {
	if !set {
		return nil
	}
	return &val
}
