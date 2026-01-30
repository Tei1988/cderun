package command

import (
	"cderun/internal/config"
	"cderun/internal/container"
	"cderun/internal/runtime"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	tty               bool
	interactive       bool
	network           string
	mountSocket       string
	mountCderun       bool
	image             string
	remove            bool
	cderunTTY           bool
	cderunInteractive    bool
	cderunImage         string
	cderunNetwork       string
	cderunRemove        bool
	cderunRuntime       string
	cderunMountSocket    string
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
	dryRun            bool
	dryRunFormat      string

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
		globalCfg, _, err := config.LoadCDERunConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to load cderun config: %v\n", err)
		}
		toolsCfg, _, err := config.LoadToolsConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to load tools config: %v\n", err)
		}

		// Resolve settings using priority logic
		cliOpts := config.CLIOptions{
			Image:                image,
			ImageSet:             cmd.Flags().Changed("image"),
			TTY:                  tty,
			TTYSet:               cmd.Flags().Changed("tty"),
			Interactive:          interactive,
			InteractiveSet:       cmd.Flags().Changed("interactive"),
			Network:              network,
			NetworkSet:           cmd.Flags().Changed("network"),
			CderunNetwork:        cderunNetwork,
			CderunNetworkSet:     cmd.Flags().Changed("cderun-network"),
			Remove:               remove,
			RemoveSet:            cmd.Flags().Changed("remove"),
			CderunRemove:         cderunRemove,
			CderunRemoveSet:      cmd.Flags().Changed("cderun-remove"),
			CderunTTY:            cderunTTY,
			CderunTTYSet:         cmd.Flags().Changed("cderun-tty"),
			CderunInteractive:    cderunInteractive,
			CderunInteractiveSet: cmd.Flags().Changed("cderun-interactive"),
			CderunImage:          cderunImage,
			CderunImageSet:       cmd.Flags().Changed("cderun-image"),
			Runtime:              runtimeName,
			RuntimeSet:           cmd.Flags().Changed("runtime"),
			CderunRuntime:        cderunRuntime,
			CderunRuntimeSet:     cmd.Flags().Changed("cderun-runtime"),
			MountSocket:          mountSocket,
			MountSocketSet:       cmd.Flags().Changed("mount-socket"),
			CderunMountSocket:    cderunMountSocket,
			CderunMountSocketSet: cmd.Flags().Changed("cderun-mount-socket"),
			Env:                  env,
			CderunEnv:            cderunEnv,
			Workdir:              workdir,
			WorkdirSet:           cmd.Flags().Changed("workdir"),
			CderunWorkdir:        cderunWorkdir,
			CderunWorkdirSet:     cmd.Flags().Changed("cderun-workdir"),
			Volumes:              volumes,
			CderunVolumes:        cderunVolumes,
			MountCderun:          mountCderun,
			MountCderunSet:       cmd.Flags().Changed("mount-cderun"),
			CderunMountCderun:    cderunMountCderun,
			CderunMountCderunSet: cmd.Flags().Changed("cderun-mount-cderun"),
			MountTools:           mountTools,
			CderunMountTools:     cderunMountTools,
			MountAllTools:        mountAllTools,
			CderunMountAllTools:  cderunMountAllTools,
		}

		resolved, err := config.Resolve(subcommand, cliOpts, toolsCfg, globalCfg)
		if err != nil {
			return fmt.Errorf("configuration error: %w", err)
		}

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
				return fmt.Errorf("--mount-cderun, --mount-tools, or --mount-all-tools requires --mount-socket")
			}
			exePath, err := os.Executable()
			if err != nil {
				return fmt.Errorf("failed to get executable path: %w", err)
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
					fmt.Fprintf(os.Stderr, "Warning: --mount-all-tools specified but no tools defined in .tools.yaml\n")
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
						return fmt.Errorf("tool %q not found in tools config", toolName)
					}
					containerConfig.Volumes = append(containerConfig.Volumes, container.VolumeMount{
						HostPath:      exePath,
						ContainerPath: "/usr/local/bin/" + toolName,
						ReadOnly:      true,
					})
				}
			}
		}

		if dryRun {
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

		// Initialize Runtime
		rt, err := runtimeFactory(resolved.Runtime, resolved.Socket)
		if err != nil {
			return fmt.Errorf("failed to initialize runtime: %w", err)
		}

		// Execute Container
		ctx := cmd.Context()

		containerID, err := rt.CreateContainer(ctx, containerConfig)
		if err != nil {
			return fmt.Errorf("failed to create container: %w", err)
		}

		if containerConfig.Remove {
			cleanupCtx := context.WithoutCancel(ctx)
			defer func() {
				if err := rt.RemoveContainer(cleanupCtx, containerID); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to remove container (defer): %v\n", err)
				}
			}()
		}

		if err := rt.StartContainer(ctx, containerID); err != nil {
			return fmt.Errorf("failed to start container: %w", err)
		}

		// Attach to container IO
		var stdin io.Reader
		if containerConfig.Interactive {
			stdin = os.Stdin
		}
		if err := rt.AttachContainer(ctx, containerID, containerConfig.TTY, stdin, os.Stdout, os.Stderr); err != nil {
			return fmt.Errorf("failed to attach to container: %w", err)
		}

		exitCode, err := rt.WaitContainer(ctx, containerID)
		if err != nil {
			return fmt.Errorf("failed to wait for container: %w", err)
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
	rootCmd.PersistentFlags().BoolVar(&tty, "tty", false, "Allocate a pseudo-TTY")
	rootCmd.PersistentFlags().BoolVarP(&interactive, "interactive", "i", false, "Keep STDIN open even if not attached")
	rootCmd.PersistentFlags().StringVar(&network, "network", "bridge", "Connect a container to a network")
	rootCmd.PersistentFlags().StringVar(&mountSocket, "mount-socket", "", "Mount container runtime socket (e.g., /var/run/docker.sock)")
	rootCmd.PersistentFlags().BoolVar(&mountCderun, "mount-cderun", false, "Mount cderun binary for use inside container")
	rootCmd.PersistentFlags().StringVar(&image, "image", "", "Docker image to use")
	rootCmd.PersistentFlags().StringVar(&runtimeName, "runtime", "docker", "Container runtime to use (docker/podman)")
	rootCmd.PersistentFlags().StringSliceVarP(&env, "env", "e", nil, "Set environment variables")
	rootCmd.PersistentFlags().StringVarP(&workdir, "workdir", "w", "", "Working directory inside the container")
	rootCmd.PersistentFlags().StringSliceVarP(&volumes, "volume", "v", nil, "Bind mount a volume")
	rootCmd.PersistentFlags().StringVar(&mountTools, "mount-tools", "", "Mount specified tools into the container")
	rootCmd.PersistentFlags().BoolVar(&mountAllTools, "mount-all-tools", false, "Mount all defined tools into the container")
	rootCmd.PersistentFlags().BoolVar(&remove, "remove", true, "Automatically remove the container when it exits")
	rootCmd.PersistentFlags().BoolVar(&cderunTTY, "cderun-tty", false, "Override TTY setting (highest priority, can be used after subcommand)")
	rootCmd.PersistentFlags().BoolVar(&cderunInteractive, "cderun-interactive", false, "Override interactive setting (highest priority, can be used after subcommand)")
	rootCmd.PersistentFlags().StringVar(&cderunImage, "cderun-image", "", "Override image (highest priority, can be used after subcommand)")
	rootCmd.PersistentFlags().StringVar(&cderunNetwork, "cderun-network", "", "Override network setting (highest priority, can be used after subcommand)")
	rootCmd.PersistentFlags().BoolVar(&cderunRemove, "cderun-remove", true, "Override remove setting (highest priority, can be used after subcommand)")
	rootCmd.PersistentFlags().StringVar(&cderunRuntime, "cderun-runtime", "", "Override runtime setting (highest priority, can be used after subcommand)")
	rootCmd.PersistentFlags().StringVar(&cderunMountSocket, "cderun-mount-socket", "", "Override socket path (highest priority, can be used after subcommand)")
	rootCmd.PersistentFlags().StringSliceVar(&cderunEnv, "cderun-env", nil, "Override environment variables (highest priority, can be used after subcommand)")
	rootCmd.PersistentFlags().StringVar(&cderunWorkdir, "cderun-workdir", "", "Override workdir setting (highest priority, can be used after subcommand)")
	rootCmd.PersistentFlags().StringSliceVar(&cderunVolumes, "cderun-volume", nil, "Override volume mounts (highest priority, can be used after subcommand)")
	rootCmd.PersistentFlags().BoolVar(&cderunMountCderun, "cderun-mount-cderun", false, "Override mount-cderun setting (highest priority, can be used after subcommand)")
	rootCmd.PersistentFlags().StringVar(&cderunMountTools, "cderun-mount-tools", "", "Override mount-tools setting (highest priority, can be used after subcommand)")
	rootCmd.PersistentFlags().BoolVar(&cderunMountAllTools, "cderun-mount-all-tools", false, "Override mount-all-tools setting (highest priority, can be used after subcommand)")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Preview container configuration without execution")
	rootCmd.PersistentFlags().StringVarP(&dryRunFormat, "dry-run-format", "f", "yaml", "Output format (yaml, json, simple)")

	rootCmd.Flags().SetInterspersed(false)
}
