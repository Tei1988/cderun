package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockFileInfoLarge struct {
	name string
	size int64
}

func (m *mockFileInfoLarge) Name() string       { return m.name }
func (m *mockFileInfoLarge) Size() int64        { return m.size }
func (m *mockFileInfoLarge) Mode() os.FileMode  { return 0 }
func (m *mockFileInfoLarge) ModTime() time.Time { return time.Time{} }
func (m *mockFileInfoLarge) IsDir() bool        { return false }
func (m *mockFileInfoLarge) Sys() any           { return nil }

type largeFileMockFS struct {
	MockFileSystem
	statFileInfo os.FileInfo
	statErr      error
	failStatPath string
}

func (m *largeFileMockFS) Stat(name string) (os.FileInfo, error) {
	if m.failStatPath != "" && name == m.failStatPath {
		return nil, m.statErr
	}
	if m.statFileInfo != nil {
		return m.statFileInfo, nil
	}
	return m.MockFileSystem.Stat(name)
}

func TestUnit_ExpressionResolver_ResolveFile_LargeFile(t *testing.T) {
	fs := &largeFileMockFS{
		MockFileSystem: MockFileSystem{
			WD:    "/work",
			Files: map[string][]byte{"/work/large.txt": []byte("too large")},
			Dirs:  map[string]bool{"/work": true},
		},
		statFileInfo: &mockFileInfoLarge{name: "large.txt", size: MaxDirectiveFileSize + 1},
	}

	r, err := NewExpressionResolverWithFS(nil, fs)
	require.NoError(t, err)

	_, err = r.ResolveString("{{ file:large.txt }}")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is too large")
}

func TestUnit_ExpressionResolver_ResolveFile_LargeData(t *testing.T) {
	// Size is OK in Stat, but actual data is too large
	data := make([]byte, MaxDirectiveFileSize+1)
	fs := &largeFileMockFS{
		MockFileSystem: MockFileSystem{
			WD:    "/work",
			Files: map[string][]byte{"/work/large_data.txt": data},
			Dirs:  map[string]bool{"/work": true},
		},
		statFileInfo: &mockFileInfoLarge{name: "large_data.txt", size: 100},
	}

	r, err := NewExpressionResolverWithFS(nil, fs)
	require.NoError(t, err)

	_, err = r.ResolveString("{{ file:large_data.txt }}")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is too large")
}

func TestUnit_ExpressionResolver_ResolveFile_StatError(t *testing.T) {
	fs := &largeFileMockFS{
		MockFileSystem: MockFileSystem{
			WD:    "/work",
			Files: map[string][]byte{"/work/err.txt": []byte("content")},
			Dirs:  map[string]bool{"/work": true},
		},
		statErr: assert.AnError,
	}

	r, err := NewExpressionResolverWithFS(nil, fs)
	require.NoError(t, err)
	r.ensureLoader()

	// Ensure FindConfigs works by temporarily disabling the error
	fs.failStatPath = ""
	_ = r.shared.loader.FindConfigs("err.txt")

	// Now enable the error for direct Stat call in resolveFile
	fs.failStatPath = "/work/err.txt"

	_, err = r.ResolveString("{{ file:err.txt }}")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to stat file")
}

func TestUnit_ExpressionResolver_ResolveString_Coverage(t *testing.T) {
	t.Run("sticky error", func(t *testing.T) {
		r, _ := NewExpressionResolverWithFS(nil, &MockFileSystem{})
		r.err = assert.AnError
		assert.Equal(t, "some-string", r.resolveString("some-string"))
		assert.Equal(t, "{{HOME}}", r.resolveString("{{HOME}}"))
	})

	t.Run("expandHome direct", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, &MockFileSystem{
			HomeDir: "/home/user",
			WD:      "/work",
		})
		require.NoError(t, err)

		assert.Equal(t, "/home/user/path", r.resolveString("~/path"))
		assert.Equal(t, "/home/user", r.resolveString("~"))
	})

	t.Run("magic words exhaustive", func(t *testing.T) {
		hostCtx := &HostContext{
			HomeDir:    "/base/home",
			WorkingDir: "/base/pwd",
		}
		r, _ := NewExpressionResolverWithFS(hostCtx, &MockFileSystem{HomeDir: "/home", WD: "/pwd"})

		assert.Equal(t, "/home", r.resolveString("{{HOME}}"))
		assert.Equal(t, "/pwd", r.resolveString("{{PWD}}"))
		assert.Equal(t, "/base/home", r.resolveString("{{BASE_HOME}}"))
		assert.Equal(t, "/base/pwd", r.resolveString("{{BASE_PWD}}"))
	})

	t.Run("magic words fallback exhaustive", func(t *testing.T) {
		r, _ := NewExpressionResolverWithFS(nil, &MockFileSystem{HomeDir: "/home", WD: "/pwd"})

		assert.Equal(t, "/home", r.resolveString("{{BASE_HOME}}"))
		assert.Equal(t, "/pwd", r.resolveString("{{BASE_PWD}}"))
	})

	t.Run("env with default", func(t *testing.T) {
		r, _ := NewExpressionResolverWithFS(nil, &MockFileSystem{
			Env: map[string]string{"EXISTING": "val"},
		})

		assert.Equal(t, "val", r.resolveString("{{env:EXISTING:-default}}"))
		assert.Equal(t, "default", r.resolveString("{{env:MISSING:-default}}"))
		assert.Empty(t, r.resolveString("{{env:MISSING}}"))
	})
}

func TestUnit_ExpressionResolver_ResolveFindDir_AbsError(t *testing.T) {
	mfs := &MockFileSystem{
		WD:      "/work",
		AbsErr:  assert.AnError,
		HomeDir: "/home",
		Files:   map[string][]byte{"/work/.git": []byte("")},
		Dirs:    map[string]bool{"/work": true},
	}
	r, err := NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

	_, err = r.ResolveString("{{ find_dir:.git }}")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get absolute path")
}

func TestUnit_ExpressionResolver_ApplyReverseResolution_AbsError(t *testing.T) {
	mfs := &MockFileSystem{
		WD:      "/work",
		HomeDir: "/home",
	}
	hostCtx := &HostContext{Level: 1}
	r, err := NewExpressionResolverWithFS(hostCtx, mfs)
	require.NoError(t, err)

	// induce Abs error
	mfs.AbsErr = assert.AnError
	_, err = r.applyReverseResolution("relative/path")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get absolute path")
}

func TestUnit_ExpressionResolver_Resolve_Exhaustive(t *testing.T) {
	t.Run("sticky error in Resolve", func(t *testing.T) {
		r, _ := NewExpressionResolver(nil)
		r.err = assert.AnError
		assert.Equal(t, "input", r.Resolve("input"))
	})

	t.Run("slice resolution", func(t *testing.T) {
		r, _ := NewExpressionResolverWithFS(nil, &MockFileSystem{HomeDir: "/home/jules", WD: "/work"})
		input := []any{"{{HOME}}", "literal"}
		res := r.Resolve(input).([]any)
		assert.Equal(t, "/home/jules", res[0])
		assert.Equal(t, "literal", res[1])
	})

	t.Run("map resolution", func(t *testing.T) {
		r, _ := NewExpressionResolverWithFS(nil, &MockFileSystem{HomeDir: "/home/jules", WD: "/work"})
		input := map[string]any{"key": "{{HOME}}"}
		res := r.Resolve(input).(map[string]any)
		assert.Equal(t, "/home/jules", res["key"])
	})

	t.Run("default case", func(t *testing.T) {
		r, _ := NewExpressionResolver(nil)
		assert.Equal(t, 123, r.Resolve(123))
	})
}
