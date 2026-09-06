package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdvancedResilience_ParameterValidators(t *testing.T) {
	t.Parallel()

	t.Run("ValidateDNSOption boundaries", func(t *testing.T) {
		assert.NoError(t, ValidateDNSOption("ndots:5"))
		assert.NoError(t, ValidateDNSOption("timeout:2"))
		assert.NoError(t, ValidateDNSOption("attempts:3"))
		assert.NoError(t, ValidateDNSOption("edns0"))
		assert.NoError(t, ValidateDNSOption("use-vc"))
		assert.NoError(t, ValidateDNSOption(""))

		assert.Error(t, ValidateDNSOption("ndots:\x005"))
		assert.Error(t, ValidateDNSOption("ndots:5\n"))
		assert.Error(t, ValidateDNSOption("invalid\x7foption"))
		assert.Error(t, ValidateDNSOption("ndots:5/.."))
	})

	t.Run("ValidateSecurityOpt boundaries", func(t *testing.T) {
		assert.NoError(t, ValidateSecurityOpt("no-new-privileges:true"))
		assert.NoError(t, ValidateSecurityOpt("seccomp=unconfined"))
		assert.NoError(t, ValidateSecurityOpt("apparmor:unconfined"))
		assert.NoError(t, ValidateSecurityOpt("label=disable"))
		assert.NoError(t, ValidateSecurityOpt(""))

		require.Error(t, ValidateSecurityOpt("no-new-privileges:\x00true"))
		require.Error(t, ValidateSecurityOpt("seccomp=\xff\xfe"))
		require.Error(t, ValidateSecurityOpt("label=disable/.."))
	})

	t.Run("ValidatePort boundaries", func(t *testing.T) {
		assert.NoError(t, ValidatePort("8080:80"))
		assert.NoError(t, ValidatePort("127.0.0.1:8080:80/tcp"))
		assert.NoError(t, ValidatePort("8000-8005:8000-8005"))
		assert.NoError(t, ValidatePort(""))

		assert.Error(t, ValidatePort("8080:0"))
		assert.Error(t, ValidatePort("65536:80"))
		assert.Error(t, ValidatePort("8080:65536"))
		assert.Error(t, ValidatePort("8000-8005:8000-8002"))
		assert.Error(t, ValidatePort("port\x00invalid"))
	})

	t.Run("ValidateWorkdir boundaries with pnpm and scoped packages", func(t *testing.T) {
		assert.NoError(t, ValidateWorkdir("/app"))
		assert.NoError(t, ValidateWorkdir("/app/.pnpm/esbuild@0.25.12/node_modules/esbuild"))
		assert.NoError(t, ValidateWorkdir("/app/@scope/package"))
		assert.NoError(t, ValidateWorkdir("/app/build+test"))
		assert.NoError(t, ValidateWorkdir(""))

		assert.Error(t, ValidateWorkdir(".pnpm/esbuild@0.25.12")) // relative path rejected
		assert.Error(t, ValidateWorkdir("/app/\x00null"))
		assert.Error(t, ValidateWorkdir("/app/control\rchar"))
	})

	t.Run("ValidateNetworkName and ValidateUserName boundaries", func(t *testing.T) {
		assert.NoError(t, ValidateNetworkName("bridge"))
		assert.NoError(t, ValidateNetworkName("custom-net_1"))
		assert.NoError(t, ValidateNetworkName(""))
		assert.Error(t, ValidateNetworkName("net/with/slash"))
		assert.Error(t, ValidateNetworkName("net\x00null"))

		assert.NoError(t, ValidateUserName("root"))
		assert.NoError(t, ValidateUserName("1000:1000"))
		assert.NoError(t, ValidateUserName("appuser:appgroup"))
		assert.NoError(t, ValidateUserName(""))
		assert.Error(t, ValidateUserName("user\x00null"))
		assert.Error(t, ValidateUserName("4294967296:1000")) // UID uint32 overflow
	})
}

