package config

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_Ulimit_ParsingAndResolution(t *testing.T) {
	mfs := &MockFileSystem{
		WD: "/app",
	}

	t.Run("resolve ulimits from CLI (P2)", func(t *testing.T) {
		res, err := resolveUlimits(nil, []string{"nofile=1024:2048", "nproc=4096"}, "", nil, nil, mfs)
		require.NoError(t, err)
		assert.Len(t, res, 2)
		assert.Equal(t, "nofile", res[0].Name)
		assert.Equal(t, int64(2048), res[0].Hard)
		assert.Equal(t, int64(1024), res[0].Soft)
		assert.Equal(t, "nproc", res[1].Name)
		assert.Equal(t, int64(4096), res[1].Hard)
		assert.Equal(t, int64(4096), res[1].Soft)
	})

	t.Run("resolve ulimits from CLI override (P1 overrides P2)", func(t *testing.T) {
		res, err := resolveUlimits([]string{"nofile=512:1024"}, []string{"nofile=1024:2048"}, "", nil, nil, mfs)
		require.NoError(t, err)
		assert.Len(t, res, 1)
		assert.Equal(t, "nofile", res[0].Name)
		assert.Equal(t, int64(1024), res[0].Hard)
		assert.Equal(t, int64(512), res[0].Soft)
	})

	t.Run("resolve ulimits from environment variable", func(t *testing.T) {
		mfs.Env = map[string]string{
			"CDERUN_ULIMIT": "nofile=1024:2048,nproc=4096",
		}
		defer func() { mfs.Env = nil }()

		res, err := resolveUlimits(nil, nil, "", nil, nil, mfs)
		require.NoError(t, err)
		assert.Len(t, res, 2)
		assert.Equal(t, "nofile", res[0].Name)
		assert.Equal(t, "nproc", res[1].Name)
	})

	t.Run("resolve ulimits from tool configuration", func(t *testing.T) {
		tools := ToolsConfig{
			"node": ToolConfig{
				Ulimits: []string{"nofile=2048:4096"},
			},
		}

		res, err := resolveUlimits(nil, nil, "node", tools, nil, mfs)
		require.NoError(t, err)
		assert.Len(t, res, 1)
		assert.Equal(t, "nofile", res[0].Name)
		assert.Equal(t, int64(4096), res[0].Hard)
		assert.Equal(t, int64(2048), res[0].Soft)
	})

	t.Run("resolve ulimits from global defaults", func(t *testing.T) {
		global := &CDERunConfig{
			Defaults: ConfigDefaults{
				Ulimits: []string{"nofile=4096:8192"},
			},
		}

		res, err := resolveUlimits(nil, nil, "sh", nil, global, mfs)
		require.NoError(t, err)
		assert.Len(t, res, 1)
		assert.Equal(t, "nofile", res[0].Name)
		assert.Equal(t, int64(8192), res[0].Hard)
		assert.Equal(t, int64(4096), res[0].Soft)
	})

	t.Run("resolve ulimits with invalid value errors out", func(t *testing.T) {
		_, err := resolveUlimits(nil, []string{"invalid-format"}, "", nil, nil, mfs)
		require.Error(t, err)
		var cfgErr *InvalidConfigError
		require.True(t, errors.As(err, &cfgErr))
		assert.Equal(t, "ulimit", cfgErr.Field)
		assert.Equal(t, "invalid-format", cfgErr.Value)
	})

	t.Run("resolve ulimits with limit values below -1 errors out", func(t *testing.T) {
		_, err := resolveUlimits(nil, []string{"nofile=-2:1024"}, "", nil, nil, mfs)
		require.Error(t, err)
		var cfgErr *InvalidConfigError
		require.True(t, errors.As(err, &cfgErr))
		assert.Equal(t, "ulimit", cfgErr.Field)
		assert.Equal(t, "nofile=-2:1024", cfgErr.Value)
		assert.Contains(t, cfgErr.Err.Error(), "limit values must be at least -1")
	})
}
