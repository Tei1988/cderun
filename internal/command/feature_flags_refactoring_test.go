package command

import (
	"testing"

	"cderun/internal/config"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestUnit_Flags_RegisterFlagsInvariance(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	opts := &rootOptions{}

	require.NotPanics(t, func() {
		registerFlags(cmd, opts)
	})

	flags := cmd.PersistentFlags()

	// Verify bool flags registration
	for _, opt := range config.BoolOptions {
		f := flags.Lookup(opt.Name)
		require.NotNil(t, f, "flag %s should be registered", opt.Name)
		overrideF := flags.Lookup("cderun-" + opt.Name)
		require.NotNil(t, overrideF, "override flag cderun-%s should be registered", opt.Name)
	}

	// Verify string flags registration
	for _, opt := range config.StringOptions {
		f := flags.Lookup(opt.Name)
		require.NotNil(t, f, "flag %s should be registered", opt.Name)
		overrideF := flags.Lookup("cderun-" + opt.Name)
		require.NotNil(t, overrideF, "override flag cderun-%s should be registered", opt.Name)
	}

	// Verify int flags registration
	for _, opt := range config.IntOptions {
		f := flags.Lookup(opt.Name)
		require.NotNil(t, f, "flag %s should be registered", opt.Name)
		overrideF := flags.Lookup("cderun-" + opt.Name)
		require.NotNil(t, overrideF, "override flag cderun-%s should be registered", opt.Name)
	}

	// Verify float64 flags registration
	for _, opt := range config.Float64Options {
		f := flags.Lookup(opt.Name)
		require.NotNil(t, f, "flag %s should be registered", opt.Name)
		overrideF := flags.Lookup("cderun-" + opt.Name)
		require.NotNil(t, overrideF, "override flag cderun-%s should be registered", opt.Name)
	}

	// Verify string slice flags registration
	for _, opt := range config.StringSliceOptions {
		f := flags.Lookup(opt.Name)
		require.NotNil(t, f, "flag %s should be registered", opt.Name)
		overrideF := flags.Lookup("cderun-" + opt.Name)
		require.NotNil(t, overrideF, "override flag cderun-%s should be registered", opt.Name)
	}
}
