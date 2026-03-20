package command

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"
	"gopkg.in/yaml.v3"

	"cderun/internal/config"
	"cderun/internal/container"
	"cderun/internal/logging"
	"cderun/internal/runtime"
	"cderun/internal/version"
)

const hangTimeout = 2 * time.Second

type rootOptions struct {
	// Options matching flags
	tty               bool
	interactive       bool
	network           string
	socketPath        string
	mountSocket       bool
	mountSocketPath   string
	mountCderun       bool
	mountCderunPath   string
	mountAllTools     bool
	mountTools        string
	remove            bool
	image             string
	runtimeName       string
	workdir           string
	configPath        string
	toolConfigPath    string
	hostname          string
	user              string
	pull              string
	memory            string
	dryRun            bool
	dryRunFormat      string
	diagnosis         bool
	diagnosisFormat   string
	logLevel          string
	logFormat         string
	logTimestamp      bool
	hangTimeout       string
	strictEnv         bool
	publishAll        bool
	privileged        bool

	// String slices for multi-value flags
	env        []string
	mounts     []string
	ports      []string
	expose     []string
	dns        []string
	addHosts   []string
	capAdd     []string
	capDrop    []string
	entrypoint []string
	devices    []string

	// Float64 flags
	cpus float64

	// Internal override flags (P1)
	cderunTTY             bool
	cderunInteractive     bool
	cderunNetwork         string
	cderunSocketPath      string
	cderunMountSocket     bool
	cderunMountSocketPath string
	cderunMountCderun     bool
	cderunMountCderunPath string
	cderunMountAllTools   bool
	cderunMountTools      string
	cderunRemove          bool
	cderunImage           string
	cderunRuntime         string
	cderunWorkdir         string
	cderunConfigPath      string
	cderunToolConfigPath  string
	cderunHostname        string
	cderunUser            string
	cderunPull            string
	cderunMemory          string
	cderunDryRun          bool
	cderunDryRunFormat    string
	cderunDiagnosis       bool
	cderunDiagnosisFormat string
	cderunLogLevel        string
	cderunLogFormat       string
	cderunLogTimestamp    bool
	cderunHangTimeout     string
	cderunStrictEnv       bool
	cderunPublishAll      bool
	cderunPrivileged      bool

	cderunEnv        []string
	cderunMounts     []string
	cderunPorts      []string
	cderunExpose     []string
	cderunDNS        []string
	cderunAddHosts   []string
	cderunCapAdd     []string
	cderunCapDrop    []string
	cderunEntrypoint []string
	cderunDevices    []string

	cderunCPUs float64

	// Injectable dependencies
	fs                config.FileSystem
	configLoader      *config.ConfigLoader
	runtimeFactory    func(name, socket string) (runtime.ContainerRuntime, error)
	logger            *logging.Logger
	exitFunc          func(code int)
	isTerminal        func(fd int) bool
	termGetSize       func(fd int) (int, int, error)
	attachGracePeriod time.Duration
}

var (
	opts    rootOptions
	rootCmd *cobra.Command
)

func defaultOptions() rootOptions {
	return rootOptions{
		fs: config.RealFileSystem{},
		exitFunc: func(code int) {
			os.Exit(code)
		},
		runtimeFactory:    runtime.NewRuntime,
		logger:            logging.NewLogger(),
		isTerminal:        func(fd int) bool { return term.IsTerminal(fd) },
		termGetSize:       term.GetSize,
		attachGracePeriod: 5 * time.Second,
	}
}

func (o *rootOptions) loadConfigs(cmd *cobra.Command) (config.ToolsConfig, *config.CDERunConfig, []string, []string, error) {
	o.logger.Trace("Loading configurations...")

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
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("failed to load cderun config from %s: %w", cderunPath, err)
		}
	} else {
		globalCfg, globalPaths, err = o.configLoader.LoadCDERunConfig()
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("failed to load cderun config: %w", err)
		}
	}

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
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("failed to load tools config from %s: %w", toolsPath, err)
		}
	} else {
		toolsCfg, toolsPaths, err = o.configLoader.LoadToolsConfig()
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("failed to load tools config: %w", err)
		}
	}

	return toolsCfg, globalCfg, globalPaths, toolsPaths, nil
}

