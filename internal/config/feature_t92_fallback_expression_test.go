package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Expression_FileAndFindDirDirectiveFallbacks(t *testing.T) {
	tempDir := t.TempDir()

	// Create test structure in tempDir
	validFile := filepath.Join(tempDir, "valid.txt")
	require.NoError(t, os.WriteFile(validFile, []byte("  valid content  \n"), 0644))

	emptyFile := filepath.Join(tempDir, "empty.txt")
	require.NoError(t, os.WriteFile(emptyFile, []byte("   \n"), 0644))

	markerDir := filepath.Join(tempDir, "marker_dir")
	require.NoError(t, os.MkdirAll(markerDir, 0755))
	markerFile := filepath.Join(markerDir, "my_marker.txt")
	require.NoError(t, os.WriteFile(markerFile, []byte("marker"), 0644))

	oversizedFile := filepath.Join(tempDir, "oversized.txt")
	require.NoError(t, os.WriteFile(oversizedFile, make([]byte, MaxDirectiveFileSize+10), 0644))

	origWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tempDir))
	t.Cleanup(func() {
		_ = os.Chdir(origWd)
	})

	fs := RealFileSystem{}

	t.Run("file directive fallback when missing file", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, fs)
		require.NoError(t, err)

		val, err := r.ResolveString("{{file:nonexistent.txt:-fallback_content}}")
		assert.NoError(t, err)
		assert.NoError(t, r.Error())
		assert.Equal(t, "fallback_content", val)
	})

	t.Run("file directive fallback when file is empty", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, fs)
		require.NoError(t, err)

		val, err := r.ResolveString("{{file:empty.txt:-default_content}}")
		assert.NoError(t, err)
		assert.NoError(t, r.Error())
		assert.Equal(t, "default_content", val)
	})

	t.Run("file directive returns file content when present and non-empty", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, fs)
		require.NoError(t, err)

		val, err := r.ResolveString("{{file:valid.txt:-default_content}}")
		assert.NoError(t, err)
		assert.NoError(t, r.Error())
		assert.Equal(t, "valid content", val)
	})

	t.Run("file directive fails and does not fallback when size limit exceeded", func(t *testing.T) {
		rFresh, err := NewExpressionResolverWithFS(nil, fs)
		require.NoError(t, err)

		val, err := rFresh.ResolveString("{{file:oversized.txt:-fallback_for_oversized}}")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "too large")
		assert.NotEqual(t, "fallback_for_oversized", val)
	})

	t.Run("find_dir directive fallback when missing marker", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, fs)
		require.NoError(t, err)

		val, err := r.ResolveString("{{find_dir:nonexistent_marker:-/fallback/path}}")
		assert.NoError(t, err)
		assert.NoError(t, r.Error())
		assert.Equal(t, "/fallback/path", val)
	})

	t.Run("find_dir directive returns directory path when marker found", func(t *testing.T) {
		subDir := filepath.Join(tempDir, "marker_dir")
		origSubWd, err := os.Getwd()
		require.NoError(t, err)
		require.NoError(t, os.Chdir(subDir))
		defer func() { _ = os.Chdir(origSubWd) }()

		r, err := NewExpressionResolverWithFS(nil, fs)
		require.NoError(t, err)

		val, err := r.ResolveString("{{find_dir:my_marker.txt:-/fallback/path}}")
		assert.NoError(t, err)
		assert.NoError(t, r.Error())
		absMarkerDir, err := filepath.Abs(markerDir)
		require.NoError(t, err)
		assert.Equal(t, absMarkerDir, val)
	})

	t.Run("nested find_dir fallback to PWD magic word", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, fs)
		require.NoError(t, err)

		val, err := r.ResolveString("{{find_dir:missing_marker:-{{PWD}}}}")
		assert.NoError(t, err)
		assert.NoError(t, r.Error())
		absTempDir, err := filepath.Abs(tempDir)
		require.NoError(t, err)
		assert.Equal(t, absTempDir, val)
	})

	t.Run("multi level nested fallback find_dir to file to env to fallback", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(nil, fs)
		require.NoError(t, err)

		exprStr := "{{find_dir:missing_marker:-{{file:missing.txt:-{{env:UNSET_VAR_T92:-/ultimate/fallback}}}}}}"
		val, err := r.ResolveString(exprStr)
		assert.NoError(t, err)
		assert.NoError(t, r.Error())
		assert.Equal(t, "/ultimate/fallback", val)
	})

	t.Run("security validation rejects invalid default value in file directive", func(t *testing.T) {
		rFresh, err := NewExpressionResolverWithFS(nil, fs)
		require.NoError(t, err)

		_, err = rFresh.ResolveString("{{file:nonexistent.txt:-\x01invalid}}")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "security validation failed")
	})

	t.Run("security validation rejects invalid default value in find_dir directive", func(t *testing.T) {
		rFresh, err := NewExpressionResolverWithFS(nil, fs)
		require.NoError(t, err)

		_, err = rFresh.ResolveString("{{find_dir:nonexistent_marker:-\x01invalid}}")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "security validation failed")
	})

	t.Run("invalid parameter syntax in find_dir fails without fallback", func(t *testing.T) {
		rFresh, err := NewExpressionResolverWithFS(nil, fs)
		require.NoError(t, err)

		_, err = rFresh.ResolveString("{{find_dir:../invalid:-/fallback}}")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "only a single file or directory name is allowed")
	})

	t.Run("verify sticky error remains nil after successful fallbacks", func(t *testing.T) {
		rFresh, err := NewExpressionResolverWithFS(nil, fs)
		require.NoError(t, err)

		res1, err1 := rFresh.ResolveString("{{find_dir:missing_marker:-/default/path}}")
		assert.NoError(t, err1)
		assert.Equal(t, "/default/path", res1)

		res2, err2 := rFresh.ResolveString("{{file:missing_file.txt:-default_val}}")
		assert.NoError(t, err2)
		assert.Equal(t, "default_val", res2)

		assert.NoError(t, rFresh.Error())
	})
}
