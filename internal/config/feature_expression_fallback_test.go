package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUnit_Expression_DirectiveFallbackMechanics tests expression fallback syntax (:-default)
// across file:, find_dir:, and env: directives according to docs/features/value-resolution.md
// (Directive Fallback Syntax & Rules).
func TestUnit_Expression_DirectiveFallbackMechanics(t *testing.T) {
	t.Parallel()

	t.Run("file_directive_fallback_on_missing_empty_unreadable", func(t *testing.T) {
		t.Parallel()

		mfs := &MockFileSystem{
			HomeDir: "/home/user",
			WD:      "/home/user/project",
			Files: map[string][]byte{
				"/home/user/project/empty.txt": []byte(""),
			},
		}

		expr, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		// 1. Missing file -> triggers fallback
		res, err := expr.ResolveString("{{file:missing.txt:-default_val}}")
		require.NoError(t, err)
		assert.Equal(t, "default_val", res)

		// 2. Empty file -> triggers fallback
		res, err = expr.ResolveString("{{file:empty.txt:-fallback_empty}}")
		require.NoError(t, err)
		assert.Equal(t, "fallback_empty", res)
	})

	t.Run("file_directive_bypasses_fallback_on_invalid_parameter_or_size_limit", func(t *testing.T) {
		t.Parallel()

		mfs := &MockFileSystem{
			HomeDir: "/home/user",
			WD:      "/home/user/project",
			Files: map[string][]byte{
				"/home/user/project/large.bin": make([]byte, MaxDirectiveFileSize+10),
			},
		}

		expr, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		// 1. Path traversal in parameter -> immediate error, bypasses fallback
		_, err = expr.ResolveString("{{file:../secret.txt:-fallback}}")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "only a single file name is allowed")

		// 2. Oversized file -> immediate error, bypasses fallback
		expr2, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)
		_, err = expr2.ResolveString("{{file:large.bin:-fallback}}")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "too large")
	})

	t.Run("find_dir_directive_fallback_and_success", func(t *testing.T) {
		t.Parallel()

		mfs := &MockFileSystem{
			HomeDir: "/home/user",
			WD:      "/home/user/project/app",
			Dirs: map[string]bool{
				"/home/user/project": true,
			},
			Files: map[string][]byte{
				"/home/user/project/master": []byte(""), // file/dir named master at project root
			},
		}

		expr, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		// 1. Target not found -> triggers fallback
		res, err := expr.ResolveString("{{find_dir:nonexistent:-/fallback/path}}")
		require.NoError(t, err)
		assert.Equal(t, "/fallback/path", res)

		// 2. Target found -> returns directory containing master
		res, err = expr.ResolveString("{{find_dir:master:-/fallback/path}}")
		require.NoError(t, err)
		assert.Equal(t, "/home/user/project", res)
	})

	t.Run("env_directive_fallback_and_values", func(t *testing.T) {
		t.Parallel()

		mfs := &MockFileSystem{
			HomeDir: "/home/user",
			WD:      "/home/user/project",
			Env: map[string]string{
				"SET_VAR":   "production",
				"EMPTY_VAR": "",
			},
		}

		expr, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		// 1. Unset variable -> fallback
		res, err := expr.ResolveString("{{env:UNSET_VAR:-development}}")
		require.NoError(t, err)
		assert.Equal(t, "development", res)

		// 2. Empty variable -> fallback
		res, err = expr.ResolveString("{{env:EMPTY_VAR:-fallback_empty}}")
		require.NoError(t, err)
		assert.Equal(t, "fallback_empty", res)

		// 3. Set variable -> actual value
		res, err = expr.ResolveString("{{env:SET_VAR:-staging}}")
		require.NoError(t, err)
		assert.Equal(t, "production", res)
	})

	t.Run("nested_fallback_expressions", func(t *testing.T) {
		t.Parallel()

		mfs := &MockFileSystem{
			HomeDir: "/home/user",
			WD:      "/home/user/project",
			Files: map[string][]byte{
				"/home/user/project/.version": []byte("v2.1.0"),
			},
		}

		expr, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		// Nested env with file fallback: {{env:APP_VERSION:-{{file:.version:-v1.0.0}}}}
		// Since APP_VERSION is unset, evaluates inner {{file:.version:-v1.0.0}} -> v2.1.0
		res, err := expr.ResolveString("{{env:APP_VERSION:-{{file:.version:-v1.0.0}}}}")
		require.NoError(t, err)
		assert.Equal(t, "v2.1.0", res)

		// Nested find_dir with PWD fallback: {{find_dir:missing_target:-{{PWD}}}}
		res, err = expr.ResolveString("{{find_dir:missing_target:-{{PWD}}}}")
		require.NoError(t, err)
		assert.Equal(t, "/home/user/project", res)
	})

	t.Run("sticky_error_isolation_on_successful_fallback", func(t *testing.T) {
		t.Parallel()

		mfs := &MockFileSystem{
			HomeDir: "/home/user",
			WD:      "/home/user/project",
		}

		expr, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		// Resolving a missing file with a fallback should NOT set a sticky error on the resolver
		res, err := expr.ResolveString("{{file:nonexistent.txt:-safe_fallback}}")
		require.NoError(t, err)
		assert.Equal(t, "safe_fallback", res)
		assert.NoError(t, expr.Error())

		// Subsequent resolution should work normally
		res, err = expr.ResolveString("Hello {{HOME}}")
		require.NoError(t, err)
		assert.Equal(t, "Hello /home/user", res)
	})

	t.Run("reverse_path_resolution_under_nested_host_context", func(t *testing.T) {
		t.Parallel()

		mfs := &MockFileSystem{
			HomeDir: "/root",
			WD:      "/workspace",
			Files: map[string][]byte{
				"/workspace/go.mod": []byte("module example"),
			},
		}

		hostCtx := &HostContext{
			Level:      1,
			HomeDir:    "/Users/dev",
			WorkingDir: "/Users/dev/code/app",
			Mounts: []MountMapping{
				{
					Source: "/Users/dev/code/app",
					Target: "/workspace",
					Level:  1,
				},
			},
		}

		expr, err := NewExpressionResolverWithFS(hostCtx, mfs)
		require.NoError(t, err)

		// find_dir:go.mod finds /workspace, which maps back to host source /Users/dev/code/app via reverse path resolution
		res, err := expr.ResolveString("{{find_dir:go.mod}}")
		require.NoError(t, err)
		assert.Equal(t, "/Users/dev/code/app", res)

		// Fallback value for find_dir bypasses reverse path resolution
		res, err = expr.ResolveString("{{find_dir:nonexistent:-/explicit/host/path}}")
		require.NoError(t, err)
		assert.Equal(t, "/explicit/host/path", res)
	})
}