func (o *rootOptions) resolveSettings(cmd *cobra.Command, subcommand string, tools config.ToolsConfig, global *config.CDERunConfig) (*config.ResolvedConfig, error) {
	cli := config.CLIOptions{
		TTY:                   o.tty,
		TTYSet:                cmd.Flags().Changed("tty"),
		Interactive:           o.interactive,
		InteractiveSet:        cmd.Flags().Changed("interactive"),
		Network:               o.network,
		NetworkSet:            cmd.Flags().Changed("network"),
		SocketPath:            o.socketPath,
		SocketPathSet:         cmd.Flags().Changed("socket-path"),
		MountSocket:           o.mountSocket,
		MountSocketSet:        cmd.Flags().Changed("mount-socket"),
		MountSocketPath:       o.mountSocketPath,
		MountSocketPathSet:    cmd.Flags().Changed("mount-socket-path"),
		MountCderun:           o.mountCderun,
		MountCderunSet:        cmd.Flags().Changed("mount-cderun"),
		MountCderunPath:       o.mountCderunPath,
		MountCderunPathSet:    cmd.Flags().Changed("mount-cderun-path"),
		MountAllTools:         o.mountAllTools,
		MountAllToolsSet:      cmd.Flags().Changed("mount-all-tools"),
		MountTools:            o.mountTools,
		Remove:                o.remove,
		RemoveSet:             cmd.Flags().Changed("remove"),
		Image:                 o.image,
		Runtime:               o.runtimeName,
		Workdir:               o.workdir,
		Hostname:              o.hostname,
		User:                  o.user,
		UserSet:               cmd.Flags().Changed("user"),
		Pull:                  o.pull,
		PullSet:               cmd.Flags().Changed("pull"),
		Memory:                o.memory,
		MemorySet:             cmd.Flags().Changed("memory"),
		CPUs:                  o.cpus,
		CPUsSet:               cmd.Flags().Changed("cpus"),
		DryRun:                o.dryRun,
		DryRunSet:             cmd.Flags().Changed("dry-run"),
		DryRunFormat:          o.dryRunFormat,
		Diagnosis:             o.diagnosis,
		DiagnosisSet:          cmd.Flags().Changed("diagnosis"),
		DiagnosisFormat:       o.diagnosisFormat,
		LogLevel:              o.logLevel,
		LogFormat:             o.logFormat,
		LogTimestamp:          o.logTimestamp,
		LogTimestampSet:       cmd.Flags().Changed("log-timestamp"),
		HangTimeout:           o.hangTimeout,
		StrictEnv:             o.strictEnv,
		StrictEnvSet:          cmd.Flags().Changed("strict-env"),
		PublishAll:            o.publishAll,
		PublishAllSet:         cmd.Flags().Changed("publish-all"),
		Privileged:            o.privileged,
		PrivilegedSet:         cmd.Flags().Changed("privileged"),

		Env:        o.env,
		Mounts:     o.mounts,
		Publish:    o.ports,
		Expose:     o.expose,
		DNS:        o.dns,
		AddHosts:   o.addHosts,
		CapAdd:     o.capAdd,
		CapDrop:    o.capDrop,
		Entrypoint: o.entrypoint,
		Devices:    o.devices,

		CderunTTY:                o.cderunTTY,
		CderunTTYSet:             cmd.Flags().Changed("cderun-tty"),
		CderunInteractive:        o.cderunInteractive,
		CderunInteractiveSet:     cmd.Flags().Changed("cderun-interactive"),
		CderunNetwork:            o.cderunNetwork,
		CderunNetworkSet:         cmd.Flags().Changed("cderun-network"),
		CderunSocketPath:         o.cderunSocketPath,
		CderunSocketPathSet:      cmd.Flags().Changed("cderun-socket-path"),
		CderunMountSocket:        o.cderunMountSocket,
		CderunMountSocketSet:     cmd.Flags().Changed("cderun-mount-socket"),
		CderunMountSocketPath:    o.cderunMountSocketPath,
		CderunMountSocketPathSet: cmd.Flags().Changed("cderun-mount-socket-path"),
		CderunMountCderun:        o.cderunMountCderun,
		CderunMountCderunSet:     cmd.Flags().Changed("cderun-mount-cderun"),
		CderunMountCderunPath:    o.cderunMountCderunPath,
		CderunMountCderunPathSet: cmd.Flags().Changed("cderun-mount-cderun-path"),
		CderunMountAllTools:      o.cderunMountAllTools,
		CderunMountAllToolsSet:   cmd.Flags().Changed("cderun-mount-all-tools"),
		CderunMountTools:         o.cderunMountTools,
		CderunRemove:             o.cderunRemove,
		CderunRemoveSet:          cmd.Flags().Changed("cderun-remove"),
		CderunImage:              o.cderunImage,
		CderunRuntime:            o.cderunRuntime,
		CderunWorkdir:            o.cderunWorkdir,
		CderunHostname:           o.cderunHostname,
		CderunUser:               o.cderunUser,
		CderunUserSet:            cmd.Flags().Changed("cderun-user"),
		CderunPull:               o.cderunPull,
		CderunPullSet:            cmd.Flags().Changed("cderun-pull"),
		CderunMemory:             o.cderunMemory,
		CderunMemorySet:          cmd.Flags().Changed("cderun-memory"),
		CderunCPUs:               o.cderunCPUs,
		CderunCPUsSet:            cmd.Flags().Changed("cderun-cpus"),
		CderunDryRun:             o.cderunDryRun,
		CderunDryRunSet:          cmd.Flags().Changed("cderun-dry-run"),
		CderunDryRunFormat:       o.cderunDryRunFormat,
		CderunDiagnosis:          o.cderunDiagnosis,
		CderunDiagnosisSet:       cmd.Flags().Changed("cderun-diagnosis"),
		CderunDiagnosisFormat:    o.cderunDiagnosisFormat,
		CderunLogLevel:           o.cderunLogLevel,
		CderunLogFormat:          o.cderunLogFormat,
		CderunLogTimestamp:       o.cderunLogTimestamp,
		CderunLogTimestampSet:    cmd.Flags().Changed("cderun-log-timestamp"),
		CderunHangTimeout:        o.cderunHangTimeout,
		CderunStrictEnv:          o.cderunStrictEnv,
		CderunStrictEnvSet:       cmd.Flags().Changed("cderun-strict-env"),
		CderunPublishAll:         o.cderunPublishAll,
		CderunPublishAllSet:      cmd.Flags().Changed("cderun-publish-all"),
		CderunPrivileged:         o.cderunPrivileged,
		CderunPrivilegedSet:      cmd.Flags().Changed("cderun-privileged"),

		CderunEnv:        o.cderunEnv,
		CderunMounts:     o.cderunMounts,
		CderunPublish:    o.cderunPorts,
		CderunExpose:     o.cderunExpose,
		CderunDNS:        o.cderunDNS,
		CderunAddHosts:   o.cderunAddHosts,
		CderunCapAdd:     o.cderunCapAdd,
		CderunCapDrop:    o.cderunCapDrop,
		CderunEntrypoint: o.cderunEntrypoint,
		CderunDevices:    o.cderunDevices,
	}

	return config.ResolveWithFS(subcommand, cli, tools, global, o.fs)
}

