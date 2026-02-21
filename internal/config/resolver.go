package config

import (
	"cderun/internal/container"
	"cderun/internal/logging"
	"fmt"
	"os"
	"strconv"
	"strings"

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
	Command    []string
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
	CderunLogLevel           string
	CderunLogLevelSet        bool
	CderunLogFormat          string
	CderunLogFormatSet       bool
	CderunLogTimestamp       bool
	CderunLogTimestampSet    bool

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

	// 0. Resolve Diagnosis (CLI/Env only)
	// This is resolved early because diagnosis mode bypasses some validations.
	res.Diagnosis = resolveBool(
		cli.CderunDiagnosisSet, cli.CderunDiagnosis,
		cli.DiagnosisSet, cli.Diagnosis,
		"CDERUN_DIAGNOSIS",
		"", nil, nil,
		nil, nil,
		false,
	)

	// 1. Resolve Image (Step 10.1: Subcommand as key)
	// Priority: P1 CLI > P2 CLI > Env (P3) > Tool Config (P4) > Global Defaults (P5)
	res.Image = resolveString(
		cli.CderunImageSet, cli.CderunImage,
		cli.ImageSet, cli.Image,
		"CDERUN_IMAGE",
		subcommand, tools, func(t ToolConfig) string { return t.Image },
		global, func(g CDERunConfig) string { return "" },
		"",
		r,
	)

	if res.Image == "" && subcommand != "" && !res.Diagnosis {
		return nil, fmt.Errorf("no image mapping found for tool: %s", subcommand)
	}
	if res.Image != "" {
		logging.Debug("Resolved Image: %s", res.Image)
	}

	// 2. Resolve TTY
	res.TTY = resolveBool(
		cli.CderunTTYSet, cli.CderunTTY,
		cli.TTYSet, cli.TTY,
		"CDERUN_TTY",
		subcommand, tools, func(t ToolConfig) *bool { return t.TTY },
		global, func(g CDERunConfig) *bool { return g.Defaults.TTY },
		false,
	)

	// 3. Resolve Interactive
	res.Interactive = resolveBool(
		cli.CderunInteractiveSet, cli.CderunInteractive,
		cli.InteractiveSet, cli.Interactive,
		"CDERUN_INTERACTIVE",
		subcommand, tools, func(t ToolConfig) *bool { return t.Interactive },
		global, func(g CDERunConfig) *bool { return g.Defaults.Interactive },
		false,
	)

	// 4. Resolve Network
	res.Network = resolveString(
		cli.CderunNetworkSet, cli.CderunNetwork,
		cli.NetworkSet, cli.Network,
		"CDERUN_NETWORK",
		subcommand, tools, func(t ToolConfig) string { return t.Network },
		global, func(g CDERunConfig) string { return g.Defaults.Network },
		"bridge",
		r,
	)

	// 5. Resolve Remove
	res.Remove = resolveBool(
		cli.CderunRemoveSet, cli.CderunRemove,
		cli.RemoveSet, cli.Remove,
		"CDERUN_REMOVE",
		subcommand, tools, func(t ToolConfig) *bool { return t.Remove },
		global, func(g CDERunConfig) *bool { return g.Defaults.Remove },
		true, // Default to true as per docs
	)

	// 7. Resolve Workdir
	res.Workdir = resolveString(
		cli.CderunWorkdirSet, cli.CderunWorkdir,
		cli.WorkdirSet, cli.Workdir,
		"CDERUN_WORKDIR",
		subcommand, tools, func(t ToolConfig) string { return t.Workdir },
		global, func(g CDERunConfig) string { return g.Defaults.Workdir },
		"",
		r,
	)

	// 8. Resolve Mounts (P1 > P2 > P4)
	res.Mounts, err = resolveMounts(cli.CderunMounts, cli.Mounts, subcommand, tools, global, r)
	if err != nil {
		return nil, err
	}

	// 10. Resolve StrictEnv
	res.StrictEnv = resolveBool(
		false, false, // No P1 for strictEnv yet
		false, false, // No P2 for strictEnv yet
		"CDERUN_STRICT_ENV",
		subcommand, tools, func(t ToolConfig) *bool { return t.StrictEnv },
		global, func(g CDERunConfig) *bool { return g.Defaults.StrictEnv },
		false,
	)

	// 11. Resolve Env (P1 > P2 > Env (P3) > Tool (P4) > Global (P5))
	res.Env, err = resolveEnv(cli.CderunEnv, cli.Env, "CDERUN_ENV", subcommand, tools, global, res.StrictEnv, r)
	if err != nil {
		return nil, err
	}

	// 12. Resolve Runtime & Socket with Auto-detection
	res.Runtime = resolveString(
		cli.CderunRuntimeSet, cli.CderunRuntime,
		cli.RuntimeSet, cli.Runtime,
		"CDERUN_RUNTIME",
		"", nil, nil, // No tool-specific runtime
		global, func(g CDERunConfig) string { return g.Runtime },
		"", // Fallback to empty for auto-detection
		r,
	)

	res.SocketPath = resolveConfigPath(
		cli.CderunSocketPathSet, cli.CderunSocketPath,
		cli.SocketPathSet, cli.SocketPath,
		"CDERUN_SOCKET_PATH",
		"", nil, nil,
		global, func(g CDERunConfig) ConfigPath { return g.SocketPath },
		"", // Fallback to empty for auto-detection
		r,
		"path",
	)

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
	res.MountTools = resolveStringSliceComma(
		cli.CderunMountToolsSet, cli.CderunMountTools,
		cli.MountToolsSet, cli.MountTools,
		"CDERUN_MOUNT_TOOLS",
		subcommand, tools, func(t ToolConfig) []string { return t.MountTools },
		global, func(g CDERunConfig) []string { return g.Defaults.MountTools },
		r,
	)

	res.MountAllTools = resolveBool(
		cli.CderunMountAllToolsSet, cli.CderunMountAllTools,
		cli.MountAllToolsSet, cli.MountAllTools,
		"CDERUN_MOUNT_ALL_TOOLS",
		subcommand, tools, func(t ToolConfig) *bool { return t.MountAllTools },
		global, func(g CDERunConfig) *bool { return g.Defaults.MountAllTools },
		false,
	)

	// 14. Resolve MountCderun
	var mountCderunSpecified bool
	res.MountCderun, mountCderunSpecified = resolveBoolInfo(
		cli.CderunMountCderunSet, cli.CderunMountCderun,
		cli.MountCderunSet, cli.MountCderun,
		"CDERUN_MOUNT_CDERUN",
		subcommand, tools, func(t ToolConfig) *bool { return t.MountCderun },
		global, func(g CDERunConfig) *bool { return g.Defaults.MountCderun },
	)
	if !mountCderunSpecified {
		// Transitive auto-enablement: tools -> cderun
		res.MountCderun = len(res.MountTools) > 0 || res.MountAllTools
	}

	res.MountCderunPath = resolveConfigPath(
		cli.CderunMountCderunPathSet, cli.CderunMountCderunPath,
		cli.MountCderunPathSet, cli.MountCderunPath,
		"CDERUN_MOUNT_CDERUN_PATH",
		subcommand, tools, func(t ToolConfig) ConfigPath { return t.MountCderunPath },
		global, func(g CDERunConfig) ConfigPath { return g.Defaults.MountCderunPath },
		"",
		r,
		"path",
	)

	// 15. Resolve MountSocket and MountSocketPath
	var mountSocketSpecified bool
	res.MountSocket, mountSocketSpecified = resolveBoolInfo(
		cli.CderunMountSocketSet, cli.CderunMountSocket,
		cli.MountSocketSet, cli.MountSocket,
		"CDERUN_MOUNT_SOCKET",
		subcommand, tools, func(t ToolConfig) *bool { return t.MountSocket },
		global, func(g CDERunConfig) *bool { return g.Defaults.MountSocket },
	)
	if !mountSocketSpecified {
		// Transitive auto-enablement: cderun -> socket
		res.MountSocket = res.MountCderun
	}

	res.MountSocketPath = resolveConfigPath(
		cli.CderunMountSocketPathSet, cli.CderunMountSocketPath,
		cli.MountSocketPathSet, cli.MountSocketPath,
		"CDERUN_MOUNT_SOCKET_PATH",
		subcommand, tools, func(t ToolConfig) ConfigPath { return t.MountSocketPath },
		global, func(g CDERunConfig) ConfigPath { return g.Defaults.MountSocketPath },
		res.SocketPath, // Default to host-side socket path
		r,
		"path",
	)

	// 16. Resolve DryRun (CLI/Env only)
	res.DryRun = resolveBool(
		cli.CderunDryRunSet, cli.CderunDryRun,
		cli.DryRunSet, cli.DryRun,
		"CDERUN_DRY_RUN",
		"", nil, nil,
		nil, nil,
		false,
	)

	// 17. Resolve DryRunFormat (CLI/Env only)
	res.DryRunFormat = resolveString(
		cli.CderunDryRunFormatSet, cli.CderunDryRunFormat,
		cli.DryRunFormatSet, cli.DryRunFormat,
		"CDERUN_DRY_RUN_FORMAT",
		"", nil, nil,
		nil, nil,
		"yaml",
		r,
	)

	// 18. Resolve DiagnosisFormat (CLI/Env only)
	res.DiagnosisFormat = resolveString(
		cli.CderunDiagnosisFormatSet, cli.CderunDiagnosisFormat,
		cli.DiagnosisFormatSet, cli.DiagnosisFormat,
		"CDERUN_DIAGNOSIS_FORMAT",
		"", nil, nil,
		nil, nil,
		"yaml",
		r,
	)

	// 19. Resolve Logging
	res.LogLevel = resolveString(
		cli.CderunLogLevelSet, cli.CderunLogLevel,
		cli.LogLevelSet, cli.LogLevel,
		"CDERUN_LOG_LEVEL",
		"", nil, nil,
		global, func(g CDERunConfig) string { return g.Logging.Level },
		"warn",
		r,
	)

	res.LogFormat = resolveString(
		cli.CderunLogFormatSet, cli.CderunLogFormat,
		cli.LogFormatSet, cli.LogFormat,
		"CDERUN_LOG_FORMAT",
		"", nil, nil,
		global, func(g CDERunConfig) string { return g.Logging.Format },
		"text",
		r,
	)

	res.LogTimestamp = resolveBool(
		cli.CderunLogTimestampSet, cli.CderunLogTimestamp,
		cli.LogTimestampSet, cli.LogTimestamp,
		"CDERUN_LOG_TIMESTAMP",
		"", nil, nil,
		global, func(g CDERunConfig) *bool { return g.Logging.Timestamp },
		true, // Default to true
	)

	// Resolve Docker-compatible flags
	res.Ports = resolveStringSlice(cli.CderunPorts, cli.Ports, "CDERUN_PUBLISH", subcommand, tools, func(t ToolConfig) []string { return t.Ports }, global, func(g CDERunConfig) []string { return g.Defaults.Ports }, r)
	res.PublishAll = resolveBool(cli.CderunPublishAllSet, cli.CderunPublishAll, cli.PublishAllSet, cli.PublishAll, "CDERUN_PUBLISH_ALL", subcommand, tools, func(t ToolConfig) *bool { return t.PublishAll }, global, func(g CDERunConfig) *bool { return g.Defaults.PublishAll }, false)
	res.Expose = resolveStringSlice(cli.CderunExpose, cli.Expose, "CDERUN_EXPOSE", subcommand, tools, func(t ToolConfig) []string { return t.Expose }, global, func(g CDERunConfig) []string { return g.Defaults.Expose }, r)
	res.Hostname = resolveString(cli.CderunHostnameSet, cli.CderunHostname, cli.HostnameSet, cli.Hostname, "CDERUN_HOSTNAME", subcommand, tools, func(t ToolConfig) string { return t.Hostname }, global, func(g CDERunConfig) string { return g.Defaults.Hostname }, "", r)
	res.DNS = resolveStringSlice(cli.CderunDNS, cli.DNS, "CDERUN_DNS", subcommand, tools, func(t ToolConfig) []string { return t.DNS }, global, func(g CDERunConfig) []string { return g.Defaults.DNS }, r)
	res.AddHosts = resolveStringSlice(cli.CderunAddHosts, cli.AddHosts, "CDERUN_ADD_HOST", subcommand, tools, func(t ToolConfig) []string { return t.AddHosts }, global, func(g CDERunConfig) []string { return g.Defaults.AddHosts }, r)
	res.User = resolveString(cli.CderunUserSet, cli.CderunUser, cli.UserSet, cli.User, "CDERUN_USER", subcommand, tools, func(t ToolConfig) string { return t.User }, global, func(g CDERunConfig) string { return g.Defaults.User }, "", r)
	res.Privileged = resolveBool(cli.CderunPrivilegedSet, cli.CderunPrivileged, cli.PrivilegedSet, cli.Privileged, "CDERUN_PRIVILEGED", subcommand, tools, func(t ToolConfig) *bool { return t.Privileged }, global, func(g CDERunConfig) *bool { return g.Defaults.Privileged }, false)
	res.CapAdd = resolveStringSlice(cli.CderunCapAdd, cli.CapAdd, "CDERUN_CAP_ADD", subcommand, tools, func(t ToolConfig) []string { return t.CapAdd }, global, func(g CDERunConfig) []string { return g.Defaults.CapAdd }, r)
	res.CapDrop = resolveStringSlice(cli.CderunCapDrop, cli.CapDrop, "CDERUN_CAP_DROP", subcommand, tools, func(t ToolConfig) []string { return t.CapDrop }, global, func(g CDERunConfig) []string { return g.Defaults.CapDrop }, r)
	res.Entrypoint = resolveStringSlice(cli.CderunEntrypoint, cli.Entrypoint, "CDERUN_ENTRYPOINT", subcommand, tools, func(t ToolConfig) []string { return t.Entrypoint }, global, func(g CDERunConfig) []string { return g.Defaults.Entrypoint }, r)
	res.Command = resolveStringSlice(nil, nil, "CDERUN_COMMAND", subcommand, tools, func(t ToolConfig) []string { return t.Command }, global, func(g CDERunConfig) []string { return g.Defaults.Command }, r)
	res.Pull = resolveString(cli.CderunPullSet, cli.CderunPull, cli.PullSet, cli.Pull, "CDERUN_PULL", subcommand, tools, func(t ToolConfig) string { return t.Pull }, global, func(g CDERunConfig) string { return g.Defaults.Pull }, "missing", r)
	res.Devices, err = resolveDevices(cli.CderunDevices, cli.Devices, subcommand, tools, global, r)
	if err != nil {
		return nil, err
	}

	// Memory resolution (string to bytes)
	memStr := resolveString(cli.CderunMemorySet, cli.CderunMemory, cli.MemorySet, cli.Memory, "CDERUN_MEMORY", subcommand, tools, func(t ToolConfig) string { return t.Memory }, global, func(g CDERunConfig) string { return g.Defaults.Memory }, "", r)
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

	// CPUs resolution
	res.CPUs = resolveFloat64(cli.CderunCPUsSet, cli.CderunCPUs, cli.CPUsSet, cli.CPUs, "CDERUN_CPUS", subcommand, tools, func(t ToolConfig) float64 { return t.CPUs }, global, func(g CDERunConfig) float64 { return g.Defaults.CPUs }, 0)

	res.HostContext = hostCtx

	if err := r.Error(); err != nil {
		return nil, err
	}

	return res, nil
}

