package config

import (
	"strings"
	"sync"
)

// StringOption defines a string-based configuration option.
type StringOption struct {
	Name           string // kebab-case, used for flags (e.g. "image")
	FieldName      string // PascalCase, used for reflection (pre-calculated)
	Shorthand      string
	Usage          string
	Default        string
	EnvKey         string
	ToolGetter     func(ToolConfig) string
	GlobalGetter   func(CDERunConfig) string
	SkipResolution bool // If true, only used for flag registration or handled specially in ResolveWithFS
	info           optionFields
}

// BoolOption defines a boolean configuration option.
type BoolOption struct {
	Name         string // kebab-case, used for flags (e.g. "tty")
	FieldName    string // PascalCase, used for reflection (pre-calculated)
	Shorthand    string
	Usage        string
	Default      bool
	EnvKey       string
	ToolGetter   func(ToolConfig) *bool
	GlobalGetter func(CDERunConfig) *bool
	info         optionFields
}

// IntOption defines an integer configuration option.
type IntOption struct {
	Name         string // kebab-case, used for flags (e.g. "pull-max-retries")
	FieldName    string // PascalCase, used for reflection (pre-calculated)
	Shorthand    string
	Usage        string
	Default      int
	EnvKey       string
	ToolGetter   func(ToolConfig) *int
	GlobalGetter func(CDERunConfig) *int
	info         optionFields
}

// Float64Option defines a float64 configuration option.
type Float64Option struct {
	Name         string // kebab-case, used for flags (e.g. "cpus")
	FieldName    string // PascalCase, used for reflection (pre-calculated)
	Shorthand    string
	Usage        string
	Default      float64
	EnvKey       string
	ToolGetter   func(ToolConfig) *float64
	GlobalGetter func(CDERunConfig) *float64
	info         optionFields
}

// StringSliceOption defines a string slice configuration option.
type StringSliceOption struct {
	Name           string // kebab-case, used for flags (e.g. "env")
	FieldName      string // PascalCase, used for reflection (pre-calculated)
	Shorthand      string
	Usage          string
	EnvKey         string
	ToolGetter     func(ToolConfig) []string
	GlobalGetter   func(CDERunConfig) []string
	SkipResolution bool // If true, only used for flag registration or handled specially in ResolveWithFS
	info           optionFields
}

var IntOptions = []IntOption{
	{
		Name:    "pull-max-retries",
		EnvKey:  "CDERUN_PULL_MAX_RETRIES",
		Usage:   "Maximum number of retries for image pull",
		Default: 3,
		ToolGetter: func(t ToolConfig) *int {
			return t.PullMaxRetries
		},
		GlobalGetter: func(g CDERunConfig) *int {
			return g.Defaults.PullMaxRetries
		},
	},
	{
		Name:      "pids-limit",
		FieldName: "PidsLimit",
		EnvKey:    "CDERUN_PIDS_LIMIT",
		Usage:     "Tune container pids limit",
		Default:   0,
		ToolGetter: func(t ToolConfig) *int {
			return t.PidsLimit
		},
		GlobalGetter: func(g CDERunConfig) *int {
			return g.Defaults.PidsLimit
		},
	},
	{
		Name:      "cpu-shares",
		FieldName: "CPUShares",
		EnvKey:    "CDERUN_CPU_SHARES",
		Usage:     "CPU shares (relative weight)",
		Default:   0,
		ToolGetter: func(t ToolConfig) *int {
			return t.CPUShares
		},
		GlobalGetter: func(g CDERunConfig) *int {
			return g.Defaults.CPUShares
		},
	},
}

var Float64Options = []Float64Option{
	{
		Name:   "cpus",
		EnvKey: "CDERUN_CPUS",
		Usage:  "Number of CPUs",
		ToolGetter: func(t ToolConfig) *float64 {
			return t.CPUs
		},
		GlobalGetter: func(g CDERunConfig) *float64 {
			return g.Defaults.CPUs
		},
	},
}

