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
	if !inTest {
		return info.p1ValIdx != -1 && info.p2ValIdx != -1
	}
	expected, okExpected := expectedFieldIndices[name]
	if !okExpected || info.p1ValIdx != expected.p1ValIdx || info.p2ValIdx != expected.p2ValIdx {
		return false
	}
	if mapInfo, ok := fieldInfo[name]; ok && mapInfo != info {
		return false
	}
	return true
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

func getStringOptionFastPathPtrsAndAssign(name string) (func(*CLIOptions) (*string, *string, bool), func(*ResolvedConfig, string), bool) {
	switch name {
	case "image":
		return func(cli *CLIOptions) (*string, *string, bool) { return cli.CderunImage, cli.Image, true },
			func(res *ResolvedConfig, v string) { res.Image = v }, true
	case "pid":
		return func(cli *CLIOptions) (*string, *string, bool) { return cli.CderunPid, cli.Pid, true },
			func(res *ResolvedConfig, v string) { res.Pid = v }, true
	case "shm-size":
		return func(cli *CLIOptions) (*string, *string, bool) { return cli.CderunShmSize, cli.ShmSize, true },
			func(res *ResolvedConfig, v string) { res.ShmSize = v }, true
	case "network":
		return func(cli *CLIOptions) (*string, *string, bool) { return cli.CderunNetwork, cli.Network, true },
			func(res *ResolvedConfig, v string) { res.Network = v }, true
	case "workdir":
		return func(cli *CLIOptions) (*string, *string, bool) { return cli.CderunWorkdir, cli.Workdir, true },
			func(res *ResolvedConfig, v string) { res.Workdir = v }, true
	case "runtime":
		return func(cli *CLIOptions) (*string, *string, bool) { return cli.CderunRuntime, cli.Runtime, true },
			func(res *ResolvedConfig, v string) { res.Runtime = v }, true
	case "user":
		return func(cli *CLIOptions) (*string, *string, bool) { return cli.CderunUser, cli.User, true },
			func(res *ResolvedConfig, v string) { res.User = v }, true
	case "log-level":
		return func(cli *CLIOptions) (*string, *string, bool) { return cli.CderunLogLevel, cli.LogLevel, true },
			func(res *ResolvedConfig, v string) { res.LogLevel = v }, true
	case "log-format":
		return func(cli *CLIOptions) (*string, *string, bool) { return cli.CderunLogFormat, cli.LogFormat, true },
			func(res *ResolvedConfig, v string) { res.LogFormat = v }, true
	case "hostname":
		return func(cli *CLIOptions) (*string, *string, bool) { return cli.CderunHostname, cli.Hostname, true },
			func(res *ResolvedConfig, v string) { res.Hostname = v }, true
	case "pull":
		return func(cli *CLIOptions) (*string, *string, bool) { return cli.CderunPull, cli.Pull, true },
			func(res *ResolvedConfig, v string) { res.Pull = v }, true
	case "dry-run-format":
		return func(cli *CLIOptions) (*string, *string, bool) { return cli.CderunDryRunFormat, cli.DryRunFormat, true },
			func(res *ResolvedConfig, v string) { res.DryRunFormat = v }, true
	case "diagnosis-format":
		return func(cli *CLIOptions) (*string, *string, bool) { return cli.CderunDiagnosisFormat, cli.DiagnosisFormat, true },
			func(res *ResolvedConfig, v string) { res.DiagnosisFormat = v }, true
	case "ipc":
		return func(cli *CLIOptions) (*string, *string, bool) { return cli.CderunIPC, cli.IPC, true },
			func(res *ResolvedConfig, v string) { res.IPC = v }, true
	case "gpus":
		return func(cli *CLIOptions) (*string, *string, bool) { return cli.CderunGPUs, cli.GPUs, true },
			func(res *ResolvedConfig, v string) { res.GPUs = v }, true
	case "cgroupns":
		return func(cli *CLIOptions) (*string, *string, bool) { return cli.CderunCgroupns, cli.Cgroupns, true },
			func(res *ResolvedConfig, v string) { res.Cgroupns = v }, true
	case "cpuset-cpus":
		return func(cli *CLIOptions) (*string, *string, bool) { return cli.CderunCpusetCpus, cli.CpusetCpus, true },
			func(res *ResolvedConfig, v string) { res.CpusetCpus = v }, true
	case "cpuset-mems":
		return func(cli *CLIOptions) (*string, *string, bool) { return cli.CderunCpusetMems, cli.CpusetMems, true },
			func(res *ResolvedConfig, v string) { res.CpusetMems = v }, true
	case "restart":
		return func(cli *CLIOptions) (*string, *string, bool) { return cli.CderunRestart, cli.Restart, true },
			func(res *ResolvedConfig, v string) { res.Restart = v }, true
	case "prefetch":
		return func(cli *CLIOptions) (*string, *string, bool) { return cli.CderunPrefetch, cli.Prefetch, true },
			func(res *ResolvedConfig, v string) { res.Prefetch = v }, true
	case "oci-runtime":
		return func(cli *CLIOptions) (*string, *string, bool) { return cli.CderunOciRuntime, cli.OciRuntime, true },
			func(res *ResolvedConfig, v string) { res.OciRuntime = v }, true
	default:
		return nil, nil, false
	}
}

