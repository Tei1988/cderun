package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type CDERunConfig struct {
	Runtime     string         `yaml:"runtime"`
	RuntimePath string         `yaml:"runtimePath"`
	Defaults    ConfigDefaults `yaml:"defaults"`
}

type ConfigDefaults struct {
	TTY         *bool  `yaml:"tty"`
	Interactive *bool  `yaml:"interactive"`
	Network     string `yaml:"network"`
	Remove      *bool  `yaml:"remove"`
	SyncWorkdir *bool  `yaml:"syncWorkdir"`
}

type ToolConfig struct {
	Image       string   `yaml:"image"`
	TTY         *bool    `yaml:"tty"`
	Interactive *bool    `yaml:"interactive"`
	Network     string   `yaml:"network"`
	Remove      *bool    `yaml:"remove"`
	Volumes     []string `yaml:"volumes"`
	Env         []string `yaml:"env"`
	Workdir     string   `yaml:"workdir"`
}

type ToolsConfig map[string]ToolConfig

// LoadCDERunConfig searches for .cderun.yaml in predefined locations and loads the first one found.
func LoadCDERunConfig() (*CDERunConfig, string, error) {
	paths := []string{
		".cderun.yaml",
	}
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".config", "cderun", "config.yaml"))
	}
	paths = append(paths, "/etc/cderun/config.yaml")

	for _, path := range paths {
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, "", fmt.Errorf("stat config file %s: %w", path, err)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil, "", fmt.Errorf("failed to read config file %s: %w", path, err)
		}
		var cfg CDERunConfig
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, "", fmt.Errorf("failed to unmarshal config file %s: %w", path, err)
		}
		return &cfg, path, nil
	}
	return nil, "", nil
}

// LoadToolsConfig searches for .tools.yaml in predefined locations and loads the first one found.
func LoadToolsConfig() (ToolsConfig, string, error) {
	paths := []string{
		".tools.yaml",
	}
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".config", "cderun", "tools.yaml"))
	}
	paths = append(paths, "/etc/cderun/tools.yaml")

	for _, path := range paths {
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, "", fmt.Errorf("stat tools file %s: %w", path, err)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil, "", fmt.Errorf("failed to read tools file %s: %w", path, err)
		}
		var cfg ToolsConfig
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, "", fmt.Errorf("failed to unmarshal tools file %s: %w", path, err)
		}
		return cfg, path, nil
	}
	return nil, "", nil
}
