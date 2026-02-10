package config

import (
	"fmt"
	"os"
	"path/filepath"

	"dario.cat/mergo"
	"gopkg.in/yaml.v3"
)

type CDERunConfig struct {
	Runtime     string         `yaml:"runtime,omitempty"`
	SocketPath  ConfigPath     `yaml:"socketPath,omitempty"`
	Defaults    ConfigDefaults `yaml:"defaults,omitempty"`
	Logging     LoggingConfig  `yaml:"logging,omitempty"`
	HostContext *HostContext   `yaml:"hostContext,omitempty"`
}

type HostContext struct {
	BinPath     string         `yaml:"binPath,omitempty"`
	SnapshotDir string         `yaml:"snapshotDir,omitempty"`
	WorkingDir  string         `yaml:"workingDir,omitempty"`
	Level       int            `yaml:"level,omitempty"`
	Mounts      []MountMapping `yaml:"mounts,omitempty"`
}

type MountMapping struct {
	Source string `yaml:"source,omitempty"`
	Target string `yaml:"target,omitempty"`
	Level  int    `yaml:"level,omitempty"`
}

func (c *CDERunConfig) SetBaseDir(baseDir string) {
	if c.SocketPath.Raw != "" {
		c.SocketPath.BaseDir = baseDir
	}
	c.Defaults.SetBaseDir(baseDir)
}

type ConfigDefaults struct {
	TTY             *bool          `yaml:"tty,omitempty"`
	Interactive     *bool          `yaml:"interactive,omitempty"`
	Network         string         `yaml:"network,omitempty"`
	Remove          *bool          `yaml:"remove,omitempty"`
	StrictEnv       *bool          `yaml:"strictEnv,omitempty"`
	MountCderun     *bool          `yaml:"mountCderun,omitempty"`
	MountCderunPath ConfigPath     `yaml:"mountCderunPath,omitempty"`
	MountSocket     *bool          `yaml:"mountSocket,omitempty"`
	MountSocketPath ConfigPath     `yaml:"mountSocketPath,omitempty"`
	Ports           []string       `yaml:"ports,omitempty"`
	PublishAll      *bool          `yaml:"publishAll,omitempty"`
	Expose          []string       `yaml:"expose,omitempty"`
	Hostname        string         `yaml:"hostname,omitempty"`
	DNS             []string       `yaml:"dns,omitempty"`
	AddHosts        []string       `yaml:"addHosts,omitempty"`
	User            string         `yaml:"user,omitempty"`
	Privileged      *bool          `yaml:"privileged,omitempty"`
	CapAdd          []string       `yaml:"capAdd,omitempty"`
	CapDrop         []string       `yaml:"capDrop,omitempty"`
	Entrypoint      []string       `yaml:"entrypoint,omitempty"`
	Command         []string       `yaml:"command,omitempty"`
	Pull            string         `yaml:"pull,omitempty"`
	Memory          string         `yaml:"memory,omitempty"`
	CPUs            float64        `yaml:"cpus,omitempty"`
	Mounts          []MountConfig  `yaml:"mounts,omitempty"`
	Devices         []DeviceConfig `yaml:"devices,omitempty"`
	Env             []string       `yaml:"env,omitempty"`
}

func (c *ConfigDefaults) SetBaseDir(baseDir string) {
	if c.MountSocketPath.Raw != "" {
		c.MountSocketPath.BaseDir = baseDir
	}
	if c.MountCderunPath.Raw != "" {
		c.MountCderunPath.BaseDir = baseDir
	}
	for i := range c.Mounts {
		c.Mounts[i].SetBaseDir(baseDir)
	}
	for i := range c.Devices {
		c.Devices[i].SetBaseDir(baseDir)
	}
}

type LoggingConfig struct {
	Level     string `yaml:"level,omitempty"`
	Format    string `yaml:"format,omitempty"`
	Timestamp *bool  `yaml:"timestamp,omitempty"`
}

type ToolConfig struct {
	Image           string         `yaml:"image,omitempty"`
	TTY             *bool          `yaml:"tty,omitempty"`
	Interactive     *bool          `yaml:"interactive,omitempty"`
	Network         string         `yaml:"network,omitempty"`
	Remove          *bool          `yaml:"remove,omitempty"`
	StrictEnv       *bool          `yaml:"strictEnv,omitempty"`
	Mounts          []MountConfig  `yaml:"mounts,omitempty"`
	Env             []string       `yaml:"env,omitempty"`
	Workdir         string         `yaml:"workdir,omitempty"`
	MountCderun     *bool          `yaml:"mountCderun,omitempty"`
	MountCderunPath ConfigPath     `yaml:"mountCderunPath,omitempty"`
	MountSocket     *bool          `yaml:"mountSocket,omitempty"`
	MountSocketPath ConfigPath     `yaml:"mountSocketPath,omitempty"`
	Ports           []string       `yaml:"ports,omitempty"`
	PublishAll      *bool          `yaml:"publishAll,omitempty"`
	Expose          []string       `yaml:"expose,omitempty"`
	Hostname        string         `yaml:"hostname,omitempty"`
	DNS             []string       `yaml:"dns,omitempty"`
	AddHosts        []string       `yaml:"addHosts,omitempty"`
	User            string         `yaml:"user,omitempty"`
	Privileged      *bool          `yaml:"privileged,omitempty"`
	CapAdd          []string       `yaml:"capAdd,omitempty"`
	CapDrop         []string       `yaml:"capDrop,omitempty"`
	Entrypoint      []string       `yaml:"entrypoint,omitempty"`
	Command         []string       `yaml:"command,omitempty"`
	Pull            string         `yaml:"pull,omitempty"`
	Memory          string         `yaml:"memory,omitempty"`
	CPUs            float64        `yaml:"cpus,omitempty"`
	Devices         []DeviceConfig `yaml:"devices,omitempty"`
}

