package command

import (
	"cderun/internal/config"
	"cderun/internal/container"
	"cderun/internal/runtime"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	tty               bool
	interactive       bool
	network           string
	mountSocket       string
	mountCderun       bool
	image             string
	remove            bool
	cderunTTY         bool
	cderunInteractive bool
	runtimeName       string
	dryRun      bool
	dryRunFormat string

	// For testing
	exitFunc       = os.Exit
	runtimeFactory = func(socket string) (runtime.ContainerRuntime, error) {
		return runtime.NewDockerRuntime(socket)
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
			Remove:               remove,
			RemoveSet:            cmd.Flags().Changed("remove"),
			CderunTTY:            cderunTTY,
			CderunTTYSet:         cmd.Flags().Changed("cderun-tty"),
			CderunInteractive:    cderunInteractive,
			CderunInteractiveSet: cmd.Flags().Changed("cderun-interactive"),
			Runtime:              runtimeName,
			RuntimeSet:           cmd.Flags().Changed("runtime"),
			MountSocket:          mountSocket,
			MountSocketSet:       cmd.Flags().Changed("mount-socket"),
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

		if dryRun {
			var output string
			var err error
			switch dryRunFormat {
			case "json":
				output, err = config.ToJSON()
			case "simple":
				output = config.ToSimple()
			case "yaml", "":
				output, err = config.ToYAML()
			default:
				return fmt.Errorf("unsupported format: %s", dryRunFormat)
			}
			if err != nil {
				return fmt.Errorf("failed to format config: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), output)
			return nil
		}

		// Initialize Runtime
		var rt runtime.ContainerRuntime
		switch resolved.Runtime {
		case "docker", "podman":
			rt, err = runtimeFactory(resolved.Socket)
		default:
			return fmt.Errorf("unsupported runtime %q: only %q and %q are supported", resolved.Runtime, "docker", "podman")
		}
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
	args := preprocessArgs(rawArgs)
	if len(args) >= 1 {
		rootCmd.SetArgs(args[1:])
	} else {
		rootCmd.SetArgs([]string{})
	}
	return rootCmd.Execute()
}

func preprocessArgs(args []string) []string {
	if len(args) == 0 {
		return args
	}

	execName := filepath.Base(args[0])
	if execName == "cderun" {
		return args
	}

	// If the executable is not "cderun", treat the executable name as a subcommand.
	// For example, if "node --version" is called via a symlink:
	// args = ["node", "--version"] -> ["cderun", "node", "--version"]
	newArgs := make([]string, 0, len(args)+1)
	newArgs = append(newArgs, "cderun", execName)
	newArgs = append(newArgs, args[1:]...)
	return newArgs
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&tty, "tty", false, "Allocate a pseudo-TTY")
	rootCmd.PersistentFlags().BoolVarP(&interactive, "interactive", "i", false, "Keep STDIN open even if not attached")
	rootCmd.PersistentFlags().StringVar(&network, "network", "bridge", "Connect a container to a network")
	rootCmd.PersistentFlags().StringVar(&mountSocket, "mount-socket", "", "Mount container runtime socket (e.g., /var/run/docker.sock)")
	rootCmd.PersistentFlags().BoolVar(&mountCderun, "mount-cderun", false, "Mount cderun binary for use inside container")
	rootCmd.PersistentFlags().StringVar(&image, "image", "", "Docker image to use")
	rootCmd.PersistentFlags().StringVar(&runtimeName, "runtime", "docker", "Container runtime to use (docker/podman)")
	rootCmd.PersistentFlags().BoolVar(&remove, "remove", true, "Automatically remove the container when it exits")
	rootCmd.PersistentFlags().BoolVar(&cderunTTY, "cderun-tty", false, "Forced TTY override")
	rootCmd.PersistentFlags().BoolVar(&cderunInteractive, "cderun-interactive", false, "Forced interactive override")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Do not run container, just show the container configuration")
	rootCmd.PersistentFlags().StringVar(&dryRunFormat, "dry-run-format", "yaml", "Output format for dry-run (yaml, json, simple)")

	rootCmd.Flags().SetInterspersed(false)
}
