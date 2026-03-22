package command

import (
	"context"
	"errors"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/config"
	"cderun/internal/logging"
	"cderun/internal/runtime"
)

func TestUnit_Root_BuildContainerConfig_Nested_ResolvePathFailure(t *testing.T) {
	t.Parallel()
	mfs := &config.MockFileSystem{
		ExecPath: "/app/{{file:nonexistent}}",
	}
	o := &rootOptions{
		fs:     mfs,
		logger: logging.NewLogger(),
	}
	resolved := &config.ResolvedConfig{
		MountCderun: true,
		HostContext: &config.HostContext{
			Level: 1,
		},
	}
	cfg, err := o.buildContainerConfig(resolved, nil, nil)
	require.NoError(t, err)

	found := false
	for _, m := range cfg.Mounts {
		if m.Target == "/usr/local/bin/cderun" {
			assert.Equal(t, "/app/{{file:nonexistent}}", m.Source)
			found = true
		}
	}
	assert.True(t, found)
}

func TestUnit_Root_BuildContainerConfig_Nested_Level0_NoResolution(t *testing.T) {
	t.Parallel()
	mfs := &config.MockFileSystem{
		ExecPath: "/app/{{HOME}}",
	}
	o := &rootOptions{
		fs:     mfs,
		logger: logging.NewLogger(),
	}
	resolved := &config.ResolvedConfig{
		MountCderun: true,
		HostContext: &config.HostContext{
			Level: 0,
		},
	}
	cfg, err := o.buildContainerConfig(resolved, nil, nil)
	require.NoError(t, err)

	found := false
	for _, m := range cfg.Mounts {
		if m.Target == "/usr/local/bin/cderun" {
			assert.Equal(t, "/app/{{HOME}}", m.Source)
			found = true
		}
	}
	assert.True(t, found)
}

func TestUnit_Root_Execute_WaitContainer_Interrupted(t *testing.T) {
	t.Parallel()
	mockRuntime := &runtime.MockRuntime{
		WaitErr: errors.New("wait interrupted"),
	}
	err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
		o.runtimeFactory = func(n, s string) (runtime.ContainerRuntime, error) { return mockRuntime, nil }
		o.isTerminal = func(fd int) bool { return false }
		o.exitFunc = func(code int) {}
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "failed to wait for container: wait interrupted")
}

func TestUnit_Root_Execute_AttachAfterExit_Error(t *testing.T) {
	t.Parallel()
	mockRuntime := &runtime.MockRuntime{
		AttachErr: errors.New("attach failed after exit"),
		ExitCode:  42,
	}
	err := ExecuteContextWithOptions(context.Background(), []string{"cderun", "--image", "alpine", "sh"}, func(o *rootOptions, cmd *cobra.Command) {
		o.runtimeFactory = func(n, s string) (runtime.ContainerRuntime, error) { return mockRuntime, nil }
		o.isTerminal = func(fd int) bool { return false }
		o.exitFunc = func(code int) {}
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "failed to attach to container: attach failed after exit")
}

func TestUnit_Root_PreprocessArgs_NoSubcommandHoisting(t *testing.T) {
	t.Parallel()
	cmd := newRootCmd(&rootOptions{})
	// Standard mode, no subcommand found, but has P1 flag after some other flag.
	// preprocessArgs should still work and hoist if it's after where it *thinks* a subcommand might be,
	// but actually subcmdIdx will be -1 if no non-flag arg is found.
	// If subcmdIdx == -1, startIdx is 1.
	args := []string{"cderun", "--config", "c.yaml", "--cderun-tty"}
	expected := []string{"cderun", "--cderun-tty", "--config", "c.yaml"}
	actual, err := preprocessArgs(cmd, args)
	require.NoError(t, err)
	assert.Equal(t, expected, actual)
}

func TestUnit_Root_PreprocessArgs_UnknownP1Flag(t *testing.T) {
	t.Parallel()
	cmd := newRootCmd(&rootOptions{})
	// Hoisted flag that is not in the flag set (coverage for f == nil)
	args := []string{"cderun", "sh", "--cderun-unknown", "value"}
	expected := []string{"cderun", "--cderun-unknown", "sh", "value"}
	actual, err := preprocessArgs(cmd, args)
	require.NoError(t, err)
	assert.Equal(t, expected, actual)
}
