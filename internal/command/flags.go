package command

import (
	"fmt"

	"cderun/internal/config"

	"github.com/spf13/cobra"
)

func registerFlags(cmd *cobra.Command, o *rootOptions) {
	f := cmd.PersistentFlags()

	for _, opt := range config.BoolOptions {
		p2Field, p1Field := getBoolPointers(o, opt.Name)
		if p2Field == nil || p1Field == nil {
			panic(fmt.Sprintf("could not find fields for bool option %q", opt.Name))
		}
		if opt.Shorthand != "" {
			f.BoolVarP(p2Field, opt.Name, opt.Shorthand, opt.Default, opt.Usage)
		} else {
			f.BoolVar(p2Field, opt.Name, opt.Default, opt.Usage)
		}
		f.BoolVar(p1Field, "cderun-"+opt.Name, opt.Default, "Override "+opt.Name+" setting (highest priority, can be used after subcommand)")
	}

	for _, opt := range config.StringOptions {
		p2Field, p1Field := getStringPointers(o, opt.Name)
		if p2Field == nil || p1Field == nil {
			panic(fmt.Sprintf("could not find fields for string option %q", opt.Name))
		}
		if opt.Shorthand != "" {
			f.StringVarP(p2Field, opt.Name, opt.Shorthand, opt.Default, opt.Usage)
		} else {
			f.StringVar(p2Field, opt.Name, opt.Default, opt.Usage)
		}
		f.StringVar(p1Field, "cderun-"+opt.Name, "", "Override "+opt.Name+" setting (highest priority, can be used after subcommand)")
	}

	for _, opt := range config.IntOptions {
		p2Field, p1Field := getIntPointers(o, opt.Name)
		if p2Field == nil || p1Field == nil {
			panic(fmt.Sprintf("could not find fields for int option %q", opt.Name))
		}
		if opt.Shorthand != "" {
			f.IntVarP(p2Field, opt.Name, opt.Shorthand, opt.Default, opt.Usage)
		} else {
			f.IntVar(p2Field, opt.Name, opt.Default, opt.Usage)
		}
		f.IntVar(p1Field, "cderun-"+opt.Name, 0, "Override "+opt.Name+" setting (highest priority, can be used after subcommand)")
	}

	for _, opt := range config.Float64Options {
		p2Field, p1Field := getFloat64Pointers(o, opt.Name)
		if p2Field == nil || p1Field == nil {
			panic(fmt.Sprintf("could not find fields for float64 option %q", opt.Name))
		}
		if opt.Shorthand != "" {
			f.Float64VarP(p2Field, opt.Name, opt.Shorthand, opt.Default, opt.Usage)
		} else {
			f.Float64Var(p2Field, opt.Name, opt.Default, opt.Usage)
		}
		f.Float64Var(p1Field, "cderun-"+opt.Name, 0, "Override "+opt.Name+" setting (highest priority, can be used after subcommand)")
	}

	for _, opt := range config.StringSliceOptions {
		p2Field, p1Field := getStringSlicePointers(o, opt.Name)
		if p2Field == nil || p1Field == nil {
			panic(fmt.Sprintf("could not find fields for string slice option %q", opt.Name))
		}
		if opt.Shorthand != "" {
			f.StringArrayVarP(p2Field, opt.Name, opt.Shorthand, nil, opt.Usage)
		} else {
			f.StringArrayVar(p2Field, opt.Name, nil, opt.Usage)
		}
		f.StringArrayVar(p1Field, "cderun-"+opt.Name, nil, "Override "+opt.Name+" setting (highest priority, can be used after subcommand)")
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
	case "publish":
		return &o.ports, &o.cderunPorts
	case "expose":
		return &o.expose, &o.cderunExpose
	case "dns":
		return &o.dns, &o.cderunDNS
	case "add-host":
		return &o.addHosts, &o.cderunAddHosts
	case "cap-add":
		return &o.capAdd, &o.cderunCapAdd
	case "cap-drop":
		return &o.capDrop, &o.cderunCapDrop
	case "entrypoint":
		return &o.entrypoint, &o.cderunEntrypoint
	case "env":
		return &o.env, &o.cderunEnv
	case "mount":
		return &o.mounts, &o.cderunMounts
	case "device":
		return &o.devices, &o.cderunDevices
	default:
		return nil, nil
	}
}
