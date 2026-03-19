package command

import (
	"fmt"
	"path/filepath"
	"strings"

	"cderun/internal/config"
	"cderun/internal/container"
	"cderun/internal/logging"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// createSnapshot creates a snapshot directory and returns (containerDir, hostDir, error).
// containerDir is the path inside the current container (used for file I/O and cleanup).
// hostDir is the resolved host path (used as mount source for the next container).
func createSnapshot(logger *logging.Logger, fs config.FileSystem, globalCfg *config.CDERunConfig, toolsCfg config.ToolsConfig, currentMounts []container.Mount) (string, string, error) {
	id := uuid.New().String()
	snapshotDir := filepath.Join(fs.TempDir(), "cderun-snap-"+id)

	// Prepare HostContext (copy to avoid mutating the caller's config)
	var hostCtx config.HostContext
	if globalCfg.HostContext != nil {
		hostCtx = globalCfg.HostContext.DeepCopy()
	}

	// Increment level first so mounts are recorded at the correct level
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

	// Discover OverlayFS upperdir and add as root mapping before path resolution
	if upperDir, err := discoverOverlayUpperDir(fs); err == nil && upperDir != "" {
		logger.Debug("Discovered OverlayFS upperdir: %s", upperDir)
		hostCtx.Mounts = append(hostCtx.Mounts, config.MountMapping{
			Source: upperDir,
			Target: "/",
			Level:  hostCtx.Level,
		})
	}

	// MkdirAll uses the container-local path; this works because we are running inside the container.
	if err := fs.MkdirAll(snapshotDir, 0o700); err != nil {
		return "", "", fmt.Errorf("failed to create snapshot directory: %w", err)
	}

	// When running inside a container (level >= 1), snapshotDir is a container-local path.
	// Resolve it to a host path for SnapshotDir, which is used as a mount source.
	hostSnapshotDir := snapshotDir
	if hostCtx.Level > 1 {
		r, err := config.NewExpressionResolverWithFS(&hostCtx, fs)
		if err != nil {
			return "", "", fmt.Errorf("failed to create expression resolver: %w", err)
		}
		resolvedSnapshotDir, err := config.ResolvePath(snapshotDir, "", r)
		if err != nil {
			return "", "", fmt.Errorf("failed to resolve snapshot directory to host path: %w", err)
		}
		if resolvedSnapshotDir == "" {
			return "", "", fmt.Errorf("failed to resolve snapshot directory: empty result")
		}
		hostSnapshotDir = resolvedSnapshotDir
	}

	hostCtx.SnapshotDir = hostSnapshotDir

	exePath, err := fs.Executable()
	if err == nil {
		hostCtx.BinPath = exePath
	} else {
		logger.Debug("failed to get executable path for snapshot: %v", err)
	}

	pwd, err := fs.Getwd()
	if err == nil {
		hostCtx.WorkingDir = pwd
	} else {
		logger.Debug("failed to get working directory for snapshot: %v", err)
	}

	home, err := fs.UserHomeDir()
	if err == nil {
		hostCtx.HomeDir = home
	} else {
		logger.Debug("failed to get home directory for snapshot: %v", err)
	}

	// Create a temporary config for marshaling to avoid side effects on the caller's config
	snapshotCfg := globalCfg.DeepCopy()
	snapshotCfg.HostContext = &hostCtx

	// Create a copy of toolsCfg to avoid side effects
	snapshotToolsCfg := toolsCfg.DeepCopy()

	// Save .cderun.yaml
	cderunData, err := yaml.Marshal(snapshotCfg)
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal cderun config: %w", err)
	}
	if err := fs.WriteFile(filepath.Join(snapshotDir, ".cderun.yaml"), cderunData, 0o600); err != nil {
		return "", "", fmt.Errorf("failed to write .cderun.yaml to snapshot: %w", err)
	}

	// Save .tools.yaml
	toolsData, err := yaml.Marshal(snapshotToolsCfg)
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal tools config: %w", err)
	}
	if err := fs.WriteFile(filepath.Join(snapshotDir, ".tools.yaml"), toolsData, 0o600); err != nil {
		return "", "", fmt.Errorf("failed to write .tools.yaml to snapshot: %w", err)
	}

	return snapshotDir, hostSnapshotDir, nil
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

	lines := strings.SplitSeq(string(data), "\n")
	for line := range lines {
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
		options := strings.SplitSeq(sbOptions, ",")
		for opt := range options {
			if after, ok := strings.CutPrefix(opt, "upperdir="); ok {
				return after, nil
			}
		}
	}

	return "", nil
}
