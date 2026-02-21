package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_Expression_FindDir(t *testing.T) {
	fs := &MockFileSystem{
		Files: map[string][]byte{
			"/project/modules/foo": []byte("bar"),
			"/project/services/production/.cderun.yaml": []byte("runtime: docker"),
		},
		Dirs: map[string]bool{
			"/project":                     true,
			"/project/modules":             true,
			"/project/services":            true,
			"/project/services/production": true,
		},
		WD: "/project/services/production",
	}

	hostCtx := &HostContext{}
	r, err := NewExpressionResolverWithFS(hostCtx, fs)
	require.NoError(t, err)

	t.Run("find_dir existing directory", func(t *testing.T) {
		val := r.resolveString("{{ find_dir:modules }}")
		require.NoError(t, r.Error())
		assert.Equal(t, "../..", val)
	})

	t.Run("find_dir existing file", func(t *testing.T) {
		val := r.resolveString("{{ find_dir:modules/foo }}")
		require.NoError(t, r.Error())
		assert.Equal(t, "../../modules", val)
	})

	t.Run("find_dir not found", func(t *testing.T) {
		r2, _ := NewExpressionResolverWithFS(hostCtx, fs)
		r2.resolveString("{{ find_dir:nonexistent }}")
		require.Error(t, r2.Error())
		assert.Contains(t, r2.Error().Error(), "item not found for find_dir: nonexistent")
	})
}

func TestUnit_Config_Expression_File_Error(t *testing.T) {
	fs := &MockFileSystem{
		Files: map[string][]byte{
			"/project/.go-version": []byte("1.21\n"),
		},
		Dirs: map[string]bool{
			"/project": true,
		},
		WD: "/project",
	}

	hostCtx := &HostContext{}
	r, err := NewExpressionResolverWithFS(hostCtx, fs)
	require.NoError(t, err)

	t.Run("file found", func(t *testing.T) {
		val := r.resolveString("{{ file:.go-version }}")
		require.NoError(t, r.Error())
		assert.Equal(t, "1.21", val)
	})

	t.Run("file not found", func(t *testing.T) {
		r2, _ := NewExpressionResolverWithFS(hostCtx, fs)
		r2.resolveString("{{ file:missing }}")
		require.Error(t, r2.Error())
		assert.Contains(t, r2.Error().Error(), "file not found: missing")
	})
}

func TestUnit_Config_Expression_File_Empty(t *testing.T) {
	fs := &MockFileSystem{
		Files: map[string][]byte{
			"/project/empty.txt": []byte("   "),
			"/project/normal.txt": []byte("content"),
		},
		Dirs: map[string]bool{
			"/project": true,
		},
		WD: "/project",
	}

	hostCtx := &HostContext{}
	r, err := NewExpressionResolverWithFS(hostCtx, fs)
	require.NoError(t, err)

	t.Run("empty file resolves to empty string", func(t *testing.T) {
		val := r.resolveString("{{ file:empty.txt }}")
		require.NoError(t, r.Error())
		assert.Empty(t, val)
	})

	t.Run("normal file still works", func(t *testing.T) {
		val := r.resolveString("{{ file:normal.txt }}")
		require.NoError(t, r.Error())
		assert.Equal(t, "content", val)
	})

	t.Run("cached empty file still works", func(t *testing.T) {
		// Second call should hit cache
		val := r.resolveString("{{ file:empty.txt }}")
		require.NoError(t, r.Error())
		assert.Empty(t, val)
	})
}
