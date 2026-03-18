package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_BDD_ExpressionResolution(t *testing.T) {
	t.Parallel()

	// Given: A file system with environment variables and files
	// When: An expression resolution is requested
	// Then: The expressions should be resolved correctly according to the defined logic

	mfs := &MockFileSystem{
		WD:      "/project",
		HomeDir: "/home/user",
		Env: map[string]string{
			"VAR1": "value1",
		},
		Files: map[string][]byte{
			"/project/version.txt": []byte("1.25.0"),
		},
		Dirs: map[string]bool{"/project": true},
	}

	t.Run("PWD and HOME magic words", func(t *testing.T) {
		resolver, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		assert.Equal(t, "/project", resolver.Resolve("{{PWD}}"))
		assert.Equal(t, "/home/user", resolver.Resolve("{{HOME}}"))
	})

	t.Run("file directive with valid path", func(t *testing.T) {
		resolver, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		assert.Equal(t, "v1.25.0", resolver.Resolve("v{{file:version.txt}}"))
		require.NoError(t, resolver.Error())
	})

	t.Run("path traversal protection in file directive", func(t *testing.T) {
		resolver, err := NewExpressionResolverWithFS(nil, mfs)
		require.NoError(t, err)

		_ = resolver.Resolve("{{file:../etc/passwd}}")
		require.Error(t, resolver.Error())
		assert.Contains(t, resolver.Error().Error(), "parent directory references are not allowed")
	})
}
