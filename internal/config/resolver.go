package config

import (
	"errors"
	"fmt"
	"path"
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
	Ports           []string
	PublishAll      bool
	Expose          []string
	Hostname        string
	DNS             []string
	AddHosts        []string
	Privileged      bool
	CapAdd          []string
	CapDrop         []string
	Entrypoint      []string
	Pull            string
	PullMaxRetries  int
	PullBackoffBase time.Duration
	Memory          int64
	CPUs            float64
	Devices         []container.DeviceMapping
	SensitiveEnv    []string
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
	Ports                    []string
	CderunPorts              []string
	PublishAll               bool
	PublishAllSet            bool
	CderunPublishAll         bool
	CderunPublishAllSet      bool
	Expose                   []string
	CderunExpose             []string
	Hostname                 string
	HostnameSet              bool
	CderunHostname           string
	CderunHostnameSet        bool
	DNS                      []string
	CderunDNS                []string
	AddHosts                 []string
	CderunAddHosts           []string
	User                     string
	UserSet                  bool
	CderunUser               string
	CderunUserSet            bool
	Privileged               bool
	PrivilegedSet            bool
	CderunPrivileged         bool
	CderunPrivilegedSet      bool
	CapAdd                   []string
	CderunCapAdd             []string
	CapDrop                  []string
	CderunCapDrop            []string
	Entrypoint               []string
	CderunEntrypoint         []string
	Pull                     string
	PullSet                  bool
	CderunPull               string
	CderunPullSet            bool
	PullMaxRetries           int
	PullMaxRetriesSet        bool
	CderunPullMaxRetries     int
	CderunPullMaxRetriesSet  bool
	PullBackoffBase          string
	PullBackoffBaseSet       bool
	CderunPullBackoffBase    string
	CderunPullBackoffBaseSet bool
	Memory                   string
	MemorySet                bool
	CderunMemory             string
	CderunMemorySet          bool
	CPUs                     float64
	CPUsSet                  bool
	CderunCPUs               float64
	CderunCPUsSet            bool
	Devices                  []string
	CderunDevices            []string
	SensitiveEnv             []string
	CderunSensitiveEnv       []string
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
	targetIdx int
	p1SetIdx  int
	p1ValIdx  int
	p2SetIdx  int
	p2ValIdx  int
}

func initFieldInfo() {
	fieldInfo = make(map[string]optionFields)

	process := func(name, fieldName string) {
		if fieldName == "" {
			fieldName = PascalCase(name)
		}

		targetField, ok := resType.FieldByName(fieldName)
		if !ok {
			return
		}

		info := optionFields{
			targetIdx: targetField.Index[0],
			p1SetIdx:  -1,
			p1ValIdx:  -1,
			p2SetIdx:  -1,
			p2ValIdx:  -1,
		}

		if f, ok := cliType.FieldByName("Cderun" + fieldName + "Set"); ok {
			info.p1SetIdx = f.Index[0]
		}
		if f, ok := cliType.FieldByName("Cderun" + fieldName); ok {
			info.p1ValIdx = f.Index[0]
		}
		if f, ok := cliType.FieldByName(fieldName + "Set"); ok {
			info.p2SetIdx = f.Index[0]
		}
		if f, ok := cliType.FieldByName(fieldName); ok {
			info.p2ValIdx = f.Index[0]
		}

		if info.p1ValIdx != -1 && info.p2ValIdx != -1 {
			fieldInfo[name] = info
		}
	}

	for _, opt := range StringOptions {
		process(opt.Name, opt.FieldName)
	}
	for _, opt := range BoolOptions {
		process(opt.Name, opt.FieldName)
	}
	for _, opt := range IntOptions {
		process(opt.Name, opt.FieldName)
	}
	for _, opt := range Float64Options {
		process(opt.Name, opt.FieldName)
	}
	for _, opt := range StringSliceOptions {
		process(opt.Name, opt.FieldName)
	}
}

func getFieldInfo(val reflect.Value, setIdx, valIdx int) (bool, reflect.Value) {
	if setIdx != -1 {
		return val.Field(setIdx).Bool(), val.Field(valIdx)
	}
	// For slices and other types without a explicit Set flag
	v := val.Field(valIdx)
	if v.Kind() == reflect.Slice || v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface || v.Kind() == reflect.Map {
		return !v.IsNil(), v
	}
	return !v.IsZero(), v
}

func fetchFieldAndParams(key string, cliVal reflect.Value) (optionFields, bool, reflect.Value, bool, reflect.Value, error) {
	info, ok := fieldInfo[key]
	if !ok {
		return optionFields{}, false, reflect.Value{}, false, reflect.Value{}, fmt.Errorf("registry mismatch: info for option %q not found", key)
	}

	if info.p1ValIdx == -1 || info.p2ValIdx == -1 {
		return optionFields{}, false, reflect.Value{}, false, reflect.Value{}, fmt.Errorf("registry mismatch: CLI reflection fields for option %q missing in CLIOptions", key)
	}

	p1Set, p1Val := getFieldInfo(cliVal, info.p1SetIdx, info.p1ValIdx)
	p2Set, p2Val := getFieldInfo(cliVal, info.p2SetIdx, info.p2ValIdx)
	return info, p1Set, p1Val, p2Set, p2Val, nil
}

type resolver struct {
	subcommand string
	cli        *CLIOptions
	tools      ToolsConfig
	global     *CDERunConfig
	fs         FileSystem
	r          *ExpressionResolver
	res        *ResolvedConfig
	cliVal     reflect.Value
	resVal     reflect.Value
}

