package command

import (
	"bytes"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cderun/internal/config"
	"cderun/internal/container"
	"cderun/internal/logging"
	"cderun/internal/runtime"
)

func TestUnit_Command_Prune_ErrorPaths(t *testing.T) {
	t.Parallel()

	t.Run("Prune fails when runtimeFactory returns error", func(t *testing.T) {
		t.Parallel()
		opts := &rootOptions{
			logger: logging.NewLogger(),
			fs:     config.RealFileSystem{},
			runtimeFactory: func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return nil, errors.New("runtime daemon unreachable")
			},
		}
		cmd := &cobra.Command{}
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(new(bytes.Buffer))
		resolved := &config.ResolvedConfig{
			Prune:    true,
			LogLevel: "error",
			Runtime:  "docker",
		}
		err := opts.handlePrune(cmd, resolved)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to initialize runtime: runtime daemon unreachable")
	})

	t.Run("Prune fails when runtime PruneContainers returns error", func(t *testing.T) {
		t.Parallel()
		mockRt := &runtime.MockRuntime{
			PruneErr: errors.New("simulated prune failure"),
		}
		opts := &rootOptions{
			logger: logging.NewLogger(),
			fs:     config.RealFileSystem{},
			runtimeFactory: func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mockRt, nil
			},
		}
		cmd := &cobra.Command{}
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(new(bytes.Buffer))
		resolved := &config.ResolvedConfig{
			Prune:    true,
			LogLevel: "error",
			Runtime:  "docker",
		}
		err := opts.handlePrune(cmd, resolved)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to prune orphan containers: simulated prune failure")
	})
}

type runtimeWithCloseErr struct {
	*runtime.MockRuntime
	closeErr error
}

func (r *runtimeWithCloseErr) Close() error {
	return r.closeErr
}

func TestUnit_Command_Prefetch_LifecycleErrorPaths(t *testing.T) {
	t.Parallel()

	t.Run("Prefetch fails when runtimeFactory returns error wrapping RuntimeInitError", func(t *testing.T) {
		t.Parallel()
		opts := &rootOptions{
			logger: logging.NewLogger(),
			fs:     config.RealFileSystem{},
			runtimeFactory: func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return nil, errors.New("socket connect error")
			},
		}
		cmd := &cobra.Command{}
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(new(bytes.Buffer))
		resolved := &config.ResolvedConfig{
			Prefetch: "node",
			LogLevel: "error",
			Runtime:  "docker",
		}
		toolsCfg := config.ToolsConfig{
			"node": config.ToolConfig{Image: "node:20-alpine"},
		}
		err := opts.handlePrefetch(cmd, resolved, toolsCfg)
		require.Error(t, err)
		var initErr *config.RuntimeInitError
		require.ErrorAs(t, err, &initErr)
		assert.Equal(t, "docker", initErr.Runtime)
		assert.Contains(t, initErr.Error(), "socket connect error")
	})

	t.Run("Prefetch succeeds and logs debug message when runtime Close returns error", func(t *testing.T) {
		t.Parallel()
		mockRt := &runtimeWithCloseErr{
			MockRuntime: runtime.NewMockRuntime(),
			closeErr:    errors.New("close resource leak"),
		}
		var logBuf bytes.Buffer
		l := logging.NewLogger()
		opts := &rootOptions{
			logger: l,
			fs:     config.RealFileSystem{},
			runtimeFactory: func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
				return mockRt, nil
			},
		}
		cmd := &cobra.Command{}
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(&logBuf)

		resolved := &config.ResolvedConfig{
			Prefetch: "node",
			LogLevel: "debug",
			Runtime:  "podman",
		}
		toolsCfg := config.ToolsConfig{
			"node": config.ToolConfig{Image: "node:20-alpine"},
		}
		err := opts.handlePrefetch(cmd, resolved, toolsCfg)
		require.NoError(t, err)
		assert.Contains(t, logBuf.String(), "failed to close runtime: close resource leak")
		assert.Contains(t, logBuf.String(), "Successfully prefetched all images.")
	})
}

