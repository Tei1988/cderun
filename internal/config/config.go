package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
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
	TTY             *bool          `yaml:"tty,omitempty"`
	Interactive     *bool          `yaml:"interactive,omitempty"`
	Network         string         `yaml:"network,omitempty"`
	Remove          *bool          `yaml:"remove,omitempty"`
	StrictEnv       *bool          `yaml:"strictEnv,omitempty"`
	Workdir         string         `yaml:"workdir,omitempty"`
	MountCderun     *bool          `yaml:"mountCderun,omitempty"`
	MountCderunPath ConfigPath     `yaml:"mountCderunPath,omitempty"`
	MountSocket     *bool          `yaml:"mountSocket,omitempty"`
	MountSocketPath ConfigPath     `yaml:"mountSocketPath,omitempty"`
	MountTools      []string       `yaml:"mountTools,omitempty"`
	MountAllTools   *bool          `yaml:"mountAllTools,omitempty"`
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
	Level     string `yaml:"level"`
	Format    string `yaml:"format"`
	Timestamp *bool  `yaml:"timestamp"`
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
	MountTools      []string       `yaml:"mountTools,omitempty"`
	MountAllTools   *bool          `yaml:"mountAllTools,omitempty"`
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

// FileSystem defines the interface for filesystem operations.
type FileSystem interface {
	Getwd() (string, error)
	Stat(name string) (os.FileInfo, error)
	ReadFile(name string) ([]byte, error)
	UserHomeDir() (string, error)
	Executable() (string, error)
	Getenv(key string) string
	TempDir() string
	MkdirAll(path string, perm os.FileMode) error
	WriteFile(filename string, data []byte, perm os.FileMode) error
	RemoveAll(path string) error
}

// RealFileSystem implements FileSystem using standard os and filepath.
type RealFileSystem struct{}

func (RealFileSystem) Getwd() (string, error)                { return os.Getwd() }
func (RealFileSystem) Stat(name string) (os.FileInfo, error) { return os.Stat(name) }
func (RealFileSystem) ReadFile(name string) ([]byte, error)  { return os.ReadFile(name) } // nolint:gosec
func (RealFileSystem) UserHomeDir() (string, error)          { return os.UserHomeDir() }
func (RealFileSystem) Executable() (string, error)           { return os.Executable() }
func (RealFileSystem) Getenv(key string) string              { return os.Getenv(key) }
func (RealFileSystem) TempDir() string                       { return os.TempDir() }
func (RealFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (RealFileSystem) WriteFile(filename string, data []byte, perm os.FileMode) error {
	return os.WriteFile(filename, data, perm)
}
func (RealFileSystem) RemoveAll(path string) error { return os.RemoveAll(path) }

// ConfigLoader handles finding and loading configuration files.
type ConfigLoader struct {
	fs              FileSystem
	systemConfigDir string
	runConfigDir    string
}

// NewConfigLoader creates a new ConfigLoader with a RealFileSystem.
func NewConfigLoader() *ConfigLoader {
	return &ConfigLoader{
		fs:              RealFileSystem{},
		systemConfigDir: "/etc/cderun",
		runConfigDir:    "/run/cderun",
	}
}

// NewConfigLoaderWithFS creates a new ConfigLoader with the specified FileSystem and current default directories.
// Note: This depends on defaultLoader being initialized (which happens at the package level).
func NewConfigLoaderWithFS(fs FileSystem) *ConfigLoader {
	return &ConfigLoader{
		fs:              fs,
		systemConfigDir: defaultLoader.systemConfigDir,
		runConfigDir:    defaultLoader.runConfigDir,
	}
}

var defaultLoader = NewConfigLoader()

// SetRunConfigDirForTest sets the directory for run configuration (used for testing).
func SetRunConfigDirForTest(path string) func() {
	restoreDir := defaultLoader.runConfigDir
	defaultLoader.runConfigDir = path
	return func() { defaultLoader.runConfigDir = restoreDir }
}

// SetSystemConfigDirForTest sets the directory for system configuration (used for testing).
func SetSystemConfigDirForTest(path string) func() {
	restoreDir := defaultLoader.systemConfigDir
	defaultLoader.systemConfigDir = path
	return func() { defaultLoader.systemConfigDir = restoreDir }
}

// FindConfigs searches for config files in hierarchical order.
// Priority: Current Dir > Parent Dirs > Home Dir > System Paths.
// The returned list is ordered by priority (highest first).
func FindConfigs(filename string) []string {
	return defaultLoader.FindConfigs(filename)
}

// FindConfigs searches for config files in hierarchical order using the loader's filesystem and directories.
func (l *ConfigLoader) FindConfigs(filename string) []string {
	var paths []string
	curr, err := l.fs.Getwd()
	if err == nil {
		for {
			p := filepath.Join(curr, filename)
			if _, err := l.fs.Stat(p); err == nil {
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
	if home, err := l.fs.UserHomeDir(); err == nil {
		p := filepath.Join(home, ".config", "cderun", filename)
		if _, err := l.fs.Stat(p); err == nil {
			if abs, err := filepath.Abs(p); err == nil {
				paths = append(paths, abs)
			} else {
				paths = append(paths, p)
			}
		}
	}

	// Add system path
	p := filepath.Join(l.systemConfigDir, filename)
	if _, err := l.fs.Stat(p); err == nil {
		paths = append(paths, p)
	}

	// Add run directory path (used for nested execution config injection)
	p = filepath.Join(l.runConfigDir, filename)
	if _, err := l.fs.Stat(p); err == nil {
		paths = append(paths, p)
	}

	return paths
}

// unmarshalStrict unmarshals YAML data into v with KnownFields enabled.
func unmarshalStrict(data []byte, v any) error {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	err := dec.Decode(v)
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

// LoadCDERunConfig searches for .cderun.yaml in hierarchical locations and merges them.
func LoadCDERunConfig() (*CDERunConfig, []string, error) {
	return defaultLoader.LoadCDERunConfig()
}

// LoadCDERunConfig searches for .cderun.yaml in hierarchical locations and merges them using the loader's filesystem.
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
		data, err := l.fs.ReadFile(path)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read config file %s: %w", path, err)
		}

		var layer CDERunConfig
		if err := unmarshalStrict(data, &layer); err != nil {
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

// LoadToolsConfig searches for .tools.yaml in hierarchical locations and merges them using the loader's filesystem.
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
		data, err := l.fs.ReadFile(path)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read tools file %s: %w", path, err)
		}

		var layer ToolsConfig
		if err := unmarshalStrict(data, &layer); err != nil {
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
