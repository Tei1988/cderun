package command

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/config"
	"cderun/internal/container"
	"cderun/internal/logging"
)

func TestUnit_Snapshot_BuildSnapshotHostContext(t *testing.T) {
	t.Parallel()
	mfs := &config.MockFileSystem{}
	logger := logging.NewLogger()

	baseCtx := &config.HostContext{
		Level: 1,
		Mounts: []config.MountMapping{
			{Source: "/host/a", Target: "/container/a", Level: 1},
		},
	}

	currentMounts := []container.Mount{
		{Type: "bind", Source: "/host/b", Target: "/container/b"},
		{Type: "tmpfs", Target: "/tmp"}, // Should be ignored as non-bind
	}

	mountinfo := "24 25 0:21 / / rw,relatime - overlay overlay rw,lowerdir=/l,upperdir=/var/lib/docker/overlay2/diff,workdir=/w\n"
	reader := &mockMountInfoReader{Content: []byte(mountinfo)}

	ctx := buildSnapshotHostContext(logger, mfs, baseCtx, currentMounts, reader)

	assert.Equal(t, 2, ctx.Level)
	require.Len(t, ctx.Mounts, 3)

	// Verify original mount preserved
	assert.Equal(t, "/host/a", ctx.Mounts[0].Source)
	assert.Equal(t, 1, ctx.Mounts[0].Level)

	// Verify current bind mount added
	assert.Equal(t, "/host/b", ctx.Mounts[1].Source)
	assert.Equal(t, "/container/b", ctx.Mounts[1].Target)
	assert.Equal(t, 2, ctx.Mounts[1].Level)

	// Verify OverlayFS upperdir added
	assert.Equal(t, "/var/lib/docker/overlay2/diff", ctx.Mounts[2].Source)
	assert.Equal(t, "/", ctx.Mounts[2].Target)
	assert.Equal(t, 2, ctx.Mounts[2].Level)
}

func TestUnit_Snapshot_ResolveHostSnapshotDir(t *testing.T) {
	t.Parallel()

	t.Run("Level 0 returns snapshotDir unchanged", func(t *testing.T) {
		t.Parallel()
		mfs := &config.MockFileSystem{}
		globalCfg := &config.CDERunConfig{
			HostContext: nil,
		}
		hostCtx := &config.HostContext{Level: 1}

		res, err := resolveHostSnapshotDir(mfs, globalCfg, hostCtx, "/tmp/snap")
		require.NoError(t, err)
		assert.Equal(t, "/tmp/snap", res)
	})

	t.Run("Level > 0 resolves host path using HostContext mounts", func(t *testing.T) {
		t.Parallel()
		mfs := &config.MockFileSystem{}
		globalCfg := &config.CDERunConfig{
			HostContext: &config.HostContext{Level: 1},
		}
		hostCtx := &config.HostContext{
			Level: 2,
			Mounts: []config.MountMapping{
				{Source: "/var/lib/docker/upper", Target: "/", Level: 2},
			},
		}

		res, err := resolveHostSnapshotDir(mfs, globalCfg, hostCtx, "/tmp/snap")
		require.NoError(t, err)
		assert.Equal(t, "/var/lib/docker/upper/tmp/snap", res)
	})
}

func TestUnit_Snapshot_StartSnapshotControlSocket(t *testing.T) {
	t.Parallel()

	t.Run("Disabled when mountCderunSocket is false", func(t *testing.T) {
		t.Parallel()
		mfs := &config.MockFileSystem{}
		logger := logging.NewLogger()
		hostCtx := &config.HostContext{}

		server, err := startSnapshotControlSocket(mfs, logger, hostCtx, "/tmp/snap", "/host/snap", false)
		require.NoError(t, err)
		assert.Nil(t, server)
		assert.Empty(t, hostCtx.ControlSocket)
	})

	t.Run("Sets ControlSocket path when mountCderunSocket is true on MockFileSystem", func(t *testing.T) {
		t.Parallel()
		mfs := &config.MockFileSystem{}
		logger := logging.NewLogger()
		hostCtx := &config.HostContext{}

		server, err := startSnapshotControlSocket(mfs, logger, hostCtx, "/tmp/snap", "/host/snap", true)
		require.NoError(t, err)
		assert.Nil(t, server) // On MockFileSystem, RealFileSystem check returns false, so server is nil
		assert.Equal(t, "/host/snap/cderun.sock", hostCtx.ControlSocket)
	})
}

func TestUnit_Snapshot_PopulateHostContextPaths(t *testing.T) {
	t.Parallel()
	mfs := &config.MockFileSystem{
		ExecPath: "/usr/local/bin/cderun",
		WD:       "/home/user/project",
		HomeDir:  "/home/user",
	}
	logger := logging.NewLogger()

	hostCtx := &config.HostContext{}
	populateHostContextPaths(mfs, logger, hostCtx)

	assert.Equal(t, "/usr/local/bin/cderun", hostCtx.BinPath)
	assert.Equal(t, "/home/user/project", hostCtx.WorkingDir)
	assert.Equal(t, "/home/user", hostCtx.HomeDir)
}

func TestUnit_Snapshot_WriteSnapshotConfigFiles(t *testing.T) {
	t.Parallel()
	mfs := &config.MockFileSystem{}
	snapshotDir := "/tmp/snap-123"

	globalCfg := &config.CDERunConfig{
		Runtime: "docker",
	}
	toolsCfg := config.ToolsConfig{
		"node": {Image: "node:18"},
	}
	hostCtx := &config.HostContext{Level: 1}

	err := writeSnapshotConfigFiles(mfs, snapshotDir, globalCfg, toolsCfg, hostCtx)
	require.NoError(t, err)

	assert.Equal(t, os.FileMode(0o600), mfs.Perms[filepath.Join(snapshotDir, ".cderun.yaml")])
	assert.Equal(t, os.FileMode(0o600), mfs.Perms[filepath.Join(snapshotDir, ".tools.yaml")])

	// Verify that globalCfg and toolsCfg were not mutated
	assert.Nil(t, globalCfg.HostContext)
}
