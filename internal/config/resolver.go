package config

import (
	"fmt"
	"strings"
	"time"

	"cderun/internal/container"
	"cderun/internal/logging"

	"github.com/docker/go-units"
)

// ResolvedConfig contains the final values after resolution.
type ResolvedConfig struct {
	HostContext     *HostContext
	Image           string
	TTY             bool
	Interactive     bool
	Network         string
	Remove          bool
	Mounts          []container.Mount
	Env             []string
	Workdir         string
	User            string
	Runtime         string
	SocketPath      string
	MountSocket     bool
	MountSocketPath string
	MountCderun     bool
	MountCderunPath string
	MountTools      []string
	MountAllTools   bool
	DryRun          bool
	DryRunFormat    string
	Diagnosis       bool
	DiagnosisFormat string
	LogLevel        string
	LogFormat       string
	LogTimestamp    bool
	StrictEnv       bool
	HangTimeout     time.Duration

	// Docker-compatible flags
	Ports      []string
	PublishAll bool
	Expose     []string
	Hostname   string
	DNS        []string
	AddHosts   []string
	Privileged bool
	CapAdd     []string
	CapDrop    []string
	Entrypoint []string
	Pull       string
	Memory     int64
	CPUs       float64
	Devices    []container.DeviceMapping
}

// CLIOptions represents values from CLI flags.
type CLIOptions struct {
	Image                    string
	ImageSet                 bool
	TTY                      bool
	TTYSet                   bool
	Interactive              bool
	InteractiveSet           bool
	Network                  string
	NetworkSet               bool
	Remove                   bool
	RemoveSet                bool
	CderunTTY                bool
	CderunTTYSet             bool
	CderunInteractive        bool
	CderunInteractiveSet     bool
	CderunImage              string
	CderunImageSet           bool
	CderunNetwork            string
	CderunNetworkSet         bool
	CderunRemove             bool
	CderunRemoveSet          bool
	Runtime                  string
	RuntimeSet               bool
	CderunRuntime            string
	CderunRuntimeSet         bool
	SocketPath               string
	SocketPathSet            bool
	CderunSocketPath         string
	CderunSocketPathSet      bool
	MountSocket              bool
	MountSocketSet           bool
	CderunMountSocket        bool
	CderunMountSocketSet     bool
	MountSocketPath          string
	MountSocketPathSet       bool
	CderunMountSocketPath    string
	CderunMountSocketPathSet bool
	Env                      []string
	CderunEnv                []string
	Workdir                  string
	WorkdirSet               bool
	CderunWorkdir            string
	CderunWorkdirSet         bool
	Mounts                   []string
	CderunMounts             []string
	MountCderun              bool
	MountCderunSet           bool
	CderunMountCderun        bool
	CderunMountCderunSet     bool
	MountCderunPath          string
	MountCderunPathSet       bool
	CderunMountCderunPath    string
	CderunMountCderunPathSet bool
	MountTools               string
	MountToolsSet            bool
	CderunMountTools         string
	CderunMountToolsSet      bool
	MountAllTools            bool
	MountAllToolsSet         bool
	CderunMountAllTools      bool
	CderunMountAllToolsSet   bool
	DryRun                   bool
	DryRunSet                bool
	CderunDryRun             bool
	CderunDryRunSet          bool
	DryRunFormat             string
	DryRunFormatSet          bool
	CderunDryRunFormat       string
	CderunDryRunFormatSet    bool
	Diagnosis                bool
	DiagnosisSet             bool
	CderunDiagnosis          bool
	CderunDiagnosisSet       bool
	DiagnosisFormat          string
	DiagnosisFormatSet       bool
	CderunDiagnosisFormat    string
	CderunDiagnosisFormatSet bool
	LogLevel                 string
	LogLevelSet              bool
	LogFormat                string
	LogFormatSet             bool
	LogTimestampSet          bool
	LogTimestamp             bool
	StrictEnv                bool
	StrictEnvSet             bool
	CderunStrictEnv          bool
	CderunStrictEnvSet       bool
	CderunLogLevel           string
	CderunLogLevelSet        bool
	CderunLogFormat          string
	CderunLogFormatSet       bool
	CderunLogTimestamp       bool
	CderunLogTimestampSet    bool
	HangTimeout              string
	HangTimeoutSet           bool
	CderunHangTimeout        string
	CderunHangTimeoutSet     bool

	// Docker-compatible flags
	Ports               []string
	CderunPorts         []string
	PublishAll          bool
	PublishAllSet       bool
	CderunPublishAll    bool
	CderunPublishAllSet bool
	Expose              []string
	CderunExpose        []string
	Hostname            string
	HostnameSet         bool
	CderunHostname      string
	CderunHostnameSet   bool
	DNS                 []string
	CderunDNS           []string
	AddHosts            []string
	CderunAddHosts      []string
	User                string
	UserSet             bool
	CderunUser          string
	CderunUserSet       bool
	Privileged          bool
	PrivilegedSet       bool
	CderunPrivileged    bool
	CderunPrivilegedSet bool
	CapAdd              []string
	CderunCapAdd        []string
	CapDrop             []string
	CderunCapDrop       []string
	Entrypoint          []string
	CderunEntrypoint    []string
	Pull                string
	PullSet             bool
	CderunPull          string
	CderunPullSet       bool
	Memory              string
	MemorySet           bool
	CderunMemory        string
	CderunMemorySet     bool
	CPUs                float64
	CPUsSet             bool
	CderunCPUs          float64
	CderunCPUsSet       bool
	Devices             []string
	CderunDevices       []string
}

