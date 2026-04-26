package config

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"
	"unicode"

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

func (rv *resolver) applyStringSliceOption(opt StringSliceOption) error {
	info, p1Set, p1Val, p2Set, p2Val, err := fetchFieldAndParams(opt.Name, rv.cliVal)
	if err != nil {
		return err
	}

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

	resolved := resolveStringSliceOpt(def, ",", p1v, p2v, rv.subcommand, rv.tools, rv.global, rv.r, rv.fs)
	rv.resVal.Field(info.targetIdx).Set(reflect.ValueOf(resolved))
	return nil
}

func (rv *resolver) applyStringOption(opt StringOption) error {
	info, p1Set, p1Val, p2Set, p2Val, err := fetchFieldAndParams(opt.Name, rv.cliVal)
	if err != nil {
		return err
	}

	def := OptionDef[string]{
		EnvKey:       opt.EnvKey,
		ToolGetter:   opt.ToolGetter,
		GlobalGetter: opt.GlobalGetter,
		Fallback:     opt.Default,
	}

	resolved := resolveStringOpt(def, p1Set, p1Val.String(), p2Set, p2Val.String(), rv.subcommand, rv.tools, rv.global, rv.r, rv.fs)
	rv.resVal.Field(info.targetIdx).SetString(resolved)
	return nil
}

func (rv *resolver) applyBoolOption(opt BoolOption) error {
	info, p1Set, p1Val, p2Set, p2Val, err := fetchFieldAndParams(opt.Name, rv.cliVal)
	if err != nil {
		return err
	}

	def := OptionDef[*bool]{
		EnvKey:       opt.EnvKey,
		ToolGetter:   opt.ToolGetter,
		GlobalGetter: opt.GlobalGetter,
	}

	resolved := resolveBoolOpt(def, opt.Default, p1Set, p1Val.Bool(), p2Set, p2Val.Bool(), rv.subcommand, rv.tools, rv.global, rv.fs)
	rv.resVal.Field(info.targetIdx).SetBool(resolved)
	return nil
}

func (rv *resolver) applyIntOption(opt IntOption) error {
	info, p1Set, p1Val, p2Set, p2Val, err := fetchFieldAndParams(opt.Name, rv.cliVal)
	if err != nil {
		return err
	}

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

	resolved := resolveIntOpt(def, p1Set, p1Int, p2Set, p2Int, rv.subcommand, rv.tools, rv.global, rv.fs)
	rv.resVal.Field(info.targetIdx).SetInt(int64(resolved))
	return nil
}

func (rv *resolver) applyFloat64Option(opt Float64Option) error {
	info, p1Set, p1Val, p2Set, p2Val, err := fetchFieldAndParams(opt.Name, rv.cliVal)
	if err != nil {
		return err
	}

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
	_, p1Set, p1Val, p2Set, p2Val, err := fetchFieldAndParams(opt.Name, rv.cliVal)
	if err != nil {
		return err
	}

	valStr := resolveStringOpt(def, p1Set, p1Val.String(), p2Set, p2Val.String(), rv.subcommand, rv.tools, rv.global, rv.r, rv.fs)
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
	_, p1Set, p1Val, p2Set, p2Val, err := fetchFieldAndParams(opt.Name, rv.cliVal)
	if err != nil {
		return err
	}

	valStr := resolveStringOpt(def, p1Set, p1Val.String(), p2Set, p2Val.String(), rv.subcommand, rv.tools, rv.global, rv.r, rv.fs)
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
	// Phase 1: Early resolution (Diagnosis & StrictEnv)
	for _, name := range []string{"diagnosis", "strict-env"} {
		opt, ok := GetBoolOption(name)
		if !ok {
			return fmt.Errorf("registry mismatch: early boolean option %q not found", name)
		}
		if err := rv.applyBoolOption(opt); err != nil {
			return err
		}
	}
	return nil
}

func (rv *resolver) resolveStandardOptions() error {
	// Phase 2: Registry-based options (String & Bool)

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

		if err := rv.applyBoolOption(opt); err != nil {
			return err
		}
	}
	return nil
}