var StringSliceOptions = []StringSliceOption{
	{
		Name:      "env",
		Shorthand: "e",
		EnvKey:    "CDERUN_ENV",
		Usage:     "Set environment variables",
		ToolGetter: func(t ToolConfig) []string {
			return t.Env
		},
		GlobalGetter: func(g CDERunConfig) []string {
			return g.Defaults.Env
		},
		SkipResolution: true, // resolved in resolveComplexOptions
	},
	{
		Name:           "mount",
		FieldName:      "Mounts",
		EnvKey:         "CDERUN_MOUNT",
		Usage:          "Attach a filesystem mount to the container",
		SkipResolution: true, // resolved in resolveComplexOptions
	},
	{
		Name:      "publish",
		FieldName: "Ports",
		Shorthand: "p",
		EnvKey:    "CDERUN_PUBLISH",
		Usage:     "Publish a container's port(s) to the host",
		ToolGetter: func(t ToolConfig) []string {
			return t.Ports
		},
		GlobalGetter: func(g CDERunConfig) []string {
			return g.Defaults.Ports
		},
	},
	{
		Name:   "expose",
		EnvKey: "CDERUN_EXPOSE",
		Usage:  "Expose a port or a range of ports",
		ToolGetter: func(t ToolConfig) []string {
			return t.Expose
		},
		GlobalGetter: func(g CDERunConfig) []string {
			return g.Defaults.Expose
		},
	},
	{
		Name:   "dns",
		EnvKey: "CDERUN_DNS",
		Usage:  "Set custom DNS servers",
		ToolGetter: func(t ToolConfig) []string {
			return t.DNS
		},
		GlobalGetter: func(g CDERunConfig) []string {
			return g.Defaults.DNS
		},
	},
	{
		Name:      "add-host",
		FieldName: "AddHosts",
		EnvKey:    "CDERUN_ADD_HOST",
		Usage:     "Add a custom host-to-IP mapping (host:ip)",
		ToolGetter: func(t ToolConfig) []string {
			return t.AddHosts
		},
		GlobalGetter: func(g CDERunConfig) []string {
			return g.Defaults.AddHosts
		},
	},
	{
		Name:      "group-add",
		FieldName: "GroupAdd",
		EnvKey:    "CDERUN_GROUP_ADD",
		Usage:     "Add supplementary groups to the container (name or GID)",
		ToolGetter: func(t ToolConfig) []string {
			return t.GroupAdd
		},
		GlobalGetter: func(g CDERunConfig) []string {
			return g.Defaults.GroupAdd
		},
	},
	{
		Name:      "cap-add",
		FieldName: "CapAdd",
		EnvKey:    "CDERUN_CAP_ADD",
		Usage:     "Add Linux capabilities",
		ToolGetter: func(t ToolConfig) []string {
			return t.CapAdd
		},
		GlobalGetter: func(g CDERunConfig) []string {
			return g.Defaults.CapAdd
		},
	},
	{
		Name:      "cap-drop",
		FieldName: "CapDrop",
		EnvKey:    "CDERUN_CAP_DROP",
		Usage:     "Drop Linux capabilities",
		ToolGetter: func(t ToolConfig) []string {
			return t.CapDrop
		},
		GlobalGetter: func(g CDERunConfig) []string {
			return g.Defaults.CapDrop
		},
	},
	{
		Name:      "ulimit",
		FieldName: "Ulimits",
		EnvKey:    "CDERUN_ULIMIT",
		Usage:     "Set ulimits (e.g. nofile=65535:65535)",
		ToolGetter: func(t ToolConfig) []string {
			return t.Ulimits
		},
		GlobalGetter: func(g CDERunConfig) []string {
			return g.Defaults.Ulimits
		},
		SkipResolution: true, // resolved in resolveComplexOptions
	},
	{
		Name:   "entrypoint",
		EnvKey: "CDERUN_ENTRYPOINT",
		Usage:  "Overwrite the default ENTRYPOINT of the image",
		ToolGetter: func(t ToolConfig) []string {
			return t.Entrypoint
		},
		GlobalGetter: func(g CDERunConfig) []string {
			return g.Defaults.Entrypoint
		},
	},
	{
		Name:           "device",
		FieldName:      "Devices",
		EnvKey:         "CDERUN_DEVICE",
		Usage:          "Add a host device to the container",
		SkipResolution: true, // resolved in resolveCustomParsing
	},
	{
		Name:   "sensitive-env",
		EnvKey: "CDERUN_SENSITIVE_ENV",
		Usage:  "List of environment variable patterns to mask (default masks all variables)",
		ToolGetter: func(t ToolConfig) []string {
			return t.SensitiveEnv
		},
		GlobalGetter: func(g CDERunConfig) []string {
			return g.Defaults.SensitiveEnv
		},
		SkipResolution: true, // resolved early in resolveEarly
	},
	{
		Name:      "security-opt",
		FieldName: "SecurityOpt",
		EnvKey:    "CDERUN_SECURITY_OPT",
		Usage:     "Configure security options",
		ToolGetter: func(t ToolConfig) []string {
			return t.SecurityOpt
		},
		GlobalGetter: func(g CDERunConfig) []string {
			return g.Defaults.SecurityOpt
		},
	},
	{
		Name:      "dns-search",
		FieldName: "DNSSearch",
		EnvKey:    "CDERUN_DNS_SEARCH",
		Usage:     "Set custom DNS search domains",
		ToolGetter: func(t ToolConfig) []string {
			return t.DNSSearch
		},
		GlobalGetter: func(g CDERunConfig) []string {
			return g.Defaults.DNSSearch
		},
	},
	{
		Name:      "dns-option",
		FieldName: "DNSOptions",
		EnvKey:    "CDERUN_DNS_OPTION",
		Usage:     "Set DNS options",
		ToolGetter: func(t ToolConfig) []string {
			return t.DNSOptions
		},
		GlobalGetter: func(g CDERunConfig) []string {
			return g.Defaults.DNSOptions
		},
	},
}

