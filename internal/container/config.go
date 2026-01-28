package container

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

// VolumeMount represents a host path to container path mapping.
type VolumeMount struct {
	HostPath      string `json:"host_path" yaml:"host_path"`
	ContainerPath string `json:"container_path" yaml:"container_path"`
	ReadOnly      bool   `json:"read_only" yaml:"read_only"`
}
