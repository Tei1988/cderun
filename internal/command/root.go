package command

import (
	"cderun/internal/config"
	"cderun/internal/container"
	"cderun/internal/logging"
	"cderun/internal/runtime"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"time"
	"strings"

	"github.com/docker/go-units"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"gopkg.in/yaml.v3"
)

type rootOptions struct {
	tty                 bool
	interactive         bool
	network             string
	mountSocket         string
	mountCderun         bool
	image               string
	remove              bool
	cderunTTY           bool
	cderunInteractive   bool
	cderunImage         string
	cderunNetwork       string
	cderunRemove        bool
	cderunRuntime       string
	cderunMountSocket   string
	cderunWorkdir       string
	cderunVolumes       []string
	cderunMountCderun    bool
	cderunMountTools     string
	cderunMountAllTools  bool
	runtimeName         string
	env                 []string
	cderunEnv           []string
	workdir             string
	volumes             []string
	mountTools          string
	mountAllTools       bool
	dryRun              bool
	dryRunFormat        string
	cderunDryRun        bool
	cderunDryRunFormat  string
	logLevel             string
	logFile              string
	logFormat            string
	logTee               bool
	logTimestamp         bool
	verbose              int
	cderunLogLevel       string
	cderunLogFile        string
	cderunLogFormat      string
	cderunLogTee         bool
	cderunVerbose        int

	// New fields
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
	tmpfs            []string
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
	cderunTmpfs      []string
	cderunDevices    []string
}

var (
	opts rootOptions

	// For testing
	exitFunc       = os.Exit
	runtimeFactory = func(name string, socket string) (runtime.ContainerRuntime, error) {
		switch name {
		case "docker":
			return runtime.NewDockerRuntime(socket)
		case "podman":
			return runtime.NewPodmanRuntime(socket)
		default:
			return nil, fmt.Errorf("unsupported runtime %q", name)
		}
	}
)

func (o *rootOptions) loadConfigs() (config.ToolsConfig, *config.CDERunConfig, string, string) {
	logging.Trace("Loading configurations...")
	globalCfg, globalPath, err := config.LoadCDERunConfig()
	if err != nil {
		logging.Warn("failed to load cderun config: %v", err)
	} else if globalPath != "" {
		logging.Debug("Loaded cderun config from: %s", globalPath)
	}

	toolsCfg, toolsPath, err := config.LoadToolsConfig()
	if err != nil {
		logging.Warn("failed to load tools config: %v", err)
	} else if toolsPath != "" {
		logging.Debug("Loaded tools config from: %s", toolsPath)
	}
	return toolsCfg, globalCfg, globalPath, toolsPath
}