func resolveBool(p1Set bool, p1Val bool, p2Set bool, p2Val bool, envKey string, subcommand string, tools ToolsConfig, toolGetter func(ToolConfig) *bool, global *CDERunConfig, globalGetter func(CDERunConfig) *bool, fallback bool) bool {
	val, specified := resolveBoolInfo(p1Set, p1Val, p2Set, p2Val, envKey, subcommand, tools, toolGetter, global, globalGetter)
	if specified {
		return val
	}
	return fallback
}

func resolveBoolInfo(p1Set bool, p1Val bool, p2Set bool, p2Val bool, envKey string, subcommand string, tools ToolsConfig, toolGetter func(ToolConfig) *bool, global *CDERunConfig, globalGetter func(CDERunConfig) *bool) (bool, bool) {
	if p1Set {
		return p1Val, true
	}
	if p2Set {
		return p2Val, true
	}
	if envKey != "" {
		if env := os.Getenv(envKey); env != "" {
			if b, err := strconv.ParseBool(env); err == nil {
				return b, true
			}
		}
	}
	if tools != nil {
		if tool, ok := tools[subcommand]; ok {
			if b := toolGetter(tool); b != nil {
				return *b, true
			}
		}
	}
	if global != nil {
		if b := globalGetter(*global); b != nil {
			return *b, true
		}
	}
	return false, false
}