func (rv *resolver) extractIntValue(v reflect.Value, set bool) (int, bool) {
	if !set || !v.IsValid() {
		return 0, false
	}
	k := v.Kind()
	if k >= reflect.Int && k <= reflect.Int64 {
		return int(v.Int()), true
	}
	return 0, false
}

func (rv *resolver) extractFloatValue(v reflect.Value, set bool) (float64, bool) {
	if !set || !v.IsValid() {
		return 0.0, false
	}
	k := v.Kind()
	if k == reflect.Float32 || k == reflect.Float64 {
		return v.Float(), true
	}
	return 0.0, false
}

func (rv *resolver) extractStringSliceValue(v reflect.Value, set bool) ([]string, bool) {
	if !set || !v.IsValid() {
		return nil, false
	}
	if val, ok := v.Interface().([]string); ok {
		return val, true
	}
	return nil, false
}

func (rv *resolver) resolvePathValue(name, envKey string, tGetter func(ToolConfig) ConfigPath, gGetter func(CDERunConfig) ConfigPath, fallback string) (string, error) {
	_, p1Set, p1Val, p2Set, p2Val, err := fetchFieldAndParams(name, rv.cliVal)
	if err != nil {
		return "", err
	}

	var tools ToolsConfig
	var subcommand string
	if tGetter != nil {
		tools = rv.tools
		subcommand = rv.subcommand
	}

	return resolveConfigPath(
		p1Set, p1Val.String(),
		p2Set, p2Val.String(),
		envKey,
		subcommand, tools, tGetter,
		rv.global, gGetter,
		fallback,
		rv.r,
		"path",
		rv.fs,
	)
}

func (rv *resolver) applyStringSliceOption(opt StringSliceOption) error {
	info, p1Set, p1Val, p2Set, p2Val, err := fetchFieldAndParams(opt.Name, rv.cliVal)
	if err != nil {
		return err
	}

	p1v, _ := rv.extractStringSliceValue(p1Val, p1Set)
	p2v, _ := rv.extractStringSliceValue(p2Val, p2Set)

	def := OptionDef[[]string]{
		EnvKey:       opt.EnvKey,
		ToolGetter:   opt.ToolGetter,
		GlobalGetter: opt.GlobalGetter,
	}

	resolved := resolveStringSliceOpt(def, ",", p1v, p2v, rv.subcommand, rv.tools, rv.global, rv.r, rv.fs)
	rv.resVal.Field(info.targetIdx).Set(reflect.ValueOf(resolved))
	return nil
}

func (rv *resolver) applyStringOption(opt StringOption) error {
	if _, ok := fieldInfo[opt.Name]; !ok {
		return fmt.Errorf("registry mismatch: info for option %q not found", opt.Name)
	}

	var p1Set, p2Set bool
	var p1Val, p2Val string

	// Fast-path for common options to avoid reflection
	switch opt.Name {
	case "image":
		p1Set, p1Val, p2Set, p2Val = rv.cli.CderunImageSet, rv.cli.CderunImage, rv.cli.ImageSet, rv.cli.Image
	case "network":
		p1Set, p1Val, p2Set, p2Val = rv.cli.CderunNetworkSet, rv.cli.CderunNetwork, rv.cli.NetworkSet, rv.cli.Network
	case "workdir":
		p1Set, p1Val, p2Set, p2Val = rv.cli.CderunWorkdirSet, rv.cli.CderunWorkdir, rv.cli.WorkdirSet, rv.cli.Workdir
	case "runtime":
		p1Set, p1Val, p2Set, p2Val = rv.cli.CderunRuntimeSet, rv.cli.CderunRuntime, rv.cli.RuntimeSet, rv.cli.Runtime
	case "user":
		p1Set, p1Val, p2Set, p2Val = rv.cli.CderunUserSet, rv.cli.CderunUser, rv.cli.UserSet, rv.cli.User
	case "log-level":
		p1Set, p1Val, p2Set, p2Val = rv.cli.CderunLogLevelSet, rv.cli.CderunLogLevel, rv.cli.LogLevelSet, rv.cli.LogLevel
	case "log-format":
		p1Set, p1Val, p2Set, p2Val = rv.cli.CderunLogFormatSet, rv.cli.CderunLogFormat, rv.cli.LogFormatSet, rv.cli.LogFormat
	case "hostname":
		p1Set, p1Val, p2Set, p2Val = rv.cli.CderunHostnameSet, rv.cli.CderunHostname, rv.cli.HostnameSet, rv.cli.Hostname
	case "pull":
		p1Set, p1Val, p2Set, p2Val = rv.cli.CderunPullSet, rv.cli.CderunPull, rv.cli.PullSet, rv.cli.Pull
	case "dry-run-format":
		p1Set, p1Val, p2Set, p2Val = rv.cli.CderunDryRunFormatSet, rv.cli.CderunDryRunFormat, rv.cli.DryRunFormatSet, rv.cli.DryRunFormat
	case "diagnosis-format":
		p1Set, p1Val, p2Set, p2Val = rv.cli.CderunDiagnosisFormatSet, rv.cli.CderunDiagnosisFormat, rv.cli.DiagnosisFormatSet, rv.cli.DiagnosisFormat
	case "socket-path":
		p1Set, p1Val, p2Set, p2Val = rv.cli.CderunSocketPathSet, rv.cli.CderunSocketPath, rv.cli.SocketPathSet, rv.cli.SocketPath
	case "mount-socket-path":
		p1Set, p1Val, p2Set, p2Val = rv.cli.CderunMountSocketPathSet, rv.cli.CderunMountSocketPath, rv.cli.MountSocketPathSet, rv.cli.MountSocketPath
	case "mount-cderun-path":
		p1Set, p1Val, p2Set, p2Val = rv.cli.CderunMountCderunPathSet, rv.cli.CderunMountCderunPath, rv.cli.MountCderunPathSet, rv.cli.MountCderunPath
	default:
		info, s1, v1, s2, v2, err := fetchFieldAndParams(opt.Name, rv.cliVal)
		if err != nil {
			return err
		}
		p1Set, p1Val, p2Set, p2Val = s1, v1.String(), s2, v2.String()
		def := OptionDef[string]{EnvKey: opt.EnvKey, ToolGetter: opt.ToolGetter, GlobalGetter: opt.GlobalGetter, Fallback: opt.Default}
		resolved := resolveStringOpt(def, p1Set, p1Val, p2Set, p2Val, rv.subcommand, rv.tools, rv.global, rv.r, rv.fs)
		rv.resVal.Field(info.targetIdx).SetString(resolved)
		return nil
	}

	def := OptionDef[string]{
		EnvKey:       opt.EnvKey,
		ToolGetter:   opt.ToolGetter,
		GlobalGetter: opt.GlobalGetter,
		Fallback:     opt.Default,
	}

	resolved := resolveStringOpt(def, p1Set, p1Val, p2Set, p2Val, rv.subcommand, rv.tools, rv.global, rv.r, rv.fs)
	// Apply resolved value back to rv.res fields directly for fast-path options
	switch opt.Name {
	case "image":
		rv.res.Image = resolved
	case "network":
		rv.res.Network = resolved
	case "workdir":
		rv.res.Workdir = resolved
	case "runtime":
		rv.res.Runtime = resolved
	case "user":
		rv.res.User = resolved
	case "log-level":
		rv.res.LogLevel = resolved
	case "log-format":
		rv.res.LogFormat = resolved
	case "hostname":
		rv.res.Hostname = resolved
	case "pull":
		rv.res.Pull = resolved
	case "dry-run-format":
		rv.res.DryRunFormat = resolved
	case "diagnosis-format":
		rv.res.DiagnosisFormat = resolved
	case "socket-path":
		rv.res.SocketPath = resolved
	case "mount-socket-path":
		rv.res.MountSocketPath = resolved
	case "mount-cderun-path":
		rv.res.MountCderunPath = resolved
	}

	return nil
}

