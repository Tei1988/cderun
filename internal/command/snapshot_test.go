package command

import (
	"os"
	"path/filepath"
	"testing"

	"cderun/internal/config"
	"cderun/internal/container"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Command_Snapshot_Immutability(t *testing.T) {
	setupNoOverlay(t)
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
	setupNoOverlay(t)
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
	setupNoOverlay(t)
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
