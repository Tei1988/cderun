package command

import (
	"fmt"

	"cderun/internal/config"

	"github.com/spf13/cobra"
)

type boolFlagDef struct {
	p2Name, p1Name string
	p2Short        string
	defaultVal     bool
	p2Usage        string
	p2Field        *bool
	p1Field        *bool
}

type stringFlagDef struct {
	p2Name, p1Name string
	p2Short        string
	defaultVal     string
	p2Usage        string
	p2Field        *string
	p1Field        *string
}

type stringSliceFlagDef struct {
	p2Name, p1Name string
	p2Short        string
	p2Usage        string
	p2Field        *[]string
	p1Field        *[]string
}

type intFlagDef struct {
	p2Name, p1Name string
	p2Short        string
	defaultVal     int
	p2Usage        string
	p2Field        *int
	p1Field        *int
}

type float64FlagDef struct {
	p2Name, p1Name string
	p2Short        string
	defaultVal     float64
	p2Usage        string
	p2Field        *float64
	p1Field        *float64
}

func registerFlags(cmd *cobra.Command, o *rootOptions) {
	boolDefs := make([]boolFlagDef, 0, len(config.BoolOptions))
	for _, opt := range config.BoolOptions {
		p2Field, p1Field := getBoolPointers(o, opt.Name)
		if p2Field == nil || p1Field == nil {
			panic(fmt.Sprintf("could not find fields for bool option %q", opt.Name))
		}
		boolDefs = append(boolDefs, boolFlagDef{
			p2Name:     opt.Name,
			p1Name:     "cderun-" + opt.Name,
			p2Short:    opt.Shorthand,
			defaultVal: opt.Default,
			p2Usage:    opt.Usage,
			p2Field:    p2Field,
			p1Field:    p1Field,
		})
	}

	stringDefs := make([]stringFlagDef, 0, len(config.StringOptions))
	for _, opt := range config.StringOptions {
		p2Field, p1Field := getStringPointers(o, opt.Name)
		if p2Field == nil || p1Field == nil {
			panic(fmt.Sprintf("could not find fields for string option %q", opt.Name))
		}
		stringDefs = append(stringDefs, stringFlagDef{
			p2Name:     opt.Name,
			p1Name:     "cderun-" + opt.Name,
			p2Short:    opt.Shorthand,
			defaultVal: opt.Default,
			p2Usage:    opt.Usage,
			p2Field:    p2Field,
			p1Field:    p1Field,
		})
	}

	intDefs := make([]intFlagDef, 0, len(config.IntOptions))
	for _, opt := range config.IntOptions {
		p2Field, p1Field := getIntPointers(o, opt.Name)
		if p2Field == nil || p1Field == nil {
			panic(fmt.Sprintf("could not find fields for int option %q", opt.Name))
		}
		intDefs = append(intDefs, intFlagDef{
			p2Name:     opt.Name,
			p1Name:     "cderun-" + opt.Name,
			p2Short:    opt.Shorthand,
			defaultVal: opt.Default,
			p2Usage:    opt.Usage,
			p2Field:    p2Field,
			p1Field:    p1Field,
		})
	}

	float64Defs := make([]float64FlagDef, 0, len(config.Float64Options))
	for _, opt := range config.Float64Options {
		p2Field, p1Field := getFloat64Pointers(o, opt.Name)
		if p2Field == nil || p1Field == nil {
			panic(fmt.Sprintf("could not find fields for float64 option %q", opt.Name))
		}
		float64Defs = append(float64Defs, float64FlagDef{
			p2Name:     opt.Name,
			p1Name:     "cderun-" + opt.Name,
			p2Short:    opt.Shorthand,
			defaultVal: opt.Default,
			p2Usage:    opt.Usage,
			p2Field:    p2Field,
			p1Field:    p1Field,
		})
	}

	stringSliceDefs := make([]stringSliceFlagDef, 0, len(config.StringSliceOptions))
	for _, opt := range config.StringSliceOptions {
		p2Field, p1Field := getStringSlicePointers(o, opt.Name)
		if p2Field == nil || p1Field == nil {
			panic(fmt.Sprintf("could not find fields for string slice option %q", opt.Name))
		}
		stringSliceDefs = append(stringSliceDefs, stringSliceFlagDef{
			p2Name:  opt.Name,
			p1Name:  "cderun-" + opt.Name,
			p2Short: opt.Shorthand,
			p2Usage: opt.Usage,
			p2Field: p2Field,
			p1Field: p1Field,
		})
	}

	f := cmd.PersistentFlags()
	for _, d := range boolDefs {
		if d.p2Short != "" {
			f.BoolVarP(d.p2Field, d.p2Name, d.p2Short, d.defaultVal, d.p2Usage)
		} else {
			f.BoolVar(d.p2Field, d.p2Name, d.defaultVal, d.p2Usage)
		}
		f.BoolVar(d.p1Field, d.p1Name, d.defaultVal, "Override "+d.p2Name+" setting (highest priority, can be used after subcommand)")
	}
	for _, d := range stringDefs {
		if d.p2Short != "" {
			f.StringVarP(d.p2Field, d.p2Name, d.p2Short, d.defaultVal, d.p2Usage)
		} else {
			f.StringVar(d.p2Field, d.p2Name, d.defaultVal, d.p2Usage)
		}
		f.StringVar(d.p1Field, d.p1Name, "", "Override "+d.p2Name+" setting (highest priority, can be used after subcommand)")
	}
	for _, d := range intDefs {
		if d.p2Short != "" {
			f.IntVarP(d.p2Field, d.p2Name, d.p2Short, d.defaultVal, d.p2Usage)
		} else {
			f.IntVar(d.p2Field, d.p2Name, d.defaultVal, d.p2Usage)
		}
		f.IntVar(d.p1Field, d.p1Name, 0, "Override "+d.p2Name+" setting (highest priority, can be used after subcommand)")
	}
	for _, d := range float64Defs {
		if d.p2Short != "" {
			f.Float64VarP(d.p2Field, d.p2Name, d.p2Short, d.defaultVal, d.p2Usage)
		} else {
			f.Float64Var(d.p2Field, d.p2Name, d.defaultVal, d.p2Usage)
		}
		f.Float64Var(d.p1Field, d.p1Name, 0, "Override "+d.p2Name+" setting (highest priority, can be used after subcommand)")
	}
	for _, d := range stringSliceDefs {
		if d.p2Short != "" {
			f.StringArrayVarP(d.p2Field, d.p2Name, d.p2Short, nil, d.p2Usage)
		} else {
			f.StringArrayVar(d.p2Field, d.p2Name, nil, d.p2Usage)
		}
		f.StringArrayVar(d.p1Field, d.p1Name, nil, "Override "+d.p2Name+" setting (highest priority, can be used after subcommand)")
	}
}

