package config

import (
	"strings"
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
		SkipResolution: true, // Phase 5: Path resolution & Auto-detection
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
		SkipResolution: true, // Phase 6: Transitive options
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
		SkipResolution: true, // Phase 6: Transitive options
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
		Name:   "runtime",
		EnvKey: "CDERUN_RUNTIME",
		Usage:  "Container runtime to use (docker/podman)",
		GlobalGetter: func(g CDERunConfig) string {
			return g.Runtime
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
		SkipResolution: true, // Phase 6: Transitive options (comma-separated string)
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
		SkipResolution: true, // Phase 8: Parsed as duration
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
		SkipResolution: true, // Phase 8: Parsed as bytes
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
		Name:   "diagnosis-format",
		EnvKey: "CDERUN_DIAGNOSIS_FORMAT",
		Usage:  "Diagnosis output format (yaml, json, simple)",
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
		Default: "warn",
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
		Usage:   "Grace period after I/O completion before force-terminating the container (e.g. 2s, 500ms)",
		Default: "10s",
		ToolGetter: func(t ToolConfig) string {
			return t.HangTimeout
		},
		GlobalGetter: func(g CDERunConfig) string {
			return g.Defaults.HangTimeout
		},
		SkipResolution: true, // Phase 7: Parsed as duration
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
}

var (
	stringOptionsMap map[string]StringOption
	boolOptionsMap   map[string]BoolOption
)

func init() {
	stringOptionsMap = make(map[string]StringOption, len(StringOptions))
	for i := range StringOptions {
		StringOptions[i].FieldName = PascalCase(StringOptions[i].Name)
		stringOptionsMap[StringOptions[i].Name] = StringOptions[i]
	}

	boolOptionsMap = make(map[string]BoolOption, len(BoolOptions))
	for i := range BoolOptions {
		BoolOptions[i].FieldName = PascalCase(BoolOptions[i].Name)
		boolOptionsMap[BoolOptions[i].Name] = BoolOptions[i]
	}
}

// GetStringOption returns a string option by its kebab-case name.
func GetStringOption(name string) (StringOption, bool) {
	opt, ok := stringOptionsMap[name]
	return opt, ok
}

// GetBoolOption returns a boolean option by its kebab-case name.
func GetBoolOption(name string) (BoolOption, bool) {
	opt, ok := boolOptionsMap[name]
	return opt, ok
}

var initialisms = map[string]string{
	"tty":  "TTY",
	"dns":  "DNS",
	"cpus": "CPUs",
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
