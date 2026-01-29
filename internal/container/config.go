package container

import (
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// ContainerConfig represents the intermediate representation of a container execution request.
type ContainerConfig struct {
	// Basic settings
	Image   string   `json:"image" yaml:"image"`
	Command []string `json:"command" yaml:"command"`
	Args    []string `json:"args" yaml:"args"`

	// Execution options
	TTY         bool `json:"tty" yaml:"tty"`
	Interactive bool `json:"interactive" yaml:"interactive"`
	Remove      bool `json:"remove" yaml:"remove"`

	// Network
	Network string `json:"network" yaml:"network"`

	// Volumes
	Volumes []VolumeMount `json:"volumes" yaml:"volumes"`

	// Environment variables (format: ["KEY=value", "KEY2=value2"])
	Env []string `json:"env" yaml:"env"`

	// Working directory
	Workdir string `json:"workdir" yaml:"workdir"`

	// User
	User string `json:"user" yaml:"user"`
}

// ToYAML returns the YAML representation of the config.
func (c *ContainerConfig) ToYAML() (string, error) {
	data, err := yaml.Marshal(c)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ToJSON returns the JSON representation of the config.
func (c *ContainerConfig) ToJSON() (string, error) {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ToSimple returns a simple string representation of the config.
func (c *ContainerConfig) ToSimple() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Image: %s\n", c.Image))
	sb.WriteString(fmt.Sprintf("Command: %s %s\n", c.Command[0], strings.Join(c.Args, " ")))

	if len(c.Volumes) > 0 {
		vols := make([]string, 0, len(c.Volumes))
		for _, v := range c.Volumes {
			ro := ""
			if v.ReadOnly {
				ro = ":ro"
			}
			vols = append(vols, fmt.Sprintf("%s:%s%s", v.HostPath, v.ContainerPath, ro))
		}
		sb.WriteString(fmt.Sprintf("Volumes: %s\n", strings.Join(vols, ", ")))
	}

	if len(c.Env) > 0 {
		sb.WriteString(fmt.Sprintf("Env: %s\n", strings.Join(c.Env, ", ")))
	}

	if c.Workdir != "" {
		sb.WriteString(fmt.Sprintf("Workdir: %s\n", c.Workdir))
	}

	return sb.String()
}

// VolumeMount represents a host path to container path mapping.
type VolumeMount struct {
	HostPath      string `json:"host_path" yaml:"host_path"`
	ContainerPath string `json:"container_path" yaml:"container_path"`
	ReadOnly      bool   `json:"read_only" yaml:"read_only"`
}