func (c *ToolConfig) SetBaseDir(baseDir string) {
	if c.MountSocketPath.Raw != "" {
		c.MountSocketPath.BaseDir = baseDir
	}
	if c.MountCderunPath.Raw != "" {
		c.MountCderunPath.BaseDir = baseDir
	}
	for i := range c.Mounts {
		c.Mounts[i].SetBaseDir(baseDir)
	}
	for i := range c.Devices {
		c.Devices[i].SetBaseDir(baseDir)
	}
}

type ToolsConfig map[string]ToolConfig

var (
	systemConfigDir = "/etc/cderun"
	runConfigDir    = "/run/cderun"
)

// SetRunConfigDirForTest sets the directory for run configuration (used for testing).
func SetRunConfigDirForTest(path string) func() {
	restoreDir := runConfigDir
	runConfigDir = path
	return func() { runConfigDir = restoreDir }
}

// SetSystemConfigDirForTest sets the directory for system configuration (used for testing).
func SetSystemConfigDirForTest(path string) func() {
	restoreDir := systemConfigDir
	systemConfigDir = path
	return func() { systemConfigDir = restoreDir }
}

// FindConfigs searches for config files in hierarchical order.
// Priority: Current Dir > Parent Dirs > Home Dir > System Paths.
// The returned list is ordered by priority (highest first).
func FindConfigs(filename string) []string {
	var paths []string
	curr, err := os.Getwd()
	if err == nil {
		for {
			p := filepath.Join(curr, filename)
			if _, err := os.Stat(p); err == nil {
				if abs, err := filepath.Abs(p); err == nil {
					paths = append(paths, abs)
				} else {
					paths = append(paths, p)
				}
			}
			parent := filepath.Dir(curr)
			if parent == curr {
				break
			}
			curr = parent
		}
	}

	// Add home dir
	if home, err := os.UserHomeDir(); err == nil {
		p := filepath.Join(home, ".config", "cderun", filename)
		if _, err := os.Stat(p); err == nil {
			if abs, err := filepath.Abs(p); err == nil {
				paths = append(paths, abs)
			} else {
				paths = append(paths, p)
			}
		}
	}

	// Add system path
	p := filepath.Join(systemConfigDir, filename)
	if _, err := os.Stat(p); err == nil {
		paths = append(paths, p)
	}

	// Add run directory path (used for nested execution config injection)
	p = filepath.Join(runConfigDir, filename)
	if _, err := os.Stat(p); err == nil {
		paths = append(paths, p)
	}

	return paths
}

// LoadCDERunConfig searches for .cderun.yaml in hierarchical locations and merges them.
func LoadCDERunConfig() (*CDERunConfig, []string, error) {
	paths := FindConfigs(".cderun.yaml")
	if len(paths) == 0 {
		return nil, nil, nil
	}

	var merged CDERunConfig
	var loadedPaths []string
	// Merge from lowest priority to highest (reverse of paths)
	for i := len(paths) - 1; i >= 0; i-- {
		path := paths[i]
		baseDir := filepath.Dir(path)
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read config file %s: %w", path, err)
		}

		var layer CDERunConfig
		if err := yaml.Unmarshal(data, &layer); err != nil {
			return nil, nil, fmt.Errorf("failed to unmarshal config file %s: %w", path, err)
		}

		// Assign baseDir for relative path resolution later
		// Only for non-empty paths to avoid creating non-zero values for mergo
		layer.SetBaseDir(baseDir)

		if err := mergo.Merge(&merged, &layer, mergo.WithOverride); err != nil {
			return nil, nil, fmt.Errorf("failed to merge config from %s: %w", path, err)
		}

		loadedPaths = append(loadedPaths, path)
	}

	// Reverse loadedPaths to match priority (highest first)
	for i, j := 0, len(loadedPaths)-1; i < j; i, j = i+1, j-1 {
		loadedPaths[i], loadedPaths[j] = loadedPaths[j], loadedPaths[i]
	}

	return &merged, loadedPaths, nil
}

// LoadToolsConfig searches for .tools.yaml in hierarchical locations and merges them.
func LoadToolsConfig() (ToolsConfig, []string, error) {
	paths := FindConfigs(".tools.yaml")
	if len(paths) == 0 {
		return nil, nil, nil
	}

	merged := make(ToolsConfig)
	var loadedPaths []string
	// Merge from lowest priority to highest (reverse of paths)
	for i := len(paths) - 1; i >= 0; i-- {
		path := paths[i]
		baseDir := filepath.Dir(path)
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read tools file %s: %w", path, err)
		}

		var layer ToolsConfig
		if err := yaml.Unmarshal(data, &layer); err != nil {
			return nil, nil, fmt.Errorf("failed to unmarshal tools file %s: %w", path, err)
		}

		// Merge ToolsConfig to ensure deep merge of ToolConfig
		for k, v := range layer {
			// Assign baseDir for relative path resolution later
			// Only for non-empty paths to avoid creating non-zero values for mergo
			v.SetBaseDir(baseDir)

			if existing, ok := merged[k]; ok {
				if err := mergo.Merge(&existing, &v, mergo.WithOverride); err != nil {
					return nil, nil, fmt.Errorf("failed to merge tool config for %s from %s: %w", k, path, err)
				}
				merged[k] = existing
			} else {
				merged[k] = v
			}
		}
		loadedPaths = append(loadedPaths, path)
	}

	// Reverse loadedPaths to match priority (highest first)
	for i, j := 0, len(loadedPaths)-1; i < j; i, j = i+1, j-1 {
		loadedPaths[i], loadedPaths[j] = loadedPaths[j], loadedPaths[i]
	}

	return merged, loadedPaths, nil
}