func getBoolOptionFastPathPtrsAndAssign(name string) (func(*CLIOptions) (*bool, *bool, bool), func(*ResolvedConfig, bool), bool) {
	switch name {
	case "tty":
		return func(cli *CLIOptions) (*bool, *bool, bool) { return cli.CderunTTY, cli.TTY, true },
			func(res *ResolvedConfig, v bool) { res.TTY = v }, true
	case "interactive":
		return func(cli *CLIOptions) (*bool, *bool, bool) { return cli.CderunInteractive, cli.Interactive, true },
			func(res *ResolvedConfig, v bool) { res.Interactive = v }, true
	case "read-only":
		return func(cli *CLIOptions) (*bool, *bool, bool) { return cli.CderunReadOnly, cli.ReadOnly, true },
			func(res *ResolvedConfig, v bool) { res.ReadOnly = v }, true
	case "init":
		return func(cli *CLIOptions) (*bool, *bool, bool) { return cli.CderunInit, cli.Init, true },
			func(res *ResolvedConfig, v bool) { res.Init = v }, true
	case "remove":
		return func(cli *CLIOptions) (*bool, *bool, bool) { return cli.CderunRemove, cli.Remove, true },
			func(res *ResolvedConfig, v bool) { res.Remove = v }, true
	case "diagnosis":
		return func(cli *CLIOptions) (*bool, *bool, bool) { return cli.CderunDiagnosis, cli.Diagnosis, true },
			func(res *ResolvedConfig, v bool) { res.Diagnosis = v }, true
	case "strict-env":
		return func(cli *CLIOptions) (*bool, *bool, bool) { return cli.CderunStrictEnv, cli.StrictEnv, true },
			func(res *ResolvedConfig, v bool) { res.StrictEnv = v }, true
	case "privileged":
		return func(cli *CLIOptions) (*bool, *bool, bool) { return cli.CderunPrivileged, cli.Privileged, true },
			func(res *ResolvedConfig, v bool) { res.Privileged = v }, true
	case "publish-all":
		return func(cli *CLIOptions) (*bool, *bool, bool) { return cli.CderunPublishAll, cli.PublishAll, true },
			func(res *ResolvedConfig, v bool) { res.PublishAll = v }, true
	case "log-timestamp":
		return func(cli *CLIOptions) (*bool, *bool, bool) { return cli.CderunLogTimestamp, cli.LogTimestamp, true },
			func(res *ResolvedConfig, v bool) { res.LogTimestamp = v }, true
	case "mount-socket":
		return func(cli *CLIOptions) (*bool, *bool, bool) { return cli.CderunMountSocket, cli.MountSocket, true },
			func(res *ResolvedConfig, v bool) { res.MountSocket = v }, true
	case "mount-cderun":
		return func(cli *CLIOptions) (*bool, *bool, bool) { return cli.CderunMountCderun, cli.MountCderun, true },
			func(res *ResolvedConfig, v bool) { res.MountCderun = v }, true
	case "mount-all-tools":
		return func(cli *CLIOptions) (*bool, *bool, bool) { return cli.CderunMountAllTools, cli.MountAllTools, true },
			func(res *ResolvedConfig, v bool) { res.MountAllTools = v }, true
	case "dry-run":
		return func(cli *CLIOptions) (*bool, *bool, bool) { return cli.CderunDryRun, cli.DryRun, true },
			func(res *ResolvedConfig, v bool) { res.DryRun = v }, true
	case "prefetch-all":
		return func(cli *CLIOptions) (*bool, *bool, bool) { return cli.CderunPrefetchAll, cli.PrefetchAll, true },
			func(res *ResolvedConfig, v bool) { res.PrefetchAll = v }, true
	case "mount-cderun-socket":
		return func(cli *CLIOptions) (*bool, *bool, bool) { return cli.CderunMountCderunSocket, cli.MountCderunSocket, true },
			func(res *ResolvedConfig, v bool) { res.MountCderunSocket = v }, true
	case "prune":
		return func(cli *CLIOptions) (*bool, *bool, bool) { return cli.CderunPrune, cli.Prune, true },
			func(res *ResolvedConfig, v bool) { res.Prune = v }, true
	default:
		return nil, nil, false
	}
}

