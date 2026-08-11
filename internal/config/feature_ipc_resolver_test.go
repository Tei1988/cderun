package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_IPC_Resolution(t *testing.T) {
	mfs := &MockFileSystem{
		WD: "/workspace",
		Env: map[string]string{
			"CDERUN_IPC": "host",
		},
	}

	cli := &CLIOptions{
		Image: ptr("alpine"),
	}

	res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
	require.NoError(t, err)
	assert.Equal(t, "host", res.IPC)
}

func TestUnit_Config_IPC_Validation(t *testing.T) {
	mfs := &MockFileSystem{
		WD: "/workspace",
	}

	t.Run("empty container IPC reference is rejected", func(t *testing.T) {
		cli := &CLIOptions{
			Image: ptr("alpine"),
			IPC:   ptr("container:"),
		}
		_, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported ipc namespace")
	})

	t.Run("valid container IPC reference is accepted", func(t *testing.T) {
		cli := &CLIOptions{
			Image: ptr("alpine"),
			IPC:   ptr("container:my-container-id"),
		}
		res, err := ResolveWithFS("sh", cli, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, "container:my-container-id", res.IPC)
	})
}
