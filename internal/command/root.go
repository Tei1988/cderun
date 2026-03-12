package command

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"cderun/internal/config"
	"cderun/internal/container"
	"cderun/internal/logging"
	"cderun/internal/runtime"
	"cderun/internal/version"

	"github.com/docker/go-units"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"gopkg.in/yaml.v3"
)

type rootOptions struct {
	tty                   bool
	interactive           bool
	network               string
	socketPath            string
	mountSocket           bool
	mountSocketPath       string
	mountCderun           bool
	mountCderunPath       string
	image                 string
	remove                bool
	cderunTTY             bool
	cderunInteractive     bool
	cderunImage           string
	cderunNetwork         string
	cderunRemove          bool
	cderunRuntime         string
	cderunSocketPath      string
	cderunMountSocket     bool
	cderunMountSocketPath string
	cderunWorkdir         string
	cderunMounts          []string
	cderunMountCderun     bool
	cderunMountCderunPath string
	cderunMountTools      string
	cderunMountAllTools   bool
	runtimeName           string
	env                   []string
	cderunEnv             []string
	workdir               string
	mounts                []string
	mountTools            string
	mountAllTools         bool
	configPath            string
	cderunConfigPath      string
	toolConfigPath        string
	cderunToolConfigPath  string
	dryRun                bool
	dryRunFormat          string
	cderunDryRun          bool
	cderunDryRunFormat    string
	diagnosis             bool
	diagnosisFormat       string
	cderunDiagnosis       bool
	cderunDiagnosisFormat string
	logLevel              string
	logFormat             string
	logTimestamp          bool
	strictEnv             bool
	cderunStrictEnv       bool
	cderunLogLevel        string
	cderunLogFormat       string
	cderunLogTimestamp    bool
	hangTimeout           string
	cderunHangTimeout     string

	// Docker-compatible flags
	ports            []string
	publishAll       bool
	expose           []string
	hostname         string
	dns              []string
	addHosts         []string
	user             string
	privileged       bool
	capAdd           []string
	capDrop          []string
	entrypoint       []string
	pull             string
	memory           string
	cpus             float64
	devices          []string
	cderunPorts      []string
	cderunPublishAll bool
	cderunExpose     []string
	cderunHostname   string
	cderunDNS        []string
	cderunAddHosts   []string
	cderunUser       string
	cderunPrivileged bool
	cderunCapAdd     []string
	cderunCapDrop    []string
	cderunEntrypoint []string
	cderunPull       string
	cderunMemory     string
	cderunCPUs       float64
	cderunDevices    []string

	// Dependencies
	fs           config.FileSystem
	configLoader *config.ConfigLoader
	logger       *logging.Logger

	// Testing hooks
	exitFunc       func(int)
	isTerminal     func(int) bool
	termGetSize    func(int) (int, int, error)
	makeRaw        func(int) (*term.State, error)
	restore        func(int, *term.State) error
	runtimeFactory func(string, string) (runtime.ContainerRuntime, error)
}

const (
	attachGracePeriod = 5 * time.Second
	hangTimeout       = 2 * time.Second
)

var (
	opts = defaultOptions()

	// rootCmd is initialized in init() to ensure it uses the properly initialized opts
	rootCmd *cobra.Command
)

func defaultOptions() rootOptions {
	return rootOptions{
		fs: config.RealFileSystem{},
		exitFunc: func(code int) {
			// Default to no-op for safety in tests.
			// The global 'opts' is updated in init() to use os.Exit.
		},
		isTerminal: func(fd int) bool {
			return term.IsTerminal(fd)
		},
		termGetSize: func(fd int) (int, int, error) {
			return term.GetSize(fd)
		},
		makeRaw: func(fd int) (*term.State, error) {
			return term.MakeRaw(fd)
		},
		restore: func(fd int, state *term.State) error {
			return term.Restore(fd, state)
		},
		logger: logging.GetGlobalLogger(),
		runtimeFactory: func(name string, socket string) (runtime.ContainerRuntime, error) {
			switch name {
			case "docker":
				return runtime.NewDockerRuntime(socket)
			case "podman":
				return runtime.NewPodmanRuntime(socket)
			default:
				return nil, fmt.Errorf("unsupported runtime %q", name)
			}
		},
	}
}

func (o *rootOptions) loadConfigs(cmd *cobra.Command) (config.ToolsConfig, *config.CDERunConfig, []string, []string, error) {
	o.logger.Trace("Loading configurations...")

	// Determine CDERun config path
	cderunPath := ""
	if cmd.Flags().Changed("cderun-config") {
		cderunPath = o.cderunConfigPath
	} else if cmd.Flags().Changed("config") {
		cderunPath = o.configPath
	} else if env := o.fs.Getenv("CDERUN_CONFIG"); env != "" {
		cderunPath = env
	}

	var globalCfg *config.CDERunConfig
	var globalPaths []string
	var err error
	if cderunPath != "" {
		globalCfg, globalPaths, err = o.configLoader.LoadCDERunConfigFromPath(cderunPath)
	} else {
		globalCfg, globalPaths, err = o.configLoader.LoadCDERunConfig()
	}

	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to load cderun config: %w", err)
	} else if len(globalPaths) > 0 {
		o.logger.Debug("Loaded cderun config from: %s", strings.Join(globalPaths, ", "))
	}

	// Determine Tools config path
	toolsPath := ""
	if cmd.Flags().Changed("cderun-tool-config") {
		toolsPath = o.cderunToolConfigPath
	} else if cmd.Flags().Changed("tool-config") {
		toolsPath = o.toolConfigPath
	} else if env := o.fs.Getenv("CDERUN_TOOL_CONFIG"); env != "" {
		toolsPath = env
	}

	var toolsCfg config.ToolsConfig
	var toolsPaths []string
	if toolsPath != "" {
		toolsCfg, toolsPaths, err = o.configLoader.LoadToolsConfigFromPath(toolsPath)
	} else {
		toolsCfg, toolsPaths, err = o.configLoader.LoadToolsConfig()
	}

	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to load tools config: %w", err)
	} else if len(toolsPaths) > 0 {
		o.logger.Debug("Loaded tools config from: %s", strings.Join(toolsPaths, ", "))
	}
	return toolsCfg, globalCfg, globalPaths, toolsPaths, nil
}

