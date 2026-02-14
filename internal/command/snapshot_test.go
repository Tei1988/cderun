package command

import (
	"cderun/internal/config"
	"cderun/internal/container"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Command_Snapshot_Immutability(t *testing.T) {
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

	snapshotDir, err := createSnapshot(mfs, globalCfg, toolsCfg, currentMounts)
	require.NoError(t, err)
	if snapshotDir != "" {
		t.Cleanup(func() { _ = cleanupSnapshot(mfs, snapshotDir) })
	}

	// Verify that globalCfg was NOT mutated
	assert.Equal(t, initialLevel, globalCfg.HostContext.Level)
	assert.Len(t, globalCfg.HostContext.Mounts, initialMountsCount)
	assert.Equal(t, initialMountSource, globalCfg.HostContext.Mounts[0].Source)
}

func TestUnit_Command_Snapshot_WithNilHostContext(t *testing.T) {
	mfs := &config.MockFileSystem{}
	globalCfg := &config.CDERunConfig{
		Runtime: "docker",
	}
	assert.Nil(t, globalCfg.HostContext)

	toolsCfg := config.ToolsConfig{}
	currentMounts := []container.Mount{}

	snapshotDir, err := createSnapshot(mfs, globalCfg, toolsCfg, currentMounts)
	require.NoError(t, err)
	if snapshotDir != "" {
		t.Cleanup(func() { _ = cleanupSnapshot(mfs, snapshotDir) })
	}

	// Verify that globalCfg.HostContext is still nil
	assert.Nil(t, globalCfg.HostContext)
}

func TestUnit_Command_Snapshot_Permissions(t *testing.T) {
	mfs := &config.MockFileSystem{}
	globalCfg := &config.CDERunConfig{}
	toolsCfg := config.ToolsConfig{}
	currentMounts := []container.Mount{}

	snapshotDir, err := createSnapshot(mfs, globalCfg, toolsCfg, currentMounts)
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

func TestUnit_Command_Snapshot_DiscoverOverlay(t *testing.T) {
	mfs := &config.MockFileSystem{}
	originalReader := defaultMountInfoReader
	defer func() { defaultMountInfoReader = originalReader }()

	t.Run("successfully discover upperdir", func(t *testing.T) {
		mountinfo := "24 25 0:21 / / rw,relatime - overlay overlay rw,lowerdir=/l,upperdir=/u,workdir=/w\n"
		defaultMountInfoReader = &mockMountInfoReader{Content: []byte(mountinfo)}

		upperdir, err := discoverOverlayUpperDir(mfs)
		require.NoError(t, err)
		assert.Equal(t, "/u", upperdir)
	})

	t.Run("no overlay found", func(t *testing.T) {
		mountinfo := "24 25 0:21 / / rw,relatime - ext4 /dev/sda1 rw\n"
		defaultMountInfoReader = &mockMountInfoReader{Content: []byte(mountinfo)}

		upperdir, err := discoverOverlayUpperDir(mfs)
		require.NoError(t, err)
		assert.Empty(t, upperdir)
	})

	t.Run("malformed mountinfo", func(t *testing.T) {
		mountinfo := "too few fields\n"
		defaultMountInfoReader = &mockMountInfoReader{Content: []byte(mountinfo)}

		upperdir, err := discoverOverlayUpperDir(mfs)
		require.NoError(t, err)
		assert.Empty(t, upperdir)
	})
}
