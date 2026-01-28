package command

import (
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
	tty         bool
	interactive bool
	network     string
	mountSocket string
	mountCderun bool

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

		// Build ContainerConfig
		config := &container.ContainerConfig{
			Image:       "alpine:latest", // Temporary image until Phase 2
			Command:     []string{subcommand},
			Args:        passthroughArgs,
			TTY:         tty,
			Interactive: interactive,
			Network:     network,
			Remove:      true,
		}

		// Initialize Runtime
		// TODO: Implement runtime detection in Phase 2
		socket := "/var/run/docker.sock"
		if mountSocket != "" {
			socket = mountSocket
		}

		rt, err := runtimeFactory(socket)
		if err != nil {
			return fmt.Errorf("failed to initialize runtime: %w", err)
		}

		// Execute Container
		ctx := context.Background()

		containerID, err := rt.CreateContainer(ctx, config)
		if err != nil {
			return fmt.Errorf("failed to create container: %w", err)
		}

		if config.Remove {
			defer rt.RemoveContainer(ctx, containerID)
		}

		if err := rt.StartContainer(ctx, containerID); err != nil {
			return fmt.Errorf("failed to start container: %w", err)
		}

		// Attach to container IO
		var stdin io.Reader
		if config.Interactive {
			stdin = os.Stdin
		}
		if err := rt.AttachContainer(ctx, containerID, config.TTY, stdin, os.Stdout, os.Stderr); err != nil {
			return fmt.Errorf("failed to attach to container: %w", err)
		}

		exitCode, err := rt.WaitContainer(ctx, containerID)
		if err != nil {
			return fmt.Errorf("failed to wait for container: %w", err)
		}

		if config.Remove {
			if err := rt.RemoveContainer(ctx, containerID); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to remove container: %v\n", err)
			}
		}

		exitFunc(exitCode)
		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute(rawArgs []string) error {
	args := preprocessArgs(rawArgs)
	rootCmd.SetArgs(args[1:])
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

	rootCmd.Flags().SetInterspersed(false)
}
