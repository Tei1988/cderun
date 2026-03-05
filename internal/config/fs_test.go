package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_FS_RealFileSystem(t *testing.T) {
	fs := RealFileSystem{}

	t.Run("Executable", func(t *testing.T) {
		t.Parallel()
		path, err := fs.Executable()
		require.NoError(t, err)
		assert.NotEmpty(t, path)
	})

	t.Run("Getenv", func(t *testing.T) {
		t.Setenv("CDERUN_TEST_FS", "value")
		assert.Equal(t, "value", fs.Getenv("CDERUN_TEST_FS"))
	})

	t.Run("TempDir", func(t *testing.T) {
		t.Parallel()
		assert.NotEmpty(t, fs.TempDir())
	})

	t.Run("UserHomeDir", func(t *testing.T) {
		t.Parallel()
		home, err := fs.UserHomeDir()
		require.NoError(t, err)
		assert.NotEmpty(t, home)
	})

	t.Run("Getwd", func(t *testing.T) {
		t.Parallel()
		wd, err := fs.Getwd()
		require.NoError(t, err)
		assert.NotEmpty(t, wd)
	})

	t.Run("Abs", func(t *testing.T) {
		t.Parallel()
		abs, err := fs.Abs("config.go")
		require.NoError(t, err)
		assert.True(t, os.IsPathSeparator(abs[0]) || (len(abs) > 1 && abs[1] == ':'))
	})
}

func TestUnit_FS_MockFileSystem(t *testing.T) {
	t.Parallel()

	t.Run("Getwd", func(t *testing.T) {
		t.Parallel()
		mfs := &MockFileSystem{WD: "/project"}
		wd, err := mfs.Getwd()
		require.NoError(t, err)
		assert.Equal(t, "/project", wd)
	})

	t.Run("Abs", func(t *testing.T) {
		t.Parallel()
		mfs := &MockFileSystem{WD: "/project"}
		abs, err := mfs.Abs("rel")
		require.NoError(t, err)
		assert.Equal(t, "/project/rel", abs)

		abs, err = mfs.Abs("/abs")
		require.NoError(t, err)
		assert.Equal(t, "/abs", abs)
	})

	t.Run("Executable", func(t *testing.T) {
		t.Parallel()
		mfs := &MockFileSystem{ExecPath: "/bin/cderun"}
		path, err := mfs.Executable()
		require.NoError(t, err)
		assert.Equal(t, "/bin/cderun", path)
	})

	t.Run("Getenv", func(t *testing.T) {
		t.Parallel()
		mfs := &MockFileSystem{Env: map[string]string{"K": "V"}}
		assert.Equal(t, "V", mfs.Getenv("K"))
		assert.Empty(t, mfs.Getenv("NONEXISTENT"))
	})

	t.Run("Errors", func(t *testing.T) {
		t.Parallel()
		mfs := &MockFileSystem{}
		mfs.ExecErr = os.ErrPermission
		_, err := mfs.Executable()
		require.ErrorIs(t, err, os.ErrPermission)

		mfs.MkdirAllErr = os.ErrPermission
		err = mfs.MkdirAll("/tmp", 0755)
		require.ErrorIs(t, err, os.ErrPermission)

		mfs.WriteFileErr = os.ErrPermission
		err = mfs.WriteFile("/tmp/f", []byte{}, 0644)
		require.ErrorIs(t, err, os.ErrPermission)

		mfs.ReadFileErr = os.ErrPermission
		_, err = mfs.ReadFile("/tmp/f")
		require.ErrorIs(t, err, os.ErrPermission)

		mfs.RemoveAllErr = os.ErrPermission
		err = mfs.RemoveAll("/tmp")
		require.ErrorIs(t, err, os.ErrPermission)
	})

	t.Run("RemoveAll and MkdirAll logic", func(t *testing.T) {
		t.Parallel()
		m := &MockFileSystem{
			Dirs:  map[string]bool{"/a": true, "/a/b": true, "/c": true},
			Files: map[string][]byte{"/a/f1": {}, "/a/b/f2": {}},
		}

		err := m.RemoveAll("/a")
		require.NoError(t, err)

		assert.False(t, m.Dirs["/a"])
		assert.False(t, m.Dirs["/a/b"])
		assert.True(t, m.Dirs["/c"])
		assert.Empty(t, m.Files["/a/f1"])
		assert.Empty(t, m.Files["/a/b/f2"])
	})
}

func TestUnit_ConfigLoader_Initialization(t *testing.T) {
	t.Parallel()
	mfs := &MockFileSystem{}
	loader := NewConfigLoaderWithFS(mfs)
	assert.Equal(t, mfs, loader.fs)
	assert.Equal(t, "/etc/cderun", loader.systemConfigDir)
	assert.Equal(t, "/run/cderun", loader.runConfigDir)
}