// Resolve combines CLI flags, environment variables, tool-specific config, and global defaults.
func Resolve(subcommand string, cli CLIOptions, tools ToolsConfig, global *CDERunConfig) (*ResolvedConfig, error) {
	return ResolveWithFS(subcommand, cli, tools, global, RealFileSystem{})
}

// ResolveWithFS combines CLI flags, environment variables, tool-specific config, and global defaults using the provided filesystem.
func ResolveWithFS(subcommand string, cli CLIOptions, tools ToolsConfig, global *CDERunConfig, fs FileSystem) (*ResolvedConfig, error) {
	logging.Trace("Resolving configurations for tool: %s", subcommand)
	res := &ResolvedConfig{}
	var err error

	var hostCtx *HostContext
	if global != nil {
		hostCtx = global.HostContext
	}

	r, err := NewExpressionResolverWithFS(hostCtx, fs)
	if err != nil {
		return nil, fmt.Errorf("failed to create expression resolver: %w", err)
	}

	// 0. Resolve Diagnosis
	// This is resolved early because diagnosis mode bypasses some validations.
	res.Diagnosis = resolveBoolOpt(
		OptionDef[*bool]{EnvKey: "CDERUN_DIAGNOSIS",
			ToolGetter:   func(t ToolConfig) *bool { return t.Diagnosis },
			GlobalGetter: func(g CDERunConfig) *bool { return g.Defaults.Diagnosis }},
		false,
		cli.CderunDiagnosisSet, cli.CderunDiagnosis,
		cli.DiagnosisSet, cli.Diagnosis,
		subcommand, tools, global, fs,
	)

	// 1. Resolve Image
	res.Image = resolveStringOpt(
		OptionDef[string]{EnvKey: "CDERUN_IMAGE",
			ToolGetter:   func(t ToolConfig) string { return t.Image },
			GlobalGetter: func(g CDERunConfig) string { return "" }},
		cli.CderunImageSet, cli.CderunImage,
		cli.ImageSet, cli.Image,
		subcommand, tools, global, r, fs,
	)

	if res.Image == "" && subcommand != "" && !res.Diagnosis {
		return nil, fmt.Errorf("no image mapping found for tool: %s", subcommand)
	}
	if res.Image != "" {
		logging.Debug("Resolved Image: %s", res.Image)
	}

	// 2. Resolve TTY
	res.TTY = resolveBoolOpt(
		OptionDef[*bool]{EnvKey: "CDERUN_TTY",
			ToolGetter:   func(t ToolConfig) *bool { return t.TTY },
			GlobalGetter: func(g CDERunConfig) *bool { return g.Defaults.TTY }},
		false,
		cli.CderunTTYSet, cli.CderunTTY,
		cli.TTYSet, cli.TTY,
		subcommand, tools, global, fs,
	)

	// 3. Resolve Interactive
	res.Interactive = resolveBoolOpt(
		OptionDef[*bool]{EnvKey: "CDERUN_INTERACTIVE",
			ToolGetter:   func(t ToolConfig) *bool { return t.Interactive },
			GlobalGetter: func(g CDERunConfig) *bool { return g.Defaults.Interactive }},
		false,
		cli.CderunInteractiveSet, cli.CderunInteractive,
		cli.InteractiveSet, cli.Interactive,
		subcommand, tools, global, fs,
	)

	// 4. Resolve Network
	res.Network = resolveStringOpt(
		OptionDef[string]{EnvKey: "CDERUN_NETWORK",
			ToolGetter:   func(t ToolConfig) string { return t.Network },
			GlobalGetter: func(g CDERunConfig) string { return g.Defaults.Network },
			Fallback:     "bridge"},
		cli.CderunNetworkSet, cli.CderunNetwork,
		cli.NetworkSet, cli.Network,
		subcommand, tools, global, r, fs,
	)

	// 5. Resolve Remove
	res.Remove = resolveBoolOpt(
		OptionDef[*bool]{EnvKey: "CDERUN_REMOVE",
			ToolGetter:   func(t ToolConfig) *bool { return t.Remove },
			GlobalGetter: func(g CDERunConfig) *bool { return g.Defaults.Remove }},
		true,
		cli.CderunRemoveSet, cli.CderunRemove,
		cli.RemoveSet, cli.Remove,
		subcommand, tools, global, fs,
	)

	// 7. Resolve Workdir
	res.Workdir = resolveStringOpt(
		OptionDef[string]{EnvKey: "CDERUN_WORKDIR",
			ToolGetter:   func(t ToolConfig) string { return t.Workdir },
			GlobalGetter: func(g CDERunConfig) string { return g.Defaults.Workdir }},
		cli.CderunWorkdirSet, cli.CderunWorkdir,
		cli.WorkdirSet, cli.Workdir,
		subcommand, tools, global, r, fs,
	)

	// 8. Resolve Mounts (P1 > P2 > P4)
	res.Mounts, err = resolveMounts(cli.CderunMounts, cli.Mounts, subcommand, tools, global, r, fs)
	if err != nil {
		return nil, err
	}

	// 10. Resolve StrictEnv
	res.StrictEnv = resolveBoolOpt(
		OptionDef[*bool]{EnvKey: "CDERUN_STRICT_ENV",
			ToolGetter:   func(t ToolConfig) *bool { return t.StrictEnv },
			GlobalGetter: func(g CDERunConfig) *bool { return g.Defaults.StrictEnv }},
		false,
		cli.CderunStrictEnvSet, cli.CderunStrictEnv,
		cli.StrictEnvSet, cli.StrictEnv,
		subcommand, tools, global, fs,
	)

	// 11. Resolve Env (P1 > P2 > Env (P3) > Tool (P4) > Global (P5))
	res.Env, err = resolveEnv(cli.CderunEnv, cli.Env, "CDERUN_ENV", subcommand, tools, global, res.StrictEnv, r, fs)
	if err != nil {
		return nil, err
	}

	// 12. Resolve Runtime & Socket with Auto-detection
	res.Runtime = resolveStringOpt(
		OptionDef[string]{EnvKey: "CDERUN_RUNTIME",
			GlobalGetter: func(g CDERunConfig) string { return g.Runtime }},
		cli.CderunRuntimeSet, cli.CderunRuntime,
		cli.RuntimeSet, cli.Runtime,
		"", nil, global, r, fs,
	)

	res.SocketPath, err = resolveConfigPath(
		cli.CderunSocketPathSet, cli.CderunSocketPath,
		cli.SocketPathSet, cli.SocketPath,
		"CDERUN_SOCKET_PATH",
		"", nil, nil,
		global, func(g CDERunConfig) ConfigPath { return g.SocketPath },
		"", // Fallback to empty for auto-detection
		r,
		"path",
		fs,
	)
	if err != nil {
		return nil, err
	}

	// Auto-detection logic
	if res.Runtime == "" {
		if res.SocketPath != "" {
			// Infer runtime from socket path
			if strings.Contains(res.SocketPath, "podman") {
				res.Runtime = "podman"
			} else {
				res.Runtime = "docker"
			}
		} else {
			// Check default socket paths
			if _, err := fs.Stat("/var/run/docker.sock"); err == nil {
				res.Runtime = "docker"
				res.SocketPath = "/var/run/docker.sock"
			} else if _, err := fs.Stat("/run/podman/podman.sock"); err == nil {
				res.Runtime = "podman"
				res.SocketPath = "/run/podman/podman.sock"
			} else {
				// Default fallback
				res.Runtime = "docker"
				res.SocketPath = "/var/run/docker.sock"
			}
		}
	}

	if res.SocketPath == "" {
		// Runtime was specified but socket was not
		if res.Runtime == "podman" {
			res.SocketPath = "/run/podman/podman.sock"
		} else {
			res.SocketPath = "/var/run/docker.sock"
		}
	}

	// Special handling for unix:// prefix for the host-side socket path
	res.SocketPath = strings.TrimPrefix(res.SocketPath, "unix://")

	// 13. Resolve MountTools and MountAllTools
	res.MountTools = resolveStringSliceCommaOpt(
		OptionDef[[]string]{EnvKey: "CDERUN_MOUNT_TOOLS",
			ToolGetter:   func(t ToolConfig) []string { return t.MountTools },
			GlobalGetter: func(g CDERunConfig) []string { return g.Defaults.MountTools }},
		cli.CderunMountToolsSet, cli.CderunMountTools,
		cli.MountToolsSet, cli.MountTools,
		subcommand, tools, global, r, fs,
	)

	res.MountAllTools = resolveBoolOpt(
		OptionDef[*bool]{EnvKey: "CDERUN_MOUNT_ALL_TOOLS",
			ToolGetter:   func(t ToolConfig) *bool { return t.MountAllTools },
			GlobalGetter: func(g CDERunConfig) *bool { return g.Defaults.MountAllTools }},
		false,
		cli.CderunMountAllToolsSet, cli.CderunMountAllTools,
		cli.MountAllToolsSet, cli.MountAllTools,
		subcommand, tools, global, fs,
	)

	// 14. Resolve MountCderun
	var mountCderunSpecified bool
	res.MountCderun, mountCderunSpecified = resolveBoolOptInfo(
		OptionDef[*bool]{EnvKey: "CDERUN_MOUNT_CDERUN",
			ToolGetter:   func(t ToolConfig) *bool { return t.MountCderun },
			GlobalGetter: func(g CDERunConfig) *bool { return g.Defaults.MountCderun }},
		cli.CderunMountCderunSet, cli.CderunMountCderun,
		cli.MountCderunSet, cli.MountCderun,
		subcommand, tools, global, fs,
	)
	if !mountCderunSpecified {
		// Transitive auto-enablement: tools -> cderun
		res.MountCderun = len(res.MountTools) > 0 || res.MountAllTools
	}

	res.MountCderunPath, err = resolveConfigPath(
		cli.CderunMountCderunPathSet, cli.CderunMountCderunPath,
		cli.MountCderunPathSet, cli.MountCderunPath,
		"CDERUN_MOUNT_CDERUN_PATH",
		subcommand, tools, func(t ToolConfig) ConfigPath { return t.MountCderunPath },
		global, func(g CDERunConfig) ConfigPath { return g.Defaults.MountCderunPath },
		"",
		r,
		"path",
		fs,
	)
	if err != nil {
		return nil, err
	}

	// 15. Resolve MountSocket and MountSocketPath
	var mountSocketSpecified bool
	res.MountSocket, mountSocketSpecified = resolveBoolOptInfo(
		OptionDef[*bool]{EnvKey: "CDERUN_MOUNT_SOCKET",
			ToolGetter:   func(t ToolConfig) *bool { return t.MountSocket },
			GlobalGetter: func(g CDERunConfig) *bool { return g.Defaults.MountSocket }},
		cli.CderunMountSocketSet, cli.CderunMountSocket,
		cli.MountSocketSet, cli.MountSocket,
		subcommand, tools, global, fs,
	)
	if !mountSocketSpecified {
		// Transitive auto-enablement: cderun -> socket
		res.MountSocket = res.MountCderun
	}

	res.MountSocketPath, err = resolveConfigPath(
		cli.CderunMountSocketPathSet, cli.CderunMountSocketPath,
		cli.MountSocketPathSet, cli.MountSocketPath,
		"CDERUN_MOUNT_SOCKET_PATH",
		subcommand, tools, func(t ToolConfig) ConfigPath { return t.MountSocketPath },
		global, func(g CDERunConfig) ConfigPath { return g.Defaults.MountSocketPath },
		res.SocketPath, // Default to host-side socket path
		r,
		"path",
		fs,
	)
	if err != nil {
		return nil, err
	}

	// Resolve HangTimeout
	hangTimeoutStr := resolveStringOpt(
		OptionDef[string]{EnvKey: "CDERUN_HANG_TIMEOUT",
			ToolGetter:   func(t ToolConfig) string { return t.HangTimeout },
			GlobalGetter: func(g CDERunConfig) string { return g.Defaults.HangTimeout },
			Fallback:     "2s"},
		cli.CderunHangTimeoutSet, cli.CderunHangTimeout,
		cli.HangTimeoutSet, cli.HangTimeout,
		subcommand, tools, global, r, fs,
	)
	if hangTimeoutStr != "" {
		if d, err := time.ParseDuration(hangTimeoutStr); err == nil {
			res.HangTimeout = d
		} else {
			return nil, fmt.Errorf("invalid hang-timeout value %q: %w", hangTimeoutStr, err)
		}
	}

	// Resolve DryRun
	res.DryRun = resolveBoolOpt(
		OptionDef[*bool]{EnvKey: "CDERUN_DRY_RUN",
			ToolGetter:   func(t ToolConfig) *bool { return t.DryRun },
			GlobalGetter: func(g CDERunConfig) *bool { return g.Defaults.DryRun }},
		false,
		cli.CderunDryRunSet, cli.CderunDryRun,
		cli.DryRunSet, cli.DryRun,
		subcommand, tools, global, fs,
	)

	// Resolve DryRunFormat
	res.DryRunFormat = resolveStringOpt(
		OptionDef[string]{EnvKey: "CDERUN_DRY_RUN_FORMAT",
			ToolGetter:   func(t ToolConfig) string { return t.DryRunFormat },
			GlobalGetter: func(g CDERunConfig) string { return g.Defaults.DryRunFormat },
			Fallback:     "yaml"},
		cli.CderunDryRunFormatSet, cli.CderunDryRunFormat,
		cli.DryRunFormatSet, cli.DryRunFormat,
		subcommand, tools, global, r, fs,
	)

	// Resolve DiagnosisFormat
	res.DiagnosisFormat = resolveStringOpt(
		OptionDef[string]{EnvKey: "CDERUN_DIAGNOSIS_FORMAT",
			ToolGetter:   func(t ToolConfig) string { return t.DiagnosisFormat },
			GlobalGetter: func(g CDERunConfig) string { return g.Defaults.DiagnosisFormat },
			Fallback:     "yaml"},
		cli.CderunDiagnosisFormatSet, cli.CderunDiagnosisFormat,
		cli.DiagnosisFormatSet, cli.DiagnosisFormat,
		subcommand, tools, global, r, fs,
	)


	// Resolve Logging
	res.LogLevel = resolveStringOpt(
		OptionDef[string]{EnvKey: "CDERUN_LOG_LEVEL",
			ToolGetter:   func(t ToolConfig) string { return t.LogLevel },
			GlobalGetter: func(g CDERunConfig) string { return g.Logging.Level },
			Fallback:     "warn"},
		cli.CderunLogLevelSet, cli.CderunLogLevel,
		cli.LogLevelSet, cli.LogLevel,
		subcommand, tools, global, r, fs,
	)
	res.LogFormat = resolveStringOpt(
		OptionDef[string]{EnvKey: "CDERUN_LOG_FORMAT",
			ToolGetter:   func(t ToolConfig) string { return t.LogFormat },
			GlobalGetter: func(g CDERunConfig) string { return g.Logging.Format },
			Fallback:     "text"},
		cli.CderunLogFormatSet, cli.CderunLogFormat,
		cli.LogFormatSet, cli.LogFormat,
		subcommand, tools, global, r, fs,
	)
	res.LogTimestamp = resolveBoolOpt(
		OptionDef[*bool]{EnvKey: "CDERUN_LOG_TIMESTAMP",
			ToolGetter:   func(t ToolConfig) *bool { return t.LogTimestamp },
			GlobalGetter: func(g CDERunConfig) *bool { return g.Logging.Timestamp }},
		true,
		cli.CderunLogTimestampSet, cli.CderunLogTimestamp,
		cli.LogTimestampSet, cli.LogTimestamp,
		subcommand, tools, global, fs,
	)

	// Resolve Docker-compatible flags
	res.Ports = resolveStringSliceOpt(
		OptionDef[[]string]{EnvKey: "CDERUN_PUBLISH",
			ToolGetter:   func(t ToolConfig) []string { return t.Ports },
			GlobalGetter: func(g CDERunConfig) []string { return g.Defaults.Ports }},
		",", cli.CderunPorts, cli.Ports, subcommand, tools, global, r, fs,
	)
	res.PublishAll = resolveBoolOpt(
		OptionDef[*bool]{EnvKey: "CDERUN_PUBLISH_ALL",
			ToolGetter:   func(t ToolConfig) *bool { return t.PublishAll },
			GlobalGetter: func(g CDERunConfig) *bool { return g.Defaults.PublishAll }},
		false,
		cli.CderunPublishAllSet, cli.CderunPublishAll,
		cli.PublishAllSet, cli.PublishAll,
		subcommand, tools, global, fs,
	)
	res.Expose = resolveStringSliceOpt(
		OptionDef[[]string]{EnvKey: "CDERUN_EXPOSE",
			ToolGetter:   func(t ToolConfig) []string { return t.Expose },
			GlobalGetter: func(g CDERunConfig) []string { return g.Defaults.Expose }},
		",", cli.CderunExpose, cli.Expose, subcommand, tools, global, r, fs,
	)
	res.Hostname = resolveStringOpt(
		OptionDef[string]{EnvKey: "CDERUN_HOSTNAME",
			ToolGetter:   func(t ToolConfig) string { return t.Hostname },
			GlobalGetter: func(g CDERunConfig) string { return g.Defaults.Hostname }},
		cli.CderunHostnameSet, cli.CderunHostname,
		cli.HostnameSet, cli.Hostname,
		subcommand, tools, global, r, fs,
	)
	res.DNS = resolveStringSliceOpt(
		OptionDef[[]string]{EnvKey: "CDERUN_DNS",
			ToolGetter:   func(t ToolConfig) []string { return t.DNS },
			GlobalGetter: func(g CDERunConfig) []string { return g.Defaults.DNS }},
		",", cli.CderunDNS, cli.DNS, subcommand, tools, global, r, fs,
	)
	res.AddHosts = resolveStringSliceOpt(
		OptionDef[[]string]{EnvKey: "CDERUN_ADD_HOST",
			ToolGetter:   func(t ToolConfig) []string { return t.AddHosts },
			GlobalGetter: func(g CDERunConfig) []string { return g.Defaults.AddHosts }},
		",", cli.CderunAddHosts, cli.AddHosts, subcommand, tools, global, r, fs,
	)
	res.User = resolveStringOpt(
		OptionDef[string]{EnvKey: "CDERUN_USER",
			ToolGetter:   func(t ToolConfig) string { return t.User },
			GlobalGetter: func(g CDERunConfig) string { return g.Defaults.User }},
		cli.CderunUserSet, cli.CderunUser,
		cli.UserSet, cli.User,
		subcommand, tools, global, r, fs,
	)
	res.Privileged = resolveBoolOpt(
		OptionDef[*bool]{EnvKey: "CDERUN_PRIVILEGED",
			ToolGetter:   func(t ToolConfig) *bool { return t.Privileged },
			GlobalGetter: func(g CDERunConfig) *bool { return g.Defaults.Privileged }},
		false,
		cli.CderunPrivilegedSet, cli.CderunPrivileged,
		cli.PrivilegedSet, cli.Privileged,
		subcommand, tools, global, fs,
	)
	res.CapAdd = resolveStringSliceOpt(
		OptionDef[[]string]{EnvKey: "CDERUN_CAP_ADD",
			ToolGetter:   func(t ToolConfig) []string { return t.CapAdd },
			GlobalGetter: func(g CDERunConfig) []string { return g.Defaults.CapAdd }},
		",", cli.CderunCapAdd, cli.CapAdd, subcommand, tools, global, r, fs,
	)
	res.CapDrop = resolveStringSliceOpt(
		OptionDef[[]string]{EnvKey: "CDERUN_CAP_DROP",
			ToolGetter:   func(t ToolConfig) []string { return t.CapDrop },
			GlobalGetter: func(g CDERunConfig) []string { return g.Defaults.CapDrop }},
		",", cli.CderunCapDrop, cli.CapDrop, subcommand, tools, global, r, fs,
	)
	res.Entrypoint = resolveStringSliceOpt(
		OptionDef[[]string]{EnvKey: "CDERUN_ENTRYPOINT",
			ToolGetter:   func(t ToolConfig) []string { return t.Entrypoint },
			GlobalGetter: func(g CDERunConfig) []string { return g.Defaults.Entrypoint }},
		",", cli.CderunEntrypoint, cli.Entrypoint, subcommand, tools, global, r, fs,
	)
	res.Pull = resolveStringOpt(
		OptionDef[string]{EnvKey: "CDERUN_PULL",
			ToolGetter:   func(t ToolConfig) string { return t.Pull },
			GlobalGetter: func(g CDERunConfig) string { return g.Defaults.Pull },
			Fallback:     "missing"},
		cli.CderunPullSet, cli.CderunPull,
		cli.PullSet, cli.Pull,
		subcommand, tools, global, r, fs,
	)
	res.Devices, err = resolveDevices(cli.CderunDevices, cli.Devices, subcommand, tools, global, r, fs)
	if err != nil {
		return nil, err
	}
	memStr := resolveStringOpt(
		OptionDef[string]{EnvKey: "CDERUN_MEMORY",
			ToolGetter:   func(t ToolConfig) string { return t.Memory },
			GlobalGetter: func(g CDERunConfig) string { return g.Defaults.Memory }},
		cli.CderunMemorySet, cli.CderunMemory,
		cli.MemorySet, cli.Memory,
		subcommand, tools, global, r, fs,
	)
	if memStr != "" {
		bytes, err := units.RAMInBytes(memStr)
		if err != nil {
			if exprErr := r.Error(); exprErr != nil {
				return nil, exprErr
			}
			return nil, fmt.Errorf("invalid memory value %q: %w", memStr, err)
		}
		res.Memory = bytes
	}
	res.CPUs = resolveFloat64Opt(
		OptionDef[*float64]{EnvKey: "CDERUN_CPUS",
			ToolGetter:   func(t ToolConfig) *float64 { return t.CPUs },
			GlobalGetter: func(g CDERunConfig) *float64 { return g.Defaults.CPUs }},
		cli.CderunCPUsSet, cli.CderunCPUs,
		cli.CPUsSet, cli.CPUs,
		subcommand, tools, global, fs,
	)

	res.HostContext = hostCtx

	if err := r.Error(); err != nil {
		return nil, err
	}

	return res, nil
}