func (o *rootOptions) resolveSettings(cmd *cobra.Command, subcommand string, toolsCfg config.ToolsConfig, globalCfg *config.CDERunConfig) (*config.ResolvedConfig, error) {
	cliOpts := config.CLIOptions{
		Image:                o.image,
		ImageSet:             cmd.Flags().Changed("image"),
		TTY:                  o.tty,
		TTYSet:               cmd.Flags().Changed("tty"),
		Interactive:          o.interactive,
		InteractiveSet:       cmd.Flags().Changed("interactive"),
		Network:              o.network,
		NetworkSet:           cmd.Flags().Changed("network"),
		CderunNetwork:        o.cderunNetwork,
		CderunNetworkSet:     cmd.Flags().Changed("cderun-network"),
		Remove:               o.remove,
		RemoveSet:            cmd.Flags().Changed("remove"),
		CderunRemove:         o.cderunRemove,
		CderunRemoveSet:      cmd.Flags().Changed("cderun-remove"),
		CderunTTY:            o.cderunTTY,
		CderunTTYSet:         cmd.Flags().Changed("cderun-tty"),
		CderunInteractive:    o.cderunInteractive,
		CderunInteractiveSet: cmd.Flags().Changed("cderun-interactive"),
		CderunImage:          o.cderunImage,
		CderunImageSet:       cmd.Flags().Changed("cderun-image"),
		Runtime:              o.runtimeName,
		RuntimeSet:           cmd.Flags().Changed("runtime"),
		CderunRuntime:        o.cderunRuntime,
		CderunRuntimeSet:     cmd.Flags().Changed("cderun-runtime"),
		MountSocket:          o.mountSocket,
		MountSocketSet:       cmd.Flags().Changed("mount-socket"),
		CderunMountSocket:    o.cderunMountSocket,
		CderunMountSocketSet: cmd.Flags().Changed("cderun-mount-socket"),
		Env:                  o.env,
		CderunEnv:            o.cderunEnv,
		Workdir:              o.workdir,
		WorkdirSet:           cmd.Flags().Changed("workdir"),
		CderunWorkdir:        o.cderunWorkdir,
		CderunWorkdirSet:     cmd.Flags().Changed("cderun-workdir"),
		Volumes:              o.volumes,
		CderunVolumes:        o.cderunVolumes,
		MountCderun:          o.mountCderun,
		MountCderunSet:       cmd.Flags().Changed("mount-cderun"),
		CderunMountCderun:    o.cderunMountCderun,
		CderunMountCderunSet: cmd.Flags().Changed("cderun-mount-cderun"),
		MountTools:           o.mountTools,
		CderunMountTools:      o.cderunMountTools,
		MountAllTools:         o.mountAllTools,
		CderunMountAllTools:   o.cderunMountAllTools,
		DryRun:                o.dryRun,
		DryRunSet:             cmd.Flags().Changed("dry-run"),
		CderunDryRun:          o.cderunDryRun,
		CderunDryRunSet:       cmd.Flags().Changed("cderun-dry-run"),
		DryRunFormat:          o.dryRunFormat,
		DryRunFormatSet:       cmd.Flags().Changed("dry-run-format"),
		CderunDryRunFormat:    o.cderunDryRunFormat,
		CderunDryRunFormatSet: cmd.Flags().Changed("cderun-dry-run-format"),
		LogLevel:              o.logLevel,
		LogLevelSet:           cmd.Flags().Changed("log-level"),
		LogFile:               o.logFile,
		LogFileSet:            cmd.Flags().Changed("log-file"),
		LogFormat:             o.logFormat,
		LogFormatSet:          cmd.Flags().Changed("log-format"),
		LogTee:                o.logTee,
		LogTeeSet:             cmd.Flags().Changed("log-tee"),
		LogTimestamp:          o.logTimestamp,
		LogTimestampSet:       cmd.Flags().Changed("log-timestamp"),
		Verbose:               o.verbose,
		CderunLogLevel:        o.cderunLogLevel,
		CderunLogLevelSet:     cmd.Flags().Changed("cderun-log-level"),
		CderunLogFile:         o.cderunLogFile,
		CderunLogFileSet:      cmd.Flags().Changed("cderun-log-file"),
		CderunLogFormat:       o.cderunLogFormat,
		CderunLogFormatSet:    cmd.Flags().Changed("cderun-log-format"),
		CderunLogTee:          o.cderunLogTee,
		CderunLogTeeSet:       cmd.Flags().Changed("cderun-log-tee"),
		CderunVerbose:         o.cderunVerbose,

		// New fields
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
		Tmpfs:               o.tmpfs,
		CderunTmpfs:         o.cderunTmpfs,
		Devices:             o.devices,
		CderunDevices:       o.cderunDevices,
	}

	return config.Resolve(subcommand, cliOpts, toolsCfg, globalCfg)
}