func (rv *resolver) applyBoolOption(opt BoolOption) error {
	if _, ok := fieldInfo[opt.Name]; !ok {
		return fmt.Errorf("registry mismatch: info for option %q not found", opt.Name)
	}

	var p1Set, p2Set bool
	var p1Val, p2Val bool

	// Fast-path for common options
	switch opt.Name {
	case "tty":
		p1Set, p1Val, p2Set, p2Val = rv.cli.CderunTTYSet, rv.cli.CderunTTY, rv.cli.TTYSet, rv.cli.TTY
	case "interactive":
		p1Set, p1Val, p2Set, p2Val = rv.cli.CderunInteractiveSet, rv.cli.CderunInteractive, rv.cli.InteractiveSet, rv.cli.Interactive
	case "remove":
		p1Set, p1Val, p2Set, p2Val = rv.cli.CderunRemoveSet, rv.cli.CderunRemove, rv.cli.RemoveSet, rv.cli.Remove
	case "diagnosis":
		p1Set, p1Val, p2Set, p2Val = rv.cli.CderunDiagnosisSet, rv.cli.CderunDiagnosis, rv.cli.DiagnosisSet, rv.cli.Diagnosis
	case "strict-env":
		p1Set, p1Val, p2Set, p2Val = rv.cli.CderunStrictEnvSet, rv.cli.CderunStrictEnv, rv.cli.StrictEnvSet, rv.cli.StrictEnv
	case "privileged":
		p1Set, p1Val, p2Set, p2Val = rv.cli.CderunPrivilegedSet, rv.cli.CderunPrivileged, rv.cli.PrivilegedSet, rv.cli.Privileged
	case "publish-all":
		p1Set, p1Val, p2Set, p2Val = rv.cli.CderunPublishAllSet, rv.cli.CderunPublishAll, rv.cli.PublishAllSet, rv.cli.PublishAll
	case "log-timestamp":
		p1Set, p1Val, p2Set, p2Val = rv.cli.CderunLogTimestampSet, rv.cli.CderunLogTimestamp, rv.cli.LogTimestampSet, rv.cli.LogTimestamp
	case "mount-socket":
		p1Set, p1Val, p2Set, p2Val = rv.cli.CderunMountSocketSet, rv.cli.CderunMountSocket, rv.cli.MountSocketSet, rv.cli.MountSocket
	case "mount-cderun":
		p1Set, p1Val, p2Set, p2Val = rv.cli.CderunMountCderunSet, rv.cli.CderunMountCderun, rv.cli.MountCderunSet, rv.cli.MountCderun
	case "mount-all-tools":
		p1Set, p1Val, p2Set, p2Val = rv.cli.CderunMountAllToolsSet, rv.cli.CderunMountAllTools, rv.cli.MountAllToolsSet, rv.cli.MountAllTools
	case "dry-run":
		p1Set, p1Val, p2Set, p2Val = rv.cli.CderunDryRunSet, rv.cli.CderunDryRun, rv.cli.DryRunSet, rv.cli.DryRun
	default:
		info, s1, v1, s2, v2, err := fetchFieldAndParams(opt.Name, rv.cliVal)
		if err != nil {
			return err
		}
		p1Set, p1Val, p2Set, p2Val = s1, v1.Bool(), s2, v2.Bool()
		def := OptionDef[*bool]{EnvKey: opt.EnvKey, ToolGetter: opt.ToolGetter, GlobalGetter: opt.GlobalGetter}
		resolved := resolveBoolOpt(def, opt.Default, p1Set, p1Val, p2Set, p2Val, rv.subcommand, rv.tools, rv.global, rv.fs)
		rv.resVal.Field(info.targetIdx).SetBool(resolved)
		return nil
	}

	def := OptionDef[*bool]{
		EnvKey:       opt.EnvKey,
		ToolGetter:   opt.ToolGetter,
		GlobalGetter: opt.GlobalGetter,
	}

	resolved := resolveBoolOpt(def, opt.Default, p1Set, p1Val, p2Set, p2Val, rv.subcommand, rv.tools, rv.global, rv.fs)
	switch opt.Name {
	case "tty":
		rv.res.TTY = resolved
	case "interactive":
		rv.res.Interactive = resolved
	case "remove":
		rv.res.Remove = resolved
	case "diagnosis":
		rv.res.Diagnosis = resolved
	case "strict-env":
		rv.res.StrictEnv = resolved
	case "privileged":
		rv.res.Privileged = resolved
	case "publish-all":
		rv.res.PublishAll = resolved
	case "log-timestamp":
		rv.res.LogTimestamp = resolved
	case "mount-socket":
		rv.res.MountSocket = resolved
	case "mount-cderun":
		rv.res.MountCderun = resolved
	case "mount-all-tools":
		rv.res.MountAllTools = resolved
	case "dry-run":
		rv.res.DryRun = resolved
	}
	return nil
}

