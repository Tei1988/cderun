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
	ReadOnly        bool
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
	Pid             string
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
	GroupAdd        []string
}

// CLIOptions represents values from CLI flags.
type CLIOptions struct {
	Image                    *string
	TTY                      *bool
	Interactive              *bool
	Network                  *string
	Remove                   *bool
	ReadOnly                 *bool
	CderunTTY                *bool
	CderunInteractive        *bool
	CderunImage              *string
	CderunNetwork            *string
	CderunRemove             *bool
	CderunReadOnly           *bool
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
	Ports                    []string
	CderunPorts              []string
	PublishAll               *bool
	CderunPublishAll         *bool
	Expose                   []string
	CderunExpose             []string
	Hostname                 *string
	CderunHostname           *string
	DNS                      []string
	CderunDNS                []string
	AddHosts                 []string
	CderunAddHosts           []string
	User                     *string
	CderunUser               *string
	Privileged               *bool
	CderunPrivileged         *bool
	Pid                      *string
	CderunPid                *string
	CapAdd                   []string
	CderunCapAdd             []string
	CapDrop                  []string
	CderunCapDrop            []string
	Entrypoint               []string
	CderunEntrypoint         []string
	Pull                     *string
	CderunPull               *string
	PullMaxRetries           *int
	CderunPullMaxRetries     *int
	PullBackoffBase          *string
	CderunPullBackoffBase    *string
	Memory                   *string
	CderunMemory             *string
	CPUs                     *float64
	CderunCPUs               *float64
	Devices                  []string
	CderunDevices            []string
	SensitiveEnv             []string
	CderunSensitiveEnv       []string
	GroupAdd                 []string
	CderunGroupAdd           []string
}

// Resolve combines CLI flags, environment variables, tool-specific config, and global defaults.
func Resolve(subcommand string, cli *CLIOptions, tools ToolsConfig, global *CDERunConfig) (*ResolvedConfig, error) {
	return ResolveWithFS(subcommand, cli, tools, global, RealFileSystem{})
}

func getPtrVal[T any](p *T) (bool, T) {
	if p == nil {
		var zero T
		return false, zero
	}
	return true, *p
}

var (
	cliType              = reflect.TypeFor[CLIOptions]()
	resType              = reflect.TypeFor[ResolvedConfig]()
	fieldInfo            map[string]optionFields
	expectedFieldIndices map[string]optionFields
	fieldOnce            sync.Once

	autoDetectMu           sync.RWMutex
	autoDetectedRuntime    string
	autoDetectedSocketPath string
)

type optionFields struct {
	targetIdx int
	p1ValIdx  int
	p2ValIdx  int
}

func initFieldInfo() {
	fieldInfo = make(map[string]optionFields)
	expectedFieldIndices = make(map[string]optionFields)

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
			p1ValIdx:  -1,
			p2ValIdx:  -1,
		}

		p1ValName := "Cderun" + fieldName
		p2ValName := fieldName

		if f, ok := cliType.FieldByName(p1ValName); ok {
			info.p1ValIdx = f.Index[0]
		}
		if f, ok := cliType.FieldByName(p2ValName); ok {
			info.p2ValIdx = f.Index[0]
		}

		if info.p1ValIdx != -1 && info.p2ValIdx != -1 {
			fieldInfo[name] = info
			expectedFieldIndices[name] = info
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

func getFieldInfo(val reflect.Value, valIdx int) (bool, reflect.Value) {
	v := val.Field(valIdx)
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return false, reflect.Zero(v.Type().Elem())
		}
		return true, v.Elem()
	}
	if v.Kind() == reflect.Slice || v.Kind() == reflect.Interface || v.Kind() == reflect.Map {
		return !v.IsNil(), v
	}
	return !v.IsZero(), v
}