func (o *rootOptions) buildContainerConfig(resolved *config.ResolvedConfig, args []string, toolsCfg config.ToolsConfig) (*container.ContainerConfig, error) {
	cc := &container.ContainerConfig{
		Image:       resolved.Image,
		Command:     args,
		Env:         resolved.Env,
		Mounts:      resolved.Mounts,
		Ports:       resolved.Publish,
		Expose:      resolved.Expose,
		Hostname:    resolved.Hostname,
		User:        resolved.User,
		Network:     resolved.Network,
		TTY:         resolved.TTY,
		Interactive: resolved.Interactive,
		Workdir:     resolved.Workdir,
		Memory:      resolved.Memory,
		CPUs:        0, // Set later
		DNS:         resolved.DNS,
		AddHosts:    resolved.AddHosts,
		Privileged:  resolved.Privileged,
		CapAdd:      resolved.CapAdd,
		CapDrop:     resolved.CapDrop,
		Entrypoint:  resolved.Entrypoint,
		Devices:     resolved.Devices,
		PublishAll:  resolved.PublishAll,
		Pull:        resolved.Pull,
	}
	if resolved.CPUs != nil {
		cc.CPUs = *resolved.CPUs
	}

	if resolved.MountCderun || resolved.MountAllTools || len(resolved.MountTools) > 0 {
		exePath := resolved.MountCderunPath
		if exePath == "" {
			var err error
			exePath, err = o.fs.Executable()
			if err != nil {
				return nil, fmt.Errorf("failed to get executable path: %w", err)
			}
		}

		if resolved.MountCderunPath == "" && resolved.HostContext != nil && resolved.HostContext.Level > 0 {
			r, err := config.NewExpressionResolverWithFS(resolved.HostContext, o.fs)
			if err != nil {
				o.logger.Debug("Failed to create expression resolver for nested execution (best-effort): %v. HostContext: %+v, exePath: %q", err, resolved.HostContext, exePath)
			} else {
				resolvedPath, err := config.ResolvePath(o.fs, exePath, "", r)
				if err != nil {
					o.logger.Debug("Failed to resolve exePath for nested execution (best-effort): %v. exePath: %q, HostContext: %+v", err, exePath, resolved.HostContext)
				} else {
					exePath = resolvedPath
				}
			}
		}

		if resolved.MountCderun {
			cc.Mounts = append(cc.Mounts, container.Mount{
				Type:     "bind",
				Source:   exePath,
				Target:   "/usr/local/bin/cderun",
				ReadOnly: true,
			})
		}

		if resolved.MountAllTools {
			for name, tool := range toolsCfg {
				if tool.Image != "" {
					cc.Mounts = append(cc.Mounts, container.Mount{
						Type:     "bind",
						Source:   exePath,
						Target:   "/usr/local/bin/" + name,
						ReadOnly: true,
					})
				}
			}
		} else {
			if len(resolved.MountTools) > 0 {
				for _, name := range resolved.MountTools {
					cc.Mounts = append(cc.Mounts, container.Mount{
						Type:     "bind",
						Source:   exePath,
						Target:   "/usr/local/bin/" + name,
						ReadOnly: true,
					})
				}
			}
		}
	}

	if resolved.MountSocket {
		cc.Mounts = append(cc.Mounts, container.Mount{
			Type:     "bind",
			Source:   resolved.SocketPath,
			Target:   resolved.SocketPath,
			ReadOnly: false,
		})
	}

	return cc, nil
}