func (rv *resolver) applyIntOption(opt IntOption) error {
	info, s1, v1, s2, v2, err := fetchFieldAndParams(opt.Name, rv.cliVal)
	if err != nil {
		return err
	}

	var p1Set, p2Set bool
	var p1Int, p2Int int

	// Fast-path for common options
	if opt.Name == "pull-max-retries" &&
		info.p1ValIdx != -1 && cliType.Field(info.p1ValIdx).Name == "CderunPullMaxRetries" &&
		info.p2ValIdx != -1 && cliType.Field(info.p2ValIdx).Name == "PullMaxRetries" {
		p1Set, p1Int, p2Set, p2Int = rv.cli.CderunPullMaxRetriesSet, rv.cli.CderunPullMaxRetries, rv.cli.PullMaxRetriesSet, rv.cli.PullMaxRetries
	} else {
		p1Int, p1Set = rv.extractIntValue(v1, s1)
		p2Int, p2Set = rv.extractIntValue(v2, s2)
	}

	def := OptionDef[*int]{
		EnvKey:       opt.EnvKey,
		ToolGetter:   opt.ToolGetter,
		GlobalGetter: opt.GlobalGetter,
		Fallback:     &opt.Default,
	}

	resolved := resolveIntOpt(def, p1Set, p1Int, p2Set, p2Int, rv.subcommand, rv.tools, rv.global, rv.fs)
	if opt.Name == "pull-max-retries" {
		rv.res.PullMaxRetries = resolved
	} else {
		rv.resVal.Field(info.targetIdx).SetInt(int64(resolved))
	}
	return nil
}

func (rv *resolver) applyFloat64Option(opt Float64Option) error {
	info, s1, v1, s2, v2, err := fetchFieldAndParams(opt.Name, rv.cliVal)
	if err != nil {
		return err
	}

	var p1Set, p2Set bool
	var p1Float, p2Float float64

	// Fast-path for common options
	if opt.Name == "cpus" &&
		info.p1ValIdx != -1 && cliType.Field(info.p1ValIdx).Name == "CderunCPUs" &&
		info.p2ValIdx != -1 && cliType.Field(info.p2ValIdx).Name == "CPUs" {
		p1Set, p1Float, p2Set, p2Float = rv.cli.CderunCPUsSet, rv.cli.CderunCPUs, rv.cli.CPUsSet, rv.cli.CPUs
	} else {
		p1Float, p1Set = rv.extractFloatValue(v1, s1)
		p2Float, p2Set = rv.extractFloatValue(v2, s2)
	}

	def := OptionDef[*float64]{
		EnvKey:       opt.EnvKey,
		ToolGetter:   opt.ToolGetter,
		GlobalGetter: opt.GlobalGetter,
		Fallback:     &opt.Default,
	}

	resolved := resolveFloat64Opt(def, p1Set, p1Float, p2Set, p2Float, rv.subcommand, rv.tools, rv.global, rv.fs)
	if opt.Name == "cpus" {
		rv.res.CPUs = resolved
	} else {
		rv.resVal.Field(info.targetIdx).SetFloat(resolved)
	}
	return nil
}

