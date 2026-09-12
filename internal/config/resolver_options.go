package config

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/docker/go-units"
)

func getPtrVal[T any](p *T) (bool, T) {
	if p == nil {
		var zero T
		return false, zero
	}
	return true, *p
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
	info, ok := fieldInfo[key]
	if !ok {
		return optionFields{}, false, reflect.Value{}, false, reflect.Value{}, fmt.Errorf("registry mismatch: info for option %q not found", key)
	}
	p1Set, p1Val := getFieldInfo(cliVal, info.p1ValIdx)
	p2Set, p2Val := getFieldInfo(cliVal, info.p2ValIdx)
	return info, p1Set, p1Val, p2Set, p2Val, nil
}

func resolveOptionFieldInfo(name string, info optionFields) (optionFields, error) {
	if info.p1ValIdx == 0 && info.p2ValIdx == 0 && info.targetIdx == 0 {
		var ok bool
		info, ok = fieldInfo[name]
		if !ok {
			return optionFields{}, fmt.Errorf("registry mismatch: info for option %q not found", name)
		}
	} else if inTest {
		if mapInfo, ok := fieldInfo[name]; ok && mapInfo != info {
			info = mapInfo
		}
	}
	return info, nil
}

func isDriftOk(name string, info optionFields) bool {
	if !inTest && info.p1ValIdx != -1 && info.p2ValIdx != -1 {
		return true
	}
	expected, okExpected := expectedFieldIndices[name]
	return okExpected && info.p1ValIdx == expected.p1ValIdx && info.p2ValIdx == expected.p2ValIdx
}

func (rv *resolver) resolveStringOptionWithExpressions(def OptionDef[string], p1Set bool, p1Val string, p2Set bool, p2Val string) (string, error) {
	resolved := resolveStringOpt(def, p1Set, p1Val, p2Set, p2Val, rv.subcommand, rv.tools, rv.global, rv.r, rv.fs)
	if strings.Contains(resolved, "{{") || strings.HasPrefix(resolved, "~") {
		r, err := rv.getR()
		if err != nil {
			return "", err
		}
		resolved = r.resolveString(resolved)
		if err := r.Error(); err != nil {
			return "", err
		}
	} else if rv.r != nil {
		if err := rv.r.Error(); err != nil {
			return "", err
		}
	}
	return resolved, nil
}

func (rv *resolver) resolveStringSliceOptionResolver(vals []string) (*ExpressionResolver, error) {
	for _, v := range vals {
		if strings.Contains(v, "{{") || strings.HasPrefix(v, "~") {
			return rv.getR()
		}
	}
	return nil, nil
}

func getStringOptionFastPathPtrs(cli *CLIOptions, name string) (*string, *string, bool) {
	switch name {
	case "image":
		return cli.CderunImage, cli.Image, true
	case "pid":
		return cli.CderunPid, cli.Pid, true
	case "shm-size":
		return cli.CderunShmSize, cli.ShmSize, true
	case "network":
		return cli.CderunNetwork, cli.Network, true
	case "workdir":
		return cli.CderunWorkdir, cli.Workdir, true
	case "runtime":
		return cli.CderunRuntime, cli.Runtime, true
	case "user":
		return cli.CderunUser, cli.User, true
	case "log-level":
		return cli.CderunLogLevel, cli.LogLevel, true
	case "log-format":
		return cli.CderunLogFormat, cli.LogFormat, true
	case "hostname":
		return cli.CderunHostname, cli.Hostname, true
	case "pull":
		return cli.CderunPull, cli.Pull, true
	case "dry-run-format":
		return cli.CderunDryRunFormat, cli.DryRunFormat, true
	case "diagnosis-format":
		return cli.CderunDiagnosisFormat, cli.DiagnosisFormat, true
	case "ipc":
		return cli.CderunIPC, cli.IPC, true
	case "gpus":
		return cli.CderunGPUs, cli.GPUs, true
	case "cgroupns":
		return cli.CderunCgroupns, cli.Cgroupns, true
	case "cpuset-cpus":
		return cli.CderunCpusetCpus, cli.CpusetCpus, true
	case "cpuset-mems":
		return cli.CderunCpusetMems, cli.CpusetMems, true
	case "restart":
		return cli.CderunRestart, cli.Restart, true
	case "prefetch":
		return cli.CderunPrefetch, cli.Prefetch, true
	default:
		return nil, nil, false
	}
}

