//go:build !windows

package command

import (
	"syscall"
	"testing"

	"cderun/internal/config"
	"cderun/internal/container"
	"cderun/internal/logging"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Command_ApplySocketMount_GIDAutoAdd_Unix(t *testing.T) {
	t.Parallel()

	mfs := &config.MockFileSystem{}
	sfs := &mockSocketFS{FileSystem: mfs}

	sfs.statFileInfo = &mockFileInfoWithSys{
		FileInfo: nil, // We only need Sys() to return the correct structure
		sys:      &syscall.Stat_t{Gid: 1002},
	}

	o := defaultOptions()
	o.fs = sfs
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
	assert.Contains(t, cfg.GroupAdd, "1002")
}
