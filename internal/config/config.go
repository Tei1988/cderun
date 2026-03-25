package config

import (
	"sync"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

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

func (c CDERunConfig) DeepCopy() CDERunConfig {
	res := c
	res.Defaults = c.Defaults.DeepCopy()
	res.Logging = c.Logging.DeepCopy()
	if c.HostContext != nil {
		copy := c.HostContext.DeepCopy()
		res.HostContext = &copy
	}
	return res
}

func (c *CDERunConfig) SetBaseDir(baseDir string) error {
	if c.SocketPath.Raw != "" {
		c.SocketPath.BaseDir = baseDir
	}
	c.Defaults.SetBaseDir(baseDir)
	if c.HostContext != nil {
		for i := range c.HostContext.Mounts {
			s, err := ResolvePath(c.HostContext.Mounts[i].Source, baseDir, nil)
			if err != nil {
				return err
			}
			c.HostContext.Mounts[i].Source = s
		}
	}
	return nil
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
	Pull            string         `yaml:"pull,omitempty"`
	PullMaxRetries  *int           `yaml:"pullMaxRetries,omitempty"`
	PullBackoffBase string         `yaml:"pullBackoffBase,omitempty"`
	Memory          string         `yaml:"memory,omitempty"`
	CPUs            *float64       `yaml:"cpus,omitempty"`
	HangTimeout     string         `yaml:"hangTimeout,omitempty"`
	DryRun          *bool          `yaml:"dryRun,omitempty"`
	DryRunFormat    string         `yaml:"dryRunFormat,omitempty"`
	Diagnosis       *bool          `yaml:"diagnosis,omitempty"`
	DiagnosisFormat string         `yaml:"diagnosisFormat,omitempty"`
	Devices         []DeviceConfig `yaml:"devices,omitempty"`
	Mounts          []MountConfig  `yaml:"mounts,omitempty"`
	Env             []string       `yaml:"env,omitempty"`
}

func (d ConfigDefaults) DeepCopy() ConfigDefaults {
	res := d
	res.TTY = copyBoolPtr(d.TTY)
	res.Interactive = copyBoolPtr(d.Interactive)
	res.Remove = copyBoolPtr(d.Remove)
	res.StrictEnv = copyBoolPtr(d.StrictEnv)
	res.MountSocket = copyBoolPtr(d.MountSocket)
	res.MountCderun = copyBoolPtr(d.MountCderun)
	res.MountAllTools = copyBoolPtr(d.MountAllTools)
	res.PublishAll = copyBoolPtr(d.PublishAll)
	res.Privileged = copyBoolPtr(d.Privileged)
	res.DryRun = copyBoolPtr(d.DryRun)
	res.Diagnosis = copyBoolPtr(d.Diagnosis)
	res.CPUs = copyFloat64Ptr(d.CPUs)
	res.PullMaxRetries = copyIntPtr(d.PullMaxRetries)

	res.MountTools = copyStringSlice(d.MountTools)
	res.Ports = copyStringSlice(d.Ports)
	res.Expose = copyStringSlice(d.Expose)
	res.DNS = copyStringSlice(d.DNS)
	res.AddHosts = copyStringSlice(d.AddHosts)
	res.CapAdd = copyStringSlice(d.CapAdd)
	res.CapDrop = copyStringSlice(d.CapDrop)
	res.Entrypoint = copyStringSlice(d.Entrypoint)
	res.Env = copyStringSlice(d.Env)

	if d.Devices != nil {
		res.Devices = make([]DeviceConfig, len(d.Devices))
		for i, v := range d.Devices {
			res.Devices[i] = v.DeepCopy()
		}
	}
	if d.Mounts != nil {
		res.Mounts = make([]MountConfig, len(d.Mounts))
		for i, v := range d.Mounts {
			res.Mounts[i] = v.DeepCopy()
		}
	}
	return res
}

func (d *ConfigDefaults) SetBaseDir(baseDir string) {
	d.MountCderunPath.BaseDir = baseDir
	d.MountSocketPath.BaseDir = baseDir
	for i := range d.Mounts {
		d.Mounts[i].SetBaseDir(baseDir)
	}
	for i := range d.Devices {
		d.Devices[i].SetBaseDir(baseDir)
	}
}

type LoggingConfig struct {
	Level     string `yaml:"level,omitempty"`
	Format    string `yaml:"format,omitempty"`
	Timestamp *bool  `yaml:"timestamp,omitempty"`
}

func (l LoggingConfig) DeepCopy() LoggingConfig {
	res := l
	res.Timestamp = copyBoolPtr(l.Timestamp)
	return res
}

type HostContext struct {
	Level       int            `yaml:"level"`
	SnapshotDir string         `yaml:"snapshotDir"`
	BinPath     string         `yaml:"binPath"`
	WorkingDir  string         `yaml:"workingDir"`
	HomeDir     string         `yaml:"homeDir"`
	Mounts      []MountMapping `yaml:"mounts"`
}

func (h HostContext) DeepCopy() HostContext {
	res := h
	if h.Mounts != nil {
		res.Mounts = make([]MountMapping, len(h.Mounts))
		copy(res.Mounts, h.Mounts)
	}
	return res
}

type MountMapping struct {
	Source string `yaml:"source"`
	Target string `yaml:"target"`
	Level  int    `yaml:"level"`
}

type ToolConfig struct {
	Image           string         `yaml:"image"`
	TTY             *bool          `yaml:"tty,omitempty"`
	Interactive     *bool          `yaml:"interactive,omitempty"`
	Network         string         `yaml:"network,omitempty"`
	Remove          *bool          `yaml:"remove,omitempty"`
	StrictEnv       *bool          `yaml:"strictEnv,omitempty"`
	Workdir         string         `yaml:"workdir,omitempty"`
	MountSocket     *bool          `yaml:"mountSocket,omitempty"`
	MountSocketPath ConfigPath     `yaml:"mountSocketPath,omitempty"`
	MountCderun     *bool          `yaml:"mountCderun,omitempty"`
	MountCderunPath ConfigPath     `yaml:"mountCderunPath,omitempty"`
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
	Pull            string         `yaml:"pull,omitempty"`
	PullMaxRetries  *int           `yaml:"pullMaxRetries,omitempty"`
	PullBackoffBase string         `yaml:"pullBackoffBase,omitempty"`
	Memory          string         `yaml:"memory,omitempty"`
	CPUs            *float64       `yaml:"cpus,omitempty"`
	HangTimeout     string         `yaml:"hangTimeout,omitempty"`
	LogLevel        string         `yaml:"logLevel,omitempty"`
	LogFormat       string         `yaml:"logFormat,omitempty"`
	LogTimestamp    *bool          `yaml:"logTimestamp,omitempty"`
	DryRun          *bool          `yaml:"dryRun,omitempty"`
	DryRunFormat    string         `yaml:"dryRunFormat,omitempty"`
	Diagnosis       *bool          `yaml:"diagnosis,omitempty"`
	DiagnosisFormat string         `yaml:"diagnosisFormat,omitempty"`
	Devices         []DeviceConfig `yaml:"devices,omitempty"`
	Mounts          []MountConfig  `yaml:"mounts,omitempty"`
	Env             []string       `yaml:"env,omitempty"`
}

func (t ToolConfig) DeepCopy() ToolConfig {
	res := t
	res.TTY = copyBoolPtr(t.TTY)
	res.Interactive = copyBoolPtr(t.Interactive)
	res.Remove = copyBoolPtr(t.Remove)
	res.StrictEnv = copyBoolPtr(t.StrictEnv)
	res.MountSocket = copyBoolPtr(t.MountSocket)
	res.MountCderun = copyBoolPtr(t.MountCderun)
	res.MountAllTools = copyBoolPtr(t.MountAllTools)
	res.PublishAll = copyBoolPtr(t.PublishAll)
	res.Privileged = copyBoolPtr(t.Privileged)
	res.LogTimestamp = copyBoolPtr(t.LogTimestamp)
	res.DryRun = copyBoolPtr(t.DryRun)
	res.Diagnosis = copyBoolPtr(t.Diagnosis)
	res.CPUs = copyFloat64Ptr(t.CPUs)
	res.PullMaxRetries = copyIntPtr(t.PullMaxRetries)

	res.MountTools = copyStringSlice(t.MountTools)
	res.Ports = copyStringSlice(t.Ports)
	res.Expose = copyStringSlice(t.Expose)
	res.DNS = copyStringSlice(t.DNS)
	res.AddHosts = copyStringSlice(t.AddHosts)
	res.CapAdd = copyStringSlice(t.CapAdd)
	res.CapDrop = copyStringSlice(t.CapDrop)
	res.Entrypoint = copyStringSlice(t.Entrypoint)
	res.Env = copyStringSlice(t.Env)

	if t.Devices != nil {
		res.Devices = make([]DeviceConfig, len(t.Devices))
		for i, v := range t.Devices {
			res.Devices[i] = v.DeepCopy()
		}
	}
	if t.Mounts != nil {
		res.Mounts = make([]MountConfig, len(t.Mounts))
		for i, v := range t.Mounts {
			res.Mounts[i] = v.DeepCopy()
		}
	}
	return res
}

func (t *ToolConfig) SetBaseDir(baseDir string) {
	t.MountCderunPath.BaseDir = baseDir
	t.MountSocketPath.BaseDir = baseDir
	for i := range t.Mounts {
		t.Mounts[i].SetBaseDir(baseDir)
	}
	for i := range t.Devices {
		t.Devices[i].SetBaseDir(baseDir)
	}
}

type ToolsConfig map[string]ToolConfig

func (t ToolsConfig) DeepCopy() ToolsConfig {
	if t == nil {
		return nil
	}
	res := make(ToolsConfig, len(t))
	for k, v := range t {
		res[k] = v.DeepCopy()
	}
	return res
}

func copyBoolPtr(b *bool) *bool {
	if b == nil {
		return nil
	}
	res := new(bool)
	*res = *b
	return res
}

func copyStringSlice(s []string) []string {
	if s == nil {
		return nil
	}
	res := make([]string, len(s))
	copy(res, s)
	return res
}

func expandHome(p string, fs FileSystem) (string, error) {
	if strings.HasPrefix(p, "~/") || p == "~" {
		home, err := fs.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		if p == "~" {
			return home, nil
		}
		return filepath.Join(home, p[2:]), nil
	}
	return p, nil
}

// FileSystem defines the interface for filesystem operations.
type FileSystem interface {
	Getwd() (string, error)
	Stat(name string) (os.FileInfo, error)
	ReadFile(name string) ([]byte, error)
	UserHomeDir() (string, error)
	Executable() (string, error)
	Getenv(key string) string
	LookupEnv(key string) (string, bool)
	TempDir() string
	MkdirAll(path string, perm os.FileMode) error
	WriteFile(filename string, data []byte, perm os.FileMode) error
	RemoveAll(path string) error
	Abs(path string) (string, error)
}

// RealFileSystem implements FileSystem using standard os and filepath.
type RealFileSystem struct{}

func (RealFileSystem) Getwd() (string, error)                { return os.Getwd() }
func (RealFileSystem) Stat(name string) (os.FileInfo, error) { return os.Stat(name) }
func (RealFileSystem) ReadFile(name string) ([]byte, error)  { return os.ReadFile(name) } //nolint:gosec
func (RealFileSystem) UserHomeDir() (string, error)          { return os.UserHomeDir() }
func (RealFileSystem) Executable() (string, error)           { return os.Executable() }
func (RealFileSystem) Getenv(key string) string              { return os.Getenv(key) }
func (RealFileSystem) LookupEnv(key string) (string, bool)   { return os.LookupEnv(key) }
func (RealFileSystem) TempDir() string                       { return os.TempDir() }
func (RealFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (RealFileSystem) WriteFile(filename string, data []byte, perm os.FileMode) error {
	return os.WriteFile(filename, data, perm)
}
func (RealFileSystem) RemoveAll(path string) error { return os.RemoveAll(path) }
func (RealFileSystem) Abs(path string) (string, error) { return filepath.Abs(path) }

// ConfigLoader handles finding and loading configuration files.
type statResult struct {
	info os.FileInfo
	err  error
}

// ConfigLoader handles finding and loading configuration files.
type ConfigLoader struct {
	fs              FileSystem
	systemConfigDir string
	runConfigDir    string
	statCache       map[string]statResult
	mu              sync.RWMutex
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
func FindConfigs(filename string) []string {
	return defaultLoader.FindConfigs(filename)
}

func (l *ConfigLoader) cachedStat(name string) (os.FileInfo, error) {
	l.mu.RLock()
	if l.statCache != nil {
		if res, ok := l.statCache[name]; ok {
			l.mu.RUnlock()
			return res.info, res.err
		}
	}
	l.mu.RUnlock()

	l.mu.Lock()
	defer l.mu.Unlock()
	if l.statCache == nil {
		l.statCache = make(map[string]statResult)
	}
	// Double-check after acquiring write lock
	if res, ok := l.statCache[name]; ok {
		return res.info, res.err
	}

	info, err := l.fs.Stat(name)
	l.statCache[name] = statResult{info: info, err: err}
	return info, err
}

// FindConfigs searches for config files in hierarchical order using the loader's filesystem and directories.
func (l *ConfigLoader) FindConfigs(filename string) []string {
	var paths []string
	curr, err := l.fs.Getwd()
	if err == nil {
		for {
			p := filepath.Join(curr, filename)
			if _, err := l.cachedStat(p); err == nil {
				if abs, err := l.fs.Abs(p); err == nil {
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
		if _, err := l.cachedStat(p); err == nil {
			if abs, err := l.fs.Abs(p); err == nil {
				paths = append(paths, abs)
			} else {
				paths = append(paths, p)
			}
		}
	}

	// Add system path
	p := filepath.Join(l.systemConfigDir, filename)
	if _, err := l.cachedStat(p); err == nil {
		paths = append(paths, p)
	}

	// Add run directory path (used for nested execution config injection)
	p = filepath.Join(l.runConfigDir, filename)
	if _, err := l.cachedStat(p); err == nil {
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

		if err := layer.SetBaseDir(baseDir); err != nil {
			return nil, nil, fmt.Errorf("failed to set base directory for %s: %w", path, err)
		}

		if err := mergo.Merge(&merged, &layer, mergo.WithOverride); err != nil {
			return nil, nil, fmt.Errorf("failed to merge config from %s: %w", path, err)
		}

		loadedPaths = append(loadedPaths, path)
	}

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

		for k, v := range layer {
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

	for i, j := 0, len(loadedPaths)-1; i < j; i, j = i+1, j-1 {
		loadedPaths[i], loadedPaths[j] = loadedPaths[j], loadedPaths[i]
	}

	return merged, loadedPaths, nil
}

// LoadCDERunConfigFromPath loads .cderun.yaml from a specific path.
func (l *ConfigLoader) LoadCDERunConfigFromPath(path string) (*CDERunConfig, []string, error) {
	expanded, err := expandHome(path, l.fs)
	if err != nil {
		return nil, nil, err
	}
	path = expanded
	absPath, err := l.fs.Abs(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get absolute path for %s: %w", path, err)
	}

	data, err := l.fs.ReadFile(absPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read config file %s: %w", absPath, err)
	}

	var cfg CDERunConfig
	if err := unmarshalStrict(data, &cfg); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal config file %s: %w", absPath, err)
	}

	baseDir := filepath.Dir(absPath)
	if err := cfg.SetBaseDir(baseDir); err != nil {
		return nil, nil, fmt.Errorf("failed to set base directory for %s: %w", absPath, err)
	}

	return &cfg, []string{absPath}, nil
}

// LoadToolsConfigFromPath loads .tools.yaml from a specific path.
func (l *ConfigLoader) LoadToolsConfigFromPath(path string) (ToolsConfig, []string, error) {
	expanded, err := expandHome(path, l.fs)
	if err != nil {
		return nil, nil, err
	}
	path = expanded
	absPath, err := l.fs.Abs(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get absolute path for %s: %w", path, err)
	}

	data, err := l.fs.ReadFile(absPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read tools file %s: %w", absPath, err)
	}

	cfg := make(ToolsConfig)
	if err := unmarshalStrict(data, &cfg); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal tools file %s: %w", absPath, err)
	}

	baseDir := filepath.Dir(absPath)
	for k, v := range cfg {
		v.SetBaseDir(baseDir)
		cfg[k] = v
	}

	return cfg, []string{absPath}, nil
}

func copyFloat64Ptr(f *float64) *float64 {
	if f == nil {
		return nil
	}
	res := *f
	return &res
}

func copyIntPtr(i *int) *int {
	if i == nil {
		return nil
	}
	res := *i
	return &res
}
