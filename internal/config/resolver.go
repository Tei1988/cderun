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
	Image                    *string
	TTY                      *bool
	Interactive              *bool
	Network                  *string
	Remove                   *bool
	CderunTTY                *bool
	CderunInteractive        *bool
	CderunImage              *string
	CderunNetwork            *string
	CderunRemove             *bool
	Runtime                  *string
	CderunRuntime            *string
	SocketPath               *string
	CderunSocketPath         *string
	MountSocket              *bool
	CderunMountSocket        *bool
	MountSocketPath          *string
	CderunMountSocketPath    *string
	Env                      []string
	CderunEnv                []string
	Workdir                  *string
	CderunWorkdir            *string
	Mounts                   []string
	CderunMounts             []string
	MountCderun              *bool
	CderunMountCderun        *bool
	MountCderunPath          *string
	CderunMountCderunPath    *string
	MountTools               *string
	CderunMountTools         *string
	MountAllTools            *bool
	CderunMountAllTools      *bool
	DryRun                   *bool
	CderunDryRun             *bool
	DryRunFormat             *string
	CderunDryRunFormat       *string
	Diagnosis                *bool
	CderunDiagnosis          *bool
	DiagnosisFormat          *string
	CderunDiagnosisFormat    *string
	LogLevel                 *string
	LogFormat                *string
	LogTimestamp             *bool
	StrictEnv                *bool
	CderunStrictEnv          *bool
	CderunLogLevel           *string
	CderunLogFormat          *string
	CderunLogTimestamp       *bool
	HangTimeout              *string
	CderunHangTimeout        *string

	// Docker-compatible flags
	Ports                []string
	CderunPorts          []string
	PublishAll           *bool
	CderunPublishAll     *bool
	Expose               []string
	CderunExpose         []string
	Hostname             *string
	CderunHostname       *string
	DNS                  []string
	CderunDNS                []string
	AddHosts             []string
	CderunAddHosts       []string
	User                 *string
	CderunUser           *string
	Privileged           *bool
	CderunPrivileged     *bool
	CapAdd               []string
	CderunCapAdd         []string
	CapDrop              []string
	CderunCapDrop        []string
	Entrypoint           []string
	CderunEntrypoint     []string
	Pull                 *string
	CderunPull               *string
	PullMaxRetries       *int
	CderunPullMaxRetries *int
	PullBackoffBase      *string
	CderunPullBackoffBase *string
	Memory               *string
	CderunMemory         *string
	CPUs                 *float64
	CderunCPUs           *float64
	Devices              []string
	CderunDevices        []string
	SensitiveEnv         []string
	CderunSensitiveEnv   []string
}

// Resolve combines CLI flags, environment variables, tool-specific config, and global defaults.
func Resolve(subcommand string, cli *CLIOptions, tools ToolsConfig, global *CDERunConfig) (*ResolvedConfig, error) {
	return ResolveWithFS(subcommand, cli, tools, global, RealFileSystem{})
}


var (
	cliType   = reflect.TypeFor[CLIOptions]()
	resType   = reflect.TypeFor[ResolvedConfig]()
	fieldInfo map[string]optionFields
	fieldOnce sync.Once
)

