package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"dario.cat/mergo"
	"gopkg.in/yaml.v3"
)

type CDERunConfig struct {
	Runtime    string         `yaml:"runtime"`
	SocketPath string         `yaml:"socketPath"`
	Defaults   ConfigDefaults `yaml:"defaults"`
	Logging    LoggingConfig  `yaml:"logging"`
}

type ConfigDefaults struct {
	TTY             *bool  `yaml:"tty"`
	Interactive     *bool  `yaml:"interactive"`
	Network         string `yaml:"network"`
	Remove          *bool  `yaml:"remove"`
	StrictEnv       *bool  `yaml:"strictEnv"`
	MountCderun     *bool  `yaml:"mountCderun"`
	MountSocket     *bool  `yaml:"mountSocket"`
	MountSocketPath string `yaml:"mountSocketPath"`
	DryRun          *bool  `yaml:"dryRun"`
	DryRunFormat    string `yaml:"dryRunFormat"`
	// New fields
	Ports      []string `yaml:"ports"`
	PublishAll *bool    `yaml:"publishAll"`
	Expose     []string `yaml:"expose"`
	Hostname   string   `yaml:"hostname"`
	DNS        []string `yaml:"dns"`
	AddHosts   []string `yaml:"addHosts"`
	User       string   `yaml:"user"`
	Privileged *bool    `yaml:"privileged"`
	CapAdd     []string `yaml:"capAdd"`
	CapDrop    []string `yaml:"capDrop"`
	Entrypoint []string `yaml:"entrypoint"`
	Pull       string   `yaml:"pull"`
	Memory     string   `yaml:"memory"`
	CPUs       float64  `yaml:"cpus"`
	Tmpfs      []string `yaml:"tmpfs"`
	Devices    []string `yaml:"devices"`
}

type LoggingConfig struct {
	Level     string                `yaml:"level"`
	File      string                `yaml:"file"`
	Format    string                `yaml:"format"`
	Timestamp *bool                 `yaml:"timestamp"`
	Rotation  LoggingRotationConfig `yaml:"rotation"`
	Tee       *bool                 `yaml:"tee"`
}

type LoggingRotationConfig struct {
	MaxSize    string `yaml:"maxSize"`
	MaxAge     string `yaml:"maxAge"`
	MaxBackups int    `yaml:"maxBackups"`
	Compress   bool   `yaml:"compress"`
}

type ToolConfig struct {
	Image           string   `yaml:"image"`
	TTY             *bool    `yaml:"tty"`
	Interactive     *bool    `yaml:"interactive"`
	Network         string   `yaml:"network"`
	Remove          *bool    `yaml:"remove"`
	StrictEnv       *bool    `yaml:"strictEnv"`
	Volumes         []string `yaml:"volumes"`
	Env             []string `yaml:"env"`
	Workdir         string   `yaml:"workdir"`
	MountCderun     *bool    `yaml:"mountCderun"`
	MountSocket     *bool    `yaml:"mountSocket"`
	MountSocketPath string   `yaml:"mountSocketPath"`
	DryRun          *bool    `yaml:"dryRun"`
	DryRunFormat    string   `yaml:"dryRunFormat"`
	// New fields
	Ports      []string `yaml:"ports"`
	PublishAll *bool    `yaml:"publishAll"`
	Expose     []string `yaml:"expose"`
	Hostname   string   `yaml:"hostname"`
	DNS        []string `yaml:"dns"`
	AddHosts   []string `yaml:"addHosts"`
	User       string   `yaml:"user"`
	Privileged *bool    `yaml:"privileged"`
	CapAdd     []string `yaml:"capAdd"`
	CapDrop    []string `yaml:"capDrop"`
	Entrypoint []string `yaml:"entrypoint"`
	Pull       string   `yaml:"pull"`
	Memory     string   `yaml:"memory"`
	CPUs       float64  `yaml:"cpus"`
	Tmpfs      []string `yaml:"tmpfs"`
	Devices    []string `yaml:"devices"`
}

