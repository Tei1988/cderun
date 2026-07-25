package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUnit_Config_Resolver_MultipleAnchorsConcurrency implements the "複数アンカーの取り扱い (Handling Multiple Anchors)"
// specification from docs/features/value-resolution.md.
// If multiple curly-brace expressions (anchors) appear in a single path, they all act as independent boundaries.
// The resolved path must simultaneously satisfy the boundary conditions of all anchor directories.
func TestUnit_Config_Resolver_MultipleAnchorsConcurrency(t *testing.T) {
	t.Parallel()

	home := "/home/user"
	pwd := "/work"

	// final resolved path is /home/user/work/file.
	// Anchor 1 is HOME (/home/user). Relative to /home/user is work/file, which is inside.
	// Anchor 2 is PWD (/work). Relative to /work is /home/user/work/file, which escapes /work boundary.
	// This must fail boundary validation with a path traversal detection error.
	mfs := &MockFileSystem{
		WD:      pwd,
		HomeDir: home,
	}

	r, err := NewExpressionResolverWithFS(&HostContext{}, mfs)
	require.NoError(t, err)

	_, err = ResolvePath("{{HOME}}/{{PWD}}/file", pwd, r)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path traversal detected")
}

// TestUnit_Config_Resolver_NestedExpressionErrors validates the "ネストされた式 (Nested Expressions)" and
// "セキュリティ制約 (Security Restraints)" specifications under deep/triple nested expressions with errors
// in sub-evaluations, assuring proper error propagation and safety boundaries.
func TestUnit_Config_Resolver_NestedExpressionErrors(t *testing.T) {
	t.Parallel()

	t.Run("inner evaluation has security violation", func(t *testing.T) {
		mfs := &MockFileSystem{
			WD: "/work",
		}
		r, err := NewExpressionResolverWithFS(&HostContext{}, mfs)
		require.NoError(t, err)

		// Triple nested: inner file directive has path traversal.
		// {{env:VERSION:-{{file:{{file:../passwd}}}}}}
		// This must immediately trigger the single-file restriction.
		_ = r.resolveString("{{env:VERSION:-{{file:{{file:../passwd}}}}}}")
		require.Error(t, r.Error())
		assert.Contains(t, r.Error().Error(), "only a single file name is allowed")
	})

	t.Run("inner evaluation not found raises error", func(t *testing.T) {
		mfs := &MockFileSystem{
			WD: "/work",
		}
		r, err := NewExpressionResolverWithFS(&HostContext{}, mfs)
		require.NoError(t, err)

		// Inner file "missing" does not exist.
		_ = r.resolveString("{{env:VERSION:-{{file:missing}}}}")
		require.Error(t, r.Error())
		assert.Contains(t, r.Error().Error(), "file not found")
	})
}

// TestUnit_Config_Resolver_StickyErrorPropagation verifies the "スティッキーエラー (Sticky Error) パターン"
// specification. Once the ExpressionResolver encounters an error, it transitions into a sticky error state.
// All subsequent calls to resolveString must immediately return the unresolved input, preserving the original error.
func TestUnit_Config_Resolver_StickyErrorPropagation(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD: "/work",
	}
	r, err := NewExpressionResolverWithFS(&HostContext{}, mfs)
	require.NoError(t, err)

	// Resolve missing file first to transition to sticky error state
	r.resolveString("{{file:nonexistent}}")
	firstErr := r.Error()
	require.Error(t, firstErr)
	assert.Contains(t, firstErr.Error(), "file not found")

	// Call resolveString again with valid expression.
	// It must NOT resolve, and must return the expression untouched.
	val := r.resolveString("{{PWD}}")
	assert.Equal(t, "{{PWD}}", val)

	// The error returned must still be the original first error
	assert.Same(t, firstErr, r.Error())
}

// TestUnit_Config_Resolver_EscapingComplex verifies the "エスケープ記法 (Escaping)" specification.
// Validates that outer triple braces and complex nested double-brace escaping preserves correct literal results.
func TestUnit_Config_Resolver_EscapingComplex(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD: "/work",
		Env: map[string]string{
			"VAR": "value",
		},
	}
	r, err := NewExpressionResolverWithFS(&HostContext{}, mfs)
	require.NoError(t, err)

	// {{ {{env:VAR}} }} resolves to literal {{env:VAR}}
	val1 := r.resolveString("{{ {{env:VAR}} }}")
	require.NoError(t, r.Error())
	assert.Equal(t, "{{env:VAR}}", val1)

	// Nested escaped expressions are preserved literally within their outermost double braces.
	val2 := r.resolveString("Literal {{ {{env:VAR:-{{ {{PWD}} }} }} }} text")
	require.NoError(t, r.Error())
	assert.Equal(t, "Literal {{env:VAR:-{{ {{PWD}} }} }} text", val2)
}

// TestUnit_Config_Resolver_FileLimitExactly verifies "ファイルサイズの制限" of exactly 1MB
// (MaxDirectiveFileSize) vs 1MB + 1 byte, assuring precise bounds and dynamic file checks.
func TestUnit_Config_Resolver_FileLimitExactly(t *testing.T) {
	t.Parallel()

	t.Run("exactly MaxDirectiveFileSize in bytes passes", func(t *testing.T) {
		content := make([]byte, MaxDirectiveFileSize)
		for i := range content {
			content[i] = 'X'
		}
		mfs := &MockFileSystem{
			Files: map[string][]byte{"/work/limit.txt": content},
			WD:    "/work",
		}
		r, err := NewExpressionResolverWithFS(&HostContext{}, mfs)
		require.NoError(t, err)

		val, err := r.resolveFile("limit.txt")
		require.NoError(t, err)
		assert.Len(t, val, MaxDirectiveFileSize)
	})

	t.Run("MaxDirectiveFileSize + 1 byte fails", func(t *testing.T) {
		content := make([]byte, MaxDirectiveFileSize+1)
		for i := range content {
			content[i] = 'Y'
		}
		mfs := &MockFileSystem{
			Files: map[string][]byte{"/work/overflow.txt": content},
			WD:    "/work",
		}
		r, err := NewExpressionResolverWithFS(&HostContext{}, mfs)
		require.NoError(t, err)

		_, err = r.resolveFile("overflow.txt")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "too large")
	})
}
