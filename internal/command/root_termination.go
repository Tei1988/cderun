package command

import (
	"context"
	"time"
	"cderun/internal/runtime"
)

func (o *rootOptions) getHangTimeout(isHostStdinTerminal bool, interactive bool) time.Duration {
	if val := o.fs.Getenv("CDERUN_HANG_TIMEOUT"); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			return d
		}
	}
	// The conditional below is kept for future differentiation of TTY vs non-TTY
	// timeout behavior, although they currently both use hangTimeout (2s)
	// to ensure stability in CI environments.
	if !isHostStdinTerminal || !interactive {
		return o.hangTimeout
	}
	return o.hangTimeout
}

func (o *rootOptions) forceTerminateIfRunning(ctx context.Context, rt runtime.ContainerRuntime, containerID string) (int, error) {
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
	if err := rt.SignalContainer(context.Background(), containerID, "SIGKILL"); err != nil {
		o.logger.Debug("failed to force terminate container %s: %v", containerID, err)
	}

	// Wait for the kill to take effect
	return exitCode, nil
}
