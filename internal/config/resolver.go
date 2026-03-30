package config

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"

	"cderun/internal/container"
	"cderun/internal/logging"

	"github.com/docker/go-units"
)

type boolOptEntry struct {
	target        *bool
	def           OptionDef[*bool]
	fallback      bool
	cderunFlagSet bool
	cderunFlagVal bool
	cliFlagSet    bool
	cliFlagVal    bool
}

type stringSliceOptEntry struct {
	target     *[]string
	def        OptionDef[[]string]
	sep        string
	cderunFlag []string
	cliFlag    []string
}

type float64OptEntry struct {
	target        *float64
	def           OptionDef[*float64]
	cderunFlagSet bool
	cderunFlagVal float64
	cliFlagSet    bool
	cliFlagVal    float64
}

type intOptEntry struct {
	target        *int
	def           OptionDef[*int]
	cderunFlagSet bool
	cderunFlagVal int
	cliFlagSet    bool
	cliFlagVal    int
}

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
	PullMaxRetries  int
	PullBackoffBase time.Duration
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
	PullMaxRetries      int
	PullMaxRetriesSet   bool
	CderunPullMaxRetries int
	CderunPullMaxRetriesSet bool
	PullBackoffBase     string
	PullBackoffBaseSet  bool
	CderunPullBackoffBase string
	CderunPullBackoffBaseSet bool
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
func ptr[T any](v T) *T {
	return &v
}

var (
	cliType   = reflect.TypeFor[CLIOptions]()
	resType   = reflect.TypeFor[ResolvedConfig]()
	fieldInfo map[string]optionFields
	fieldOnce sync.Once
)

type optionFields struct {
	targetIdx []int
	p1SetIdx  []int
	p1ValIdx  []int
	p2SetIdx  []int
	p2ValIdx  []int
}