func assignResolvedString(res *ResolvedConfig, name string, resolved string) {
	switch name {
	case "image":
		res.Image = resolved
	case "pid":
		res.Pid = resolved
	case "shm-size":
		res.ShmSize = resolved
	case "network":
		res.Network = resolved
	case "workdir":
		res.Workdir = resolved
	case "runtime":
		res.Runtime = resolved
	case "user":
		res.User = resolved
	case "log-level":
		res.LogLevel = resolved
	case "log-format":
		res.LogFormat = resolved
	case "hostname":
		res.Hostname = resolved
	case "pull":
		res.Pull = resolved
	case "dry-run-format":
		res.DryRunFormat = resolved
	case "diagnosis-format":
		res.DiagnosisFormat = resolved
	case "ipc":
		res.IPC = resolved
	case "gpus":
		res.GPUs = resolved
	case "cgroupns":
		res.Cgroupns = resolved
	case "cpuset-cpus":
		res.CpusetCpus = resolved
	case "cpuset-mems":
		res.CpusetMems = resolved
	case "restart":
		res.Restart = resolved
	case "prefetch":
		res.Prefetch = resolved
	}
}

func getBoolOptionFastPathPtrs(cli *CLIOptions, name string) (*bool, *bool, bool) {
	switch name {
	case "tty":
		return cli.CderunTTY, cli.TTY, true
	case "interactive":
		return cli.CderunInteractive, cli.Interactive, true
	case "read-only":
		return cli.CderunReadOnly, cli.ReadOnly, true
	case "init":
		return cli.CderunInit, cli.Init, true
	case "remove":
		return cli.CderunRemove, cli.Remove, true
	case "diagnosis":
		return cli.CderunDiagnosis, cli.Diagnosis, true
	case "strict-env":
		return cli.CderunStrictEnv, cli.StrictEnv, true
	case "privileged":
		return cli.CderunPrivileged, cli.Privileged, true
	case "publish-all":
		return cli.CderunPublishAll, cli.PublishAll, true
	case "log-timestamp":
		return cli.CderunLogTimestamp, cli.LogTimestamp, true
	case "mount-socket":
		return cli.CderunMountSocket, cli.MountSocket, true
	case "mount-cderun":
		return cli.CderunMountCderun, cli.MountCderun, true
	case "mount-all-tools":
		return cli.CderunMountAllTools, cli.MountAllTools, true
	case "dry-run":
		return cli.CderunDryRun, cli.DryRun, true
	case "prefetch-all":
		return cli.CderunPrefetchAll, cli.PrefetchAll, true
	default:
		return nil, nil, false
	}
}

func assignResolvedBool(res *ResolvedConfig, name string, resolved bool) {
	switch name {
	case "tty":
		res.TTY = resolved
	case "interactive":
		res.Interactive = resolved
	case "read-only":
		res.ReadOnly = resolved
	case "init":
		res.Init = resolved
	case "remove":
		res.Remove = resolved
	case "diagnosis":
		res.Diagnosis = resolved
	case "strict-env":
		res.StrictEnv = resolved
	case "privileged":
		res.Privileged = resolved
	case "publish-all":
		res.PublishAll = resolved
	case "log-timestamp":
		res.LogTimestamp = resolved
	case "mount-socket":
		res.MountSocket = resolved
	case "mount-cderun":
		res.MountCderun = resolved
	case "mount-all-tools":
		res.MountAllTools = resolved
	case "dry-run":
		res.DryRun = resolved
	case "prefetch-all":
		res.PrefetchAll = resolved
	}
}

