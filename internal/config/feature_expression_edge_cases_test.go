package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Expression_AdvancedEdgeCasesAndResilience(t *testing.T) {
	t.Parallel()

	t.Run("nested fallbacks across find_dir, file, and env directives", func(t *testing.T) {
		mfs := &MockFileSystem{
			WD: "/workspace/project/sub",
			Dirs: map[string]bool{
				"/workspace":             true,
				"/workspace/project":     true,
				"/workspace/project/sub": true,
			},
			Files: map[string][]byte{
				"/workspace/project/fallback.txt": []byte("  /fallback/dir  "),
			},
		}

		resolver, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		// {{find_dir:nonexistent:-{{file:fallback.txt:-{{env:UNSET_ENV:-/default/path}}}}}}
		// find_dir:nonexistent fails -> evaluates file:fallback.txt which exists and returns "/fallback/dir"
		res, err := resolver.ResolveString("{{find_dir:nonexistent:-{{file:fallback.txt:-{{env:UNSET_ENV:-/default/path}}}}}}")
		require.NoError(t, err)
		assert.Equal(t, "/fallback/dir", res)
	})

	t.Run("deep fallback chain reaching env directive default", func(t *testing.T) {
		mfs := &MockFileSystem{
			WD: "/app",
			Dirs: map[string]bool{
				"/app": true,
			},
		}

		resolver, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		// find_dir fails -> file fails -> env fails -> resolves to "/fallback/default"
		res, err := resolver.ResolveString("{{find_dir:missing:-{{file:missing.txt:-{{env:MISSING_VAR:-/fallback/default}}}}}}")
		require.NoError(t, err)
		assert.Equal(t, "/fallback/default", res)
	})

	t.Run("path traversal in find_dir does not fallback and produces error", func(t *testing.T) {
		mfs := &MockFileSystem{
			WD: "/app",
		}

		resolver, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		// Path traversal (..) is an invalid directive argument error and should NOT fall back
		_, err = resolver.ResolveString("{{find_dir:../secret:-/fallback}}")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "find_dir directive")
	})

	t.Run("escaped braces combined with fallback directives", func(t *testing.T) {
		mfs := &MockFileSystem{
			WD: "/app",
		}

		resolver, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		// Escaped double braces \{\{...\}\} remain as escaped literal in string resolution
		res, err := resolver.ResolveString(`\{\{LITERAL\}\}-{{env:MISSING_ENV_VAR:-fallback_value}}`)
		require.NoError(t, err)
		assert.Equal(t, `\{\{LITERAL\}\}-fallback_value`, res)
	})

	t.Run("sticky error is clean when fallback succeeds", func(t *testing.T) {
		mfs := &MockFileSystem{
			WD: "/app",
		}

		resolver, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		// First resolve with a successful fallback
		val1, err1 := resolver.ResolveString("{{file:nonexistent.txt:-fallback_str}}")
		require.NoError(t, err1)
		assert.Equal(t, "fallback_str", val1)

		// Subsequent resolve should not be contaminated by sticky error
		val2, err2 := resolver.ResolveString("hello-{{PWD}}")
		require.NoError(t, err2)
		assert.Equal(t, "hello-/app", val2)
	})
}

func TestUnit_ConfigValidation_EdgeCaseInvariants(t *testing.T) {
	t.Parallel()

	t.Run("ValidateAddHost rejects empty host or IP", func(t *testing.T) {
		require.Error(t, ValidateAddHost(":127.0.0.1"))
		require.Error(t, ValidateAddHost("myhost:"))
		require.Error(t, ValidateAddHost("invalid_host"))
		require.NoError(t, ValidateAddHost("myhost:127.0.0.1"))
	})

	t.Run("ValidateImageName rejects malformed repository names", func(t *testing.T) {
		require.Error(t, ValidateImageName("alpine:"))
		require.Error(t, ValidateImageName("ubuntu/"))
		require.Error(t, ValidateImageName("ubuntu::latest"))
		require.NoError(t, ValidateImageName("docker.io/library/alpine:latest"))
	})

	t.Run("MaskSensitiveEnvList masks sensitive values when nil keywords (mask all) or matching keywords provided", func(t *testing.T) {
		envList := []string{
			"API_KEY=secret_key_123",
			"PATH=/usr/bin:/bin",
			"DB_PASSWORD=super_secret",
			"CLEAN_VAR=normal_value",
		}

		// When nil sensitiveKeywords is passed, all values are masked (Secure-by-default mask-all)
		maskedDefault := MaskSensitiveEnvList(envList, nil)
		assert.Equal(t, []string{
			"API_KEY=[REDACTED]",
			"PATH=[REDACTED]",
			"DB_PASSWORD=[REDACTED]",
			"CLEAN_VAR=[REDACTED]",
		}, maskedDefault)
	})
}