var StringOptions = []StringOption{
	{
		Name:    "network",
		EnvKey:  "CDERUN_NETWORK",
		Usage:   "Connect a container to a network",
		Default: "bridge",
		ToolGetter: func(t ToolConfig) string {
			return t.Network
		},
		GlobalGetter: func(g CDERunConfig) string {
			return g.Defaults.Network
		},
	},
	{
		Name:   "socket-path",
		EnvKey: "CDERUN_SOCKET_PATH",
		Usage:  "Path to the container runtime socket on the host",
		GlobalGetter: func(g CDERunConfig) string {
			return g.SocketPath.Raw
		},
		SkipResolution: true, // resolved in resolveRuntimeAndSocket
	},
	{
		Name:   "mount-socket-path",
		EnvKey: "CDERUN_MOUNT_SOCKET_PATH",
		Usage:  "Path where the socket should be mounted inside the container (defaults to host path)",
		ToolGetter: func(t ToolConfig) string {
			return t.MountSocketPath.Raw
		},
		GlobalGetter: func(g CDERunConfig) string {
			return g.Defaults.MountSocketPath.Raw
		},
		SkipResolution: true, // resolved in resolveTransitiveOptions
	},
	{
		Name:   "mount-cderun-path",
		EnvKey: "CDERUN_MOUNT_CDERUN_PATH",
		Usage:  "Host path to cderun binary to mount inside container",
		ToolGetter: func(t ToolConfig) string {
			return t.MountCderunPath.Raw
		},
		GlobalGetter: func(g CDERunConfig) string {
			return g.Defaults.MountCderunPath.Raw
		},
		SkipResolution: true, // resolved in resolveTransitiveOptions
	},
	{
		Name:   "image",
		EnvKey: "CDERUN_IMAGE",
		Usage:  "Docker image to use",
		ToolGetter: func(t ToolConfig) string {
			return t.Image
		},
	},
	{
		Name:   "pid",
		EnvKey: "CDERUN_PID",
		Usage:  "PID namespace to use",
		ToolGetter: func(t ToolConfig) string {
			return t.Pid
		},
		GlobalGetter: func(g CDERunConfig) string {
			return g.Defaults.Pid
		},
	},
	{
		Name:   "runtime",
		EnvKey: "CDERUN_RUNTIME",
		Usage:  "Container runtime to use (docker/podman/containerd)",
		GlobalGetter: func(g CDERunConfig) string {
			return g.Runtime
		},
	},
	{
		Name:   "shm-size",
		EnvKey: "CDERUN_SHM_SIZE",
		Usage:  "Size of /dev/shm (e.g. 512m, 1g, 2147483648)",
		ToolGetter: func(t ToolConfig) string {
			return t.ShmSize
		},
		GlobalGetter: func(g CDERunConfig) string {
			return g.Defaults.ShmSize
		},
	},
	{
		Name:      "workdir",
		Shorthand: "w",
		EnvKey:    "CDERUN_WORKDIR",
		Usage:     "Working directory inside the container",
		ToolGetter: func(t ToolConfig) string {
			return t.Workdir
		},
		GlobalGetter: func(g CDERunConfig) string {
			return g.Defaults.Workdir
		},
	},
	{
		Name:   "mount-tools",
		EnvKey: "CDERUN_MOUNT_TOOLS",
		Usage:  "Mount specified tools into the container",
		ToolGetter: func(t ToolConfig) string {
			return strings.Join(t.MountTools, ",")
		},
		GlobalGetter: func(g CDERunConfig) string {
			return strings.Join(g.Defaults.MountTools, ",")
		},
		SkipResolution: true, // resolved in resolveTransitiveOptions (comma-separated string)
	},
	{
		Name:           "config",
		EnvKey:         "CDERUN_CONFIG",
		Usage:          "Path to cderun config file",
		SkipResolution: true,
	},
	{
		Name:           "tool-config",
		EnvKey:         "CDERUN_TOOL_CONFIG",
		Usage:          "Path to tools config file",
		SkipResolution: true,
	},
	{
		Name:   "hostname",
		EnvKey: "CDERUN_HOSTNAME",
		Usage:  "Container host name",
		ToolGetter: func(t ToolConfig) string {
			return t.Hostname
		},
		GlobalGetter: func(g CDERunConfig) string {
			return g.Defaults.Hostname
		},
	},
	{
		Name:      "user",
		Shorthand: "u",
		EnvKey:    "CDERUN_USER",
		Usage:     "Username or UID (format: <name|uid>[:<group|gid>])",
		ToolGetter: func(t ToolConfig) string {
			return t.User
		},
		GlobalGetter: func(g CDERunConfig) string {
			return g.Defaults.User
		},
	},
	{
		Name:    "pull",
		EnvKey:  "CDERUN_PULL",
		Usage:   "Pull image before running (always, missing, never)",
		Default: "missing",
		ToolGetter: func(t ToolConfig) string {
			return t.Pull
		},
		GlobalGetter: func(g CDERunConfig) string {
			return g.Defaults.Pull
		},
	},
	{
		Name:    "pull-backoff-base",
		EnvKey:  "CDERUN_PULL_BACKOFF_BASE",
		Usage:   "Base duration for exponential backoff during image pull (e.g. 1s, 500ms)",
		Default: "1s",
		ToolGetter: func(t ToolConfig) string {
			return t.PullBackoffBase
		},
		GlobalGetter: func(g CDERunConfig) string {
			return g.Defaults.PullBackoffBase
		},
		SkipResolution: true, // resolved in resolveCustomParsing (parsed as duration)
	},
	{
		Name:      "memory",
		Shorthand: "m",
		EnvKey:    "CDERUN_MEMORY",
		Usage:     "Memory limit",
		ToolGetter: func(t ToolConfig) string {
			return t.Memory
		},
		GlobalGetter: func(g CDERunConfig) string {
			return g.Defaults.Memory
		},
		SkipResolution: true, // resolved in resolveCustomParsing (parsed as bytes)
	},
	{
		Name:      "dry-run-format",
		Shorthand: "f",
		EnvKey:    "CDERUN_DRY_RUN_FORMAT",
		Usage:     "Output format (yaml, json, simple)",
		Default:   "yaml",
		ToolGetter: func(t ToolConfig) string {
			return t.DryRunFormat
		},
		GlobalGetter: func(g CDERunConfig) string {
			return g.Defaults.DryRunFormat
		},
	},
	{
		Name:    "diagnosis-format",
		EnvKey:  "CDERUN_DIAGNOSIS_FORMAT",
		Usage:   "Diagnosis output format (yaml, json, simple)",
		Default: "yaml",
		ToolGetter: func(t ToolConfig) string {
			return t.DiagnosisFormat
		},
		GlobalGetter: func(g CDERunConfig) string {
			return g.Defaults.DiagnosisFormat
		},
	},
	{
		Name:    "log-level",
		EnvKey:  "CDERUN_LOG_LEVEL",
		Usage:   "Set log level (error, warn, info, debug, trace)",
		Default: "error",
		ToolGetter: func(t ToolConfig) string {
			return t.LogLevel
		},
		GlobalGetter: func(g CDERunConfig) string {
			return g.Logging.Level
		},
	},
	{
		Name:    "log-format",
		EnvKey:  "CDERUN_LOG_FORMAT",
		Usage:   "Set log format (text, json)",
		Default: "text",
		ToolGetter: func(t ToolConfig) string {
			return t.LogFormat
		},
		GlobalGetter: func(g CDERunConfig) string {
			return g.Logging.Format
		},
	},
	{
		Name:    "hang-timeout",
		EnvKey:  "CDERUN_HANG_TIMEOUT",
		Usage:   "Grace period after I/O completion before force-terminating the container (e.g. 10s, 5s)",
		Default: "10s",
		ToolGetter: func(t ToolConfig) string {
			return t.HangTimeout
		},
		GlobalGetter: func(g CDERunConfig) string {
			return g.Defaults.HangTimeout
		},
		SkipResolution: true, // resolved in resolveCustomParsing (parsed as duration)
	},
	{
		Name:      "ipc",
		FieldName: "IPC",
		EnvKey:    "CDERUN_IPC",
		Usage:     "IPC namespace to use",
		Default:   "",
		ToolGetter: func(t ToolConfig) string {
			return t.IPC
		},
		GlobalGetter: func(g CDERunConfig) string {
			return g.Defaults.IPC
		},
	},
	{
		Name:      "gpus",
		FieldName: "GPUs",
		EnvKey:    "CDERUN_GPUS",
		Usage:     "GPU devices to request",
		Default:   "",
		ToolGetter: func(t ToolConfig) string {
			return t.GPUs
		},
		GlobalGetter: func(g CDERunConfig) string {
			return g.Defaults.GPUs
		},
	},
	{
		Name:      "cgroupns",
		FieldName: "Cgroupns",
		EnvKey:    "CDERUN_CGROUPNS",
		Usage:     "Cgroup namespace to use",
		Default:   "",
		ToolGetter: func(t ToolConfig) string {
			return t.Cgroupns
		},
		GlobalGetter: func(g CDERunConfig) string {
			return g.Defaults.Cgroupns
		},
	},
	{
		Name:      "cpuset-cpus",
		FieldName: "CpusetCpus",
		EnvKey:    "CDERUN_CPUSET_CPUS",
		Usage:     "CPUs in which to allow execution (0-3, 0,1)",
		Default:   "",
		ToolGetter: func(t ToolConfig) string {
			return t.CpusetCpus
		},
		GlobalGetter: func(g CDERunConfig) string {
			return g.Defaults.CpusetCpus
		},
	},
	{
		Name:      "cpuset-mems",
		FieldName: "CpusetMems",
		EnvKey:    "CDERUN_CPUSET_MEMS",
		Usage:     "MEMs in which to allow execution (0-3, 0,1)",
		Default:   "",
		ToolGetter: func(t ToolConfig) string {
			return t.CpusetMems
		},
		GlobalGetter: func(g CDERunConfig) string {
			return g.Defaults.CpusetMems
		},
	},
	{
		Name:      "restart",
		FieldName: "Restart",
		EnvKey:    "CDERUN_RESTART",
		Usage:     "Restart policy to apply when a container exits",
		Default:   "",
		ToolGetter: func(t ToolConfig) string {
			return t.Restart
		},
		GlobalGetter: func(g CDERunConfig) string {
			return g.Defaults.Restart
		},
	},
}

