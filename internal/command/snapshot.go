package command

import (
	"cderun/internal/config"
	"cderun/internal/container"
	"cderun/internal/logging"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// createSnapshot creates a temporary snapshot directory and records the current host context and mounts into it.
// It ensures globalCfg.HostContext exists, sets HostContext.SnapshotDir, BinPath (when available), WorkingDir (when available),
// increments HostContext.Level, appends bind mounts from currentMounts and an OverlayFS upperdir (if discovered) to HostContext.Mounts,
// and writes .cderun.yaml and .tools.yaml into the snapshot directory.
// On success it returns the path to the created snapshot directory; on failure it returns a non-nil error.
func createSnapshot(globalCfg *config.CDERunConfig, toolsCfg config.ToolsConfig, currentMounts []container.Mount) (string, error) {
	id := uuid.New().String()
	snapshotDir := filepath.Join(os.TempDir(), "cderun-snap-"+id)

	if err := os.MkdirAll(snapshotDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create snapshot directory: %w", err)
	}

	// Prepare HostContext
	if globalCfg.HostContext == nil {
		globalCfg.HostContext = &config.HostContext{
			Level: 0,
		}
	}

	hostCtx := globalCfg.HostContext
	hostCtx.SnapshotDir = snapshotDir

	exePath, err := os.Executable()
	if err == nil {
		hostCtx.BinPath = exePath
	}

	pwd, err := os.Getwd()
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
	if upperDir, err := discoverOverlayUpperDir(); err == nil && upperDir != "" {
		logging.Debug("Discovered OverlayFS upperdir: %s", upperDir)
		hostCtx.Mounts = append(hostCtx.Mounts, config.MountMapping{
			Source: upperDir,
			Target: "/",
			Level:  hostCtx.Level,
		})
	}

	// Save .cderun.yaml
	cderunData, err := yaml.Marshal(globalCfg)
	if err != nil {
		return "", fmt.Errorf("failed to marshal cderun config: %w", err)
	}
	if err := os.WriteFile(filepath.Join(snapshotDir, ".cderun.yaml"), cderunData, 0644); err != nil {
		return "", fmt.Errorf("failed to write .cderun.yaml to snapshot: %w", err)
	}

	// Save .tools.yaml
	toolsData, err := yaml.Marshal(toolsCfg)
	if err != nil {
		return "", fmt.Errorf("failed to marshal tools config: %w", err)
	}
	if err := os.WriteFile(filepath.Join(snapshotDir, ".tools.yaml"), toolsData, 0644); err != nil {
		return "", fmt.Errorf("failed to write .tools.yaml to snapshot: %w", err)
	}

	return snapshotDir, nil
}

// cleanupSnapshot removes the snapshot directory identified by snapshotDir.
// If snapshotDir is empty the function does nothing and returns nil; otherwise it removes
// the path and returns any error produced by os.RemoveAll.
func cleanupSnapshot(snapshotDir string) error {
	if snapshotDir == "" {
		return nil
	}
	return os.RemoveAll(snapshotDir)
}

// discoverOverlayUpperDir reads /proc/self/mountinfo and returns the OverlayFS `upperdir` path for the root mount if present.
// If an `upperdir` option is found for an overlay filesystem mounted at `/`, the path is returned.
// If no matching overlay `upperdir` is found the function returns an empty string and a nil error.
// Any error reading /proc/self/mountinfo is returned.
func discoverOverlayUpperDir() (string, error) {
	data, err := os.ReadFile("/proc/self/mountinfo")
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