func TestUnit_Command_AttachAndCompletion_BoundaryScenarios(t *testing.T) {
	t.Parallel()

	t.Run("waitForWaitResult unwrapped on error when wrapInExitCodeError is false", func(t *testing.T) {
		t.Parallel()
		waitDone := make(chan waitResult, 1)
		waitDone <- waitResult{code: 0, err: errors.New("container process lost")}

		code, err := waitForWaitResult(waitDone, false)
		require.Error(t, err)
		assert.Equal(t, 0, code)
		var exitErr *ExitCodeError
		assert.NotErrorAs(t, err, &exitErr)
		assert.Contains(t, err.Error(), "failed to wait for container: container process lost")
	})

	t.Run("waitForWaitResult wrapped in ExitCodeError 125 when wrapInExitCodeError is true", func(t *testing.T) {
		t.Parallel()
		waitDone := make(chan waitResult, 1)
		waitDone <- waitResult{code: 0, err: errors.New("container daemon failure")}

		code, err := waitForWaitResult(waitDone, true)
		require.Error(t, err)
		assert.Equal(t, 0, code)
		var exitErr *ExitCodeError
		require.ErrorAs(t, err, &exitErr)
		assert.Equal(t, 125, exitErr.Code)
		assert.Contains(t, exitErr.Error(), "failed to wait for container: container daemon failure")
	})

	t.Run("waitForWaitResult returns code on success", func(t *testing.T) {
		t.Parallel()
		waitDone := make(chan waitResult, 1)
		waitDone <- waitResult{code: 42, err: nil}

		code, err := waitForWaitResult(waitDone, true)
		require.NoError(t, err)
		assert.Equal(t, 42, code)
	})

	t.Run("waitForWaitResultWithTimeout returns code when wait completes before timeout", func(t *testing.T) {
		t.Parallel()
		waitDone := make(chan waitResult, 1)
		waitDone <- waitResult{code: 7, err: nil}

		code, timedOut, err := waitForWaitResultWithTimeout(waitDone, 100*time.Millisecond)
		require.NoError(t, err)
		assert.False(t, timedOut)
		assert.Equal(t, 7, code)
	})

	t.Run("waitForWaitResultWithTimeout returns timedOut=true when duration elapses", func(t *testing.T) {
		t.Parallel()
		waitDone := make(chan waitResult, 1)

		code, timedOut, err := waitForWaitResultWithTimeout(waitDone, 10*time.Millisecond)
		require.NoError(t, err)
		assert.True(t, timedOut)
		assert.Equal(t, 0, code)
	})

	t.Run("waitForWaitResultWithTimeout returns ExitCodeError 125 when wait returns error", func(t *testing.T) {
		t.Parallel()
		waitDone := make(chan waitResult, 1)
		waitDone <- waitResult{code: 0, err: errors.New("timeout channel broken")}

		code, timedOut, err := waitForWaitResultWithTimeout(waitDone, 100*time.Millisecond)
		require.Error(t, err)
		assert.False(t, timedOut)
		assert.Equal(t, 0, code)
		var exitErr *ExitCodeError
		require.ErrorAs(t, err, &exitErr)
		assert.Equal(t, 125, exitErr.Code)
		assert.Contains(t, exitErr.Error(), "failed to wait for container: timeout channel broken")
	})

	t.Run("handleAttachFinishedBeforeExit: attach error without hang timeout returns wait result", func(t *testing.T) {
		t.Parallel()
		opts := defaultOptions()
		mockRt := runtime.NewMockRuntime()
		cc := &container.ContainerConfig{Interactive: false}
		waitDone := make(chan waitResult, 1)
		waitDone <- waitResult{code: 137, err: nil}

		code, err := opts.handleAttachFinishedBeforeExit(mockRt, "c1", cc, false, 0, errors.New("stream closed"), waitDone)
		require.NoError(t, err)
		assert.Equal(t, 137, code)
	})

	t.Run("handleAttachFinishedBeforeExit: attach error without hang timeout wraps wait error in exit 125", func(t *testing.T) {
		t.Parallel()
		opts := defaultOptions()
		mockRt := runtime.NewMockRuntime()
		cc := &container.ContainerConfig{Interactive: false}
		waitDone := make(chan waitResult, 1)
		waitDone <- waitResult{code: 0, err: errors.New("inspect failed during wait")}

		code, err := opts.handleAttachFinishedBeforeExit(mockRt, "c1", cc, false, 0, errors.New("stream closed"), waitDone)
		require.Error(t, err)
		assert.Equal(t, 0, code)
		var exitErr *ExitCodeError
		require.ErrorAs(t, err, &exitErr)
		assert.Equal(t, 125, exitErr.Code)
		assert.Contains(t, exitErr.Error(), "failed to wait for container: inspect failed during wait")
	})

	t.Run("handleAttachFinishedBeforeExit: attach error with hang timeout success returns code", func(t *testing.T) {
		t.Parallel()
		opts := defaultOptions()
		mockRt := runtime.NewMockRuntime()
		cc := &container.ContainerConfig{Interactive: false}
		waitDone := make(chan waitResult, 1)
		waitDone <- waitResult{code: 0, err: nil}

		code, err := opts.handleAttachFinishedBeforeExit(mockRt, "c1", cc, false, 50*time.Millisecond, errors.New("attach failed"), waitDone)
		require.NoError(t, err)
		assert.Equal(t, 0, code)
	})

	t.Run("handleAttachFinishedBeforeExit: attach success, non-interactive, hang timeout elapses and kill fails", func(t *testing.T) {
		t.Parallel()
		opts := defaultOptions()
		failingRt := &TerminationMockRuntime{
			MockRuntime: &runtime.MockRuntime{SignalErr: errors.New("signal delivery failure")},
			InspectErr:  errors.New("inspect error"),
		}

		cc := &container.ContainerConfig{Interactive: false}
		waitDone := make(chan waitResult, 1) // leaves wait empty so timeout triggers

		// When InspectErr triggers, signalKillIfRunning falls back to signaling and logs warning, but doesn't return err directly
		// unless signalKillIfRunning itself errors.
		code, err := opts.handleAttachFinishedBeforeExit(failingRt, "c1", cc, false, 10*time.Millisecond, nil, waitDone)
		// Because inspect fails, signalKillIfRunning logs warning and returns (0, nil), then waits again and times out with code 125
		require.Error(t, err)
		assert.Equal(t, 0, code)
		var exitErr *ExitCodeError
		require.ErrorAs(t, err, &exitErr)
		assert.Equal(t, 125, exitErr.Code)
		assert.Contains(t, exitErr.Error(), "failed to exit after SIGKILL timeout")
	})

	t.Run("handleAttachFinishedBeforeExit: attach success, non-interactive, hang timeout elapses, kill sent, container exits", func(t *testing.T) {
		t.Parallel()
		opts := defaultOptions()
		mockRt := &TerminationMockRuntime{
			MockRuntime: runtime.NewMockRuntime(),
			IsRunning:   true,
		}

		cc := &container.ContainerConfig{Interactive: false}
		waitDone := make(chan waitResult, 1)

		var once sync.Once
		// Listen on mockRt.SigChan which is thread-safe and non-blocking in MockRuntime.SignalContainer
		go func() {
			for sig := range mockRt.SigChan {
				if sig == "SIGKILL" {
					once.Do(func() {
						waitDone <- waitResult{code: 137, err: nil}
					})
					return
				}
			}
		}()

		code, err := opts.handleAttachFinishedBeforeExit(mockRt, "c1", cc, false, time.Second, nil, waitDone)
		require.NoError(t, err)
		assert.Equal(t, 137, code)
	})

	t.Run("handleAttachFinishedBeforeExit: attach success, non-interactive, hang timeout elapses, kill sent, but wait times out again", func(t *testing.T) {
		t.Parallel()
		opts := defaultOptions()
		mockRt := &TerminationMockRuntime{
			MockRuntime: runtime.NewMockRuntime(),
			IsRunning:   true,
		}

		cc := &container.ContainerConfig{Interactive: false}
		waitDone := make(chan waitResult, 1) // never receives anything

		code, err := opts.handleAttachFinishedBeforeExit(mockRt, "c1", cc, false, 10*time.Millisecond, nil, waitDone)
		require.Error(t, err)
		assert.Equal(t, 0, code)
		var exitErr *ExitCodeError
		require.ErrorAs(t, err, &exitErr)
		assert.Equal(t, 125, exitErr.Code)
		assert.Contains(t, exitErr.Error(), "failed to exit after SIGKILL timeout")
	})

	t.Run("handleAttachFinishedBeforeExit: attach success, non-interactive, no hang timeout waits indefinitely", func(t *testing.T) {
		t.Parallel()
		opts := defaultOptions()
		mockRt := runtime.NewMockRuntime()
		cc := &container.ContainerConfig{Interactive: false}
		waitDone := make(chan waitResult, 1)
		waitDone <- waitResult{code: 10, err: nil}

		code, err := opts.handleAttachFinishedBeforeExit(mockRt, "c1", cc, false, 0, nil, waitDone)
		require.NoError(t, err)
		assert.Equal(t, 10, code)
	})

	t.Run("handleAttachFinishedBeforeExit: attach success, interactive terminal waits directly", func(t *testing.T) {
		t.Parallel()
		opts := defaultOptions()
		mockRt := runtime.NewMockRuntime()
		cc := &container.ContainerConfig{Interactive: true}
		waitDone := make(chan waitResult, 1)
		waitDone <- waitResult{code: 0, err: nil}

		code, err := opts.handleAttachFinishedBeforeExit(mockRt, "c1", cc, true, 0, nil, waitDone)
		require.NoError(t, err)
		assert.Equal(t, 0, code)
	})
}