func (o *rootOptions) resolveSettings(cmd *cobra.Command, subcommand string, toolsCfg config.ToolsConfig, globalCfg *config.CDERunConfig) (*config.ResolvedConfig, error) {
	cliOpts := config.CLIOptions{
		Image:                    o.image,
		ImageSet:                 cmd.Flags().Changed("image"),
		TTY:                      o.tty,
		TTYSet:                   cmd.Flags().Changed("tty"),
		Interactive:              o.interactive,
		InteractiveSet:           cmd.Flags().Changed("interactive"),
		Network:                  o.network,
		NetworkSet:               cmd.Flags().Changed("network"),
		CderunNetwork:            o.cderunNetwork,
		CderunNetworkSet:         cmd.Flags().Changed("cderun-network"),
		Remove:                   o.remove,
		RemoveSet:                cmd.Flags().Changed("remove"),
		CderunRemove:             o.cderunRemove,
		CderunRemoveSet:          cmd.Flags().Changed("cderun-remove"),
		CderunTTY:                o.cderunTTY,
		CderunTTYSet:             cmd.Flags().Changed("cderun-tty"),
		CderunInteractive:        o.cderunInteractive,
		CderunInteractiveSet:     cmd.Flags().Changed("cderun-interactive"),
		CderunImage:              o.cderunImage,
		CderunImageSet:           cmd.Flags().Changed("cderun-image"),
		Runtime:                  o.runtimeName,
		RuntimeSet:               cmd.Flags().Changed("runtime"),
		CderunRuntime:            o.cderunRuntime,
		CderunRuntimeSet:         cmd.Flags().Changed("cderun-runtime"),
		SocketPath:               o.socketPath,
		SocketPathSet:            cmd.Flags().Changed("socket-path"),
		CderunSocketPath:         o.cderunSocketPath,
		CderunSocketPathSet:      cmd.Flags().Changed("cderun-socket-path"),
		MountSocket:              o.mountSocket,
		MountSocketSet:           cmd.Flags().Changed("mount-socket"),
		CderunMountSocket:        o.cderunMountSocket,
		CderunMountSocketSet:     cmd.Flags().Changed("cderun-mount-socket"),
		MountSocketPath:          o.mountSocketPath,
		MountSocketPathSet:       cmd.Flags().Changed("mount-socket-path"),
		CderunMountSocketPath:    o.cderunMountSocketPath,
		CderunMountSocketPathSet: cmd.Flags().Changed("cderun-mount-socket-path"),
		Env:                      o.env,
		CderunEnv:                o.cderunEnv,
		Workdir:                  o.workdir,
		WorkdirSet:               cmd.Flags().Changed("workdir"),
		CderunWorkdir:            o.cderunWorkdir,
		CderunWorkdirSet:         cmd.Flags().Changed("cderun-workdir"),
		Mounts:                   o.mounts,
		CderunMounts:             o.cderunMounts,
		MountCderun:              o.mountCderun,
		MountCderunSet:           cmd.Flags().Changed("mount-cderun"),
		CderunMountCderun:        o.cderunMountCderun,
		CderunMountCderunSet:     cmd.Flags().Changed("cderun-mount-cderun"),
		MountCderunPath:          o.mountCderunPath,
		MountCderunPathSet:       cmd.Flags().Changed("mount-cderun-path"),
		CderunMountCderunPath:    o.cderunMountCderunPath,
		CderunMountCderunPathSet: cmd.Flags().Changed("cderun-mount-cderun-path"),
		MountTools:               o.mountTools,
		MountToolsSet:            cmd.Flags().Changed("mount-tools"),
		CderunMountTools:         o.cderunMountTools,
		CderunMountToolsSet:      cmd.Flags().Changed("cderun-mount-tools"),
		MountAllTools:            o.mountAllTools,
		MountAllToolsSet:         cmd.Flags().Changed("mount-all-tools"),
		CderunMountAllTools:      o.cderunMountAllTools,
		CderunMountAllToolsSet:   cmd.Flags().Changed("cderun-mount-all-tools"),
		DryRun:                   o.dryRun,
		DryRunSet:                cmd.Flags().Changed("dry-run"),
		CderunDryRun:             o.cderunDryRun,
		CderunDryRunSet:          cmd.Flags().Changed("cderun-dry-run"),
		DryRunFormat:             o.dryRunFormat,
		DryRunFormatSet:          cmd.Flags().Changed("dry-run-format"),
		CderunDryRunFormat:       o.cderunDryRunFormat,
		CderunDryRunFormatSet:    cmd.Flags().Changed("cderun-dry-run-format"),
		Diagnosis:                o.diagnosis,
		DiagnosisSet:             cmd.Flags().Changed("diagnosis"),
		CderunDiagnosis:          o.cderunDiagnosis,
		CderunDiagnosisSet:       cmd.Flags().Changed("cderun-diagnosis"),
		DiagnosisFormat:          o.diagnosisFormat,
		DiagnosisFormatSet:       cmd.Flags().Changed("diagnosis-format"),
		CderunDiagnosisFormat:    o.cderunDiagnosisFormat,
		CderunDiagnosisFormatSet: cmd.Flags().Changed("cderun-diagnosis-format"),
		LogLevel:                 o.logLevel,
		LogLevelSet:              cmd.Flags().Changed("log-level"),
		LogFormat:                o.logFormat,
		LogFormatSet:             cmd.Flags().Changed("log-format"),
		LogTimestamp:             o.logTimestamp,
		LogTimestampSet:          cmd.Flags().Changed("log-timestamp"),
		CderunLogLevel:           o.cderunLogLevel,
		CderunLogLevelSet:        cmd.Flags().Changed("cderun-log-level"),
		CderunLogFormat:          o.cderunLogFormat,
		CderunLogFormatSet:       cmd.Flags().Changed("cderun-log-format"),
		CderunLogTimestamp:       o.cderunLogTimestamp,
		CderunLogTimestampSet:    cmd.Flags().Changed("cderun-log-timestamp"),
		StrictEnv:                o.strictEnv,
		StrictEnvSet:             cmd.Flags().Changed("strict-env"),
		CderunStrictEnv:          o.cderunStrictEnv,
		CderunStrictEnvSet:       cmd.Flags().Changed("cderun-strict-env"),
		HangTimeout:              o.hangTimeout,
		HangTimeoutSet:           cmd.Flags().Changed("hang-timeout"),
		CderunHangTimeout:        o.cderunHangTimeout,
		CderunHangTimeoutSet:     cmd.Flags().Changed("cderun-hang-timeout"),

		// Docker-compatible flags
		Ports:               o.ports,
		CderunPorts:         o.cderunPorts,
		PublishAll:          o.publishAll,
		PublishAllSet:       cmd.Flags().Changed("publish-all"),
		CderunPublishAll:    o.cderunPublishAll,
		CderunPublishAllSet: cmd.Flags().Changed("cderun-publish-all"),
		Expose:              o.expose,
		CderunExpose:        o.cderunExpose,
		Hostname:            o.hostname,
		HostnameSet:         cmd.Flags().Changed("hostname"),
		CderunHostname:      o.cderunHostname,
		CderunHostnameSet:   cmd.Flags().Changed("cderun-hostname"),
		DNS:                 o.dns,
		CderunDNS:           o.cderunDNS,
		AddHosts:            o.addHosts,
		CderunAddHosts:      o.cderunAddHosts,
		User:                o.user,
		UserSet:             cmd.Flags().Changed("user"),
		CderunUser:          o.cderunUser,
		CderunUserSet:       cmd.Flags().Changed("cderun-user"),
		Privileged:          o.privileged,
		PrivilegedSet:       cmd.Flags().Changed("privileged"),
		CderunPrivileged:    o.cderunPrivileged,
		CderunPrivilegedSet: cmd.Flags().Changed("cderun-privileged"),
		CapAdd:              o.capAdd,
		CderunCapAdd:        o.cderunCapAdd,
		CapDrop:             o.capDrop,
		CderunCapDrop:       o.cderunCapDrop,
		Entrypoint:          o.entrypoint,
		CderunEntrypoint:    o.cderunEntrypoint,
		Pull:                o.pull,
		PullSet:             cmd.Flags().Changed("pull"),
		CderunPull:          o.cderunPull,
		CderunPullSet:       cmd.Flags().Changed("cderun-pull"),
		Memory:              o.memory,
		MemorySet:           cmd.Flags().Changed("memory"),
		CderunMemory:        o.cderunMemory,
		CderunMemorySet:     cmd.Flags().Changed("cderun-memory"),
		CPUs:                o.cpus,
		CPUsSet:             cmd.Flags().Changed("cpus"),
		CderunCPUs:          o.cderunCPUs,
		CderunCPUsSet:       cmd.Flags().Changed("cderun-cpus"),
		Devices:             o.devices,
		CderunDevices:       o.cderunDevices,
	}

	return config.ResolveWithFS(subcommand, cliOpts, toolsCfg, globalCfg, o.fs)
}

