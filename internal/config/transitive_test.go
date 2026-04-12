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
			Image:         "alpine",
			ImageSet:      true,
			MountTools:    "node",
			MountToolsSet: true,
		}
		res, err := ResolveWithFS("sh", cli, nil, nil, fs)
		require.NoError(t, err)
		assert.Equal(t, []string{"node"}, res.MountTools)
		assert.True(t, res.MountCderun)
		assert.True(t, res.MountSocket)
	})

	t.Run("mount-all-tools enables mount-cderun and mount-socket", func(t *testing.T) {
		cli := &CLIOptions{
			Image:            "alpine",
			ImageSet:         true,
			MountAllTools:    true,
			MountAllToolsSet: true,
		}
		res, err := ResolveWithFS("sh", cli, nil, nil, fs)
		require.NoError(t, err)
		assert.True(t, res.MountAllTools)
		assert.True(t, res.MountCderun)
		assert.True(t, res.MountSocket)
	})

	t.Run("mount-cderun enables mount-socket but not tools", func(t *testing.T) {
		cli := &CLIOptions{
			Image:          "alpine",
			ImageSet:       true,
			MountCderun:    true,
			MountCderunSet: true,
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
			Image:          "alpine",
			ImageSet:       true,
			MountTools:     "node",
			MountToolsSet:  true,
			MountCderun:    false,
			MountCderunSet: true,
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
			Image:          "alpine",
			ImageSet:       true,
			MountCderun:    true,
			MountCderunSet: true,
			MountSocket:    false,
			MountSocketSet: true,
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
		res, err := ResolveWithFS("sh", &CLIOptions{Image: "alpine", ImageSet: true}, nil, nil, mfs)
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
		res, err := ResolveWithFS("sh", &CLIOptions{Image: "alpine", ImageSet: true}, nil, global, fs)
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
		res, err := ResolveWithFS("sh", &CLIOptions{Image: "alpine", ImageSet: true}, tools, nil, fs)
		require.NoError(t, err)
		assert.Equal(t, []string{"node"}, res.MountTools)
		assert.True(t, res.MountCderun)
		assert.True(t, res.MountSocket)
	})

	t.Run("complex override: P4 Tool enables, but P2 CLI disables mount-cderun", func(t *testing.T) {
		tools := ToolsConfig{
			"sh": ToolConfig{
				MountTools: []string{"node"},
			},
		}
		cli := &CLIOptions{
			Image:          "alpine",
			ImageSet:       true,
			MountCderun:    false,
			MountCderunSet: true,
		}
		res, err := ResolveWithFS("sh", cli, tools, nil, fs)
		require.NoError(t, err)
		assert.Equal(t, []string{"node"}, res.MountTools)
		assert.False(t, res.MountCderun)
		assert.False(t, res.MountSocket)
	})
}
