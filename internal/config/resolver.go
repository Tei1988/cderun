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
	"regexp"

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

// resolver encapsulates the state for the configuration resolution process.
type resolver struct {
	subcommand string
	cli        CLIOptions
	tools      ToolsConfig
	global     *CDERunConfig
	fs         FileSystem
	expr       *ExpressionResolver

	cliVal reflect.Value
	res    *ResolvedConfig
	resVal reflect.Value
}

func newResolver(subcommand string, cli CLIOptions, tools ToolsConfig, global *CDERunConfig, fs FileSystem) (*resolver, error) {
	var hostCtx *HostContext
	if global != nil {
		hostCtx = global.HostContext
	}

	expr, err := NewExpressionResolverWithFS(hostCtx, fs)
	if err != nil {
		return nil, fmt.Errorf("failed to create expression resolver: %w", err)
	}

	res := &ResolvedConfig{
		HostContext: hostCtx,
	}

	return &resolver{
		subcommand: subcommand,
		cli:        cli,
		tools:      tools,
		global:     global,
		fs:         fs,
		expr:       expr,
		cliVal:     reflect.ValueOf(cli),
		res:        res,
		resVal:     reflect.ValueOf(res).Elem(),
	}, nil
}

func (r *resolver) resolveEarly() error {
	fieldOnce.Do(initFieldInfo)
	for _, name := range []string{"diagnosis", "strict-env"} {
		opt, ok := GetBoolOption(name)
		if !ok {
			continue
		}

		fieldName := opt.FieldName
		if fieldName == "" {
			fieldName = PascalCase(opt.Name)
		}

		targetField := r.resVal.FieldByName(fieldName)
		if !targetField.IsValid() {
			continue
		}

		p1SetField := r.cliVal.FieldByName("Cderun" + fieldName + "Set")
		p1ValField := r.cliVal.FieldByName("Cderun" + fieldName)
		p2SetField := r.cliVal.FieldByName(fieldName + "Set")
		p2ValField := r.cliVal.FieldByName(fieldName)

		if !p1SetField.IsValid() || !p1ValField.IsValid() || !p2SetField.IsValid() || !p2ValField.IsValid() {
			continue
		}

		def := OptionDef[*bool]{
			EnvKey:       opt.EnvKey,
			ToolGetter:   opt.ToolGetter,
			GlobalGetter: opt.GlobalGetter,
		}

		resolved := resolveBoolOpt(def, opt.Default, p1SetField.Bool(), p1ValField.Bool(), p2SetField.Bool(), p2ValField.Bool(), r.subcommand, r.tools, r.global, r.fs)
		targetField.SetBool(resolved)
	}
	return nil
}

func (r *resolver) resolveStandardBool(opt BoolOption) error {
	info, ok := fieldInfo[opt.Name]
	if !ok {
		return &RegistryMismatchError{
			Option:  opt.Name,
			Message: fmt.Sprintf("registry mismatch: info for bool option %q not found", opt.Name),
		}
	}

	p1Set, p1Val := getFieldInfo(r.cliVal, info.p1SetIdx, info.p1ValIdx)
	p2Set, p2Val := getFieldInfo(r.cliVal, info.p2SetIdx, info.p2ValIdx)

	def := OptionDef[*bool]{
		EnvKey:       opt.EnvKey,
		ToolGetter:   opt.ToolGetter,
		GlobalGetter: opt.GlobalGetter,
	}

	resolved := resolveBoolOpt(def, opt.Default, p1Set, p1Val.Bool(), p2Set, p2Val.Bool(), r.subcommand, r.tools, r.global, r.fs)
	r.resVal.FieldByIndex(info.targetIdx).SetBool(resolved)
	return nil
}

func (r *resolver) resolveStandardString(opt StringOption) error {
	if opt.SkipResolution {
		return nil
	}

	info, ok := fieldInfo[opt.Name]
	if !ok {
		return &RegistryMismatchError{
			Option:  opt.Name,
			Message: fmt.Sprintf("registry mismatch: CLI reflection fields for option %q missing in CLIOptions", opt.Name),
		}
	}

	p1Set, p1Val := getFieldInfo(r.cliVal, info.p1SetIdx, info.p1ValIdx)
	p2Set, p2Val := getFieldInfo(r.cliVal, info.p2SetIdx, info.p2ValIdx)

	def := OptionDef[string]{
		EnvKey:       opt.EnvKey,
		ToolGetter:   opt.ToolGetter,
		GlobalGetter: opt.GlobalGetter,
		Fallback:     opt.Default,
	}

	resolved := resolveStringOpt(def, p1Set, p1Val.String(), p2Set, p2Val.String(), r.subcommand, r.tools, r.global, r.expr, r.fs)
	r.resVal.FieldByIndex(info.targetIdx).SetString(resolved)
	return nil
}