func (o *rootOptions) buildContainerConfig(resolved *config.ResolvedConfig, passthroughArgs []string, toolsCfg config.ToolsConfig) (*container.ContainerConfig, error) {
	// Step 10.2: Container command assembly.
	// The subcommand itself is NOT included in fullCommand.
	// the passthrough arguments provided after the subcommand are used.
	var fullCommand []string
	if len(passthroughArgs) > 0 {
		fullCommand = append([]string{}, passthroughArgs...)
	}

	// Build ContainerConfig
	containerConfig := &container.ContainerConfig{
		Image:       resolved.Image,
		Command:     fullCommand,
		TTY:         resolved.TTY,
		Interactive: resolved.Interactive,
		Network:     resolved.Network,
		Remove:      resolved.Remove,
		Mounts:      resolved.Mounts,
		Env:         resolved.Env,
		Workdir:     resolved.Workdir,
		User:        resolved.User,

		// Docker-compatible flags
		Ports:      resolved.Ports,
		PublishAll: resolved.PublishAll,
		Expose:     resolved.Expose,
		Hostname:   resolved.Hostname,
		DNS:        resolved.DNS,
		AddHosts:   resolved.AddHosts,
		Privileged: resolved.Privileged,
		CapAdd:     resolved.CapAdd,
		CapDrop:    resolved.CapDrop,
		Entrypoint: resolved.Entrypoint,
		Pull:       resolved.Pull,
		Memory:     resolved.Memory,
		CPUs:       resolved.CPUs,
		Devices:    resolved.Devices,
	}

	// Handle mounting flags
	if resolved.MountCderun || resolved.MountAllTools || len(resolved.MountTools) > 0 {
		exePath := resolved.MountCderunPath
		if exePath == "" {
			var err error
			exePath, err = o.fs.Executable()
			if err != nil {
				return nil, fmt.Errorf("failed to get executable path: %w", err)
			}
		}

		// Translate exePath for nested execution if it was determined from os.Executable()
		// (MountCderunPath is already resolved during resolution if it came from config/flags)
		if resolved.MountCderunPath == "" && resolved.HostContext != nil && resolved.HostContext.Level > 0 {
			r, err := config.NewExpressionResolver(resolved.HostContext)
			if err != nil {
				o.logger.Debug("Failed to create expression resolver for nested execution (best-effort): %v. HostContext: %+v, exePath: %q", err, resolved.HostContext, exePath)
			} else {
				resolvedPath, err := config.ResolvePath(exePath, "", r)
				if err != nil {
					o.logger.Debug("Failed to resolve exePath for nested execution (best-effort): %v. exePath: %q, HostContext: %+v", err, exePath, resolved.HostContext)
				} else {
					exePath = resolvedPath
				}
			}
		}

		// Add binary mount
		containerConfig.Mounts = append(containerConfig.Mounts, container.Mount{
			Type:     "bind",
			Source:   exePath,
			Target:   "/usr/local/bin/cderun",
			ReadOnly: true,
		})

		// Handle MountTools / MountAllTools
		if resolved.MountAllTools {
			if len(toolsCfg) == 0 {
				o.logger.Warn("--mount-all-tools specified but no tools defined in .tools.yaml")
			}
			// Sort tool names to ensure deterministic mount order
			toolNames := make([]string, 0, len(toolsCfg))
			for name := range toolsCfg {
				toolNames = append(toolNames, name)
			}
			sort.Strings(toolNames)

			for _, toolName := range toolNames {
				containerConfig.Mounts = append(containerConfig.Mounts, container.Mount{
					Type:     "bind",
					Source:   exePath,
					Target:   "/usr/local/bin/" + toolName,
					ReadOnly: true,
				})
			}
		} else if len(resolved.MountTools) > 0 {
			for _, toolName := range resolved.MountTools {
				if _, ok := toolsCfg[toolName]; !ok {
					available := make([]string, 0, len(toolsCfg))
					for k := range toolsCfg {
						available = append(available, k)
					}
					sort.Strings(available)
					return nil, fmt.Errorf("tool %q not found in .tools.yaml\navailable tools: %s", toolName, strings.Join(available, ", "))
				}
				containerConfig.Mounts = append(containerConfig.Mounts, container.Mount{
					Type:     "bind",
					Source:   exePath,
					Target:   "/usr/local/bin/" + toolName,
					ReadOnly: true,
				})
			}
		}
	}

	if resolved.MountSocket {
		// Add socket mount
		containerConfig.Mounts = append(containerConfig.Mounts, container.Mount{
			Type:     "bind",
			Source:   resolved.SocketPath,
			Target:   resolved.MountSocketPath,
			ReadOnly: false, // Socket needs to be writable
		})
	}

	return containerConfig, nil
}

type diagnosticsInfo struct {
	Runtime struct {
		Name   string `json:"name" yaml:"name"`
		Socket string `json:"socket" yaml:"socket"`
		Status string `json:"status" yaml:"status"`
	} `json:"runtime" yaml:"runtime"`
	Configs struct {
		Global []string `json:"global" yaml:"global"`
		Tools  []string `json:"tools" yaml:"tools"`
	} `json:"configs" yaml:"configs"`
	AvailableTools []string `json:"available_tools,omitempty" yaml:"available_tools,omitempty"`
}

