package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegration_Config_FS_RealFileSystem_Operations(t *testing.T) {
	fs := RealFileSystem{}

	t.Run("Executable", func(t *testing.T) {
		exe, err := fs.Executable()
		require.NoError(t, err)
		assert.NotEmpty(t, exe)
	})

	t.Run("Getenv", func(t *testing.T) {
		// Use t.Setenv for RealFileSystem as it truly uses OS environment.
		// RealFileSystem tests should not be run in parallel with other tests that depend on environment.
		t.Setenv("TEST_VAR", "value")
		assert.Equal(t, "value", fs.Getenv("TEST_VAR"))
	})

	t.Run("TempDir", func(t *testing.T) {
		assert.NotEmpty(t, fs.TempDir())
	})

	t.Run("File operations", func(t *testing.T) {
		tmp := t.TempDir()
		path := filepath.Join(tmp, "subdir", "file.txt")

		err := fs.MkdirAll(filepath.Dir(path), 0o755)
		require.NoError(t, err)

		err = fs.WriteFile(path, []byte("hello"), 0o644)
		require.NoError(t, err)

		data, err := fs.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, "hello", string(data))

		err = fs.RemoveAll(filepath.Dir(path))
		require.NoError(t, err)

		_, err = fs.Stat(path)
		require.Error(t, err)
	})
}

func TestUnit_Config_FS_MockFileSystem_Operations(t *testing.T) {
	t.Parallel()

	t.Run("Executable", func(t *testing.T) {
		t.Parallel()
		mfs := &MockFileSystem{
			ExecPath: "/bin/cderun",
		}
		exe, err := mfs.Executable()
		require.NoError(t, err)
		assert.Equal(t, "/bin/cderun", exe)

		mfs.ExecErr = os.ErrPermission
		_, err = mfs.Executable()
		require.Error(t, err)
		mfs.ExecErr = nil
	})

	t.Run("Getenv", func(t *testing.T) {
		t.Parallel()
		mfs := &MockFileSystem{
			Env: map[string]string{"K": "V"},
		}
		assert.Equal(t, "V", mfs.Getenv("K"))
		assert.Empty(t, mfs.Getenv("UNKNOWN"))
	})

	t.Run("TempDir", func(t *testing.T) {
		t.Parallel()
		mfs := &MockFileSystem{}
		assert.Equal(t, "/tmp", mfs.TempDir())
		mfs.TempDirValue = "/custom/tmp"
		assert.Equal(t, "/custom/tmp", mfs.TempDir())
	})

	t.Run("MkdirAll and Stat", func(t *testing.T) {
		t.Parallel()
		mfs := &MockFileSystem{}
		err := mfs.MkdirAll("/a/b", 0o755)
		require.NoError(t, err)
		_, err = mfs.Stat("/a/b")
		require.NoError(t, err)

		mfs.MkdirAllErr = os.ErrPermission
		err = mfs.MkdirAll("/c", 0o755)
		require.Error(t, err)
	})

	t.Run("WriteFile and ReadFile", func(t *testing.T) {
		t.Parallel()
		mfs := &MockFileSystem{}
		err := mfs.WriteFile("/f", []byte("d"), 0o644)
		require.NoError(t, err)
		data, err := mfs.ReadFile("/f")
		require.NoError(t, err)
		assert.Equal(t, "d", string(data))

		mfs.WriteFileErr = os.ErrPermission
		err = mfs.WriteFile("/g", []byte("d"), 0o644)
		require.Error(t, err)

		mfs.ReadFileErr = os.ErrPermission
		_, err = mfs.ReadFile("/f")
		require.Error(t, err)
	})

	t.Run("RemoveAll", func(t *testing.T) {
		t.Parallel()
		mfs := &MockFileSystem{}
		require.NoError(t, mfs.WriteFile("/d/f1", []byte("1"), 0o644))
		require.NoError(t, mfs.WriteFile("/d/f2", []byte("2"), 0o644))
		require.NoError(t, mfs.MkdirAll("/d", 0o755))

		err := mfs.RemoveAll("/d")
		require.NoError(t, err)

		_, err = mfs.Stat("/d")
		require.Error(t, err)
		_, err = mfs.ReadFile("/d/f1")
		require.Error(t, err)

		mfs.RemoveAllErr = os.ErrPermission
		err = mfs.RemoveAll("/x")
		require.Error(t, err)
	})
}

func TestUnit_Config_Loader_NewWithFS(t *testing.T) {
	mfs := &MockFileSystem{}
	loader := NewConfigLoaderWithFS(mfs)
	assert.Equal(t, mfs, loader.fs)
	assert.Equal(t, defaultLoader.systemConfigDir, loader.systemConfigDir)
	assert.Equal(t, defaultLoader.runConfigDir, loader.runConfigDir)
}

func TestUnit_Config_FS_Abs_Resolution(t *testing.T) {
	t.Run("RealFileSystem", func(t *testing.T) {
		fs := RealFileSystem{}
		abs, err := fs.Abs(".")
		require.NoError(t, err)
		assert.True(t, filepath.IsAbs(abs))
	})

	t.Run("MockFileSystem", func(t *testing.T) {
		mfs := &MockFileSystem{WD: "/work"}
		tests := []struct {
			name     string
			path     string
			expected string
		}{
			{"relative", "dir/file", "/work/dir/file"},
			{"absolute", "/other/file", "/other/file"},
			{"dot", ".", "/work"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				abs, err := mfs.Abs(tt.path)
				require.NoError(t, err)
				assert.Equal(t, tt.expected, filepath.ToSlash(abs))
			})
		}
	})

	t.Run("ResolvePath propagates Abs error", func(t *testing.T) {
		mfs := &MockFileSystem{
			WD:     "/work",
			AbsErr: assert.AnError,
		}
		// ResolvePath uses fs.Abs when r.HostContext.Level > 0
		r := &ExpressionResolver{
			fs: mfs,
			HostContext: &HostContext{
				Level:  1,
				Mounts: []MountMapping{{Source: "/host", Target: "/work", Level: 1}},
			},
		}

		_, err := ResolvePath("relative", "/work", r)
		require.Error(t, err)
		require.ErrorIs(t, err, assert.AnError)
		assert.Contains(t, err.Error(), "failed to get absolute path")
	})
}
