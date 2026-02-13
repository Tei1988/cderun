package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_RealFileSystem(t *testing.T) {
	fs := RealFileSystem{}

	t.Run("Executable", func(t *testing.T) {
		exe, err := fs.Executable()
		assert.NoError(t, err)
		assert.NotEmpty(t, exe)
	})

	t.Run("Getenv", func(t *testing.T) {
		t.Setenv("TEST_VAR", "value")
		assert.Equal(t, "value", fs.Getenv("TEST_VAR"))
	})

	t.Run("TempDir", func(t *testing.T) {
		assert.NotEmpty(t, fs.TempDir())
	})

	t.Run("File operations", func(t *testing.T) {
		tmp := t.TempDir()
		path := filepath.Join(tmp, "subdir", "file.txt")

		err := fs.MkdirAll(filepath.Dir(path), 0755)
		assert.NoError(t, err)

		err = fs.WriteFile(path, []byte("hello"), 0644)
		assert.NoError(t, err)

		data, err := fs.ReadFile(path)
		assert.NoError(t, err)
		assert.Equal(t, "hello", string(data))

		err = fs.RemoveAll(filepath.Dir(path))
		assert.NoError(t, err)

		_, err = fs.Stat(path)
		assert.Error(t, err)
	})
}

func TestUnit_Config_MockFileSystem_NewMethods(t *testing.T) {
	mfs := &MockFileSystem{
		ExecPath: "/bin/cderun",
		Env:      map[string]string{"K": "V"},
	}

	t.Run("Executable", func(t *testing.T) {
		exe, err := mfs.Executable()
		assert.NoError(t, err)
		assert.Equal(t, "/bin/cderun", exe)

		mfs.ExecErr = os.ErrPermission
		_, err = mfs.Executable()
		assert.Error(t, err)
		mfs.ExecErr = nil
	})

	t.Run("Getenv", func(t *testing.T) {
		assert.Equal(t, "V", mfs.Getenv("K"))
		assert.Equal(t, "", mfs.Getenv("UNKNOWN"))
	})

	t.Run("TempDir", func(t *testing.T) {
		assert.Equal(t, "/tmp", mfs.TempDir())
		mfs.TempDirValue = "/custom/tmp"
		assert.Equal(t, "/custom/tmp", mfs.TempDir())
	})

	t.Run("MkdirAll and Stat", func(t *testing.T) {
		err := mfs.MkdirAll("/a/b", 0755)
		assert.NoError(t, err)
		_, err = mfs.Stat("/a/b")
		assert.NoError(t, err)

		mfs.MkdirAllErr = os.ErrPermission
		err = mfs.MkdirAll("/c", 0755)
		assert.Error(t, err)
		mfs.MkdirAllErr = nil
	})

	t.Run("WriteFile and ReadFile", func(t *testing.T) {
		err := mfs.WriteFile("/f", []byte("d"), 0644)
		assert.NoError(t, err)
		data, err := mfs.ReadFile("/f")
		assert.NoError(t, err)
		assert.Equal(t, "d", string(data))

		mfs.WriteFileErr = os.ErrPermission
		err = mfs.WriteFile("/g", []byte("d"), 0644)
		assert.Error(t, err)
		mfs.WriteFileErr = nil

		mfs.ReadFileErr = os.ErrPermission
		_, err = mfs.ReadFile("/f")
		assert.Error(t, err)
		mfs.ReadFileErr = nil
	})

	t.Run("RemoveAll", func(t *testing.T) {
		require.NoError(t, mfs.WriteFile("/d/f1", []byte("1"), 0644))
		require.NoError(t, mfs.WriteFile("/d/f2", []byte("2"), 0644))
		require.NoError(t, mfs.MkdirAll("/d", 0755))

		err := mfs.RemoveAll("/d")
		assert.NoError(t, err)

		_, err = mfs.Stat("/d")
		assert.Error(t, err)
		_, err = mfs.ReadFile("/d/f1")
		assert.Error(t, err)

		mfs.RemoveAllErr = os.ErrPermission
		err = mfs.RemoveAll("/x")
		assert.Error(t, err)
		mfs.RemoveAllErr = nil
	})
}

func TestUnit_Config_NewConfigLoaderWithFS(t *testing.T) {
	mfs := &MockFileSystem{}
	loader := NewConfigLoaderWithFS(mfs)
	assert.Equal(t, mfs, loader.fs)
	assert.Equal(t, defaultLoader.systemConfigDir, loader.systemConfigDir)
	assert.Equal(t, defaultLoader.runConfigDir, loader.runConfigDir)
}
