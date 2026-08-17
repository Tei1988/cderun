package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecurityEnhancement_ValidateMountType(t *testing.T) {
	t.Parallel()

	assert.NoError(t, ValidateMountType(""))
	assert.NoError(t, ValidateMountType("bind"))
	assert.NoError(t, ValidateMountType("volume"))
	assert.NoError(t, ValidateMountType("tmpfs"))

	err := ValidateMountType("nfs")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported mount type")

	err = ValidateMountType("overlay")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported mount type")
}

func TestSecurityEnhancement_MountTypeInResolver(t *testing.T) {
	t.Parallel()

	t.Run("invalid mount type rejected", func(t *testing.T) {
		mfs := &MockFileSystem{
			Files: map[string][]byte{
				"/workspace/.tools.yaml": []byte(`
testtool:
  image: alpine
  mounts:
    - type: invalid_type
      source: /workspace
      target: /app
`),
			},
			Env: map[string]string{},
			WD:  "/workspace",
		}

		loader := NewConfigLoaderWithFS(mfs)
		tools, _, err := loader.LoadToolsConfigFromPath("/workspace/.tools.yaml")
		require.NoError(t, err)

		_, err = ResolveWithFS("testtool", &CLIOptions{}, tools, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported mount type")
	})

	t.Run("empty mount type normalizes to bind in MountConfig.Resolve and Resolver", func(t *testing.T) {
		mfs := &MockFileSystem{
			Files: map[string][]byte{
				"/workspace/.tools.yaml": []byte(`
testtool:
  image: alpine
  mounts:
    - source: /workspace
      target: /app
`),
			},
			Env: map[string]string{},
			WD:  "/workspace",
		}

		loader := NewConfigLoaderWithFS(mfs)
		tools, _, err := loader.LoadToolsConfigFromPath("/workspace/.tools.yaml")
		require.NoError(t, err)

		res, err := ResolveWithFS("testtool", &CLIOptions{}, tools, nil, mfs)
		require.NoError(t, err)
		require.Len(t, res.Mounts, 1)
		assert.Equal(t, "bind", res.Mounts[0].Type)

		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		mc := MountConfig{
			Source: ConfigPath{Raw: "/workspace", BaseDir: "/workspace"},
			Target: ConfigPath{Raw: "/app"},
		}
		resolvedMount, err := mc.Resolve(r)
		require.NoError(t, err)
		assert.Equal(t, "bind", resolvedMount.Type)
	})
}

func TestSecurityEnhancement_DirectiveTraversalRejections(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		Files: map[string][]byte{},
		Env:   map[string]string{},
		WD:    "/workspace",
	}
	r, err := NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

	testCases := []struct {
		name      string
		directive string
	}{
		{"dot file", "{{file:.}}"},
		{"dotdot file", "{{file:..}}"},
		{"traversal file", "{{file:../etc/passwd}}"},
		{"nested slash file", "{{file:sub/config.txt}}"},
		{"dot find_dir", "{{find_dir:.}}"},
		{"dotdot find_dir", "{{find_dir:..}}"},
		{"traversal find_dir", "{{find_dir:../dir}}"},
		{"nested slash find_dir", "{{find_dir:sub/dir}}"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := r.ResolveString(tc.directive)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "only a single file")
		})
	}
}

func TestSecurityEnhancement_ResolvedEnvControlCharValidation(t *testing.T) {
	t.Parallel()

	t.Run("null byte injection rejected", func(t *testing.T) {
		mfs := &MockFileSystem{
			Files: map[string][]byte{},
			Env: map[string]string{
				"BAD_VAR": "val\x00inject",
			},
			WD: "/workspace",
		}

		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		_, err = r.ResolveString("{{env:BAD_VAR}}")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "security validation failed for resolved environment variable value")
	})

	t.Run("C1 control character U+0085 rejected", func(t *testing.T) {
		mfs := &MockFileSystem{
			Files: map[string][]byte{},
			Env: map[string]string{
				"C1_VAR": "val\u0085inject",
			},
			WD: "/workspace",
		}

		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		_, err = r.ResolveString("{{env:C1_VAR}}")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "security validation failed for resolved environment variable value")
	})

	t.Run("allowlisted LF CR TAB preserved and accepted", func(t *testing.T) {
		mfs := &MockFileSystem{
			Files: map[string][]byte{},
			Env: map[string]string{
				"VALID_VAR": "line1\nline2\r\ttabbed",
			},
			WD: "/workspace",
		}

		r, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		val, err := r.ResolveString("{{env:VALID_VAR}}")
		require.NoError(t, err)
		assert.Equal(t, "line1\nline2\r\ttabbed", val)
	})
}

func TestSecurityEnhancement_CLIImageControlCharValidation(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		Files: map[string][]byte{
			"/workspace/.tools.yaml": []byte(`
testtool:
  image: alpine
`),
		},
		Env: map[string]string{},
		WD:  "/workspace",
	}

	loader := NewConfigLoaderWithFS(mfs)
	tools, _, err := loader.LoadToolsConfigFromPath("/workspace/.tools.yaml")
	require.NoError(t, err)

	badImage := "alpine\x00inject"
	cli := &CLIOptions{
		CderunImage: &badImage,
	}

	_, err = ResolveWithFS("testtool", cli, tools, nil, mfs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "security validation failed for image")
}

func TestSecurityEnhancement_AnchorPathValidation(t *testing.T) {
	t.Parallel()

	mfs := &MockFileSystem{
		Files: map[string][]byte{},
		Env: map[string]string{
			"SAFE_KEY": "clean_val",
		},
		WD: "/workspace",
	}

	r, err := NewExpressionResolverWithFS(nil, mfs)
	require.NoError(t, err)

	// Inject control character directly into resolver's Home directory field to exercise validateAnchorBoundaries anchorPath validation
	r.Home = "/home/user\x00inject"

	_, err = ResolvePath("~/file.txt", "/workspace", r)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid character in path or configuration")
}