func getStringSliceOptionFastPathSlices(cli *CLIOptions, name string) ([]string, []string, bool) {
	switch name {
	case "publish":
		return cli.CderunPorts, cli.Ports, true
	case "expose":
		return cli.CderunExpose, cli.Expose, true
	case "dns":
		return cli.CderunDNS, cli.DNS, true
	case "add-host":
		return cli.CderunAddHosts, cli.AddHosts, true
	case "group-add":
		return cli.CderunGroupAdd, cli.GroupAdd, true
	case "cap-add":
		return cli.CderunCapAdd, cli.CapAdd, true
	case "cap-drop":
		return cli.CderunCapDrop, cli.CapDrop, true
	case "entrypoint":
		return cli.CderunEntrypoint, cli.Entrypoint, true
	case "security-opt":
		return cli.CderunSecurityOpt, cli.SecurityOpt, true
	case "dns-search":
		return cli.CderunDNSSearch, cli.DNSSearch, true
	case "dns-option":
		return cli.CderunDNSOptions, cli.DNSOptions, true
	default:
		return nil, nil, false
	}
}

func assignResolvedStringSlice(res *ResolvedConfig, name string, resolved []string) {
	switch name {
	case "publish":
		res.Ports = resolved
	case "expose":
		res.Expose = resolved
	case "dns":
		res.DNS = resolved
	case "add-host":
		res.AddHosts = resolved
	case "group-add":
		res.GroupAdd = resolved
	case "cap-add":
		res.CapAdd = resolved
	case "cap-drop":
		res.CapDrop = resolved
	case "entrypoint":
		res.Entrypoint = resolved
	case "security-opt":
		res.SecurityOpt = resolved
	case "dns-search":
		res.DNSSearch = resolved
	case "dns-option":
		res.DNSOptions = resolved
	}
}

func getIntOptionFastPathPtrs(cli *CLIOptions, name string) (*int, *int, bool) {
	switch name {
	case "pull-max-retries":
		return cli.CderunPullMaxRetries, cli.PullMaxRetries, true
	case "pids-limit":
		return cli.CderunPidsLimit, cli.PidsLimit, true
	case "cpu-shares":
		return cli.CderunCPUShares, cli.CPUShares, true
	default:
		return nil, nil, false
	}
}

func assignResolvedInt(res *ResolvedConfig, name string, resolved int) {
	switch name {
	case "pull-max-retries":
		res.PullMaxRetries = resolved
	case "pids-limit":
		res.PidsLimit = resolved
	case "cpu-shares":
		res.CPUShares = resolved
	}
}

func getFloat64OptionFastPathPtrs(cli *CLIOptions, name string) (*float64, *float64, bool) {
	if name == "cpus" {
		return cli.CderunCPUs, cli.CPUs, true
	}
	return nil, nil, false
}

func assignResolvedFloat64(res *ResolvedConfig, name string, resolved float64) {
	if name == "cpus" {
		res.CPUs = resolved
	}
}

func (rv *resolver) applyStringSliceOption(opt StringSliceOption) error {
	p1v, p2v, fastPathUsed := getStringSliceOptionFastPathSlices(rv.cli, opt.Name)

	info, err := resolveOptionFieldInfo(opt.Name, opt.info)
	if err != nil {
		return err
	}

	if fastPathUsed && isDriftOk(opt.Name, info) {
		def := OptionDef[[]string]{
			EnvKey:       opt.EnvKey,
			ToolGetter:   opt.ToolGetter,
			GlobalGetter: opt.GlobalGetter,
		}

		vals := getWinningStringSlice(def, ",", p1v, p2v, rv.subcommand, rv.tools, rv.global, rv.fs)
		rForSlice, err := rv.resolveStringSliceOptionResolver(vals)
		if err != nil {
			return err
		}
		resolved := resolveStringSliceOptWithVals(vals, rForSlice)
		assignResolvedStringSlice(rv.res, opt.Name, resolved)
		return nil
	}

	if info.targetIdx == -1 || info.p1ValIdx == -1 || info.p2ValIdx == -1 {
		return fmt.Errorf("registry mismatch: info for option %q not found", opt.Name)
	}

	s1, p1Val := getFieldInfo(rv.getCliVal(), info.p1ValIdx)
	s2, p2Val := getFieldInfo(rv.getCliVal(), info.p2ValIdx)

	p1v, _ = rv.extractStringSliceValue(p1Val, s1)
	p2v, _ = rv.extractStringSliceValue(p2Val, s2)

	def := OptionDef[[]string]{
		EnvKey:       opt.EnvKey,
		ToolGetter:   opt.ToolGetter,
		GlobalGetter: opt.GlobalGetter,
	}

	vals := getWinningStringSlice(def, ",", p1v, p2v, rv.subcommand, rv.tools, rv.global, rv.fs)
	rForSlice, err := rv.resolveStringSliceOptionResolver(vals)
	if err != nil {
		return err
	}

	resolved := resolveStringSliceOptWithVals(vals, rForSlice)
	rv.getResVal().Field(info.targetIdx).Set(reflect.ValueOf(resolved))
	return nil
}