func (o *rootOptions) handleDiagnosis(cmd *cobra.Command, resolved *config.ResolvedConfig, toolsCfg config.ToolsConfig, globalPaths, toolsPaths []string) error {
	info := diagnosticsInfo{}
	info.Runtime.Name = resolved.Runtime
	info.Runtime.Socket = resolved.SocketPath
	if _, err := o.fs.Stat(resolved.SocketPath); err == nil {
		info.Runtime.Status = "accessible"
	} else {
		info.Runtime.Status = fmt.Sprintf("not found or inaccessible: %v", err)
	}
	info.Configs.Global = globalPaths
	info.Configs.Tools = toolsPaths
	for toolName := range toolsCfg {
		info.AvailableTools = append(info.AvailableTools, toolName)
	}
	sort.Strings(info.AvailableTools)

	switch strings.ToLower(resolved.DiagnosisFormat) {
	case "json":
		data, err := json.MarshalIndent(info, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
	case "simple":
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Runtime: %s (%s)\n", info.Runtime.Name, info.Runtime.Socket)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Runtime Status: %s\n", info.Runtime.Status)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Global Config: %s\n", strings.Join(info.Configs.Global, ", "))
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Tools Config: %s\n", strings.Join(info.Configs.Tools, ", "))
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Available Tools: %s\n", strings.Join(info.AvailableTools, ", "))
	default: // Default to YAML
		data, err := yaml.Marshal(info)
		if err != nil {
			return fmt.Errorf("failed to marshal YAML: %w", err)
		}
		_, _ = fmt.Fprint(cmd.OutOrStdout(), string(data))
	}
	return nil
}

func (o *rootOptions) handleDryRun(cmd *cobra.Command, containerConfig *container.ContainerConfig, resolved *config.ResolvedConfig) error {
	switch strings.ToLower(resolved.DryRunFormat) {
	case "json":
		data, err := json.MarshalIndent(containerConfig, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
	case "simple":
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Image: %s\n", containerConfig.Image)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Command: %s\n", strings.Join(containerConfig.Command, " "))
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "TTY: %v\n", containerConfig.TTY)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Interactive: %v\n", containerConfig.Interactive)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Network: %s\n", containerConfig.Network)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Remove: %v\n", containerConfig.Remove)
		var mounts []string
		for _, m := range containerConfig.Mounts {
			mounts = append(mounts, fmt.Sprintf("type=%s,source=%s,target=%s,readonly=%v", m.Type, m.Source, m.Target, m.ReadOnly))
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Mounts: %s\n", strings.Join(mounts, ", "))
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Env: %s\n", strings.Join(containerConfig.Env, ", "))
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Workdir: %s\n", containerConfig.Workdir)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "User: %s\n", containerConfig.User)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Ports: %s\n", strings.Join(containerConfig.Ports, ", "))
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "PublishAll: %v\n", containerConfig.PublishAll)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Expose: %s\n", strings.Join(containerConfig.Expose, ", "))
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Hostname: %s\n", containerConfig.Hostname)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "DNS: %s\n", strings.Join(containerConfig.DNS, ", "))
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "AddHosts: %s\n", strings.Join(containerConfig.AddHosts, ", "))
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Privileged: %v\n", containerConfig.Privileged)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "CapAdd: %s\n", strings.Join(containerConfig.CapAdd, ", "))
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "CapDrop: %s\n", strings.Join(containerConfig.CapDrop, ", "))
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Entrypoint: %s\n", strings.Join(containerConfig.Entrypoint, ", "))
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Pull: %s\n", containerConfig.Pull)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Memory: %s\n", units.BytesSize(float64(containerConfig.Memory)))
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "CPUs: %g\n", containerConfig.CPUs)
		var devices []string
		for _, d := range containerConfig.Devices {
			if d.PathOnHost == d.PathInContainer && d.CgroupPermissions == "rwm" {
				devices = append(devices, d.PathOnHost)
			} else {
				devices = append(devices, fmt.Sprintf("%s:%s:%s", d.PathOnHost, d.PathInContainer, d.CgroupPermissions))
			}
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Devices: %s\n", strings.Join(devices, ", "))
	default: // Default to YAML
		data, err := yaml.Marshal(containerConfig)
		if err != nil {
			return fmt.Errorf("failed to marshal YAML: %w", err)
		}
		_, _ = fmt.Fprint(cmd.OutOrStdout(), string(data))
	}
	return nil
}

func getFd(r any) (int, bool) {
	if f, ok := r.(interface{ Fd() uintptr }); ok {
		return int(f.Fd()), true
	}
	return -1, false
}

type syncReader struct {
	inner io.Reader
	ready <-chan struct{}
	ctx   context.Context
}

func (s *syncReader) Read(p []byte) (n int, err error) {
	select {
	case <-s.ctx.Done():
		return 0, s.ctx.Err()
	case <-s.ready:
		return s.inner.Read(p)
	}
}