type optionFields struct {
	targetIdx int
	p1Idx     int
	p2Idx     int
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
			p1Idx:     -1,
			p2Idx:     -1,
		}

		if f, ok := cliType.FieldByName("Cderun" + fieldName); ok {
			info.p1Idx = f.Index[0]
		}
		if f, ok := cliType.FieldByName(fieldName); ok {
			info.p2Idx = f.Index[0]
		}

		if info.p1Idx != -1 && info.p2Idx != -1 {
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

func getFieldInfo(val reflect.Value, idx int) (bool, reflect.Value) {
	v := val.Field(idx)
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

	if info.p1Idx == -1 || info.p2Idx == -1 {
		return optionFields{}, false, reflect.Value{}, false, reflect.Value{}, fmt.Errorf("registry mismatch: CLI reflection fields for option %q missing in CLIOptions", key)
	}

	p1Set, p1Val := getFieldInfo(cliVal, info.p1Idx)
	p2Set, p2Val := getFieldInfo(cliVal, info.p2Idx)
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
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return 0, false
		}
		v = v.Elem()
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
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return 0.0, false
		}
		v = v.Elem()
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
	_, p1Set, p1v, p2Set, p2v, err := fetchFieldAndParams(name, rv.cliVal)
	if err != nil {
		return "", err
	}

	var p1Val, p2Val string
	if p1Set {
		p1Val = *p1v.Interface().(*string)
	}
	if p2Set {
		p2Val = *p2v.Interface().(*string)
	}

	var tools ToolsConfig
	var subcommand string
	if tGetter != nil {
		tools = rv.tools
		subcommand = rv.subcommand
	}

	return resolveConfigPath(
		p1Set, p1Val,
		p2Set, p2Val,
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
	info, ok := fieldInfo[opt.Name]
	if !ok {
		return fmt.Errorf("registry mismatch: info for option %q not found", opt.Name)
	}

	var p1Set, p2Set bool
	var p1Val, p2Val string
	var fastPathUsed bool

	// Fast-path for common options to avoid reflection and redundant map lookups
	switch opt.Name {
	case "image":
		if rv.cli.CderunImage != nil {
			p1Set, p1Val = true, *rv.cli.CderunImage
		}
		if rv.cli.Image != nil {
			p2Set, p2Val = true, *rv.cli.Image
		}
		fastPathUsed = true
	case "network":
		if rv.cli.CderunNetwork != nil {
			p1Set, p1Val = true, *rv.cli.CderunNetwork
		}
		if rv.cli.Network != nil {
			p2Set, p2Val = true, *rv.cli.Network
		}
		fastPathUsed = true
	case "workdir":
		if rv.cli.CderunWorkdir != nil {
			p1Set, p1Val = true, *rv.cli.CderunWorkdir
		}
		if rv.cli.Workdir != nil {
			p2Set, p2Val = true, *rv.cli.Workdir
		}
		fastPathUsed = true
	case "runtime":
		if rv.cli.CderunRuntime != nil {
			p1Set, p1Val = true, *rv.cli.CderunRuntime
		}
		if rv.cli.Runtime != nil {
			p2Set, p2Val = true, *rv.cli.Runtime
		}
		fastPathUsed = true
	case "user":
		if rv.cli.CderunUser != nil {
			p1Set, p1Val = true, *rv.cli.CderunUser
		}
		if rv.cli.User != nil {
			p2Set, p2Val = true, *rv.cli.User
		}
		fastPathUsed = true
	case "log-level":
		if rv.cli.CderunLogLevel != nil {
			p1Set, p1Val = true, *rv.cli.CderunLogLevel
		}
		if rv.cli.LogLevel != nil {
			p2Set, p2Val = true, *rv.cli.LogLevel
		}
		fastPathUsed = true
	case "log-format":
		if rv.cli.CderunLogFormat != nil {
			p1Set, p1Val = true, *rv.cli.CderunLogFormat
		}
		if rv.cli.LogFormat != nil {
			p2Set, p2Val = true, *rv.cli.LogFormat
		}
		fastPathUsed = true
	case "hostname":
		if rv.cli.CderunHostname != nil {
			p1Set, p1Val = true, *rv.cli.CderunHostname
		}
		if rv.cli.Hostname != nil {
			p2Set, p2Val = true, *rv.cli.Hostname
		}
		fastPathUsed = true
	case "pull":
		if rv.cli.CderunPull != nil {
			p1Set, p1Val = true, *rv.cli.CderunPull
		}
		if rv.cli.Pull != nil {
			p2Set, p2Val = true, *rv.cli.Pull
		}
		fastPathUsed = true
	case "dry-run-format":
		if rv.cli.CderunDryRunFormat != nil {
			p1Set, p1Val = true, *rv.cli.CderunDryRunFormat
		}
		if rv.cli.DryRunFormat != nil {
			p2Set, p2Val = true, *rv.cli.DryRunFormat
		}
		fastPathUsed = true
	case "diagnosis-format":
		if rv.cli.CderunDiagnosisFormat != nil {
			p1Set, p1Val = true, *rv.cli.CderunDiagnosisFormat
		}
		if rv.cli.DiagnosisFormat != nil {
			p2Set, p2Val = true, *rv.cli.DiagnosisFormat
		}
		fastPathUsed = true
	}

	if fastPathUsed {
		def := OptionDef[string]{
			EnvKey:       opt.EnvKey,
			ToolGetter:   opt.ToolGetter,
			GlobalGetter: opt.GlobalGetter,
			Fallback:     opt.Default,
		}
		resolved := resolveStringOpt(def, p1Set, p1Val, p2Set, p2Val, rv.subcommand, rv.tools, rv.global, rv.r, rv.fs)
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
		}
		return nil
	}

	s1, p1v := getFieldInfo(rv.cliVal, info.p1Idx)
	s2, p2v := getFieldInfo(rv.cliVal, info.p2Idx)
	if s1 {
		p1Val = *p1v.Interface().(*string)
	}
	if s2 {
		p2Val = *p2v.Interface().(*string)
	}
	def := OptionDef[string]{EnvKey: opt.EnvKey, ToolGetter: opt.ToolGetter, GlobalGetter: opt.GlobalGetter, Fallback: opt.Default}
	resolved := resolveStringOpt(def, s1, p1Val, s2, p2Val, rv.subcommand, rv.tools, rv.global, rv.r, rv.fs)
	rv.resVal.Field(info.targetIdx).SetString(resolved)
	return nil
}

