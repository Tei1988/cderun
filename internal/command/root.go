package command

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/docker/go-units"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"gopkg.in/yaml.v3"

	"cderun/internal/config"
	"cderun/internal/container"
	"cderun/internal/logging"
	"cderun/internal/runtime"
	"cderun/internal/version"
)

// ExitCodeError is an error that carries an exit code.
// It is used to propagate exit codes back to main while allowing defers to run.
type ExitCodeError struct {
	Code int
	Err  error
}

func (e *ExitCodeError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return fmt.Sprintf("exit status %d", e.Code)
}

func (e *ExitCodeError) Unwrap() error {
	return e.Err
}

type rootOptions struct {
	readOnly              bool
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
	cderunReadOnly        bool
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
	ports                 []string
	publishAll            bool
	expose                []string
	hostname              string
	dns                   []string
	addHosts              []string
	user                  string
	privileged            bool
	capAdd                []string
	capDrop               []string
	entrypoint            []string
	pull                  string
	memory                string
	cpus                  float64
	devices               []string
	groupAdd              []string
	cderunPorts           []string
	cderunPublishAll      bool
	cderunExpose          []string
	cderunHostname        string
	cderunDNS             []string
	cderunAddHosts        []string
	cderunUser            string
	cderunPrivileged      bool
	cderunCapAdd          []string
	cderunCapDrop         []string
	cderunGroupAdd        []string
	cderunEntrypoint      []string
	cderunPull            string
	cderunMemory          string
	cderunCPUs            float64
	cderunDevices         []string
	sensitiveEnv          []string
	cderunSensitiveEnv    []string
	pullMaxRetries        int
	cderunPullMaxRetries  int
	pullBackoffBase       string
	cderunPullBackoffBase string

	// Dependencies
	fs           config.FileSystem
	configLoader *config.ConfigLoader
	logger       *logging.Logger

	// Testing hooks
	exitFunc           func(int)
	isTerminal         func(int) bool
	termGetSize        func(int) (int, int, error)
	makeRaw            func(int) (*term.State, error)
	restore            func(int, *term.State) error
	setupSignals       func(chan os.Signal)
	setupResizeSignal  func(chan os.Signal)
	stopSignalHandling func(chan os.Signal)
	runtimeFactory     func(string, string, *logging.Logger) (runtime.ContainerRuntime, error)
	jsonMarshalIndent  func(v any, prefix, indent string) ([]byte, error)
	yamlMarshal        func(v any) ([]byte, error)

	attachGracePeriod time.Duration
}

const (
	attachGracePeriod = 5 * time.Second
	hangTimeout       = 10 * time.Second
)