func (o *rootOptions) execute(cmd *cobra.Command, resolved *config.ResolvedConfig, containerConfig *container.ContainerConfig) (int, error) {
	ctx := cmd.Context()
	fullCmdStr := strings.Join(containerConfig.Command, " ")
	if len(containerConfig.Entrypoint) > 0 {
		fullCmdStr = strings.Join(containerConfig.Entrypoint, " ") + " " + fullCmdStr
	}
	o.logger.Info("Running: %s", fullCmdStr)
	o.logger.Debug("Image: %s", containerConfig.Image)
	o.logger.Debug("Command: %v", containerConfig.Command)
	o.logger.Debug("Entrypoint: %v", containerConfig.Entrypoint)
	o.logger.Debug("Interactive: %v, TTY: %v", containerConfig.Interactive, containerConfig.TTY)
	o.logger.Debug("Runtime: %s", resolved.Runtime)
	o.logger.Debug("Socket: %s", resolved.SocketPath)

	ctxG, cancel := context.WithCancel(ctx)
	defer cancel()

	// Initialize Runtime
	rt, err := o.runtimeFactory(resolved.Runtime, resolved.SocketPath)
	if err != nil {
		return 0, fmt.Errorf("failed to initialize runtime: %w", err)
	}

	o.logger.Trace("Creating container...")
	if err := rt.PullImage(ctx, containerConfig.Image, containerConfig.Pull); err != nil {
		return 0, fmt.Errorf("failed to pull image: %w", err)
	}

	containerID, err := rt.CreateContainer(ctx, containerConfig)
	if err != nil {
		return 0, fmt.Errorf("failed to create container: %w", err)
	}

	if containerConfig.Remove {
		cleanupCtx := context.WithoutCancel(ctx)
		defer func() {
			o.logger.Trace("Removing container: %s", containerID)
			if err := rt.RemoveContainer(cleanupCtx, containerID); err != nil {
				o.logger.Warn("failed to remove container (defer): %v", err)
			}
		}()
	}

	// Detect if host stdin is a terminal and get its FD
	stdinFd, isHostStdinTerminal := getFd(cmd.InOrStdin())
	if isHostStdinTerminal {
		isHostStdinTerminal = o.isTerminal(stdinFd)
	}
	o.logger.Debug("Host STDIN is terminal: %v", isHostStdinTerminal)

	effectiveHangTimeout := o.getHangTimeout(isHostStdinTerminal, containerConfig.Interactive, resolved)

	// Set up terminal raw mode if TTY is requested and we are in a terminal
	if isHostStdinTerminal && containerConfig.TTY {
		o.logger.Trace("Setting terminal to raw mode")
		state, err := o.makeRaw(stdinFd)
		if err != nil {
			o.logger.Warn("failed to set terminal to raw mode: %v", err)
		} else {
			defer func() { _ = o.restore(stdinFd, state) }() //nolint:errcheck
		}
	}

	// Handle signals and forward them to the container
	sigChan := make(chan os.Signal, 1)
	setupSignals(sigChan)
	defer signal.Stop(sigChan)
	go func() {
		firstSignal := true
		for {
			select {
			case sig := <-sigChan:
				if firstSignal {
					sigName := getSignalName(sig)
					o.logger.Debug("Forwarding signal %v (%s) to container", sig, sigName)
					if err := rt.SignalContainer(ctxG, containerID, sigName); err != nil {
						o.logger.Warn("failed to forward signal %v: %v", sig, err)
					} else {
						o.logger.Debug("Successfully forwarded signal %v to container", sig)
					}
					firstSignal = false
				} else {
					o.logger.Info("Received second signal, terminating...")
					cancel()
					return
				}
			case <-ctxG.Done():
				return
			}
		}
	}()

	// Attach to container IO concurrently
	var stdin io.Reader
	startSignal := make(chan struct{})
	if containerConfig.Interactive {
		stdin = &syncReader{
			inner: cmd.InOrStdin(),
			ready: startSignal,
			ctx:   ctxG,
		}
	}

	attachCtx, cancelAttach := context.WithCancel(ctxG)
	defer cancelAttach()

	attachReady := make(chan struct{})
	attachDone := make(chan error, 1)
	go func() {
		attachDone <- rt.AttachContainer(attachCtx, containerID, containerConfig.TTY, stdin, cmd.OutOrStdout(), cmd.ErrOrStderr(), attachReady)
	}()

	// Give a tiny bit of time for the goroutine to reach AttachContainer call,
	// reducing race condition where container starts and finishes before attachment.
	select {
	case <-attachReady:
	case err := <-attachDone:
		if err != nil {
			return 0, fmt.Errorf("failed to attach to container: %w", err)
		}
	case <-ctxG.Done():
		return 0, ctxG.Err()
	}

	o.logger.Trace("Starting container: %s", containerID)
	if err := rt.StartContainer(ctx, containerID); err != nil {
		return 0, fmt.Errorf("failed to start container: %w", err)
	}
	close(startSignal) // Signal stdin to start reading

	// Handle window resize synchronization
	if fd, ok := getFd(cmd.OutOrStdout()); ok && containerConfig.TTY && o.isTerminal(fd) {
		resizeChan := make(chan os.Signal, 1)
		setupResizeSignal(resizeChan)
		defer signal.Stop(resizeChan)
		go func() {
			for {
				select {
				case <-resizeChan:
					w, h, err := o.termGetSize(fd)
					if err == nil && h >= 0 && w >= 0 {
						_ = rt.ResizeContainerTTY(ctxG, containerID, uint(h), uint(w)) //nolint:gosec,errcheck
					}
				case <-ctxG.Done():
					return
				}
			}
		}()

		// Initial resize to match current terminal size
		w, h, err := o.termGetSize(fd)
		if err == nil && h >= 0 && w >= 0 {
			_ = rt.ResizeContainerTTY(ctxG, containerID, uint(h), uint(w)) //nolint:gosec,errcheck
		}
	}

	o.logger.Trace("Waiting for container: %s", containerID)

	type waitResult struct {
		code int
		err  error
	}
	waitDone := make(chan waitResult, 1)
	go func() {
		code, err := rt.WaitContainer(ctxG, containerID)
		waitDone <- waitResult{code, err}
	}()

	var exitCode int
	select {
	case result := <-waitDone:
		if result.err != nil {
			o.logger.Debug("WaitContainer for %s failed or was interrupted: %v", containerID, result.err)
			return 0, fmt.Errorf("failed to wait for container: %w", result.err)
		}
		exitCode = result.code
		o.logger.Debug("Container %s finished with exit code %d", containerID, exitCode)

		// After container exits, wait a short grace period for remaining output
		o.logger.Trace("Waiting for remaining output from container %s (grace period: %v)", containerID, attachGracePeriod)
		select {
		case err := <-attachDone:
			if err != nil && !errors.Is(err, context.Canceled) {
				o.logger.Debug("AttachContainer finished with error after container exit for %s: %v", containerID, err)
				return exitCode, fmt.Errorf("failed to attach to container: %w", err)
			}
			o.logger.Debug("AttachContainer finished successfully for %s", containerID)
		case <-time.After(attachGracePeriod):
			o.logger.Debug("AttachContainer timed out after container exit for %s, forcing close", containerID)
			cancelAttach()
			<-attachDone
		}

	case err := <-attachDone:
		if err != nil && !errors.Is(err, context.Canceled) {
			o.logger.Debug("AttachContainer finished with error before container exit for %s: %v", containerID, err)
			// Wait for container to finish (best effort)
			cancel()
			select {
			case res := <-waitDone:
				exitCode = res.code
			case <-time.After(effectiveHangTimeout):
				o.logger.Debug("Timeout waiting for container %s after attach error", containerID)
			}
			return exitCode, fmt.Errorf("failed to attach to container: %w", err)
		}
		o.logger.Debug("AttachContainer finished successfully before container exit for %s", containerID)
		// IO finished before container exited.
		// In non-TTY mode, or if the host input is a pipe, if it doesn't exit soon, we might be hitting the Docker 29.1.5 hang.
		// If host stdin is not a terminal, we use a much shorter timeout because we don't expect interactive behavior.
		if !isHostStdinTerminal || !containerConfig.Interactive {
			o.logger.Trace("IO finished, waiting up to %v for container %s to exit", effectiveHangTimeout, containerID)
			select {
			case result := <-waitDone:
				if result.err != nil {
					return 0, fmt.Errorf("failed to wait for container: %w", result.err)
				}
				exitCode = result.code
			case <-time.After(effectiveHangTimeout):
					exitCode, err = o.forceTerminateIfRunning(context.Background(), rt, containerID)
					if err != nil {
						return 0, err
					}
					select {
					case result := <-waitDone:
						exitCode = result.code
					case <-time.After(effectiveHangTimeout):
						return exitCode, nil
					}
			}
		} else {
			// TTY mode: it's normal to wait for container exit even after IO might seem "done"
			result := <-waitDone
			if result.err != nil {
				return 0, fmt.Errorf("failed to wait for container: %w", result.err)
			}
			exitCode = result.code
		}
	}

	o.logger.Debug("Total execution finished for container: %s", containerID)
	return exitCode, nil
}

