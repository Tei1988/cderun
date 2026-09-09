package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFeatureScenarios_BoundaryResilience_Validators(t *testing.T) {
	t.Parallel()

	t.Run("ValidatePort_BoundaryCases", func(t *testing.T) {
		t.Parallel()

		// Valid cases
		require.NoError(t, ValidatePort("8080:80"))
		require.NoError(t, ValidatePort("0:80")) // host port 0 is allowed for dynamic allocation
		require.NoError(t, ValidatePort("8080:80/tcp"))
		require.NoError(t, ValidatePort("127.0.0.1:8080:80/udp"))
		require.NoError(t, ValidatePort("8000-8005:9000-9005"))

		// Invalid cases
		require.Error(t, ValidatePort("invalid"))
		require.Error(t, ValidatePort("80:0")) // container port 0 is disallowed
		require.Error(t, ValidatePort("70000:80"))
	})

	t.Run("ValidateWorkdir_BoundaryCases", func(t *testing.T) {
		t.Parallel()

		// Valid cases including pnpm virtual store and scoped packages
		require.NoError(t, ValidateWorkdir("/app"))
		require.NoError(t, ValidateWorkdir("/app/node_modules/.pnpm/esbuild@0.25.12"))
		require.NoError(t, ValidateWorkdir("/app/@scope/pkg"))
		require.NoError(t, ValidateWorkdir("/app/path+with+plus"))

		// Invalid cases
		require.Error(t, ValidateWorkdir("relative/path"))
		require.Error(t, ValidateWorkdir("/app/path\x00invalid"))
		require.Error(t, ValidateWorkdir("/app/path\x1fcontrol"))
	})

	t.Run("ValidateDNSOption_BoundaryCases", func(t *testing.T) {
		t.Parallel()

		require.NoError(t, ValidateDNSOption("ndots:5"))
		require.NoError(t, ValidateDNSOption("timeout:2"))
		require.Error(t, ValidateDNSOption("ndots:\x00invalid"))
	})

	t.Run("ValidateSecurityOpt_BoundaryCases", func(t *testing.T) {
		t.Parallel()

		require.NoError(t, ValidateSecurityOpt("no-new-privileges:true"))
		require.NoError(t, ValidateSecurityOpt("seccomp=unconfined"))
		require.Error(t, ValidateSecurityOpt("seccomp=\x00bad"))
	})

	t.Run("ValidateSysctlKeyAndValue", func(t *testing.T) {
		t.Parallel()

		require.NoError(t, ValidateSysctlKey("net.ipv4.ip_forward"))
		require.Error(t, ValidateSysctlKey("net.ipv4.ip_forward;rm -rf"))

		require.NoError(t, ValidateSysctlValue("1"))
		require.Error(t, ValidateSysctlValue("1\x000"))
	})

	t.Run("ValidateGPUsAndCpuset", func(t *testing.T) {
		t.Parallel()

		require.NoError(t, ValidateGPUs("all"))
		require.NoError(t, ValidateGPUs("device=0,1"))
		require.Error(t, ValidateGPUs("invalid_gpu_spec!"))

		require.NoError(t, ValidateCpuset("0-3,5"))
		require.Error(t, ValidateCpuset("0-abc"))
	})

	t.Run("ValidateAddHostAndImageName", func(t *testing.T) {
		t.Parallel()

		require.NoError(t, ValidateAddHost("host.docker.internal:127.0.0.1"))
		require.Error(t, ValidateAddHost(":127.0.0.1"))

		require.NoError(t, ValidateImageName("alpine:latest"))
		require.NoError(t, ValidateImageName("docker.io/library/alpine:3.18"))
		require.Error(t, ValidateImageName("alpine::latest"))
		require.Error(t, ValidateImageName("/alpine"))
	})
}