func (rv *resolver) applyStringOption(opt StringOption) error {
	p1Ptr, p2Ptr, fastPathUsed := getStringOptionFastPathPtrs(rv.cli, opt.Name)
	p1Set, p1Val := getPtrVal(p1Ptr)
	p2Set, p2Val := getPtrVal(p2Ptr)

	info, err := resolveOptionFieldInfo(opt.Name, opt.info)
	if err != nil {
		return err
	}

	if fastPathUsed && isDriftOk(opt.Name, info) {
		def := OptionDef[string]{
			EnvKey:       opt.EnvKey,
			ToolGetter:   opt.ToolGetter,
			GlobalGetter: opt.GlobalGetter,
			Fallback:     opt.Default,
		}
		resolved, err := rv.resolveStringOptionWithExpressions(def, p1Set, p1Val, p2Set, p2Val)
		if err != nil {
			return err
		}
		assignResolvedString(rv.res, opt.Name, resolved)
		return nil
	}

	if info.targetIdx == -1 || info.p1ValIdx == -1 || info.p2ValIdx == -1 {
		return fmt.Errorf("registry mismatch: info for option %q not found", opt.Name)
	}

	s1, p1v := getFieldInfo(rv.getCliVal(), info.p1ValIdx)
	s2, p2v := getFieldInfo(rv.getCliVal(), info.p2ValIdx)
	p1Val, p2Val = p1v.String(), p2v.String()
	def := OptionDef[string]{EnvKey: opt.EnvKey, ToolGetter: opt.ToolGetter, GlobalGetter: opt.GlobalGetter, Fallback: opt.Default}
	resolved, err := rv.resolveStringOptionWithExpressions(def, s1, p1Val, s2, p2Val)
	if err != nil {
		return err
	}
	rv.getResVal().Field(info.targetIdx).SetString(resolved)
	return nil
}