func (r *resolver) resolveStandardInt(opt IntOption) error {
	info, ok := fieldInfo[opt.Name]
	if !ok {
		return &RegistryMismatchError{
			Option:  opt.Name,
			Message: fmt.Sprintf("registry mismatch: info for int option %q not found", opt.Name),
		}
	}

	p1Set, p1Val := getFieldInfo(r.cliVal, info.p1SetIdx, info.p1ValIdx)
	p2Set, p2Val := getFieldInfo(r.cliVal, info.p2SetIdx, info.p2ValIdx)

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

	resolved := resolveIntOpt(def, p1Set, p1Int, p2Set, p2Int, r.subcommand, r.tools, r.global, r.fs)
	r.resVal.FieldByIndex(info.targetIdx).SetInt(int64(resolved))
	return nil
}

func (r *resolver) resolveStandardFloat64(opt Float64Option) error {
	info, ok := fieldInfo[opt.Name]
	if !ok {
		return &RegistryMismatchError{
			Option:  opt.Name,
			Message: fmt.Sprintf("registry mismatch: info for float64 option %q not found", opt.Name),
		}
	}

	p1Set, p1Val := getFieldInfo(r.cliVal, info.p1SetIdx, info.p1ValIdx)
	p2Set, p2Val := getFieldInfo(r.cliVal, info.p2SetIdx, info.p2ValIdx)

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

	resolved := resolveFloat64Opt(def, p1Set, p1Float, p2Set, p2Float, r.subcommand, r.tools, r.global, r.fs)
	r.resVal.FieldByIndex(info.targetIdx).SetFloat(resolved)
	return nil
}

func (r *resolver) resolveStandardStringSlice(opt StringSliceOption) error {
	if opt.SkipResolution {
		return nil
	}

	info, ok := fieldInfo[opt.Name]
	if !ok {
		return &RegistryMismatchError{
			Option:  opt.Name,
			Message: fmt.Sprintf("registry mismatch: info for string slice option %q not found", opt.Name),
		}
	}

	p1Set, p1Val := getFieldInfo(r.cliVal, info.p1SetIdx, info.p1ValIdx)
	p2Set, p2Val := getFieldInfo(r.cliVal, info.p2SetIdx, info.p2ValIdx)

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

	resolved := resolveStringSliceOpt(def, ",", p1v, p2v, r.subcommand, r.tools, r.global, r.expr, r.fs)
	r.resVal.FieldByIndex(info.targetIdx).Set(reflect.ValueOf(resolved))
	return nil
}

func (r *resolver) resolveStandardOptions() error {
	// Phase 2: String options
	for _, opt := range StringOptions {
		if err := r.resolveStandardString(opt); err != nil {
			return err
		}
	}

	if r.res.Image == "" && r.subcommand != "" && !r.res.Diagnosis {
		return &ImageNotFoundError{Tool: r.subcommand}
	}
	if r.res.Image != "" {
		logging.Debug("Resolved Image: %s", r.res.Image)
	}

	// Phase 3: Remaining Boolean options
	for _, opt := range BoolOptions {
		if opt.Name == "diagnosis" || opt.Name == "strict-env" {
			continue
		}
		if opt.Name == "mount-socket" || opt.Name == "mount-cderun" || opt.Name == "mount-all-tools" {
			continue
		}
		if err := r.resolveStandardBool(opt); err != nil {
			return err
		}
	}

	// Phase 7: String Slice options (non-skipped)
	for _, opt := range StringSliceOptions {
		if err := r.resolveStandardStringSlice(opt); err != nil {
			return err
		}
	}

	// Phase 8: Integer & Float options
	for _, opt := range IntOptions {
		if err := r.resolveStandardInt(opt); err != nil {
			return err
		}
	}
	for _, opt := range Float64Options {
		if err := r.resolveStandardFloat64(opt); err != nil {
			return err
		}
	}

	if r.res.PullMaxRetries <= 0 {
		return &InvalidConfigError{Field: "PullMaxRetries", Value: fmt.Sprintf("%d", r.res.PullMaxRetries), Err: errors.New("must be greater than 0")}
	}

	return nil
}

