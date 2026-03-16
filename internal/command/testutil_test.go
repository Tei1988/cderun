package command

import (
	"github.com/spf13/cobra"

	"cderun/internal/config"
	"cderun/internal/runtime"
)

func withMockFS(mfs *config.MockFileSystem) func(o *rootOptions, cmd *cobra.Command) {
	return func(o *rootOptions, cmd *cobra.Command) {
		o.fs = mfs
		o.configLoader = config.NewConfigLoaderWithFS(mfs)
	}
}

func withMockRuntime(mock runtime.ContainerRuntime, extras ...func(o *rootOptions, cmd *cobra.Command)) func(o *rootOptions, cmd *cobra.Command) {
	return func(o *rootOptions, cmd *cobra.Command) {
		o.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
			return mock, nil
		}
		// Default exitFunc to a no-op to prevent tests from terminating the process.
		// Callers can still override this behavior by providing an extra setup function.
		o.exitFunc = func(code int) {}

		for _, extra := range extras {
			extra(o, cmd)
		}
	}
}