func getBoolPointers(o *rootOptions, name string) (p2, p1 *bool) {
	switch name {
	case "tty":
		return &o.tty, &o.cderunTTY
	case "interactive":
		return &o.interactive, &o.cderunInteractive
	case "mount-socket":
		return &o.mountSocket, &o.cderunMountSocket
	case "mount-cderun":
		return &o.mountCderun, &o.cderunMountCderun
	case "mount-all-tools":
		return &o.mountAllTools, &o.cderunMountAllTools
	case "remove":
		return &o.remove, &o.cderunRemove
	case "publish-all":
		return &o.publishAll, &o.cderunPublishAll
	case "privileged":
		return &o.privileged, &o.cderunPrivileged
	case "strict-env":
		return &o.strictEnv, &o.cderunStrictEnv
	case "dry-run":
		return &o.dryRun, &o.cderunDryRun
	case "diagnosis":
		return &o.diagnosis, &o.cderunDiagnosis
	case "log-timestamp":
		return &o.logTimestamp, &o.cderunLogTimestamp
	case "read-only":
		return &o.readOnly, &o.cderunReadOnly
	default:
		return nil, nil
	}
}

func getIntPointers(o *rootOptions, name string) (p2, p1 *int) {
	switch name {
	case "pull-max-retries":
		return &o.pullMaxRetries, &o.cderunPullMaxRetries
	default:
		return nil, nil
	}
}

func getFloat64Pointers(o *rootOptions, name string) (p2, p1 *float64) {
	switch name {
	case "cpus":
		return &o.cpus, &o.cderunCPUs
	default:
		return nil, nil
	}
}

func getStringSlicePointers(o *rootOptions, name string) (p2, p1 *[]string) {
	switch name {
	case "env":
		return &o.env, &o.cderunEnv
	case "mount":
		return &o.mounts, &o.cderunMounts
	case "publish":
		return &o.ports, &o.cderunPorts
	case "expose":
		return &o.expose, &o.cderunExpose
	case "dns":
		return &o.dns, &o.cderunDNS
	case "add-host":
		return &o.addHosts, &o.cderunAddHosts
	case "group-add":
		return &o.groupAdd, &o.cderunGroupAdd
	case "cap-add":
		return &o.capAdd, &o.cderunCapAdd
	case "cap-drop":
		return &o.capDrop, &o.cderunCapDrop
	case "entrypoint":
		return &o.entrypoint, &o.cderunEntrypoint
	case "device":
		return &o.devices, &o.cderunDevices
	case "sensitive-env":
		return &o.sensitiveEnv, &o.cderunSensitiveEnv
	default:
		return nil, nil
	}
}

func getStringPointers(o *rootOptions, name string) (p2, p1 *string) {
	switch name {
	case "network":
		return &o.network, &o.cderunNetwork
	case "socket-path":
		return &o.socketPath, &o.cderunSocketPath
	case "mount-socket-path":
		return &o.mountSocketPath, &o.cderunMountSocketPath
	case "mount-cderun-path":
		return &o.mountCderunPath, &o.cderunMountCderunPath
	case "image":
		return &o.image, &o.cderunImage
	case "runtime":
		return &o.runtimeName, &o.cderunRuntime
	case "workdir":
		return &o.workdir, &o.cderunWorkdir
	case "mount-tools":
		return &o.mountTools, &o.cderunMountTools
	case "config":
		return &o.configPath, &o.cderunConfigPath
	case "tool-config":
		return &o.toolConfigPath, &o.cderunToolConfigPath
	case "hostname":
		return &o.hostname, &o.cderunHostname
	case "user":
		return &o.user, &o.cderunUser
	case "pull":
		return &o.pull, &o.cderunPull
	case "pull-backoff-base":
		return &o.pullBackoffBase, &o.cderunPullBackoffBase
	case "memory":
		return &o.memory, &o.cderunMemory
	case "dry-run-format":
		return &o.dryRunFormat, &o.cderunDryRunFormat
	case "diagnosis-format":
		return &o.diagnosisFormat, &o.cderunDiagnosisFormat
	case "log-level":
		return &o.logLevel, &o.cderunLogLevel
	case "log-format":
		return &o.logFormat, &o.cderunLogFormat
	case "hang-timeout":
		return &o.hangTimeout, &o.cderunHangTimeout
	default:
		return nil, nil
	}
}
