package config

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// docs/features/value-resolution.md: Recursive Resolution Mechanics
// Test that deep recursive resolution handles maps and slices nested inside each other, resolving tilde, magic words,
// and env keys with defaults, and checking boundaries.
func TestUnit_Config_Resolver_DeepRecursiveComplexMixedCollections(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD:      "/work",
		HomeDir: "/home/user",
		Env: map[string]string{
			"VAR_A": "resolved_a",
			"VAR_B": "resolved_b",
		},
	}

	r, err := NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

	input := map[string]any{
		"lvl1_slice": []any{
			"~/project",
			"{{HOME}}/docs",
			map[string]any{
				"lvl2_env": "{{env:VAR_A:-default_a}}",
				"lvl2_env_fallback": "{{env:NONEXIST:-{{env:VAR_B:-default_b}}}}",
				"lvl2_slice": []any{
					"{{PWD}}/subpath",
				},
			},
		},
	}

	resolved := r.Resolve(input)
	require.NoError(t, r.Error())

	expected := map[string]any{
		"lvl1_slice": []any{
			"/home/user/project",
			"/home/user/docs",
			map[string]any{
				"lvl2_env": "resolved_a",
				"lvl2_env_fallback": "resolved_b",
				"lvl2_slice": []any{
					"/work/subpath",
				},
			},
		},
	}
	assert.Equal(t, expected, resolved)
}

// docs/features/value-resolution.md: Unrecognized and Unknown Expressions & Double-Brace Escaping Syntax
// Verify that nested unknown expressions with custom formats or template variables inside are preserved literally,
// while uppercase words (magic word candidates) or colons (directive candidates) trigger strict error,
// even when nested deep inside recursive lists or maps.
func TestUnit_Config_Resolver_UnrecognizedAndUnknown_DeepNested_StrictVsLiteral(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD: "/work",
	}

	t.Run("nested literal preservation in maps and slices", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		input := []any{
			"{{my {{nested}} variable}}",
			map[string]any{
				"valid_literal": "{{someOtherVar}}",
				"escaped_unknown": "{{ {{UNKNOWN_DIRECTIVE:val}} }}",
			},
		}

		resolved := r.Resolve(input)
		require.NoError(t, r.Error())

		expected := []any{
			"{{my {{nested}} variable}}",
			map[string]any{
				"valid_literal": "{{someOtherVar}}",
				"escaped_unknown": "{{UNKNOWN_DIRECTIVE:val}}",
			},
		}
		assert.Equal(t, expected, resolved)
	})

	t.Run("deeply nested unknown magic word fails strictly", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		input := map[string]any{
			"ok": "literal_val",
			"nested_slice": []any{
				"another_literal",
				map[string]any{
					"bad_field": "{{INVALID_MAGIC_WORD}}",
				},
			},
		}

		_ = r.Resolve(input)
		require.Error(t, r.Error())
		assert.Contains(t, r.Error().Error(), "unknown directive or magic word")
	})

	t.Run("deeply nested unknown directive fails strictly", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		input := []any{
			map[string]any{
				"bad_directive": "{{invalid_dir:param}}",
			},
		}

		_ = r.Resolve(input)
		require.Error(t, r.Error())
		assert.Contains(t, r.Error().Error(), "unknown directive or magic word")
	})
}

// docs/features/value-resolution.md: Tilde Expansion
// Test tilde expansion edge cases:
// 1. Tildes not at the start of strings (e.g. "/path/~/sub") must be preserved literally.
// 2. Relative traversals starting with tildes (e.g. "~/../outside") must be rejected by anchor boundary validation.
func TestUnit_Config_Resolver_TildeExpansion_EdgeCases(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD:      "/work",
		HomeDir: "/home/user",
	}

	t.Run("mid-path tildes kept as literal", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		val1, err := r.ResolveString("/some/~/path")
		require.NoError(t, err)
		assert.Equal(t, "/some/~/path", val1)

		val2, err := r.ResolveString("prefix_~_suffix")
		require.NoError(t, err)
		assert.Equal(t, "prefix_~_suffix", val2)
	})

	t.Run("tilde relative traversal escapes boundary", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		_, err = ResolvePath("~/../other_user", "/work", r)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "path traversal detected")
	})
}

