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
