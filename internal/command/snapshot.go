package command

import (
	"fmt"
	"path/filepath"

	"cderun/internal/config"
	"cderun/internal/container"
	"cderun/internal/logging"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

func createSnapshot(fs config.FileSystem, globalCfg *config.CDERunConfig, toolsCfg config.ToolsConfig, currentMounts []container.Mount) (string, error) {
	id := uuid.New().String()
	snapshotDir := filepath.Join(fs.TempDir(), "cderun-snap-"+id)

	if err := fs.MkdirAll(snapshotDir, 0o700); err != nil {
		return "", fmt.Errorf("failed to create snapshot directory: %w", err)
	}

	// Prepare HostContext (copy to avoid mutating the caller's config)
	var hostCtx config.HostContext
	if globalCfg.HostContext != nil {
		hostCtx = *globalCfg.HostContext
		// Deep copy Mounts slice
		hostCtx.Mounts = make([]config.MountMapping, len(globalCfg.HostContext.Mounts))
		copy(hostCtx.Mounts, globalCfg.HostContext.Mounts)
	}

	hostCtx.SnapshotDir = snapshotDir

	exePath, err := fs.Executable()
	if err == nil {
		hostCtx.BinPath = exePath
	}

	pwd, err := fs.Getwd()
	if err == nil {
		hostCtx.WorkingDir = pwd
	}

	// Increment level
	hostCtx.Level++

	// Map current mounts into HostContext.Mounts
	for _, m := range currentMounts {
		if m.Type == "bind" {
			hostCtx.Mounts = append(hostCtx.Mounts, config.MountMapping{
				Source: m.Source,
				Target: m.Target,
				Level:  hostCtx.Level,
			})
		}
	}

	// OverlayFS root discovery (only at level 1 if we want to find the host root)
	// Actually, it can be done at any level if we want to find the "upperdir" of the current container.
	if upperDir, err := config.DiscoverOverlayUpperDir(fs); err == nil && upperDir != "" {
		logging.Debug("Discovered OverlayFS upperdir: %s", upperDir)
		hostCtx.Mounts = append(hostCtx.Mounts, config.MountMapping{
			Source: upperDir,
			Target: "/",
			Level:  hostCtx.Level,
		})
	}

	// Create a temporary config for marshaling to avoid side effects on the caller's config
	snapshotCfg := *globalCfg
	snapshotCfg.HostContext = &hostCtx

	// Save .cderun.yaml
	cderunData, err := yaml.Marshal(snapshotCfg)
	if err != nil {
		return "", fmt.Errorf("failed to marshal cderun config: %w", err)
	}
	if err := fs.WriteFile(filepath.Join(snapshotDir, ".cderun.yaml"), cderunData, 0o600); err != nil {
		return "", fmt.Errorf("failed to write .cderun.yaml to snapshot: %w", err)
	}

	// Save .tools.yaml
	toolsData, err := yaml.Marshal(toolsCfg)
	if err != nil {
		return "", fmt.Errorf("failed to marshal tools config: %w", err)
	}
	if err := fs.WriteFile(filepath.Join(snapshotDir, ".tools.yaml"), toolsData, 0o600); err != nil {
		return "", fmt.Errorf("failed to write .tools.yaml to snapshot: %w", err)
	}

	return snapshotDir, nil
}

func cleanupSnapshot(fs config.FileSystem, snapshotDir string) error {
	if snapshotDir == "" {
		return nil
	}
	return fs.RemoveAll(snapshotDir)
}
