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
func Resolve(subcommand string, cli *CLIOptions, tools ToolsConfig, global *CDERunConfig) (*ResolvedConfig, error) {
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

	process := func(name, fieldName string, skip bool) {
		if skip {
			return
		}
		if fieldName == "" {
			fieldName = PascalCase(name)
		}

		targetField, ok := resType.FieldByName(fieldName)
		if !ok {
			return
		}

		info := optionFields{targetIdx: targetField.Index}

		if f, ok := cliType.FieldByName("Cderun" + fieldName + "Set"); ok {
			info.p1SetIdx = f.Index
		}
		if f, ok := cliType.FieldByName("Cderun" + fieldName); ok {
			info.p1ValIdx = f.Index
		}
		if f, ok := cliType.FieldByName(fieldName + "Set"); ok {
			info.p2SetIdx = f.Index
		}
		if f, ok := cliType.FieldByName(fieldName); ok {
			info.p2ValIdx = f.Index
		}

		if info.p1ValIdx != nil && info.p2ValIdx != nil {
			fieldInfo[name] = info
		}
	}

	for _, opt := range StringOptions {
		process(opt.Name, opt.FieldName, opt.SkipResolution)
	}
	for _, opt := range BoolOptions {
		process(opt.Name, opt.FieldName, false)
	}
	for _, opt := range IntOptions {
		process(opt.Name, opt.FieldName, false)
	}
	for _, opt := range Float64Options {
		process(opt.Name, opt.FieldName, false)
	}
	for _, opt := range StringSliceOptions {
		process(opt.Name, opt.FieldName, opt.SkipResolution)
	}
}

func getFieldInfo(val reflect.Value, setIdx, valIdx []int) (bool, reflect.Value) {
	if len(setIdx) > 0 {
		return val.FieldByIndex(setIdx).Bool(), val.FieldByIndex(valIdx)
	}
	// For slices and other types without an explicit Set flag
	v := val.FieldByIndex(valIdx)
	// We check for nil (slices) or presence (other types) to determine if a value was set.
	// For types that are not slices and don't have a Set flag, we check if they are non-zero.
	k := v.Kind()
	if k == reflect.Slice || k == reflect.Map || k == reflect.Ptr || k == reflect.Interface || k == reflect.Chan || k == reflect.Func {
		return !v.IsNil(), v
	}
	return !v.IsZero(), v
}

