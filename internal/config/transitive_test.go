package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Config_TransitiveOptions_Exhaustive(t *testing.T) {
	t.Parallel()
	fs := &MockFileSystem{WD: "/work"}

	t.Run("mount-tools enables mount-cderun and mount-socket", func(t *testing.T) {
		cli := &CLIOptions{
			Image:      ptr("alpine"),
			MountTools: ptr("node"),
		}
		res, err := ResolveWithFS("sh", cli, nil, nil, fs)
		require.NoError(t, err)
		assert.Equal(t, []string{"node"}, res.MountTools)
		assert.True(t, res.MountCderun)
		assert.True(t, res.MountSocket)
	})

	t.Run("mount-all-tools enables mount-cderun and mount-socket", func(t *testing.T) {
		cli := &CLIOptions{
			Image:         ptr("alpine"),
			MountAllTools: ptr(true),
		}
		res, err := ResolveWithFS("sh", cli, nil, nil, fs)
		require.NoError(t, err)
		assert.True(t, res.MountAllTools)
		assert.True(t, res.MountCderun)
		assert.True(t, res.MountSocket)
	})

	t.Run("mount-cderun enables mount-socket but not tools", func(t *testing.T) {
		cli := &CLIOptions{
			Image:       ptr("alpine"),
			MountCderun: ptr(true),
		}
		res, err := ResolveWithFS("sh", cli, nil, nil, fs)
		require.NoError(t, err)
		assert.Empty(t, res.MountTools)
		assert.False(t, res.MountAllTools)
		assert.True(t, res.MountCderun)
		assert.True(t, res.MountSocket)
	})

	t.Run("explicit mount-cderun=false overrides mount-tools trigger", func(t *testing.T) {
		cli := &CLIOptions{
			Image:       ptr("alpine"),
			MountTools:  ptr("node"),
			MountCderun: ptr(false),
		}
		res, err := ResolveWithFS("sh", cli, nil, nil, fs)
		require.NoError(t, err)
		assert.Equal(t, []string{"node"}, res.MountTools)
		assert.False(t, res.MountCderun)
		// socket is transitive from cderun, so if cderun is explicitly false,
		// and socket is not specified, it should also be false.
		assert.False(t, res.MountSocket)
	})

	t.Run("explicit mount-socket=false overrides mount-cderun trigger", func(t *testing.T) {
		cli := &CLIOptions{
			Image:       ptr("alpine"),
			MountCderun: ptr(true),
			MountSocket: ptr(false),
		}
		res, err := ResolveWithFS("sh", cli, nil, nil, fs)
		require.NoError(t, err)
		assert.True(t, res.MountCderun)
		assert.False(t, res.MountSocket)
	})

	t.Run("transitive triggers from Env", func(t *testing.T) {
		mfs := &MockFileSystem{
			Env: map[string]string{"CDERUN_MOUNT_TOOLS": "node"},
			WD:  "/work",
		}
		res, err := ResolveWithFS("sh", &CLIOptions{Image: ptr("alpine")}, nil, nil, mfs)
		require.NoError(t, err)
		assert.Equal(t, []string{"node"}, res.MountTools)
		assert.True(t, res.MountCderun)
		assert.True(t, res.MountSocket)
	})

	t.Run("transitive triggers from Global Config", func(t *testing.T) {
		global := &CDERunConfig{
			Defaults: ConfigDefaults{
				MountTools: []string{"node"},
			},
		}
		res, err := ResolveWithFS("sh", &CLIOptions{Image: ptr("alpine")}, nil, global, fs)
		require.NoError(t, err)
		assert.Equal(t, []string{"node"}, res.MountTools)
		assert.True(t, res.MountCderun)
		assert.True(t, res.MountSocket)
	})

	t.Run("transitive triggers from Tool Config", func(t *testing.T) {
		tools := ToolsConfig{
			"sh": ToolConfig{
				MountTools: []string{"node"},
			},
		}
		res, err := ResolveWithFS("sh", &CLIOptions{Image: ptr("alpine")}, tools, nil, fs)
		require.NoError(t, err)
		assert.Equal(t, []string{"node"}, res.MountTools)
		assert.True(t, res.MountCderun)
		assert.True(t, res.MountSocket)
	})

	t.Run("complex override: P4 Tool enables mount-tools, but P2 CLI disables mount-cderun", func(t *testing.T) {
		tools := ToolsConfig{
			"sh": ToolConfig{
				MountTools: []string{"node"},
			},
		}
		cli := &CLIOptions{
			Image:       ptr("alpine"),
			MountCderun: ptr(false),
		}
		res, err := ResolveWithFS("sh", cli, tools, nil, fs)
		require.NoError(t, err)
		assert.Equal(t, []string{"node"}, res.MountTools)
		assert.False(t, res.MountCderun)
		// MountSocket should also be false because it inherits from MountCderun if not specified
		assert.False(t, res.MountSocket)
	})

	t.Run("complex override: P3 Env enables mount-all-tools, but P5 Global disables mount-socket", func(t *testing.T) {
		mfs := &MockFileSystem{
			Env: map[string]string{"CDERUN_MOUNT_ALL_TOOLS": "true"},
			WD:  "/work",
		}
		global := &CDERunConfig{
			Defaults: ConfigDefaults{
				MountSocket: ptr(false),
			},
		}
		res, err := ResolveWithFS("sh", &CLIOptions{Image: ptr("alpine")}, nil, global, mfs)
		require.NoError(t, err)
		assert.True(t, res.MountAllTools)
		assert.True(t, res.MountCderun)
		// Global Config (P5) should override the transitive trigger from mount-all-tools
		assert.False(t, res.MountSocket)
	})
}