func (o *rootOptions) buildContainerConfig(resolved *config.ResolvedConfig, subcommand string, passthroughArgs []string, toolsCfg config.ToolsConfig) (*container.ContainerConfig, error) {
	// Build ContainerConfig
	containerConfig := &container.ContainerConfig{
		Image:       resolved.Image,
		Command:     []string{subcommand},
		Args:        passthroughArgs,
		TTY:         resolved.TTY,
		Interactive: resolved.Interactive,
		Network:     resolved.Network,
		Remove:      resolved.Remove,
		Volumes:     resolved.Volumes,
		Env:         resolved.Env,
		Workdir:     resolved.Workdir,
		User:        resolved.User,

		// New fields
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
		Tmpfs:      resolved.Tmpfs,
		Devices:    resolved.Devices,
	}

	// Handle mounting flags
	if resolved.MountCderun || resolved.MountAllTools || resolved.MountTools != "" {
		if !resolved.SocketSet {
			return nil, fmt.Errorf("--mount-cderun, --mount-tools, or --mount-all-tools requires --mount-socket")
		}
		exePath, err := os.Executable()
		if err != nil {
			return nil, fmt.Errorf("failed to get executable path: %w", err)
		}

		// Add binary mount
		containerConfig.Volumes = append(containerConfig.Volumes, container.VolumeMount{
			HostPath:      exePath,
			ContainerPath: "/usr/local/bin/cderun",
			ReadOnly:      true,
		})

		// Add socket mount
		containerConfig.Volumes = append(containerConfig.Volumes, container.VolumeMount{
			HostPath:      resolved.Socket,
			ContainerPath: resolved.Socket,
			ReadOnly:      false, // Socket needs to be writable
		})

		// Handle MountTools / MountAllTools
		if resolved.MountAllTools {
			if toolsCfg == nil || len(toolsCfg) == 0 {
				logging.Warn("--mount-all-tools specified but no tools defined in .tools.yaml")
			}
			for toolName := range toolsCfg {
				containerConfig.Volumes = append(containerConfig.Volumes, container.VolumeMount{
					HostPath:      exePath,
					ContainerPath: "/usr/local/bin/" + toolName,
					ReadOnly:      true,
				})
			}
		} else if resolved.MountTools != "" {
			tools := strings.Split(resolved.MountTools, ",")
			for _, toolName := range tools {
				toolName = strings.TrimSpace(toolName)
				if _, ok := toolsCfg[toolName]; !ok {
					return nil, fmt.Errorf("tool %q not found in tools config", toolName)
				}
				containerConfig.Volumes = append(containerConfig.Volumes, container.VolumeMount{
					HostPath:      exePath,
					ContainerPath: "/usr/local/bin/" + toolName,
					ReadOnly:      true,
				})
			}
		}
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
		Global string `json:"global" yaml:"global"`
		Tools  string `json:"tools" yaml:"tools"`
	} `json:"configs" yaml:"configs"`
	AvailableTools []string `json:"available_tools,omitempty" yaml:"available_tools,omitempty"`
}

