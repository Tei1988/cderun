package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type pathExtraMockFS struct {
	MockFileSystem
	homeDirErr error
}

func (m *pathExtraMockFS) UserHomeDir() (string, error) {
	if m.homeDirErr != nil {
		return "", m.homeDirErr
	}
	return m.MockFileSystem.UserHomeDir()
}

func TestUnit_Config_Path_ValidateAnchorBoundaries_Extra(t *testing.T) {
	t.Parallel()
	home := "/home/user"
	pwd := "/work"
	baseHome := "/base/home"
	basePwd := "/base/pwd"

	mfs := &MockFileSystem{
		WD:      pwd,
		HomeDir: home,
	}

	rBase, err := NewExpressionResolverWithFS(&HostContext{
		HomeDir:    baseHome,
		WorkingDir: basePwd,
	}, mfs)
	require.NoError(t, err)

	rFallback, err := NewExpressionResolverWithFS(&HostContext{}, mfs)
	require.NoError(t, err)

	tests := []struct {
		name     string
		resolver *ExpressionResolver
		input    string
		wantErr  bool
	}{
		{"BASE_HOME success", rBase, "{{BASE_HOME}}/file", false},
		{"BASE_HOME traversal", rBase, "{{BASE_HOME}}/../../etc/passwd", true},
		{"BASE_HOME fallback success", rFallback, "{{BASE_HOME}}/file", false},
		{"BASE_HOME fallback traversal", rFallback, "{{BASE_HOME}}/../../etc/passwd", true},

		{"BASE_PWD success", rBase, "{{BASE_PWD}}/file", false},
		{"BASE_PWD traversal", rBase, "{{BASE_PWD}}/../../etc/passwd", true},
		{"BASE_PWD fallback success", rFallback, "{{BASE_PWD}}/file", false},
		{"BASE_PWD fallback traversal", rFallback, "{{BASE_PWD}}/../../etc/passwd", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ResolvePath(tt.input, pwd, tt.resolver)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "path traversal detected")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestUnit_Config_Path_ValidateAnchorBoundaries_Errors(t *testing.T) {
	t.Parallel()

	t.Run("r is nil for magic word", func(t *testing.T) {
		// validateAnchorBoundaries expects r != nil for magic words
		err := validateAnchorBoundaries("{{HOME}}/file", "/home/user/file", nil, &MockFileSystem{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expression resolver required for anchor validation")
	})

	t.Run("r is nil for tilde success", func(t *testing.T) {
		mfs := &MockFileSystem{HomeDir: "/home/user"}
		err := validateAnchorBoundaries("~/file", "/home/user/file", nil, mfs)
		require.NoError(t, err)
	})

	t.Run("r is nil for tilde UserHomeDir error", func(t *testing.T) {
		mfs := &pathExtraMockFS{homeDirErr: assert.AnError}
		err := validateAnchorBoundaries("~/file", "/home/user/file", nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get anchor home directory")
	})

	t.Run("Abs failure for anchorPath", func(t *testing.T) {
		mfs := &MockFileSystem{
			AbsErr:  assert.AnError,
			HomeDir: "/home/user",
		}
		// We need to bypass NewExpressionResolver's Abs check if any,
		// but ResolvePath calls it.
		r, err := NewExpressionResolverWithFS(nil, &MockFileSystem{HomeDir: "/home/user"})
		require.NoError(t, err)

		err = validateAnchorBoundaries("~/file", "/home/user/file", r, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get absolute path for anchor")
	})

	t.Run("Abs failure for resolved path", func(t *testing.T) {
		mfs := &MockFileSystem{
			HomeDir: "/home/user",
			WD:      "/work",
		}
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		// Create a filesystem that fails Abs
		mfsErr := &MockFileSystem{
			HomeDir: "/home/user",
			WD:      "/work",
			AbsErr:  assert.AnError,
		}

		err = validateAnchorBoundaries("~/file", "relative/path", r, mfsErr)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get absolute path for resolved path")
	})

	t.Run("anchorPath is empty", func(t *testing.T) {
		r := &ExpressionResolver{} // Home is empty
		err := validateAnchorBoundaries("~/file", "/file", r, &MockFileSystem{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "anchor path is empty")
	})
}

func TestUnit_Config_Path_ResolveVolumeDevice_Errors(t *testing.T) {
	t.Parallel()
	mfs := &MockFileSystem{WD: "/work", HomeDir: "/home/user"}
	r, err := NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

	t.Run("resolveVolumePath ResolvePath error", func(t *testing.T) {
		// Trigger error via traversal
		_, err := resolveVolumePath("~/../../etc/passwd:/data", "/work", r)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "path traversal detected")
	})

	t.Run("resolveDevicePath ResolvePath error", func(t *testing.T) {
		// Trigger error via traversal
		_, err := resolveDevicePath("~/../../etc/passwd:/dev/passwd", "/work", r)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "path traversal detected")
	})
}