func getStringSliceOptionFastPathSlicesAndAssign(name string) (func(*CLIOptions) ([]string, []string, bool), func(*ResolvedConfig, []string), bool) {
	switch name {
	case "publish":
		return func(cli *CLIOptions) ([]string, []string, bool) { return cli.CderunPorts, cli.Ports, true },
			func(res *ResolvedConfig, v []string) { res.Ports = v }, true
	case "expose":
		return func(cli *CLIOptions) ([]string, []string, bool) { return cli.CderunExpose, cli.Expose, true },
			func(res *ResolvedConfig, v []string) { res.Expose = v }, true
	case "dns":
		return func(cli *CLIOptions) ([]string, []string, bool) { return cli.CderunDNS, cli.DNS, true },
			func(res *ResolvedConfig, v []string) { res.DNS = v }, true
	case "add-host":
		return func(cli *CLIOptions) ([]string, []string, bool) { return cli.CderunAddHosts, cli.AddHosts, true },
			func(res *ResolvedConfig, v []string) { res.AddHosts = v }, true
	case "group-add":
		return func(cli *CLIOptions) ([]string, []string, bool) { return cli.CderunGroupAdd, cli.GroupAdd, true },
			func(res *ResolvedConfig, v []string) { res.GroupAdd = v }, true
	case "cap-add":
		return func(cli *CLIOptions) ([]string, []string, bool) { return cli.CderunCapAdd, cli.CapAdd, true },
			func(res *ResolvedConfig, v []string) { res.CapAdd = v }, true
	case "cap-drop":
		return func(cli *CLIOptions) ([]string, []string, bool) { return cli.CderunCapDrop, cli.CapDrop, true },
			func(res *ResolvedConfig, v []string) { res.CapDrop = v }, true
	case "entrypoint":
		return func(cli *CLIOptions) ([]string, []string, bool) { return cli.CderunEntrypoint, cli.Entrypoint, true },
			func(res *ResolvedConfig, v []string) { res.Entrypoint = v }, true
	case "security-opt":
		return func(cli *CLIOptions) ([]string, []string, bool) { return cli.CderunSecurityOpt, cli.SecurityOpt, true },
			func(res *ResolvedConfig, v []string) { res.SecurityOpt = v }, true
	case "dns-search":
		return func(cli *CLIOptions) ([]string, []string, bool) { return cli.CderunDNSSearch, cli.DNSSearch, true },
			func(res *ResolvedConfig, v []string) { res.DNSSearch = v }, true
	case "dns-option":
		return func(cli *CLIOptions) ([]string, []string, bool) { return cli.CderunDNSOptions, cli.DNSOptions, true },
			func(res *ResolvedConfig, v []string) { res.DNSOptions = v }, true
	default:
		return nil, nil, false
	}
}

func getIntOptionFastPathPtrsAndAssign(name string) (func(*CLIOptions) (*int, *int, bool), func(*ResolvedConfig, int), bool) {
	switch name {
	case "pull-max-retries":
		return func(cli *CLIOptions) (*int, *int, bool) { return cli.CderunPullMaxRetries, cli.PullMaxRetries, true },
			func(res *ResolvedConfig, v int) { res.PullMaxRetries = v }, true
	case "pids-limit":
		return func(cli *CLIOptions) (*int, *int, bool) { return cli.CderunPidsLimit, cli.PidsLimit, true },
			func(res *ResolvedConfig, v int) { res.PidsLimit = v }, true
	case "cpu-shares":
		return func(cli *CLIOptions) (*int, *int, bool) { return cli.CderunCPUShares, cli.CPUShares, true },
			func(res *ResolvedConfig, v int) { res.CPUShares = v }, true
	default:
		return nil, nil, false
	}
}