func resolveConfigPath(p1Set bool, p1Val string, cliSet bool, cliVal string, envKey string, subcommand string, tools ToolsConfig, toolGetter func(ToolConfig) ConfigPath, global *CDERunConfig, globalGetter func(CDERunConfig) ConfigPath, fallback string, r *ExpressionResolver, pathType string, fs FileSystem) (string, error) {
	var cp ConfigPath
	if p1Set {
		cp = ConfigPath{Raw: p1Val, BaseDir: r.Pwd}
	} else if cliSet {
		cp = ConfigPath{Raw: cliVal, BaseDir: r.Pwd}
	} else if env := fs.Getenv(envKey); env != "" {
		cp = ConfigPath{Raw: env, BaseDir: r.Pwd}
	} else {
		found := false
		if tools != nil {
			if tool, ok := tools[subcommand]; ok {
				if t := toolGetter(tool); !t.IsEmpty() {
					cp = t
					found = true
				}
			}
		}
		if !found && global != nil {
			if g := globalGetter(*global); !g.IsEmpty() {
				cp = g
				found = true
			}
		}
		if !found {
			cp = ConfigPath{Raw: fallback, BaseDir: r.Pwd}
		}
	}

	switch pathType {
	case "volume":
		return cp.ResolveVolume(r)
	case "device":
		return cp.ResolveDevice(r)
	default:
		return cp.Resolve(r)
	}
}

