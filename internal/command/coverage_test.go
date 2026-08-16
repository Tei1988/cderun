package command

import (
	"bytes"
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

func TestUnit_Root_SetupTerminal(t *testing.T) {
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

func TestUnit_Root_AttachContainer(t *testing.T) {
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

func TestUnit_Root_WaitForCompletion(t *testing.T) {
	t.Parallel()

	t.Run("attachDone error before container exit, timeout expires", func(t *testing.T) {
		o := &rootOptions{logger: logging.NewLogger()}
		o.logger.Init("warn", "text", false)
		var stderrBuf bytes.Buffer
		o.logger.SetOutput(&stderrBuf)

		mockRuntime := &TerminationMockRuntime{
			MockRuntime: &runtime.MockRuntime{
				WaitFunc: func(ctx context.Context, id string) (int, error) {
					<-ctx.Done()
					return 137, ctx.Err()
				},
			},
			IsRunning: true,
		}
		cmd := &cobra.Command{}
		att := &attachResult{
			attachDone:   make(chan error, 1),
			cancelAttach: func() {},
		}
		att.attachDone <- errors.New("early attach error")

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		_, err := o.waitForCompletion(ctx, cmd, mockRuntime, "c1", &container.ContainerConfig{}, &config.ResolvedConfig{HangTimeout: 10 * time.Millisecond}, false, att)
		require.Error(t, err)
		var exitErr *ExitCodeError
		require.ErrorAs(t, err, &exitErr)
		assert.Equal(t, 125, exitErr.Code)
		assert.Contains(t, exitErr.Error(), "timeout waiting for container to exit after attach error")
		cancel()
		assert.Contains(t, stderrBuf.String(), "failed to attach to container: early attach error")
	})

	t.Run("AttachContainer fails after container exit within grace period", func(t *testing.T) {
		o := &rootOptions{
			logger:            logging.NewLogger(),
			attachGracePeriod: 1 * time.Second,
		}
		o.logger.Init("warn", "text", false)
		var stderrBuf bytes.Buffer
		o.logger.SetOutput(&stderrBuf)

		waitStarted := make(chan struct{})
		exitNotified := make(chan struct{})
		mockRuntime := &runtime.MockRuntime{
			WaitFunc: func(ctx context.Context, id string) (int, error) {
				close(waitStarted)
				close(exitNotified)
				return 42, nil
			},
		}
		cmd := &cobra.Command{}
		att := &attachResult{
			attachDone:   make(chan error, 1),
			cancelAttach: func() {},
		}

		go func() {
			<-exitNotified
			att.attachDone <- errors.New("attach failure after exit")
		}()

		exitCode, err := o.waitForCompletion(context.Background(), cmd, mockRuntime, "c1", &container.ContainerConfig{}, &config.ResolvedConfig{}, false, att)
		require.NoError(t, err)
		assert.Equal(t, 42, exitCode)
		assert.Contains(t, stderrBuf.String(), "failed to attach to container: attach failure after exit")
	})
}

func TestUnit_Root_LoadConfigs_Coverage(t *testing.T) {
	t.Parallel()

	t.Run("CDERUN_CONFIG environment variable", func(t *testing.T) {
		mfs := &config.MockFileSystem{
			Env: map[string]string{
				"CDERUN_CONFIG": "/env/cderun.yaml",
			},
			Files: map[string][]byte{
				"/env/cderun.yaml": []byte("runtime: podman"),
			},
		}
		o := defaultOptions()
		o.fs = mfs
		o.configLoader = config.NewConfigLoaderWithFS(mfs)
		cmd := newRootCmd(&o)

		_, globalCfg, _, _, err := o.loadConfigs(cmd)
		require.NoError(t, err)
		assert.Equal(t, "podman", globalCfg.Engine)
	})

	t.Run("CDERUN_TOOL_CONFIG environment variable", func(t *testing.T) {
		mfs := &config.MockFileSystem{
			Env: map[string]string{
				"CDERUN_TOOL_CONFIG": "/env/tools.yaml",
			},
			Files: map[string][]byte{
				"/env/tools.yaml": []byte("node: { image: node:env }"),
			},
		}
		o := defaultOptions()
		o.fs = mfs
		o.configLoader = config.NewConfigLoaderWithFS(mfs)
		cmd := newRootCmd(&o)

		toolsCfg, _, _, _, err := o.loadConfigs(cmd)
		require.NoError(t, err)
		assert.Equal(t, "node:env", toolsCfg["node"].Image)
	})
}

type mockLargeFd struct{}

func (m mockLargeFd) Fd() uintptr {
	// Return a value that is definitely larger than math.MaxInt64 on any platform
	// but uintptr can hold it. Since math.MaxInt is what we check in getFd.
	return ^uintptr(0)
}

func TestUnit_Root_GetFd_LargeValue(t *testing.T) {
	t.Parallel()
	fd, ok := getFd(mockLargeFd{})
	assert.False(t, ok)
	assert.Equal(t, -1, fd)
}

func TestUnit_Root_AttachContainer_Timeout(t *testing.T) {
	t.Parallel()

	t.Run("AttachContainer returns nil but does not signal ready", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		o := &rootOptions{logger: logging.NewLogger()}
		mockRuntime := &runtime.MockRuntime{
			AttachFunc: func(ctx context.Context, id string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error {
				// Success but no ready signal
				<-ctx.Done()
				return nil
			},
		}
		cmd := &cobra.Command{}

		att, err := o.attachContainer(ctx, cmd, mockRuntime, "test-container", &container.ContainerConfig{})
		require.ErrorIs(t, err, context.DeadlineExceeded)
		assert.Nil(t, att)
	})

	t.Run("AttachContainer returns error before ready", func(t *testing.T) {
		o := &rootOptions{logger: logging.NewLogger()}
		mockRuntime := &runtime.MockRuntime{
			AttachFunc: func(ctx context.Context, id string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error {
				return errors.New("immediate attach error")
			},
		}
		cmd := &cobra.Command{}

		att, err := o.attachContainer(context.Background(), cmd, mockRuntime, "test-container", &container.ContainerConfig{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "immediate attach error")
		assert.Nil(t, att)
	})
}

func TestUnit_Root_WriteFormatted(t *testing.T) {
	t.Parallel()

	o := &rootOptions{logger: logging.NewLogger()}
	err := o.writeFormatted(io.Discard, "invalid", nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported output format: \"invalid\"")
}

func TestUnit_Root_PreprocessArgs(t *testing.T) {
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

func TestUnit_Root_CreateSnapshot(t *testing.T) {
	t.Parallel()

	logger := logging.NewLogger()

	t.Run("MkdirAll failure", func(t *testing.T) {
		mfs := &RootErrorFS{
			MockFileSystem: &config.MockFileSystem{},
			MkdirErr:       errors.New("mkdir failed"),
		}
		_, _, err := createSnapshot(logger, mfs, &config.CDERunConfig{}, nil, nil, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create snapshot directory: mkdir failed")
	})

	t.Run("WriteFile .cderun.yaml failure", func(t *testing.T) {
		mfs := &RootErrorFS{
			MockFileSystem: &config.MockFileSystem{},
			WriteFileFunc: func(path string, data []byte, perm os.FileMode) error {
				if filepath.Base(path) == ".cderun.yaml" {
					return errors.New("write cderun failed")
				}
				return nil
			},
		}
		_, _, err := createSnapshot(logger, mfs, &config.CDERunConfig{}, nil, nil, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to write .cderun.yaml to snapshot: write cderun failed")
	})

	t.Run("WriteFile .tools.yaml failure", func(t *testing.T) {
		mfs := &RootErrorFS{
			MockFileSystem: &config.MockFileSystem{},
			WriteFileFunc: func(path string, data []byte, perm os.FileMode) error {
				if filepath.Base(path) == ".tools.yaml" {
					return errors.New("write tools failed")
				}
				return nil
			},
		}
		_, _, err := createSnapshot(logger, mfs, &config.CDERunConfig{}, nil, nil, nil)
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

		_, _, err := createSnapshot(logger, mfs, globalCfg, nil, nil, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to resolve snapshot directory to host path")
	})
}