func (rv *resolver) applyBoolOption(opt BoolOption) error {
	p1Ptr, p2Ptr, fastPathUsed := getBoolOptionFastPathPtrs(rv.cli, opt.Name)
	p1Set, p1Val := getPtrVal(p1Ptr)
	p2Set, p2Val := getPtrVal(p2Ptr)

	info, err := resolveOptionFieldInfo(opt.Name, opt.info)
	if err != nil {
		return err
	}

	if fastPathUsed && isDriftOk(opt.Name, info) {
		def := OptionDef[*bool]{
			EnvKey:       opt.EnvKey,
			ToolGetter:   opt.ToolGetter,
			GlobalGetter: opt.GlobalGetter,
		}
		resolved, err := resolveBoolOpt(def, opt.Default, p1Set, p1Val, p2Set, p2Val, rv.subcommand, rv.tools, rv.global, rv.fs)
		if err != nil {
			return err
		}
		assignResolvedBool(rv.res, opt.Name, resolved)
		return nil
	}

	if info.targetIdx == -1 || info.p1ValIdx == -1 || info.p2ValIdx == -1 {
		return fmt.Errorf("registry mismatch: info for option %q not found", opt.Name)
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
	p1Ptr, p2Ptr, fastPathUsed := getIntOptionFastPathPtrs(rv.cli, opt.Name)
	p1Set, p1Int := getPtrVal(p1Ptr)
	p2Set, p2Int := getPtrVal(p2Ptr)

	info, err := resolveOptionFieldInfo(opt.Name, opt.info)
	if err != nil {
		return err
	}

	if fastPathUsed && isDriftOk(opt.Name, info) {
		def := OptionDef[*int]{
			EnvKey:       opt.EnvKey,
			ToolGetter:   opt.ToolGetter,
			GlobalGetter: opt.GlobalGetter,
		}
		resolved, err := resolveIntOpt(def, opt.Default, p1Set, p1Int, p2Set, p2Int, rv.subcommand, rv.tools, rv.global, rv.fs)
		if err != nil {
			return err
		}
		assignResolvedInt(rv.res, opt.Name, resolved)
		return nil
	}

	if info.targetIdx == -1 || info.p1ValIdx == -1 || info.p2ValIdx == -1 {
		return fmt.Errorf("registry mismatch: info for option %q not found", opt.Name)
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
	p1Ptr, p2Ptr, fastPathUsed := getFloat64OptionFastPathPtrs(rv.cli, opt.Name)
	p1Set, p1Float := getPtrVal(p1Ptr)
	p2Set, p2Float := getPtrVal(p2Ptr)

	info, err := resolveOptionFieldInfo(opt.Name, opt.info)
	if err != nil {
		return err
	}

	if fastPathUsed && isDriftOk(opt.Name, info) {
		def := OptionDef[*float64]{
			EnvKey:       opt.EnvKey,
			ToolGetter:   opt.ToolGetter,
			GlobalGetter: opt.GlobalGetter,
		}
		resolved, err := resolveFloat64Opt(def, opt.Default, p1Set, p1Float, p2Set, p2Float, rv.subcommand, rv.tools, rv.global, rv.fs)
		if err != nil {
			return err
		}
		assignResolvedFloat64(rv.res, opt.Name, resolved)
		return nil
	}

	if info.targetIdx == -1 || info.p1ValIdx == -1 || info.p2ValIdx == -1 {
		return fmt.Errorf("registry mismatch: info for option %q not found", opt.Name)
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

	valStr, err := rv.resolveStringOptionWithExpressions(def, p1Set, p1Val, p2Set, p2Val)
	if err != nil {
		return err
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

// resolveBoolOption retrieves a registered boolean option by name, extracts priority values,
// and resolves its final boolean value using the standard resolution chain.
func (rv *resolver) resolveBoolOption(name string, p1, p2 *bool) (bool, error) {
	opt, ok := GetBoolOption(name)
	if !ok {
		return false, fmt.Errorf("registry mismatch: boolean option %q not found", name)
	}
	p1Set, p1Val := getPtrVal(p1)
	p2Set, p2Val := getPtrVal(p2)
	def := OptionDef[*bool]{
		EnvKey:       opt.EnvKey,
		ToolGetter:   opt.ToolGetter,
		GlobalGetter: opt.GlobalGetter,
	}
	return resolveBoolOpt(def, opt.Default, p1Set, p1Val, p2Set, p2Val, rv.subcommand, rv.tools, rv.global, rv.fs)
}

// resolveBoolOptionInfo retrieves a registered boolean option by name, extracts priority values,
// and resolves its final boolean value along with whether it was explicitly specified.
func (rv *resolver) resolveBoolOptionInfo(name string, p1, p2 *bool) (bool, bool, error) {
	opt, ok := GetBoolOption(name)
	if !ok {
		return false, false, fmt.Errorf("registry mismatch: boolean option %q not found", name)
	}
	p1Set, p1Val := getPtrVal(p1)
	p2Set, p2Val := getPtrVal(p2)
	def := OptionDef[*bool]{
		EnvKey:       opt.EnvKey,
		ToolGetter:   opt.ToolGetter,
		GlobalGetter: opt.GlobalGetter,
	}
	return resolveBoolOptInfo(def, p1Set, p1Val, p2Set, p2Val, rv.subcommand, rv.tools, rv.global, rv.fs)
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

	valStr, err := rv.resolveStringOptionWithExpressions(def, p1Set, p1Val, p2Set, p2Val)
	if err != nil {
		return err
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
