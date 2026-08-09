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
	case "init":
		p1Set, p1Val = getPtrVal(rv.cli.CderunInit)
		p2Set, p2Val = getPtrVal(rv.cli.Init)
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
		case "init":
			rv.res.Init = resolved
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

func (rv *resolver) applyShmSizeOption(opt StringOption, target *int64) error {
	def := OptionDef[string]{
		EnvKey:       opt.EnvKey,
		ToolGetter:   opt.ToolGetter,
		GlobalGetter: opt.GlobalGetter,
		Fallback:     opt.Default,
	}

	var p1Set, p2Set bool
	var p1Val, p2Val string
	switch opt.Name {
	case "shm-size":
		p1Set, p1Val = getPtrVal(rv.cli.CderunShmSize)
		p2Set, p2Val = getPtrVal(rv.cli.ShmSize)
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
