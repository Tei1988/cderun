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
	Image           string
	TTY             bool
	Interactive     bool
	Network         string
	Remove          bool
	Volumes         []container.VolumeMount
	Env             []string
	Workdir         string
	User            string
	Runtime         string
	SocketPath      string
	MountSocket     bool
	MountSocketPath string
	MountCderun     bool
	MountTools      string
	MountAllTools   bool
	DryRun          bool
	DryRunFormat    string
	LogLevel        string
	LogFile         string
	LogFormat       string
	LogTee          bool
	LogTimestamp    bool
	StrictEnv       bool

	// New fields
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
	Tmpfs      []string
	Devices    []string
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
	Volumes                  []string
	CderunVolumes            []string
	MountCderun              bool
	MountCderunSet           bool
	CderunMountCderun        bool
	CderunMountCderunSet     bool
	MountTools               string
	CderunMountTools         string
	MountAllTools            bool
	CderunMountAllTools      bool
	DryRun                   bool
	DryRunSet                bool
	CderunDryRun             bool
	CderunDryRunSet          bool
	DryRunFormat             string
	DryRunFormatSet          bool
	CderunDryRunFormat       string
	CderunDryRunFormatSet    bool
	LogLevel                 string
	LogLevelSet              bool
	LogFile                  string
	LogFileSet               bool
	LogFormat                string
	LogFormatSet             bool
	LogTee                   bool
	LogTeeSet                bool
	LogTimestampSet          bool
	LogTimestamp             bool
	Verbose                  int
	CderunLogLevel           string
	CderunLogLevelSet        bool
	CderunLogFile            string
	CderunLogFileSet         bool
	CderunLogFormat          string
	CderunLogFormatSet       bool
	CderunLogTee             bool
	CderunLogTeeSet          bool
	CderunVerbose            int

	// New fields
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
	Tmpfs               []string
	CderunTmpfs         []string
	Devices             []string
	CderunDevices       []string
}

