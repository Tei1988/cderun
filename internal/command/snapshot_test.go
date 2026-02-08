package command

import (
	"cderun/internal/config"
	"cderun/internal/container"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateSnapshotImmutability(t *testing.T) {
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

	snapshotDir, err := createSnapshot(globalCfg, toolsCfg, currentMounts)
	assert.NoError(t, err)
	if snapshotDir != "" {
		defer func() { _ = os.RemoveAll(snapshotDir) }()
	}

	// Verify that globalCfg was NOT mutated
	assert.Equal(t, initialLevel, globalCfg.HostContext.Level)
	assert.Equal(t, initialMountsCount, len(globalCfg.HostContext.Mounts))
	assert.Equal(t, initialMountSource, globalCfg.HostContext.Mounts[0].Source)
}

func TestCreateSnapshotWithNilHostContext(t *testing.T) {
	globalCfg := &config.CDERunConfig{
		Runtime: "docker",
	}
	assert.Nil(t, globalCfg.HostContext)

	toolsCfg := config.ToolsConfig{}
	currentMounts := []container.Mount{}

	snapshotDir, err := createSnapshot(globalCfg, toolsCfg, currentMounts)
	assert.NoError(t, err)
	if snapshotDir != "" {
		defer func() { _ = os.RemoveAll(snapshotDir) }()
	}

	// Verify that globalCfg.HostContext is still nil
	assert.Nil(t, globalCfg.HostContext)
}