func resolveDevices(p1 []string, p2 []string, subcommand string, tools ToolsConfig, global *CDERunConfig, r *ExpressionResolver, fs FileSystem) ([]container.DeviceMapping, error) {
	var dcs []DeviceConfig

	if p1 != nil {
		dcs = []DeviceConfig{}
		for _, d := range p1 {
			parsed, ok := ParseDeviceConfig(d)
			if !ok {
				return nil, fmt.Errorf("invalid device config (override): %s", d)
			}
			parsed.SetBaseDir(r.Pwd)
			dcs = append(dcs, parsed)
		}
	} else if p2 != nil {
		dcs = []DeviceConfig{}
		for _, d := range p2 {
			parsed, ok := ParseDeviceConfig(d)
			if !ok {
				return nil, fmt.Errorf("invalid device config: %s", d)
			}
			parsed.SetBaseDir(r.Pwd)
			dcs = append(dcs, parsed)
		}
	} else if env, ok := fs.LookupEnv("CDERUN_DEVICE"); ok {
		dcs = []DeviceConfig{}
		for d := range strings.SplitSeq(env, ",") {
			d = strings.TrimSpace(d)
			if d == "" {
				continue
			}
			parsed, ok := ParseDeviceConfig(d)
			if !ok {
				return nil, fmt.Errorf("invalid device config in CDERUN_DEVICE: %s", d)
			}
			parsed.SetBaseDir(r.Pwd)
			dcs = append(dcs, parsed)
		}
	} else if tools != nil {
		if tool, ok := tools[subcommand]; ok && tool.Devices != nil {
			dcs = tool.Devices
		}
	}

	if dcs == nil && global != nil {
		dcs = global.Defaults.Devices
	}

	var res []container.DeviceMapping
	for _, dc := range dcs {
		resolved, err := dc.Resolve(r)
		if err != nil {
			return nil, err
		}
		res = append(res, resolved)
	}
	return res, nil
}

