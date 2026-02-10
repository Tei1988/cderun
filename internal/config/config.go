package config

import (
	"fmt"
	"os"
	"path/filepath"

	"dario.cat/mergo"
	"gopkg.in/yaml.v3"
)

type CDERunConfig struct {
	Runtime     string         `yaml:"runtime"`
	SocketPath  ConfigPath     `yaml:"socketPath"`
	Defaults    ConfigDefaults `yaml:"defaults"`
	Logging     LoggingConfig  `yaml:"logging"`
	HostContext *HostContext   `yaml:"hostContext,omitempty"`
}

type HostContext struct {
	BinPath     string         `yaml:"binPath"`
	SnapshotDir string         `yaml:"snapshotDir"`
	WorkingDir  string         `yaml:"workingDir"`
	Level       int            `yaml:"level"`
	Mounts      []MountMapping `yaml:"mounts"`
}

type MountMapping struct {
	Source string `yaml:"source"`
	Target string `yaml:"target"`
	Level  int    `yaml:"level"`
}

func (c *CDERunConfig) SetBaseDir(baseDir string) {
	if c.SocketPath.Raw != "" {
		c.SocketPath.BaseDir = baseDir
	}
	c.Defaults.SetBaseDir(baseDir)
}

type ConfigDefaults struct {
	TTY             *bool          `yaml:"tty"`
	Interactive     *bool          `yaml:"interactive"`
	Network         string         `yaml:"network"`
	Remove          *bool          `yaml:"remove"`
	StrictEnv       *bool          `yaml:"strictEnv"`
	MountCderun     *bool          `yaml:"mountCderun"`
	MountCderunPath ConfigPath     `yaml:"mountCderunPath"`
	MountSocket     *bool          `yaml:"mountSocket"`
	MountSocketPath ConfigPath     `yaml:"mountSocketPath"`
	Ports           []string       `yaml:"ports"`
	PublishAll      *bool          `yaml:"publishAll"`
	Expose          []string       `yaml:"expose"`
	Hostname        string         `yaml:"hostname"`
	DNS             []string       `yaml:"dns"`
	AddHosts        []string       `yaml:"addHosts"`
	User            string         `yaml:"user"`
	Privileged      *bool          `yaml:"privileged"`
	CapAdd          []string       `yaml:"capAdd"`
	CapDrop         []string       `yaml:"capDrop"`
	Entrypoint      []string       `yaml:"entrypoint"`
	Command         []string       `yaml:"command"`
	Pull            string         `yaml:"pull"`
	Memory          string         `yaml:"memory"`
	CPUs            float64        `yaml:"cpus"`
	Mounts          []MountConfig  `yaml:"mounts"`
	Devices         []DeviceConfig `yaml:"devices"`
	Env             []string       `yaml:"env"`
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
	Level     string `yaml:"level"`
	Format    string `yaml:"format"`
	Timestamp *bool  `yaml:"timestamp"`
}