// Resolve combines CLI flags, environment variables, tool-specific config, and global defaults.
func Resolve(subcommand string, cli CLIOptions, tools ToolsConfig, global *CDERunConfig) (*ResolvedConfig, error) {
	logging.Trace("Resolving configurations for tool: %s", subcommand)
	res := &ResolvedConfig{}

	r, err := NewExpressionResolver()
	if err != nil {
		return nil, fmt.Errorf("failed to create expression resolver: %w", err)
	}

	// 1. Resolve Image (Step 10.1: Strict resolution)
	// Priority: P1 CLI > P2 CLI > Tool Config.
	// Bypasses Env (P3) and Global Defaults (P5).
	res.Image = resolveString(
		cli.CderunImageSet, cli.CderunImage,
		cli.ImageSet, cli.Image,
		"", // No Environment Variable for Image
		subcommand, tools, func(t ToolConfig) string { return t.Image },
		nil, nil, // No Global Default for Image
		"",
		r,
	)

	if res.Image == "" && subcommand != "" {
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
		global, func(g CDERunConfig) string { return "" },
		"",
		r,
	)

	// 8. Resolve Volumes (P1 > P2 > P4)
	res.Volumes = resolveVolumes(cli.CderunVolumes, cli.Volumes, subcommand, tools, r)

	// 10. Resolve StrictEnv
	res.StrictEnv = resolveBool(
		false, false, // No P1 for strictEnv yet
		false, false, // No P2 for strictEnv yet
		"CDERUN_STRICT_ENV",
		subcommand, tools, func(t ToolConfig) *bool { return t.StrictEnv },
		global, func(g CDERunConfig) *bool { return g.Defaults.StrictEnv },
		false,
	)

	// 11. Resolve Env (P1 > P2 > P4)
	var toolsEnv []string
	if tools != nil {
		if tool, ok := tools[subcommand]; ok {
			toolsEnv = tool.Env
		}
	}
	res.Env, err = resolveEnvValues(mergeEnv(toolsEnv, cli.Env, cli.CderunEnv), res.StrictEnv, r)
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
			if _, err := os.Stat("/var/run/docker.sock"); err == nil {
				res.Runtime = "docker"
				res.SocketPath = "/var/run/docker.sock"
			} else if _, err := os.Stat("/run/podman/podman.sock"); err == nil {
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

	// Special handling for unix:// prefix for the host-side socket connection
	res.SocketPath = strings.TrimPrefix(res.SocketPath, "unix://")

	// 12. Resolve MountSocket and MountSocketPath
	res.MountSocket = resolveBool(
		cli.CderunMountSocketSet, cli.CderunMountSocket,
		cli.MountSocketSet, cli.MountSocket,
		"CDERUN_MOUNT_SOCKET",
		subcommand, tools, func(t ToolConfig) *bool { return t.MountSocket },
		global, func(g CDERunConfig) *bool { return g.Defaults.MountSocket },
		false,
	)

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

	// 13. Resolve MountCderun
	res.MountCderun = resolveBool(
		cli.CderunMountCderunSet, cli.CderunMountCderun,
		cli.MountCderunSet, cli.MountCderun,
		"CDERUN_MOUNT_CDERUN",
		subcommand, tools, func(t ToolConfig) *bool { return t.MountCderun },
		global, func(g CDERunConfig) *bool { return g.Defaults.MountCderun },
		false,
	)

	// 14. Pass-through other mounting flags
	if cli.CderunMountTools != "" {
		res.MountTools = r.resolveString(cli.CderunMountTools)
	} else {
		res.MountTools = r.resolveString(cli.MountTools)
	}

	if cli.CderunMountAllTools {
		res.MountAllTools = true
	} else {
		res.MountAllTools = cli.MountAllTools
	}

	// 15. Resolve DryRun
	res.DryRun = resolveBool(
		cli.CderunDryRunSet, cli.CderunDryRun,
		cli.DryRunSet, cli.DryRun,
		"CDERUN_DRY_RUN",
		subcommand, tools, func(t ToolConfig) *bool { return t.DryRun },
		global, func(g CDERunConfig) *bool { return g.Defaults.DryRun },
		false,
	)

	// 16. Resolve DryRunFormat
	res.DryRunFormat = resolveString(
		cli.CderunDryRunFormatSet, cli.CderunDryRunFormat,
		cli.DryRunFormatSet, cli.DryRunFormat,
		"CDERUN_DRY_RUN_FORMAT",
		subcommand, tools, func(t ToolConfig) string { return t.DryRunFormat },
		global, func(g CDERunConfig) string { return g.Defaults.DryRunFormat },
		"yaml",
		r,
	)

	// 17. Resolve Logging
	res.LogLevel = resolveString(
		cli.CderunLogLevelSet, cli.CderunLogLevel,
		cli.LogLevelSet, cli.LogLevel,
		"CDERUN_LOG_LEVEL",
		"", nil, nil,
		global, func(g CDERunConfig) string { return g.Logging.Level },
		"info",
		r,
	)
	// Handle verbose flag overrides
	vLevel := cli.Verbose
	if cli.CderunVerbose > vLevel {
		vLevel = cli.CderunVerbose
	}

	if !cli.CderunLogLevelSet {
		if vLevel >= 3 {
			res.LogLevel = "trace"
		} else if vLevel >= 2 {
			res.LogLevel = "debug"
		}
	}

	res.LogFile = resolveConfigPath(
		cli.CderunLogFileSet, cli.CderunLogFile,
		cli.LogFileSet, cli.LogFile,
		"CDERUN_LOG_FILE",
		"", nil, nil,
		global, func(g CDERunConfig) ConfigPath { return g.Logging.File },
		"",
		r,
		"path",
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

	res.LogTee = resolveBool(
		cli.CderunLogTeeSet, cli.CderunLogTee,
		cli.LogTeeSet, cli.LogTee,
		"CDERUN_LOG_TEE",
		"", nil, nil,
		global, func(g CDERunConfig) *bool { return g.Logging.Tee },
		false,
	)

	res.LogTimestamp = resolveBool(
		false, false, // No P1 for timestamp yet
		cli.LogTimestampSet, cli.LogTimestamp,
		"CDERUN_LOG_TIMESTAMP",
		"", nil, nil,
		global, func(g CDERunConfig) *bool { return g.Logging.Timestamp },
		true, // Default to true
	)

	// Resolve new fields
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
	res.Tmpfs = resolveStringSlice(cli.CderunTmpfs, cli.Tmpfs, "CDERUN_TMPFS", subcommand, tools, func(t ToolConfig) []string { return t.Tmpfs }, global, func(g CDERunConfig) []string { return g.Defaults.Tmpfs }, r)
	res.Devices = resolveConfigPathSlice(cli.CderunDevices, cli.Devices, "CDERUN_DEVICE", subcommand, tools, func(t ToolConfig) []ConfigPath { return t.Devices }, global, func(g CDERunConfig) []ConfigPath { return g.Defaults.Devices }, r, "device")

	// Memory resolution (string to bytes)
	memStr := resolveString(cli.CderunMemorySet, cli.CderunMemory, cli.MemorySet, cli.Memory, "CDERUN_MEMORY", subcommand, tools, func(t ToolConfig) string { return t.Memory }, global, func(g CDERunConfig) string { return g.Defaults.Memory }, "", r)
	if memStr != "" {
		bytes, err := units.RAMInBytes(memStr)
		if err != nil {
			return nil, fmt.Errorf("invalid memory value %q: %w", memStr, err)
		}
		res.Memory = bytes
	}

	// CPUs resolution
	res.CPUs = resolveFloat64(cli.CderunCPUsSet, cli.CderunCPUs, cli.CPUsSet, cli.CPUs, "CDERUN_CPUS", subcommand, tools, func(t ToolConfig) float64 { return t.CPUs }, global, func(g CDERunConfig) float64 { return g.Defaults.CPUs }, 0)

	return res, nil
}

func resolveBool(p1Set bool, p1Val bool, p2Set bool, p2Val bool, envKey string, subcommand string, tools ToolsConfig, toolGetter func(ToolConfig) *bool, global *CDERunConfig, globalGetter func(CDERunConfig) *bool, fallback bool) bool {
	if p1Set {
		return p1Val
	}
	if p2Set {
		return p2Val
	}
	if env := os.Getenv(envKey); env != "" {
		if b, err := strconv.ParseBool(env); err == nil {
			return b
		}
	}
	if tools != nil {
		if tool, ok := tools[subcommand]; ok {
			if b := toolGetter(tool); b != nil {
				return *b
			}
		}
	}
	if global != nil {
		if b := globalGetter(*global); b != nil {
			return *b
		}
	}
	return fallback
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
		cp = ConfigPath{Raw: p1Val, BaseDir: "."}
	} else if cliSet {
		cp = ConfigPath{Raw: cliVal, BaseDir: "."}
	} else if env := os.Getenv(envKey); env != "" {
		cp = ConfigPath{Raw: env, BaseDir: "."}
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
			cp = ConfigPath{Raw: fallback, BaseDir: "."}
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

func resolveConfigPathSlice(p1 []string, p2 []string, envKey string, subcommand string, tools ToolsConfig, toolGetter func(ToolConfig) []ConfigPath, global *CDERunConfig, globalGetter func(CDERunConfig) []ConfigPath, r *ExpressionResolver, pathType string) []string {
	var cps []ConfigPath
	if len(p1) > 0 {
		for _, v := range p1 {
			cps = append(cps, ConfigPath{Raw: v, BaseDir: "."})
		}
	} else if len(p2) > 0 {
		for _, v := range p2 {
			cps = append(cps, ConfigPath{Raw: v, BaseDir: "."})
		}
	} else if env := os.Getenv(envKey); env != "" {
		for _, v := range strings.Split(env, ",") {
			cps = append(cps, ConfigPath{Raw: v, BaseDir: "."})
		}
	} else if tools != nil {
		if tool, ok := tools[subcommand]; ok {
			cps = toolGetter(tool)
		}
	}
	if len(cps) == 0 && global != nil {
		cps = globalGetter(*global)
	}

	var res []string
	for _, cp := range cps {
		switch pathType {
		case "volume":
			res = append(res, cp.ResolveVolume(r))
		case "device":
			res = append(res, cp.ResolveDevice(r))
		default:
			res = append(res, cp.Resolve(r))
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

func resolveVolumes(p1 []string, p2 []string, subcommand string, tools ToolsConfig, r *ExpressionResolver) []container.VolumeMount {
	var cps []ConfigPath

	// Tool-specific volumes (lowest priority)
	if tools != nil {
		if tool, ok := tools[subcommand]; ok {
			cps = append(cps, tool.Volumes...)
		}
	}

	// P2 CLI volumes (middle priority)
	for _, v := range p2 {
		cps = append(cps, ConfigPath{Raw: v, BaseDir: "."})
	}

	// P1 CLI volumes (highest priority)
	for _, v := range p1 {
		cps = append(cps, ConfigPath{Raw: v, BaseDir: "."})
	}

	var resolvedVols []string
	for _, cp := range cps {
		resolvedVols = append(resolvedVols, cp.ResolveVolume(r))
	}

	return parseVolumes(resolvedVols)
}

func parseVolumes(vols []string) []container.VolumeMount {
	var mounts []container.VolumeMount
	for _, v := range vols {
		if v == "" {
			continue
		}

		var hostPath, containerPath string
		var readOnly bool

		// Locate the last colon to check for options or container path
		lastColon := strings.LastIndex(v, ":")
		if lastColon == -1 {
			// Malformed entry: needs at least host:container
			continue
		}

		partAfterLastColon := v[lastColon+1:]
		if partAfterLastColon == "ro" || partAfterLastColon == "rw" {
			readOnly = (partAfterLastColon == "ro")
			// The part before the last colon must contain both host and container paths
			remaining := v[:lastColon]
			nextLastColon := strings.LastIndex(remaining, ":")
			if nextLastColon == -1 {
				// Malformed: only one colon found, but it was followed by a mode
				continue
			}
			hostPath = remaining[:nextLastColon]
			containerPath = remaining[nextLastColon+1:]
		} else {
			// No ro/rw mode at the end, so the part after the last colon is the container path
			hostPath = v[:lastColon]
			containerPath = v[lastColon+1:]
		}

		if hostPath != "" && containerPath != "" {
			mounts = append(mounts, container.VolumeMount{
				HostPath:      hostPath,
				ContainerPath: containerPath,
				ReadOnly:      readOnly,
			})
		}
	}
	return mounts
}