var BoolOptions = []BoolOption{
	{
		Name:      "tty",
		Shorthand: "t",
		EnvKey:    "CDERUN_TTY",
		Usage:     "Allocate a pseudo-TTY",
		Default:   false,
		ToolGetter: func(t ToolConfig) *bool {
			return t.TTY
		},
		GlobalGetter: func(g CDERunConfig) *bool {
			return g.Defaults.TTY
		},
	},
	{
		Name:      "interactive",
		Shorthand: "i",
		EnvKey:    "CDERUN_INTERACTIVE",
		Usage:     "Keep STDIN open even if not attached",
		Default:   false,
		ToolGetter: func(t ToolConfig) *bool {
			return t.Interactive
		},
		GlobalGetter: func(g CDERunConfig) *bool {
			return g.Defaults.Interactive
		},
	},
	{
		Name:   "mount-socket",
		EnvKey: "CDERUN_MOUNT_SOCKET",
		Usage:  "Mount the container runtime socket into the container",
		ToolGetter: func(t ToolConfig) *bool {
			return t.MountSocket
		},
		GlobalGetter: func(g CDERunConfig) *bool {
			return g.Defaults.MountSocket
		},
	},
	{
		Name:   "mount-cderun",
		EnvKey: "CDERUN_MOUNT_CDERUN",
		Usage:  "Mount cderun binary for use inside container",
		ToolGetter: func(t ToolConfig) *bool {
			return t.MountCderun
		},
		GlobalGetter: func(g CDERunConfig) *bool {
			return g.Defaults.MountCderun
		},
	},
	{
		Name:   "mount-all-tools",
		EnvKey: "CDERUN_MOUNT_ALL_TOOLS",
		Usage:  "Mount all defined tools into the container",
		ToolGetter: func(t ToolConfig) *bool {
			return t.MountAllTools
		},
		GlobalGetter: func(g CDERunConfig) *bool {
			return g.Defaults.MountAllTools
		},
	},
	{
		Name:    "remove",
		EnvKey:  "CDERUN_REMOVE",
		Usage:   "Automatically remove the container when it exits",
		Default: true,
		ToolGetter: func(t ToolConfig) *bool {
			return t.Remove
		},
		GlobalGetter: func(g CDERunConfig) *bool {
			return g.Defaults.Remove
		},
	},
	{
		Name:      "publish-all",
		Shorthand: "P",
		EnvKey:    "CDERUN_PUBLISH_ALL",
		Usage:     "Publish all exposed ports to random ports",
		ToolGetter: func(t ToolConfig) *bool {
			return t.PublishAll
		},
		GlobalGetter: func(g CDERunConfig) *bool {
			return g.Defaults.PublishAll
		},
	},
	{
		Name:   "privileged",
		EnvKey: "CDERUN_PRIVILEGED",
		Usage:  "Give extended privileges to this container",
		ToolGetter: func(t ToolConfig) *bool {
			return t.Privileged
		},
		GlobalGetter: func(g CDERunConfig) *bool {
			return g.Defaults.Privileged
		},
	},
	{
		Name:   "strict-env",
		EnvKey: "CDERUN_STRICT_ENV",
		Usage:  "Require all environment variables to be present on the host",
		ToolGetter: func(t ToolConfig) *bool {
			return t.StrictEnv
		},
		GlobalGetter: func(g CDERunConfig) *bool {
			return g.Defaults.StrictEnv
		},
	},
	{
		Name:   "dry-run",
		EnvKey: "CDERUN_DRY_RUN",
		Usage:  "Preview container configuration without execution",
		ToolGetter: func(t ToolConfig) *bool {
			return t.DryRun
		},
		GlobalGetter: func(g CDERunConfig) *bool {
			return g.Defaults.DryRun
		},
	},
	{
		Name:   "diagnosis",
		EnvKey: "CDERUN_DIAGNOSIS",
		Usage:  "Show system diagnostics and available tools",
		ToolGetter: func(t ToolConfig) *bool {
			return t.Diagnosis
		},
		GlobalGetter: func(g CDERunConfig) *bool {
			return g.Defaults.Diagnosis
		},
	},
	{
		Name:    "log-timestamp",
		EnvKey:  "CDERUN_LOG_TIMESTAMP",
		Usage:   "Include timestamp in logs",
		Default: true,
		ToolGetter: func(t ToolConfig) *bool {
			return t.LogTimestamp
		},
		GlobalGetter: func(g CDERunConfig) *bool {
			return g.Logging.Timestamp
		},
	},
	{
		Name:   "read-only",
		EnvKey: "CDERUN_READ_ONLY",
		Usage:  "Mount the container's root filesystem as read-only",
		ToolGetter: func(t ToolConfig) *bool {
			return t.ReadOnly
		},
		GlobalGetter: func(g CDERunConfig) *bool {
			return g.Defaults.ReadOnly
		},
	},
	{
		Name:   "init",
		EnvKey: "CDERUN_INIT",
		Usage:  "Run an init process inside the container to forward signals and reap processes",
		ToolGetter: func(t ToolConfig) *bool {
			return t.Init
		},
		GlobalGetter: func(g CDERunConfig) *bool {
			return g.Defaults.Init
		},
	},
}