func ResolveWithFS(subcommand string, cli *CLIOptions, tools ToolsConfig, global *CDERunConfig, fs FileSystem) (*ResolvedConfig, error) {
	logging.Trace("Resolving configurations for tool: %s", subcommand)
	if cli == nil {
		return nil, fmt.Errorf("cli options cannot be nil")
	}
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

	// Phase 1: Initialize reflection cache
	fieldOnce.Do(initFieldInfo)

	cliVal := reflect.ValueOf(cli).Elem()
	resVal := reflect.ValueOf(res).Elem()

	// Phase 1: Early resolution (Diagnosis & StrictEnv)
	for _, name := range []string{"diagnosis", "strict-env"} {
		opt, okOpt := GetBoolOption(name)
		if !okOpt {
			continue
		}

		info, okInfo := fieldInfo[name]
		if !okInfo {
			continue
		}

		def := OptionDef[*bool]{
			EnvKey:       opt.EnvKey,
			ToolGetter:   opt.ToolGetter,
			GlobalGetter: opt.GlobalGetter,
		}

		p1Set, p1Val := getFieldInfo(cliVal, info.p1SetIdx, info.p1ValIdx)
		p2Set, p2Val := getFieldInfo(cliVal, info.p2SetIdx, info.p2ValIdx)

		resolved := resolveBoolOpt(def, opt.Default, p1Set, p1Val.Bool(), p2Set, p2Val.Bool(), subcommand, tools, global, fs)
		resVal.FieldByIndex(info.targetIdx).SetBool(resolved)
	}

	for _, opt := range StringOptions {
		if opt.SkipResolution {
			continue
		}

		info, ok := fieldInfo[opt.Name]
		if !ok {
			return nil, fmt.Errorf("registry mismatch: CLI reflection fields for option %q missing in CLIOptions", opt.Name)
		}

		p1Set, p1Val := getFieldInfo(cliVal, info.p1SetIdx, info.p1ValIdx)
		p2Set, p2Val := getFieldInfo(cliVal, info.p2SetIdx, info.p2ValIdx)

		def := OptionDef[string]{
			EnvKey:       opt.EnvKey,
			ToolGetter:   opt.ToolGetter,
			GlobalGetter: opt.GlobalGetter,
			Fallback:     opt.Default,
		}

		resolved := resolveStringOpt(def, p1Set, p1Val.String(), p2Set, p2Val.String(), subcommand, tools, global, r, fs)
		resVal.FieldByIndex(info.targetIdx).SetString(resolved)
	}

	if res.Image == "" && subcommand != "" && !res.Diagnosis {
		return nil, fmt.Errorf("no image mapping found for tool: %s", subcommand)
	}
	if res.Image != "" {
		logging.Debug("Resolved Image: %s", res.Image)
	}

	// Phase 3: Remaining Boolean options
	for _, opt := range BoolOptions {
		// Skip early options already resolved in Phase 1
		if opt.Name == "diagnosis" || opt.Name == "strict-env" {
			continue
		}
		// Skip transitive options handled in Phase 6
		if opt.Name == "mount-socket" || opt.Name == "mount-cderun" || opt.Name == "mount-all-tools" {
			continue
		}

		info, ok := fieldInfo[opt.Name]
		if !ok {
			return nil, fmt.Errorf("registry mismatch: info for bool option %q not found", opt.Name)
		}

		p1Set, p1Val := getFieldInfo(cliVal, info.p1SetIdx, info.p1ValIdx)
		p2Set, p2Val := getFieldInfo(cliVal, info.p2SetIdx, info.p2ValIdx)

		def := OptionDef[*bool]{
			EnvKey:       opt.EnvKey,
			ToolGetter:   opt.ToolGetter,
			GlobalGetter: opt.GlobalGetter,
		}

		resolved := resolveBoolOpt(def, opt.Default, p1Set, p1Val.Bool(), p2Set, p2Val.Bool(), subcommand, tools, global, fs)
		resVal.FieldByIndex(info.targetIdx).SetBool(resolved)
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
			} else {
				res.Runtime = "docker"
				res.SocketPath = "/var/run/docker.sock"
			}
		}
	}

	if res.SocketPath == "" {
		if res.Runtime == "podman" {
			res.SocketPath = "/run/podman/podman.sock"
		} else {
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

	// Resolve mount-all-tools (transitive trigger)
	{
		opt, ok := GetBoolOption("mount-all-tools")
		info, ok2 := fieldInfo["mount-all-tools"]
		if !ok || !ok2 {
			return nil, fmt.Errorf("registry mismatch: 'mount-all-tools' not found")
		}
		p1Set := cliVal.FieldByIndex(info.p1SetIdx).Bool()
		p1Val := cliVal.FieldByIndex(info.p1ValIdx).Bool()
		p2Set := cliVal.FieldByIndex(info.p2SetIdx).Bool()
		p2Val := cliVal.FieldByIndex(info.p2ValIdx).Bool()
		def := OptionDef[*bool]{EnvKey: opt.EnvKey, ToolGetter: opt.ToolGetter, GlobalGetter: opt.GlobalGetter}
		res.MountAllTools = resolveBoolOpt(def, opt.Default, p1Set, p1Val, p2Set, p2Val, subcommand, tools, global, fs)
	}

	var mountCderunSpecified bool
	{
		opt, ok := GetBoolOption("mount-cderun")
		info, ok2 := fieldInfo["mount-cderun"]
		if !ok || !ok2 {
			return nil, fmt.Errorf("registry mismatch: 'mount-cderun' not found")
		}
		def := OptionDef[*bool]{EnvKey: opt.EnvKey, ToolGetter: opt.ToolGetter, GlobalGetter: opt.GlobalGetter}
		p1Set := cliVal.FieldByIndex(info.p1SetIdx).Bool()
		p1Val := cliVal.FieldByIndex(info.p1ValIdx).Bool()
		p2Set := cliVal.FieldByIndex(info.p2SetIdx).Bool()
		p2Val := cliVal.FieldByIndex(info.p2ValIdx).Bool()
		res.MountCderun, mountCderunSpecified = resolveBoolOptInfo(def, p1Set, p1Val, p2Set, p2Val, subcommand, tools, global, fs)
	}
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
	{
		opt, ok := GetBoolOption("mount-socket")
		info, ok2 := fieldInfo["mount-socket"]
		if !ok || !ok2 {
			return nil, fmt.Errorf("registry mismatch: 'mount-socket' not found")
		}
		def := OptionDef[*bool]{EnvKey: opt.EnvKey, ToolGetter: opt.ToolGetter, GlobalGetter: opt.GlobalGetter}
		p1Set := cliVal.FieldByIndex(info.p1SetIdx).Bool()
		p1Val := cliVal.FieldByIndex(info.p1ValIdx).Bool()
		p2Set := cliVal.FieldByIndex(info.p2SetIdx).Bool()
		p2Val := cliVal.FieldByIndex(info.p2ValIdx).Bool()
		res.MountSocket, mountSocketSpecified = resolveBoolOptInfo(def, p1Set, p1Val, p2Set, p2Val, subcommand, tools, global, fs)
	}
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

	for _, opt := range StringSliceOptions {
		if opt.SkipResolution {
			continue
		}

		info, ok := fieldInfo[opt.Name]
		if !ok {
			return nil, fmt.Errorf("registry mismatch: info for string slice option %q not found", opt.Name)
		}

		p1Set, p1Val := getFieldInfo(cliVal, info.p1SetIdx, info.p1ValIdx)
		p2Set, p2Val := getFieldInfo(cliVal, info.p2SetIdx, info.p2ValIdx)

		var p1v, p2v []string
		if p1Set {
			if v, ok := p1Val.Interface().([]string); ok {
				p1v = v
			}
		}
		if p2Set {
			if v, ok := p2Val.Interface().([]string); ok {
				p2v = v
			}
		}

		def := OptionDef[[]string]{
			EnvKey:       opt.EnvKey,
			ToolGetter:   opt.ToolGetter,
			GlobalGetter: opt.GlobalGetter,
		}

		resolved := resolveStringSliceOpt(def, ",", p1v, p2v, subcommand, tools, global, r, fs)
		resVal.FieldByIndex(info.targetIdx).Set(reflect.ValueOf(resolved))
	}

	// Phase 8: Integer & Float options
	for _, opt := range IntOptions {
		info, ok := fieldInfo[opt.Name]
		if !ok {
			return nil, fmt.Errorf("registry mismatch: info for int option %q not found", opt.Name)
		}

		p1Set, p1Val := getFieldInfo(cliVal, info.p1SetIdx, info.p1ValIdx)
		p2Set, p2Val := getFieldInfo(cliVal, info.p2SetIdx, info.p2ValIdx)

		p1Int := 0
		if p1Set && p1Val.IsValid() {
			k := p1Val.Kind()
			if k >= reflect.Int && k <= reflect.Int64 {
				p1Int = int(p1Val.Int())
			} else {
				p1Set = false
			}
		}

		p2Int := 0
		if p2Set && p2Val.IsValid() {
			k := p2Val.Kind()
			if k >= reflect.Int && k <= reflect.Int64 {
				p2Int = int(p2Val.Int())
			} else {
				p2Set = false
			}
		}

		def := OptionDef[*int]{
			EnvKey:       opt.EnvKey,
			ToolGetter:   opt.ToolGetter,
			GlobalGetter: opt.GlobalGetter,
			Fallback:     &opt.Default,
		}

		resolved := resolveIntOpt(def, p1Set, p1Int, p2Set, p2Int, subcommand, tools, global, fs)
		resVal.FieldByIndex(info.targetIdx).SetInt(int64(resolved))
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

	for _, opt := range Float64Options {
		info, ok := fieldInfo[opt.Name]
		if !ok {
			return nil, fmt.Errorf("registry mismatch: info for float64 option %q not found", opt.Name)
		}

		p1Set, p1Val := getFieldInfo(cliVal, info.p1SetIdx, info.p1ValIdx)
		p2Set, p2Val := getFieldInfo(cliVal, info.p2SetIdx, info.p2ValIdx)

		p1Float := 0.0
		if p1Set && p1Val.IsValid() {
			k := p1Val.Kind()
			if k == reflect.Float32 || k == reflect.Float64 {
				p1Float = p1Val.Float()
			} else {
				p1Set = false
			}
		}

		p2Float := 0.0
		if p2Set && p2Val.IsValid() {
			k := p2Val.Kind()
			if k == reflect.Float32 || k == reflect.Float64 {
				p2Float = p2Val.Float()
			} else {
				p2Set = false
			}
		}

		def := OptionDef[*float64]{
			EnvKey:       opt.EnvKey,
			ToolGetter:   opt.ToolGetter,
			GlobalGetter: opt.GlobalGetter,
			Fallback:     &opt.Default,
		}

		resolved := resolveFloat64Opt(def, p1Set, p1Float, p2Set, p2Float, subcommand, tools, global, fs)
		resVal.FieldByIndex(info.targetIdx).SetFloat(resolved)
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