func resolveString(p1Set bool, p1Val string, cliSet bool, cliVal string, envKey string, subcommand string, tools ToolsConfig, toolGetter func(ToolConfig) string, global *CDERunConfig, globalGetter func(CDERunConfig) string, fallback string, r *ExpressionResolver) string {
	if p1Set {
		return r.resolveString(p1Val)
	}
	if cliSet {
		return r.resolveString(cliVal)
	}
	if env := os.Getenv(envKey); env != "" {
		return r.resolveString(env)
	}
	if tools != nil {
		if tool, ok := tools[subcommand]; ok {
			if s := toolGetter(tool); s != "" {
				return r.resolveString(s)
			}
		}
	}
	if global != nil {
		if s := globalGetter(*global); s != "" {
			return r.resolveString(s)
		}
	}
	return r.resolveString(fallback)
}

func resolveConfigPath(p1Set bool, p1Val string, cliSet bool, cliVal string, envKey string, subcommand string, tools ToolsConfig, toolGetter func(ToolConfig) ConfigPath, global *CDERunConfig, globalGetter func(CDERunConfig) ConfigPath, fallback string, r *ExpressionResolver, pathType string) string {
	var cp ConfigPath
	if p1Set {
		cp = ConfigPath{Raw: p1Val, BaseDir: r.Pwd}
	} else if cliSet {
		cp = ConfigPath{Raw: cliVal, BaseDir: r.Pwd}
	} else if env := os.Getenv(envKey); env != "" {
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

func resolveDevices(p1 []string, p2 []string, subcommand string, tools ToolsConfig, global *CDERunConfig, r *ExpressionResolver) ([]container.DeviceMapping, error) {
	var dcs []DeviceConfig

	if len(p1) > 0 {
		for _, d := range p1 {
			parsed, ok := ParseDeviceConfig(d)
			if !ok {
				return nil, fmt.Errorf("invalid device config (override): %s", d)
			}
			parsed.SetBaseDir(r.Pwd)
			dcs = append(dcs, parsed)
		}
	} else if len(p2) > 0 {
		for _, d := range p2 {
			parsed, ok := ParseDeviceConfig(d)
			if !ok {
				return nil, fmt.Errorf("invalid device config: %s", d)
			}
			parsed.SetBaseDir(r.Pwd)
			dcs = append(dcs, parsed)
		}
	} else if env := os.Getenv("CDERUN_DEVICE"); env != "" {
		for d := range strings.SplitSeq(env, ",") {
			parsed, ok := ParseDeviceConfig(d)
			if !ok {
				return nil, fmt.Errorf("invalid device config in CDERUN_DEVICE: %s", d)
			}
			parsed.SetBaseDir(r.Pwd)
			dcs = append(dcs, parsed)
		}
	} else if tools != nil {
		if tool, ok := tools[subcommand]; ok && len(tool.Devices) > 0 {
			dcs = tool.Devices
		}
	}

	if len(dcs) == 0 && global != nil {
		dcs = global.Defaults.Devices
	}

	var res []container.DeviceMapping
	for _, dc := range dcs {
		res = append(res, dc.Resolve(r))
	}
	return res, nil
}

func resolveStringSliceComma(p1Set bool, p1Val string, p2Set bool, p2Val string, envKey string, subcommand string, tools ToolsConfig, toolGetter func(ToolConfig) []string, global *CDERunConfig, globalGetter func(CDERunConfig) []string, r *ExpressionResolver) []string {
	var vals []string
	if p1Set {
		vals = strings.Split(p1Val, ",")
	} else if p2Set {
		vals = strings.Split(p2Val, ",")
	} else if env := os.Getenv(envKey); env != "" {
		vals = strings.Split(env, ",")
	} else if tools != nil {
		if tool, ok := tools[subcommand]; ok {
			vals = toolGetter(tool)
		}
	}
	if len(vals) == 0 && global != nil {
		vals = globalGetter(*global)
	}

	var res []string
	for _, v := range vals {
		v = strings.TrimSpace(v)
		if v != "" {
			res = append(res, r.resolveString(v))
		}
	}
	return res
}

func resolveStringSlice(p1 []string, p2 []string, envKey string, subcommand string, tools ToolsConfig, toolGetter func(ToolConfig) []string, global *CDERunConfig, globalGetter func(CDERunConfig) []string, r *ExpressionResolver) []string {
	var vals []string
	if len(p1) > 0 {
		vals = p1
	} else if len(p2) > 0 {
		vals = p2
	} else if env := os.Getenv(envKey); env != "" {
		vals = strings.Split(env, ",")
	} else if tools != nil {
		if tool, ok := tools[subcommand]; ok {
			vals = toolGetter(tool)
		}
	}
	if len(vals) == 0 && global != nil {
		vals = globalGetter(*global)
	}

	var res []string
	for _, v := range vals {
		res = append(res, r.resolveString(v))
	}
	return res
}

func resolveFloat64(p1Set bool, p1Val float64, cliSet bool, cliVal float64, envKey string, subcommand string, tools ToolsConfig, toolGetter func(ToolConfig) float64, global *CDERunConfig, globalGetter func(CDERunConfig) float64, fallback float64) float64 {
	if p1Set {
		return p1Val
	}
	if cliSet {
		return cliVal
	}
	if env := os.Getenv(envKey); env != "" {
		if f, err := strconv.ParseFloat(env, 64); err == nil {
			return f
		}
	}
	if tools != nil {
		if tool, ok := tools[subcommand]; ok {
			if f := toolGetter(tool); f != 0 {
				return f
			}
		}
	}
	if global != nil {
		if f := globalGetter(*global); f != 0 {
			return f
		}
	}
	return fallback
}

func resolveEnv(p1 []string, p2 []string, envKey string, subcommand string, tools ToolsConfig, global *CDERunConfig, strict bool, r *ExpressionResolver) ([]string, error) {
	var envs []string

	if len(p1) > 0 {
		envs = p1
	} else if len(p2) > 0 {
		envs = p2
	} else if env := os.Getenv(envKey); env != "" {
		for e := range strings.SplitSeq(env, ";") {
			e = strings.TrimSpace(e)
			if e != "" {
				envs = append(envs, e)
			}
		}
	} else if tools != nil {
		if tool, ok := tools[subcommand]; ok && len(tool.Env) > 0 {
			envs = tool.Env
		}
	}

	if len(envs) == 0 && global != nil {
		envs = global.Defaults.Env
	}

	// Deduplicate within the winning source (last-one-wins for the same key)
	// We use mergeEnv with nil/nil for other sources to leverage its deduplication logic.
	merged := mergeEnv(nil, nil, envs)

	return resolveEnvValues(merged, strict, r)
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

func resolveEnvValues(env []string, strict bool, r *ExpressionResolver) ([]string, error) {
	var res []string
	for _, e := range env {
		resolvedE := r.resolveString(e)
		if err := r.Error(); err != nil {
			return nil, err
		}
		if strings.Contains(resolvedE, "=") {
			res = append(res, resolvedE)
		} else {
			val, found := os.LookupEnv(resolvedE)
			if !found && strict {
				return nil, fmt.Errorf("required environment variable not found: %s", resolvedE)
			}
			res = append(res, fmt.Sprintf("%s=%s", resolvedE, val))
		}
	}
	return res, nil
}

func resolveMounts(p1 []string, p2 []string, subcommand string, tools ToolsConfig, global *CDERunConfig, r *ExpressionResolver) ([]container.Mount, error) {
	var mcs []MountConfig

	if len(p1) > 0 {
		for _, m := range p1 {
			parsed, err := ParseMountFlag(m)
			if err != nil {
				return nil, fmt.Errorf("invalid mount config (override): %w", err)
			}
			parsed.SetBaseDir(r.Pwd)
			mcs = append(mcs, parsed)
		}
	} else if len(p2) > 0 {
		for _, m := range p2 {
			parsed, err := ParseMountFlag(m)
			if err != nil {
				return nil, fmt.Errorf("invalid mount config: %w", err)
			}
			parsed.SetBaseDir(r.Pwd)
			mcs = append(mcs, parsed)
		}
	} else if env := os.Getenv("CDERUN_MOUNT"); env != "" {
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
		if tool, ok := tools[subcommand]; ok && len(tool.Mounts) > 0 {
			mcs = tool.Mounts
		}
	}

	if len(mcs) == 0 && global != nil {
		mcs = global.Defaults.Mounts
	}

	var res []container.Mount
	for _, mc := range mcs {
		res = append(res, mc.Resolve(r))
	}
	return res, nil
}