func (r *resolver) resolveComplexOptions() error {
	var err error
	r.res.Mounts, err = resolveMounts(r.cli.CderunMounts, r.cli.Mounts, r.subcommand, r.tools, r.global, r.expr, r.fs)
	if err != nil {
		return err
	}

	r.res.Env, err = resolveEnv(r.cli.CderunEnv, r.cli.Env, "CDERUN_ENV", r.subcommand, r.tools, r.global, r.res.StrictEnv, r.expr, r.fs)
	if err != nil {
		return err
	}
	return nil
}

func (r *resolver) resolveRuntimeAndSocket() error {
	var err error
	r.res.SocketPath, err = resolveConfigPath(
		r.cli.CderunSocketPathSet, r.cli.CderunSocketPath,
		r.cli.SocketPathSet, r.cli.SocketPath,
		"CDERUN_SOCKET_PATH",
		"", nil, nil,
		r.global, func(g CDERunConfig) ConfigPath { return g.SocketPath },
		"",
		r.expr,
		"path",
		r.fs,
	)
	if err != nil {
		return err
	}

	if r.res.Runtime == "" {
		if r.res.SocketPath != "" {
			if strings.Contains(r.res.SocketPath, "podman") {
				r.res.Runtime = "podman"
			} else {
				r.res.Runtime = "docker"
			}
		} else {
			if _, err := r.fs.Stat("/var/run/docker.sock"); err == nil {
				r.res.Runtime = "docker"
				r.res.SocketPath = "/var/run/docker.sock"
			} else if _, err := r.fs.Stat("/run/podman/podman.sock"); err == nil {
				r.res.Runtime = "podman"
				r.res.SocketPath = "/run/podman/podman.sock"
			} else {
				r.res.Runtime = "docker"
				r.res.SocketPath = "/var/run/docker.sock"
			}
		}
	}

	if r.res.SocketPath == "" {
		if r.res.Runtime == "podman" {
			r.res.SocketPath = "/run/podman/podman.sock"
		} else {
			r.res.SocketPath = "/var/run/docker.sock"
		}
	}
	r.res.SocketPath = strings.TrimPrefix(r.res.SocketPath, "unix://")
	return nil
}