func initFieldInfo() {
	fieldInfo = make(map[string]optionFields)
	for _, opt := range StringOptions {
		if opt.SkipResolution {
			continue
		}
		fieldName := opt.FieldName
		if fieldName == "" {
			fieldName = PascalCase(opt.Name)
		}

		targetField, ok := resType.FieldByName(fieldName)
		if !ok {
			continue
		}

		p1SetField, ok1 := cliType.FieldByName("Cderun" + fieldName + "Set")
		p1ValField, ok2 := cliType.FieldByName("Cderun" + fieldName)
		p2SetField, ok3 := cliType.FieldByName(fieldName + "Set")
		p2ValField, ok4 := cliType.FieldByName(fieldName)

		if ok1 && ok2 && ok3 && ok4 {
			fieldInfo[opt.Name] = optionFields{
				targetIdx: targetField.Index,
				p1SetIdx:  p1SetField.Index,
				p1ValIdx:  p1ValField.Index,
				p2SetIdx:  p2SetField.Index,
				p2ValIdx:  p2ValField.Index,
			}
		}
	}
}

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

	// Phase 1: Early resolution (Diagnosis & StrictEnv)
	earlyBoolOpts := []boolOptEntry{
		{target: &res.Diagnosis, def: OptionDef[*bool]{EnvKey: "CDERUN_DIAGNOSIS", ToolGetter: func(t ToolConfig) *bool { return t.Diagnosis }, GlobalGetter: func(g CDERunConfig) *bool { return g.Defaults.Diagnosis }}, fallback: false, cderunFlagSet: cli.CderunDiagnosisSet, cderunFlagVal: cli.CderunDiagnosis, cliFlagSet: cli.DiagnosisSet, cliFlagVal: cli.Diagnosis},
		{target: &res.StrictEnv, def: OptionDef[*bool]{EnvKey: "CDERUN_STRICT_ENV", ToolGetter: func(t ToolConfig) *bool { return t.StrictEnv }, GlobalGetter: func(g CDERunConfig) *bool { return g.Defaults.StrictEnv }}, fallback: false, cderunFlagSet: cli.CderunStrictEnvSet, cderunFlagVal: cli.CderunStrictEnv, cliFlagSet: cli.StrictEnvSet, cliFlagVal: cli.StrictEnv},
	}
	for _, o := range earlyBoolOpts {
		*o.target = resolveBoolOpt(o.def, o.fallback, o.cderunFlagSet, o.cderunFlagVal, o.cliFlagSet, o.cliFlagVal, subcommand, tools, global, fs)
	}

	// Phase 2: String-based options from registry
	fieldOnce.Do(initFieldInfo)

	cliVal := reflect.ValueOf(cli)
	resVal := reflect.ValueOf(res).Elem()

	for _, opt := range StringOptions {
		if opt.SkipResolution {
			continue
		}

		info, ok := fieldInfo[opt.Name]
		if !ok {
			// Fallback to slow path if not in fieldInfo
			fieldName := opt.FieldName
			if fieldName == "" {
				fieldName = PascalCase(opt.Name)
			}
			targetField := resVal.FieldByName(fieldName)
			if !targetField.IsValid() {
				return nil, fmt.Errorf("registry mismatch: field %q for option %q not found in ResolvedConfig", fieldName, opt.Name)
			}
			p1SetField := cliVal.FieldByName("Cderun" + fieldName + "Set")
			p1ValField := cliVal.FieldByName("Cderun" + fieldName)
			p2SetField := cliVal.FieldByName(fieldName + "Set")
			p2ValField := cliVal.FieldByName(fieldName)

			if !p1SetField.IsValid() || !p1ValField.IsValid() || !p2SetField.IsValid() || !p2ValField.IsValid() {
				return nil, fmt.Errorf("registry mismatch: CLI reflection fields for option %q missing in CLIOptions", opt.Name)
			}

			p1Set := p1SetField.IsValid() && p1SetField.Bool()
			p2Set := p2SetField.IsValid() && p2SetField.Bool()

			p1ValStr := ""
			if p1ValField.IsValid() {
				p1ValStr = p1ValField.String()
			}
			p2ValStr := ""
			if p2ValField.IsValid() {
				p2ValStr = p2ValField.String()
			}

			def := OptionDef[string]{
				EnvKey:       opt.EnvKey,
				ToolGetter:   opt.ToolGetter,
				GlobalGetter: opt.GlobalGetter,
				Fallback:     opt.Default,
			}
			resolved := resolveStringOpt(def, p1Set, p1ValStr, p2Set, p2ValStr, subcommand, tools, global, r, fs)
			targetField.SetString(resolved)
			continue
		}

		p1Set := cliVal.FieldByIndex(info.p1SetIdx).Bool()
		p1Val := cliVal.FieldByIndex(info.p1ValIdx).String()
		p2Set := cliVal.FieldByIndex(info.p2SetIdx).Bool()
		p2Val := cliVal.FieldByIndex(info.p2ValIdx).String()

		def := OptionDef[string]{
			EnvKey:       opt.EnvKey,
			ToolGetter:   opt.ToolGetter,
			GlobalGetter: opt.GlobalGetter,
			Fallback:     opt.Default,
		}

		resolved := resolveStringOpt(def, p1Set, p1Val, p2Set, p2Val, subcommand, tools, global, r, fs)
		resVal.FieldByIndex(info.targetIdx).SetString(resolved)
	}

	if res.Image == "" && subcommand != "" && !res.Diagnosis {
		return nil, fmt.Errorf("no image mapping found for tool: %s", subcommand)
	}
	if res.Image != "" {
		logging.Debug("Resolved Image: %s", res.Image)
	}

	// Phase 3: Remaining Boolean options
	boolOpts := []boolOptEntry{
		{target: &res.TTY, def: OptionDef[*bool]{EnvKey: "CDERUN_TTY", ToolGetter: func(t ToolConfig) *bool { return t.TTY }, GlobalGetter: func(g CDERunConfig) *bool { return g.Defaults.TTY }}, fallback: false, cderunFlagSet: cli.CderunTTYSet, cderunFlagVal: cli.CderunTTY, cliFlagSet: cli.TTYSet, cliFlagVal: cli.TTY},
		{target: &res.Interactive, def: OptionDef[*bool]{EnvKey: "CDERUN_INTERACTIVE", ToolGetter: func(t ToolConfig) *bool { return t.Interactive }, GlobalGetter: func(g CDERunConfig) *bool { return g.Defaults.Interactive }}, fallback: false, cderunFlagSet: cli.CderunInteractiveSet, cderunFlagVal: cli.CderunInteractive, cliFlagSet: cli.InteractiveSet, cliFlagVal: cli.Interactive},
		{target: &res.Remove, def: OptionDef[*bool]{EnvKey: "CDERUN_REMOVE", ToolGetter: func(t ToolConfig) *bool { return t.Remove }, GlobalGetter: func(g CDERunConfig) *bool { return g.Defaults.Remove }}, fallback: true, cderunFlagSet: cli.CderunRemoveSet, cderunFlagVal: cli.CderunRemove, cliFlagSet: cli.RemoveSet, cliFlagVal: cli.Remove},
		{target: &res.MountAllTools, def: OptionDef[*bool]{EnvKey: "CDERUN_MOUNT_ALL_TOOLS", ToolGetter: func(t ToolConfig) *bool { return t.MountAllTools }, GlobalGetter: func(g CDERunConfig) *bool { return g.Defaults.MountAllTools }}, fallback: false, cderunFlagSet: cli.CderunMountAllToolsSet, cderunFlagVal: cli.CderunMountAllTools, cliFlagSet: cli.MountAllToolsSet, cliFlagVal: cli.MountAllTools},
		{target: &res.DryRun, def: OptionDef[*bool]{EnvKey: "CDERUN_DRY_RUN", ToolGetter: func(t ToolConfig) *bool { return t.DryRun }, GlobalGetter: func(g CDERunConfig) *bool { return g.Defaults.DryRun }}, fallback: false, cderunFlagSet: cli.CderunDryRunSet, cderunFlagVal: cli.CderunDryRun, cliFlagSet: cli.DryRunSet, cliFlagVal: cli.DryRun},
		{target: &res.LogTimestamp, def: OptionDef[*bool]{EnvKey: "CDERUN_LOG_TIMESTAMP", ToolGetter: func(t ToolConfig) *bool { return t.LogTimestamp }, GlobalGetter: func(g CDERunConfig) *bool { return g.Logging.Timestamp }}, fallback: true, cderunFlagSet: cli.CderunLogTimestampSet, cderunFlagVal: cli.CderunLogTimestamp, cliFlagSet: cli.LogTimestampSet, cliFlagVal: cli.LogTimestamp},
		{target: &res.PublishAll, def: OptionDef[*bool]{EnvKey: "CDERUN_PUBLISH_ALL", ToolGetter: func(t ToolConfig) *bool { return t.PublishAll }, GlobalGetter: func(g CDERunConfig) *bool { return g.Defaults.PublishAll }}, fallback: false, cderunFlagSet: cli.CderunPublishAllSet, cderunFlagVal: cli.CderunPublishAll, cliFlagSet: cli.PublishAllSet, cliFlagVal: cli.PublishAll},
		{target: &res.Privileged, def: OptionDef[*bool]{EnvKey: "CDERUN_PRIVILEGED", ToolGetter: func(t ToolConfig) *bool { return t.Privileged }, GlobalGetter: func(g CDERunConfig) *bool { return g.Defaults.Privileged }}, fallback: false, cderunFlagSet: cli.CderunPrivilegedSet, cderunFlagVal: cli.CderunPrivileged, cliFlagSet: cli.PrivilegedSet, cliFlagVal: cli.Privileged},
	}
	for _, o := range boolOpts {
		*o.target = resolveBoolOpt(o.def, o.fallback, o.cderunFlagSet, o.cderunFlagVal, o.cliFlagSet, o.cliFlagVal, subcommand, tools, global, fs)
	}

	// Phase 4: Complex types (Mounts, Env)
	res.Mounts, err = resolveMounts(cli.CderunMounts, cli.Mounts, subcommand, tools, global, r, fs)
	if err != nil {
		return nil, err
	}

	res.Env, err = resolveEnv(cli.CderunEnv, cli.Env, "CDERUN_ENV", subcommand, tools, global, res.StrictEnv, r, fs)
	if err != nil {
		return nil, err
	}

	// Phase 5: Path resolution & Auto-detection (Socket)
	res.SocketPath, err = resolveConfigPath(
		cli.CderunSocketPathSet, cli.CderunSocketPath,
		cli.SocketPathSet, cli.SocketPath,
		"CDERUN_SOCKET_PATH",
		"", nil, nil,
		global, func(g CDERunConfig) ConfigPath { return g.SocketPath },
		"",
		r,
		"path",
		fs,
	)
	if err != nil {
		return nil, err
	}

	if res.Runtime == "" {
		if res.SocketPath != "" {
			if strings.Contains(res.SocketPath, "podman") {
				res.Runtime = "podman"
			} else if strings.Contains(res.SocketPath, "containerd") {
				res.Runtime = "containerd"
			} else {
				res.Runtime = "docker"
			}
		} else {
			if _, err := fs.Stat("/var/run/docker.sock"); err == nil {
				res.Runtime = "docker"
				res.SocketPath = "/var/run/docker.sock"
			} else if _, err := fs.Stat("/run/podman/podman.sock"); err == nil {
				res.Runtime = "podman"
				res.SocketPath = "/run/podman/podman.sock"
			} else if _, err := fs.Stat("/run/containerd/containerd.sock"); err == nil {
				res.Runtime = "containerd"
				res.SocketPath = "/run/containerd/containerd.sock"
			} else {
				res.Runtime = "docker"
				res.SocketPath = "/var/run/docker.sock"
			}
		}
	}

	if res.SocketPath == "" {
		switch res.Runtime {
		case "podman":
			res.SocketPath = "/run/podman/podman.sock"
		case "containerd":
			res.SocketPath = "/run/containerd/containerd.sock"
		default:
			res.SocketPath = "/var/run/docker.sock"
		}
	}
	res.SocketPath = strings.TrimPrefix(res.SocketPath, "unix://")

	// Phase 6: Transitive options (MountTools -> MountCderun -> MountSocket)
	res.MountTools = resolveStringSliceCommaOpt(
		OptionDef[[]string]{EnvKey: "CDERUN_MOUNT_TOOLS",
			ToolGetter:   func(t ToolConfig) []string { return t.MountTools },
			GlobalGetter: func(g CDERunConfig) []string { return g.Defaults.MountTools }},
		cli.CderunMountToolsSet, cli.CderunMountTools,
		cli.MountToolsSet, cli.MountTools,
		subcommand, tools, global, r, fs,
	)

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
		res.MountSocket = res.MountCderun
	}

	res.MountSocketPath, err = resolveConfigPath(
		cli.CderunMountSocketPathSet, cli.CderunMountSocketPath,
		cli.MountSocketPathSet, cli.MountSocketPath,
		"CDERUN_MOUNT_SOCKET_PATH",
		subcommand, tools, func(t ToolConfig) ConfigPath { return t.MountSocketPath },
		global, func(g CDERunConfig) ConfigPath { return g.Defaults.MountSocketPath },
		res.SocketPath,
		r,
		"path",
		fs,
	)
	if err != nil {
		return nil, err
	}

	// Phase 7: Duration and Slice options
	// Resolve hang-timeout via registry entry (skipped in Phase 2)
	var hangTimeoutStr string
	if opt, ok := GetStringOption("hang-timeout"); ok {
		def := OptionDef[string]{
			EnvKey:       opt.EnvKey,
			ToolGetter:   opt.ToolGetter,
			GlobalGetter: opt.GlobalGetter,
			Fallback:     opt.Default,
		}
		hangTimeoutStr = resolveStringOpt(def, cli.CderunHangTimeoutSet, cli.CderunHangTimeout, cli.HangTimeoutSet, cli.HangTimeout, subcommand, tools, global, r, fs)
	}
	if hangTimeoutStr != "" {
		if d, err := time.ParseDuration(hangTimeoutStr); err == nil {
			if d < 0 {
				return nil, fmt.Errorf("invalid hang-timeout value %q: duration cannot be negative", hangTimeoutStr)
			}
			res.HangTimeout = d
		} else {
			return nil, fmt.Errorf("invalid hang-timeout value %q: %w", hangTimeoutStr, err)
		}
	}

	sliceOpts := []stringSliceOptEntry{
		{target: &res.Ports, def: OptionDef[[]string]{EnvKey: "CDERUN_PUBLISH", ToolGetter: func(t ToolConfig) []string { return t.Ports }, GlobalGetter: func(g CDERunConfig) []string { return g.Defaults.Ports }}, sep: ",", cderunFlag: cli.CderunPorts, cliFlag: cli.Ports},
		{target: &res.Expose, def: OptionDef[[]string]{EnvKey: "CDERUN_EXPOSE", ToolGetter: func(t ToolConfig) []string { return t.Expose }, GlobalGetter: func(g CDERunConfig) []string { return g.Defaults.Expose }}, sep: ",", cderunFlag: cli.CderunExpose, cliFlag: cli.Expose},
		{target: &res.DNS, def: OptionDef[[]string]{EnvKey: "CDERUN_DNS", ToolGetter: func(t ToolConfig) []string { return t.DNS }, GlobalGetter: func(g CDERunConfig) []string { return g.Defaults.DNS }}, sep: ",", cderunFlag: cli.CderunDNS, cliFlag: cli.DNS},
		{target: &res.AddHosts, def: OptionDef[[]string]{EnvKey: "CDERUN_ADD_HOST", ToolGetter: func(t ToolConfig) []string { return t.AddHosts }, GlobalGetter: func(g CDERunConfig) []string { return g.Defaults.AddHosts }}, sep: ",", cderunFlag: cli.CderunAddHosts, cliFlag: cli.AddHosts},
		{target: &res.CapAdd, def: OptionDef[[]string]{EnvKey: "CDERUN_CAP_ADD", ToolGetter: func(t ToolConfig) []string { return t.CapAdd }, GlobalGetter: func(g CDERunConfig) []string { return g.Defaults.CapAdd }}, sep: ",", cderunFlag: cli.CderunCapAdd, cliFlag: cli.CapAdd},
		{target: &res.CapDrop, def: OptionDef[[]string]{EnvKey: "CDERUN_CAP_DROP", ToolGetter: func(t ToolConfig) []string { return t.CapDrop }, GlobalGetter: func(g CDERunConfig) []string { return g.Defaults.CapDrop }}, sep: ",", cderunFlag: cli.CderunCapDrop, cliFlag: cli.CapDrop},
		{target: &res.Entrypoint, def: OptionDef[[]string]{EnvKey: "CDERUN_ENTRYPOINT", ToolGetter: func(t ToolConfig) []string { return t.Entrypoint }, GlobalGetter: func(g CDERunConfig) []string { return g.Defaults.Entrypoint }}, sep: ",", cderunFlag: cli.CderunEntrypoint, cliFlag: cli.Entrypoint},
	}
	for _, o := range sliceOpts {
		*o.target = resolveStringSliceOpt(o.def, o.sep, o.cderunFlag, o.cliFlag, subcommand, tools, global, r, fs)
	}

	// Phase 8: Integer & Float options
	intOpts := []intOptEntry{
		{target: &res.PullMaxRetries, def: OptionDef[*int]{EnvKey: "CDERUN_PULL_MAX_RETRIES", ToolGetter: func(t ToolConfig) *int { return t.PullMaxRetries }, GlobalGetter: func(g CDERunConfig) *int { return g.Defaults.PullMaxRetries }, Fallback: ptr(3)}, cderunFlagSet: cli.CderunPullMaxRetriesSet, cderunFlagVal: cli.CderunPullMaxRetries, cliFlagSet: cli.PullMaxRetriesSet, cliFlagVal: cli.PullMaxRetries},
	}
	for _, o := range intOpts {
		*o.target = resolveIntOpt(o.def, o.cderunFlagSet, o.cderunFlagVal, o.cliFlagSet, o.cliFlagVal, subcommand, tools, global, fs)
	}
	if res.PullMaxRetries <= 0 {
		return nil, fmt.Errorf("invalid PullMaxRetries (%d): must be greater than 0", res.PullMaxRetries)
	}

	// Resolve pull-backoff-base via registry
	var pullBackoffBaseStr string
	if opt, ok := GetStringOption("pull-backoff-base"); ok {
		def := OptionDef[string]{
			EnvKey:       opt.EnvKey,
			ToolGetter:   opt.ToolGetter,
			GlobalGetter: opt.GlobalGetter,
			Fallback:     opt.Default,
		}
		pullBackoffBaseStr = resolveStringOpt(def, cli.CderunPullBackoffBaseSet, cli.CderunPullBackoffBase, cli.PullBackoffBaseSet, cli.PullBackoffBase, subcommand, tools, global, r, fs)
	}

	if pullBackoffBaseStr != "" {
		if d, err := time.ParseDuration(pullBackoffBaseStr); err == nil {
			if d <= 0 {
				return nil, fmt.Errorf("invalid PullBackoffBase duration %q: must be positive", pullBackoffBaseStr)
			}
			res.PullBackoffBase = d
		} else {
			return nil, fmt.Errorf("failed to parse PullBackoffBase from %q: %w", pullBackoffBaseStr, err)
		}
	}

	res.Devices, err = resolveDevices(cli.CderunDevices, cli.Devices, subcommand, tools, global, r, fs)
	if err != nil {
		return nil, err
	}

	// Resolve memory via registry
	var memStr string
	if opt, ok := GetStringOption("memory"); ok {
		def := OptionDef[string]{
			EnvKey:       opt.EnvKey,
			ToolGetter:   opt.ToolGetter,
			GlobalGetter: opt.GlobalGetter,
			Fallback:     opt.Default,
		}
		memStr = resolveStringOpt(def, cli.CderunMemorySet, cli.CderunMemory, cli.MemorySet, cli.Memory, subcommand, tools, global, r, fs)
	}
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

	floatOpts := []float64OptEntry{
		{target: &res.CPUs, def: OptionDef[*float64]{EnvKey: "CDERUN_CPUS", ToolGetter: func(t ToolConfig) *float64 { return t.CPUs }, GlobalGetter: func(g CDERunConfig) *float64 { return g.Defaults.CPUs }}, cderunFlagSet: cli.CderunCPUsSet, cderunFlagVal: cli.CderunCPUs, cliFlagSet: cli.CPUsSet, cliFlagVal: cli.CPUs},
	}
	for _, o := range floatOpts {
		*o.target = resolveFloat64Opt(o.def, o.cderunFlagSet, o.cderunFlagVal, o.cliFlagSet, o.cliFlagVal, subcommand, tools, global, fs)
	}

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
			key, _, _ := strings.Cut(e, "=")
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
		if mc.Optional && (mc.Type == "bind" || mc.Type == "") && !mc.Source.IsEmpty() {
			hostPath, err := mc.Source.Resolve(r)
			if err != nil {
				return nil, err
			}
			if _, err := fs.Stat(hostPath); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					// Skip if source doesn't exist
					continue
				}
				return nil, err
			}
		}

		resolved, err := mc.Resolve(r)
		if err != nil {
			return nil, err
		}
		res = append(res, resolved)
	}
	return res, nil
}
