package command

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/term"

	"cderun/internal/config"
	"cderun/internal/container"
	"cderun/internal/logging"
	"cderun/internal/runtime"
)

func TestUnit_Root_SetupTerminal_Coverage(t *testing.T) {
	t.Parallel()

	t.Run("no-op when not a terminal", func(t *testing.T) {
		o := &rootOptions{logger: logging.NewLogger()}
		cleanup := o.setupTerminal(0, false, &container.ContainerConfig{TTY: true})
		assert.NotNil(t, cleanup)
		cleanup() // Should be no-op
	})

	t.Run("no-op when TTY not requested", func(t *testing.T) {
		o := &rootOptions{logger: logging.NewLogger()}
		cleanup := o.setupTerminal(0, true, &container.ContainerConfig{TTY: false})
		assert.NotNil(t, cleanup)
		cleanup() // Should be no-op
	})

	t.Run("restore called when makeRaw succeeds", func(t *testing.T) {
		restoreCalled := false
		fakeState := &term.State{}
		o := &rootOptions{
			logger: logging.NewLogger(),
			makeRaw: func(fd int) (*term.State, error) {
				return fakeState, nil
			},
			restore: func(fd int, state *term.State) error {
				if state == fakeState {
					restoreCalled = true
				}
				return nil
			},
		}

		cleanup := o.setupTerminal(0, true, &container.ContainerConfig{TTY: true})
		assert.NotNil(t, cleanup)
		cleanup()
		assert.True(t, restoreCalled)
	})
}

func TestUnit_Root_AttachContainer_Coverage(t *testing.T) {
	t.Parallel()

	t.Run("context canceled before attachment ready", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		o := &rootOptions{logger: logging.NewLogger()}
		mockRuntime := &runtime.MockRuntime{
			AttachFunc: func(ctx context.Context, id string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error {
				// Don't signal ready and wait for cancellation
				<-ctx.Done()
				return ctx.Err()
			},
		}
		cmd := &cobra.Command{}

		cancel()
		att, err := o.attachContainer(ctx, cmd, mockRuntime, "test-container", &container.ContainerConfig{Interactive: true})
		require.ErrorIs(t, err, context.Canceled)
		assert.Nil(t, att)
	})
}

func TestUnit_Root_WaitForCompletion_Coverage(t *testing.T) {
	t.Parallel()

	t.Run("attachDone error before container exit, timeout expires", func(t *testing.T) {
		o := &rootOptions{logger: logging.NewLogger()}
		mockRuntime := &terminationMockRuntime{
			MockRuntime: &runtime.MockRuntime{
				WaitFunc: func(ctx context.Context, id string) (int, error) {
					<-ctx.Done()
					return 137, ctx.Err()
				},
			},
			isRunning: true,
		}
		cmd := &cobra.Command{}
		att := &attachResult{
			attachDone:   make(chan error, 1),
			cancelAttach: func() {},
		}
		att.attachDone <- errors.New("early attach error")

		exitCode, err := o.waitForCompletion(context.Background(), cmd, mockRuntime, "c1", &container.ContainerConfig{}, &config.ResolvedConfig{HangTimeout: 10 * time.Millisecond}, false, att)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "early attach error")
		assert.Equal(t, 0, exitCode)
	})
}

func TestUnit_Root_WriteFormatted_Coverage(t *testing.T) {
	t.Parallel()

	o := &rootOptions{logger: logging.NewLogger()}
	err := o.writeFormatted(io.Discard, "invalid", nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported output format: \"invalid\"")
}

func TestUnit_Root_PreprocessArgs_Coverage(t *testing.T) {
	t.Parallel()

	cmd := newRootCmd(&rootOptions{})

	t.Run("shorthand cluster with argument", func(t *testing.T) {
		// -p requires an argument. In -pf, 'f' becomes the argument for -p.
		args := []string{"cderun", "-pf", "node", "app.js"}
		expected := []string{"cderun", "-pf", "node", "app.js"}
		actual, err := preprocessArgs(cmd, args)
		require.NoError(t, err)
		assert.Equal(t, expected, actual)
	})

	t.Run("long flag with equals", func(t *testing.T) {
		args := []string{"cderun", "--config=c.yaml", "node", "app.js"}
		expected := []string{"cderun", "--config=c.yaml", "node", "app.js"}
		actual, err := preprocessArgs(cmd, args)
		require.NoError(t, err)
		assert.Equal(t, expected, actual)
	})
}

func TestUnit_Root_CreateSnapshot_Coverage(t *testing.T) {
	t.Parallel()

	logger := logging.NewLogger()

	t.Run("MkdirAll failure", func(t *testing.T) {
		mfs := &rootErrorFS{
			MockFileSystem: &config.MockFileSystem{},
			mkdirErr:       errors.New("mkdir failed"),
		}
		_, _, err := createSnapshot(logger, mfs, &config.CDERunConfig{}, nil, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create snapshot directory: mkdir failed")
	})

	t.Run("WriteFile .cderun.yaml failure", func(t *testing.T) {
		mfs := &rootErrorFS{
			MockFileSystem: &config.MockFileSystem{},
			wfFunc: func(path string, data []byte, perm os.FileMode) error {
				if filepath.Base(path) == ".cderun.yaml" {
					return errors.New("write cderun failed")
				}
				return nil
			},
		}
		_, _, err := createSnapshot(logger, mfs, &config.CDERunConfig{}, nil, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to write .cderun.yaml to snapshot: write cderun failed")
	})

	t.Run("WriteFile .tools.yaml failure", func(t *testing.T) {
		mfs := &rootErrorFS{
			MockFileSystem: &config.MockFileSystem{},
			wfFunc: func(path string, data []byte, perm os.FileMode) error {
				if filepath.Base(path) == ".tools.yaml" {
					return errors.New("write tools failed")
				}
				return nil
			},
		}
		_, _, err := createSnapshot(logger, mfs, &config.CDERunConfig{}, nil, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to write .tools.yaml to snapshot: write tools failed")
	})

	t.Run("ResolvePath failure at Level > 1", func(t *testing.T) {
		mfs := &config.MockFileSystem{
			Files: map[string][]byte{},
		}
		globalCfg := &config.CDERunConfig{
			HostContext: &config.HostContext{
				Level: 1, // Will become 2
			},
		}
		// ResolvePath will fail because we have no mount mappings for temp dir
		// and we use an invalid expression.
		mfs.TempDirValue = "/tmp/{{file:..}}"

		_, _, err := createSnapshot(logger, mfs, globalCfg, nil, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to resolve snapshot directory to host path")
	})
}