type ToolsConfig map[string]ToolConfig

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
	p := filepath.Join("/etc", "cderun", filename)
	if _, err := os.Stat(p); err == nil {
		paths = append(paths, p)
	}

	return paths
}

// ResolvePathsGlobal resolves relative paths and tilde in CDERunConfig.
func ResolvePathsGlobal(cfg *CDERunConfig, baseDir string) {
	cfg.SocketPath = resolvePath(cfg.SocketPath, baseDir)
	cfg.Defaults.MountSocketPath = resolvePath(cfg.Defaults.MountSocketPath, baseDir)
	cfg.Logging.File = resolvePath(cfg.Logging.File, baseDir)
}

// ResolvePathsTool resolves relative paths and tilde in ToolConfig.
func ResolvePathsTool(cfg *ToolConfig, baseDir string) {
	cfg.MountSocketPath = resolvePath(cfg.MountSocketPath, baseDir)
	for i, v := range cfg.Volumes {
		cfg.Volumes[i] = resolveVolumePath(v, baseDir)
	}
	for i, d := range cfg.Devices {
		cfg.Devices[i] = resolveDevicePath(d, baseDir)
	}
}

func resolvePath(p string, baseDir string) string {
	if p == "" {
		return p
	}
	// Tilde expansion
	if strings.HasPrefix(p, "~/") || p == "~" {
		home, err := os.UserHomeDir()
		if err == nil {
			if p == "~" {
				p = home
			} else {
				p = filepath.Join(home, p[2:])
			}
		}
	}
	// Relative path resolution
	if !filepath.IsAbs(p) && (strings.HasPrefix(p, "./") || strings.HasPrefix(p, "../") || p == "." || p == "..") {
		p = filepath.Join(baseDir, p)
	}
	return filepath.Clean(p)
}

var winDriveRegex = regexp.MustCompile(`^[A-Za-z]:[\\/]`)

func resolveVolumePath(v string, baseDir string) string {
	host, remainder, ok := splitHostRemainder(v)
	if !ok {
		return v
	}
	return resolvePath(host, baseDir) + ":" + remainder
}

func resolveDevicePath(d string, baseDir string) string {
	// host-path:container-path[:permissions]
	host, remainder, ok := splitHostRemainder(d)
	if !ok {
		return d
	}
	return resolvePath(host, baseDir) + ":" + remainder
}

func splitHostRemainder(s string) (string, string, bool) {
	sepIdx := strings.Index(s, ":")
	if sepIdx == -1 {
		return "", "", false
	}

	// If it's a Windows drive letter (e.g. C:\ or C:/), the first colon is part of the path.
	// We need to look for the separator colon after the drive letter (index > 1).
	if winDriveRegex.MatchString(s) {
		nextSep := strings.Index(s[sepIdx+1:], ":")
		if nextSep == -1 {
			return "", "", false
		}
		sepIdx = sepIdx + 1 + nextSep
	}

	return s[:sepIdx], s[sepIdx+1:], true
}

// LoadCDERunConfig searches for .cderun.yaml in hierarchical locations and merges them.
func LoadCDERunConfig() (*CDERunConfig, []string, error) {
	paths := FindConfigs(".cderun.yaml")
	if len(paths) == 0 {
		return nil, nil, nil
	}

	resolver, _ := NewExpressionResolver()
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

		// Resolve expressions and paths for this layer
		resolvedLayer := resolver.ResolveConfig(&layer).(*CDERunConfig)
		ResolvePathsGlobal(resolvedLayer, baseDir)

		if err := mergo.Merge(&merged, resolvedLayer, mergo.WithOverride); err != nil {
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

	resolver, _ := NewExpressionResolver()
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
			// Resolve expressions and paths for this tool
			resolvedTool := resolver.ResolveConfig(&v).(*ToolConfig)
			ResolvePathsTool(resolvedTool, baseDir)

			if existing, ok := merged[k]; ok {
				if err := mergo.Merge(&existing, resolvedTool, mergo.WithOverride); err != nil {
					return nil, nil, fmt.Errorf("failed to merge tool config for %s from %s: %w", k, path, err)
				}
				merged[k] = existing
			} else {
				merged[k] = *resolvedTool
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
