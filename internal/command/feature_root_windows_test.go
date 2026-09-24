//go:build windows

package command

import (
	"testing"

	"cderun/internal/config"
	"cderun/internal/container"
	"cderun/internal/logging"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Command_ApplySocketMount_GIDAutoAdd_Windows(t *testing.T) {
	t.Parallel()

	mfs := &config.MockFileSystem{}
	o := defaultOptions()
	o.fs = mfs
	o.logger = logging.NewLogger()

	cfg := &container.ContainerConfig{}
	resolved := &config.ResolvedConfig{
		MountSocket:     true,
		SocketPath:      "/var/run/docker.sock",
		MountSocketPath: "/var/run/docker.sock",
	}

	err := o.applySocketMount(cfg, resolved)
	require.NoError(t, err)
	require.Len(t, cfg.Mounts, 1)
	assert.Empty(t, cfg.GroupAdd)
}
