package command

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/config"
	"cderun/internal/container"
	"cderun/internal/logging"
)

func TestUnit_Snapshot_PathResolutionInNestedExecution(t *testing.T) {
	mfs := &config.MockFileSystem{}
	originalReader := defaultMountInfoReader
	defer func() { defaultMountInfoReader = originalReader }()

	// Simulate OverlayFS: container / maps to host /var/lib/docker/overlay2/abc/diff
	mountinfo := "24 25 0:21 / / rw,relatime - overlay overlay rw,lowerdir=/l,upperdir=/var/lib/docker/overlay2/abc/diff,workdir=/w\n"
	defaultMountInfoReader = &mockMountInfoReader{Content: []byte(mountinfo)}

	globalCfg := &config.CDERunConfig{
		HostContext: &config.HostContext{
			Level: 1,
		},
	}

	containerDir, hostDir, err := createSnapshot(logging.NewLogger(), mfs, globalCfg, config.ToolsConfig{}, nil)
	require.NoError(t, err)
	if containerDir != "" {
		t.Cleanup(func() { _ = cleanupSnapshot(mfs, containerDir) })
	}

	// hostDir should be resolved via OverlayFS upperdir, not container-local /tmp
	assert.True(t, strings.HasPrefix(hostDir, "/var/lib/docker/overlay2/abc/diff/"), "expected host path via OverlayFS, got: %s", hostDir)
}

func TestUnit_Snapshot_ConfigurationImmutability(t *testing.T) {
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

	containerDir, _, err := createSnapshot(logging.NewLogger(), mfs, globalCfg, toolsCfg, currentMounts)
	require.NoError(t, err)
	if containerDir != "" {
		t.Cleanup(func() { _ = cleanupSnapshot(mfs, containerDir) })
	}

	// Verify that globalCfg was NOT mutated
	assert.Equal(t, initialLevel, globalCfg.HostContext.Level)
	assert.Len(t, globalCfg.HostContext.Mounts, initialMountsCount)
	assert.Equal(t, initialMountSource, globalCfg.HostContext.Mounts[0].Source)
}

func TestUnit_Snapshot_InitializationWithNilHostContext(t *testing.T) {
	mfs := &config.MockFileSystem{}
	globalCfg := &config.CDERunConfig{
		Runtime: "docker",
	}
	assert.Nil(t, globalCfg.HostContext)

	toolsCfg := config.ToolsConfig{}
	currentMounts := []container.Mount{}

	containerDir, _, err := createSnapshot(logging.NewLogger(), mfs, globalCfg, toolsCfg, currentMounts)
	require.NoError(t, err)
	if containerDir != "" {
		t.Cleanup(func() { _ = cleanupSnapshot(mfs, containerDir) })
	}

	// Verify that globalCfg.HostContext is still nil
	assert.Nil(t, globalCfg.HostContext)
}

func TestUnit_Snapshot_DirectoryAndFilePermissions(t *testing.T) {
	mfs := &config.MockFileSystem{}
	globalCfg := &config.CDERunConfig{}
	toolsCfg := config.ToolsConfig{}
	currentMounts := []container.Mount{}

	containerDir, _, err := createSnapshot(logging.NewLogger(), mfs, globalCfg, toolsCfg, currentMounts)
	require.NoError(t, err)
	if containerDir != "" {
		t.Cleanup(func() { _ = cleanupSnapshot(mfs, containerDir) })
	}

	// Verify snapshot directory permissions (0700)
	assert.Equal(t, os.FileMode(0o700), mfs.Perms[containerDir])

	// Verify configuration file permissions (0600)
	assert.Equal(t, os.FileMode(0o600), mfs.Perms[filepath.Join(containerDir, ".cderun.yaml")])
	assert.Equal(t, os.FileMode(0o600), mfs.Perms[filepath.Join(containerDir, ".tools.yaml")])
}

type mockMountInfoReader struct {
	Content []byte
	Err     error
}

func (m *mockMountInfoReader) ReadMountInfo(fs config.FileSystem) ([]byte, error) {
	return m.Content, m.Err
}

func TestUnit_Snapshot_OverlayFSDiscovery(t *testing.T) {
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