func (o *rootOptions) execute(cmd *cobra.Command, resolved *config.ResolvedConfig, containerConfig *container.ContainerConfig) (int, error) {
	rt, err := o.runtimeFactory(resolved.Runtime, resolved.SocketPath)
	if err != nil {
		return 0, fmt.Errorf("failed to initialize container runtime: %w", err)
	}

	ctx := cmd.Context()

	if resolved.Pull != "never" {
		o.logger.Info("Pulling image: %s...", containerConfig.Image)
		err = rt.PullImage(ctx, containerConfig.Image, resolved.Pull)
		if err != nil {
			return 0, fmt.Errorf("failed to pull image: %w", err)
		}
	}

	o.logger.Debug("Creating container with config: %+v", containerConfig)
	containerID, err := rt.CreateContainer(ctx, containerConfig)
	if err != nil {
		return 0, fmt.Errorf("failed to create container: %w", err)
	}
	o.logger.Debug("Container created: %s", containerID)

	if resolved.Remove {
		defer func() {
			o.logger.Debug("Removing container: %s", containerID)
			if err := rt.RemoveContainer(context.Background(), containerID); err != nil {
				o.logger.Warn("failed to remove container: %v", err)
			}
		}()
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, os.Kill)
	defer signal.Stop(sigChan)

	go func() {
		for sig := range sigChan {
			o.logger.Debug("Received signal: %v, forwarding to container", sig)
			if err := rt.SignalContainer(context.Background(), containerID, sig.String()); err != nil {
				o.logger.Warn("failed to signal container: %v", err)
			}
		}
	}()

	if err := rt.StartContainer(ctx, containerID); err != nil {
		return 0, fmt.Errorf("failed to start container: %w", err)
	}

	waitDone := make(chan struct {
		code int
		err  error
	}, 1)
	go func() {
		code, err := rt.WaitContainer(ctx, containerID)
		waitDone <- struct { code int; err  error }{code, err}
	}()

	ready := make(chan struct{})
	attachErrChan := make(chan error, 1)
	go func() {
		attachErrChan <- rt.AttachContainer(ctx, containerID, resolved.TTY, cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr(), ready)
	}()

	select {
	case <-ready:
	case err := <-attachErrChan:
		if err != nil {
			return 0, fmt.Errorf("failed to attach to container: %w", err)
		}
	case <-time.After(5 * time.Second):
		o.logger.Debug("Timeout waiting for container attach ready, proceeding...")
	}

	var exitCode int
	select {
	case result := <-waitDone:
		if result.err != nil {
			return 0, fmt.Errorf("failed to wait for container: %w", result.err)
		}
		exitCode = result.code
	case <-ctx.Done():
		return 0, ctx.Err()
	}

	return exitCode, nil
}