func resolveEnv(p1 []string, p2 []string, envKey string, subcommand string, tools ToolsConfig, global *CDERunConfig, strict bool, r *ExpressionResolver, fs FileSystem) ([]string, error) {
	var envs []string

	if p1 != nil {
		envs = p1
	} else if p2 != nil {
		envs = p2
	} else if env, ok := fs.LookupEnv(envKey); ok {
		envs = []string{}
		for e := range strings.SplitSeq(env, ";") {
			e = strings.TrimSpace(e)
			if e == "" {
				continue
			}
			envs = append(envs, e)
		}
	} else if tools != nil {
		if tool, ok := tools[subcommand]; ok && tool.Env != nil {
			envs = tool.Env
		}
	}

	if envs == nil && global != nil {
		envs = global.Defaults.Env
	}

	// Deduplicate within the winning source (last-one-wins for the same key)
	// We use mergeEnv with nil/nil for other sources to leverage its deduplication logic.
	merged := mergeEnv(nil, nil, envs)

	return resolveEnvValues(merged, strict, r, fs)
}

func mergeEnv(base, p2, p1 []string) []string {
	m := make(map[string]string)
	var keys []string

	add := func(env []string) {
		for _, e := range env {
			key := strings.SplitN(e, "=", 2)[0]
			if _, ok := m[key]; !ok {
				keys = append(keys, key)
			}
			m[key] = e
		}
	}

	add(base)
	add(p2)
	add(p1)

	var res []string
	for _, k := range keys {
		res = append(res, m[k])
	}
	return res
}

