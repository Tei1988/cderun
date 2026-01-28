package cmd

import (
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

		// For now, just print the parsed results
		cmd.Printf("--- cderun Configuration ---\n")
		cmd.Printf("TTY: %v\n", tty)
		cmd.Printf("Interactive: %v\n", interactive)
		cmd.Printf("Network: %s\n", network)
		cmd.Printf("Mount Socket: %s\n", mountSocket)
		cmd.Printf("Mount Cderun: %v\n", mountCderun)
		cmd.Printf("---------------------------\n")
		cmd.Printf("Subcommand: %s\n", subcommand)
		cmd.Printf("Passthrough Args: %v\n", passthroughArgs)
		cmd.Printf("---------------------------\n")

		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	args := preprocessArgs(os.Args)
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
