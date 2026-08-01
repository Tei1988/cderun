package command

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cderun/internal/config"
	"cderun/internal/runtime"
)

type waitResult struct {
	code int
	err  error
}

func (o *rootOptions) getHangTimeout(isHostStdinTerminal bool, interactive bool, resolved *config.ResolvedConfig) time.Duration {
	// Hang timeout only applies if host stdin is NOT a TTY or if interactive mode is NOT enabled.
	if isHostStdinTerminal && interactive {
		return 0
	}

	if resolved != nil {
		return resolved.HangTimeout
	}

	// Fallback to default hangTimeout (e.g. 10s)
	return hangTimeout
}

func fallbackExitCode(code int) int {
	if code == 0 {
		return 125
	}
	return code
}

func (o *rootOptions) signalKillIfRunning(ctx context.Context, rt runtime.ContainerRuntime, containerID string) (int, error) {
	isRunning, exitCode, err := rt.InspectContainer(ctx, containerID)
	if err != nil {
		o.logger.Debug("failed to inspect container %s before kill: %v", containerID, err)
		// Fallback to kill if inspect fails
		isRunning = true
	}

	if !isRunning {
		o.logger.Debug("Container %s already exited with code %d, skipping SIGKILL", containerID, exitCode)
		return exitCode, nil
	}

	o.logger.Debug("Container %s still running, forcing termination", containerID)
	if err := rt.SignalContainer(ctx, containerID, "SIGKILL"); err != nil {
		o.logger.Debug("failed to force terminate container %s: %v", containerID, err)
		return exitCode, &ExitCodeError{Code: fallbackExitCode(exitCode), Err: fmt.Errorf("failed to force terminate container: %w", err)}
	}

	// SIGKILL is sent asynchronously. The caller should wait for the container to exit if needed.
	return exitCode, nil
}

// drainRemainingOutput waits for remaining output after container exits, up to attachGracePeriod.
func (o *rootOptions) drainRemainingOutput(containerID string, exitCode int, att *attachResult) error {
	o.logger.Trace("Waiting for remaining output from container %s (grace period: %v)", containerID, o.attachGracePeriod)
	select {
	case err := <-att.attachDone:
		if err != nil && !errors.Is(err, context.Canceled) {
			o.logger.Debug("AttachContainer finished with error after container exit for %s: %v", containerID, err)
			return &ExitCodeError{Code: fallbackExitCode(exitCode), Err: fmt.Errorf("failed to attach to container: %w", err)}
		}
		o.logger.Debug("AttachContainer finished successfully for %s", containerID)
	case <-time.After(o.attachGracePeriod):
		o.logger.Debug("AttachContainer timed out after container exit for %s, forcing close", containerID)
		att.cancelAttach()
		<-att.attachDone
	}
	return nil
}

// handleAttachErrorBeforeExit handles error on AttachContainer before the container exits.
func (o *rootOptions) handleAttachErrorBeforeExit(containerID string, err error, effectiveHangTimeout time.Duration, waitDone chan waitResult) (int, error) {
	o.logger.Debug("AttachContainer finished with error before container exit for %s: %v", containerID, err)
	// Wait for container to finish (best effort)
	// We do NOT call cancel() here to allow rt.WaitContainer to continue normally
	// until it finishes, the timeout expires, or a second signal is received.
	var exitCode int
	var waitErr error
	if effectiveHangTimeout > 0 {
		select {
		case res := <-waitDone:
			exitCode = res.code
			waitErr = res.err
		case <-time.After(effectiveHangTimeout):
			o.logger.Debug("Timeout waiting for container %s after attach error", containerID)
		}
	} else {
		res := <-waitDone
		exitCode = res.code
		waitErr = res.err
	}

	errReason := fmt.Errorf("failed to attach to container: %w", err)
	if waitErr != nil {
		errReason = fmt.Errorf("failed to attach to container: %v; wait failed: %w", err, waitErr)
	}

	return exitCode, &ExitCodeError{Code: fallbackExitCode(exitCode), Err: errReason}
}

// handleIOFinishedBeforeExit handles the case where AttachContainer finished before WaitContainer.
func (o *rootOptions) handleIOFinishedBeforeExit(ctx context.Context, rt runtime.ContainerRuntime, containerID string, effectiveHangTimeout time.Duration, waitDone chan waitResult) (int, error) {
	var exitCode int
	if effectiveHangTimeout > 0 {
		o.logger.Trace("IO finished, waiting up to %v for container %s to exit", effectiveHangTimeout, containerID)
		select {
		case result := <-waitDone:
			if result.err != nil {
				return 0, &ExitCodeError{Code: 125, Err: fmt.Errorf("failed to wait for container: %w", result.err)}
			}
			exitCode = result.code
		case <-time.After(effectiveHangTimeout):
			killCtx, killCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer killCancel()
			var err error
			exitCode, err = o.signalKillIfRunning(killCtx, rt, containerID)
			if err != nil {
				return 0, err
			}
			select {
			case result := <-waitDone:
				if result.err != nil {
					return 0, &ExitCodeError{Code: 125, Err: fmt.Errorf("failed to wait for container: %w", result.err)}
				}
				exitCode = result.code
			case <-time.After(effectiveHangTimeout):
				return exitCode, &ExitCodeError{Code: fallbackExitCode(exitCode), Err: fmt.Errorf("container %s failed to exit after SIGKILL timeout", containerID)}
			}
		}
	} else {
		// effectiveHangTimeout is 0, wait indefinitely
		o.logger.Trace("IO finished, waiting indefinitely for container %s to exit", containerID)
		result := <-waitDone
		if result.err != nil {
			return 0, &ExitCodeError{Code: 125, Err: fmt.Errorf("failed to wait for container: %w", result.err)}
		}
		exitCode = result.code
	}
	return exitCode, nil
}