func (r *resolver) resolveTransitiveOptions() error {
	var err error
	r.res.MountTools = resolveStringSliceCommaOpt(
		OptionDef[[]string]{EnvKey: "CDERUN_MOUNT_TOOLS",
			ToolGetter:   func(t ToolConfig) []string { return t.MountTools },
			GlobalGetter: func(g CDERunConfig) []string { return g.Defaults.MountTools }},
		r.cli.CderunMountToolsSet, r.cli.CderunMountTools,
		r.cli.MountToolsSet, r.cli.MountTools,
		r.subcommand, r.tools, r.global, r.expr, r.fs,
	)

	// Resolve mount-all-tools (transitive trigger)
	{
		opt, ok := GetBoolOption("mount-all-tools")
		if !ok {
			return &RegistryMismatchError{Option: "mount-all-tools", Message: "registry mismatch: 'mount-all-tools' not found"}
		}
		info, ok2 := fieldInfo["mount-all-tools"]
		if !ok2 {
			return &RegistryMismatchError{Option: "mount-all-tools", Message: "registry mismatch: info for bool option \"mount-all-tools\" not found"}
		}
		p1Set := r.cliVal.FieldByIndex(info.p1SetIdx).Bool()
		p1Val := r.cliVal.FieldByIndex(info.p1ValIdx).Bool()
		p2Set := r.cliVal.FieldByIndex(info.p2SetIdx).Bool()
		p2Val := r.cliVal.FieldByIndex(info.p2ValIdx).Bool()
		def := OptionDef[*bool]{EnvKey: opt.EnvKey, ToolGetter: opt.ToolGetter, GlobalGetter: opt.GlobalGetter}
		r.res.MountAllTools = resolveBoolOpt(def, opt.Default, p1Set, p1Val, p2Set, p2Val, r.subcommand, r.tools, r.global, r.fs)
	}

	var mountCderunSpecified bool
	{
		opt, ok := GetBoolOption("mount-cderun")
		if !ok {
			return &RegistryMismatchError{Option: "mount-cderun", Message: "registry mismatch: 'mount-cderun' not found"}
		}
		info, ok2 := fieldInfo["mount-cderun"]
		if !ok2 {
			return &RegistryMismatchError{Option: "mount-cderun", Message: "registry mismatch: info for bool option \"mount-cderun\" not found"}
		}
		def := OptionDef[*bool]{EnvKey: opt.EnvKey, ToolGetter: opt.ToolGetter, GlobalGetter: opt.GlobalGetter}
		p1Set := r.cliVal.FieldByIndex(info.p1SetIdx).Bool()
		p1Val := r.cliVal.FieldByIndex(info.p1ValIdx).Bool()
		p2Set := r.cliVal.FieldByIndex(info.p2SetIdx).Bool()
		p2Val := r.cliVal.FieldByIndex(info.p2ValIdx).Bool()
		r.res.MountCderun, mountCderunSpecified = resolveBoolOptInfo(def, p1Set, p1Val, p2Set, p2Val, r.subcommand, r.tools, r.global, r.fs)
	}
	if !mountCderunSpecified {
		r.res.MountCderun = len(r.res.MountTools) > 0 || r.res.MountAllTools
	}

	r.res.MountCderunPath, err = resolveConfigPath(
		r.cli.CderunMountCderunPathSet, r.cli.CderunMountCderunPath,
		r.cli.MountCderunPathSet, r.cli.MountCderunPath,
		"CDERUN_MOUNT_CDERUN_PATH",
		r.subcommand, r.tools, func(t ToolConfig) ConfigPath { return t.MountCderunPath },
		r.global, func(g CDERunConfig) ConfigPath { return g.Defaults.MountCderunPath },
		"",
		r.expr,
		"path",
		r.fs,
	)
	if err != nil {
		return err
	}

	var mountSocketSpecified bool
	{
		opt, ok := GetBoolOption("mount-socket")
		if !ok {
			return &RegistryMismatchError{Option: "mount-socket", Message: "registry mismatch: 'mount-socket' not found"}
		}
		info, ok2 := fieldInfo["mount-socket"]
		if !ok2 {
			return &RegistryMismatchError{Option: "mount-socket", Message: "registry mismatch: info for bool option \"mount-socket\" not found"}
		}
		def := OptionDef[*bool]{EnvKey: opt.EnvKey, ToolGetter: opt.ToolGetter, GlobalGetter: opt.GlobalGetter}
		p1Set := r.cliVal.FieldByIndex(info.p1SetIdx).Bool()
		p1Val := r.cliVal.FieldByIndex(info.p1ValIdx).Bool()
		p2Set := r.cliVal.FieldByIndex(info.p2SetIdx).Bool()
		p2Val := r.cliVal.FieldByIndex(info.p2ValIdx).Bool()
		r.res.MountSocket, mountSocketSpecified = resolveBoolOptInfo(def, p1Set, p1Val, p2Set, p2Val, r.subcommand, r.tools, r.global, r.fs)
	}
	if !mountSocketSpecified {
		r.res.MountSocket = r.res.MountCderun
	}

	r.res.MountSocketPath, err = resolveConfigPath(
		r.cli.CderunMountSocketPathSet, r.cli.CderunMountSocketPath,
		r.cli.MountSocketPathSet, r.cli.MountSocketPath,
		"CDERUN_MOUNT_SOCKET_PATH",
		r.subcommand, r.tools, func(t ToolConfig) ConfigPath { return t.MountSocketPath },
		r.global, func(g CDERunConfig) ConfigPath { return g.Defaults.MountSocketPath },
		r.res.SocketPath,
		r.expr,
		"path",
		r.fs,
	)
	if err != nil {
		return err
	}
	return nil
}