func defaultOptions() rootOptions {
	return rootOptions{
		fs: config.RealFileSystem{},
		exitFunc: func(code int) {
			// Default to no-op for safety.
			// Updated to os.Exit in ExecuteContextWithOptions for production use.
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
		setupSignals: func(sigChan chan os.Signal) {
			setupSignals(sigChan)
		},
		setupResizeSignal: func(resizeChan chan os.Signal) {
			setupResizeSignal(resizeChan)
		},
		stopSignalHandling: func(sigChan chan os.Signal) {
			signal.Stop(sigChan)
		},
		attachGracePeriod: attachGracePeriod,
		logger:            logging.GetGlobalLogger(),
		runtimeFactory: func(name string, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
			switch name {
			case "docker":
				return runtime.NewDockerRuntimeWithOptions(socket, "docker", nil, runtime.WithLogger(l))
			case "podman":
				return runtime.NewPodmanRuntime(socket, runtime.WithLogger(l))
			case "containerd":
				return runtime.NewContainerdRuntime(socket, runtime.WithContainerdLogger(l))
			default:
				return nil, fmt.Errorf("unsupported runtime %q", name)
			}
		},
		jsonMarshalIndent: json.MarshalIndent,
		yamlMarshal:       yaml.Marshal,
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

func (o *rootOptions) ensureHooks() {
	if o.jsonMarshalIndent == nil {
		o.jsonMarshalIndent = json.MarshalIndent
	}
	if o.yamlMarshal == nil {
		o.yamlMarshal = yaml.Marshal
	}
}

func (o *rootOptions) resolveSettings(cmd *cobra.Command, subcommand string, toolsCfg config.ToolsConfig, globalCfg *config.CDERunConfig) (*config.ResolvedConfig, error) {
	cliOpts := config.CLIOptions{
		ReadOnly:                 o.readOnly,
		ReadOnlySet:              cmd.Flags().Changed("read-only"),
		CderunReadOnly:           o.cderunReadOnly,
		CderunReadOnlySet:        cmd.Flags().Changed("cderun-read-only"),
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
		Ports:                    o.ports,
		CderunPorts:              o.cderunPorts,
		PublishAll:               o.publishAll,
		PublishAllSet:            cmd.Flags().Changed("publish-all"),
		CderunPublishAll:         o.cderunPublishAll,
		CderunPublishAllSet:      cmd.Flags().Changed("cderun-publish-all"),
		Expose:                   o.expose,
		CderunExpose:             o.cderunExpose,
		Hostname:                 o.hostname,
		HostnameSet:              cmd.Flags().Changed("hostname"),
		CderunHostname:           o.cderunHostname,
		CderunHostnameSet:        cmd.Flags().Changed("cderun-hostname"),
		DNS:                      o.dns,
		CderunDNS:                o.cderunDNS,
		AddHosts:                 o.addHosts,
		CderunAddHosts:           o.cderunAddHosts,
		User:                     o.user,
		UserSet:                  cmd.Flags().Changed("user"),
		CderunUser:               o.cderunUser,
		CderunUserSet:            cmd.Flags().Changed("cderun-user"),
		Privileged:               o.privileged,
		PrivilegedSet:            cmd.Flags().Changed("privileged"),
		CderunPrivileged:         o.cderunPrivileged,
		CderunPrivilegedSet:      cmd.Flags().Changed("cderun-privileged"),
		CapAdd:                   o.capAdd,
		CderunCapAdd:             o.cderunCapAdd,
		CapDrop:                  o.capDrop,
		CderunCapDrop:            o.cderunCapDrop,
		Entrypoint:               o.entrypoint,
		CderunEntrypoint:         o.cderunEntrypoint,
		Pull:                     o.pull,
		PullSet:                  cmd.Flags().Changed("pull"),
		CderunPull:               o.cderunPull,
		CderunPullSet:            cmd.Flags().Changed("cderun-pull"),
		PullMaxRetries:           o.pullMaxRetries,
		PullMaxRetriesSet:        cmd.Flags().Changed("pull-max-retries"),
		CderunPullMaxRetries:     o.cderunPullMaxRetries,
		CderunPullMaxRetriesSet:  cmd.Flags().Changed("cderun-pull-max-retries"),
		PullBackoffBase:          o.pullBackoffBase,
		PullBackoffBaseSet:       cmd.Flags().Changed("pull-backoff-base"),
		CderunPullBackoffBase:    o.cderunPullBackoffBase,
		CderunPullBackoffBaseSet: cmd.Flags().Changed("cderun-pull-backoff-base"),
		Memory:                   o.memory,
		MemorySet:                cmd.Flags().Changed("memory"),
		CderunMemory:             o.cderunMemory,
		CderunMemorySet:          cmd.Flags().Changed("cderun-memory"),
		CPUs:                     o.cpus,
		CPUsSet:                  cmd.Flags().Changed("cpus"),
		CderunCPUs:               o.cderunCPUs,
		CderunCPUsSet:            cmd.Flags().Changed("cderun-cpus"),
		Devices:                  o.devices,
		CderunDevices:            o.cderunDevices,
		SensitiveEnv:             o.sensitiveEnv,
		CderunSensitiveEnv:       o.cderunSensitiveEnv,
		GroupAdd:                 o.groupAdd,
		CderunGroupAdd:           o.cderunGroupAdd,
	}

	return config.ResolveWithFS(subcommand, &cliOpts, toolsCfg, globalCfg, o.fs)
}

// applyToolMounts applies cderun binary mount and MountTools/MountAllTools mounts.
func (o *rootOptions) applyToolMounts(
	cfg *container.ContainerConfig,
	resolved *config.ResolvedConfig,
	toolsCfg config.ToolsConfig,
) error {
	if !resolved.MountCderun && !resolved.MountAllTools && len(resolved.MountTools) == 0 {
		return nil
	}

	exePath := resolved.MountCderunPath
	if exePath == "" {
		var err error
		exePath, err = o.fs.Executable()
		if err != nil {
			return fmt.Errorf("failed to get executable path: %w", err)
		}
	}

	// Translate exePath for nested execution if it was determined from os.Executable()
	// (MountCderunPath is already resolved during resolution if it came from config/flags)
	if resolved.MountCderunPath == "" && resolved.HostContext != nil && resolved.HostContext.Level > 0 {
		r, err := config.NewExpressionResolverWithFS(resolved.HostContext, o.fs)
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
	cfg.Mounts = append(cfg.Mounts, container.Mount{
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
			if err := config.ValidateToolName(toolName); err != nil {
				return fmt.Errorf("invalid tool name in tools configuration %q: %w", toolName, err)
			}
			cfg.Mounts = append(cfg.Mounts, container.Mount{
				Type:     "bind",
				Source:   exePath,
				Target:   "/usr/local/bin/" + toolName,
				ReadOnly: true,
			})
		}
	} else if len(resolved.MountTools) > 0 {
		for _, toolName := range resolved.MountTools {
			if err := config.ValidateToolName(toolName); err != nil {
				return fmt.Errorf("invalid tool name in mount-tools %q: %w", toolName, err)
			}
			if _, ok := toolsCfg[toolName]; !ok {
				available := make([]string, 0, len(toolsCfg))
				for k := range toolsCfg {
					available = append(available, k)
				}
				sort.Strings(available)
				return fmt.Errorf("tool %q not found in .tools.yaml\navailable tools: %s", toolName, strings.Join(available, ", "))
			}
			cfg.Mounts = append(cfg.Mounts, container.Mount{
				Type:     "bind",
				Source:   exePath,
				Target:   "/usr/local/bin/" + toolName,
				ReadOnly: true,
			})
		}
	}

	return nil
}

// applySocketMount applies socket mount and GID auto-detection.
func (o *rootOptions) applySocketMount(
	cfg *container.ContainerConfig,
	resolved *config.ResolvedConfig,
) error {
	if !resolved.MountSocket {
		return nil
	}

	// Add socket mount
	cfg.Mounts = append(cfg.Mounts, container.Mount{
		Type:     "bind",
		Source:   resolved.SocketPath,
		Target:   resolved.MountSocketPath,
		ReadOnly: false, // Socket needs to be writable
	})

	// Auto-add socket GID so non-root users can access the mounted socket.
	if socketGID, err := getSocketGID(o.fs, resolved.SocketPath); err == nil {
		if socketGID != "" {
			if !slices.Contains(cfg.GroupAdd, socketGID) {
				cfg.GroupAdd = append(cfg.GroupAdd, socketGID)
				o.logger.Debug("Auto-added socket GID %s from %s", socketGID, resolved.SocketPath)
			}
		}
	} else {
		o.logger.Debug("Failed to stat socket for GID auto-detection: %v", err)
	}

	return nil
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
		ReadOnly:    resolved.ReadOnly,
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
		GroupAdd:   resolved.GroupAdd,
	}

	if err := o.applyToolMounts(containerConfig, resolved, toolsCfg); err != nil {
		return nil, err
	}

	if err := o.applySocketMount(containerConfig, resolved); err != nil {
		return nil, err
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

func (o *rootOptions) writeFormatted(w io.Writer, format string, data any, simpleWriter func(io.Writer)) error {
	switch strings.ToLower(format) {
	case "json":
		marshaled, err := o.jsonMarshalIndent(data, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		_, err = fmt.Fprintln(w, string(marshaled))
		return err
	case "simple":
		simpleWriter(w)
		return nil
	case "yaml", "": // Default to YAML if format is empty
		marshaled, err := o.yamlMarshal(data)
		if err != nil {
			return fmt.Errorf("failed to marshal YAML: %w", err)
		}
		_, err = fmt.Fprint(w, string(marshaled))
		return err
	default:
		return fmt.Errorf("unsupported output format: %q", format)
	}
}

func (o *rootOptions) handleDiagnosis(cmd *cobra.Command, resolved *config.ResolvedConfig, toolsCfg config.ToolsConfig, globalPaths, toolsPaths []string) error {
	o.ensureHooks()
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

	return o.writeFormatted(cmd.OutOrStdout(), resolved.DiagnosisFormat, info, func(w io.Writer) {
		_, _ = fmt.Fprintf(w, "Runtime: %s (%s)\n", info.Runtime.Name, info.Runtime.Socket)
		_, _ = fmt.Fprintf(w, "Runtime Status: %s\n", info.Runtime.Status)
		_, _ = fmt.Fprintf(w, "Global Config: %s\n", strings.Join(info.Configs.Global, ", "))
		_, _ = fmt.Fprintf(w, "Tools Config: %s\n", strings.Join(info.Configs.Tools, ", "))
		_, _ = fmt.Fprintf(w, "Available Tools: %s\n", strings.Join(info.AvailableTools, ", "))
	})
}

func (o *rootOptions) handleDryRun(cmd *cobra.Command, containerConfig *container.ContainerConfig, resolved *config.ResolvedConfig) error {
	o.ensureHooks()

	// Mask sensitive environment variables in dry-run output
	maskedContainerConfig := *containerConfig
	maskedContainerConfig.Env = config.MaskSensitiveEnvList(containerConfig.Env, resolved.SensitiveEnv)

	return o.writeFormatted(cmd.OutOrStdout(), resolved.DryRunFormat, &maskedContainerConfig, func(w io.Writer) {
		_, _ = fmt.Fprintf(w, "Image: %s\n", maskedContainerConfig.Image)
		var quotedCmd []string
		for _, arg := range maskedContainerConfig.Command {
			quotedCmd = append(quotedCmd, fmt.Sprintf("%q", arg))
		}
		_, _ = fmt.Fprintf(w, "Command: %s\n", strings.Join(quotedCmd, " "))
		_, _ = fmt.Fprintf(w, "TTY: %v\n", maskedContainerConfig.TTY)
		_, _ = fmt.Fprintf(w, "Interactive: %v\n", maskedContainerConfig.Interactive)
		_, _ = fmt.Fprintf(w, "Network: %s\n", maskedContainerConfig.Network)
		_, _ = fmt.Fprintf(w, "Remove: %v\n", maskedContainerConfig.Remove)
		var mounts []string
		for _, m := range maskedContainerConfig.Mounts {
			mounts = append(mounts, fmt.Sprintf("type=%s,source=%q,target=%q,readonly=%v", m.Type, m.Source, m.Target, m.ReadOnly))
		}
		_, _ = fmt.Fprintf(w, "Mounts: %s\n", strings.Join(mounts, ", "))
		var quotedEnvs []string
		for _, e := range maskedContainerConfig.Env {
			if k, v, found := strings.Cut(e, "="); found {
				quotedEnvs = append(quotedEnvs, fmt.Sprintf("%q=%q", k, v))
			} else {
				quotedEnvs = append(quotedEnvs, fmt.Sprintf("%q", e))
			}
		}
		_, _ = fmt.Fprintf(w, "Env: %s\n", strings.Join(quotedEnvs, ", "))
		_, _ = fmt.Fprintf(w, "Workdir: %s\n", maskedContainerConfig.Workdir)
		_, _ = fmt.Fprintf(w, "User: %s\n", maskedContainerConfig.User)
		_, _ = fmt.Fprintf(w, "Ports: %s\n", strings.Join(maskedContainerConfig.Ports, ", "))
		_, _ = fmt.Fprintf(w, "PublishAll: %v\n", maskedContainerConfig.PublishAll)
		_, _ = fmt.Fprintf(w, "Expose: %s\n", strings.Join(maskedContainerConfig.Expose, ", "))
		_, _ = fmt.Fprintf(w, "Hostname: %s\n", maskedContainerConfig.Hostname)
		_, _ = fmt.Fprintf(w, "DNS: %s\n", strings.Join(maskedContainerConfig.DNS, ", "))
		_, _ = fmt.Fprintf(w, "AddHosts: %s\n", strings.Join(maskedContainerConfig.AddHosts, ", "))
		_, _ = fmt.Fprintf(w, "Privileged: %v\n", maskedContainerConfig.Privileged)
		_, _ = fmt.Fprintf(w, "CapAdd: %s\n", strings.Join(maskedContainerConfig.CapAdd, ", "))
		_, _ = fmt.Fprintf(w, "CapDrop: %s\n", strings.Join(maskedContainerConfig.CapDrop, ", "))
		_, _ = fmt.Fprintf(w, "GroupAdd: %s\n", strings.Join(maskedContainerConfig.GroupAdd, ", "))

		var devices []string
		for _, d := range maskedContainerConfig.Devices {
			if d.PathOnHost == d.PathInContainer && d.CgroupPermissions == "rwm" {
				devices = append(devices, d.PathOnHost)
			} else {
				devices = append(devices, fmt.Sprintf("%s:%s:%s", d.PathOnHost, d.PathInContainer, d.CgroupPermissions))
			}
		}
		_, _ = fmt.Fprintf(w, "Devices: %s\n", strings.Join(devices, ", "))

		if maskedContainerConfig.Memory > 0 {
			_, _ = fmt.Fprintf(w, "Memory: %s\n", units.BytesSize(float64(maskedContainerConfig.Memory)))
		}
		if maskedContainerConfig.CPUs > 0 {
			_, _ = fmt.Fprintf(w, "CPUs: %s\n", strconv.FormatFloat(maskedContainerConfig.CPUs, 'f', -1, 64))
		}
		if len(maskedContainerConfig.Entrypoint) > 0 {
			var quotedEntry []string
			for _, arg := range maskedContainerConfig.Entrypoint {
				quotedEntry = append(quotedEntry, fmt.Sprintf("%q", arg))
			}
			_, _ = fmt.Fprintf(w, "Entrypoint: %s\n", strings.Join(quotedEntry, " "))
		}
	})
}

func getFd(r any) (int, bool) {
	if f, ok := r.(interface{ Fd() uintptr }); ok {
		fd := f.Fd()
		if fd > math.MaxInt {
			return -1, false
		}
		return int(fd), true
	}
	return -1, false
}

type syncReader struct {
	inner io.Reader
	ready <-chan struct{}
	ctx   context.Context
}

type attachResult struct {
	startSignal        chan struct{}
	attachDone         chan error
	cancelAttach       context.CancelFunc
	attachDoneConsumed bool
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

	rt, containerID, cleanup, err := o.initContainer(ctx, resolved, containerConfig)
	if err != nil {
		return 0, &ExitCodeError{Code: 125, Err: err}
	}
	defer func() {
		if rt != nil {
			if closeErr := rt.Close(); closeErr != nil {
				o.logger.Debug("failed to close runtime: %v", closeErr)
			}
		}
	}()
	defer cleanup()

	// Detect if host stdin is a terminal once
	stdinFd, isHostStdinTerminal := getFd(cmd.InOrStdin())
	if isHostStdinTerminal {
		isHostStdinTerminal = o.isTerminal(stdinFd)
	}

	restoreTerminal := o.setupTerminal(stdinFd, isHostStdinTerminal, containerConfig)
	defer restoreTerminal()

	stopSignals := o.startSignalForwarder(ctxG, cancel, rt, containerID)
	defer stopSignals()

	att, err := o.attachContainer(ctxG, cmd, rt, containerID, containerConfig)
	if err != nil {
		return 0, &ExitCodeError{Code: 125, Err: err}
	}
	defer att.cancelAttach()

	o.logger.Trace("Starting container: %s", containerID)
	if err := rt.StartContainer(ctx, containerID); err != nil {
		return 0, &ExitCodeError{Code: 125, Err: fmt.Errorf("failed to start container: %w", err)}
	}
	close(att.startSignal) // Signal stdin to start reading

	stopResize := o.startResizeHandler(ctxG, cmd, rt, containerID, containerConfig)
	defer stopResize()

	return o.waitForCompletion(ctxG, cmd, rt, containerID, containerConfig, resolved, isHostStdinTerminal, att)
}

func (o *rootOptions) logContainerConfig(resolved *config.ResolvedConfig, cc *container.ContainerConfig) {
	if o.logger == nil || !o.logger.DebugEnabled() {
		return
	}
	if cc == nil {
		return
	}
	var mounts []string
	for _, m := range cc.Mounts {
		mounts = append(mounts, fmt.Sprintf("    - %s %s -> %s", m.Type, m.Source, m.Target))
	}
	mountsStr := ""
	if len(mounts) > 0 {
		mountsStr = "\n" + strings.Join(mounts, "\n")
	}

	var maskedEnv []string
	if cc.Env != nil {
		var sensitivePatterns []string
		if resolved != nil {
			sensitivePatterns = resolved.SensitiveEnv
		}
		maskedEnv = config.MaskSensitiveEnvList(cc.Env, sensitivePatterns)
	}
	userStr := cc.User
	if userStr == "" {
		userStr = "(empty)"
	}

	o.logger.Debug("ContainerConfig:\n  Image:      %s\n  Command:    %v\n  Entrypoint: %v\n  Mounts:%s\n  Env:        %v\n  User:       %s",
		cc.Image, cc.Command, cc.Entrypoint, mountsStr, maskedEnv, userStr)
}

func (o *rootOptions) initContainer(ctx context.Context, resolved *config.ResolvedConfig, cc *container.ContainerConfig) (rt runtime.ContainerRuntime, containerID string, cleanup func(), err error) {
	// Log ContainerConfig at debug level
	o.logContainerConfig(resolved, cc)

	// Initialize Runtime
	rt, err = o.runtimeFactory(resolved.Runtime, resolved.SocketPath, o.logger)
	if err != nil {
		err = &config.RuntimeInitError{Runtime: resolved.Runtime, Err: err}
		return
	}

	// Ensure runtime is closed on early error paths
	defer func() {
		if err != nil && rt != nil {
			if closeErr := rt.Close(); closeErr != nil {
				o.logger.Debug("failed to close runtime on init failure: %v", closeErr)
			}
		}
	}()

	o.logger.Trace("Creating container...")
	if err = rt.PullImage(ctx, cc.Image, cc.Pull, resolved.PullMaxRetries, resolved.PullBackoffBase); err != nil {
		err = fmt.Errorf("failed to pull image: %w", err)
		return
	}

	containerID, err = rt.CreateContainer(ctx, cc)
	if err != nil {
		err = fmt.Errorf("failed to create container: %w", err)
		return
	}

	cleanup = func() {}
	if cc.Remove {
		cleanupCtx := context.WithoutCancel(ctx)
		cleanup = func() {
			o.logger.Trace("Removing container: %s", containerID)
			if err := rt.RemoveContainer(cleanupCtx, containerID); err != nil {
				o.logger.Warn("failed to remove container (defer): %v", err)
			}
		}
	}

	return
}
func (o *rootOptions) setupTerminal(stdinFd int, isHostStdinTerminal bool, cc *container.ContainerConfig) func() {
	o.logger.Debug("Host STDIN is terminal: %v", isHostStdinTerminal)

	// Set up terminal raw mode if TTY is requested and we are in a terminal
	if isHostStdinTerminal && cc.TTY {
		o.logger.Trace("Setting terminal to raw mode")
		state, err := o.makeRaw(stdinFd)
		if err != nil {
			o.logger.Warn("failed to set terminal to raw mode: %v", err)
		} else {
			return func() { _ = o.restore(stdinFd, state) } //nolint:errcheck
		}
	}
	return func() {}
}

func (o *rootOptions) startSignalForwarder(ctx context.Context, cancel context.CancelFunc, rt runtime.ContainerRuntime, containerID string) func() {
	sigChan := make(chan os.Signal, 1)
	o.setupSignals(sigChan)
	go func() {
		firstSignal := true
		for {
			select {
			case sig := <-sigChan:
				if firstSignal {
					sigName := getSignalName(sig)
					o.logger.Debug("Forwarding signal %v (%s) to container", sig, sigName)
					if err := rt.SignalContainer(ctx, containerID, sigName); err != nil {
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
			case <-ctx.Done():
				return
			}
		}
	}()
	return func() { o.stopSignalHandling(sigChan) }
}

func (o *rootOptions) attachContainer(ctx context.Context, cmd *cobra.Command, rt runtime.ContainerRuntime, containerID string, cc *container.ContainerConfig) (*attachResult, error) {
	// Attach to container IO concurrently
	var stdin io.Reader
	startSignal := make(chan struct{})
	if cc.Interactive {
		stdin = &syncReader{
			inner: cmd.InOrStdin(),
			ready: startSignal,
			ctx:   ctx,
		}
	}

	attachCtx, cancelAttach := context.WithCancel(ctx)
	attachReady := make(chan struct{})
	attachDone := make(chan error, 1)
	go func() {
		attachDone <- rt.AttachContainer(attachCtx, containerID, cc.TTY, stdin, cmd.OutOrStdout(), cmd.ErrOrStderr(), attachReady)
	}()

	// Give a tiny bit of time for the goroutine to reach AttachContainer call,
	// reducing race condition where container starts and finishes before attachment.
	var attachDoneConsumed bool
	select {
	case <-attachReady:
	case err := <-attachDone:
		attachDoneConsumed = true
		if err != nil {
			cancelAttach()
			return nil, fmt.Errorf("failed to attach to container: %w", err)
		}
	case <-ctx.Done():
		cancelAttach()
		return nil, ctx.Err()
	}

	return &attachResult{
		startSignal:        startSignal,
		attachDone:         attachDone,
		cancelAttach:       cancelAttach,
		attachDoneConsumed: attachDoneConsumed,
	}, nil
}

func (o *rootOptions) startResizeHandler(ctx context.Context, cmd *cobra.Command, rt runtime.ContainerRuntime, containerID string, cc *container.ContainerConfig) func() {
	if fd, ok := getFd(cmd.OutOrStdout()); ok && cc.TTY && o.isTerminal(fd) {
		resizeChan := make(chan os.Signal, 1)
		o.setupResizeSignal(resizeChan)

		handleResize := func() {
			w, h, err := o.termGetSize(fd)
			if err == nil && h >= 0 && w >= 0 {
				_ = rt.ResizeContainerTTY(ctx, containerID, uint(h), uint(w)) //nolint:gosec,errcheck
			}
		}

		go func() {
			for {
				select {
				case <-resizeChan:
					handleResize()
				case <-ctx.Done():
					return
				}
			}
		}()

		// Initial resize to match current terminal size
		handleResize()

		return func() { o.stopSignalHandling(resizeChan) }
	}
	return func() {}
}

func (o *rootOptions) waitForCompletion(ctx context.Context, cmd *cobra.Command, rt runtime.ContainerRuntime, containerID string, cc *container.ContainerConfig, resolved *config.ResolvedConfig, isHostStdinTerminal bool, att *attachResult) (int, error) {
	o.logger.Trace("Waiting for container: %s", containerID)

	effectiveHangTimeout := o.getHangTimeout(isHostStdinTerminal, cc.Interactive, resolved)

	type waitResult struct {
		code int
		err  error
	}
	waitDone := make(chan waitResult, 1)
	go func() {
		code, err := rt.WaitContainer(ctx, containerID)
		waitDone <- waitResult{code, err}
	}()

	var exitCode int
	select {
	case result := <-waitDone:
		if result.err != nil {
			o.logger.Debug("WaitContainer for %s failed or was interrupted: %v", containerID, result.err)
			return 0, &ExitCodeError{Code: 125, Err: fmt.Errorf("failed to wait for container: %w", result.err)}
		}
		exitCode = result.code
		o.logger.Debug("Container %s finished with exit code %d", containerID, exitCode)

		// After container exits, wait a short grace period for remaining output
		if !att.attachDoneConsumed {
			o.logger.Trace("Waiting for remaining output from container %s (grace period: %v)", containerID, o.attachGracePeriod)
			select {
			case err := <-att.attachDone:
				if err != nil && !errors.Is(err, context.Canceled) {
					o.logger.Debug("AttachContainer finished with error after container exit for %s: %v", containerID, err)
					code := exitCode
					if code == 0 {
						code = 125
					}
					return exitCode, &ExitCodeError{Code: code, Err: fmt.Errorf("failed to attach to container: %w", err)}
				}
				o.logger.Debug("AttachContainer finished successfully for %s", containerID)
			case <-time.After(o.attachGracePeriod):
				o.logger.Debug("AttachContainer timed out after container exit for %s, forcing close", containerID)
				att.cancelAttach()
				<-att.attachDone
			}
		}

	case err := <-att.attachDone:
		if err != nil && !errors.Is(err, context.Canceled) {
			o.logger.Debug("AttachContainer finished with error before container exit for %s: %v", containerID, err)
			// Wait for container to finish (best effort)
			// We do NOT call cancel() here to allow rt.WaitContainer to continue normally
			// until it finishes, the timeout expires, or a second signal is received.
			if effectiveHangTimeout > 0 {
				select {
				case res := <-waitDone:
					exitCode = res.code
				case <-time.After(effectiveHangTimeout):
					o.logger.Debug("Timeout waiting for container %s after attach error", containerID)
				}
			} else {
				res := <-waitDone
				exitCode = res.code
			}
			code := exitCode
			if code == 0 {
				code = 125
			}
			return exitCode, &ExitCodeError{Code: code, Err: fmt.Errorf("failed to attach to container: %w", err)}
		}
		o.logger.Debug("AttachContainer finished successfully before container exit for %s", containerID)

		// IO finished before container exited.
		if !isHostStdinTerminal || !cc.Interactive {
			if effectiveHangTimeout > 0 {
				o.logger.Trace("IO finished, waiting up to %v for container %s to exit", effectiveHangTimeout, containerID)
				select {
				case result := <-waitDone:
					if result.err != nil {
						return 0, &ExitCodeError{Code: 125, Err: fmt.Errorf("failed to wait for container: %w", result.err)}
					}
					exitCode = result.code
				case <-time.After(effectiveHangTimeout):
					killCtx, killCancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer killCancel()
					var err error
					exitCode, err = o.signalKillIfRunning(killCtx, rt, containerID)
					if err != nil {
						return 0, err
					}
					select {
					case result := <-waitDone:
						exitCode = result.code
					case <-time.After(effectiveHangTimeout):
						code := exitCode
						if code == 0 {
							code = 125
						}
						return exitCode, &ExitCodeError{Code: code, Err: fmt.Errorf("container %s failed to exit after SIGKILL timeout", containerID)}
					}
				}
			} else {
				// effectiveHangTimeout is 0, wait indefinitely
				o.logger.Trace("IO finished, waiting indefinitely for container %s to exit", containerID)
				result := <-waitDone
				if result.err != nil {
					return 0, &ExitCodeError{Code: 125, Err: fmt.Errorf("failed to wait for container: %w", result.err)}
				}
				exitCode = result.code
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

		if subcommand == "" {
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
		containerConfig, err := o.buildContainerConfig(resolved, passthroughArgs, toolsCfg)
		if err != nil {
			return fmt.Errorf("container configuration error: %w", err)
		}

		if resolved.DryRun {
			return o.handleDryRun(cmd, containerConfig, resolved)
		}

		// Create snapshot if nested execution support is requested or already active
		var snapshotDir string
		explicitNested := resolved.MountCderun || resolved.MountAllTools || len(resolved.MountTools) > 0
		if explicitNested || (globalCfg != nil && globalCfg.HostContext != nil) {
			o.logger.Debug("Creating execution snapshot for nested support...")
			// Ensure globalCfg is initialized for snapshot if it was nil
			if globalCfg == nil {
				globalCfg = &config.CDERunConfig{}
			}
			sDir, hostDir, err := createSnapshot(o.logger, o.fs, globalCfg, toolsCfg, containerConfig.Mounts)
			if err != nil {
				if explicitNested {
					return fmt.Errorf("failed to create snapshot for nested execution: %w", err)
				}
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
			var exitErr *ExitCodeError
			if errors.As(err, &exitErr) {
				return exitErr
			}
			return &ExitCodeError{Code: 125, Err: err}
		}
		if exitCode != 0 {
			return &ExitCodeError{Code: exitCode}
		}
		return nil
	}

	registerFlags(cmd, o)

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
	// Always create fresh state to avoid global state leaks.
	localOpts := defaultOptions()
	localOpts.logger = logging.NewLogger()

	cmd := newRootCmd(&localOpts)

	// Redirect logger to the command's error writer early to capture initial logs.
	localOpts.logger.SetOutput(cmd.ErrOrStderr())

	if setup != nil {
		setup(&localOpts, cmd)
		// Re-bind logger output in case setup() replaced the command's error writer.
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
				f := cmd.PersistentFlags().Lookup(name)
				if f == nil {
					f = cmd.Flags().Lookup(name)
				}
				if f != nil && f.NoOptDefVal == "" && !strings.Contains(arg, "=") {
					// Flag exists, takes an argument, and no '=' used, so skip next argument.
					i++
				}
			} else if len(arg) > 1 {
				// Shorthand(s), e.g., -i, -it, -p 80:80
				// For shorthand, we only handle the case where the last shorthand in the group takes an argument.
				lastChar := string(arg[len(arg)-1])
				f := cmd.PersistentFlags().ShorthandLookup(lastChar)
				if f == nil {
					f = cmd.Flags().ShorthandLookup(lastChar)
				}
				if f != nil && f.NoOptDefVal == "" {
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
	doubleDashFound := false

	// Scan all arguments after the executable name
	// In polyglot mode, everything after index 0 is after the subcommand.
	// In standard mode, only arguments after subcmdIdx are considered for hoisting P1 overrides.
	startIdx := 1
	if !isPolyglot && subcmdIdx != -1 {
		// Standard mode: hoist only from after the subcommand
		for i := 1; i <= subcmdIdx; i++ {
			if args[i] == "--" {
				doubleDashFound = true
			}
			others = append(others, args[i])
		}
		startIdx = subcmdIdx + 1
	}

	for i := startIdx; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			doubleDashFound = true
		}

		shouldHoist := !doubleDashFound && strings.HasPrefix(arg, "--cderun-")

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