func resolveEnvValues(env []string, strict bool, r *ExpressionResolver, fs FileSystem) ([]string, error) {
	var res []string
	for _, e := range env {
		resolvedE := r.resolveString(e)
		if err := r.Error(); err != nil {
			return nil, err
		}
		if strings.Contains(resolvedE, "=") {
			res = append(res, resolvedE)
		} else {
			val, found := fs.LookupEnv(resolvedE)
			if !found && strict {
				return nil, fmt.Errorf("required environment variable not found: %s", resolvedE)
			}
			res = append(res, fmt.Sprintf("%s=%s", resolvedE, val))
		}
	}
	return res, nil
}

func resolveMounts(p1 []string, p2 []string, subcommand string, tools ToolsConfig, global *CDERunConfig, r *ExpressionResolver, fs FileSystem) ([]container.Mount, error) {
	var mcs []MountConfig

	if p1 != nil {
		mcs = []MountConfig{}
		for _, m := range p1 {
			parsed, err := ParseMountFlag(m)
			if err != nil {
				return nil, fmt.Errorf("invalid mount config (override): %w", err)
			}
			parsed.SetBaseDir(r.Pwd)
			mcs = append(mcs, parsed)
		}
	} else if p2 != nil {
		mcs = []MountConfig{}
		for _, m := range p2 {
			parsed, err := ParseMountFlag(m)
			if err != nil {
				return nil, fmt.Errorf("invalid mount config: %w", err)
			}
			parsed.SetBaseDir(r.Pwd)
			mcs = append(mcs, parsed)
		}
	} else if env, ok := fs.LookupEnv("CDERUN_MOUNT"); ok {
		mcs = []MountConfig{}
		for m := range strings.SplitSeq(env, ";") {
			m = strings.TrimSpace(m)
			if m == "" {
				continue
			}
			parsed, err := ParseMountFlag(m)
			if err != nil {
				return nil, fmt.Errorf("invalid mount config in CDERUN_MOUNT: %w", err)
			}
			parsed.SetBaseDir(r.Pwd)
			mcs = append(mcs, parsed)
		}
	} else if tools != nil {
		if tool, ok := tools[subcommand]; ok && tool.Mounts != nil {
			mcs = tool.Mounts
		}
	}

	if mcs == nil && global != nil {
		mcs = global.Defaults.Mounts
	}

	var res []container.Mount
	for _, mc := range mcs {
		resolved, err := mc.Resolve(r)
		if err != nil {
			return nil, err
		}
		res = append(res, resolved)
	}
	return res, nil
}
