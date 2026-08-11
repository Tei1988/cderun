package command

import (
	"bytes"
	"testing"

	"cderun/internal/config"
	"cderun/internal/container"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Command_IpcFlagResolution(t *testing.T) {
	t.Parallel()

	t.Run("ipc option resolved via standard flags", func(t *testing.T) {
		o := &rootOptions{}
		o.fs = config.RealFileSystem{}
		o.ensureHooks()
		cmd := newRootCmd(o)

		err := cmd.ParseFlags([]string{"--ipc", "host", "--image", "alpine"})
		require.NoError(t, err)

		resolved, err := o.resolveSettings(cmd, "node", nil, nil)
		require.NoError(t, err)
		assert.Equal(t, "host", resolved.Ipc)
	})

	t.Run("cderun-ipc option resolved via wrapper-mode override", func(t *testing.T) {
		o := &rootOptions{}
		o.fs = config.RealFileSystem{}
		o.ensureHooks()
		cmd := newRootCmd(o)

		err := cmd.ParseFlags([]string{"--ipc", "invalid", "--cderun-ipc", "host", "--image", "alpine"})
		require.NoError(t, err)

		resolved, err := o.resolveSettings(cmd, "node", nil, nil)
		require.NoError(t, err)
		assert.Equal(t, "host", resolved.Ipc)
	})

	t.Run("ipc option fails on invalid configuration format", func(t *testing.T) {
		o := &rootOptions{}
		o.fs = config.RealFileSystem{}
		o.ensureHooks()
		cmd := newRootCmd(o)

		err := cmd.ParseFlags([]string{"--ipc", "invalid_mode_here", "--image", "alpine"})
		require.NoError(t, err)

		_, err = o.resolveSettings(cmd, "node", nil, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported ipc namespace")
	})

	t.Run("ipc dry-run prints resolved ipc namespace value", func(t *testing.T) {
		o := &rootOptions{}
		cmd := newRootCmd(o)
		containerConfig := &container.ContainerConfig{
			Image: "alpine",
			Ipc:   "host",
		}
		resolved := &config.ResolvedConfig{
			Image:        "alpine",
			Ipc:          "host",
			DryRunFormat: "simple",
		}

		var buf bytes.Buffer
		cmd.SetOut(&buf)
		err := o.handleDryRun(cmd, containerConfig, resolved)
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "Ipc: host")
	})
}