func (r *resolver) resolveCustomParsing() error {
	// HangTimeout
	var hangTimeoutStr string
	if opt, ok := GetStringOption("hang-timeout"); ok {
		def := OptionDef[string]{
			EnvKey:       opt.EnvKey,
			ToolGetter:   opt.ToolGetter,
			GlobalGetter: opt.GlobalGetter,
			Fallback:     opt.Default,
		}
		hangTimeoutStr = resolveStringOpt(def, r.cli.CderunHangTimeoutSet, r.cli.CderunHangTimeout, r.cli.HangTimeoutSet, r.cli.HangTimeout, r.subcommand, r.tools, r.global, r.expr, r.fs)
	}
	if hangTimeoutStr != "" {
		if d, err := time.ParseDuration(hangTimeoutStr); err == nil {
			if d < 0 {
				return fmt.Errorf("invalid hang-timeout value %q: duration cannot be negative", hangTimeoutStr)
			}
			r.res.HangTimeout = d
		} else {
			return fmt.Errorf("invalid hang-timeout value %q: %w", hangTimeoutStr, err)
		}
	}

	// PullBackoffBase
	var pullBackoffBaseStr string
	if opt, ok := GetStringOption("pull-backoff-base"); ok {
		def := OptionDef[string]{
			EnvKey:       opt.EnvKey,
			ToolGetter:   opt.ToolGetter,
			GlobalGetter: opt.GlobalGetter,
			Fallback:     opt.Default,
		}
		pullBackoffBaseStr = resolveStringOpt(def, r.cli.CderunPullBackoffBaseSet, r.cli.CderunPullBackoffBase, r.cli.PullBackoffBaseSet, r.cli.PullBackoffBase, r.subcommand, r.tools, r.global, r.expr, r.fs)
	}
	if pullBackoffBaseStr != "" {
		if d, err := time.ParseDuration(pullBackoffBaseStr); err == nil {
			if d <= 0 {
				return fmt.Errorf("invalid PullBackoffBase duration %q: must be positive", pullBackoffBaseStr)
			}
			r.res.PullBackoffBase = d
		} else {
			return fmt.Errorf("failed to parse PullBackoffBase from %q: %w", pullBackoffBaseStr, err)
		}
	}

	// Devices
	var err error
	r.res.Devices, err = resolveDevices(r.cli.CderunDevices, r.cli.Devices, r.subcommand, r.tools, r.global, r.expr, r.fs)
	if err != nil {
		return err
	}

	// Memory
	var memStr string
	if opt, ok := GetStringOption("memory"); ok {
		def := OptionDef[string]{
			EnvKey:       opt.EnvKey,
			ToolGetter:   opt.ToolGetter,
			GlobalGetter: opt.GlobalGetter,
			Fallback:     opt.Default,
		}
		memStr = resolveStringOpt(def, r.cli.CderunMemorySet, r.cli.CderunMemory, r.cli.MemorySet, r.cli.Memory, r.subcommand, r.tools, r.global, r.expr, r.fs)
	}
	if memStr != "" {
		bytes, err := units.RAMInBytes(memStr)
		if err != nil {
			if exprErr := r.expr.Error(); exprErr != nil {
				return exprErr
			}
			return fmt.Errorf("invalid memory value %q: %w", memStr, err)
		}
		r.res.Memory = bytes
	}
	return nil
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
	// For slices and other types without a explicit Set flag
	v := val.FieldByIndex(valIdx)
	return !v.IsNil(), v
}

func ResolveWithFS(subcommand string, cli CLIOptions, tools ToolsConfig, global *CDERunConfig, fs FileSystem) (*ResolvedConfig, error) {
	logging.Trace("Resolving configurations for tool: %s", subcommand)

	rsv, err := newResolver(subcommand, cli, tools, global, fs)
	if err != nil {
		return nil, err
	}

	if err := rsv.resolveEarly(); err != nil {
		return nil, err
	}

	if err := rsv.resolveStandardOptions(); err != nil {
		return nil, err
	}

	if err := rsv.resolveComplexOptions(); err != nil {
		return nil, err
	}

	if err := rsv.resolveRuntimeAndSocket(); err != nil {
		return nil, err
	}

	if err := rsv.resolveTransitiveOptions(); err != nil {
		return nil, err
	}

	if err := rsv.resolveCustomParsing(); err != nil {
		return nil, err
	}

	if err := rsv.expr.Error(); err != nil {
		return nil, err
	}

	return rsv.res, nil
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
			masked := MaskSensitiveEnv([]string{resolvedE})
			logging.Debug("Environment variable: %s", masked[0])
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

var (
	sensitiveRegex    = regexp.MustCompile(`(?i)^(PASSWORD|SECRET|TOKEN|KEY|AUTH|SIG)$`)
	wordBoundaryRegex = regexp.MustCompile(`[^a-zA-Z0-9]+`)
	camelCaseRegex    = regexp.MustCompile(`([a-z])([A-Z])`)
)

// MaskSensitiveEnv redacts sensitive values from environment variables.
// It matches keys containing common sensitive keywords.
func MaskSensitiveEnv(env []string) []string {
	res := make([]string, len(env))
	for i, e := range env {
		key, val, ok := strings.Cut(e, "=")
		if !ok {
			res[i] = e
			continue
		}

		isSensitive := false
		// Normalize camelCase (apiToken -> api Token) before splitting
		normalizedKey := camelCaseRegex.ReplaceAllString(key, "$1 $2")

		// Match keywords within word boundaries (e.g. MY_PASSWORD matches, MONKEY does not)
		parts := wordBoundaryRegex.Split(normalizedKey, -1)
		for _, part := range parts {
			if part == "" {
				continue
			}
			if sensitiveRegex.MatchString(part) {
				isSensitive = true
				break
			}
		}

		if isSensitive {
			res[i] = key + "=[REDACTED]"
		} else {
			res[i] = key + "=" + val
		}
	}
	return res
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
