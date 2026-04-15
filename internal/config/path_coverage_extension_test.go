package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_Path_ValidateToolName_Exhaustive(t *testing.T) {
	tests := []struct {
		name    string
		tool    string
		wantErr bool
	}{
		{"Empty name", "", true},
		{"Absolute path", "/usr/bin/node", true},
		{"Current directory", ".", true},
		{"Parent directory", "..", true},
		{"Valid with dot", "node.js", false},
		{"Valid with underscore", "my_tool", false},
		{"Valid with hyphen", "tool-v1", false},
		{"Invalid character space", "node js", true},
		{"Invalid character colon", "node:js", true},
		{"Invalid character slash", "node/js", true},
		{"Invalid character backslash", "node\\js", true},
		{"Valid alphanumeric", "node123", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateToolName(tt.tool)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUnit_Config_Path_ResolvePath_AbsError(t *testing.T) {
	mfs := &MockFileSystem{
		AbsErr: assert.AnError,
		WD:     "/work",
	}
	hostCtx := &HostContext{
		Level: 1,
	}
	r, err := NewExpressionResolverWithFS(hostCtx, mfs)
	require.NoError(t, err)

	_, err = ResolvePath("relative/path", "/work", r)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get absolute path")
}

func TestUnit_Config_Path_ResolvePath_HostContext_Coverage(t *testing.T) {
	t.Parallel()
	mfs := &MockFileSystem{
		WD: "/work",
	}
	// Case where multiple mounts match, should pick the longest target or deepest level
	hostCtx := &HostContext{
		Level: 1,
		Mounts: []MountMapping{
			{Source: "/host/a", Target: "/work", Level: 1},
			{Source: "/host/b", Target: "/work/subdir", Level: 1},
			{Source: "/host/c", Target: "/work/subdir", Level: 2},
		},
	}

	t.Run("pick deepest level for same target length", func(t *testing.T) {
		r, err := NewExpressionResolverWithFS(hostCtx, mfs)
		require.NoError(t, err)
		res, err := ResolvePath("/work/subdir/file", "/work", r)
		require.NoError(t, err)
		assert.Equal(t, "/host/c/file", res)
	})

	t.Run("pick longest target match", func(t *testing.T) {
		hostCtx2 := &HostContext{
			Level: 1,
			Mounts: []MountMapping{
				{Source: "/host/a", Target: "/work", Level: 1},
				{Source: "/host/b", Target: "/work/subdir", Level: 1},
			},
		}
		r, err := NewExpressionResolverWithFS(hostCtx2, mfs)
		require.NoError(t, err)
		res, err := ResolvePath("/work/subdir/file", "/work", r)
		require.NoError(t, err)
		assert.Equal(t, "/host/b/file", res)
	})
}

func TestUnit_Config_Path_SplitHostRemainder_Windows(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input     string
		wantHost  string
		wantRem   string
		wantFound bool
	}{
		{"C:\\path:rem", "C:\\path", "rem", true},
		{"D:/path:rem", "D:/path", "rem", true},
		{"C:\\path", "", "", false},
		{"E:/path", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			host, rem, ok := SplitHostRemainder(tt.input)
			assert.Equal(t, tt.wantFound, ok)
			assert.Equal(t, tt.wantHost, host)
			assert.Equal(t, tt.wantRem, rem)
		})
	}
}

func TestUnit_Config_Path_ParseDeviceConfig_Exhaustive(t *testing.T) {
	t.Parallel()
	t.Run("valid with perms", func(t *testing.T) {
		dc, ok := ParseDeviceConfig("/dev/host:/dev/cont:rw")
		assert.True(t, ok)
		assert.Equal(t, "/dev/host", dc.Source.Raw)
		assert.Equal(t, "/dev/cont", dc.Destination.Raw)
		assert.Equal(t, "rw", dc.Permissions)
	})

	t.Run("valid without perms", func(t *testing.T) {
		dc, ok := ParseDeviceConfig("/dev/host:/dev/cont")
		assert.True(t, ok)
		assert.Equal(t, "/dev/host", dc.Source.Raw)
		assert.Equal(t, "/dev/cont", dc.Destination.Raw)
		assert.Equal(t, "rwm", dc.Permissions)
	})

	t.Run("no colon defaults to same path", func(t *testing.T) {
		dc, ok := ParseDeviceConfig("/dev/host")
		assert.True(t, ok)
		assert.Equal(t, "/dev/host", dc.Source.Raw)
		assert.Equal(t, "/dev/host", dc.Destination.Raw)
	})

	t.Run("empty segments", func(t *testing.T) {
		_, ok := ParseDeviceConfig(":/dev/cont")
		assert.False(t, ok)
		_, ok = ParseDeviceConfig("/dev/host:")
		assert.False(t, ok)
	})
}

func TestUnit_Config_Path_ValidateAnchorBoundaries_Coverage(t *testing.T) {
	t.Run("r is nil with tilde", func(t *testing.T) {
		mfs := &MockFileSystem{HomeDir: "/home/user"}
		err := validateAnchorBoundaries("~", "/home/user/file", "/work", nil, mfs)
		require.NoError(t, err)

		err = validateAnchorBoundaries("~", "/etc/passwd", "/work", nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "path traversal detected")
	})

	t.Run("r is nil with non-tilde anchor", func(t *testing.T) {
		err := validateAnchorBoundaries("{{HOME}}", "/home/user/file", "/work", nil, &MockFileSystem{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expression resolver required")
	})

	t.Run("BASE_HOME and BASE_PWD", func(t *testing.T) {
		hostCtx := &HostContext{
			HomeDir:    "/base/home",
			WorkingDir: "/base/pwd",
		}
		r, err := NewExpressionResolverWithFS(hostCtx, &MockFileSystem{HomeDir: "/home", WD: "/pwd"})
		require.NoError(t, err)

		err = validateAnchorBoundaries("{{BASE_HOME}}/file", "/base/home/file", "/work", r, r.fs)
		require.NoError(t, err)

		err = validateAnchorBoundaries("{{BASE_PWD}}/file", "/base/pwd/file", "/work", r, r.fs)
		require.NoError(t, err)
	})

	t.Run("BASE_HOME and BASE_PWD fallback", func(t *testing.T) {
		hostCtx := &HostContext{}
		r, err := NewExpressionResolverWithFS(hostCtx, &MockFileSystem{HomeDir: "/home", WD: "/pwd"})
		require.NoError(t, err)

		err = validateAnchorBoundaries("{{BASE_HOME}}/file", "/home/file", "/work", r, r.fs)
		require.NoError(t, err)

		err = validateAnchorBoundaries("{{BASE_PWD}}/file", "/pwd/file", "/work", r, r.fs)
		require.NoError(t, err)
	})

	t.Run("fs.Abs failure for anchorPath", func(t *testing.T) {
		mfs := &MockFileSystem{
			HomeDir: "home", // relative path to trigger fs.Abs
		}
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		// Make fs.Abs fail
		mfs.AbsErr = assert.AnError

		err = validateAnchorBoundaries("~", "/home/file", "/work", r, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get absolute path for anchor")
	})

	t.Run("fs.Abs failure for resolved path", func(t *testing.T) {
		mfs := &MockFileSystem{
			HomeDir: "/home",
		}
		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		// Make fs.Abs fail for a specific path
		mfs.AbsErr = assert.AnError
		// validateAnchorBoundaries calls fs.Abs(resolved) if resolved is not absolute.
		err = validateAnchorBoundaries("~", "relative/path", "/work", r, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get absolute path for resolved path")
	})

	t.Run("anchorPath is empty", func(t *testing.T) {
		r := &ExpressionResolver{Home: ""}
		err := validateAnchorBoundaries("~", "/file", "/work", r, &MockFileSystem{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "anchor path is empty")
	})

	t.Run("UserHomeDir failure when r is nil", func(t *testing.T) {
		mfs := &MockFileSystem{
			HomeDirErr: assert.AnError,
		}
		err := validateAnchorBoundaries("~", "/file", "/work", nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get anchor home directory")
	})
}

func TestUnit_Config_Path_ResolveVolume_ResolveDevice_Errors(t *testing.T) {
	t.Parallel()
	mfs := &MockFileSystem{HomeDirErr: assert.AnError}
	// Note: NewExpressionResolverWithFS returns error if fs.UserHomeDir fails
	_, err := NewExpressionResolverWithFS(nil, mfs)
	require.Error(t, err)

	// Since we can't get a resolver with erroring FS easily for ResolvePath,
	// let's use a resolver with valid FS but make the resolution fail if possible.
	r, err := NewExpressionResolver(nil)
	require.NoError(t, err)
	require.NotNil(t, r)

	t.Run("resolveVolumePath ResolvePath error", func(t *testing.T) {
		// ResolvePath errors if anchor boundary is escaped
		_, err := resolveVolumePath("~/../../etc/passwd:/data", "/work", r)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "path traversal detected")
	})

	t.Run("resolveDevicePath ResolvePath error", func(t *testing.T) {
		_, err := resolveDevicePath("~/../../etc/passwd:/dev/a", "/work", r)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "path traversal detected")
	})
}