func (rv *resolver) applyBoolOption(opt BoolOption) error {
	info, ok := fieldInfo[opt.Name]
	if !ok {
		return fmt.Errorf("registry mismatch: info for option %q not found", opt.Name)
	}

	var p1Set, p2Set bool
	var p1Val, p2Val bool
	var fastPathUsed bool

	// Fast-path for common options to avoid reflection and redundant map lookups
	switch opt.Name {
	case "tty":
		if rv.cli.CderunTTY != nil {
			p1Set, p1Val = true, *rv.cli.CderunTTY
		}
		if rv.cli.TTY != nil {
			p2Set, p2Val = true, *rv.cli.TTY
		}
		fastPathUsed = true
	case "interactive":
		if rv.cli.CderunInteractive != nil {
			p1Set, p1Val = true, *rv.cli.CderunInteractive
		}
		if rv.cli.Interactive != nil {
			p2Set, p2Val = true, *rv.cli.Interactive
		}
		fastPathUsed = true
	case "remove":
		if rv.cli.CderunRemove != nil {
			p1Set, p1Val = true, *rv.cli.CderunRemove
		}
		if rv.cli.Remove != nil {
			p2Set, p2Val = true, *rv.cli.Remove
		}
		fastPathUsed = true
	case "diagnosis":
		if rv.cli.CderunDiagnosis != nil {
			p1Set, p1Val = true, *rv.cli.CderunDiagnosis
		}
		if rv.cli.Diagnosis != nil {
			p2Set, p2Val = true, *rv.cli.Diagnosis
		}
		fastPathUsed = true
	case "strict-env":
		if rv.cli.CderunStrictEnv != nil {
			p1Set, p1Val = true, *rv.cli.CderunStrictEnv
		}
		if rv.cli.StrictEnv != nil {
			p2Set, p2Val = true, *rv.cli.StrictEnv
		}
		fastPathUsed = true
	case "privileged":
		if rv.cli.CderunPrivileged != nil {
			p1Set, p1Val = true, *rv.cli.CderunPrivileged
		}
		if rv.cli.Privileged != nil {
			p2Set, p2Val = true, *rv.cli.Privileged
		}
		fastPathUsed = true
	case "publish-all":
		if rv.cli.CderunPublishAll != nil {
			p1Set, p1Val = true, *rv.cli.CderunPublishAll
		}
		if rv.cli.PublishAll != nil {
			p2Set, p2Val = true, *rv.cli.PublishAll
		}
		fastPathUsed = true
	case "log-timestamp":
		if rv.cli.CderunLogTimestamp != nil {
			p1Set, p1Val = true, *rv.cli.CderunLogTimestamp
		}
		if rv.cli.LogTimestamp != nil {
			p2Set, p2Val = true, *rv.cli.LogTimestamp
		}
		fastPathUsed = true
	case "mount-socket":
		if rv.cli.CderunMountSocket != nil {
			p1Set, p1Val = true, *rv.cli.CderunMountSocket
		}
		if rv.cli.MountSocket != nil {
			p2Set, p2Val = true, *rv.cli.MountSocket
		}
		fastPathUsed = true
	case "mount-cderun":
		if rv.cli.CderunMountCderun != nil {
			p1Set, p1Val = true, *rv.cli.CderunMountCderun
		}
		if rv.cli.MountCderun != nil {
			p2Set, p2Val = true, *rv.cli.MountCderun
		}
		fastPathUsed = true
	case "mount-all-tools":
		if rv.cli.CderunMountAllTools != nil {
			p1Set, p1Val = true, *rv.cli.CderunMountAllTools
		}
		if rv.cli.MountAllTools != nil {
			p2Set, p2Val = true, *rv.cli.MountAllTools
		}
		fastPathUsed = true
	case "dry-run":
		if rv.cli.CderunDryRun != nil {
			p1Set, p1Val = true, *rv.cli.CderunDryRun
		}
		if rv.cli.DryRun != nil {
			p2Set, p2Val = true, *rv.cli.DryRun
		}
		fastPathUsed = true
	}

	if fastPathUsed {
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

	s1, p1v := getFieldInfo(rv.cliVal, info.p1Idx)
	s2, p2v := getFieldInfo(rv.cliVal, info.p2Idx)
	if s1 {
		p1Val = *p1v.Interface().(*bool)
	}
	if s2 {
		p2Val = *p2v.Interface().(*bool)
	}
	def := OptionDef[*bool]{EnvKey: opt.EnvKey, ToolGetter: opt.ToolGetter, GlobalGetter: opt.GlobalGetter}
	resolved := resolveBoolOpt(def, opt.Default, s1, p1Val, s2, p2Val, rv.subcommand, rv.tools, rv.global, rv.fs)
	rv.resVal.Field(info.targetIdx).SetBool(resolved)
	return nil
}

func (rv *resolver) applyIntOption(opt IntOption) error {
	info, ok := fieldInfo[opt.Name]
	if !ok {
		return fmt.Errorf("registry mismatch: info for option %q not found", opt.Name)
	}

	var p1Set, p2Set bool
	var p1Int, p2Int int
	var fastPathUsed bool

	if opt.Name == "pull-max-retries" {
		if rv.cli.CderunPullMaxRetries != nil {
			p1Set, p1Int = true, *rv.cli.CderunPullMaxRetries
		}
		if rv.cli.PullMaxRetries != nil {
			p2Set, p2Int = true, *rv.cli.PullMaxRetries
		}
		fastPathUsed = true
	}

	if fastPathUsed {
		def := OptionDef[*int]{
			EnvKey:       opt.EnvKey,
			ToolGetter:   opt.ToolGetter,
			GlobalGetter: opt.GlobalGetter,
			Fallback:     &opt.Default,
		}
		resolved := resolveIntOpt(def, p1Set, p1Int, p2Set, p2Int, rv.subcommand, rv.tools, rv.global, rv.fs)
		switch opt.Name {
		case "pull-max-retries":
			rv.res.PullMaxRetries = resolved
		}
		return nil
	}

	s1, p1v := getFieldInfo(rv.cliVal, info.p1Idx)
	s2, p2v := getFieldInfo(rv.cliVal, info.p2Idx)
	p1Int, p1Set = rv.extractIntValue(p1v, s1)
	p2Int, p2Set = rv.extractIntValue(p2v, s2)

	def := OptionDef[*int]{
		EnvKey:       opt.EnvKey,
		ToolGetter:   opt.ToolGetter,
		GlobalGetter: opt.GlobalGetter,
		Fallback:     &opt.Default,
	}

	resolved := resolveIntOpt(def, p1Set, p1Int, p2Set, p2Int, rv.subcommand, rv.tools, rv.global, rv.fs)
	rv.resVal.Field(info.targetIdx).SetInt(int64(resolved))
	return nil
}

func (rv *resolver) applyFloat64Option(opt Float64Option) error {
	info, ok := fieldInfo[opt.Name]
	if !ok {
		return fmt.Errorf("registry mismatch: info for option %q not found", opt.Name)
	}

	var p1Set, p2Set bool
	var p1Float, p2Float float64
	var fastPathUsed bool

	if opt.Name == "cpus" {
		if rv.cli.CderunCPUs != nil {
			p1Set, p1Float = true, *rv.cli.CderunCPUs
		}
		if rv.cli.CPUs != nil {
			p2Set, p2Float = true, *rv.cli.CPUs
		}
		fastPathUsed = true
	}

	if fastPathUsed {
		def := OptionDef[*float64]{
			EnvKey:       opt.EnvKey,
			ToolGetter:   opt.ToolGetter,
			GlobalGetter: opt.GlobalGetter,
			Fallback:     &opt.Default,
		}
		resolved := resolveFloat64Opt(def, p1Set, p1Float, p2Set, p2Float, rv.subcommand, rv.tools, rv.global, rv.fs)
		switch opt.Name {
		case "cpus":
			rv.res.CPUs = resolved
		}
		return nil
	}

	s1, p1v := getFieldInfo(rv.cliVal, info.p1Idx)
	s2, p2v := getFieldInfo(rv.cliVal, info.p2Idx)
	p1Float, p1Set = rv.extractFloatValue(p1v, s1)
	p2Float, p2Set = rv.extractFloatValue(p2v, s2)

	def := OptionDef[*float64]{
		EnvKey:       opt.EnvKey,
		ToolGetter:   opt.ToolGetter,
		GlobalGetter: opt.GlobalGetter,
		Fallback:     &opt.Default,
	}

	resolved := resolveFloat64Opt(def, p1Set, p1Float, p2Set, p2Float, rv.subcommand, rv.tools, rv.global, rv.fs)
	rv.resVal.Field(info.targetIdx).SetFloat(resolved)
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
		if rv.cli.CderunHangTimeout != nil {
			p1Set, p1Val = true, *rv.cli.CderunHangTimeout
		}
		if rv.cli.HangTimeout != nil {
			p2Set, p2Val = true, *rv.cli.HangTimeout
		}
	case "pull-backoff-base":
		if rv.cli.CderunPullBackoffBase != nil {
			p1Set, p1Val = true, *rv.cli.CderunPullBackoffBase
		}
		if rv.cli.PullBackoffBase != nil {
			p2Set, p2Val = true, *rv.cli.PullBackoffBase
		}
	default:
		_, s1, v1, s2, v2, err := fetchFieldAndParams(opt.Name, rv.cliVal)
		if err != nil {
			return err
		}
		if s1 {
			p1Set, p1Val = true, *v1.Interface().(*string)
		}
		if s2 {
			p2Set, p2Val = true, *v2.Interface().(*string)
		}
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
		if rv.cli.CderunMemory != nil {
			p1Set, p1Val = true, *rv.cli.CderunMemory
		}
		if rv.cli.Memory != nil {
			p2Set, p2Val = true, *rv.cli.Memory
		}
	default:
		_, s1, v1, s2, v2, err := fetchFieldAndParams(opt.Name, rv.cliVal)
		if err != nil {
			return err
		}
		if s1 {
			p1Set, p1Val = true, *v1.Interface().(*string)
		}
		if s2 {
			p2Set, p2Val = true, *v2.Interface().(*string)
		}
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

// ResolveWithFS combines CLI flags, environment variables, tool-specific config, and global defaults using the provided filesystem.
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
	// We call applyBoolOption directly which uses fast-paths and fieldInfo.
	if opt, ok := GetBoolOption("diagnosis"); ok {
		if err := rv.applyBoolOption(opt); err != nil {
			return err
		}
	} else {
		return fmt.Errorf("registry mismatch: early boolean option \"diagnosis\" not found")
	}

	if opt, ok := GetBoolOption("strict-env"); ok {
		if err := rv.applyBoolOption(opt); err != nil {
			return err
		}
	} else {
		return fmt.Errorf("registry mismatch: early boolean option \"strict-env\" not found")
	}

	// Sensitive Env Resolution (needed for masking in debug logs during further resolution)
	{
		opt, ok := GetStringSliceOption("sensitive-env")
		if !ok {
			return fmt.Errorf("registry mismatch: early string slice option %q not found", "sensitive-env")
		}
		info, ok := fieldInfo["sensitive-env"]
		if !ok {
			return fmt.Errorf("registry mismatch: info for option \"sensitive-env\" not found")
		}

		_, p1v := getFieldInfo(rv.cliVal, info.p1Idx)
		_, p2v := getFieldInfo(rv.cliVal, info.p2Idx)

		def := OptionDef[[]string]{
			EnvKey:       opt.EnvKey,
			ToolGetter:   opt.ToolGetter,
			GlobalGetter: opt.GlobalGetter,
		}

		rv.res.SensitiveEnv = resolveStringSliceOpt(def, ",", p1v.Interface().([]string), p2v.Interface().([]string), rv.subcommand, rv.tools, rv.global, rv.r, rv.fs)
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
				if rv.cli.CderunImage != nil {
					cliImage = *rv.cli.CderunImage
				} else if rv.cli.Image != nil {
					cliImage = *rv.cli.Image
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
	var p1s, p2s bool
	var p1v, p2v string
	if rv.cli.CderunMountTools != nil {
		p1s, p1v = true, *rv.cli.CderunMountTools
	}
	if rv.cli.MountTools != nil {
		p2s, p2v = true, *rv.cli.MountTools
	}
	rv.res.MountTools = resolveStringSliceCommaOpt(
		OptionDef[[]string]{EnvKey: "CDERUN_MOUNT_TOOLS",
			ToolGetter:   func(t ToolConfig) []string { return t.MountTools },
			GlobalGetter: func(g CDERunConfig) []string { return g.Defaults.MountTools }},
		p1s, p1v,
		p2s, p2v,
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
		var p1Set, p2Set bool
		var p1Val, p2Val bool
		if rv.cli.CderunMountAllTools != nil {
			p1Set, p1Val = true, *rv.cli.CderunMountAllTools
		}
		if rv.cli.MountAllTools != nil {
			p2Set, p2Val = true, *rv.cli.MountAllTools
		}
		def := OptionDef[*bool]{EnvKey: opt.EnvKey, ToolGetter: opt.ToolGetter, GlobalGetter: opt.GlobalGetter}
		rv.res.MountAllTools = resolveBoolOpt(def, opt.Default, p1Set, p1Val, p2Set, p2Val, rv.subcommand, rv.tools, rv.global, rv.fs)
	}

	var mountCderunSpecified bool
	{
		opt, _ := GetBoolOption("mount-cderun")
		var p1Set, p2Set bool
		var p1Val, p2Val bool
		if rv.cli.CderunMountCderun != nil {
			p1Set, p1Val = true, *rv.cli.CderunMountCderun
		}
		if rv.cli.MountCderun != nil {
			p2Set, p2Val = true, *rv.cli.MountCderun
		}
		def := OptionDef[*bool]{EnvKey: opt.EnvKey, ToolGetter: opt.ToolGetter, GlobalGetter: opt.GlobalGetter}
		rv.res.MountCderun, mountCderunSpecified = resolveBoolOptInfo(def, p1Set, p1Val, p2Set, p2Val, rv.subcommand, rv.tools, rv.global, rv.fs)
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
		var p1Set, p2Set bool
		var p1Val, p2Val bool
		if rv.cli.CderunMountSocket != nil {
			p1Set, p1Val = true, *rv.cli.CderunMountSocket
		}
		if rv.cli.MountSocket != nil {
			p2Set, p2Val = true, *rv.cli.MountSocket
		}
		def := OptionDef[*bool]{EnvKey: opt.EnvKey, ToolGetter: opt.ToolGetter, GlobalGetter: opt.GlobalGetter}
		rv.res.MountSocket, mountSocketSpecified = resolveBoolOptInfo(def, p1Set, p1Val, p2Set, p2Val, rv.subcommand, rv.tools, rv.global, rv.fs)
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
