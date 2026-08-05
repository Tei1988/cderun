package config

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// docs/features/value-resolution.md: Recursive Resolution Mechanics
// Value resolution is recursively applied down configuration trees.
func TestUnit_Config_Resolver_RecursiveResolve(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD:      "/work",
		HomeDir: "/home/user",
		Env: map[string]string{
			"VERSION": "1.2.3",
		},
	}

	r, err := NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

	// A slice containing mixed types, string expressions, and nested slices
	sliceInput := []any{
		"{{HOME}}/projects",
		42,
		"{{env:VERSION}}",
		[]any{"{{PWD}}/sub", true},
	}

	resolvedSlice := r.Resolve(sliceInput)
	require.NoError(t, r.Error())

	expectedSlice := []any{
		"/home/user/projects",
		42,
		"1.2.3",
		[]any{"/work/sub", true},
	}
	assert.Equal(t, expectedSlice, resolvedSlice)

	// A map containing mixed types, string expressions, and nested maps/slices
	mapInput := map[string]any{
		"home_path": "{{HOME}}",
		"id":        101,
		"nested": map[string]any{
			"pwd_path": "{{PWD}}",
			"tags":     []any{"tag-{{env:VERSION}}", false},
		},
	}

	resolvedMap := r.Resolve(mapInput)
	require.NoError(t, r.Error())

	expectedMap := map[string]any{
		"home_path": "/home/user",
		"id":        101,
		"nested": map[string]any{
			"pwd_path": "/work",
			"tags":     []any{"tag-1.2.3", false},
		},
	}
	assert.Equal(t, expectedMap, resolvedMap)
}

// docs/features/value-resolution.md: Unrecognized and Unknown Expressions
// Magic-word-like or directive-like unrecognized expressions must immediately trigger failure,
// while other expressions are preserved literally without modifications.
func TestUnit_Config_Resolver_UnrecognizedAndUnknown_StrictVsLiteral(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD: "/work",
	}

	// 1. Strict Resolution: ALL_UPPER is treated as magic-word candidate and fails.
	t.Run("ALL_UPPER unrecognized magic word style fails", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		_ = r.resolveString("{{NOT_EXIST_MAGIC_WORD}}")
		require.Error(t, r.Error())
		assert.Contains(t, r.Error().Error(), "unknown directive or magic word")
	})

	// 2. Strict Resolution: Contains colon is treated as directive candidate and fails.
	t.Run("contains colon unrecognized directive style fails", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		_ = r.resolveString("{{unknown_directive:param}}")
		require.Error(t, r.Error())
		assert.Contains(t, r.Error().Error(), "unknown directive or magic word")
	})

	// 3. Literal Preservation: non-upper/non-colon words are kept as-is and do NOT trigger error.
	t.Run("literal preservation of non-magic non-directive expressions", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		val1 := r.resolveString("{{camelCaseVar}}")
		require.NoError(t, r.Error())
		assert.Equal(t, "{{camelCaseVar}}", val1)

		val2 := r.resolveString("{{some template variable}}")
		require.NoError(t, r.Error())
		assert.Equal(t, "{{some template variable}}", val2)
	} )

	// 4. Double-Brace Escaping of Unknown Expressions:
	// Escaped unknown expressions must bypass strict resolution and be preserved literally.
	t.Run("escaping unknown expressions bypasses strict resolution and preserves literally", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		val := r.resolveString("{{ {{UNKNOWN_MAGIC_WORD}} }}")
		require.NoError(t, r.Error())
		assert.Equal(t, "{{UNKNOWN_MAGIC_WORD}}", val)

		val2 := r.resolveString("{{ {{bad:directive}} }}")
		require.NoError(t, r.Error())
		assert.Equal(t, "{{bad:directive}}", val2)
	})
}

// docs/features/value-resolution.md: Multiple Anchors Evaluation
// A path containing multiple anchors must satisfy boundary verification checks for every active anchor.
func TestUnit_Config_Resolver_MultipleAnchorsBoundaryValidation_Expansion(t *testing.T) {
	t.Parallel()

	home := "/home/user"
	pwd := "/work"

	mfs := &MockFileSystem{
		WD:      pwd,
		HomeDir: home,
	}

	t.Run("fails multiple boundaries due to mismatch", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		// Evaluates to "/home/user/work/subdir" which escapes the /work boundary of PWD.
		_, err = ResolvePath("{{HOME}}/{{PWD}}/subdir", pwd, r)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "path traversal detected")
	})

	t.Run("escapes single anchor boundary using parent traversal", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		// Evaluates to "/home/user/../etc" which resolves to "/home/etc" (escapes /home/user)
		_, err = ResolvePath("{{HOME}}/../etc", pwd, r)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "path traversal detected")
	})
}

// docs/features/value-resolution.md: Anchor Boundary Validation - Empty Anchor
// If an anchor resolves to empty string, it should return an error.
func TestUnit_Config_Resolver_EmptyAnchor_EdgeCases(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD:      "/work",
		HomeDir: "/home/user",
		Env: map[string]string{
			"EMPTY_VAR": "",
		},
	}

	t.Run("anchor resolves to empty string", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		_, err = ResolvePath("{{env:EMPTY_VAR}}/some_file", "/work", r)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "anchor path is empty")
	})

	t.Run("home directory is missing or empty on tilde expansion", func(t *testing.T) {
		mfsNoHome := &julesExprMockFS{
			homeDirErr: errors.New("home directory is empty"),
			MockFileSystem: MockFileSystem{
				WD: "/work",
			},
		}

		_, err := NewExpressionResolverWithFS(nil, mfsNoHome)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get user home directory")
	})
}

type julesExprMockFS struct {
	MockFileSystem
	homeDirErr error
}

func (m *julesExprMockFS) UserHomeDir() (string, error) {
	if m.homeDirErr != nil {
		return "", m.homeDirErr
	}
	return m.MockFileSystem.UserHomeDir()
}