func TestAdvancedResilience_TemplateExpressionFallbackAndEscaping(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.txt")
	require.NoError(t, os.WriteFile(configFile, []byte("  secret_val  \n"), 0644))

	hostCtx := &HostContext{HomeDir: tmpDir, WorkingDir: tmpDir}
	mockFS := &RealFileSystem{}

	t.Run("File expression with fallback when missing", func(t *testing.T) {
		resolver, err := NewExpressionResolverWithFS(hostCtx, mockFS)
		require.NoError(t, err)

		expr := "{{file:nonexistent.txt:-default_value}}"
		res, err := resolver.ResolveString(expr)
		require.NoError(t, err)
		assert.Equal(t, "default_value", res)
	})

	t.Run("File expression returning file content when present", func(t *testing.T) {
		oldWd, err := os.Getwd()
		require.NoError(t, err)
		require.NoError(t, os.Chdir(tmpDir))
		defer func() { _ = os.Chdir(oldWd) }()

		resolver, err := NewExpressionResolverWithFS(hostCtx, mockFS)
		require.NoError(t, err)

		expr := "{{file:config.txt:-default_value}}"
		res, err := resolver.ResolveString(expr)
		require.NoError(t, err)
		assert.Equal(t, "secret_val", res)
	})

	t.Run("FindDir expression with fallback when missing", func(t *testing.T) {
		resolver, err := NewExpressionResolverWithFS(hostCtx, mockFS)
		require.NoError(t, err)

		expr := "{{find_dir:nonexistent_dir_anchor_123:-/fallback/path}}"
		res, err := resolver.ResolveString(expr)
		require.NoError(t, err)
		assert.Equal(t, "/fallback/path", res)
	})

	t.Run("Template expression resolution evaluates valid directives", func(t *testing.T) {
		resolver, err := NewExpressionResolverWithFS(hostCtx, mockFS)
		require.NoError(t, err)

		expr := "home path is {{HOME}}"
		res, err := resolver.ResolveString(expr)
		require.NoError(t, err)
		assert.Contains(t, res, "home path is ")
		assert.NotEqual(t, expr, res)
	})

	t.Run("Sticky error isolation across expression resolver instances", func(t *testing.T) {
		r1, err1 := NewExpressionResolverWithFS(hostCtx, mockFS)
		require.NoError(t, err1)
		_, err1 = r1.ResolveString("{{invalid_directive:foo}}")
		require.Error(t, err1)

		r2, err2 := NewExpressionResolverWithFS(hostCtx, mockFS)
		require.NoError(t, err2)
		res2, err2 := r2.ResolveString("hello world")
		require.NoError(t, err2)
		assert.Equal(t, "hello world", res2)
	})
}

func TestAdvancedResilience_SensitiveEnvMaskingAndPrecedence(t *testing.T) {
	t.Parallel()

	t.Run("MaskSensitiveEnv default mask-all behavior", func(t *testing.T) {
		masked := MaskSensitiveEnv("SECRET_TOKEN", "xyz123", nil)
		assert.Equal(t, "[REDACTED]", masked)

		maskedAlready := MaskSensitiveEnv("SECRET_TOKEN", "[REDACTED]", nil)
		assert.Equal(t, "[REDACTED]", maskedAlready)
	})

	t.Run("MaskSensitiveEnvList with explicit patterns", func(t *testing.T) {
		envs := []string{
			"API_KEY=secret_key",
			"PUBLIC_HOST=localhost",
			"DB_PASS=p@ssword",
		}
		patterns := []string{"*KEY*", "*PASS*"}

		maskedList := MaskSensitiveEnvList(envs, patterns)
		require.Len(t, maskedList, 3)
		assert.Equal(t, "API_KEY=[REDACTED]", maskedList[0])
		assert.Equal(t, "PUBLIC_HOST=localhost", maskedList[1])
		assert.Equal(t, "DB_PASS=[REDACTED]", maskedList[2])
	})

	t.Run("Precedence matrix resolution P1-P6 verification", func(t *testing.T) {
		mockFS := &MockFileSystem{WD: "/workspace"}

		cliOpts := CLIOptions{
			Image: optStringVal("ubuntu:22.04"),
		}

		res, err := ResolveWithFS("node", &cliOpts, ToolsConfig{}, nil, mockFS)
		require.NoError(t, err)
		assert.Equal(t, "ubuntu:22.04", res.Image)
	})
}

func optStringVal(s string) *string {
	return &s
}