var (
	stringOptionsMap      map[string]StringOption
	boolOptionsMap        map[string]BoolOption
	intOptionsMap         map[string]IntOption
	float64OptionsMap     map[string]Float64Option
	stringSliceOptionsMap map[string]StringSliceOption
	registryOnce          sync.Once
)

func ensureRegistryMaps() {
	registryOnce.Do(func() {
		stringOptionsMap = make(map[string]StringOption, len(StringOptions))
		for i := range StringOptions {
			if StringOptions[i].FieldName == "" {
				StringOptions[i].FieldName = PascalCase(StringOptions[i].Name)
			}
			stringOptionsMap[StringOptions[i].Name] = StringOptions[i]
		}

		boolOptionsMap = make(map[string]BoolOption, len(BoolOptions))
		for i := range BoolOptions {
			if BoolOptions[i].FieldName == "" {
				BoolOptions[i].FieldName = PascalCase(BoolOptions[i].Name)
			}
			boolOptionsMap[BoolOptions[i].Name] = BoolOptions[i]
		}

		intOptionsMap = make(map[string]IntOption, len(IntOptions))
		for i := range IntOptions {
			if IntOptions[i].FieldName == "" {
				IntOptions[i].FieldName = PascalCase(IntOptions[i].Name)
			}
			intOptionsMap[IntOptions[i].Name] = IntOptions[i]
		}

		float64OptionsMap = make(map[string]Float64Option, len(Float64Options))
		for i := range Float64Options {
			if Float64Options[i].FieldName == "" {
				Float64Options[i].FieldName = PascalCase(Float64Options[i].Name)
			}
			float64OptionsMap[Float64Options[i].Name] = Float64Options[i]
		}

		stringSliceOptionsMap = make(map[string]StringSliceOption, len(StringSliceOptions))
		for i := range StringSliceOptions {
			if StringSliceOptions[i].FieldName == "" {
				StringSliceOptions[i].FieldName = PascalCase(StringSliceOptions[i].Name)
			}
			stringSliceOptionsMap[StringSliceOptions[i].Name] = StringSliceOptions[i]
		}
	})
}

