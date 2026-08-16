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
	Init            bool
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
	ShmSize         string
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
	Ulimits         []container.Ulimit
	Sysctls         map[string]string
	IPC             string
	SecurityOpt     []string
	DNSSearch       []string
	DNSOptions      []string
	GPUs            string
	Cgroupns        string
	PidsLimit       int
	CPUShares       int
	CpusetCpus      string
	CpusetMems      string
	Restart         string
}

// Resolve combines CLI flags, environment variables, tool-specific config, and global defaults.
func Resolve(subcommand string, cli *CLIOptions, tools ToolsConfig, global *CDERunConfig) (*ResolvedConfig, error) {
	return ResolveWithFS(subcommand, cli, tools, global, RealFileSystem{})
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
	var overrideSet, cliSet bool
	var overrideValStr, cliValStr string

	switch name {
	case "socket-path":
		overrideSet, overrideValStr = getPtrVal(rv.cli.CderunSocketPath)
		cliSet, cliValStr = getPtrVal(rv.cli.SocketPath)
	case "mount-socket-path":
		overrideSet, overrideValStr = getPtrVal(rv.cli.CderunMountSocketPath)
		cliSet, cliValStr = getPtrVal(rv.cli.MountSocketPath)
	case "mount-cderun-path":
		overrideSet, overrideValStr = getPtrVal(rv.cli.CderunMountCderunPath)
		cliSet, cliValStr = getPtrVal(rv.cli.MountCderunPath)
	default:
		_, s1, v1, s2, v2, err := fetchFieldAndParams(name, rv.getCliVal())
		if err != nil {
			return "", err
		}
		overrideSet, overrideValStr, cliSet, cliValStr = s1, v1.String(), s2, v2.String()
	}

	if !rv.hasPathToResolve(overrideSet, overrideValStr, cliSet, cliValStr, envKey, tGetter, gGetter, fallback) {
		return "", nil
	}

	// Find the winning raw path
	var raw string
	if overrideSet {
		raw = overrideValStr
	} else if cliSet {
		raw = cliValStr
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
		overrideSet, overrideValStr,
		cliSet, cliValStr,
		envKey,
		subcommand, tools, tGetter,
		rv.global, gGetter,
		fallback,
		r,
		"path",
		rv.fs,
	)
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
	if opt, ok := GetBoolOption("diagnosis"); !ok {
		return fmt.Errorf("registry mismatch: early boolean option \"diagnosis\" not found")
	} else if err := rv.applyBoolOption(opt); err != nil {
		return err
	}

	if opt, ok := GetBoolOption("strict-env"); !ok {
		return fmt.Errorf("registry mismatch: early boolean option \"strict-env\" not found")
	} else if err := rv.applyBoolOption(opt); err != nil {
		return err
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

		vals := getWinningStringSlice(def, ",", rv.cli.CderunSensitiveEnv, rv.cli.SensitiveEnv, rv.subcommand, rv.tools, rv.global, rv.fs)
		var rForSens *ExpressionResolver
		for _, v := range vals {
			if strings.Contains(v, "{{") || strings.HasPrefix(v, "~") {
				var err error
				rForSens, err = rv.getR()
				if err != nil {
					return err
				}
				break
			}
		}

		rv.res.SensitiveEnv = resolveStringSliceOptWithVals(vals, rForSens)
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

	if err := rv.resolveAndValidateImage(); err != nil {
		return err
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

func (rv *resolver) resolveAndValidateImage() error {
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

	// Resolve Ulimits
	rv.res.Ulimits, err = resolveUlimits(rv.cli.CderunUlimits, rv.cli.Ulimits, rv.subcommand, rv.tools, rv.global, rv.fs)
	if err != nil {
		return err
	}

	// Resolve Sysctls
	var rForSysctls *ExpressionResolver
	rawSysctls, err := pickConfigs(
		rv.cli.CderunSysctls, rv.cli.Sysctls, "CDERUN_SYSCTL", ",", rv.subcommand, rv.tools,
		func(t ToolConfig) []string { return t.Sysctls },
		rv.global, func(g CDERunConfig) []string { return g.Defaults.Sysctls },
		nil,
		rv.fs,
	)
	if err != nil {
		return err
	}

	if len(rawSysctls) > 0 {
		needResolver := false
		for _, raw := range rawSysctls {
			if strings.Contains(raw, "{{") || strings.HasPrefix(raw, "~") {
				needResolver = true
				break
			}
		}
		if !needResolver && rv.global != nil && rv.global.HostContext != nil && rv.global.HostContext.Level > 0 {
			needResolver = true
		}

		if needResolver {
			var err error
			rForSysctls, err = rv.getR()
			if err != nil {
				return err
			}
		}
	}

	rv.res.Sysctls, err = resolveSysctls(rv.cli.CderunSysctls, rv.cli.Sysctls, rv.subcommand, rv.tools, rv.global, rForSysctls, rv.fs)
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
	var err error
	rv.res.MountAllTools, err = rv.resolveBoolOption("mount-all-tools", rv.cli.CderunMountAllTools, rv.cli.MountAllTools)
	if err != nil {
		return err
	}

	var mountCderunSpecified bool
	rv.res.MountCderun, mountCderunSpecified, err = rv.resolveBoolOptionInfo("mount-cderun", rv.cli.CderunMountCderun, rv.cli.MountCderun)
	if err != nil {
		return err
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
	rv.res.MountSocket, mountSocketSpecified, err = rv.resolveBoolOptionInfo("mount-socket", rv.cli.CderunMountSocket, rv.cli.MountSocket)
	if err != nil {
		return err
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
