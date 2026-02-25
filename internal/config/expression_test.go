package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Expression_FindDir(t *testing.T) {
	fs := &MockFileSystem{
		Files: map[string][]byte{
			"/project/modules/foo":                      []byte("bar"),
			"/project/services/production/.cderun.yaml": []byte("runtime: docker"),
		},
		Dirs: map[string]bool{
			"/project":                     true,
			"/project/modules":             true,
			"/project/services":            true,
			"/project/services/production": true,
		},
		WD: "/project/services/production",
	}

	hostCtx := &HostContext{}
	r, err := NewExpressionResolverWithFS(hostCtx, fs)
	require.NoError(t, err)

	t.Run("find_dir existing directory", func(t *testing.T) {
		val := r.resolveString("{{ find_dir:modules }}")
		require.NoError(t, r.Error())
		assert.Equal(t, "../..", val)
	})

	t.Run("find_dir existing file", func(t *testing.T) {
		val := r.resolveString("{{ find_dir:modules/foo }}")
		require.NoError(t, r.Error())
		assert.Equal(t, "../../modules", val)
	})

	t.Run("find_dir not found", func(t *testing.T) {
		r2, err := NewExpressionResolverWithFS(hostCtx, fs)
		require.NoError(t, err)
		r2.resolveString("{{ find_dir:nonexistent }}")
		require.Error(t, r2.Error())
		assert.Contains(t, r2.Error().Error(), "item not found for find_dir: nonexistent")
	})
}

func TestUnit_Expression_File_Error(t *testing.T) {
	fs := &MockFileSystem{
		Files: map[string][]byte{
			"/project/.go-version": []byte("1.21\n"),
		},
		Dirs: map[string]bool{
			"/project": true,
		},
		WD: "/project",
	}

	hostCtx := &HostContext{}
	r, err := NewExpressionResolverWithFS(hostCtx, fs)
	require.NoError(t, err)

	t.Run("file found", func(t *testing.T) {
		val := r.resolveString("{{ file:.go-version }}")
		require.NoError(t, r.Error())
		assert.Equal(t, "1.21", val)
	})

	t.Run("file not found", func(t *testing.T) {
		r2, err := NewExpressionResolverWithFS(hostCtx, fs)
		require.NoError(t, err)
		r2.resolveString("{{ file:missing }}")
		require.Error(t, r2.Error())
		assert.Contains(t, r2.Error().Error(), "file not found: missing")
	})
}

func TestUnit_Expression_File_Empty(t *testing.T) {
	fs := &MockFileSystem{
		Files: map[string][]byte{
			"/project/empty.txt":  []byte("   "),
			"/project/normal.txt": []byte("content"),
		},
		Dirs: map[string]bool{
			"/project": true,
		},
		WD: "/project",
	}

	hostCtx := &HostContext{}
	r, err := NewExpressionResolverWithFS(hostCtx, fs)
	require.NoError(t, err)

	t.Run("empty file resolves to empty string", func(t *testing.T) {
		val := r.resolveString("{{ file:empty.txt }}")
		require.NoError(t, r.Error())
		assert.Empty(t, val)
	})

	t.Run("normal file still works", func(t *testing.T) {
		val := r.resolveString("{{ file:normal.txt }}")
		require.NoError(t, r.Error())
		assert.Equal(t, "content", val)
	})

	t.Run("cached empty file still works", func(t *testing.T) {
		// Second call should hit cache
		val := r.resolveString("{{ file:empty.txt }}")
		require.NoError(t, r.Error())
		assert.Empty(t, val)
	})
}

func TestUnit_Expression_SecurityAndEdgeCases(t *testing.T) {
	fs := &MockFileSystem{
		Files: map[string][]byte{
			"/project/inner.txt": []byte("outer.txt"),
			"/project/outer.txt": []byte("content"),
		},
		Dirs: map[string]bool{
			"/project": true,
		},
		WD: "/project",
	}

	hostCtx := &HostContext{}

	t.Run("nested expressions (partial match due to non-recursive regex)", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(hostCtx, fs)
		require.NoError(t, err)
		val := r.resolveString("{{ file:{{ file:inner.txt }} }}")
		// Matches "{{ file:{{ file:inner.txt }}" and fails to find such file
		require.Error(t, r.Error())
		assert.Contains(t, r.Error().Error(), "file not found")
		assert.Equal(t, "{{ file:{{ file:inner.txt }} }}", val)
	})

	t.Run("multiple expressions", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(hostCtx, fs)
		require.NoError(t, err)
		val := r.resolveString("{{ PWD }}/{{ file:inner.txt }}")
		require.NoError(t, r.Error())
		assert.Equal(t, "/project/outer.txt", val)
	})

	t.Run("path traversal attempt in file", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(hostCtx, fs)
		require.NoError(t, err)
		r.resolveString("{{ file:../../etc/passwd }}")
		require.Error(t, r.Error())
		assert.Contains(t, r.Error().Error(), "parent directory references are not allowed")
	})

	t.Run("absolute path attempt in file", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(hostCtx, fs)
		require.NoError(t, err)
		r.resolveString("{{ file:/etc/passwd }}")
		require.Error(t, r.Error())
		assert.Contains(t, r.Error().Error(), "absolute paths")
		assert.Contains(t, r.Error().Error(), "are not allowed")
	})

	t.Run("invalid directive", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(hostCtx, fs)
		require.NoError(t, err)
		val := r.resolveString("{{ unknown:value }}")
		require.NoError(t, r.Error())
		// Unknown directive is returned as is (but trimmed)
		assert.Equal(t, "{{unknown:value}}", val)
	})

	t.Run("unclosed expression", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(hostCtx, fs)
		require.NoError(t, err)
		val := r.resolveString("{{ PWD")
		require.NoError(t, r.Error())
		assert.Equal(t, "{{ PWD", val)
	})

	t.Run("empty expression", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(hostCtx, fs)
		require.NoError(t, err)
		val := r.resolveString("{{}}")
		require.NoError(t, r.Error())
		assert.Equal(t, "{{}}", val)
	})
}

func TestUnit_Expression_EdgeCases(t *testing.T) {
	fs := &MockFileSystem{
		WD: "/project",
	}
	r, _ := NewExpressionResolverWithFS(nil, fs)

	t.Run("unmatched braces in middle", func(t *testing.T) {
		val := r.resolveString("prefix {{ PWD suffix")
		assert.Equal(t, "prefix {{ PWD suffix", val)
	})

	t.Run("multiple directives on one line", func(t *testing.T) {
		val := r.resolveString("{{PWD}} and {{HOME}}")
		assert.Contains(t, val, "/project")
	})

	t.Run("invalid directive syntax", func(t *testing.T) {
		val := r.resolveString("{{ !@#$% }}")
		assert.Equal(t, "{{!@#$%}}", val) // Unknown directive returned as is
	})

	t.Run("malformed file directive", func(t *testing.T) {
		r2, _ := NewExpressionResolverWithFS(nil, fs)
		r2.resolveString("{{ file: }}")
		// The regex matches "file: ", and resolveFile handles empty string
		assert.Error(t, r2.Error())
	})

	t.Run("HOME when UserHomeDir fails", func(t *testing.T) {
		mfs := &MockFileSystem{
			HomeErr: assert.AnError,
		}
		r2, _ := NewExpressionResolverWithFS(nil, mfs)
		val := r2.resolveString("{{HOME}}")
		assert.Empty(t, val) // Should fallback to empty string in NewExpressionResolverWithFS
	})
}
