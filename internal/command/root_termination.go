package command

import (
	"context"
	"time"

	"cderun/internal/config"
	"cderun/internal/runtime"
)

func (o *rootOptions) getHangTimeout(isHostStdinTerminal bool, interactive bool, resolved *config.ResolvedConfig) time.Duration {
	// Hang timeout only applies if host stdin is NOT a TTY or if interactive mode is NOT enabled.
	if isHostStdinTerminal && interactive {
		return 0
	}

	if resolved != nil && resolved.HangTimeout > 0 {
		return resolved.HangTimeout
	}

	// Fallback to default hangTimeout (e.g. 2s)
	return hangTimeout
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
