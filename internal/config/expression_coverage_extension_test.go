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
	statFileInfo  os.FileInfo
	statErr       error
	failStatPath  string
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
