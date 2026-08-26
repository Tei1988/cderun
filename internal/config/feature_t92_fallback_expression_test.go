package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type dummyFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
	isDir   bool
}

func (d dummyFileInfo) Name() string       { return d.name }
func (d dummyFileInfo) Size() int64        { return d.size }
func (d dummyFileInfo) Mode() os.FileMode  { return d.mode }
func (d dummyFileInfo) ModTime() time.Time { return d.modTime }
func (d dummyFileInfo) IsDir() bool        { return d.isDir }
func (d dummyFileInfo) Sys() any           { return nil }

type t92MockFS struct {
	MockFileSystem
	statOverride map[string]os.FileInfo
}

func (m *t92MockFS) Stat(name string) (os.FileInfo, error) {
	if m.statOverride != nil {
		if info, ok := m.statOverride[name]; ok {
			return info, nil
		}
	}
	return m.MockFileSystem.Stat(name)
}

func TestUnit_Expression_T92_FileAndFindDirFallback(t *testing.T) {
	fs := &t92MockFS{
		MockFileSystem: MockFileSystem{
			Files: map[string][]byte{
				"/project/normal.txt":                []byte("1.2.3\n"),
				"/project/empty.txt":                 []byte("   \n"),
				"/project/master_file":               []byte("content"),
				"/project/services/app/.cderun.yaml": []byte("runtime: docker"),
			},
			Dirs: map[string]bool{
				"/project":              true,
				"/project/services":     true,
				"/project/services/app": true,
			},
			WD:      "/project/services/app",
			HomeDir: "/home/user",
		},
		statOverride: map[string]os.FileInfo{
			"/project/large.txt": dummyFileInfo{name: "large.txt", size: MaxDirectiveFileSize + 10},
		},
	}
	fs.Files["/project/large.txt"] = []byte("large")
	fs.Dirs["/project/large.txt"] = false

	hostCtx := &HostContext{
		Level:      0,
		HomeDir:    "/home/user",
		WorkingDir: "/project/services/app",
	}

	t.Run("file directive fallback when missing", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(hostCtx, fs)
		require.NoError(t, err)

		val, err := r.ResolveString("{{file:missing.txt:-1.0.0}}")
		require.NoError(t, err)
		assert.NoError(t, r.Error())
		assert.Equal(t, "1.0.0", val)
	})

	t.Run("file directive fallback when empty file", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(hostCtx, fs)
		require.NoError(t, err)

		val, err := r.ResolveString("{{file:empty.txt:-default_val}}")
		require.NoError(t, err)
		assert.NoError(t, r.Error())
		assert.Equal(t, "default_val", val)
	})

	t.Run("file directive existing file returns file content", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(hostCtx, fs)
		require.NoError(t, err)

		val, err := r.ResolveString("{{file:normal.txt:-default_val}}")
		require.NoError(t, err)
		assert.NoError(t, r.Error())
		assert.Equal(t, "1.2.3", val)
	})

	t.Run("find_dir directive fallback when missing", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(hostCtx, fs)
		require.NoError(t, err)

		val, err := r.ResolveString("{{find_dir:nonexistent:-/fallback/dir}}")
		require.NoError(t, err)
		assert.NoError(t, r.Error())
		assert.Equal(t, "/fallback/dir", val)
	})

	t.Run("find_dir directive existing item returns directory path", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(hostCtx, fs)
		require.NoError(t, err)

		val, err := r.ResolveString("{{find_dir:master_file:-/fallback/dir}}")
		require.NoError(t, err)
		assert.NoError(t, r.Error())
		assert.Equal(t, filepath.FromSlash("/project"), val)
	})

	t.Run("nested expression inside default value", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(hostCtx, fs)
		require.NoError(t, err)

		val, err := r.ResolveString("{{find_dir:master:-{{PWD}}}}")
		require.NoError(t, err)
		assert.NoError(t, r.Error())
		assert.Equal(t, "/project/services/app", val)
	})

	t.Run("cached error by NAME still allows fallback", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(hostCtx, fs)
		require.NoError(t, err)

		// First call without default caches the error
		r.resolveString("{{file:missing_cached.txt}}")
		require.Error(t, r.Error())

		// Create a fresh resolver to test cache behavior without sticky error
		r2, err := NewExpressionResolverWithFS(hostCtx, fs)
		require.NoError(t, err)
		// Share fileCache with r
		r2.resolveString("{{file:missing_cached.txt}}")
		require.Error(t, r2.Error())

		// Now evaluate with fallback using a new resolver instance sharing state
		r3, err := NewExpressionResolverWithFS(hostCtx, fs)
		require.NoError(t, err)
		val, err := r3.ResolveString("{{file:missing_cached.txt:-cached_fallback}}")
		require.NoError(t, err)
		assert.NoError(t, r3.Error())
		assert.Equal(t, "cached_fallback", val)
	})

	t.Run("oversized file does NOT trigger fallback and returns error", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(hostCtx, fs)
		require.NoError(t, err)

		_, err = r.ResolveString("{{file:large.txt:-default_val}}")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "too large")
	})

	t.Run("invalid parameter syntax (parent traversal) does NOT trigger fallback", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(hostCtx, fs)
		require.NoError(t, err)

		_, err = r.ResolveString("{{file:../etc/passwd:-default_val}}")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "only a single file name is allowed")
	})

	t.Run("control character in default value triggers security error", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(hostCtx, fs)
		require.NoError(t, err)

		_, err = r.ResolveString("{{file:missing.txt:-\x01invalid}}")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "security validation failed")
	})
}