func TestFeatureScenarios_BoundaryResilience_ExpressionResolution(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		Files: map[string][]byte{
			"/app/config.txt": []byte("app_secret_value\n"),
			"/app/empty.txt":  []byte(""),
		},
		WD: "/app",
		Env: map[string]string{
			"APP_ENV": "production",
		},
	}

	hostCtx := &HostContext{Level: 0, HomeDir: "/home/testuser", WorkingDir: "/app"}
	resolver, err := NewExpressionResolverWithFS(hostCtx, mfs)
	require.NoError(t, err)

	t.Run("DirectivesWithDefaults", func(t *testing.T) {
		t.Parallel()

		// Existing env
		res := resolver.Resolve("{{env:APP_ENV:-development}}")
		assert.Equal(t, "production", res)

		// Missing env with default
		res = resolver.Resolve("{{env:MISSING_ENV:-staging}}")
		assert.Equal(t, "staging", res)

		// Existing file
		res = resolver.Resolve("{{file:config.txt:-fallback}}")
		assert.Equal(t, "app_secret_value", res)

		// Missing file with fallback
		res = resolver.Resolve("{{file:nonexistent.txt:-fallback_content}}")
		assert.Equal(t, "fallback_content", res)

		// Empty file with fallback
		res = resolver.Resolve("{{file:empty.txt:-default_content}}")
		assert.Equal(t, "default_content", res)
	})

	t.Run("PrecedenceResolutionMatrix", func(t *testing.T) {
		t.Parallel()

		// Test precedence between CLI layer (P1) and Tools/Global layer (P3/P4)
		opts := &CLIOptions{
			Image: ptrVal("golang:1.22-alpine"), // P1 winner
		}
		toolsCfg := ToolsConfig{
			"golang": ToolConfig{
				Image: "golang:1.21-alpine", // P3 lower priority
			},
		}

		res, err := ResolveWithFS("golang", opts, toolsCfg, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, "golang:1.22-alpine", res.Image)

		// Test tool configuration (P3) wins when CLI (P1) is unset
		emptyOpts := &CLIOptions{}
		resTool, err := ResolveWithFS("golang", emptyOpts, toolsCfg, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, "golang:1.21-alpine", resTool.Image)
	})
}

func TestFeatureScenarios_BoundaryResilience_MaskingInvariants(t *testing.T) {
	t.Parallel()

	t.Run("MaskSensitiveEnv_DefaultMaskAll", func(t *testing.T) {
		t.Parallel()

		env := []string{
			"PATH=/usr/bin:/bin",
			"SECRET_KEY=supersecret123",
			"TOKEN=xyz987",
		}

		// When pattern is nil, default masks all values
		maskedNil := MaskSensitiveEnvList(env, nil)
		require.Len(t, maskedNil, 3)
		assert.Equal(t, "PATH=[REDACTED]", maskedNil[0])
		assert.Equal(t, "SECRET_KEY=[REDACTED]", maskedNil[1])
		assert.Equal(t, "TOKEN=[REDACTED]", maskedNil[2])

		// Calling with empty slice []string{} disables masking (no patterns match)
		maskedEmpty := MaskSensitiveEnvList(env, []string{})
		require.Len(t, maskedEmpty, 3)
		assert.Equal(t, "PATH=/usr/bin:/bin", maskedEmpty[0])
		assert.Equal(t, "SECRET_KEY=supersecret123", maskedEmpty[1])
		assert.Equal(t, "TOKEN=xyz987", maskedEmpty[2])
	})

	t.Run("MaskSensitiveEnv_ExplicitPatterns", func(t *testing.T) {
		t.Parallel()

		env := []string{
			"LOG_LEVEL=debug",
			"API_TOKEN=secret_token",
			"DATABASE_PASSWORD=secret_pass",
		}

		patterns := []string{"*TOKEN*", "*PASSWORD*"}
		masked := MaskSensitiveEnvList(env, patterns)
		require.Len(t, masked, 3)
		assert.Equal(t, "LOG_LEVEL=debug", masked[0])
		assert.Equal(t, "API_TOKEN=[REDACTED]", masked[1])
		assert.Equal(t, "DATABASE_PASSWORD=[REDACTED]", masked[2])
	})
}

func ptrVal[T any](v T) *T {
	return &v
}