type ToolConfig struct {
	Image           string         `yaml:"image"`
	TTY             *bool          `yaml:"tty"`
	Interactive     *bool          `yaml:"interactive"`
	Network         string         `yaml:"network"`
	Remove          *bool          `yaml:"remove"`
	StrictEnv       *bool          `yaml:"strictEnv"`
	Mounts          []MountConfig  `yaml:"mounts"`
	Env             []string       `yaml:"env"`
	Workdir         string         `yaml:"workdir"`
	MountCderun     *bool          `yaml:"mountCderun"`
	MountCderunPath ConfigPath     `yaml:"mountCderunPath"`
	MountSocket     *bool          `yaml:"mountSocket"`
	MountSocketPath ConfigPath     `yaml:"mountSocketPath"`
	Ports           []string       `yaml:"ports"`
	PublishAll      *bool          `yaml:"publishAll"`
	Expose          []string       `yaml:"expose"`
	Hostname        string         `yaml:"hostname"`
	DNS             []string       `yaml:"dns"`
	AddHosts        []string       `yaml:"addHosts"`
	User            string         `yaml:"user"`
	Privileged      *bool          `yaml:"privileged"`
	CapAdd          []string       `yaml:"capAdd"`
	CapDrop         []string       `yaml:"capDrop"`
	Entrypoint      []string       `yaml:"entrypoint"`
	Command         []string       `yaml:"command"`
	Pull            string         `yaml:"pull"`
	Memory          string         `yaml:"memory"`
	CPUs            float64        `yaml:"cpus"`
	Devices         []DeviceConfig `yaml:"devices"`
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

// FileSystem defines the operations needed for configuration discovery and loading.
type FileSystem interface {
	Getwd() (string, error)
	UserHomeDir() (string, error)
	Stat(name string) (os.FileInfo, error)
	ReadFile(name string) ([]byte, error)
}

// OSFileSystem implements FileSystem using the standard os package.
type OSFileSystem struct{}

func (f OSFileSystem) Getwd() (string, error)                { return os.Getwd() }
func (f OSFileSystem) UserHomeDir() (string, error)          { return os.UserHomeDir() }
func (f OSFileSystem) Stat(name string) (os.FileInfo, error) { return os.Stat(name) }
func (f OSFileSystem) ReadFile(name string) ([]byte, error)  { return os.ReadFile(name) }

// ConfigLoader handles configuration discovery and loading.
type ConfigLoader struct {
	FS              FileSystem
	SystemConfigDir string
	RunConfigDir    string
}

// NewConfigLoader creates a new ConfigLoader with default OS-based dependencies.
func NewConfigLoader() *ConfigLoader {
	return &ConfigLoader{
		FS:              OSFileSystem{},
		SystemConfigDir: "/etc/cderun",
		RunConfigDir:    "/run/cderun",
	}
}

var defaultLoader = NewConfigLoader()

// SetRunConfigDirForTest sets the directory for run configuration (used for testing).
func SetRunConfigDirForTest(path string) func() {
	restoreDir := defaultLoader.RunConfigDir
	defaultLoader.RunConfigDir = path
	return func() { defaultLoader.RunConfigDir = restoreDir }
}

// SetSystemConfigDirForTest sets the directory for system configuration (used for testing).
func SetSystemConfigDirForTest(path string) func() {
	restoreDir := defaultLoader.SystemConfigDir
	defaultLoader.SystemConfigDir = path
	return func() { defaultLoader.SystemConfigDir = restoreDir }
}

// FindConfigs searches for config files in hierarchical order.
// Priority: Current Dir > Parent Dirs > Home Dir > System Paths.
// The returned list is ordered by priority (highest first).
func FindConfigs(filename string) []string {
	return defaultLoader.FindConfigs(filename)
}

// FindConfigs searches for config files in hierarchical order.
func (l *ConfigLoader) FindConfigs(filename string) []string {
	var paths []string
	curr, err := l.FS.Getwd()
	if err == nil {
		for {
			p := filepath.Join(curr, filename)
			if _, err := l.FS.Stat(p); err == nil {
				// We still use filepath.Abs which is fine as it doesn't hit FS
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
	if home, err := l.FS.UserHomeDir(); err == nil {
		p := filepath.Join(home, ".config", "cderun", filename)
		if _, err := l.FS.Stat(p); err == nil {
			if abs, err := filepath.Abs(p); err == nil {
				paths = append(paths, abs)
			} else {
				paths = append(paths, p)
			}
		}
	}

	// Add system path
	p := filepath.Join(l.SystemConfigDir, filename)
	if _, err := l.FS.Stat(p); err == nil {
		paths = append(paths, p)
	}

	// Add run directory path (used for nested execution config injection)
	p = filepath.Join(l.RunConfigDir, filename)
	if _, err := l.FS.Stat(p); err == nil {
		paths = append(paths, p)
	}

	return paths
}

// LoadCDERunConfig searches for .cderun.yaml in hierarchical locations and merges them.
func LoadCDERunConfig() (*CDERunConfig, []string, error) {
	return defaultLoader.LoadCDERunConfig()
}

// LoadCDERunConfig searches for .cderun.yaml in hierarchical locations and merges them.
func (l *ConfigLoader) LoadCDERunConfig() (*CDERunConfig, []string, error) {
	paths := l.FindConfigs(".cderun.yaml")
	if len(paths) == 0 {
		return nil, nil, nil
	}

	var merged CDERunConfig
	var loadedPaths []string
	// Merge from lowest priority to highest (reverse of paths)
	for i := len(paths) - 1; i >= 0; i-- {
		path := paths[i]
		baseDir := filepath.Dir(path)
		data, err := l.FS.ReadFile(path)
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
	return defaultLoader.LoadToolsConfig()
}

// LoadToolsConfig searches for .tools.yaml in hierarchical locations and merges them.
func (l *ConfigLoader) LoadToolsConfig() (ToolsConfig, []string, error) {
	paths := l.FindConfigs(".tools.yaml")
	if len(paths) == 0 {
		return nil, nil, nil
	}

	merged := make(ToolsConfig)
	var loadedPaths []string
	// Merge from lowest priority to highest (reverse of paths)
	for i := len(paths) - 1; i >= 0; i-- {
		path := paths[i]
		baseDir := filepath.Dir(path)
		data, err := l.FS.ReadFile(path)
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