func newRootCmd(o *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Version:       version.Info(),
		Use:           "cderun",
		SilenceUsage:  true,
		SilenceErrors: true,
		Short:         "A wrapper tool to run commands in a containerized environment.",
		Long: `cderun is a CLI wrapper tool that simplifies running commands
within a container. It separates its own flags from the flags
intended for the subcommand.`,
	}
	cmd.SetVersionTemplate("{{.Version}}\n")
	cmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		if o.fs == nil {
			o.fs = config.RealFileSystem{}
		}
		if o.configLoader == nil {
			o.configLoader = config.NewConfigLoaderWithFS(o.fs)
		}
	}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
			// Early logger initialization with CLI and Environment settings before config loading.
			// This allows loadConfigs() to use the correct log level.
			initialLevel := "warn"
			if env := o.fs.Getenv("CDERUN_LOG_LEVEL"); env != "" {
				initialLevel = env
			}
			if o.logLevel != "" {
				initialLevel = o.logLevel
			}
			if o.cderunLogLevel != "" {
				initialLevel = o.cderunLogLevel
			}
			_ = o.logger.Init(initialLevel, "text", true) //nolint:errcheck

			// Load configurations
			toolsCfg, globalCfg, globalPaths, toolsPaths, err := o.loadConfigs(cmd)
			if err != nil {
				return err
			}

			subcommand := ""
			passthroughArgs := []string{}
			if len(args) > 0 {
				subcommand = args[0]
				passthroughArgs = args[1:]
			}

			// Resolve settings using priority logic (CLI > Env > Config > Default)
			resolved, err := o.resolveSettings(cmd, subcommand, toolsCfg, globalCfg)
			if err != nil {
				return fmt.Errorf("configuration error: %w", err)
			}

			// Validate pull policy
			switch resolved.Pull {
			case "always", "missing", "never":
				// Valid
			default:
				return fmt.Errorf("invalid pull policy %q: allowed values are \"always\", \"missing\", or \"never\"", resolved.Pull)
			}

			if resolved.Diagnosis {
				return o.handleDiagnosis(cmd, resolved, toolsCfg, globalPaths, toolsPaths)
			}

			if len(args) == 0 {
				if resolved.DryRun {
					return fmt.Errorf("--dry-run requires a subcommand")
				}
				return cmd.Help()
			}

			// Re-initialize logger with fully resolved settings including those from config files.
			if err := o.logger.Init(resolved.LogLevel, resolved.LogFormat, resolved.LogTimestamp); err != nil {
				return fmt.Errorf("failed to initialize logger: %w", err)
			}
			// Redirect logging to the command's stderr stream.
			o.logger.SetOutput(cmd.ErrOrStderr())
			o.logger.Debug("Logger initialized with level: %s", resolved.LogLevel)

			// Build ContainerConfig
			var containerConfig *container.ContainerConfig
			if subcommand != "" {
				containerConfig, err = o.buildContainerConfig(resolved, passthroughArgs, toolsCfg)
				if err != nil {
					return fmt.Errorf("container configuration error: %w", err)
				}
			}

			if resolved.DryRun {
				return o.handleDryRun(cmd, containerConfig, resolved)
			}

			// Create snapshot if nested execution support is requested or already active
			var snapshotDir string
			if resolved.MountCderun || resolved.MountAllTools || len(resolved.MountTools) > 0 || (globalCfg != nil && globalCfg.HostContext != nil) {
				o.logger.Debug("Creating execution snapshot for nested support...")
				// Ensure globalCfg is initialized for snapshot if it was nil
				if globalCfg == nil {
					globalCfg = &config.CDERunConfig{}
				}
				sDir, hostDir, err := createSnapshot(o.logger, o.fs, globalCfg, toolsCfg, containerConfig.Mounts)
				if err != nil {
					o.logger.Warn("failed to create snapshot: %v", err)
				} else {
					snapshotDir = sDir
					defer func() {
						o.logger.Trace("Cleaning up snapshot: %s", snapshotDir)
						if err := cleanupSnapshot(o.fs, snapshotDir); err != nil {
							o.logger.Warn("failed to cleanup snapshot: %v", err)
						}
					}()
					// Mount the snapshot directory using the host path as source
					containerConfig.Mounts = append(containerConfig.Mounts, container.Mount{
						Type:     "bind",
						Source:   hostDir,
						Target:   "/run/cderun",
						ReadOnly: true,
					})
				}
			}

			// Execute Container
			exitCode, err := o.execute(cmd, resolved, containerConfig)
			if err != nil {
				return err
			}
			o.exitFunc(exitCode)
			return nil
		}