func (o *rootOptions) handleDryRun(containerConfig *container.ContainerConfig, resolved *config.ResolvedConfig, toolsCfg config.ToolsConfig, globalPath, toolsPath string) error {
	if containerConfig == nil {
		// Global Dry Run / Diagnostics
		info := diagnosticsInfo{}
		info.Runtime.Name = resolved.Runtime
		info.Runtime.Socket = resolved.Socket
		if _, err := os.Stat(resolved.Socket); err == nil {
			info.Runtime.Status = "accessible"
		} else {
			info.Runtime.Status = fmt.Sprintf("not found or inaccessible: %v", err)
		}
		info.Configs.Global = globalPath
		info.Configs.Tools = toolsPath
		for toolName := range toolsCfg {
			info.AvailableTools = append(info.AvailableTools, toolName)
		}

		switch strings.ToLower(resolved.DryRunFormat) {
		case "json":
			data, err := json.MarshalIndent(info, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal JSON: %w", err)
			}
			fmt.Println(string(data))
		case "simple":
			fmt.Printf("Runtime: %s (%s)\n", info.Runtime.Name, info.Runtime.Socket)
			fmt.Printf("Runtime Status: %s\n", info.Runtime.Status)
			fmt.Printf("Global Config: %s\n", info.Configs.Global)
			fmt.Printf("Tools Config: %s\n", info.Configs.Tools)
			fmt.Printf("Available Tools: %s\n", strings.Join(info.AvailableTools, ", "))
		default: // Default to YAML
			data, err := yaml.Marshal(info)
			if err != nil {
				return fmt.Errorf("failed to marshal YAML: %w", err)
			}
			fmt.Print(string(data))
		}
		return nil
	}

	switch strings.ToLower(resolved.DryRunFormat) {
	case "json":
		data, err := json.MarshalIndent(containerConfig, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(data))
	case "simple":
		fmt.Printf("Image: %s\n", containerConfig.Image)
		fullCmd := strings.Join(containerConfig.Command, " ")
		if len(containerConfig.Args) > 0 {
			fullCmd += " " + strings.Join(containerConfig.Args, " ")
		}
		fmt.Printf("Command: %s\n", fullCmd)
		fmt.Printf("TTY: %v\n", containerConfig.TTY)
		fmt.Printf("Interactive: %v\n", containerConfig.Interactive)
		fmt.Printf("Network: %s\n", containerConfig.Network)
		fmt.Printf("Remove: %v\n", containerConfig.Remove)
		var volumes []string
		for _, v := range containerConfig.Volumes {
			volumes = append(volumes, fmt.Sprintf("%s:%s", v.HostPath, v.ContainerPath))
		}
		fmt.Printf("Volumes: %s\n", strings.Join(volumes, ", "))
		fmt.Printf("Env: %s\n", strings.Join(containerConfig.Env, ", "))
		fmt.Printf("Workdir: %s\n", containerConfig.Workdir)
		fmt.Printf("User: %s\n", containerConfig.User)
		fmt.Printf("Ports: %s\n", strings.Join(containerConfig.Ports, ", "))
		fmt.Printf("PublishAll: %v\n", containerConfig.PublishAll)
		fmt.Printf("Expose: %s\n", strings.Join(containerConfig.Expose, ", "))
		fmt.Printf("Hostname: %s\n", containerConfig.Hostname)
		fmt.Printf("DNS: %s\n", strings.Join(containerConfig.DNS, ", "))
		fmt.Printf("AddHosts: %s\n", strings.Join(containerConfig.AddHosts, ", "))
		fmt.Printf("Privileged: %v\n", containerConfig.Privileged)
		fmt.Printf("CapAdd: %s\n", strings.Join(containerConfig.CapAdd, ", "))
		fmt.Printf("CapDrop: %s\n", strings.Join(containerConfig.CapDrop, ", "))
		fmt.Printf("Entrypoint: %s\n", strings.Join(containerConfig.Entrypoint, ", "))
		fmt.Printf("Pull: %s\n", containerConfig.Pull)
		fmt.Printf("Memory: %s\n", units.BytesSize(float64(containerConfig.Memory)))
		fmt.Printf("CPUs: %g\n", containerConfig.CPUs)
		fmt.Printf("Tmpfs: %s\n", strings.Join(containerConfig.Tmpfs, ", "))
		fmt.Printf("Devices: %s\n", strings.Join(containerConfig.Devices, ", "))
	default: // Default to YAML
		data, err := yaml.Marshal(containerConfig)
		if err != nil {
			return fmt.Errorf("failed to marshal YAML: %w", err)
		}
		fmt.Print(string(data))
	}
	return nil
}

