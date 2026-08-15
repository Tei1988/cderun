package config

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpressionResolver_AdvancedReverseResolutionAndMounts(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD:      "/home/user/project",
		HomeDir: "/home/user",
	}

	hostCtx := &HostContext{
		Level:      1,
		HomeDir:    "/host/home/user",
		WorkingDir: "/host/home/user/project",
		Mounts: []MountMapping{
			{
				Source: "/host/data",
				Target: "/data",
				Level:  1,
			},
			{
				Source: "/host/data/nested",
				Target: "/data/nested",
				Level:  1,
			},
			{
				Source: "/host/override/data/nested",
				Target: "/data/nested",
				Level:  2, // Higher level tie-breaker
			},
		},
	}

	resolver, err := NewExpressionResolverWithFS(hostCtx, mfs)
	require.NoError(t, err)

	t.Run("reverse resolution tie breaker prefers longer target or higher level", func(t *testing.T) {
		// Matches /data/nested with level 2 override via ResolvePath
		res, err := ResolvePath("/data/nested/file.txt", "/base", resolver)
		require.NoError(t, err)
		assert.Equal(t, filepath.Clean("/host/override/data/nested/file.txt"), filepath.Clean(res))

		// Matches /data with level 1 via ResolvePath
		res2, err := ResolvePath("/data/other.txt", "/base", resolver)
		require.NoError(t, err)
		assert.Equal(t, filepath.Clean("/host/data/other.txt"), filepath.Clean(res2))

		// Unmatched path returns absolute path as-is
		res3, err := ResolvePath("/unmatched/path.txt", "/base", resolver)
		require.NoError(t, err)
		assert.Equal(t, filepath.Clean("/unmatched/path.txt"), filepath.Clean(res3))
	})
}

func TestExpressionResolver_DirectiveEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("file directive exceeds max size", func(t *testing.T) {
		mfs := &MockFileSystem{
			WD:      "/app",
			HomeDir: "/home/user",
			Files: map[string][]byte{
				"/app/large.txt": make([]byte, MaxDirectiveFileSize+10),
			},
		}

		resolver, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		_, err = resolver.ResolveString("{{file:large.txt}}")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "too large")
	})

	t.Run("env directive default value invalid characters rejection", func(t *testing.T) {
		mfs := &MockFileSystem{
			WD:      "/app",
			HomeDir: "/home/user",
		}

		resolver, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		// Bad default value with control character
		_, err = resolver.ResolveString("{{env:NON_EXISTENT_VAR:-invalid\x00value}}")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "security validation failed")
	})

	t.Run("find_dir directive with invalid path characters", func(t *testing.T) {
		mfs := &MockFileSystem{
			WD:      "/app",
			HomeDir: "/home/user",
		}

		resolver, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		_, err = resolver.ResolveString("{{find_dir:bad\x00dir}}")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid character")
	})

	t.Run("nested expression resolution in env default value", func(t *testing.T) {
		mfs := &MockFileSystem{
			WD:      "/app",
			HomeDir: "/home/user",
		}

		resolver, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		// Resolves {{env:UNDEFINED:-{{HOME}}/config}} -> /home/user/config
		res, err := resolver.ResolveString("{{env:UNDEFINED:-{{HOME}}/config}}")
		require.NoError(t, err)
		assert.Equal(t, "/home/user/config", res)
	})
}

func TestExpressionResolver_StickyErrorPropagationInStructures(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD:      "/app",
		HomeDir: "/home/user",
	}

	resolver, err := NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

	input := map[string]any{
		"valid":   "{{HOME}}/valid",
		"invalid": "{{file:nonexistent.txt}}",
		"nested_map": map[string]any{
			"key": "{{HOME}}/nested",
		},
		"nested_slice": []any{
			"{{HOME}}/slice1",
			"{{HOME}}/slice2",
		},
	}

	resolved := resolver.Resolve(input)
	require.Error(t, resolver.Error())
	assert.Contains(t, resolver.Error().Error(), "file not found")

	// Verify resolution stops after encountering the sticky file error, preserving unresolved expressions
	resolvedMap, ok := resolved.(map[string]any)
	require.True(t, ok)
	nestedMap, ok := resolvedMap["nested_map"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "{{HOME}}/nested", nestedMap["key"])

	nestedSlice, ok := resolvedMap["nested_slice"].([]any)
	require.True(t, ok)
	require.Len(t, nestedSlice, 2)
	assert.Equal(t, "{{HOME}}/slice1", nestedSlice[0])
	assert.Equal(t, "{{HOME}}/slice2", nestedSlice[1])

	// Verify sticky error prevents further resolution when called again
	resString, err2 := resolver.ResolveString("{{HOME}}/should_not_resolve")
	require.Error(t, err2)
	assert.Equal(t, "{{HOME}}/should_not_resolve", resString)
}

func TestExpressionResolver_UnknownDirectivesAndEscapes(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD:      "/app",
		HomeDir: "/home/user",
	}

	t.Run("unknown uppercase directive returns error", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		_, err = r.ResolveString("{{UNKNOWN_MAGIC_WORD}}")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown directive or magic word")
	})

	t.Run("unknown colon directive returns error", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		_, err = r.ResolveString("{{custom_prefix:value}}")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown directive or magic word")
	})

	t.Run("double brace escape is preserved", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		// {{{{HOME}}}} should resolve to {{HOME}}
		res, err := r.ResolveString("{{{{HOME}}}}")
		require.NoError(t, err)
		assert.Equal(t, "{{HOME}}", res)
	})
}