func (rv *resolver) resolveComplexOptions() error {
	var err error
	// Phase 4: Complex types (Mounts, Env)
	rv.res.Mounts, err = resolveMounts(rv.cli.CderunMounts, rv.cli.Mounts, rv.subcommand, rv.tools, rv.global, rv.r, rv.fs)
	if err != nil {
		return err
	}

	rv.res.Env, err = resolveEnv(rv.cli.CderunEnv, rv.cli.Env, "CDERUN_ENV", rv.subcommand, rv.tools, rv.global, rv.res.StrictEnv, rv.r, rv.fs)
	if err != nil {
		return err
	}
	return nil
}

func (rv *resolver) resolveRuntimeAndSocket() error {
	// Phase 5: Path resolution & Auto-detection (Socket)
	{
		_, p1Set, p1Val, p2Set, p2Val, err := fetchFieldAndParams("socket-path", rv.cliVal)
		if err != nil {
			return err
		}

		var errPath error
		rv.res.SocketPath, errPath = resolveConfigPath(
			p1Set, p1Val.String(),
			p2Set, p2Val.String(),
			"CDERUN_SOCKET_PATH",
			"", nil, nil,
			rv.global, func(g CDERunConfig) ConfigPath { return g.SocketPath },
			"",
			rv.r,
			"path",
			rv.fs,
		)
		if errPath != nil {
			return errPath
		}
	}

	if rv.res.Runtime == "" {
		if rv.res.SocketPath != "" {
			if strings.Contains(rv.res.SocketPath, "podman") {
				rv.res.Runtime = "podman"
			} else {
				rv.res.Runtime = "docker"
			}
		} else {
			if _, err := rv.fs.Stat("/var/run/docker.sock"); err == nil {
				rv.res.Runtime = "docker"
				rv.res.SocketPath = "/var/run/docker.sock"
			} else if _, err := rv.fs.Stat("/run/podman/podman.sock"); err == nil {
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
		} else {
			rv.res.SocketPath = "/var/run/docker.sock"
		}
	}
	rv.res.SocketPath = strings.TrimPrefix(rv.res.SocketPath, "unix://")
	return nil
}

func (rv *resolver) resolveTransitiveOptions() error {
	// Phase 6: Transitive options (MountTools -> MountCderun -> MountSocket)
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
		_, p1Set, p1Val, p2Set, p2Val, err := fetchFieldAndParams("mount-cderun-path", rv.cliVal)
		if err != nil {
			return err
		}

		var errPath error
		rv.res.MountCderunPath, errPath = resolveConfigPath(
			p1Set, p1Val.String(),
			p2Set, p2Val.String(),
			"CDERUN_MOUNT_CDERUN_PATH",
			rv.subcommand, rv.tools, func(t ToolConfig) ConfigPath { return t.MountCderunPath },
			rv.global, func(g CDERunConfig) ConfigPath { return g.Defaults.MountCderunPath },
			"",
			rv.r,
			"path",
			rv.fs,
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
		_, p1Set, p1Val, p2Set, p2Val, err := fetchFieldAndParams("mount-socket-path", rv.cliVal)
		if err != nil {
			return err
		}

		var errPath error
		rv.res.MountSocketPath, errPath = resolveConfigPath(
			p1Set, p1Val.String(),
			p2Set, p2Val.String(),
			"CDERUN_MOUNT_SOCKET_PATH",
			rv.subcommand, rv.tools, func(t ToolConfig) ConfigPath { return t.MountSocketPath },
			rv.global, func(g CDERunConfig) ConfigPath { return g.Defaults.MountSocketPath },
			rv.res.SocketPath,
			rv.r,
			"path",
			rv.fs,
		)
		if errPath != nil {
			return errPath
		}
	}
	return nil
}

func (rv *resolver) resolveCustomParsing() error {
	// Phase 7: Duration and Slice options
	// Resolve hang-timeout via registry entry (skipped in Phase 2)
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

	// Phase 8: Integer & Float options
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
	// Security: validate resolved configuration for injection characters and identifier formats.
	criticalFields := []struct {
		name      string
		value     string
		validator func(string) error
	}{
		{"image", rv.res.Image, ValidateImageName},
		{"user", rv.res.User, ValidateUserName},
		{"network", rv.res.Network, ValidateNetworkName},
		{"hostname", rv.res.Hostname, ValidateHostname},
		{"workdir", rv.res.Workdir, nil},
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

	criticalSlices := []struct {
		name      string
		slice     []string
		validator func(string) error
	}{
		{"entrypoint", rv.res.Entrypoint, nil},
		{"ports", rv.res.Ports, ValidatePort},
		{"expose", rv.res.Expose, ValidateExposePort},
		{"dns", rv.res.DNS, nil},
		{"add-hosts", rv.res.AddHosts, nil},
		{"cap-add", rv.res.CapAdd, nil},
		{"cap-drop", rv.res.CapDrop, nil},
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

	for i, e := range rv.res.Env {
		key, _, _ := strings.Cut(e, "=")
		if err := validatePathChars(key); err != nil {
			return fmt.Errorf("security validation failed for env[%d] (key): %w", i, err)
		}
		if err := ValidateEnvKey(key); err != nil {
			return fmt.Errorf("security validation failed for env[%d] (key): %w", i, err)
		}
	}

	for i, m := range rv.res.Mounts {
		if err := validatePathChars(m.Source); err != nil {
			return fmt.Errorf("security validation failed for mounts[%d] (source): %w", i, err)
		}
		if err := validatePathChars(m.Target); err != nil {
			return fmt.Errorf("security validation failed for mounts[%d] (target): %w", i, err)
		}
	}

	for i, d := range rv.res.Devices {
		if err := validatePathChars(d.PathOnHost); err != nil {
			return fmt.Errorf("security validation failed for devices[%d] (path-on-host): %w", i, err)
		}
		if err := validatePathChars(d.PathInContainer); err != nil {
			return fmt.Errorf("security validation failed for devices[%d] (path-in-container): %w", i, err)
		}
	}

	if rv.res.Privileged {
		if logging.Enabled(logging.WarnLevel) {
			logging.Warn("Container is running in privileged mode. This reduces container isolation and may pose security risks.")
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

func resolveDevices(p1 []string, p2 []string, subcommand string, tools ToolsConfig, global *CDERunConfig, r *ExpressionResolver, fs FileSystem) ([]container.DeviceMapping, error) {
	var dcs []DeviceConfig

	if p1 != nil {
		dcs = []DeviceConfig{}
		for _, d := range p1 {
			parsed, ok := ParseDeviceConfig(d)
			if !ok {
				return nil, fmt.Errorf("invalid device config (override): %q", d)
			}
			parsed.SetBaseDir(r.Pwd)
			dcs = append(dcs, parsed)
		}
	} else if p2 != nil {
		dcs = []DeviceConfig{}
		for _, d := range p2 {
			parsed, ok := ParseDeviceConfig(d)
			if !ok {
				return nil, fmt.Errorf("invalid device config: %q", d)
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
				return nil, fmt.Errorf("invalid device config in CDERUN_DEVICE: %q", d)
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
	total := len(base) + len(p2) + len(p1)
	if total == 0 {
		return nil
	}
	m := make(map[string]string, total)
	keys := make([]string, 0, total)

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

	res := make([]string, 0, len(keys))
	for _, k := range keys {
		res = append(res, m[k])
	}
	return res
}

func validateImageRegistryMatch(cliImage, configImage string) error {
	if cliImage == "" || configImage == "" {
		return nil
	}

	normalize := func(img string) (string, string) {
		parts := strings.Split(img, "/")
		var host, repo string
		if len(parts) == 1 {
			host = "docker.io"
			repo = "library/" + parts[0]
		} else if len(parts) == 2 {
			if strings.ContainsAny(parts[0], ".:") || parts[0] == "localhost" {
				host = parts[0]
				repo = parts[1]
			} else {
				host = "docker.io"
				repo = parts[0] + "/" + parts[1]
			}
		} else {
			host = parts[0]
			repo = strings.Join(parts[1:], "/")
		}

		if host == "docker.io" && !strings.Contains(repo, "/") {
			repo = "library/" + repo
		}

		// Strip tags or digests from repo
		if idx := strings.IndexAny(repo, ":@"); idx != -1 {
			repo = repo[:idx]
		}
		return host, repo
	}

	cliHost, cliRepo := normalize(cliImage)
	cfgHost, cfgRepo := normalize(configImage)

	if cliHost != cfgHost || cliRepo != cfgRepo {
		return &RegistryMismatchError{
			ExpectedRegistry: fmt.Sprintf("%s/%s", cfgHost, cfgRepo),
			ActualRegistry:   fmt.Sprintf("%s/%s", cliHost, cliRepo),
		}
	}

	return nil
}

func resolveEnvValues(env []string, strict bool, r *ExpressionResolver, fs FileSystem) ([]string, error) {
	var res []string
	for _, e := range env {
		resolvedE := r.resolveString(e)
		if err := r.Error(); err != nil {
			return nil, err
		}

		var key, val string
		if k, v, found := strings.Cut(resolvedE, "="); found {
			key = k
			val = v
			if err := ValidateEnvKey(key); err != nil {
				return nil, err
			}
		} else {
			v, found := fs.LookupEnv(resolvedE)
			if !found && strict {
				return nil, fmt.Errorf("required environment variable not found: %q", resolvedE)
			}
			key = resolvedE
			val = v
		}

		// Apply masking for debug logs and quoting for safety
		if logging.DebugEnabled() {
			logging.Debug("Resolved Env: %q=%q", key, MaskSensitiveEnv(key, val))
		}

		res = append(res, key+"="+val)
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

var sensitiveKeywords = map[string]struct{}{
	"PASSWORD":    {},
	"SECRET":      {},
	"TOKEN":       {},
	"KEY":         {},
	"AUTH":        {},
	"SIG":         {},
	"CERT":        {},
	"PEM":         {},
	"PRIVATE":     {},
	"CREDENTIALS": {},
	"PASSPHRASE":  {},
	"APIKEY":      {},
	"SESSION":     {},
	"ACCESS":      {},
	"JWT":         {},
	"SALT":        {},
}

// MaskSensitiveEnv redacts sensitive environment variables based on key names.
func MaskSensitiveEnv(key, value string) string {
	if value == "" {
		return ""
	}

	// Fast path: if the key doesn't contain any potential sensitive keywords, skip complex splitting.
	upperKey := strings.ToUpper(key)
	hasSensitive := false
	for kw := range sensitiveKeywords {
		if strings.Contains(upperKey, kw) {
			hasSensitive = true
			break
		}
	}
	if !hasSensitive {
		return value
	}

	// Split by non-alphanumeric characters and also split camelCase.
	// This ensures segments like "dbPassword" are correctly identified as ["db", "Password"].
	// We perform a single pass and check segments against sensitiveKeywords without extra allocations.
	useUpperDirectly := len(upperKey) == len(key)
	start := -1
	var lastRune rune

	for i, r := range key {
		isAlphaNum := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || (r > 127 && (unicode.IsLetter(r) || unicode.IsDigit(r)))

		if isAlphaNum {
			if start == -1 {
				start = i
			} else {
				// Boundary split logic (camelCase, letter/digit transition, acronyms)
				isCamel := unicode.IsLower(lastRune) && unicode.IsUpper(r)
				isLetterDigit := (unicode.IsLetter(lastRune) && unicode.IsDigit(r)) || (unicode.IsDigit(lastRune) && unicode.IsLetter(r))
				isAcronym := false
				if unicode.IsUpper(lastRune) && unicode.IsUpper(r) {
					// Check for acronym boundary (e.g. APIKey -> API, Key)
					for _, nextRune := range key[i+len(string(r)):] {
						if unicode.IsLower(nextRune) {
							isAcronym = true
						}
						break
					}
				}

				if isCamel || isLetterDigit || isAcronym {
					var segment string
					if useUpperDirectly {
						segment = upperKey[start:i]
					} else {
						segment = strings.ToUpper(key[start:i])
					}
					if _, ok := sensitiveKeywords[segment]; ok {
						return "[REDACTED]"
					}
					start = i
				}
			}
		} else {
			if start != -1 {
				var segment string
				if useUpperDirectly {
					segment = upperKey[start:i]
				} else {
					segment = strings.ToUpper(key[start:i])
				}
				if _, ok := sensitiveKeywords[segment]; ok {
					return "[REDACTED]"
				}
				start = -1
			}
		}
		lastRune = r
	}

	if start != -1 {
		var segment string
		if useUpperDirectly {
			segment = upperKey[start:]
		} else {
			segment = strings.ToUpper(key[start:])
		}
		if _, ok := sensitiveKeywords[segment]; ok {
			return "[REDACTED]"
		}
	}

	return value
}

// MaskSensitiveEnvList returns a new slice of environment variables with sensitive values masked.
func MaskSensitiveEnvList(env []string) []string {
	if env == nil {
		return nil
	}
	res := make([]string, len(env))
	for i, e := range env {
		if k, v, found := strings.Cut(e, "="); found {
			res[i] = fmt.Sprintf("%s=%s", k, MaskSensitiveEnv(k, v))
		} else {
			res[i] = e
		}
	}
	return res
}