func fetchFieldAndParams(key string, cliVal reflect.Value) (optionFields, bool, reflect.Value, bool, reflect.Value, error) {
	info := fieldInfo[key]
	p1Set, p1Val := getFieldInfo(cliVal, info.p1ValIdx)
	p2Set, p2Val := getFieldInfo(cliVal, info.p2ValIdx)
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

func (rv *resolver) getCliVal() reflect.Value {
	if !rv.cliVal.IsValid() {
		rv.cliVal = reflect.ValueOf(rv.cli).Elem()
	}
	return rv.cliVal
}

func (rv *resolver) getResVal() reflect.Value {
	if !rv.resVal.IsValid() {
		rv.resVal = reflect.ValueOf(rv.res).Elem()
	}
	return rv.resVal
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

func (rv *resolver) getR() (*ExpressionResolver, error) {
	if rv.r == nil {
		var hostCtx *HostContext
		if rv.global != nil {
			hostCtx = rv.global.HostContext
		}
		r, err := NewExpressionResolverWithFS(hostCtx, rv.fs)
		if err != nil {
			return nil, fmt.Errorf("failed to create expression resolver: %w", err)
		}
		rv.r = r
	}
	return rv.r, nil
}

func (rv *resolver) hasPathToResolve(p1Set bool, p1Val string, p2Set bool, p2Val string, envKey string, tGetter func(ToolConfig) ConfigPath, gGetter func(CDERunConfig) ConfigPath, fallback string) bool {
	if p1Set {
		return p1Val != ""
	}
	if p2Set {
		return p2Val != ""
	}
	if envKey != "" {
		if env := rv.fs.Getenv(envKey); env != "" {
			return true
		}
	}
	if rv.tools != nil && tGetter != nil {
		if tool, ok := rv.tools[rv.subcommand]; ok {
			if t := tGetter(tool); !t.IsEmpty() {
				return true
			}
		}
	}
	if rv.global != nil && gGetter != nil {
		if g := gGetter(*rv.global); !g.IsEmpty() {
			return true
		}
	}
	return fallback != ""
}

func (rv *resolver) resolvePathValue(name, envKey string, tGetter func(ToolConfig) ConfigPath, gGetter func(CDERunConfig) ConfigPath, fallback string) (string, error) {
	_, p1Set, p1Val, p2Set, p2Val, err := fetchFieldAndParams(name, rv.getCliVal())
	if err != nil {
		return "", err
	}

	p1ValStr := p1Val.String()
	p2ValStr := p2Val.String()

	if !rv.hasPathToResolve(p1Set, p1ValStr, p2Set, p2ValStr, envKey, tGetter, gGetter, fallback) {
		return "", nil
	}

	// Find the winning raw path
	var raw string
	if p1Set {
		raw = p1ValStr
	} else if p2Set {
		raw = p2ValStr
	} else if env := rv.fs.Getenv(envKey); env != "" {
		raw = env
	} else {
		found := false
		if rv.tools != nil && tGetter != nil {
			if tool, ok := rv.tools[rv.subcommand]; ok {
				if t := tGetter(tool); !t.IsEmpty() {
					raw = t.Raw
					found = true
				}
			}
		}
		if !found && rv.global != nil && gGetter != nil {
			if g := gGetter(*rv.global); !g.IsEmpty() {
				raw = g.Raw
				found = true
			}
		}
		if !found {
			raw = fallback
		}
	}

	var r *ExpressionResolver
	needResolver := strings.Contains(raw, "{{") || strings.HasPrefix(raw, "~")
	if !needResolver && rv.global != nil && rv.global.HostContext != nil && rv.global.HostContext.Level > 0 {
		needResolver = true
	}

	if needResolver {
		var err error
		r, err = rv.getR()
		if err != nil {
			return "", err
		}
	}

	var tools ToolsConfig
	var subcommand string
	if tGetter != nil {
		tools = rv.tools
		subcommand = rv.subcommand
	}

	return resolveConfigPath(
		p1Set, p1ValStr,
		p2Set, p2ValStr,
		envKey,
		subcommand, tools, tGetter,
		rv.global, gGetter,
		fallback,
		r,
		"path",
		rv.fs,
	)
}

func (rv *resolver) resolverForSlice(def OptionDef[[]string], envSep string, p1 []string, p2 []string) (*ExpressionResolver, error) {
	var vals []string
	if p1 != nil {
		vals = p1
	} else if p2 != nil {
		vals = p2
	} else if def.EnvKey != "" {
		if env, ok := rv.fs.LookupEnv(def.EnvKey); ok {
			vals = []string{}
			for v := range strings.SplitSeq(env, envSep) {
				v = strings.TrimSpace(v)
				if v == "" {
					continue
				}
				vals = append(vals, v)
			}
		}
	}
	if vals == nil && def.ToolGetter != nil && rv.tools != nil {
		if tool, ok := rv.tools[rv.subcommand]; ok {
			vals = def.ToolGetter(tool)
		}
	}
	if vals == nil && def.GlobalGetter != nil && rv.global != nil {
		vals = def.GlobalGetter(*rv.global)
	}

	for _, v := range vals {
		if strings.Contains(v, "{{") || strings.HasPrefix(v, "~") {
			return rv.getR()
		}
	}
	return nil, nil
}

func (rv *resolver) applyStringSliceOption(opt StringSliceOption) error {
	info, ok := fieldInfo[opt.Name]
	if !ok {
		return fmt.Errorf("registry mismatch: info for option %q not found", opt.Name)
	}

	var p1v, p2v []string
	var fastPathUsed bool

	expected := expectedFieldIndices[opt.Name]
	if info.p1ValIdx == expected.p1ValIdx && info.p2ValIdx == expected.p2ValIdx {
		switch opt.Name {
		case "publish":
			p1v, p2v = rv.cli.CderunPorts, rv.cli.Ports
			fastPathUsed = true
		case "expose":
			p1v, p2v = rv.cli.CderunExpose, rv.cli.Expose
			fastPathUsed = true
		case "dns":
			p1v, p2v = rv.cli.CderunDNS, rv.cli.DNS
			fastPathUsed = true
		case "add-host":
			p1v, p2v = rv.cli.CderunAddHosts, rv.cli.AddHosts
			fastPathUsed = true
		case "group-add":
			p1v, p2v = rv.cli.CderunGroupAdd, rv.cli.GroupAdd
			fastPathUsed = true
		case "cap-add":
			p1v, p2v = rv.cli.CderunCapAdd, rv.cli.CapAdd
			fastPathUsed = true
		case "cap-drop":
			p1v, p2v = rv.cli.CderunCapDrop, rv.cli.CapDrop
			fastPathUsed = true
		case "entrypoint":
			p1v, p2v = rv.cli.CderunEntrypoint, rv.cli.Entrypoint
			fastPathUsed = true
		}
	}

	def := OptionDef[[]string]{
		EnvKey:       opt.EnvKey,
		ToolGetter:   opt.ToolGetter,
		GlobalGetter: opt.GlobalGetter,
	}

	if fastPathUsed {
		rForSlice, err := rv.resolverForSlice(def, ",", p1v, p2v)
		if err != nil {
			return err
		}
		resolved := resolveStringSliceOpt(def, ",", p1v, p2v, rv.subcommand, rv.tools, rv.global, rForSlice, rv.fs)
		switch opt.Name {
		case "publish":
			rv.res.Ports = resolved
		case "expose":
			rv.res.Expose = resolved
		case "dns":
			rv.res.DNS = resolved
		case "add-host":
			rv.res.AddHosts = resolved
		case "group-add":
			rv.res.GroupAdd = resolved
		case "cap-add":
			rv.res.CapAdd = resolved
		case "cap-drop":
			rv.res.CapDrop = resolved
		case "entrypoint":
			rv.res.Entrypoint = resolved
		}
		return nil
	}

	_, p1Set, p1Val, p2Set, p2Val, err := fetchFieldAndParams(opt.Name, rv.getCliVal())
	if err != nil {
		return err
	}

	p1v, _ = rv.extractStringSliceValue(p1Val, p1Set)
	p2v, _ = rv.extractStringSliceValue(p2Val, p2Set)

	rForSlice, err := rv.resolverForSlice(def, ",", p1v, p2v)
	if err != nil {
		return err
	}

	resolved := resolveStringSliceOpt(def, ",", p1v, p2v, rv.subcommand, rv.tools, rv.global, rForSlice, rv.fs)
	rv.getResVal().Field(info.targetIdx).Set(reflect.ValueOf(resolved))
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
		p1Set, p1Val = getPtrVal(rv.cli.CderunImage)
		p2Set, p2Val = getPtrVal(rv.cli.Image)
		fastPathUsed = true
	case "pid":
		p1Set, p1Val = getPtrVal(rv.cli.CderunPid)
		p2Set, p2Val = getPtrVal(rv.cli.Pid)
		fastPathUsed = true
	case "network":
		p1Set, p1Val = getPtrVal(rv.cli.CderunNetwork)
		p2Set, p2Val = getPtrVal(rv.cli.Network)
		fastPathUsed = true
	case "workdir":
		p1Set, p1Val = getPtrVal(rv.cli.CderunWorkdir)
		p2Set, p2Val = getPtrVal(rv.cli.Workdir)
		fastPathUsed = true
	case "runtime":
		p1Set, p1Val = getPtrVal(rv.cli.CderunRuntime)
		p2Set, p2Val = getPtrVal(rv.cli.Runtime)
		fastPathUsed = true
	case "user":
		p1Set, p1Val = getPtrVal(rv.cli.CderunUser)
		p2Set, p2Val = getPtrVal(rv.cli.User)
		fastPathUsed = true
	case "log-level":
		p1Set, p1Val = getPtrVal(rv.cli.CderunLogLevel)
		p2Set, p2Val = getPtrVal(rv.cli.LogLevel)
		fastPathUsed = true
	case "log-format":
		p1Set, p1Val = getPtrVal(rv.cli.CderunLogFormat)
		p2Set, p2Val = getPtrVal(rv.cli.LogFormat)
		fastPathUsed = true
	case "hostname":
		p1Set, p1Val = getPtrVal(rv.cli.CderunHostname)
		p2Set, p2Val = getPtrVal(rv.cli.Hostname)
		fastPathUsed = true
	case "pull":
		p1Set, p1Val = getPtrVal(rv.cli.CderunPull)
		p2Set, p2Val = getPtrVal(rv.cli.Pull)
		fastPathUsed = true
	case "dry-run-format":
		p1Set, p1Val = getPtrVal(rv.cli.CderunDryRunFormat)
		p2Set, p2Val = getPtrVal(rv.cli.DryRunFormat)
		fastPathUsed = true
	case "diagnosis-format":
		p1Set, p1Val = getPtrVal(rv.cli.CderunDiagnosisFormat)
		p2Set, p2Val = getPtrVal(rv.cli.DiagnosisFormat)
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
		if strings.Contains(resolved, "{{") || strings.HasPrefix(resolved, "~") {
			r, err := rv.getR()
			if err != nil {
				return err
			}
			resolved = r.resolveString(resolved)
			if err := r.Error(); err != nil {
				return err
			}
		}
		switch opt.Name {
		case "image":
			rv.res.Image = resolved
		case "pid":
			rv.res.Pid = resolved
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

	s1, p1v := getFieldInfo(rv.getCliVal(), info.p1ValIdx)
	s2, p2v := getFieldInfo(rv.getCliVal(), info.p2ValIdx)
	p1Val, p2Val = p1v.String(), p2v.String()
	def := OptionDef[string]{EnvKey: opt.EnvKey, ToolGetter: opt.ToolGetter, GlobalGetter: opt.GlobalGetter, Fallback: opt.Default}
	resolved := resolveStringOpt(def, s1, p1Val, s2, p2Val, rv.subcommand, rv.tools, rv.global, rv.r, rv.fs)
	if strings.Contains(resolved, "{{") || strings.HasPrefix(resolved, "~") {
		r, err := rv.getR()
		if err != nil {
			return err
		}
		resolved = r.resolveString(resolved)
		if err := r.Error(); err != nil {
			return err
		}
	}
	rv.getResVal().Field(info.targetIdx).SetString(resolved)
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
		p1Set, p1Val = getPtrVal(rv.cli.CderunTTY)
		p2Set, p2Val = getPtrVal(rv.cli.TTY)
		fastPathUsed = true
	case "interactive":
		p1Set, p1Val = getPtrVal(rv.cli.CderunInteractive)
		p2Set, p2Val = getPtrVal(rv.cli.Interactive)
		fastPathUsed = true
	case "read-only":
		p1Set, p1Val = getPtrVal(rv.cli.CderunReadOnly)
		p2Set, p2Val = getPtrVal(rv.cli.ReadOnly)
		fastPathUsed = true
	case "remove":
		p1Set, p1Val = getPtrVal(rv.cli.CderunRemove)
		p2Set, p2Val = getPtrVal(rv.cli.Remove)
		fastPathUsed = true
	case "diagnosis":
		p1Set, p1Val = getPtrVal(rv.cli.CderunDiagnosis)
		p2Set, p2Val = getPtrVal(rv.cli.Diagnosis)
		fastPathUsed = true
	case "strict-env":
		p1Set, p1Val = getPtrVal(rv.cli.CderunStrictEnv)
		p2Set, p2Val = getPtrVal(rv.cli.StrictEnv)
		fastPathUsed = true
	case "privileged":
		p1Set, p1Val = getPtrVal(rv.cli.CderunPrivileged)
		p2Set, p2Val = getPtrVal(rv.cli.Privileged)
		fastPathUsed = true
	case "publish-all":
		p1Set, p1Val = getPtrVal(rv.cli.CderunPublishAll)
		p2Set, p2Val = getPtrVal(rv.cli.PublishAll)
		fastPathUsed = true
	case "log-timestamp":
		p1Set, p1Val = getPtrVal(rv.cli.CderunLogTimestamp)
		p2Set, p2Val = getPtrVal(rv.cli.LogTimestamp)
		fastPathUsed = true
	case "mount-socket":
		p1Set, p1Val = getPtrVal(rv.cli.CderunMountSocket)
		p2Set, p2Val = getPtrVal(rv.cli.MountSocket)
		fastPathUsed = true
	case "mount-cderun":
		p1Set, p1Val = getPtrVal(rv.cli.CderunMountCderun)
		p2Set, p2Val = getPtrVal(rv.cli.MountCderun)
		fastPathUsed = true
	case "mount-all-tools":
		p1Set, p1Val = getPtrVal(rv.cli.CderunMountAllTools)
		p2Set, p2Val = getPtrVal(rv.cli.MountAllTools)
		fastPathUsed = true
	case "dry-run":
		p1Set, p1Val = getPtrVal(rv.cli.CderunDryRun)
		p2Set, p2Val = getPtrVal(rv.cli.DryRun)
		fastPathUsed = true
	}

	if fastPathUsed {
		def := OptionDef[*bool]{
			EnvKey:       opt.EnvKey,
			ToolGetter:   opt.ToolGetter,
			GlobalGetter: opt.GlobalGetter,
		}
		resolved, err := resolveBoolOpt(def, opt.Default, p1Set, p1Val, p2Set, p2Val, rv.subcommand, rv.tools, rv.global, rv.fs)
		if err != nil {
			return err
		}
		switch opt.Name {
		case "tty":
			rv.res.TTY = resolved
		case "interactive":
			rv.res.Interactive = resolved
		case "read-only":
			rv.res.ReadOnly = resolved
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

	s1, p1v := getFieldInfo(rv.getCliVal(), info.p1ValIdx)
	s2, p2v := getFieldInfo(rv.getCliVal(), info.p2ValIdx)
	p1Val, p2Val = p1v.Bool(), p2v.Bool()
	def := OptionDef[*bool]{EnvKey: opt.EnvKey, ToolGetter: opt.ToolGetter, GlobalGetter: opt.GlobalGetter}
	resolved, err := resolveBoolOpt(def, opt.Default, s1, p1Val, s2, p2Val, rv.subcommand, rv.tools, rv.global, rv.fs)
	if err != nil {
		return err
	}
	rv.getResVal().Field(info.targetIdx).SetBool(resolved)
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
		expected := expectedFieldIndices["pull-max-retries"]
		if info.p1ValIdx == expected.p1ValIdx && info.p2ValIdx == expected.p2ValIdx {
			p1Set, p1Int = getPtrVal(rv.cli.CderunPullMaxRetries)
			p2Set, p2Int = getPtrVal(rv.cli.PullMaxRetries)
			fastPathUsed = true
		}
	}

	if fastPathUsed {
		def := OptionDef[*int]{
			EnvKey:       opt.EnvKey,
			ToolGetter:   opt.ToolGetter,
			GlobalGetter: opt.GlobalGetter,
		}
		resolved, err := resolveIntOpt(def, opt.Default, p1Set, p1Int, p2Set, p2Int, rv.subcommand, rv.tools, rv.global, rv.fs)
		if err != nil {
			return err
		}
		switch opt.Name {
		case "pull-max-retries":
			rv.res.PullMaxRetries = resolved
		}
		return nil
	}

	s1, p1v := getFieldInfo(rv.getCliVal(), info.p1ValIdx)
	s2, p2v := getFieldInfo(rv.getCliVal(), info.p2ValIdx)
	p1Int, p1Set = rv.extractIntValue(p1v, s1)
	p2Int, p2Set = rv.extractIntValue(p2v, s2)

	def := OptionDef[*int]{
		EnvKey:       opt.EnvKey,
		ToolGetter:   opt.ToolGetter,
		GlobalGetter: opt.GlobalGetter,
	}

	resolved, err := resolveIntOpt(def, opt.Default, p1Set, p1Int, p2Set, p2Int, rv.subcommand, rv.tools, rv.global, rv.fs)
	if err != nil {
		return err
	}
	rv.getResVal().Field(info.targetIdx).SetInt(int64(resolved))
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
		expected := expectedFieldIndices["cpus"]
		if info.p1ValIdx == expected.p1ValIdx && info.p2ValIdx == expected.p2ValIdx {
			p1Set, p1Float = getPtrVal(rv.cli.CderunCPUs)
			p2Set, p2Float = getPtrVal(rv.cli.CPUs)
			fastPathUsed = true
		}
	}

	if fastPathUsed {
		def := OptionDef[*float64]{
			EnvKey:       opt.EnvKey,
			ToolGetter:   opt.ToolGetter,
			GlobalGetter: opt.GlobalGetter,
		}
		resolved, err := resolveFloat64Opt(def, opt.Default, p1Set, p1Float, p2Set, p2Float, rv.subcommand, rv.tools, rv.global, rv.fs)
		if err != nil {
			return err
		}
		switch opt.Name {
		case "cpus":
			rv.res.CPUs = resolved
		}
		return nil
	}

	s1, p1v := getFieldInfo(rv.getCliVal(), info.p1ValIdx)
	s2, p2v := getFieldInfo(rv.getCliVal(), info.p2ValIdx)
	p1Float, p1Set = rv.extractFloatValue(p1v, s1)
	p2Float, p2Set = rv.extractFloatValue(p2v, s2)

	def := OptionDef[*float64]{
		EnvKey:       opt.EnvKey,
		ToolGetter:   opt.ToolGetter,
		GlobalGetter: opt.GlobalGetter,
	}

	resolved, err := resolveFloat64Opt(def, opt.Default, p1Set, p1Float, p2Set, p2Float, rv.subcommand, rv.tools, rv.global, rv.fs)
	if err != nil {
		return err
	}
	rv.getResVal().Field(info.targetIdx).SetFloat(resolved)
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
		p1Set, p1Val = getPtrVal(rv.cli.CderunHangTimeout)
		p2Set, p2Val = getPtrVal(rv.cli.HangTimeout)
	case "pull-backoff-base":
		p1Set, p1Val = getPtrVal(rv.cli.CderunPullBackoffBase)
		p2Set, p2Val = getPtrVal(rv.cli.PullBackoffBase)
	default:
		_, s1, v1, s2, v2, err := fetchFieldAndParams(opt.Name, rv.getCliVal())
		if err != nil {
			return err
		}
		p1Set, p1Val, p2Set, p2Val = s1, v1.String(), s2, v2.String()
	}

	valStr := resolveStringOpt(def, p1Set, p1Val, p2Set, p2Val, rv.subcommand, rv.tools, rv.global, rv.r, rv.fs)
	if strings.Contains(valStr, "{{") || strings.HasPrefix(valStr, "~") {
		r, err := rv.getR()
		if err != nil {
			return err
		}
		valStr = r.resolveString(valStr)
		if err := r.Error(); err != nil {
			return err
		}
	} else if rv.r != nil {
		if err := rv.r.Error(); err != nil {
			return err
		}
	}

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
		p1Set, p1Val = getPtrVal(rv.cli.CderunMemory)
		p2Set, p2Val = getPtrVal(rv.cli.Memory)
	default:
		_, s1, v1, s2, v2, err := fetchFieldAndParams(opt.Name, rv.getCliVal())
		if err != nil {
			return err
		}
		p1Set, p1Val, p2Set, p2Val = s1, v1.String(), s2, v2.String()
	}

	valStr := resolveStringOpt(def, p1Set, p1Val, p2Set, p2Val, rv.subcommand, rv.tools, rv.global, rv.r, rv.fs)
	if strings.Contains(valStr, "{{") || strings.HasPrefix(valStr, "~") {
		r, err := rv.getR()
		if err != nil {
			return err
		}
		valStr = r.resolveString(valStr)
		if err := r.Error(); err != nil {
			return err
		}
	}

	if valStr != "" {
		bytes, err := units.RAMInBytes(valStr)
		if err != nil {
			if rv.r != nil {
				if exprErr := rv.r.Error(); exprErr != nil {
					return exprErr
				}
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

	if _, err := fs.UserHomeDir(); err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}
	if _, err := fs.Getwd(); err != nil {
		return nil, fmt.Errorf("failed to get current working directory: %w", err)
	}

	fieldOnce.Do(initFieldInfo)

	res := &ResolvedConfig{}
	rv := &resolver{
		subcommand: subcommand,
		cli:        cli,
		tools:      tools,
		global:     global,
		fs:         fs,
		r:          nil, // Lazily initialized
		res:        res,
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
	// We call applyBoolOption directly which uses fast-paths.
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

		def := OptionDef[[]string]{
			EnvKey:       opt.EnvKey,
			ToolGetter:   opt.ToolGetter,
			GlobalGetter: opt.GlobalGetter,
		}

		rForSens, err := rv.resolverForSlice(def, ",", rv.cli.CderunSensitiveEnv, rv.cli.SensitiveEnv)
		if err != nil {
			return err
		}

		rv.res.SensitiveEnv = resolveStringSliceOpt(def, ",", rv.cli.CderunSensitiveEnv, rv.cli.SensitiveEnv, rv.subcommand, rv.tools, rv.global, rForSens, rv.fs)
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
					var errCLI, errCfg error
					resolvedCLIImage := cliImage
					resolvedCfgImage := tool.Image

					if strings.Contains(cliImage, "{{") || strings.HasPrefix(cliImage, "~") {
						r, err := rv.getR()
						if err != nil {
							return err
						}
						resolvedCLIImage, errCLI = r.ResolveString(cliImage)
					}
					if strings.Contains(tool.Image, "{{") || strings.HasPrefix(tool.Image, "~") {
						r, err := rv.getR()
						if err != nil {
							return err
						}
						resolvedCfgImage, errCfg = r.ResolveString(tool.Image)
					}

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
	var rForMounts *ExpressionResolver
	hasMounts := len(rv.cli.CderunMounts) > 0 || len(rv.cli.Mounts) > 0 || rv.fs.Getenv("CDERUN_MOUNT") != ""
	if !hasMounts && rv.tools != nil {
		if tool, ok := rv.tools[rv.subcommand]; ok && len(tool.Mounts) > 0 {
			hasMounts = true
		}
	}
	if !hasMounts && rv.global != nil && len(rv.global.Defaults.Mounts) > 0 {
		hasMounts = true
	}

	if hasMounts {
		var err error
		rForMounts, err = rv.getR()
		if err != nil {
			return err
		}
	}

	// Complex types (Mounts, Env)
	rv.res.Mounts, err = resolveMounts(rv.cli.CderunMounts, rv.cli.Mounts, rv.subcommand, rv.tools, rv.global, rForMounts, rv.fs)
	if err != nil {
		return err
	}

	var rForEnv *ExpressionResolver
	// Check if there are any environment variables
	var envs []string
	envs, err = pickConfigs(
		rv.cli.CderunEnv, rv.cli.Env, "CDERUN_ENV", ";", rv.subcommand, rv.tools,
		func(t ToolConfig) []string { return t.Env },
		rv.global, func(g CDERunConfig) []string { return g.Defaults.Env },
		nil,
		rv.fs,
	)
	if err != nil {
		return err
	}

	hasEnvWithExpr := false
	for _, e := range envs {
		if strings.Contains(e, "{{") || strings.HasPrefix(e, "~") {
			hasEnvWithExpr = true
			break
		}
	}
	if hasEnvWithExpr {
		var err error
		rForEnv, err = rv.getR()
		if err != nil {
			return err
		}
	}

	// Deduplicate within the winning source (last-one-wins for the same key)
	var merged []string
	if len(envs) > 0 {
		merged = deduplicateEnv(envs)
	}

	rv.res.Env, err = resolveEnvValues(merged, rv.res.SensitiveEnv, rv.res.StrictEnv, rForEnv, rv.fs)
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
			base := path.Base(rv.res.SocketPath)
			if strings.Contains(base, "podman") {
				rv.res.Runtime = "podman"
			} else if strings.Contains(base, "containerd") {
				rv.res.Runtime = "containerd"
			} else {
				rv.res.Runtime = "docker"
			}
		} else {
			if _, isReal := rv.fs.(RealFileSystem); isReal {
				autoDetectMu.RLock()
				cachedRuntime := autoDetectedRuntime
				cachedSocketPath := autoDetectedSocketPath
				autoDetectMu.RUnlock()

				if cachedRuntime != "" {
					rv.res.Runtime = cachedRuntime
					rv.res.SocketPath = cachedSocketPath
				} else {
					autoDetectMu.Lock()
					if autoDetectedRuntime != "" {
						rv.res.Runtime = autoDetectedRuntime
						rv.res.SocketPath = autoDetectedSocketPath
					} else {
						var detectedRuntime string
						var detectedSocketPath string

						if _, err := rv.fs.Stat("/var/run/docker.sock"); err == nil {
							detectedRuntime = "docker"
							detectedSocketPath = "/var/run/docker.sock"
						} else if _, err := rv.fs.Stat("/run/containerd/containerd.sock"); err == nil {
							detectedRuntime = "containerd"
							detectedSocketPath = "/run/containerd/containerd.sock"
						} else if _, err := rv.fs.Stat("/run/podman/podman.sock"); err == nil {
							detectedRuntime = "podman"
							detectedSocketPath = "/run/podman/podman.sock"
						}

						if detectedRuntime != "" {
							autoDetectedRuntime = detectedRuntime
							autoDetectedSocketPath = detectedSocketPath
							rv.res.Runtime = detectedRuntime
							rv.res.SocketPath = detectedSocketPath
						}
					}
					autoDetectMu.Unlock()
				}
			} else {
				if _, err := rv.fs.Stat("/var/run/docker.sock"); err == nil {
					rv.res.Runtime = "docker"
					rv.res.SocketPath = "/var/run/docker.sock"
				} else if _, err := rv.fs.Stat("/run/containerd/containerd.sock"); err == nil {
					rv.res.Runtime = "containerd"
					rv.res.SocketPath = "/run/containerd/containerd.sock"
				} else if _, err := rv.fs.Stat("/run/podman/podman.sock"); err == nil {
					rv.res.Runtime = "podman"
					rv.res.SocketPath = "/run/podman/podman.sock"
				}
			}

			if rv.res.Runtime == "" {
				rv.res.Runtime = "docker"
				rv.res.SocketPath = "/var/run/docker.sock"
			}
		}
	}

	if rv.res.SocketPath == "" {
		switch rv.res.Runtime {
		case "podman":
			rv.res.SocketPath = "/run/podman/podman.sock"
		case "containerd":
			rv.res.SocketPath = "/run/containerd/containerd.sock"
		default:
			rv.res.SocketPath = "/var/run/docker.sock"
		}
	}
	rv.res.SocketPath = strings.TrimPrefix(rv.res.SocketPath, "unix://")
	return nil
}

func (rv *resolver) resolveTransitiveOptions() error {
	// Transitive options (MountTools -> MountCderun -> MountSocket)
	p2SetTools, p2ValTools := getPtrVal(rv.cli.MountTools)
	p1SetTools, p1ValTools := getPtrVal(rv.cli.CderunMountTools)
	rv.res.MountTools = resolveStringSliceCommaOpt(
		OptionDef[[]string]{EnvKey: "CDERUN_MOUNT_TOOLS",
			ToolGetter:   func(t ToolConfig) []string { return t.MountTools },
			GlobalGetter: func(g CDERunConfig) []string { return g.Defaults.MountTools }},
		p1SetTools, p1ValTools,
		p2SetTools, p2ValTools,
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
		p1Set, p1Val := getPtrVal(rv.cli.CderunMountAllTools)
		p2Set, p2Val := getPtrVal(rv.cli.MountAllTools)
		def := OptionDef[*bool]{EnvKey: opt.EnvKey, ToolGetter: opt.ToolGetter, GlobalGetter: opt.GlobalGetter}
		var err error
		rv.res.MountAllTools, err = resolveBoolOpt(def, opt.Default, p1Set, p1Val, p2Set, p2Val, rv.subcommand, rv.tools, rv.global, rv.fs)
		if err != nil {
			return err
		}
	}

	var mountCderunSpecified bool
	{
		opt, _ := GetBoolOption("mount-cderun")
		p1Set, p1Val := getPtrVal(rv.cli.CderunMountCderun)
		p2Set, p2Val := getPtrVal(rv.cli.MountCderun)
		def := OptionDef[*bool]{EnvKey: opt.EnvKey, ToolGetter: opt.ToolGetter, GlobalGetter: opt.GlobalGetter}
		var err error
		rv.res.MountCderun, mountCderunSpecified, err = resolveBoolOptInfo(def, p1Set, p1Val, p2Set, p2Val, rv.subcommand, rv.tools, rv.global, rv.fs)
		if err != nil {
			return err
		}
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
		p1Set, p1Val := getPtrVal(rv.cli.CderunMountSocket)
		p2Set, p2Val := getPtrVal(rv.cli.MountSocket)
		def := OptionDef[*bool]{EnvKey: opt.EnvKey, ToolGetter: opt.ToolGetter, GlobalGetter: opt.GlobalGetter}
		var err error
		rv.res.MountSocket, mountSocketSpecified, err = resolveBoolOptInfo(def, p1Set, p1Val, p2Set, p2Val, rv.subcommand, rv.tools, rv.global, rv.fs)
		if err != nil {
			return err
		}
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

	var rForDevices *ExpressionResolver
	// Check if there are any devices to resolve
	hasDevices := len(rv.cli.CderunDevices) > 0 || len(rv.cli.Devices) > 0 || rv.fs.Getenv("CDERUN_DEVICE") != ""
	if !hasDevices && rv.tools != nil {
		if tool, ok := rv.tools[rv.subcommand]; ok && len(tool.Devices) > 0 {
			hasDevices = true
		}
	}
	if !hasDevices && rv.global != nil && len(rv.global.Defaults.Devices) > 0 {
		hasDevices = true
	}

	if hasDevices {
		var err error
		rForDevices, err = rv.getR()
		if err != nil {
			return err
		}
	}

	var errDevices error
	rv.res.Devices, errDevices = resolveDevices(rv.cli.CderunDevices, rv.cli.Devices, rv.subcommand, rv.tools, rv.global, rForDevices, rv.fs)
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

	if rv.r != nil {
		if err := rv.r.Error(); err != nil {
			return err
		}
	}
	return nil
}

// highlyPrivilegedCaps defines highly privileged Linux capabilities in both standard
// and "CAP_"-prefixed forms. Granting these capabilities poses security risks (e.g., container breakout).
var highlyPrivilegedCaps = map[string]bool{
	"ALL":            true,
	"SYS_ADMIN":      true,
	"NET_ADMIN":      true,
	"SYS_RAWIO":      true,
	"SYS_PTRACE":     true,
	"SYS_MODULE":     true,
	"CAP_ALL":        true,
	"CAP_SYS_ADMIN":  true,
	"CAP_NET_ADMIN":  true,
	"CAP_SYS_RAWIO":  true,
	"CAP_SYS_PTRACE": true,
	"CAP_SYS_MODULE": true,
}

// isHighlyPrivilegedCapability returns true if the given capability name is highly privileged.
// It performs a case-insensitive, trimmed, read-only lookup to prevent mutation of the underlying map.
func isHighlyPrivilegedCapability(capName string) bool {
	upperCap := strings.ToUpper(strings.TrimSpace(capName))
	return highlyPrivilegedCaps[upperCap]
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
	var found []string
	for _, capName := range rv.res.CapAdd {
		if isHighlyPrivilegedCapability(capName) {
			found = append(found, capName)
		}
	}

	if logging.Enabled(logging.WarnLevel) {
		if rv.res.Privileged {
			logging.Warn("Container is running in privileged mode. This reduces container isolation and may pose security risks.")
			if len(found) > 0 {
				logging.Warn("Highly privileged capability %v detected in CapAdd while running in privileged mode. Please consider minimizing privileges.", found)
			}
		} else if len(found) > 0 {
			logging.Warn("Highly privileged capability %v detected in CapAdd. Please consider minimizing privileges.", found)
		}
		if rv.res.Network == "host" {
			logging.Warn("Container is running with host network mode enabled. This disables network isolation and may expose host network services to the container.")
		}
		for _, m := range rv.res.Mounts {
			if (m.Type == "bind" || m.Type == "") && m.Source != "" {
				cleanSource := path.Clean(m.Source)
				sensitivePaths := []string{"/boot", "/dev", "/etc", "/proc", "/sys"}
				isSensitive := cleanSource == "/"
				for _, p := range sensitivePaths {
					if cleanSource == p || strings.HasPrefix(cleanSource, p+"/") {
						isSensitive = true
						break
					}
				}
				if isSensitive {
					logging.Warn("Mounting highly sensitive host path %q into the container reduces host security isolation. Please ensure this is intended.", m.Source)
				}
			}
		}
	}
	if rv.res.MountSocket {
		if logging.Enabled(logging.WarnLevel) {
			logging.Warn("Container socket mounting is enabled. Granting access to the container runtime socket is highly privileged and allows full control over the container engine.")

			if ContainsNumericGID(rv.res.GroupAdd) {
				logging.Warn("Granting container socket permissions through a numeric VM socket GID allows socket access but is highly privileged. Limit such deployments to trusted environments.")
			}
		}
	}
	return nil
}

func (rv *resolver) validateCriticalFields() error {
	// image
	if err := validatePathChars(rv.res.Image); err != nil {
		return fmt.Errorf("security validation failed for %q: %w", "image", err)
	}
	if err := ValidateImageName(rv.res.Image); err != nil {
		return fmt.Errorf("security validation failed for %q: %w", "image", err)
	}

	// pid
	if err := validatePathChars(rv.res.Pid); err != nil {
		return fmt.Errorf("security validation failed for %q: %w", "pid", err)
	}
	if rv.res.Pid != "" && rv.res.Pid != "host" {
		return fmt.Errorf("security validation failed for %q: unsupported pid namespace: %q", "pid", rv.res.Pid)
	}

	// user
	if err := validatePathChars(rv.res.User); err != nil {
		return fmt.Errorf("security validation failed for %q: %w", "user", err)
	}
	if err := ValidateUserName(rv.res.User); err != nil {
		return fmt.Errorf("security validation failed for %q: %w", "user", err)
	}

	// network
	if err := validatePathChars(rv.res.Network); err != nil {
		return fmt.Errorf("security validation failed for %q: %w", "network", err)
	}
	if err := ValidateNetworkName(rv.res.Network); err != nil {
		return fmt.Errorf("security validation failed for %q: %w", "network", err)
	}

	// hostname
	if err := validatePathChars(rv.res.Hostname); err != nil {
		return fmt.Errorf("security validation failed for %q: %w", "hostname", err)
	}
	if err := ValidateHostname(rv.res.Hostname); err != nil {
		return fmt.Errorf("security validation failed for %q: %w", "hostname", err)
	}

	// workdir
	if err := validatePathChars(rv.res.Workdir); err != nil {
		return fmt.Errorf("security validation failed for %q: %w", "workdir", err)
	}
	if err := ValidateWorkdir(rv.res.Workdir); err != nil {
		return fmt.Errorf("security validation failed for %q: %w", "workdir", err)
	}

	// runtime
	if err := validatePathChars(rv.res.Runtime); err != nil {
		return fmt.Errorf("security validation failed for %q: %w", "runtime", err)
	}
	if rv.res.Runtime != "docker" && rv.res.Runtime != "podman" && rv.res.Runtime != "containerd" {
		return fmt.Errorf("security validation failed for %q: unsupported runtime: %q", "runtime", rv.res.Runtime)
	}

	// pull
	if err := validatePathChars(rv.res.Pull); err != nil {
		return fmt.Errorf("security validation failed for %q: %w", "pull", err)
	}
	if rv.res.Pull != "" && rv.res.Pull != "always" && rv.res.Pull != "missing" && rv.res.Pull != "never" {
		return fmt.Errorf("security validation failed for %q: invalid pull policy %q: allowed values are \"always\", \"missing\", or \"never\"", "pull", rv.res.Pull)
	}

	// socket-path
	if err := validatePathChars(rv.res.SocketPath); err != nil {
		return fmt.Errorf("security validation failed for %q: %w", "socket-path", err)
	}

	// mount-socket-path
	if err := validatePathChars(rv.res.MountSocketPath); err != nil {
		return fmt.Errorf("security validation failed for %q: %w", "mount-socket-path", err)
	}

	// mount-cderun-path
	if err := validatePathChars(rv.res.MountCderunPath); err != nil {
		return fmt.Errorf("security validation failed for %q: %w", "mount-cderun-path", err)
	}

	// dry-run-format
	if err := validatePathChars(rv.res.DryRunFormat); err != nil {
		return fmt.Errorf("security validation failed for %q: %w", "dry-run-format", err)
	}
	if rv.res.DryRunFormat != "" && rv.res.DryRunFormat != "yaml" && rv.res.DryRunFormat != "json" && rv.res.DryRunFormat != "simple" {
		return fmt.Errorf("security validation failed for %q: unsupported dry-run format: %q", "dry-run-format", rv.res.DryRunFormat)
	}

	// diagnosis-format
	if err := validatePathChars(rv.res.DiagnosisFormat); err != nil {
		return fmt.Errorf("security validation failed for %q: %w", "diagnosis-format", err)
	}
	if rv.res.DiagnosisFormat != "" && rv.res.DiagnosisFormat != "yaml" && rv.res.DiagnosisFormat != "json" && rv.res.DiagnosisFormat != "simple" {
		return fmt.Errorf("security validation failed for %q: unsupported diagnosis format: %q", "diagnosis-format", rv.res.DiagnosisFormat)
	}

	// log-level
	if err := validatePathChars(rv.res.LogLevel); err != nil {
		return fmt.Errorf("security validation failed for %q: %w", "log-level", err)
	}
	if rv.res.LogLevel != "" {
		l := strings.ToLower(rv.res.LogLevel)
		if l != "error" && l != "warn" && l != "warning" && l != "info" && l != "debug" && l != "trace" {
			return fmt.Errorf("security validation failed for %q: unsupported log level: %q", "log-level", rv.res.LogLevel)
		}
	}

	// log-format
	if err := validatePathChars(rv.res.LogFormat); err != nil {
		return fmt.Errorf("security validation failed for %q: %w", "log-format", err)
	}
	if rv.res.LogFormat != "" && rv.res.LogFormat != "text" && rv.res.LogFormat != "json" {
		return fmt.Errorf("security validation failed for %q: unsupported log format: %q", "log-format", rv.res.LogFormat)
	}

	if rv.res.Memory < 0 {
		return &InvalidConfigError{
			Field: "memory",
			Value: fmt.Sprintf("%d", rv.res.Memory),
			Err:   errors.New("memory limit cannot be negative"),
		}
	}
	if rv.res.CPUs < 0 {
		return &InvalidConfigError{
			Field: "cpus",
			Value: fmt.Sprintf("%g", rv.res.CPUs),
			Err:   errors.New("CPU limit cannot be negative"),
		}
	}

	return nil
}

func (rv *resolver) validateSlices() error {
	// entrypoint
	for i, e := range rv.res.Entrypoint {
		if err := validatePathChars(e); err != nil {
			return fmt.Errorf("security validation failed for %s[%d]: %w", "entrypoint", i, err)
		}
	}

	// ports
	for i, e := range rv.res.Ports {
		if err := validatePathChars(e); err != nil {
			return fmt.Errorf("security validation failed for %s[%d]: %w", "ports", i, err)
		}
		if err := ValidatePort(e); err != nil {
			return fmt.Errorf("security validation failed for %s[%d]: %w", "ports", i, err)
		}
	}

	// expose
	for i, e := range rv.res.Expose {
		if err := validatePathChars(e); err != nil {
			return fmt.Errorf("security validation failed for %s[%d]: %w", "expose", i, err)
		}
		if err := ValidateExposePort(e); err != nil {
			return fmt.Errorf("security validation failed for %s[%d]: %w", "expose", i, err)
		}
	}

	// dns
	for i, e := range rv.res.DNS {
		if err := validatePathChars(e); err != nil {
			return fmt.Errorf("security validation failed for %s[%d]: %w", "dns", i, err)
		}
		if err := ValidateDNS(e); err != nil {
			return fmt.Errorf("security validation failed for %s[%d]: %w", "dns", i, err)
		}
	}

	// add-hosts
	for i, e := range rv.res.AddHosts {
		if err := validatePathChars(e); err != nil {
			return fmt.Errorf("security validation failed for %s[%d]: %w", "add-hosts", i, err)
		}
		if err := ValidateAddHost(e); err != nil {
			return fmt.Errorf("security validation failed for %s[%d]: %w", "add-hosts", i, err)
		}
	}

	// cap-add
	for i, e := range rv.res.CapAdd {
		if err := validatePathChars(e); err != nil {
			return fmt.Errorf("security validation failed for %s[%d]: %w", "cap-add", i, err)
		}
		if err := ValidateCapability(e); err != nil {
			return fmt.Errorf("security validation failed for %s[%d]: %w", "cap-add", i, err)
		}
	}

	// cap-drop
	for i, e := range rv.res.CapDrop {
		if err := validatePathChars(e); err != nil {
			return fmt.Errorf("security validation failed for %s[%d]: %w", "cap-drop", i, err)
		}
		if err := ValidateCapability(e); err != nil {
			return fmt.Errorf("security validation failed for %s[%d]: %w", "cap-drop", i, err)
		}
	}

	// group-add
	for i, e := range rv.res.GroupAdd {
		if err := validatePathChars(e); err != nil {
			return fmt.Errorf("security validation failed for %s[%d]: %w", "group-add", i, err)
		}
		if err := ValidateGroupAdd(e); err != nil {
			return fmt.Errorf("security validation failed for %s[%d]: %w", "group-add", i, err)
		}
	}

	// sensitive-env
	for i, e := range rv.res.SensitiveEnv {
		if err := validatePathChars(e); err != nil {
			return fmt.Errorf("security validation failed for %s[%d]: %w", "sensitive-env", i, err)
		}
		_, err := path.Match(e, "TEST")
		if err != nil {
			return fmt.Errorf("security validation failed for %s[%d]: invalid glob pattern: %w", "sensitive-env", i, err)
		}
	}

	return nil
}

func (rv *resolver) validateEnvSecurity() error {
	for i, e := range rv.res.Env {
		key, val, _ := strings.Cut(e, "=")
		if err := validatePathChars(key); err != nil {
			return fmt.Errorf("security validation failed for env[%d] (key): %w", i, err)
		}
		if err := ValidateEnvKey(key); err != nil {
			return fmt.Errorf("security validation failed for env[%d] (key): %w", i, err)
		}
		if strings.ContainsRune(val, 0) {
			return fmt.Errorf("security validation failed for env[%d] (value): null byte injection detected", i)
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
		if m.Target == "" {
			return fmt.Errorf("security validation failed for mounts[%d] (target): target path cannot be empty", i)
		}
		if !path.IsAbs(m.Target) {
			return fmt.Errorf("security validation failed for mounts[%d] (target): target path must be an absolute path: %q", i, m.Target)
		}
		if HasParentTraversal(m.Target) {
			return fmt.Errorf("security validation failed for mounts[%d] (target): target path cannot contain parent directory references: %q", i, m.Target)
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
		if d.PathInContainer == "" {
			return fmt.Errorf("security validation failed for devices[%d] (path-in-container): destination path cannot be empty", i)
		}
		if !path.IsAbs(d.PathInContainer) {
			return fmt.Errorf("security validation failed for devices[%d] (path-in-container): destination path must be an absolute path: %q", i, d.PathInContainer)
		}
		if HasParentTraversal(d.PathInContainer) {
			return fmt.Errorf("security validation failed for devices[%d] (path-in-container): destination path cannot contain parent directory references: %q", i, d.PathInContainer)
		}
		if d.CgroupPermissions != "" {
			if err := validatePathChars(d.CgroupPermissions); err != nil {
				return fmt.Errorf("security validation failed for devices[%d] (permissions): %w", i, err)
			}
			if !permsRegex.MatchString(d.CgroupPermissions) {
				return fmt.Errorf("security validation failed for devices[%d] (permissions): invalid cgroup permissions %q", i, d.CgroupPermissions)
			}
		}
	}
	return nil
}

func resolveConfigPath(p1Set bool, p1Val string, cliSet bool, cliVal string, envKey string, subcommand string, tools ToolsConfig, toolGetter func(ToolConfig) ConfigPath, global *CDERunConfig, globalGetter func(CDERunConfig) ConfigPath, fallback string, r *ExpressionResolver, pathType string, fs FileSystem) (string, error) {
	var cp ConfigPath
	var baseDir string
	if r != nil {
		baseDir = r.Pwd
	} else {
		wd, err := fs.Getwd()
		if err != nil {
			return "", err
		}
		baseDir = wd
	}

	if p1Set {
		cp = ConfigPath{Raw: p1Val, BaseDir: baseDir}
	} else if cliSet {
		cp = ConfigPath{Raw: cliVal, BaseDir: baseDir}
	} else if env := fs.Getenv(envKey); env != "" {
		cp = ConfigPath{Raw: env, BaseDir: baseDir}
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
			cp = ConfigPath{Raw: fallback, BaseDir: baseDir}
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
