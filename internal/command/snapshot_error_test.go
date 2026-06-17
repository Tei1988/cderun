package command

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/config"
	"cderun/internal/logging"
	"cderun/internal/runtime"
)

func TestUnit_Snapshot_ErrorHandling(t *testing.T) {
	t.Run("Hard error when explicit nested execution requested", func(t *testing.T) {
		mfs := &RootErrorFS{
			MockFileSystem: &config.MockFileSystem{
				WD:       "/work",
				HomeDir:  "/home/user",
				ExecPath: "/usr/local/bin/cderun",
			},
			MkdirErr: errors.New("mkdir failed"),
		}

		opts := defaultOptions()
		opts.fs = mfs
		opts.logger = logging.NewLogger()
		// Disable logging to avoid noise
		opts.logger.Init("error", "text", false)

		// Set up a mock runtime factory
		opts.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
			return &runtime.MockRuntime{}, nil
		}

		cmd := newRootCmd(&opts)
		// Request explicit nested execution via --mount-cderun
		// Use --image to pass validation
		cmd.SetArgs([]string{"--image", "alpine", "--mount-cderun", "node", "--version"})

		err := cmd.ExecuteContext(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create snapshot for nested execution")
		assert.Contains(t, err.Error(), "mkdir failed")
	})

	t.Run("Warning only when implicit propagation active", func(t *testing.T) {
		mfs := &RootErrorFS{
			MockFileSystem: &config.MockFileSystem{
				WD:       "/work",
				HomeDir:  "/home/user",
				ExecPath: "/usr/local/bin/cderun",
				Dirs: map[string]bool{
					"/work": true,
				},
				Files: map[string][]byte{
					"/work/.cderun.yaml": []byte("hostContext:\n  level: 1\n  snapshotDir: /tmp/snap\n"),
				},
			},
			MkdirErr: errors.New("mkdir failed"),
		}

		opts := defaultOptions()
		opts.fs = mfs
		opts.logger = logging.NewLogger()
		// Capture logs to verify warning.
		var logBuf snapshotErrorTestStringWriter
		opts.logger.Init("warn", "text", false)
		// We DO NOT call o.logger.SetOutput(cmd.ErrOrStderr()) in root.go BEFORE the snapshot logic,
		// but wait, RunE DOES call it at line 1118.
		// However, it calls cmd.ErrOrStderr().
		// So we should set the output of the COMMAND'S stderr.

		// Mock runtime that returns a successful exit code
		opts.runtimeFactory = func(name, socket string) (runtime.ContainerRuntime, error) {
			return &runtime.MockRuntime{
				ExitCode: 0,
			}, nil
		}

		opts.configLoader = config.NewConfigLoaderWithFS(mfs)

		cmd := newRootCmd(&opts)
		cmd.SetErr(&logBuf) // Redirect command's stderr to our buffer

		// Normal command, no explicit nested flags.
		cmd.SetArgs([]string{"--image", "alpine", "node", "--version"})

		err := cmd.ExecuteContext(context.Background())
		// Should NOT return an error because it's only a warning for implicit propagation
		assert.NoError(t, err)

		assert.Contains(t, logBuf.String(), "failed to create snapshot: failed to create snapshot directory: mkdir failed")
	})
}

type snapshotErrorTestStringWriter struct {
	buf []byte
}

func (s *snapshotErrorTestStringWriter) Write(p []byte) (n int, err error) {
	s.buf = append(s.buf, p...)
	return len(p), nil
}

func (s *snapshotErrorTestStringWriter) String() string {
	return string(s.buf)
}
