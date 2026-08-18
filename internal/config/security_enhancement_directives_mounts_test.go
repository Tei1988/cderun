package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_SecurityEnhancements_ValidateMountType(t *testing.T) {
	t.Parallel()

	validTypes := []string{"", "bind", "volume", "tmpfs"}
	for _, vt := range validTypes {
		t.Run("valid_mount_type_"+vt, func(t *testing.T) {
			err := ValidateMountType(vt)
			assert.NoError(t, err)
		})
	}

	invalidTypes := []string{"overlay", "devtmpfs", "nfs", "proc", "sysfs", "invalid_type"}
	for _, it := range invalidTypes {
		t.Run("invalid_mount_type_"+it, func(t *testing.T) {
			err := ValidateMountType(it)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "unsupported mount type")
		})
	}
}

func TestUnit_Config_SecurityEnhancements_MountConfigResolveTypeValidation(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{WD: "/workspace", HomeDir: "/workspace/home"}
	r, err := NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

	t.Run("valid mount type resolves successfully", func(t *testing.T) {
		mc := MountConfig{
			Type:   "tmpfs",
			Target: ConfigPath{Raw: "/tmp/data"},
		}
		m, err := mc.Resolve(r)
		require.NoError(t, err)
		assert.Equal(t, "tmpfs", m.Type)
		assert.Equal(t, "/tmp/data", m.Target)
	})

	t.Run("empty mount type resolves as bind", func(t *testing.T) {
		mc := MountConfig{
			Type:   "",
			Source: ConfigPath{Raw: "/host/src"},
			Target: ConfigPath{Raw: "/app/dst"},
		}
		m, err := mc.Resolve(r)
		require.NoError(t, err)
		assert.Equal(t, "bind", m.Type)
		assert.Equal(t, "/host/src", m.Source)
		assert.Equal(t, "/app/dst", m.Target)
	})

	t.Run("invalid mount type rejected during resolve", func(t *testing.T) {
		mc := MountConfig{
			Type:   "overlay",
			Target: ConfigPath{Raw: "/tmp/data"},
		}
		_, err := mc.Resolve(r)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported mount type")
	})
}

func TestUnit_Config_SecurityEnhancements_DirectivesTraversalHardening(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{WD: "/workspace", HomeDir: "/workspace/home"}
	r, err := NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

	traversalInputs := []string{".", "..", "../secret.txt", "..\\secret.txt"}

	for _, input := range traversalInputs {
		t.Run("resolveFile_rejects_"+input, func(t *testing.T) {
			_, err := r.resolveFile(input)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "only a single file name is allowed in file directive")
		})

		t.Run("resolveFindDir_rejects_"+input, func(t *testing.T) {
			_, err := r.resolveFindDir(input)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "only a single file or directory name is allowed in find_dir directive")
		})
	}
}

func TestUnit_Config_SecurityEnhancements_EnvValuesControlCharacterValidation(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{WD: "/workspace", HomeDir: "/workspace/home"}

	t.Run("env value with C0 control character rejected", func(t *testing.T) {
		cli := &CLIOptions{
			Image: ptr("alpine"),
			Env:   []string{"FOO=bar\x01baz"},
		}
		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid control character")
	})

	t.Run("env value with C1 control character rejected", func(t *testing.T) {
		cli := &CLIOptions{
			Image: ptr("alpine"),
			Env:   []string{"FOO=bar\u0085baz"},
		}
		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid control character")
	})

	t.Run("env value with null byte rejected", func(t *testing.T) {
		cli := &CLIOptions{
			Image: ptr("alpine"),
			Env:   []string{"FOO=bar\x00baz"},
		}
		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "null byte injection detected")
	})

	t.Run("env value with multiline text allowed", func(t *testing.T) {
		cli := &CLIOptions{
			Image: ptr("alpine"),
			Env:   []string{"CERT=-----BEGIN CERTIFICATE-----\nMIIC...\r\n-----END CERTIFICATE-----"},
		}
		res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)
		require.Len(t, res.Env, 1)
		assert.Equal(t, "CERT=-----BEGIN CERTIFICATE-----\nMIIC...\r\n-----END CERTIFICATE-----", res.Env[0])
	})
}

func TestUnit_Config_SecurityEnhancements_ImageControlCharacterValidation(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{WD: "/workspace", HomeDir: "/workspace/home"}

	t.Run("image with control character rejected", func(t *testing.T) {
		cli := &CLIOptions{
			Image: ptr("alpine\x07:latest"),
		}
		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "security validation failed for image")
	})
}

func TestUnit_Config_SecurityEnhancements_AnchorBoundaryControlCharacterValidation(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		WD:      "/workspace",
		HomeDir: "/workspace/home",
		Env: map[string]string{
			"HOME_WITH_CTRL": "/workspace/user\x02",
		},
	}

	r, err := NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

	_, err = ResolvePath("{{env:HOME_WITH_CTRL}}/sub", "/workspace", r)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid character in resolved anchor path")
}
