package command

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/config"
	"cderun/internal/container"
	"cderun/internal/logging"
)

func TestUnit_Snapshot_Immutability(t *testing.T) {
	mfs := &config.MockFileSystem{}
	globalCfg := &config.CDERunConfig{
		Runtime: "docker",
		HostContext: &config.HostContext{
			Level: 1,
			Mounts: []config.MountMapping{
				{Source: "/h1", Target: "/c1", Level: 1},
			},
		},
	}
	// Save initial state for comparison
	initialLevel := globalCfg.HostContext.Level
	initialMountsCount := len(globalCfg.HostContext.Mounts)
	initialMountSource := globalCfg.HostContext.Mounts[0].Source

	toolsCfg := config.ToolsConfig{}
	currentMounts := []container.Mount{
		{Type: "bind", Source: "/h2", Target: "/c2"},
	}

	snapshotDir, err := createSnapshot(logging.NewLogger(), mfs, globalCfg, toolsCfg, currentMounts, nil)
	require.NoError(t, err)
	if snapshotDir != "" {
		t.Cleanup(func() { _ = cleanupSnapshot(mfs, snapshotDir) })
	}

	// Verify that globalCfg was NOT mutated
	assert.Equal(t, initialLevel, globalCfg.HostContext.Level)
	assert.Len(t, globalCfg.HostContext.Mounts, initialMountsCount)
	assert.Equal(t, initialMountSource, globalCfg.HostContext.Mounts[0].Source)
}

func TestUnit_Snapshot_WithNilHostContext(t *testing.T) {
	mfs := &config.MockFileSystem{}
	globalCfg := &config.CDERunConfig{
		Runtime: "docker",
	}
	assert.Nil(t, globalCfg.HostContext)

	toolsCfg := config.ToolsConfig{}
	currentMounts := []container.Mount{}

	snapshotDir, err := createSnapshot(logging.NewLogger(), mfs, globalCfg, toolsCfg, currentMounts, nil)
	require.NoError(t, err)
	if snapshotDir != "" {
		t.Cleanup(func() { _ = cleanupSnapshot(mfs, snapshotDir) })
	}

	// Verify that globalCfg.HostContext is still nil
	assert.Nil(t, globalCfg.HostContext)
}

func TestUnit_Snapshot_Permissions(t *testing.T) {
	mfs := &config.MockFileSystem{}
	globalCfg := &config.CDERunConfig{}
	toolsCfg := config.ToolsConfig{}
	currentMounts := []container.Mount{}

	snapshotDir, err := createSnapshot(logging.NewLogger(), mfs, globalCfg, toolsCfg, currentMounts, nil)
	require.NoError(t, err)
	if snapshotDir != "" {
		t.Cleanup(func() { _ = cleanupSnapshot(mfs, snapshotDir) })
	}

	// Verify snapshot directory permissions (0700)
	assert.Equal(t, os.FileMode(0o700), mfs.Perms[snapshotDir])

	// Verify configuration file permissions (0600)
	assert.Equal(t, os.FileMode(0o600), mfs.Perms[filepath.Join(snapshotDir, ".cderun.yaml")])
	assert.Equal(t, os.FileMode(0o600), mfs.Perms[filepath.Join(snapshotDir, ".tools.yaml")])
}

type mockMountInfoReader struct {
	Content []byte
	Err     error
}

func (m *mockMountInfoReader) ReadMountInfo(fs config.FileSystem) ([]byte, error) {
	return m.Content, m.Err
}

func TestUnit_Snapshot_DiscoverOverlay(t *testing.T) {
	mfs := &config.MockFileSystem{}

	t.Run("successfully discover upperdir", func(t *testing.T) {
		mountinfo := "24 25 0:21 / / rw,relatime - overlay overlay rw,lowerdir=/l,upperdir=/u,workdir=/w\n"
		mockReader := &mockMountInfoReader{Content: []byte(mountinfo)}

		upperdir, err := discoverOverlayUpperDir(mfs, mockReader)
		require.NoError(t, err)
		assert.Equal(t, "/u", upperdir)
	})

	t.Run("no overlay found", func(t *testing.T) {
		mountinfo := "24 25 0:21 / / rw,relatime - ext4 /dev/sda1 rw\n"
		mockReader := &mockMountInfoReader{Content: []byte(mountinfo)}

		upperdir, err := discoverOverlayUpperDir(mfs, mockReader)
		require.NoError(t, err)
		assert.Empty(t, upperdir)
	})

	t.Run("malformed mountinfo", func(t *testing.T) {
		mountinfo := "too few fields\n"
		mockReader := &mockMountInfoReader{Content: []byte(mountinfo)}

		upperdir, err := discoverOverlayUpperDir(mfs, mockReader)
		require.NoError(t, err)
		assert.Empty(t, upperdir)
	})
}

func TestUnit_Snapshot_EdgeCases(t *testing.T) {
	mfs := &config.MockFileSystem{}
	logger := logging.NewLogger()
	globalCfg := &config.CDERunConfig{}
	toolsCfg := config.ToolsConfig{}
	currentMounts := []container.Mount{}

	t.Run("mkdir failure", func(t *testing.T) {
		mfs_err := &config.MockFileSystem{
			MkdirAllErr: errors.New("mkdir failed"),
		}
		_, err := createSnapshot(logger, mfs_err, globalCfg, toolsCfg, currentMounts, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create snapshot directory")
	})

	t.Run("write file failure", func(t *testing.T) {
		mfs_err := &config.MockFileSystem{
			WriteFileErr: errors.New("write failed"),
		}
		_, err := createSnapshot(logger, mfs_err, globalCfg, toolsCfg, currentMounts, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to write .cderun.yaml to snapshot")
	})

	t.Run("multiple overlay mounts", func(t *testing.T) {
		mountinfo := "24 25 0:21 / / rw,relatime - ext4 /dev/sda1 rw\n" +
			"124 25 0:22 / / rw,relatime - overlay overlay rw,lowerdir=/l,upperdir=/u,workdir=/w\n"
		mockReader := &mockMountInfoReader{Content: []byte(mountinfo)}
		upperdir, err := discoverOverlayUpperDir(mfs, mockReader)
		require.NoError(t, err)
		assert.Equal(t, "/u", upperdir)
	})

	t.Run("non-root overlay mount ignored", func(t *testing.T) {
		mountinfo := "124 25 0:22 /data /data rw,relatime - overlay overlay rw,lowerdir=/l,upperdir=/u,workdir=/w\n"
		mockReader := &mockMountInfoReader{Content: []byte(mountinfo)}
		upperdir, err := discoverOverlayUpperDir(mfs, mockReader)
		require.NoError(t, err)
		assert.Empty(t, upperdir)
	})
}