func (o *rootOptions) execute(ctx context.Context, resolved *config.ResolvedConfig, containerConfig *container.ContainerConfig) (int, error) {
	logging.Info("Running: %s %s", containerConfig.Command[0], strings.Join(containerConfig.Args, " "))
	logging.Debug("Image: %s", containerConfig.Image)
	logging.Debug("Runtime: %s", resolved.Runtime)
	logging.Debug("Socket: %s", resolved.Socket)

	ctxG, cancel := context.WithCancel(ctx)
	defer cancel()

	// Initialize Runtime
	rt, err := runtimeFactory(resolved.Runtime, resolved.Socket)
	if err != nil {
		return 0, fmt.Errorf("failed to initialize runtime: %w", err)
	}

	logging.Trace("Creating container...")
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
			logging.Trace("Removing container: %s", containerID)
			if err := rt.RemoveContainer(cleanupCtx, containerID); err != nil {
				logging.Warn("failed to remove container (defer): %v", err)
			}
		}()
	}

	// Set up terminal raw mode if TTY is requested and we are in a terminal
	if containerConfig.TTY && term.IsTerminal(int(os.Stdin.Fd())) {
		logging.Trace("Setting terminal to raw mode")
		state, err := term.MakeRaw(int(os.Stdin.Fd()))
		if err != nil {
			logging.Warn("failed to set terminal to raw mode: %v", err)
		} else {
			defer term.Restore(int(os.Stdin.Fd()), state)
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
					logging.Debug("Forwarding signal %v to container", sig)
					if err := rt.SignalContainer(ctxG, containerID, sigName); err != nil {
						logging.Warn("failed to forward signal %v: %v", sig, err)
					}
					firstSignal = false
				} else {
					logging.Info("Received second signal, terminating...")
					cancel()
					return
				}
			case <-ctxG.Done():
				return
			}
		}
	}()

	logging.Trace("Starting container: %s", containerID)
	if err := rt.StartContainer(ctx, containerID); err != nil {
		return 0, fmt.Errorf("failed to start container: %w", err)
	}

	// Handle window resize synchronization
	if containerConfig.TTY && term.IsTerminal(int(os.Stdout.Fd())) {
		resizeChan := make(chan os.Signal, 1)
		setupResizeSignal(resizeChan)
		defer signal.Stop(resizeChan)
		go func() {
			for {
				select {
				case <-resizeChan:
					w, h, err := term.GetSize(int(os.Stdout.Fd()))
					if err == nil {
						_ = rt.ResizeContainerTTY(ctxG, containerID, uint(h), uint(w))
					}
				case <-ctxG.Done():
					return
				}
			}
		}()

		// Initial resize to match current terminal size
		w, h, err := term.GetSize(int(os.Stdout.Fd()))
		if err == nil {
			_ = rt.ResizeContainerTTY(ctxG, containerID, uint(h), uint(w))
		}
	}

	// Attach to container IO concurrently
	var stdin io.Reader
	if containerConfig.Interactive {
		stdin = os.Stdin
	}

	attachCtx, cancelAttach := context.WithCancel(ctxG)
	defer cancelAttach()

	attachDone := make(chan error, 1)
	go func() {
		attachDone <- rt.AttachContainer(attachCtx, containerID, containerConfig.TTY, stdin, os.Stdout, os.Stderr)
	}()

	logging.Trace("Waiting for container: %s", containerID)
	exitCode, err := rt.WaitContainer(ctxG, containerID)
	if err != nil {
		return 0, fmt.Errorf("failed to wait for container: %w", err)
	}

	// After container exits, wait a short grace period for remaining output
	select {
	case err := <-attachDone:
		if err != nil && err != context.Canceled {
			return 0, fmt.Errorf("failed to attach to container: %w", err)
		}
	case <-time.After(500 * time.Millisecond):
		logging.Debug("AttachContainer timed out after container exit, forcing close")
		cancelAttach()
		<-attachDone
	}

	logging.Debug("Container exited with code: %d", exitCode)
	return exitCode, nil
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cderun",
		Short: "A wrapper tool to run commands in a containerized environment.",
		Long: `cderun is a CLI wrapper tool that simplifies running commands
within a container. It separates its own flags from the flags
intended for the subcommand.`,
		RunE: func(cmd *cobra.Command, args []string) error {
		// Early logger initialization with CLI and Environment settings before config loading.
		// This allows loadConfigs() to use the correct log level.
		initialLevel := "info"
		vLevel := opts.verbose
		if opts.cderunVerbose > vLevel {
			vLevel = opts.cderunVerbose
		}
		if vLevel >= 3 {
			initialLevel = "trace"
		} else if vLevel >= 2 {
			initialLevel = "debug"
		}
		if env := os.Getenv("CDERUN_LOG_LEVEL"); env != "" {
			initialLevel = env
		}
		if opts.cderunLogLevel != "" {
			initialLevel = opts.cderunLogLevel
		} else if opts.logLevel != "" {
			initialLevel = opts.logLevel
		}
		_ = logging.Init(initialLevel, "text", "", false, true)

		// Load configurations
		toolsCfg, globalCfg, globalPath, toolsPath := opts.loadConfigs()

		subcommand := ""
		passthroughArgs := []string{}
		if len(args) > 0 {
			subcommand = args[0]
			passthroughArgs = args[1:]
		}

		// Resolve settings using priority logic (CLI > Env > Config > Default)
		resolved, err := opts.resolveSettings(cmd, subcommand, toolsCfg, globalCfg)
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

		if len(args) == 0 && !resolved.DryRun {
			return cmd.Help()
		}

		// Re-initialize logger with fully resolved settings including those from config files.
		if err := logging.Init(resolved.LogLevel, resolved.LogFormat, resolved.LogFile, resolved.LogTee, resolved.LogTimestamp); err != nil {
			return fmt.Errorf("failed to initialize logger: %w", err)
		}
		logging.Debug("Logger initialized with level: %s", resolved.LogLevel)

		// Build ContainerConfig
		var containerConfig *container.ContainerConfig
		if subcommand != "" {
			containerConfig, err = opts.buildContainerConfig(resolved, subcommand, passthroughArgs, toolsCfg)
			if err != nil {
				return fmt.Errorf("container configuration error: %w", err)
			}
		}

		if resolved.DryRun {
			return opts.handleDryRun(containerConfig, resolved, toolsCfg, globalPath, toolsPath)
		}

		// Execute Container
		exitCode, err := opts.execute(cmd.Context(), resolved, containerConfig)
		if err != nil {
			return err
		}
			exitFunc(exitCode)
			return nil
		},
	}

	cmd.PersistentFlags().BoolVar(&opts.tty, "tty", false, "Allocate a pseudo-TTY")
	cmd.PersistentFlags().BoolVarP(&opts.interactive, "interactive", "i", false, "Keep STDIN open even if not attached")
	cmd.PersistentFlags().StringVar(&opts.network, "network", "bridge", "Connect a container to a network")
	cmd.PersistentFlags().StringVar(&opts.mountSocket, "mount-socket", "", "Mount container runtime socket (e.g., /var/run/docker.sock)")
	cmd.PersistentFlags().BoolVar(&opts.mountCderun, "mount-cderun", false, "Mount cderun binary for use inside container")
	cmd.PersistentFlags().StringVar(&opts.image, "image", "", "Docker image to use")
	cmd.PersistentFlags().StringVar(&opts.runtimeName, "runtime", "docker", "Container runtime to use (docker/podman)")
	cmd.PersistentFlags().StringSliceVarP(&opts.env, "env", "e", nil, "Set environment variables")
	cmd.PersistentFlags().StringVarP(&opts.workdir, "workdir", "w", "", "Working directory inside the container")
	cmd.PersistentFlags().StringSliceVarP(&opts.volumes, "volume", "v", nil, "Bind mount a volume")
	cmd.PersistentFlags().StringVar(&opts.mountTools, "mount-tools", "", "Mount specified tools into the container")
	cmd.PersistentFlags().BoolVar(&opts.mountAllTools, "mount-all-tools", false, "Mount all defined tools into the container")
	cmd.PersistentFlags().BoolVar(&opts.remove, "remove", true, "Automatically remove the container when it exits")

	// New fields
	cmd.PersistentFlags().StringSliceVarP(&opts.ports, "publish", "p", nil, "Publish a container's port(s) to the host")
	cmd.PersistentFlags().BoolVarP(&opts.publishAll, "publish-all", "P", false, "Publish all exposed ports to random ports")
	cmd.PersistentFlags().StringSliceVar(&opts.expose, "expose", nil, "Expose a port or a range of ports")
	cmd.PersistentFlags().StringVar(&opts.hostname, "hostname", "", "Container host name")
	cmd.PersistentFlags().StringSliceVar(&opts.dns, "dns", nil, "Set custom DNS servers")
	cmd.PersistentFlags().StringSliceVar(&opts.addHosts, "add-host", nil, "Add a custom host-to-IP mapping (host:ip)")
	cmd.PersistentFlags().StringVarP(&opts.user, "user", "u", "", "Username or UID (format: <name|uid>[:<group|gid>])")
	cmd.PersistentFlags().BoolVar(&opts.privileged, "privileged", false, "Give extended privileges to this container")
	cmd.PersistentFlags().StringSliceVar(&opts.capAdd, "cap-add", nil, "Add Linux capabilities")
	cmd.PersistentFlags().StringSliceVar(&opts.capDrop, "cap-drop", nil, "Drop Linux capabilities")
	cmd.PersistentFlags().StringSliceVar(&opts.entrypoint, "entrypoint", nil, "Overwrite the default ENTRYPOINT of the image")
	cmd.PersistentFlags().StringVar(&opts.pull, "pull", "missing", "Pull image before running (always, missing, never)")
	cmd.PersistentFlags().StringVarP(&opts.memory, "memory", "m", "", "Memory limit")
	cmd.PersistentFlags().Float64Var(&opts.cpus, "cpus", 0, "Number of CPUs")
	cmd.PersistentFlags().StringSliceVar(&opts.tmpfs, "tmpfs", nil, "Mount a tmpfs directory")
	cmd.PersistentFlags().StringSliceVar(&opts.devices, "device", nil, "Add a host device to the container")

	cmd.PersistentFlags().BoolVar(&opts.cderunTTY, "cderun-tty", false, "Override TTY setting (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().BoolVar(&opts.cderunInteractive, "cderun-interactive", false, "Override interactive setting (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().StringVar(&opts.cderunImage, "cderun-image", "", "Override image (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().StringVar(&opts.cderunNetwork, "cderun-network", "", "Override network setting (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().BoolVar(&opts.cderunRemove, "cderun-remove", true, "Override remove setting (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().StringVar(&opts.cderunRuntime, "cderun-runtime", "", "Override runtime setting (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().StringVar(&opts.cderunMountSocket, "cderun-mount-socket", "", "Override socket path (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().StringSliceVar(&opts.cderunEnv, "cderun-env", nil, "Override environment variables (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().StringVar(&opts.cderunWorkdir, "cderun-workdir", "", "Override workdir setting (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().StringSliceVar(&opts.cderunVolumes, "cderun-volume", nil, "Override volume mounts (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().BoolVar(&opts.cderunMountCderun, "cderun-mount-cderun", false, "Override mount-cderun setting (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().StringVar(&opts.cderunMountTools, "cderun-mount-tools", "", "Override mount-tools setting (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().BoolVar(&opts.cderunMountAllTools, "cderun-mount-all-tools", false, "Override mount-all-tools setting (highest priority, can be used after subcommand)")

	// New fields P1
	cmd.PersistentFlags().StringSliceVar(&opts.cderunPorts, "cderun-publish", nil, "Override publish setting (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().BoolVar(&opts.cderunPublishAll, "cderun-publish-all", false, "Override publish-all setting (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().StringSliceVar(&opts.cderunExpose, "cderun-expose", nil, "Override expose setting (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().StringVar(&opts.cderunHostname, "cderun-hostname", "", "Override hostname setting (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().StringSliceVar(&opts.cderunDNS, "cderun-dns", nil, "Override DNS setting (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().StringSliceVar(&opts.cderunAddHosts, "cderun-add-host", nil, "Override add-host setting (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().StringVar(&opts.cderunUser, "cderun-user", "", "Override user setting (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().BoolVar(&opts.cderunPrivileged, "cderun-privileged", false, "Override privileged setting (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().StringSliceVar(&opts.cderunCapAdd, "cderun-cap-add", nil, "Override cap-add setting (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().StringSliceVar(&opts.cderunCapDrop, "cderun-cap-drop", nil, "Override cap-drop setting (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().StringSliceVar(&opts.cderunEntrypoint, "cderun-entrypoint", nil, "Override entrypoint setting (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().StringVar(&opts.cderunPull, "cderun-pull", "", "Override pull setting (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().StringVar(&opts.cderunMemory, "cderun-memory", "", "Override memory setting (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().Float64Var(&opts.cderunCPUs, "cderun-cpus", 0, "Override cpus setting (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().StringSliceVar(&opts.cderunTmpfs, "cderun-tmpfs", nil, "Override tmpfs setting (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().StringSliceVar(&opts.cderunDevices, "cderun-device", nil, "Override device setting (highest priority, can be used after subcommand)")

	cmd.PersistentFlags().BoolVar(&opts.dryRun, "dry-run", false, "Preview container configuration without execution")
	cmd.PersistentFlags().StringVarP(&opts.dryRunFormat, "dry-run-format", "f", "yaml", "Output format (yaml, json, simple)")
	cmd.PersistentFlags().BoolVar(&opts.cderunDryRun, "cderun-dry-run", false, "Override dry-run setting (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().StringVar(&opts.cderunDryRunFormat, "cderun-dry-run-format", "", "Override dry-run-format setting (highest priority, can be used after subcommand)")

	cmd.PersistentFlags().CountVar(&opts.verbose, "verbose", "Enable verbose logging (--verbose: info, --verbose --verbose: debug, --verbose --verbose --verbose: trace)")
	cmd.PersistentFlags().StringVar(&opts.logLevel, "log-level", "", "Set log level (error, warn, info, debug, trace)")
	cmd.PersistentFlags().StringVar(&opts.logFile, "log-file", "", "Set log file path")
	cmd.PersistentFlags().StringVar(&opts.logFormat, "log-format", "text", "Set log format (text, json)")
	cmd.PersistentFlags().BoolVar(&opts.logTee, "log-tee", false, "Output log to both stderr and log file")
	cmd.PersistentFlags().BoolVar(&opts.logTimestamp, "log-timestamp", true, "Include timestamp in logs")

	cmd.PersistentFlags().StringVar(&opts.cderunLogLevel, "cderun-log-level", "", "Override log level (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().StringVar(&opts.cderunLogFile, "cderun-log-file", "", "Override log file path (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().StringVar(&opts.cderunLogFormat, "cderun-log-format", "", "Override log format (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().BoolVar(&opts.cderunLogTee, "cderun-log-tee", false, "Override log-tee setting (highest priority, can be used after subcommand)")
	cmd.PersistentFlags().CountVar(&opts.cderunVerbose, "cderun-verbose", "Override verbose level (highest priority, can be used after subcommand)")

	cmd.Flags().SetInterspersed(false)
	return cmd
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = newRootCmd()

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute(rawArgs []string) error {
	args, err := preprocessArgs(rawArgs)
	if err != nil {
		return err
	}
	if len(args) >= 1 {
		rootCmd.SetArgs(args[1:])
	} else {
		rootCmd.SetArgs([]string{})
	}
	return rootCmd.Execute()
}

func preprocessArgs(args []string) ([]string, error) {
	if len(args) == 0 {
		return args, nil
	}

	execName := filepath.Base(args[0])
	isPolyglot := execName != "cderun"

	// Find the subcommand index
	subcmdIdx := -1
	if isPolyglot {
		subcmdIdx = 0
	} else {
		for i := 1; i < len(args); i++ {
			if !strings.HasPrefix(args[i], "-") {
				subcmdIdx = i
				break
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

	newArgs := make([]string, 0, len(args)+1)
	if isPolyglot {
		newArgs = append(newArgs, "cderun")
	} else {
		newArgs = append(newArgs, args[0])
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
		if strings.HasPrefix(args[i], "--cderun-") {
			overrides = append(overrides, args[i])
		} else {
			others = append(others, args[i])
		}
	}

	// Place --cderun-* overrides immediately after "cderun" so they are always parsed
	newArgs = append(newArgs, overrides...)

	if isPolyglot {
		// In polyglot mode, the original executable name becomes the subcommand
		newArgs = append(newArgs, execName)
	}

	newArgs = append(newArgs, others...)

	return newArgs, nil
}

func init() {
	// Intentionally empty. All initialization is done in newRootCmd.
}