func (rv *resolver) applyDurationOption(opt StringOption, target *time.Duration, positive bool) error {
	def := OptionDef[string]{
		EnvKey:       opt.EnvKey,
		ToolGetter:   opt.ToolGetter,
		GlobalGetter: opt.GlobalGetter,
		Fallback:     opt.Default,
	}

	var p1Set, p2Set bool
	var p1Val, p2Val string
	switch opt.Name {
	case "hang-timeout":
		p1Set, p1Val, p2Set, p2Val = rv.cli.CderunHangTimeoutSet, rv.cli.CderunHangTimeout, rv.cli.HangTimeoutSet, rv.cli.HangTimeout
	case "pull-backoff-base":
		p1Set, p1Val, p2Set, p2Val = rv.cli.CderunPullBackoffBaseSet, rv.cli.CderunPullBackoffBase, rv.cli.PullBackoffBaseSet, rv.cli.PullBackoffBase
	default:
		_, s1, v1, s2, v2, err := fetchFieldAndParams(opt.Name, rv.cliVal)
		if err != nil {
			return err
		}
		p1Set, p1Val, p2Set, p2Val = s1, v1.String(), s2, v2.String()
	}

	valStr := resolveStringOpt(def, p1Set, p1Val, p2Set, p2Val, rv.subcommand, rv.tools, rv.global, rv.r, rv.fs)
	if valStr != "" {
		d, err := time.ParseDuration(valStr)
		if err != nil {
			return &InvalidConfigError{Field: opt.Name, Value: valStr, Err: err}
		}
		if positive && d <= 0 {
			return &InvalidConfigError{Field: opt.Name, Value: valStr, Err: errors.New("must be positive")}
		}
		if !positive && d < 0 {
			return &InvalidConfigError{Field: opt.Name, Value: valStr, Err: errors.New("duration cannot be negative")}
		}
		*target = d
	}
	return nil
}

func (rv *resolver) applyMemoryOption(opt StringOption, target *int64) error {
	def := OptionDef[string]{
		EnvKey:       opt.EnvKey,
		ToolGetter:   opt.ToolGetter,
		GlobalGetter: opt.GlobalGetter,
		Fallback:     opt.Default,
	}

	var p1Set, p2Set bool
	var p1Val, p2Val string
	switch opt.Name {
	case "memory":
		p1Set, p1Val, p2Set, p2Val = rv.cli.CderunMemorySet, rv.cli.CderunMemory, rv.cli.MemorySet, rv.cli.Memory
	default:
		_, s1, v1, s2, v2, err := fetchFieldAndParams(opt.Name, rv.cliVal)
		if err != nil {
			return err
		}
		p1Set, p1Val, p2Set, p2Val = s1, v1.String(), s2, v2.String()
	}

	valStr := resolveStringOpt(def, p1Set, p1Val, p2Set, p2Val, rv.subcommand, rv.tools, rv.global, rv.r, rv.fs)
	if valStr != "" {
		bytes, err := units.RAMInBytes(valStr)
		if err != nil {
			if exprErr := rv.r.Error(); exprErr != nil {
				return exprErr
			}
			return &InvalidConfigError{Field: opt.Name, Value: valStr, Err: err}
		}
		*target = bytes
	}
	return nil
}

func ResolveWithFS(subcommand string, cli *CLIOptions, tools ToolsConfig, global *CDERunConfig, fs FileSystem) (*ResolvedConfig, error) {
	if cli == nil {
		cli = &CLIOptions{}
	}
	if subcommand != "" {
		if err := ValidateToolName(subcommand); err != nil {
			return nil, err
		}
	}
	if logging.TraceEnabled() {
		logging.Trace("Resolving configurations for tool: %s", subcommand)
	}

	var hostCtx *HostContext
	if global != nil {
		hostCtx = global.HostContext
	}

	r, err := NewExpressionResolverWithFS(hostCtx, fs)
	if err != nil {
		return nil, fmt.Errorf("failed to create expression resolver: %w", err)
	}

	fieldOnce.Do(initFieldInfo)

	res := &ResolvedConfig{}
	rv := &resolver{
		subcommand: subcommand,
		cli:        cli,
		tools:      tools,
		global:     global,
		fs:         fs,
		r:          r,
		res:        res,
		cliVal:     reflect.ValueOf(cli).Elem(),
		resVal:     reflect.ValueOf(res).Elem(),
	}

	if err := rv.resolveEarly(); err != nil {
		return nil, err
	}

	if err := rv.resolveStandardOptions(); err != nil {
		return nil, err
	}

	if err := rv.resolveComplexOptions(); err != nil {
		return nil, err
	}

	if err := rv.resolveRuntimeAndSocket(); err != nil {
		return nil, err
	}

	if err := rv.resolveTransitiveOptions(); err != nil {
		return nil, err
	}

	if err := rv.resolveCustomParsing(); err != nil {
		return nil, err
	}

	if err := rv.validateSecurity(); err != nil {
		return nil, err
	}

	return rv.res, nil
}

func (rv *resolver) resolveEarly() error {
	// Early resolution (Diagnosis & StrictEnv)
	for _, name := range []string{"diagnosis", "strict-env"} {
		opt, ok := GetBoolOption(name)
		if !ok {
			return fmt.Errorf("registry mismatch: early boolean option %q not found", name)
		}
		if err := rv.applyBoolOption(opt); err != nil {
			return err
		}
	}

	// Sensitive Env Resolution (needed for masking in debug logs during further resolution)
	{
		opt, ok := GetStringSliceOption("sensitive-env")
		if !ok {
			return fmt.Errorf("registry mismatch: early string slice option %q not found", "sensitive-env")
		}
		_, p1Set, p1Val, p2Set, p2Val, err := fetchFieldAndParams("sensitive-env", rv.cliVal)
		if err != nil {
			return err
		}
		p1v, _ := rv.extractStringSliceValue(p1Val, p1Set)
		p2v, _ := rv.extractStringSliceValue(p2Val, p2Set)

		def := OptionDef[[]string]{
			EnvKey:       opt.EnvKey,
			ToolGetter:   opt.ToolGetter,
			GlobalGetter: opt.GlobalGetter,
		}

		rv.res.SensitiveEnv = resolveStringSliceOpt(def, ",", p1v, p2v, rv.subcommand, rv.tools, rv.global, rv.r, rv.fs)
	}
	return nil
}