func getFloat64OptionFastPathPtrsAndAssign(name string) (func(*CLIOptions) (*float64, *float64, bool), func(*ResolvedConfig, float64), bool) {
	if name == "cpus" {
		return func(cli *CLIOptions) (*float64, *float64, bool) { return cli.CderunCPUs, cli.CPUs, true },
			func(res *ResolvedConfig, v float64) { res.CPUs = v }, true
	}
	return nil, nil, false
}

func (rv *resolver) applyStringSliceOption(opt StringSliceOption) error {
	var p1v, p2v []string
	var fastPathUsed bool
	if opt.fastPathGet != nil {
		p1v, p2v, fastPathUsed = opt.fastPathGet(rv.cli)
	}

	if fastPathUsed && isDriftOk(opt.Name, opt.info) {
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
		opt.fastPathAssign(rv.res, resolved)
		return nil
	}

	info, err := resolveOptionFieldInfo(opt.Name, opt.info)
	if err != nil {
		return err
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
	var p1Ptr, p2Ptr *string
	var fastPathUsed bool
	if opt.fastPathGet != nil {
		p1Ptr, p2Ptr, fastPathUsed = opt.fastPathGet(rv.cli)
	}
	p1Set, p1Val := getPtrVal(p1Ptr)
	p2Set, p2Val := getPtrVal(p2Ptr)

	if fastPathUsed && isDriftOk(opt.Name, opt.info) {
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
		opt.fastPathAssign(rv.res, resolved)
		return nil
	}

	info, err := resolveOptionFieldInfo(opt.Name, opt.info)
	if err != nil {
		return err
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
	var p1Ptr, p2Ptr *bool
	var fastPathUsed bool
	if opt.fastPathGet != nil {
		p1Ptr, p2Ptr, fastPathUsed = opt.fastPathGet(rv.cli)
	}
	p1Set, p1Val := getPtrVal(p1Ptr)
	p2Set, p2Val := getPtrVal(p2Ptr)

	if fastPathUsed && isDriftOk(opt.Name, opt.info) {
		def := OptionDef[*bool]{
			EnvKey:       opt.EnvKey,
			ToolGetter:   opt.ToolGetter,
			GlobalGetter: opt.GlobalGetter,
		}
		resolved, err := resolveBoolOpt(def, opt.Default, p1Set, p1Val, p2Set, p2Val, rv.subcommand, rv.tools, rv.global, rv.fs)
		if err != nil {
			return err
		}
		opt.fastPathAssign(rv.res, resolved)
		return nil
	}

	info, err := resolveOptionFieldInfo(opt.Name, opt.info)
	if err != nil {
		return err
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
	var p1Ptr, p2Ptr *int
	var fastPathUsed bool
	if opt.fastPathGet != nil {
		p1Ptr, p2Ptr, fastPathUsed = opt.fastPathGet(rv.cli)
	}
	p1Set, p1Int := getPtrVal(p1Ptr)
	p2Set, p2Int := getPtrVal(p2Ptr)

	if fastPathUsed && isDriftOk(opt.Name, opt.info) {
		def := OptionDef[*int]{
			EnvKey:       opt.EnvKey,
			ToolGetter:   opt.ToolGetter,
			GlobalGetter: opt.GlobalGetter,
		}
		resolved, err := resolveIntOpt(def, opt.Default, p1Set, p1Int, p2Set, p2Int, rv.subcommand, rv.tools, rv.global, rv.fs)
		if err != nil {
			return err
		}
		opt.fastPathAssign(rv.res, resolved)
		return nil
	}

	info, err := resolveOptionFieldInfo(opt.Name, opt.info)
	if err != nil {
		return err
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
	var p1Ptr, p2Ptr *float64
	var fastPathUsed bool
	if opt.fastPathGet != nil {
		p1Ptr, p2Ptr, fastPathUsed = opt.fastPathGet(rv.cli)
	}
	p1Set, p1Float := getPtrVal(p1Ptr)
	p2Set, p2Float := getPtrVal(p2Ptr)

	if fastPathUsed && isDriftOk(opt.Name, opt.info) {
		def := OptionDef[*float64]{
			EnvKey:       opt.EnvKey,
			ToolGetter:   opt.ToolGetter,
			GlobalGetter: opt.GlobalGetter,
		}
		resolved, err := resolveFloat64Opt(def, opt.Default, p1Set, p1Float, p2Set, p2Float, rv.subcommand, rv.tools, rv.global, rv.fs)
		if err != nil {
			return err
		}
		opt.fastPathAssign(rv.res, resolved)
		return nil
	}

	info, err := resolveOptionFieldInfo(opt.Name, opt.info)
	if err != nil {
		return err
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
