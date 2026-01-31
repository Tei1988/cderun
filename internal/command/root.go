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
	"strings"

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

func (o *rootOptions) loadConfigs() (config.ToolsConfig, *config.CDERunConfig) {
	logging.Trace("Loading configurations...")
	globalCfg, path, err := config.LoadCDERunConfig()
	if err != nil {
		logging.Warn("failed to load cderun config: %v", err)
	} else if path != "" {
		logging.Debug("Loaded cderun config from: %s", path)
	}

	toolsCfg, path, err := config.LoadToolsConfig()
	if err != nil {
		logging.Warn("failed to load tools config: %v", err)
	} else if path != "" {
		logging.Debug("Loaded tools config from: %s", path)
	}
	return toolsCfg, globalCfg
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

func (o *rootOptions) handleDryRun(containerConfig *container.ContainerConfig, dryRunFormat string) error {
	switch strings.ToLower(dryRunFormat) {
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
		for {
			select {
			case sig := <-sigChan:
				sigName := getSignalName(sig)
				if err := rt.SignalContainer(ctxG, containerID, sigName); err != nil {
					logging.Warn("failed to forward signal %v: %v", sig, err)
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

	// Attach to container IO
	var stdin io.Reader
	if containerConfig.Interactive {
		stdin = os.Stdin
	}
	if err := rt.AttachContainer(ctx, containerID, containerConfig.TTY, stdin, os.Stdout, os.Stderr); err != nil {
		return 0, fmt.Errorf("failed to attach to container: %w", err)
	}

	logging.Trace("Waiting for container: %s", containerID)
	exitCode, err := rt.WaitContainer(ctx, containerID)
	if err != nil {
		return 0, fmt.Errorf("failed to wait for container: %w", err)
	}

	logging.Debug("Container exited with code: %d", exitCode)
	return exitCode, nil
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "cderun",
	Short: "A wrapper tool to run commands in a containerized environment.",
	Long: `cderun is a CLI wrapper tool that simplifies running commands
within a container. It separates its own flags from the flags
intended for the subcommand.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}

		// The first non-flag argument is the subcommand
		subcommand := args[0]
		passthroughArgs := args[1:]

		// Load configurations
		toolsCfg, globalCfg := opts.loadConfigs()

		// Resolve settings using priority logic
		resolved, err := opts.resolveSettings(cmd, subcommand, toolsCfg, globalCfg)
		if err != nil {
			return fmt.Errorf("configuration error: %w", err)
		}

		// Initialize logger with resolved settings
		if err := logging.Init(resolved.LogLevel, resolved.LogFormat, resolved.LogFile, resolved.LogTee, resolved.LogTimestamp); err != nil {
			return fmt.Errorf("failed to initialize logger: %w", err)
		}
		logging.Debug("Logger initialized with level: %s", resolved.LogLevel)

		// Build ContainerConfig
		containerConfig, err := opts.buildContainerConfig(resolved, subcommand, passthroughArgs, toolsCfg)
		if err != nil {
			return fmt.Errorf("container configuration error: %w", err)
		}

		if resolved.DryRun {
			return opts.handleDryRun(containerConfig, resolved.DryRunFormat)
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
	logging.Trace("Preprocessing args: %v", args)
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
	rootCmd.PersistentFlags().BoolVar(&opts.tty, "tty", false, "Allocate a pseudo-TTY")
	rootCmd.PersistentFlags().BoolVarP(&opts.interactive, "interactive", "i", false, "Keep STDIN open even if not attached")
	rootCmd.PersistentFlags().StringVar(&opts.network, "network", "bridge", "Connect a container to a network")
	rootCmd.PersistentFlags().StringVar(&opts.mountSocket, "mount-socket", "", "Mount container runtime socket (e.g., /var/run/docker.sock)")
	rootCmd.PersistentFlags().BoolVar(&opts.mountCderun, "mount-cderun", false, "Mount cderun binary for use inside container")
	rootCmd.PersistentFlags().StringVar(&opts.image, "image", "", "Docker image to use")
	rootCmd.PersistentFlags().StringVar(&opts.runtimeName, "runtime", "docker", "Container runtime to use (docker/podman)")
	rootCmd.PersistentFlags().StringSliceVarP(&opts.env, "env", "e", nil, "Set environment variables")
	rootCmd.PersistentFlags().StringVarP(&opts.workdir, "workdir", "w", "", "Working directory inside the container")
	rootCmd.PersistentFlags().StringSliceVarP(&opts.volumes, "volume", "v", nil, "Bind mount a volume")
	rootCmd.PersistentFlags().StringVar(&opts.mountTools, "mount-tools", "", "Mount specified tools into the container")
	rootCmd.PersistentFlags().BoolVar(&opts.mountAllTools, "mount-all-tools", false, "Mount all defined tools into the container")
	rootCmd.PersistentFlags().BoolVar(&opts.remove, "remove", true, "Automatically remove the container when it exits")
	rootCmd.PersistentFlags().BoolVar(&opts.cderunTTY, "cderun-tty", false, "Override TTY setting (highest priority, can be used after subcommand)")
	rootCmd.PersistentFlags().BoolVar(&opts.cderunInteractive, "cderun-interactive", false, "Override interactive setting (highest priority, can be used after subcommand)")
	rootCmd.PersistentFlags().StringVar(&opts.cderunImage, "cderun-image", "", "Override image (highest priority, can be used after subcommand)")
	rootCmd.PersistentFlags().StringVar(&opts.cderunNetwork, "cderun-network", "", "Override network setting (highest priority, can be used after subcommand)")
	rootCmd.PersistentFlags().BoolVar(&opts.cderunRemove, "cderun-remove", true, "Override remove setting (highest priority, can be used after subcommand)")
	rootCmd.PersistentFlags().StringVar(&opts.cderunRuntime, "cderun-runtime", "", "Override runtime setting (highest priority, can be used after subcommand)")
	rootCmd.PersistentFlags().StringVar(&opts.cderunMountSocket, "cderun-mount-socket", "", "Override socket path (highest priority, can be used after subcommand)")
	rootCmd.PersistentFlags().StringSliceVar(&opts.cderunEnv, "cderun-env", nil, "Override environment variables (highest priority, can be used after subcommand)")
	rootCmd.PersistentFlags().StringVar(&opts.cderunWorkdir, "cderun-workdir", "", "Override workdir setting (highest priority, can be used after subcommand)")
	rootCmd.PersistentFlags().StringSliceVar(&opts.cderunVolumes, "cderun-volume", nil, "Override volume mounts (highest priority, can be used after subcommand)")
	rootCmd.PersistentFlags().BoolVar(&opts.cderunMountCderun, "cderun-mount-cderun", false, "Override mount-cderun setting (highest priority, can be used after subcommand)")
	rootCmd.PersistentFlags().StringVar(&opts.cderunMountTools, "cderun-mount-tools", "", "Override mount-tools setting (highest priority, can be used after subcommand)")
	rootCmd.PersistentFlags().BoolVar(&opts.cderunMountAllTools, "cderun-mount-all-tools", false, "Override mount-all-tools setting (highest priority, can be used after subcommand)")
	rootCmd.PersistentFlags().BoolVar(&opts.dryRun, "dry-run", false, "Preview container configuration without execution")
	rootCmd.PersistentFlags().StringVarP(&opts.dryRunFormat, "dry-run-format", "f", "yaml", "Output format (yaml, json, simple)")
	rootCmd.PersistentFlags().BoolVar(&opts.cderunDryRun, "cderun-dry-run", false, "Override dry-run setting (highest priority, can be used after subcommand)")
	rootCmd.PersistentFlags().StringVar(&opts.cderunDryRunFormat, "cderun-dry-run-format", "", "Override dry-run-format setting (highest priority, can be used after subcommand)")

	rootCmd.PersistentFlags().CountVar(&opts.verbose, "verbose", "Enable verbose logging (--verbose: info, --verbose --verbose: debug, --verbose --verbose --verbose: trace)")
	rootCmd.PersistentFlags().StringVar(&opts.logLevel, "log-level", "", "Set log level (error, warn, info, debug, trace)")
	rootCmd.PersistentFlags().StringVar(&opts.logFile, "log-file", "", "Set log file path")
	rootCmd.PersistentFlags().StringVar(&opts.logFormat, "log-format", "text", "Set log format (text, json)")
	rootCmd.PersistentFlags().BoolVar(&opts.logTee, "log-tee", false, "Output log to both stderr and log file")
	rootCmd.PersistentFlags().BoolVar(&opts.logTimestamp, "log-timestamp", true, "Include timestamp in logs")

	rootCmd.PersistentFlags().StringVar(&opts.cderunLogLevel, "cderun-log-level", "", "Override log level (highest priority, can be used after subcommand)")
	rootCmd.PersistentFlags().StringVar(&opts.cderunLogFile, "cderun-log-file", "", "Override log file path (highest priority, can be used after subcommand)")
	rootCmd.PersistentFlags().StringVar(&opts.cderunLogFormat, "cderun-log-format", "", "Override log format (highest priority, can be used after subcommand)")
	rootCmd.PersistentFlags().BoolVar(&opts.cderunLogTee, "cderun-log-tee", false, "Override log-tee setting (highest priority, can be used after subcommand)")
	rootCmd.PersistentFlags().CountVar(&opts.cderunVerbose, "cderun-verbose", "Override verbose level (highest priority, can be used after subcommand)")

	rootCmd.Flags().SetInterspersed(false)
}
