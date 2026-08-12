package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_RestartPolicy_Resolution(t *testing.T) {
	mfs := &MockFileSystem{
		WD: "/workspace",
		Env: map[string]string{
			"CDERUN_RESTART": "always",
		},
	}

	cli := &CLIOptions{
		Image:  ptr("alpine"),
		Remove: ptr(false),
	}

	res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
	require.NoError(t, err)
	assert.Equal(t, "always", res.Restart)
}

func TestUnit_Config_RestartPolicy_Validation(t *testing.T) {
	mfs := &MockFileSystem{
		WD: "/workspace",
	}

	t.Run("invalid restart policy is rejected", func(t *testing.T) {
		cli := &CLIOptions{
			Image:   ptr("alpine"),
			Restart: ptr("invalid-policy"),
		}
		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported restart policy")
	})

	t.Run("suffix on non-on-failure policy is rejected", func(t *testing.T) {
		cli := &CLIOptions{
			Image:   ptr("alpine"),
			Restart: ptr("always:3"),
		}
		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "does not support retry suffix")
	})

	t.Run("multiple suffixes on on-failure policy are rejected", func(t *testing.T) {
		cli := &CLIOptions{
			Image:   ptr("alpine"),
			Restart: ptr("on-failure:3:4"),
		}
		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "supports at most one retry suffix")
	})

	t.Run("malformed suffix on on-failure policy is rejected", func(t *testing.T) {
		cli := &CLIOptions{
			Image:   ptr("alpine"),
			Restart: ptr("on-failure:3abc"),
		}
		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "retry suffix must be a non-negative integer")
	})

	t.Run("negative suffix on on-failure policy is rejected", func(t *testing.T) {
		cli := &CLIOptions{
			Image:   ptr("alpine"),
			Restart: ptr("on-failure:-3"),
		}
		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "retry suffix must be a non-negative integer")
	})

	t.Run("restart policy with remove enabled is rejected", func(t *testing.T) {
		cli := &CLIOptions{
			Image:   ptr("alpine"),
			Restart: ptr("always"),
			Remove:  ptr(true),
		}
		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "the --restart policy cannot be used when --remove is enabled")
	})
}