func (rv *resolver) resolveStandardOptions() error {
	// Registry-based options (String & Bool)

	for _, opt := range StringOptions {
		if opt.SkipResolution {
			continue
		}

		if err := rv.applyStringOption(opt); err != nil {
			return err
		}
	}

	if rv.res.Image == "" && rv.subcommand != "" && !rv.res.Diagnosis {
		return &ImageNotFoundError{Tool: rv.subcommand}
	}
	if rv.res.Image != "" {
		// Security validation before any further use (including logging)
		if err := validatePathChars(rv.res.Image); err != nil {
			return fmt.Errorf("security validation failed for image: %w", err)
		}

		// Registry mismatch validation: if image is provided via CLI/Env, ensure it matches tool config registry
		if rv.tools != nil {
			if tool, ok := rv.tools[rv.subcommand]; ok && tool.Image != "" {
				cliImage := ""
				if rv.cli.CderunImageSet {
					cliImage = rv.cli.CderunImage
				} else if rv.cli.ImageSet {
					cliImage = rv.cli.Image
				} else if env := rv.fs.Getenv("CDERUN_IMAGE"); env != "" {
					cliImage = env
				}

				if cliImage != "" {
					resolvedCLIImage, errCLI := rv.r.ResolveString(cliImage)
					resolvedCfgImage, errCfg := rv.r.ResolveString(tool.Image)
					if errCLI == nil && errCfg == nil {
						if err := validateImageRegistryMatch(resolvedCLIImage, resolvedCfgImage); err != nil {
							return err
						}
					}
				}
			}
		}

		if logging.DebugEnabled() {
			logging.Debug("Resolved Image: %s", rv.res.Image)
		}
	}

	// Remaining Boolean options
	for _, opt := range BoolOptions {
		// Skip early options already resolved in resolveEarly
		if opt.Name == "diagnosis" || opt.Name == "strict-env" {
			continue
		}
		// Skip transitive options handled in resolveTransitiveOptions
		if opt.Name == "mount-socket" || opt.Name == "mount-cderun" || opt.Name == "mount-all-tools" {
			continue
		}

		if err := rv.applyBoolOption(opt); err != nil {
			return err
		}
	}
	return nil
}

func (rv *resolver) resolveComplexOptions() error {
	var err error
	// Complex types (Mounts, Env)
	rv.res.Mounts, err = resolveMounts(rv.cli.CderunMounts, rv.cli.Mounts, rv.subcommand, rv.tools, rv.global, rv.r, rv.fs)
	if err != nil {
		return err
	}

	rv.res.Env, err = resolveEnv(rv.cli.CderunEnv, rv.cli.Env, "CDERUN_ENV", rv.subcommand, rv.tools, rv.global, rv.res.SensitiveEnv, rv.res.StrictEnv, rv.r, rv.fs)
	if err != nil {
		return err
	}
	return nil
}

func (rv *resolver) resolveRuntimeAndSocket() error {
	// Path resolution & Auto-detection (Socket)
	{
		var errPath error
		rv.res.SocketPath, errPath = rv.resolvePathValue(
			"socket-path",
			"CDERUN_SOCKET_PATH",
			nil,
			func(g CDERunConfig) ConfigPath { return g.SocketPath },
			"",
		)
		if errPath != nil {
			return errPath
		}
	}

	if rv.res.Runtime == "" {
		if rv.res.SocketPath != "" {
			if strings.Contains(rv.res.SocketPath, "podman") {
				rv.res.Runtime = "podman"
			} else if strings.Contains(rv.res.SocketPath, "containerd") {
				rv.res.Runtime = "containerd"
			} else {
				rv.res.Runtime = "docker"
			}
		} else {
			if _, err := rv.r.Stat("/var/run/docker.sock"); err == nil {
				rv.res.Runtime = "docker"
				rv.res.SocketPath = "/var/run/docker.sock"
			} else if _, err := rv.r.Stat("/run/containerd/containerd.sock"); err == nil {
				rv.res.Runtime = "containerd"
				rv.res.SocketPath = "/run/containerd/containerd.sock"
			} else if _, err := rv.r.Stat("/run/podman/podman.sock"); err == nil {
				rv.res.Runtime = "podman"
				rv.res.SocketPath = "/run/podman/podman.sock"
			} else {
				rv.res.Runtime = "docker"
				rv.res.SocketPath = "/var/run/docker.sock"
			}
		}
	}

	if rv.res.SocketPath == "" {
		if rv.res.Runtime == "podman" {
			rv.res.SocketPath = "/run/podman/podman.sock"
		} else if rv.res.Runtime == "containerd" {
			rv.res.SocketPath = "/run/containerd/containerd.sock"
		} else {
			rv.res.SocketPath = "/var/run/docker.sock"
		}
	}
	rv.res.SocketPath = strings.TrimPrefix(rv.res.SocketPath, "unix://")
	return nil
}

