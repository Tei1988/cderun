package command

import (
	"cderun/internal/config"
	"cderun/internal/container"
	"cderun/internal/logging"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

func createSnapshot(fs config.FileSystem, globalCfg *config.CDERunConfig, toolsCfg config.ToolsConfig, currentMounts []container.Mount) (string, error) {
	id := uuid.New().String()
	snapshotDir := filepath.Join(fs.TempDir(), "cderun-snap-"+id)

	if err := fs.MkdirAll(snapshotDir, 0755); err != nil {
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
	if upperDir, err := discoverOverlayUpperDir(fs); err == nil && upperDir != "" {
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
	if err := fs.WriteFile(filepath.Join(snapshotDir, ".cderun.yaml"), cderunData, 0644); err != nil {
		return "", fmt.Errorf("failed to write .cderun.yaml to snapshot: %w", err)
	}

	// Save .tools.yaml
	toolsData, err := yaml.Marshal(toolsCfg)
	if err != nil {
		return "", fmt.Errorf("failed to marshal tools config: %w", err)
	}
	if err := fs.WriteFile(filepath.Join(snapshotDir, ".tools.yaml"), toolsData, 0644); err != nil {
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

// mountInfoReader is an interface for reading mount information (e.g., from /proc/self/mountinfo).
type mountInfoReader interface {
	ReadMountInfo(fs config.FileSystem) ([]byte, error)
}

// realMountInfoReader reads from /proc/self/mountinfo.
type realMountInfoReader struct{}

func (realMountInfoReader) ReadMountInfo(fs config.FileSystem) ([]byte, error) {
	return fs.ReadFile("/proc/self/mountinfo")
}

var defaultMountInfoReader mountInfoReader = realMountInfoReader{}

func discoverOverlayUpperDir(fs config.FileSystem) (string, error) {
	data, err := defaultMountInfoReader.ReadMountInfo(fs)
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}

		// Find separator "-"
		sepIdx := -1
		for i, f := range fields {
			if f == "-" {
				sepIdx = i
				break
			}
		}
		if sepIdx == -1 || len(fields) <= sepIdx+3 {
			continue
		}

		fsType := fields[sepIdx+1]
		if fsType != "overlay" {
			continue
		}

		mountPoint := fields[4]
		if mountPoint != "/" {
			continue
		}

		// Superblock options are at the end
		sbOptions := fields[len(fields)-1]
		options := strings.Split(sbOptions, ",")
		for _, opt := range options {
			if strings.HasPrefix(opt, "upperdir=") {
				return strings.TrimPrefix(opt, "upperdir="), nil
			}
		}
	}

	return "", nil
}