// GetStringOption returns a string option by its kebab-case name.
func GetStringOption(name string) (StringOption, bool) {
	ensureRegistryMaps()
	opt, ok := stringOptionsMap[name]
	return opt, ok
}

// GetBoolOption returns a boolean option by its kebab-case name.
func GetBoolOption(name string) (BoolOption, bool) {
	ensureRegistryMaps()
	opt, ok := boolOptionsMap[name]
	return opt, ok
}

// GetIntOption returns an integer option by its kebab-case name.
func GetIntOption(name string) (IntOption, bool) {
	ensureRegistryMaps()
	opt, ok := intOptionsMap[name]
	return opt, ok
}

// GetFloat64Option returns a float64 option by its kebab-case name.
func GetFloat64Option(name string) (Float64Option, bool) {
	ensureRegistryMaps()
	opt, ok := float64OptionsMap[name]
	return opt, ok
}

// GetStringSliceOption returns a string slice option by its kebab-case name.
func GetStringSliceOption(name string) (StringSliceOption, bool) {
	ensureRegistryMaps()
	opt, ok := stringSliceOptionsMap[name]
	return opt, ok
}

var initialisms = map[string]string{
	"tty":  "TTY",
	"dns":  "DNS",
	"cpus": "CPUs",
	"ipc":  "IPC",
	"gpus": "GPUs",
}

// PascalCase converts kebab-case (e.g. "dry-run-format") to PascalCase (e.g. "DryRunFormat").
// It respects known initialisms (e.g. "tty" -> "TTY").
func PascalCase(s string) string {
	var builder strings.Builder
	for part := range strings.SplitSeq(s, "-") {
		if part == "" {
			continue
		}
		if val, ok := initialisms[strings.ToLower(part)]; ok {
			builder.WriteString(val)
			continue
		}
		builder.WriteString(strings.ToUpper(part[0:1]))
		builder.WriteString(part[1:])
	}
	return builder.String()
}