func (rv *resolver) resolveTransitiveOptions() error {
	// Transitive options (MountTools -> MountCderun -> MountSocket)
	rv.res.MountTools = resolveStringSliceCommaOpt(
		OptionDef[[]string]{EnvKey: "CDERUN_MOUNT_TOOLS",
			ToolGetter:   func(t ToolConfig) []string { return t.MountTools },
			GlobalGetter: func(g CDERunConfig) []string { return g.Defaults.MountTools }},
		rv.cli.CderunMountToolsSet, rv.cli.CderunMountTools,
		rv.cli.MountToolsSet, rv.cli.MountTools,
		rv.subcommand, rv.tools, rv.global, rv.r, rv.fs,
	)
	for _, tool := range rv.res.MountTools {
		if err := ValidateToolName(tool); err != nil {
			return fmt.Errorf("invalid tool name in mount-tools: %w", err)
		}
	}

	// Resolve mount-all-tools (transitive trigger)
	{
		opt, _ := GetBoolOption("mount-all-tools")
		_, p1Set, p1Val, p2Set, p2Val, err := fetchFieldAndParams("mount-all-tools", rv.cliVal)
		if err != nil {
			return err
		}
		def := OptionDef[*bool]{EnvKey: opt.EnvKey, ToolGetter: opt.ToolGetter, GlobalGetter: opt.GlobalGetter}
		rv.res.MountAllTools = resolveBoolOpt(def, opt.Default, p1Set, p1Val.Bool(), p2Set, p2Val.Bool(), rv.subcommand, rv.tools, rv.global, rv.fs)
	}

	var mountCderunSpecified bool
	{
		opt, _ := GetBoolOption("mount-cderun")
		_, p1Set, p1Val, p2Set, p2Val, err := fetchFieldAndParams("mount-cderun", rv.cliVal)
		if err != nil {
			return err
		}
		def := OptionDef[*bool]{EnvKey: opt.EnvKey, ToolGetter: opt.ToolGetter, GlobalGetter: opt.GlobalGetter}
		rv.res.MountCderun, mountCderunSpecified = resolveBoolOptInfo(def, p1Set, p1Val.Bool(), p2Set, p2Val.Bool(), rv.subcommand, rv.tools, rv.global, rv.fs)
	}
	if !mountCderunSpecified {
		rv.res.MountCderun = len(rv.res.MountTools) > 0 || rv.res.MountAllTools
	}

	{
		var errPath error
		rv.res.MountCderunPath, errPath = rv.resolvePathValue(
			"mount-cderun-path",
			"CDERUN_MOUNT_CDERUN_PATH",
			func(t ToolConfig) ConfigPath { return t.MountCderunPath },
			func(g CDERunConfig) ConfigPath { return g.Defaults.MountCderunPath },
			"",
		)
		if errPath != nil {
			return errPath
		}
	}

	var mountSocketSpecified bool
	{
		opt, _ := GetBoolOption("mount-socket")
		_, p1Set, p1Val, p2Set, p2Val, err := fetchFieldAndParams("mount-socket", rv.cliVal)
		if err != nil {
			return err
		}
		def := OptionDef[*bool]{EnvKey: opt.EnvKey, ToolGetter: opt.ToolGetter, GlobalGetter: opt.GlobalGetter}
		rv.res.MountSocket, mountSocketSpecified = resolveBoolOptInfo(def, p1Set, p1Val.Bool(), p2Set, p2Val.Bool(), rv.subcommand, rv.tools, rv.global, rv.fs)
	}
	if !mountSocketSpecified {
		rv.res.MountSocket = rv.res.MountCderun
	}

	{
		var errPath error
		rv.res.MountSocketPath, errPath = rv.resolvePathValue(
			"mount-socket-path",
			"CDERUN_MOUNT_SOCKET_PATH",
			func(t ToolConfig) ConfigPath { return t.MountSocketPath },
			func(g CDERunConfig) ConfigPath { return g.Defaults.MountSocketPath },
			rv.res.SocketPath,
		)
		if errPath != nil {
			return errPath
		}
	}
	return nil
}

func (rv *resolver) resolveCustomParsing() error {
	// Duration and Slice options
	// Resolve hang-timeout via registry entry (skipped in resolveStandardOptions)
	if opt, ok := GetStringOption("hang-timeout"); ok {
		if err := rv.applyDurationOption(opt, &rv.res.HangTimeout, false); err != nil {
			return err
		}
	}

	for _, opt := range StringSliceOptions {
		if opt.SkipResolution {
			continue
		}

		if err := rv.applyStringSliceOption(opt); err != nil {
			return err
		}
	}

	// Integer & Float options
	for _, opt := range IntOptions {
		if err := rv.applyIntOption(opt); err != nil {
			return err
		}
	}

	if rv.res.PullMaxRetries <= 0 {
		return &InvalidConfigError{Field: "pull-max-retries", Value: fmt.Sprintf("%d", rv.res.PullMaxRetries), Err: errors.New("must be greater than 0")}
	}

	// Resolve pull-backoff-base via registry
	if opt, ok := GetStringOption("pull-backoff-base"); ok {
		if err := rv.applyDurationOption(opt, &rv.res.PullBackoffBase, true); err != nil {
			return err
		}
	}

	var errDevices error
	rv.res.Devices, errDevices = resolveDevices(rv.cli.CderunDevices, rv.cli.Devices, rv.subcommand, rv.tools, rv.global, rv.r, rv.fs)
	if errDevices != nil {
		return errDevices
	}

	// Resolve memory via registry
	if opt, ok := GetStringOption("memory"); ok {
		if err := rv.applyMemoryOption(opt, &rv.res.Memory); err != nil {
			return err
		}
	}

	for _, opt := range Float64Options {
		if err := rv.applyFloat64Option(opt); err != nil {
			return err
		}
	}

	var hostCtx *HostContext
	if rv.global != nil {
		hostCtx = rv.global.HostContext
	}
	rv.res.HostContext = hostCtx

	if err := rv.r.Error(); err != nil {
		return err
	}
	return nil
}

