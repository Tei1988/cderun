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

		p1SetIdx := []int{-1}
		p1SetField, ok1 := cliType.FieldByName("Cderun" + fieldName + "Set")
		if ok1 {
			p1SetIdx = p1SetField.Index
		}

		p1ValField, ok2 := cliType.FieldByName("Cderun" + fieldName)
		if !ok2 {
			return
		}

		p2SetIdx := []int{-1}
		p2SetField, ok3 := cliType.FieldByName(fieldName + "Set")
		if ok3 {
			p2SetIdx = p2SetField.Index
		}

		p2ValField, ok4 := cliType.FieldByName(fieldName)
		if !ok4 {
			return
		}

		fieldInfo[name] = optionFields{
			targetIdx: targetField.Index,
			p1SetIdx:  p1SetIdx,
			p1ValIdx:  p1ValField.Index,
			p2SetIdx:  p2SetIdx,
			p2ValIdx:  p2ValField.Index,
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

	cliVal := reflect.ValueOf(cli)
	resVal := reflect.ValueOf(res).Elem()

	// Phase 1: Early resolution (Diagnosis & StrictEnv)
	for _, name := range []string{"diagnosis", "strict-env"} {
		opt, ok := GetBoolOption(name)
		if !ok {
			continue
		}
		def := OptionDef[*bool]{
			EnvKey:       opt.EnvKey,
			ToolGetter:   opt.ToolGetter,
			GlobalGetter: opt.GlobalGetter,
		}

		fieldName := opt.FieldName
		if fieldName == "" {
			fieldName = PascalCase(opt.Name)
		}

		// Use reflection for early resolution as well to avoid duplication
		p1SetField := cliVal.FieldByName("Cderun" + fieldName + "Set")
		p1ValField := cliVal.FieldByName("Cderun" + fieldName)
		p2SetField := cliVal.FieldByName(fieldName + "Set")
		p2ValField := cliVal.FieldByName(fieldName)
		targetField := resVal.FieldByName(fieldName)

		if !p1SetField.IsValid() || !p1ValField.IsValid() || !p2SetField.IsValid() || !p2ValField.IsValid() || !targetField.IsValid() {
			continue
		}

		p1Set := p1SetField.Bool()
		p1Val := p1ValField.Bool()
		p2Set := p2SetField.Bool()
		p2Val := p2ValField.Bool()

		resolved := resolveBoolOpt(def, opt.Default, p1Set, p1Val, p2Set, p2Val, subcommand, tools, global, fs)
		targetField.SetBool(resolved)
	}

	// Phase 2: Registry-based options (String & Bool)
	fieldOnce.Do(initFieldInfo)

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

		p1Set := cliVal.FieldByIndex(info.p1SetIdx).Bool()
		p1Val := cliVal.FieldByIndex(info.p1ValIdx).Bool()
		p2Set := cliVal.FieldByIndex(info.p2SetIdx).Bool()
		p2Val := cliVal.FieldByIndex(info.p2ValIdx).Bool()

		def := OptionDef[*bool]{
			EnvKey:       opt.EnvKey,
			ToolGetter:   opt.ToolGetter,
			GlobalGetter: opt.GlobalGetter,
		}

		resolved := resolveBoolOpt(def, opt.Default, p1Set, p1Val, p2Set, p2Val, subcommand, tools, global, fs)
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

		p1Val, ok1 := (cliVal.FieldByIndex(info.p1ValIdx).Interface()).([]string)
		p2Val, ok2 := (cliVal.FieldByIndex(info.p2ValIdx).Interface()).([]string)
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("internal error: field %s in CLIOptions is not []string", opt.Name)
		}

		def := OptionDef[[]string]{
			EnvKey:       opt.EnvKey,
			ToolGetter:   opt.ToolGetter,
			GlobalGetter: opt.GlobalGetter,
		}

		resolved := resolveStringSliceOpt(def, ",", p1Val, p2Val, subcommand, tools, global, r, fs)
		resVal.FieldByIndex(info.targetIdx).Set(reflect.ValueOf(resolved))
	}

	// Phase 8: Integer & Float options
	for _, opt := range IntOptions {
		info, ok := fieldInfo[opt.Name]
		if !ok {
			return nil, fmt.Errorf("registry mismatch: info for int option %q not found", opt.Name)
		}

		p1Set := false
		if info.p1SetIdx[0] != -1 {
			p1Set = cliVal.FieldByIndex(info.p1SetIdx).Bool()
		}
		p1Val, ok1 := (cliVal.FieldByIndex(info.p1ValIdx).Interface()).(int)

		p2Set := false
		if info.p2SetIdx[0] != -1 {
			p2Set = cliVal.FieldByIndex(info.p2SetIdx).Bool()
		}
		p2Val, ok2 := (cliVal.FieldByIndex(info.p2ValIdx).Interface()).(int)
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("internal error: field %s in CLIOptions is not int", opt.Name)
		}

		def := OptionDef[*int]{
			EnvKey:       opt.EnvKey,
			ToolGetter:   opt.ToolGetter,
			GlobalGetter: opt.GlobalGetter,
			Fallback:     &opt.Default,
		}

		resolved := resolveIntOpt(def, p1Set, p1Val, p2Set, p2Val, subcommand, tools, global, fs)
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

		p1Set := false
		if info.p1SetIdx[0] != -1 {
			p1Set = cliVal.FieldByIndex(info.p1SetIdx).Bool()
		}
		p1Val, ok1 := (cliVal.FieldByIndex(info.p1ValIdx).Interface()).(float64)

		p2Set := false
		if info.p2SetIdx[0] != -1 {
			p2Set = cliVal.FieldByIndex(info.p2SetIdx).Bool()
		}
		p2Val, ok2 := (cliVal.FieldByIndex(info.p2ValIdx).Interface()).(float64)
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("internal error: field %s in CLIOptions is not float64", opt.Name)
		}

		def := OptionDef[*float64]{
			EnvKey:       opt.EnvKey,
			ToolGetter:   opt.ToolGetter,
			GlobalGetter: opt.GlobalGetter,
			Fallback:     &opt.Default,
		}

		resolved := resolveFloat64Opt(def, p1Set, p1Val, p2Set, p2Val, subcommand, tools, global, fs)
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