cmd.PersistentFlags().BoolVarP(&o.tty, "tty", "t", false, "Allocate a pseudo-TTY")
	cmd.PersistentFlags().BoolVarP(&o.interactive, "interactive", "i", false, "Keep STDIN open even if not attached")
	cmd.PersistentFlags().StringVar(&o.network, "network", "bridge", "Connect a container to a network")
	cmd.PersistentFlags().StringVar(&o.socketPath, "socket-path", "", "Path to the container runtime socket on the host")
	cmd.PersistentFlags().BoolVar(&o.mountSocket, "mount-socket", false, "Mount the container runtime socket into the container")
	cmd.PersistentFlags().StringVar(&o.mountSocketPath, "mount-socket-path", "", "Path where the socket should be mounted inside the container (defaults to host path)")
	cmd.PersistentFlags().BoolVar(&o.mountCderun, "mount-cderun", false, "Mount cderun binary for use inside container")
	cmd.PersistentFlags().StringVar(&o.mountCderunPath, "mount-cderun-path", "", "Host path to cderun binary to mount inside container")
	cmd.PersistentFlags().StringVar(&o.image, "image", "", "Docker image to use")
	cmd.PersistentFlags().StringVar(&o.runtimeName, "runtime", "docker", "Container runtime to use (docker/podman)")
	cmd.PersistentFlags().StringArrayVarP(&o.env, "env", "e", nil, "Set environment variables")
	cmd.PersistentFlags().StringVarP(&o.workdir, "workdir", "w", "", "Working directory inside the container")
	cmd.PersistentFlags().StringArrayVar(&o.mounts, "mount", nil, "Attach a filesystem mount to the container")
	cmd.PersistentFlags().StringVar(&o.mountTools, "mount-tools", "", "Mount specified tools into the container")
	cmd.PersistentFlags().BoolVar(&o.mountAllTools, "mount-all-tools", false, "Mount all defined tools into the container")
	cmd.PersistentFlags().BoolVar(&o.remove, "remove", true, "Automatically remove the container when it exits")

	// Docker-compatible flags
	cmd.PersistentFlags().StringArrayVarP(&o.ports, "publish", "p", nil, "Publish a container's port(s) to the host")
	cmd.PersistentFlags().BoolVarP(&o.publishAll, "publish-all", "P", false, "Publish all exposed ports to random ports")
	cmd.PersistentFlags().StringArrayVar(&o.expose, "expose", nil, "Expose a port or a range of ports")
	cmd.PersistentFlags().StringVar(&o.hostname, "hostname", "", "Container host name")
	cmd.PersistentFlags().StringArrayVar(&o.dns, "dns", nil, "Set custom DNS servers")
	cmd.PersistentFlags().StringArrayVar(&o.addHosts, "add-host", nil, "Add a custom host-to-IP mapping (host:ip)")
	cmd.PersistentFlags().StringVarP(&o.user, "user", "u", "", "Username or UID (format: <name|uid>[:<group|gid>])")
	cmd.PersistentFlags().BoolVar(&o.privileged, "privileged", false, "Give extended privileges to this container")
	cmd.PersistentFlags().StringArrayVar(&o.capAdd, "cap-add", nil, "Add Linux capabilities")
	cmd.PersistentFlags().StringArrayVar(&o.capDrop, "cap-drop", nil, "Drop Linux capabilities")
	cmd.PersistentFlags().StringArrayVar(&o.entrypoint, "entrypoint", nil, "Overwrite the default ENTRYPOINT of the image")
	cmd.PersistentFlags().StringVar(&o.pull, "pull", "missing", "Pull image before running (always, missing, never)")
	cmd.PersistentFlags().StringVarP(&o.memory, "memory", "m", "", "Memory limit")
	cmd.PersistentFlags().Float64Var(&o.cpus, "cpus", 0, "Number of CPUs")
	cmd.PersistentFlags().StringArrayVar(&o.devices, "device", nil, "Add a host device to the container")

	cmd.PersistentFlags().BoolVar(&o.cderunTTY, "cderun-tty", false, "Override TTY setting (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().BoolVar(&o.cderunInteractive, "cderun-interactive", false, "Override interactive setting (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().StringVar(&o.cderunImage, "cderun-image", "", "Override image (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().StringVar(&o.cderunNetwork, "cderun-network", "", "Override network setting (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().BoolVar(&o.cderunRemove, "cderun-remove", true, "Override remove setting (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().StringVar(&o.cderunRuntime, "cderun-runtime", "", "Override runtime setting (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().StringVar(&o.cderunSocketPath, "cderun-socket-path", "", "Override socket path (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().BoolVar(&o.cderunMountSocket, "cderun-mount-socket", false, "Override mount-socket setting (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().StringVar(&o.cderunMountSocketPath, "cderun-mount-socket-path", "", "Override mount-socket-path setting (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().StringArrayVar(&o.cderunEnv, "cderun-env", nil, "Override environment variables (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().StringVar(&o.cderunWorkdir, "cderun-workdir", "", "Override workdir setting (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().BoolVar(&o.strictEnv, "strict-env", false, "Require all environment variables to be present on the host")
	cmd.PersistentFlags().BoolVar(&o.cderunStrictEnv, "cderun-strict-env", false, "Override strict-env setting (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().StringArrayVar(&o.cderunMounts, "cderun-mount", nil, "Override mounts (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().BoolVar(&o.cderunMountCderun, "cderun-mount-cderun", false, "Override mount-cderun setting (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().StringVar(&o.cderunMountCderunPath, "cderun-mount-cderun-path", "", "Override mount-cderun-path setting (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().StringVar(&o.cderunMountTools, "cderun-mount-tools", "", "Override mount-tools setting (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().BoolVar(&o.cderunMountAllTools, "cderun-mount-all-tools", false, "Override mount-all-tools setting (highest priority, can be used after subcommand)")

	cmd.PersistentFlags().StringVar(&o.configPath, "config", "", "Path to cderun config file")
	cmd.PersistentFlags().StringVar(&o.cderunConfigPath, "cderun-config", "", "Override cderun config file (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().StringVar(&o.toolConfigPath, "tool-config", "", "Path to tools config file")
	cmd.PersistentFlags().StringVar(&o.cderunToolConfigPath, "cderun-tool-config", "", "Override tools config file (highest priority, can be used after subcommand)")

	// Priority 1 overrides
	cmd.PersistentFlags().StringArrayVar(&o.cderunPorts, "cderun-publish", nil, "Override publish setting (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().BoolVar(&o.cderunPublishAll, "cderun-publish-all", false, "Override publish-all setting (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().StringArrayVar(&o.cderunExpose, "cderun-expose", nil, "Override expose setting (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().StringVar(&o.cderunHostname, "cderun-hostname", "", "Override hostname setting (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().StringArrayVar(&o.cderunDNS, "cderun-dns", nil, "Override DNS setting (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().StringArrayVar(&o.cderunAddHosts, "cderun-add-host", nil, "Override add-host setting (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().StringVar(&o.cderunUser, "cderun-user", "", "Override user setting (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().BoolVar(&o.cderunPrivileged, "cderun-privileged", false, "Override privileged setting (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().StringArrayVar(&o.cderunCapAdd, "cderun-cap-add", nil, "Override cap-add setting (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().StringArrayVar(&o.cderunCapDrop, "cderun-cap-drop", nil, "Override cap-drop setting (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().StringArrayVar(&o.cderunEntrypoint, "cderun-entrypoint", nil, "Override entrypoint setting (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().StringVar(&o.cderunPull, "cderun-pull", "", "Override pull setting (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().StringVar(&o.cderunMemory, "cderun-memory", "", "Override memory setting (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().Float64Var(&o.cderunCPUs, "cderun-cpus", 0, "Override cpus setting (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().StringArrayVar(&o.cderunDevices, "cderun-device", nil, "Override device setting (highest priority, can be used after subcommand)")

	cmd.PersistentFlags().BoolVar(&o.dryRun, "dry-run", false, "Preview container configuration without execution")
	cmd.PersistentFlags().StringVarP(&o.dryRunFormat, "dry-run-format", "f", "yaml", "Output format (yaml, json, simple)")
	cmd.PersistentFlags().BoolVar(&o.cderunDryRun, "cderun-dry-run", false, "Override dry-run setting (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().StringVar(&o.cderunDryRunFormat, "cderun-dry-run-format", "", "Override dry-run-format setting (highest priority, can be used after subcommand)")

	cmd.PersistentFlags().BoolVar(&o.diagnosis, "diagnosis", false, "Show system diagnostics and available tools")
	cmd.PersistentFlags().StringVar(&o.diagnosisFormat, "diagnosis-format", "yaml", "Diagnosis output format (yaml, json, simple)")
	cmd.PersistentFlags().BoolVar(&o.cderunDiagnosis, "cderun-diagnosis", false, "Override diagnosis setting (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().StringVar(&o.cderunDiagnosisFormat, "cderun-diagnosis-format", "", "Override diagnosis-format setting (highest priority, can be used after subcommand)")

	cmd.PersistentFlags().StringVar(&o.logLevel, "log-level", "", "Set log level (error, warn, info, debug, trace)")
	cmd.PersistentFlags().StringVar(&o.logFormat, "log-format", "text", "Set log format (text, json)")
	cmd.PersistentFlags().BoolVar(&o.logTimestamp, "log-timestamp", true, "Include timestamp in logs")

	cmd.PersistentFlags().StringVar(&o.cderunLogLevel, "cderun-log-level", "", "Override log level (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().StringVar(&o.cderunLogFormat, "cderun-log-format", "", "Override log format (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().BoolVar(&o.cderunLogTimestamp, "cderun-log-timestamp", true, "Override log-timestamp setting (highest priority, can be used after subcommand)")

	cmd.PersistentFlags().StringVar(&o.hangTimeout, "hang-timeout", "", "Grace period after I/O completion before force-terminating the container (e.g. 2s, 500ms)")
	cmd.PersistentFlags().StringVar(&o.cderunHangTimeout, "cderun-hang-timeout", "", "Override hang-timeout setting (highest priority, can be used after subcommand)")

	cmd.Flags().SetInterspersed(false)
	return cmd
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute(rawArgs []string) error {
	return ExecuteContext(context.Background(), rawArgs)
}

// ExecuteContext adds all child commands to the root command and sets flags appropriately, using the provided context.
func ExecuteContext(ctx context.Context, rawArgs []string) error {
	return ExecuteContextWithOptions(ctx, rawArgs, nil)
}

// ExecuteContextWithOptions adds all child commands to a new command instance and sets flags appropriately,
// using the provided context and allowing for option customization.
func ExecuteContextWithOptions(ctx context.Context, rawArgs []string, setup func(o *rootOptions, cmd *cobra.Command)) error {
	var cmd *cobra.Command

	if setup == nil {
		// Use global state for standard execution
		cmd = rootCmd
	} else {
		// Create fresh state for testing
		localOpts := defaultOptions()
		localOpts.logger = logging.NewLogger() // Fresh logger for isolation
		cmd = newRootCmd(&localOpts)
		setup(&localOpts, cmd)
		// Redirect logger to the command's error writer early to capture initial logs.
		localOpts.logger.SetOutput(cmd.ErrOrStderr())
	}

	args, err := preprocessArgs(cmd, rawArgs)
	if err != nil {
		return err
	}
	if len(args) >= 1 {
		cmd.SetArgs(args[1:])
	} else {
		cmd.SetArgs([]string{})
	}
	return cmd.ExecuteContext(ctx)
}

func preprocessArgs(cmd *cobra.Command, args []string) ([]string, error) {
	if len(args) == 0 {
		return args, nil
	}

	execName := filepath.Base(args[0])
	isPolyglot := execName != "cderun"

	// Find the subcommand index robustly by skipping flags and their arguments
	subcmdIdx := -1
	if isPolyglot {
		subcmdIdx = 0
	} else {
		for i := 1; i < len(args); i++ {
			arg := args[i]
			if !strings.HasPrefix(arg, "-") {
				subcmdIdx = i
				break
			}
			// It's a flag. Check if it's a long flag or shorthand and if it takes an argument.
			if strings.HasPrefix(arg, "--") {
				name := strings.SplitN(arg[2:], "=", 2)[0]
				if f := cmd.Flags().Lookup(name); f != nil && f.NoOptDefVal == "" && !strings.Contains(arg, "=") {
					// Flag exists, takes an argument, and no '=' used, so skip next argument.
					i++
				}
			} else if len(arg) > 1 {
				// Shorthand(s), e.g., -i, -it, -p 80:80
				// For shorthand, we only handle the case where the last shorthand in the group takes an argument.
				lastChar := string(arg[len(arg)-1])
				if f := cmd.Flags().ShorthandLookup(lastChar); f != nil && f.NoOptDefVal == "" {
					// Last shorthand takes an argument, skip next argument.
					i++
				}
			}
		}
	}

	// If not polyglot, check for P1 flags before the subcommand
	if !isPolyglot && subcmdIdx != -1 {
		for i := 1; i < subcmdIdx; i++ {
			if strings.HasPrefix(args[i], "--cderun-") {
				return nil, fmt.Errorf("cderun internal override flag %q must be placed after the subcommand", args[i])
			}
		}
	}

	processedArgs := make([]string, 0, len(args)+1)
	if isPolyglot {
		processedArgs = append(processedArgs, "cderun")
	} else {
		processedArgs = append(processedArgs, args[0])
	}

	var overrides []string
	var others []string

	// Scan all arguments after the executable name
	// In polyglot mode, everything after index 0 is after the subcommand.
	// In standard mode, only arguments after subcmdIdx are considered for hoisting P1 overrides.
	startIdx := 1
	if !isPolyglot && subcmdIdx != -1 {
		// Standard mode: hoist only from after the subcommand
		for i := 1; i <= subcmdIdx; i++ {
			others = append(others, args[i])
		}
		startIdx = subcmdIdx + 1
	}

	for i := startIdx; i < len(args); i++ {
		arg := args[i]
		shouldHoist := false

		if strings.HasPrefix(arg, "--cderun-") {
			shouldHoist = true
		}

		if shouldHoist {
			overrides = append(overrides, arg)
			// Handle flags that take arguments (skip next arg if it's the value)
			// Note: only --cderun- flags are hoisted here.
			if strings.HasPrefix(arg, "--") && !strings.Contains(arg, "=") {
				name := arg[2:]
				f := cmd.PersistentFlags().Lookup(name)
				if f == nil {
					f = cmd.Flags().Lookup(name)
				}
				if f != nil && f.NoOptDefVal == "" && i+1 < len(args) {
					overrides = append(overrides, args[i+1])
					i++
				}
			}
		} else {
			others = append(others, arg)
		}
	}

	// Place --cderun-* overrides immediately after "cderun" so they are always parsed
	processedArgs = append(processedArgs, overrides...)

	if isPolyglot {
		// In polyglot mode, the original executable name becomes the subcommand
		processedArgs = append(processedArgs, execName)
	}

	processedArgs = append(processedArgs, others...)

	return processedArgs, nil
}

func init() {
	opts.exitFunc = os.Exit
	rootCmd = newRootCmd(&opts)
}
