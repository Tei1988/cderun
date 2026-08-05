package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// docs/features/value-resolution.md: "Sticky Error" Pattern
// To prevent invalid or compromised configurations from propagating,
// the first resolution or security validation error encountered is captured and stored internally.
// Subsequent evaluation steps transition to a safe, non-evaluating fallback state where raw input strings are returned unmodified.
func TestUnit_Config_Resolver_StickyErrorSafeFallback(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD:      "/work",
		HomeDir: "/home/user",
		Env: map[string]string{
			"VALID_VAR": "val1",
		},
	}

	t.Run("first error is preserved and subsequent slice elements bypass resolution", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		sliceInput := []any{
			"{{INVALID_DIRECTIVE:param}}", // triggers strict resolution magic/directive error
			"{{VALID_VAR}}",               // would be resolved if not in sticky error state
		}

		resolved := r.Resolve(sliceInput)
		require.Error(t, r.Error())
		assert.Contains(t, r.Error().Error(), "unknown directive or magic word")

		// The resolved output slice should preserve the unresolved second element literally
		// because the resolver transitioned into a safe fallback state.
		expectedSlice := []any{
			"{{INVALID_DIRECTIVE:param}}",
			"{{VALID_VAR}}",
		}
		assert.Equal(t, expectedSlice, resolved)
	})

	t.Run("first error is preserved and subsequent map values bypass resolution", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		mapInput := map[string]any{
			"a_err": "{{env:VAR_WITH\x00NULL}}", // fails env key validation due to null byte
			"b_ok":  "{{env:VALID_VAR}}",
		}

		resolved := r.Resolve(mapInput)
		require.Error(t, r.Error())
		assert.Contains(t, r.Error().Error(), "security validation failed")

		expectedMap := map[string]any{
			"a_err": "{{env:VAR_WITH\x00NULL}}",
			"b_ok":  "{{env:VALID_VAR}}",
		}
		assert.Equal(t, expectedMap, resolved)
	})
}

// docs/features/value-resolution.md: Nested Expressions
// Expressions can be nested to configure complex fallback scenarios. The expression engine evaluates expressions from the inside out.
func TestUnit_Config_Resolver_DeeplyNestedExpressions(t *testing.T) {
	t.Parallel()

	t.Run("resolves deeply nested env fallbacks when all are unset", func(t *testing.T) {
		mfs := &MockFileSystem{
			WD:      "/work",
			HomeDir: "/home/user",
			Env:     map[string]string{},
		}
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		val, err := r.ResolveString("{{env:ENV_A:-{{env:ENV_B:-{{env:ENV_C:-fallback_val}}}}}}")
		require.NoError(t, err)
		assert.Equal(t, "fallback_val", val)
	})

	t.Run("resolves deeply nested env fallbacks with intermediate value set", func(t *testing.T) {
		mfs := &MockFileSystem{
			WD:      "/work",
			HomeDir: "/home/user",
			Env: map[string]string{
				"ENV_B": "value_b",
			},
		}
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		val, err := r.ResolveString("{{env:ENV_A:-{{env:ENV_B:-{{env:ENV_C:-fallback_val}}}}}}")
		require.NoError(t, err)
		assert.Equal(t, "value_b", val)
	})

	t.Run("resolves deeply nested env fallbacks with file directive nested inside fallback", func(t *testing.T) {
		mfs := &MockFileSystem{
			WD:      "/work",
			HomeDir: "/home/user",
			Env:     map[string]string{},
			Files: map[string][]byte{
				"/work/.version": []byte("2.0.0"),
			},
		}
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		val, err := r.ResolveString("{{env:VERSION:-{{file:.version}}}}")
		require.NoError(t, err)
		assert.Equal(t, "2.0.0", val)
	})
}

// docs/features/value-resolution.md: Security Hardening and Constraints - Null-Byte Injections Guard
// The engine scans environmental keys and values for null bytes (\x00). The presence of any null byte triggers an immediate security validation error.
func TestUnit_Config_Resolver_NullByteSafety(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD:      "/work",
		HomeDir: "/home/user",
	}

	t.Run("rejects env key containing null byte", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		_, err = r.ResolveString("{{env:BAD\x00KEY}}")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "security validation failed")
	})

	t.Run("rejects env value containing null byte", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		// Set up an env list to resolve via resolveEnvValues helper
		envInput := []string{
			"VALID=value",
			"BAD_VAL=something\x00else",
		}

		resolved, err := resolveEnvValues(envInput, nil, false, r, mfs)
		require.Error(t, err)
		assert.Nil(t, resolved)
		assert.Contains(t, err.Error(), "null byte injection detected")
	})
}

// docs/features/value-resolution.md: Multiple Anchors Evaluation & Anchor Boundary Validation
// A path containing a mixture of anchors (HOME, PWD, tilde) must satisfy boundary verification checks for every active anchor.
func TestUnit_Config_Resolver_MixedAnchorsBoundaryValidation(t *testing.T) {
	t.Parallel()

	home := "/home/user"
	pwd := "/work"

	mfs := &MockFileSystem{
		WD:      pwd,
		HomeDir: home,
	}

	t.Run("multiple mixed anchors within boundaries passes", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		// ResolvePath is imported or implemented in package.
		// Since mixed paths are resolved using the ResolvePath logic:
		resolved, err := ResolvePath("{{HOME}}/projects/../projects/file.txt", pwd, r)
		require.NoError(t, err)
		assert.Equal(t, "/home/user/projects/file.txt", resolved)
	})

	t.Run("mixed anchors boundary failure due to traversal escaping home", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		// Traverses out of {{HOME}} boundary
		_, err = ResolvePath("{{HOME}}/../../etc/passwd", pwd, r)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "path traversal detected")
		assert.Contains(t, err.Error(), "/home/user")
	})

	t.Run("tilde mixed with magic word boundary failure", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		// Tilde expansion resolves to home directory. Relative traversal traverses out of home boundary.
		_, err = ResolvePath("~/../outside/file.txt", pwd, r)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "path traversal detected")
	})
}
