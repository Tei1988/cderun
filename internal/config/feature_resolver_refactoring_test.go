package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_ResolverRefactoring_NeedsResolver(t *testing.T) {
	t.Run("empty slice returns false", func(t *testing.T) {
		assert.False(t, needsResolver(nil, nil))
		assert.False(t, needsResolver([]string{}, nil))
	})

	t.Run("host context level > 0 returns true even without expression markers", func(t *testing.T) {
		global := &CDERunConfig{
			HostContext: &HostContext{Level: 1},
		}
		assert.True(t, needsResolver([]string{"noflags"}, global))
	})

	t.Run("template or tilde markers return true", func(t *testing.T) {
		assert.True(t, needsResolver([]string{"{{HOME}}"}, nil))
		assert.True(t, needsResolver([]string{"~/workspace"}, nil))
		assert.False(t, needsResolver([]string{"plain_value"}, nil))
	})
}

func TestUnit_Config_ResolverRefactoring_GetResolverIfNeeded(t *testing.T) {
	rv := &resolver{
		fs: &MockFileSystem{
			Files: map[string][]byte{},
			WD:    "/test",
		},
	}

	t.Run("no resolver needed returns nil", func(t *testing.T) {
		r, err := rv.getResolverIfNeeded([]string{"plain"})
		require.NoError(t, err)
		assert.Nil(t, r)
	})

	t.Run("resolver needed returns non-nil resolver", func(t *testing.T) {
		r, err := rv.getResolverIfNeeded([]string{"{{PWD}}"})
		require.NoError(t, err)
		require.NotNil(t, r)
		assert.Equal(t, "/test", r.Pwd)
	})
}

func TestUnit_Config_ResolverRefactoring_ResolveStringOptionWithExpressions(t *testing.T) {
	mfs := &MockFileSystem{
		WD: "/workspace",
		Env: map[string]string{
			"FOO": "bar",
		},
	}
	rv := &resolver{
		fs: mfs,
	}

	def := OptionDef[string]{
		EnvKey: "FOO",
	}

	t.Run("plain string resolution without expressions", func(t *testing.T) {
		val, err := rv.resolveStringOptionWithExpressions(def, true, "custom", false, "")
		require.NoError(t, err)
		assert.Equal(t, "custom", val)
	})

	t.Run("template expression resolution", func(t *testing.T) {
		val, err := rv.resolveStringOptionWithExpressions(def, true, "{{PWD}}/out", false, "")
		require.NoError(t, err)
		assert.Equal(t, "/workspace/out", val)
	})
}

func TestUnit_Config_ResolverRefactoring_ResolveStringSliceOptionResolver(t *testing.T) {
	rv := &resolver{
		fs: &MockFileSystem{
			WD: "/workspace",
		},
	}

	t.Run("no expression markers returns nil", func(t *testing.T) {
		r, err := rv.resolveStringSliceOptionResolver([]string{"a", "b"})
		require.NoError(t, err)
		assert.Nil(t, r)
	})

	t.Run("expression marker returns resolver", func(t *testing.T) {
		r, err := rv.resolveStringSliceOptionResolver([]string{"a", "{{PWD}}"})
		require.NoError(t, err)
		require.NotNil(t, r)
		assert.Equal(t, "/workspace", r.Pwd)
	})
}

func TestUnit_Config_ResolverRefactoring_MergeEnvSmall(t *testing.T) {
	base := []string{"A=1", "B=2"}
	p2 := []string{"B=20", "C=3"}
	p1 := []string{"C=30", "D=4"}

	res := mergeEnv(base, p2, p1)
	assert.Equal(t, []string{"A=1", "B=20", "C=30", "D=4"}, res)
}