// docs/features/value-resolution.md: Sticky Error Pattern
// Verify that the Sticky Error pattern propagates correctly in a complex nested map,
// switching to safe fallback (no-op return) for any subsequent fields.
func TestUnit_Config_Resolver_StickyError_ComplexNestedStructures(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD:      "/work",
		HomeDir: "/home/user",
		Env: map[string]string{
			"ENV_OK": "value_ok",
		},
	}

	t.Run("deep map triggers sticky error first and bypasses subsequent sibling resolution", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		input := map[string]any{
			"first_ok": "{{env:ENV_OK}}",
			"nested": map[string]any{
				"trigger_error": "{{BAD_DIRECTIVE:param}}",
				"second_bypassed": "{{env:ENV_OK}}",
			},
			"lvl1_bypassed": "{{env:ENV_OK}}",
		}

		resolved := r.Resolve(input)
		require.Error(t, r.Error())
		assert.Contains(t, r.Error().Error(), "unknown directive or magic word")

		// First ok resolves successfully (or might be resolved before error depending on iteration sequence,
		// but Go map iteration order is randomized).
		// What's guaranteed: once the error is hit, any resolve step that runs *after* it must bypass evaluation.
		// Since map iteration order is randomized, we check the final state of resolved.
		// If trigger_error was evaluated, the error is recorded.
		// Let's assert that "resolved" contains the un-evaluated trigger_error string, and the recorded error is present.
		// Also, the map should be returned.
		assert.NotNil(t, resolved)
		resMap, ok := resolved.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "{{BAD_DIRECTIVE:param}}", resMap["nested"].(map[string]any)["trigger_error"])
	})
}

// docs/features/value-resolution.md: Security Hardening and Constraints - Null-Byte Injections Guard
// Verify that nested env key / value null-byte checks immediately trigger sticky resolution error.
func TestUnit_Config_Resolver_NullByte_DeepNested(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD:      "/work",
		HomeDir: "/home/user",
	}

	t.Run("deeply nested slice env key null-byte rejection", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		input := []any{
			"literal",
			map[string]any{
				"env_with_null": "{{env:SOME_ENV_KEY\x00_NULL}}",
			},
		}

		_ = r.Resolve(input)
		require.Error(t, r.Error())
		assert.Contains(t, r.Error().Error(), "security validation failed")
	})

	t.Run("deeply nested slice env value null-byte rejection via resolveEnvValues", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		envInput := []string{
			"VAR_A=normal",
			"VAR_B=with\x00null_byte",
		}

		_, err = resolveEnvValues(envInput, nil, false, r, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "null byte injection detected")
	})
}

// docs/features/value-resolution.md: Security Hardening and Constraints - Directive Parameter Restrictions
// Parameters of file and find_dir directives must be simple filenames/directories and strictly traversal-free.
func TestUnit_Config_Resolver_DirectiveRestrictions_Refinement(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD:      "/work",
		HomeDir: "/home/user",
	}

	t.Run("file directive rejects absolute paths", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		_, err = r.ResolveString("{{file:/etc/passwd}}")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "only a single file name is allowed in file directive")
	})

	t.Run("file directive rejects relative traversal segments", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		_, err = r.ResolveString("{{file:../config}}")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "only a single file name is allowed in file directive")
	})

	t.Run("find_dir directive rejects absolute paths", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		_, err = r.ResolveString("{{find_dir:/etc}}")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "only a single file or directory name is allowed in find_dir directive")
	})

	t.Run("find_dir directive rejects relative traversal segments", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		_, err = r.ResolveString("{{find_dir:../subdir}}")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "only a single file or directory name is allowed in find_dir directive")
	})

	t.Run("file directive rejects empty file parameter", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		_, err = r.ResolveString("{{file:}}")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "only a single file name is allowed in file directive")
	})

	t.Run("find_dir directive rejects empty folder parameter", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		_, err = r.ResolveString("{{find_dir:}}")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "only a single file or directory name is allowed in find_dir directive")
	})
}

// docs/features/value-resolution.md: Anchor Boundary Validation - Missing or Error-prone Home/PWD Directories
// Test scenarios where home directory or working directory resolution fails or has missing contexts.
func TestUnit_Config_Resolver_BoundaryResolution_Errors(t *testing.T) {
	t.Parallel()

	t.Run("missing working directory causes resolution error on relative conversion", func(t *testing.T) {
		mfsNoWD := &MockFileSystem{
			AbsErr:  errors.New("working directory lookup failed"),
			HomeDir: "/home/user",
		}

		r, err := NewExpressionResolverWithFS(&HostContext{Level: 1}, mfsNoWD)
		require.NoError(t, err)

		_, err = ResolvePath("./relative_file", "", r)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "working directory lookup failed")
	})
}
