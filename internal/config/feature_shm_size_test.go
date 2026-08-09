package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func shmSizePtr[T any](v T) *T {
	return &v
}

func TestUnit_Config_ShmSize_Resolution(t *testing.T) {
	t.Parallel()

	t.Run("resolve from CLI override", func(t *testing.T) {
		cli := &CLIOptions{
			Image:   shmSizePtr("alpine"),
			ShmSize: shmSizePtr("256m"),
		}
		res, err := ResolveWithFS("test", cli, nil, nil, &MockFileSystem{})
		require.NoError(t, err)
		assert.Equal(t, int64(268435456), res.ShmSize)
	})

	t.Run("resolve from CLI override with cderun- prefix", func(t *testing.T) {
		cli := &CLIOptions{
			Image:         shmSizePtr("alpine"),
			CderunShmSize: shmSizePtr("1g"),
		}
		res, err := ResolveWithFS("test", cli, nil, nil, &MockFileSystem{})
		require.NoError(t, err)
		assert.Equal(t, int64(1073741824), res.ShmSize)
	})

	t.Run("resolve from Env variable", func(t *testing.T) {
		cli := &CLIOptions{
			Image: shmSizePtr("alpine"),
		}
		mfs := MockFileSystem{
			Env: map[string]string{"CDERUN_SHM_SIZE": "512m"},
		}
		res, err := ResolveWithFS("test", cli, nil, nil, &mfs)
		require.NoError(t, err)
		assert.Equal(t, int64(536870912), res.ShmSize)
	})

	t.Run("resolve from Tool config fallback", func(t *testing.T) {
		tools := ToolsConfig{
			"test": ToolConfig{
				Image:   "alpine",
				ShmSize: "64m",
			},
		}
		res, err := ResolveWithFS("test", &CLIOptions{}, tools, nil, &MockFileSystem{})
		require.NoError(t, err)
		assert.Equal(t, int64(67108864), res.ShmSize)
	})

	t.Run("resolve from Global config fallback", func(t *testing.T) {
		cli := &CLIOptions{
			Image: shmSizePtr("alpine"),
		}
		global := &CDERunConfig{
			Defaults: ConfigDefaults{
				ShmSize: "128m",
			},
		}
		res, err := ResolveWithFS("test", cli, nil, global, &MockFileSystem{})
		require.NoError(t, err)
		assert.Equal(t, int64(134217728), res.ShmSize)
	})

	t.Run("invalid format returns error", func(t *testing.T) {
		cli := &CLIOptions{
			Image:   shmSizePtr("alpine"),
			ShmSize: shmSizePtr("invalid-size"),
		}
		_, err := ResolveWithFS("test", cli, nil, nil, &MockFileSystem{})
		require.Error(t, err)
		var invalidErr *InvalidConfigError
		require.ErrorAs(t, err, &invalidErr)
		assert.Equal(t, "shm-size", invalidErr.Field)
		assert.Equal(t, "invalid-size", invalidErr.Value)
	})
}