func (o *rootOptions) handleDryRun(cmd *cobra.Command, containerConfig *container.ContainerConfig, resolved *config.ResolvedConfig) error {
	switch resolved.DryRunFormat {
	case "json":
		data, err := json.MarshalIndent(containerConfig, "", "  ")
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
	case "yaml":
		data, err := yaml.Marshal(containerConfig)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
	default: // simple
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Image: %s\n", containerConfig.Image)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Command: %s\n", strings.Join(containerConfig.Command, " "))
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Network: %s\n", containerConfig.Network)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "TTY: %v\n", containerConfig.TTY)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Interactive: %v\n", containerConfig.Interactive)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "User: %s\n", containerConfig.User)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Workdir: %s\n", containerConfig.Workdir)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Env: %s\n", strings.Join(containerConfig.Env, ", "))
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Pull: %s\n", containerConfig.Pull)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Memory: %d\n", containerConfig.Memory)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "CPUs: %g\n", containerConfig.CPUs)
		var mounts []string
		for _, m := range containerConfig.Mounts {
			mounts = append(mounts, fmt.Sprintf("%s:%s:%v", m.Source, m.Target, m.ReadOnly))
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Mounts: %s\n", strings.Join(mounts, ", "))
	}
	return nil
}

func (o *rootOptions) handleDiagnosis(cmd *cobra.Command, resolved *config.ResolvedConfig, toolsCfg config.ToolsConfig, globalPaths, toolsPaths []string) error {
	diag := struct {
		Resolved    *config.ResolvedConfig `json:"resolved" yaml:"resolved"`
		GlobalPaths []string               `json:"globalPaths" yaml:"globalPaths"`
		ToolsPaths  []string               `json:"toolsPaths" yaml:"toolsPaths"`
	}{
		Resolved:    resolved,
		GlobalPaths: globalPaths,
		ToolsPaths:  toolsPaths,
	}

	switch resolved.DiagnosisFormat {
	case "json":
		data, err := json.MarshalIndent(diag, "", "  ")
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
	default: // text/yaml
		data, err := yaml.Marshal(diag)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
	}
	return nil
}

func newRootCmd(o *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Version:       version.Info(),
		Use:           "cderun",
		SilenceUsage:  true,
		SilenceErrors: true,
		Short:         "A wrapper tool to run commands in a containerized environment.",
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
		_ = o.logger.Init(initialLevel, "text", true)

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

		resolved, err := o.resolveSettings(cmd, subcommand, toolsCfg, globalCfg)
		if err != nil {
			return fmt.Errorf("configuration error: %w", err)
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

		if err := o.logger.Init(resolved.LogLevel, resolved.LogFormat, resolved.LogTimestamp); err != nil {
			return fmt.Errorf("failed to initialize logger: %w", err)
		}
		o.logger.SetOutput(cmd.ErrOrStderr())

		containerConfig, err := o.buildContainerConfig(resolved, passthroughArgs, toolsCfg)
		if err != nil {
			return fmt.Errorf("container configuration error: %w", err)
		}

		if resolved.DryRun {
			return o.handleDryRun(cmd, containerConfig, resolved)
		}

		exitCode, err := o.execute(cmd, resolved, containerConfig)
		if err != nil {
			return err
		}
		o.exitFunc(exitCode)
		return nil
	}

	registerFlags(cmd, o)
	cmd.Flags().SetInterspersed(false)
	return cmd
}

func Execute(rawArgs []string) error {
	return ExecuteContext(context.Background(), rawArgs)
}

func ExecuteContext(ctx context.Context, rawArgs []string) error {
	return ExecuteContextWithOptions(ctx, rawArgs, nil)
}

func ExecuteContextWithOptions(ctx context.Context, rawArgs []string, setup func(o *rootOptions, cmd *cobra.Command)) error {
	var cmd *cobra.Command

	if setup == nil {
		cmd = rootCmd
	} else {
		localOpts := defaultOptions()
		localOpts.logger = logging.NewLogger()
		cmd = newRootCmd(&localOpts)
		setup(&localOpts, cmd)
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
			if strings.HasPrefix(arg, "--") {
				name := strings.SplitN(arg[2:], "=", 2)[0]
				if f := cmd.Flags().Lookup(name); f != nil && f.NoOptDefVal == "" && !strings.Contains(arg, "=") {
					i++
				}
			} else if len(arg) > 1 {
				lastChar := string(arg[len(arg)-1])
				if f := cmd.Flags().ShorthandLookup(lastChar); f != nil && f.NoOptDefVal == "" {
					i++
				}
			}
		}
	}

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

	startIdx := 1
	if !isPolyglot && subcmdIdx != -1 {
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

	processedArgs = append(processedArgs, overrides...)
	if isPolyglot {
		processedArgs = append(processedArgs, execName)
	}
	processedArgs = append(processedArgs, others...)

	return processedArgs, nil
}

func init() {
	opts = defaultOptions()
	rootCmd = newRootCmd(&opts)
}
