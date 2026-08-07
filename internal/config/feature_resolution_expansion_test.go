package config

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// docs/features/value-resolution.md: Recursive Resolution Mechanics
// Verifies that recursive resolution correctly traverses slices of slices, maps of maps,
// and deeply nested combinations.
func TestUnit_Config_Resolver_DeepRecursiveResolutionAndEscaping(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD:      "/work",
		HomeDir: "/home/user",
		Env: map[string]string{
			"APP_ENV":  "prod",
			"APP_PORT": "8080",
		},
	}

	r, err := NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

	// A deeply nested complex structure with nested slices, maps, integers, and boolean values.
	input := map[string]any{
		"env":      "{{env:APP_ENV}}",
		"port":     "{{env:APP_PORT}}",
		"literal":  "{{ {{env:APP_ENV}} }}",
		"num":      123,
		"bool_val": true,
		"nested_slice": []any{
			"{{HOME}}/src",
			[]any{
				"{{PWD}}/bin",
				map[string]any{
					"subdir": "~/projects/{{env:APP_ENV}}",
				},
			},
		},
	}

	resolved := r.Resolve(input)
	require.NoError(t, r.Error())

	expected := map[string]any{
		"env":      "prod",
		"port":     "8080",
		"literal":  "{{env:APP_ENV}}",
		"num":      123,
		"bool_val": true,
		"nested_slice": []any{
			"/home/user/src",
			[]any{
				"/work/bin",
				map[string]any{
					"subdir": "/home/user/projects/prod",
				},
			},
		},
	}

	assert.Equal(t, expected, resolved)
}

// docs/features/value-resolution.md: Nested Expressions
// Evaluates deeply nested fallback combinations with multiple environmental variables and defaults.
func TestUnit_Config_Resolver_ComplexNestedFallbackExpressions(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD:      "/work",
		HomeDir: "/home/user",
		Env: map[string]string{
			"VAR_B": "val_b",
		},
	}

	r, err := NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

	// Scenario 1: VAR_A is missing, falls back to VAR_B which is present.
	val1, err := r.ResolveString("{{env:VAR_A:-{{env:VAR_B:-fallback_default}}}}")
	require.NoError(t, err)
	assert.Equal(t, "val_b", val1)

	// Scenario 2: Both VAR_A and VAR_B are missing, falls back to inner default.
	val2, err := r.ResolveString("{{env:VAR_A:-{{env:VAR_C:-fallback_default}}}}")
	require.NoError(t, err)
	assert.Equal(t, "fallback_default", val2)

	// Scenario 3: Mixed magic word and nested env fallback.
	val3, err := r.ResolveString("{{env:VAR_A:-{{HOME}}/config}}")
	require.NoError(t, err)
	assert.Equal(t, "/home/user/config", val3)
}

// docs/features/value-resolution.md: Anchor Boundary Validation
// Verifies path resolution across various anchor types and bounds.
func TestUnit_Config_Resolver_BoundaryValidation_EdgeCases(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD:      "/work/dir",
		HomeDir: "/home/user",
	}

	r, err := NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

	// Valid path containing sub-directory under HOME.
	path1, err := ResolvePath("{{HOME}}/subdir/file.txt", "/work/dir", r)
	require.NoError(t, err)
	assert.Equal(t, "/home/user/subdir/file.txt", path1)

	// Invalid path escaping the boundary of PWD.
	_, err = ResolvePath("{{PWD}}/../../etc/passwd", "/work/dir", r)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path traversal detected")
}

// docs/features/value-resolution.md: "Sticky Error" Pattern
// Once an error is encountered during resolution, the resolver transitions to a safe,
// non-evaluating fallback state where raw inputs are returned unmodified and the original error is retained.
func TestUnit_Config_Resolver_StickyErrorPattern(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD:      "/work",
		HomeDir: "/home/user",
	}

	r, err := NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

	// 1. Encounter an error first (e.g. unknown magic word style)
	_ = r.resolveString("{{UNKNOWN_MAGIC_WORD}}")
	require.Error(t, r.Error())
	firstErr := r.Error()
	assert.Contains(t, firstErr.Error(), "unknown directive or magic word")

	// 2. Subsequent resolutions should return unmodified inputs
	val1 := r.resolveString("{{HOME}}")
	assert.Equal(t, "{{HOME}}", val1)

	val2 := r.resolveString("plain-text")
	assert.Equal(t, "plain-text", val2)

	// 3. Resolve function should also operate in safe fallback mode
	sliceInput := []any{"{{HOME}}", "hello"}
	resolvedSlice := r.Resolve(sliceInput)
	assert.Equal(t, sliceInput, resolvedSlice)

	// 4. Retained error is still the same original error
	assert.Equal(t, firstErr, r.Error())
}

// docs/features/value-resolution.md: Empty Anchor
// Verify resolver error reporting when an empty anchor evaluates to empty string.
func TestUnit_Config_Resolver_EmptyAnchor_ResolutionError(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD:      "/work",
		HomeDir: "/home/user",
		Env: map[string]string{
			"EMPTY_VAR": "",
		},
	}

	r, err := NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

	_, err = ResolvePath("{{env:EMPTY_VAR}}/nested/file", "/work", r)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "anchor path is empty")
}

// docs/features/value-resolution.md: Directive Parameter Restrictions
// Test absolute paths, parent traversal, and invalid characters in find_dir and file directives.
func TestUnit_Config_Resolver_DirectiveRestrictions(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD:      "/work",
		HomeDir: "/home/user",
	}

	r, err := NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

	// Absolute path prohibited in directive
	_ = r.resolveString("{{file:/etc/passwd}}")
	require.Error(t, r.Error())
	assert.Contains(t, r.Error().Error(), "only a single file name is allowed in file directive")

	// Parent traversals prohibited in directive
	r2, err := NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)
	_ = r2.resolveString("{{file:../config}}")
	require.Error(t, r2.Error())
	assert.Contains(t, r2.Error().Error(), "only a single file name is allowed in file directive")
}

// docs/features/value-resolution.md: Anchor Boundary Validation - Under-the-Hood Security Logic
// Verify with MockFileSystem custom behaviors (e.g., directory matching error propagation).
func TestUnit_Config_Resolver_UnderTheHoodSecurityMockFS(t *testing.T) {
	t.Parallel()

	mfs := &julesExprMockFS2{
		MockFileSystem: MockFileSystem{
			WD:      "/work",
			HomeDir: "/home/user",
		},
	}

	r, err := NewExpressionResolverWithFS(&HostContext{Level: 1}, mfs)
	require.NoError(t, err)

	mfs.absErr = errors.New("simulated abs path error")
	_, err = ResolvePath("relative/path", "/work", r)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "simulated abs path error")
}

type julesExprMockFS2 struct {
	MockFileSystem
	absErr error
}

func (m *julesExprMockFS2) Abs(path string) (string, error) {
	if m.absErr != nil {
		return "", m.absErr
	}
	return m.MockFileSystem.Abs(path)
}
