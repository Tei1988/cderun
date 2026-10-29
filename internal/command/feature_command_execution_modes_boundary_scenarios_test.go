package command

import (
	"context"
	"errors"
	"testing"
	"time"

	"cderun/internal/config"
	"cderun/internal/container"
	"cderun/internal/logging"
	"cderun/internal/runtime"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveSnapshotBaseDirBoundary(t *testing.T) {
	t.Parallel()

	t.Run("Base host with absolute temp dir on MockFileSystem returns mock temp dir", func(t *testing.T) {
		t.Parallel()
		mfs := &config.MockFileSystem{TempDirValue: "/custom/tmp"}
		globalCfg := &config.CDERunConfig{}

		res := resolveSnapshotBaseDir(mfs, globalCfg)
		assert.Equal(t, "/custom/tmp", res)
	})

	t.Run("Nested execution host returns fallback temp dir /tmp", func(t *testing.T) {
		t.Parallel()
		rfs := config.RealFileSystem{}
		globalCfg := &config.CDERunConfig{
			HostContext: &config.HostContext{Level: 1},
		}

		res := resolveSnapshotBaseDir(rfs, globalCfg)
		assert.Equal(t, fallbackSnapshotBaseDir, res)
	})
}

type errMockFS struct {
	*config.MockFileSystem
	getwdErr error
	homeErr  error
	execErr  error
}

func (e *errMockFS) Getwd() (string, error) {
	if e.getwdErr != nil {
		return "", e.getwdErr
	}
	return e.MockFileSystem.Getwd()
}

func (e *errMockFS) UserHomeDir() (string, error) {
	if e.homeErr != nil {
		return "", e.homeErr
	}
	return e.MockFileSystem.UserHomeDir()
}

func (e *errMockFS) Executable() (string, error) {
	if e.execErr != nil {
		return "", e.execErr
	}
	return e.MockFileSystem.Executable()
}

func TestPopulateHostContextPathsFallbacks(t *testing.T) {
	t.Parallel()

	logger := logging.NewLogger()
	negUid := -1
	negGid := -1
	posUid := 1001
	posGid := 1002

	t.Run("Populates defaults when filesystem functions return errors or negative values", func(t *testing.T) {
		t.Parallel()
		mfs := &errMockFS{
			MockFileSystem: &config.MockFileSystem{
				UIDValue: &negUid,
				GIDValue: &negGid,
			},
			getwdErr: errors.New("getwd failed"),
			homeErr:  errors.New("home failed"),
			execErr:  errors.New("exec failed"),
		}

		hostCtx := &config.HostContext{}
		populateHostContextPaths(mfs, logger, hostCtx)

		assert.Empty(t, hostCtx.BinPath)
		assert.Empty(t, hostCtx.WorkingDir)
		assert.Empty(t, hostCtx.HomeDir)
		assert.Equal(t, "0", hostCtx.UID)
		assert.Equal(t, "0", hostCtx.GID)
	})

	t.Run("Preserves existing non-empty HostContext fields and populated values", func(t *testing.T) {
		t.Parallel()
		mfs := &config.MockFileSystem{
			WD:       "/work",
			HomeDir:  "/home/user",
			ExecPath: "/usr/bin/cderun",
			UIDValue: &posUid,
			GIDValue: &posGid,
		}

		hostCtx := &config.HostContext{
			UID: "999",
			GID: "888",
		}
		populateHostContextPaths(mfs, logger, hostCtx)

		assert.Equal(t, "/usr/bin/cderun", hostCtx.BinPath)
		assert.Equal(t, "/work", hostCtx.WorkingDir)
		assert.Equal(t, "/home/user", hostCtx.HomeDir)
		assert.Equal(t, "999", hostCtx.UID)
		assert.Equal(t, "888", hostCtx.GID)
	})
}

func TestGetHangTimeoutBoundary(t *testing.T) {
	t.Parallel()

	o := &rootOptions{}

	t.Run("Returns 0 when host stdin is TTY and interactive mode is enabled", func(t *testing.T) {
		t.Parallel()
		resolved := &config.ResolvedConfig{HangTimeout: 30 * time.Second}
		res := o.getHangTimeout(true, true, resolved)
		assert.Equal(t, time.Duration(0), res)
	})

	t.Run("Returns resolved hang timeout when not host TTY interactive", func(t *testing.T) {
		t.Parallel()
		resolved := &config.ResolvedConfig{HangTimeout: 15 * time.Second}
		res := o.getHangTimeout(false, true, resolved)
		assert.Equal(t, 15*time.Second, res)
	})

	t.Run("Returns default hangTimeout fallback when resolved is nil", func(t *testing.T) {
		t.Parallel()
		res := o.getHangTimeout(false, false, nil)
		assert.Equal(t, hangTimeout, res)
	})
}

func TestSignalKillIfRunningBoundary(t *testing.T) {
	t.Parallel()

	logger := logging.NewLogger()
	o := &rootOptions{logger: logger}

	t.Run("Skips SIGKILL when container is already stopped", func(t *testing.T) {
		t.Parallel()
		mockRT := &runtime.MockRuntime{
			InspectFunc: func(ctx context.Context, containerID string) (bool, int, error) {
				return false, 42, nil
			},
		}

		code, err := o.signalKillIfRunning(t.Context(), mockRT, "test-cont-1")
		require.NoError(t, err)
		assert.Equal(t, 42, code)

		var signaled string
		mockRT.WithLockedMock(func(m *runtime.MockRuntime) {
			signaled = m.SignaledContainerID
		})
		assert.Empty(t, signaled)
	})

	t.Run("Sends SIGKILL when container is still running", func(t *testing.T) {
		t.Parallel()
		mockRT := &runtime.MockRuntime{
			InspectFunc: func(ctx context.Context, containerID string) (bool, int, error) {
				return true, 0, nil
			},
		}

		code, err := o.signalKillIfRunning(t.Context(), mockRT, "test-cont-2")
		require.NoError(t, err)
		assert.Equal(t, 0, code)

		var signaled, sig string
		mockRT.WithLockedMock(func(m *runtime.MockRuntime) {
			signaled = m.SignaledContainerID
			sig = m.Signal
		})
		assert.Equal(t, "test-cont-2", signaled)
		assert.Equal(t, "SIGKILL", sig)
	})

	t.Run("Falls back to SIGKILL when inspect returns error", func(t *testing.T) {
		t.Parallel()
		mockRT := &runtime.MockRuntime{
			InspectFunc: func(ctx context.Context, containerID string) (bool, int, error) {
				return false, 0, errors.New("inspect failed")
			},
		}

		code, err := o.signalKillIfRunning(t.Context(), mockRT, "test-cont-3")
		require.NoError(t, err)
		assert.Equal(t, 0, code)

		var signaled string
		mockRT.WithLockedMock(func(m *runtime.MockRuntime) {
			signaled = m.SignaledContainerID
		})
		assert.Equal(t, "test-cont-3", signaled)
	})
}

func TestPreprocessArgsHoistingBoundary(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{Use: "cderun"}
	o := &rootOptions{}
	registerFlags(cmd, o)

	t.Run("Returns empty slice unchanged", func(t *testing.T) {
		t.Parallel()
		args, err := preprocessArgs(cmd, nil)
		require.NoError(t, err)
		assert.Nil(t, args)
	})

	t.Run("Hoists overrides in polyglot mode", func(t *testing.T) {
		t.Parallel()
		input := []string{"kubectl", "get", "pods", "--cderun-image", "custom-kubectl:latest"}
		processed, err := preprocessArgs(cmd, input)
		require.NoError(t, err)
		assert.Equal(t, []string{"cderun", "--cderun-image", "custom-kubectl:latest", "kubectl", "get", "pods"}, processed)
	})

	t.Run("Hoists space-separated override flags after subcommand", func(t *testing.T) {
		t.Parallel()
		input := []string{"cderun", "run", "--cderun-image", "alpine:3.18", "echo", "hello"}
		processed, err := preprocessArgs(cmd, input)
		require.NoError(t, err)
		assert.Equal(t, []string{"cderun", "--cderun-image", "alpine:3.18", "run", "echo", "hello"}, processed)
	})
}

func TestContainerConfigAssemblyInvariantsBoundary(t *testing.T) {
	t.Parallel()

	t.Run("Rejects null-byte control characters in passthrough command argument", func(t *testing.T) {
		t.Parallel()
		o := &rootOptions{}
		resolved := &config.ResolvedConfig{
			Image: "alpine:latest",
		}
		_, err := o.buildContainerConfig(resolved, []string{"echo", "hi\x00there"}, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "null byte")
	})

	t.Run("Clones Mounts slice to prevent caller state mutation", func(t *testing.T) {
		t.Parallel()
		o := &rootOptions{}
		mountsInput := []container.Mount{
			{Source: "/host/src", Target: "/container/target", Type: "bind"},
		}
		resolved := &config.ResolvedConfig{
			Image:  "alpine:latest",
			Mounts: mountsInput,
		}
		cc, err := o.buildContainerConfig(resolved, []string{"echo", "hi"}, nil)
		require.NoError(t, err)
		require.NotNil(t, cc)

		mountsInput[0].Source = "/host/mutated"
		assert.Equal(t, "/host/src", cc.Mounts[0].Source)
	})
}