func (rv *resolver) validateSecurity() error {
	if err := rv.validateCriticalFields(); err != nil {
		return err
	}
	if err := rv.validateSlices(); err != nil {
		return err
	}
	if err := rv.validateEnvSecurity(); err != nil {
		return err
	}
	if err := rv.validateMountSecurity(); err != nil {
		return err
	}
	if err := rv.validateDeviceSecurity(); err != nil {
		return err
	}
	if rv.res.Privileged {
		if logging.Enabled(logging.WarnLevel) {
			logging.Warn("Container is running in privileged mode. This reduces container isolation and may pose security risks.")
		}
	}
	return nil
}

func (rv *resolver) validateCriticalFields() error {
	criticalFields := []struct {
		name      string
		value     string
		validator func(string) error
	}{
		{"image", rv.res.Image, ValidateImageName},
		{"user", rv.res.User, ValidateUserName},
		{"network", rv.res.Network, ValidateNetworkName},
		{"hostname", rv.res.Hostname, ValidateHostname},
		{"workdir", rv.res.Workdir, ValidateWorkdir},
		{"runtime", rv.res.Runtime, func(s string) error {
			if s != "docker" && s != "podman" && s != "containerd" {
				return fmt.Errorf("unsupported runtime: %q", s)
			}
			return nil
		}},
		{"socket-path", rv.res.SocketPath, nil},
		{"mount-socket-path", rv.res.MountSocketPath, nil},
		{"mount-cderun-path", rv.res.MountCderunPath, nil},
		{"dry-run-format", rv.res.DryRunFormat, func(s string) error {
			if s != "" && s != "yaml" && s != "json" && s != "simple" {
				return fmt.Errorf("unsupported dry-run format: %q", s)
			}
			return nil
		}},
		{"diagnosis-format", rv.res.DiagnosisFormat, func(s string) error {
			if s != "" && s != "yaml" && s != "json" && s != "simple" {
				return fmt.Errorf("unsupported diagnosis format: %q", s)
			}
			return nil
		}},
		{"log-level", rv.res.LogLevel, func(s string) error {
			if s != "" {
				l := strings.ToLower(s)
				if l != "error" && l != "warn" && l != "warning" && l != "info" && l != "debug" && l != "trace" {
					return fmt.Errorf("unsupported log level: %q", s)
				}
			}
			return nil
		}},
		{"log-format", rv.res.LogFormat, func(s string) error {
			if s != "" && s != "text" && s != "json" {
				return fmt.Errorf("unsupported log format: %q", s)
			}
			return nil
		}},
	}
	for _, f := range criticalFields {
		if err := validatePathChars(f.value); err != nil {
			return fmt.Errorf("security validation failed for %q: %w", f.name, err)
		}
		if f.validator != nil {
			if err := f.validator(f.value); err != nil {
				return fmt.Errorf("security validation failed for %q: %w", f.name, err)
			}
		}
	}
	return nil
}

func (rv *resolver) validateSlices() error {
	criticalSlices := []struct {
		name      string
		slice     []string
		validator func(string) error
	}{
		{"entrypoint", rv.res.Entrypoint, nil},
		{"ports", rv.res.Ports, ValidatePort},
		{"expose", rv.res.Expose, ValidateExposePort},
		{"dns", rv.res.DNS, ValidateDNS},
		{"add-hosts", rv.res.AddHosts, ValidateAddHost},
		{"cap-add", rv.res.CapAdd, ValidateCapability},
		{"cap-drop", rv.res.CapDrop, ValidateCapability},
		{"sensitive-env", rv.res.SensitiveEnv, func(s string) error {
			_, err := path.Match(s, "TEST")
			if err != nil {
				return fmt.Errorf("invalid glob pattern: %w", err)
			}
			return nil
		}},
	}
	for _, s := range criticalSlices {
		for i, e := range s.slice {
			if err := validatePathChars(e); err != nil {
				return fmt.Errorf("security validation failed for %s[%d]: %w", s.name, i, err)
			}
			if s.validator != nil {
				if err := s.validator(e); err != nil {
					return fmt.Errorf("security validation failed for %s[%d]: %w", s.name, i, err)
				}
			}
		}
	}
	return nil
}

func (rv *resolver) validateEnvSecurity() error {
	for i, e := range rv.res.Env {
		key, _, _ := strings.Cut(e, "=")
		if err := validatePathChars(key); err != nil {
			return fmt.Errorf("security validation failed for env[%d] (key): %w", i, err)
		}
		if err := ValidateEnvKey(key); err != nil {
			return fmt.Errorf("security validation failed for env[%d] (key): %w", i, err)
		}
	}
	return nil
}

func (rv *resolver) validateMountSecurity() error {
	for i, m := range rv.res.Mounts {
		if err := validatePathChars(m.Source); err != nil {
			return fmt.Errorf("security validation failed for mounts[%d] (source): %w", i, err)
		}
		if err := validatePathChars(m.Target); err != nil {
			return fmt.Errorf("security validation failed for mounts[%d] (target): %w", i, err)
		}
	}
	return nil
}

func (rv *resolver) validateDeviceSecurity() error {
	for i, d := range rv.res.Devices {
		if err := validatePathChars(d.PathOnHost); err != nil {
			return fmt.Errorf("security validation failed for devices[%d] (path-on-host): %w", i, err)
		}
		if err := validatePathChars(d.PathInContainer); err != nil {
			return fmt.Errorf("security validation failed for devices[%d] (path-in-container): %w", i, err)
		}
	}
	return nil
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
