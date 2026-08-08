package container

// ContainerConfig represents the intermediate representation of a container execution request.
//
// Field values hold Docker-CLI-compatible notation as entered by the user
// (e.g. CapAdd: "SYS_ADMIN", not "CAP_SYS_ADMIN"). The Docker daemon normalizes
// such notation implicitly, but runtimes that build an OCI spec directly
// (containerd) do NOT: each runtime adapter is responsible for converting every
// field it consumes into its native representation, and for returning an explicit
// error for fields it cannot support — never pass a value through unconverted or
// drop it silently.
type ContainerConfig struct {
	// Basic settings
	Image   string   `json:"image" yaml:"image"`
	Command []string `json:"command" yaml:"command"`

	// Execution options
	TTY         bool `json:"tty" yaml:"tty"`
	Interactive bool `json:"interactive" yaml:"interactive"`
	Remove      bool `json:"remove" yaml:"remove"`
	ReadOnly    bool `json:"read_only,omitempty" yaml:"read_only,omitempty"`
	Init        bool `json:"init,omitempty" yaml:"init,omitempty"`

	// Network
	Network    string   `json:"network" yaml:"network"`
	Ports      []string `json:"ports,omitempty" yaml:"ports,omitempty"`
	PublishAll bool     `json:"publish_all,omitempty" yaml:"publish_all,omitempty"`
	Expose     []string `json:"expose,omitempty" yaml:"expose,omitempty"`
	Hostname   string   `json:"hostname,omitempty" yaml:"hostname,omitempty"`
	DNS        []string `json:"dns,omitempty" yaml:"dns,omitempty"`
	AddHosts   []string `json:"add_hosts,omitempty" yaml:"add_hosts,omitempty"`

	// Mounts
	Mounts []Mount `json:"mounts" yaml:"mounts"`

	// Environment variables (format: ["KEY=value", "KEY2=value2"])
	Env []string `json:"env" yaml:"env"`

	// Working directory
	Workdir string `json:"workdir" yaml:"workdir"`

	// User
	User string `json:"user" yaml:"user"`

	// Permissions and entrypoint
	Privileged bool     `json:"privileged,omitempty" yaml:"privileged,omitempty"`
	Pid        string   `json:"pid,omitempty" yaml:"pid,omitempty"`
	CapAdd     []string `json:"cap_add,omitempty" yaml:"cap_add,omitempty"`
	CapDrop    []string `json:"cap_drop,omitempty" yaml:"cap_drop,omitempty"`
	Entrypoint []string `json:"entrypoint,omitempty" yaml:"entrypoint,omitempty"`

	// Management
	Pull string `json:"pull,omitempty" yaml:"pull,omitempty"`

	// Resources
	Memory int64   `json:"memory,omitempty" yaml:"memory,omitempty"`
	CPUs   float64 `json:"cpus,omitempty" yaml:"cpus,omitempty"`

	// Storage and Devices
	Devices []DeviceMapping `json:"devices,omitempty" yaml:"devices,omitempty"`

	// Supplementary groups
	GroupAdd []string `json:"group_add,omitempty" yaml:"group_add,omitempty"`

	// Resources limits (ulimits)
	Ulimits []Ulimit `json:"ulimits,omitempty" yaml:"ulimits,omitempty"`
}

// Ulimit represents a ulimit setting for a container.
type Ulimit struct {
	Name string `json:"name" yaml:"name"`
	Hard int64  `json:"hard" yaml:"hard"`
	Soft int64  `json:"soft" yaml:"soft"`
}

// Mount represents a mount point in the container (bind, volume, or tmpfs).
type Mount struct {
	Type     string `json:"type" yaml:"type"`
	Source   string `json:"source,omitempty" yaml:"source,omitempty"`
	Target   string `json:"target" yaml:"target"`
	ReadOnly bool   `json:"read_only,omitempty" yaml:"read_only,omitempty"`
	Optional bool   `json:"optional,omitempty" yaml:"optional,omitempty"`
}

// DeviceMapping represents a host device to container device mapping.
type DeviceMapping struct {
	PathOnHost        string `json:"path_on_host" yaml:"path_on_host"`
	PathInContainer   string `json:"path_in_container" yaml:"path_in_container"`
	CgroupPermissions string `json:"cgroup_permissions" yaml:"cgroup_permissions"`
}
