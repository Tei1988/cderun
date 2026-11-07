# Feature Specification: Hang Timeout Protection

This document details the design, configuration, and execution conditions of the automated Hang Timeout Protection mechanism in `cderun`.

---

## Overview

In non-interactive or non-TTY environments, container execution can occasionally hang due to network blockages, blocked I/O descriptors, or zombie processes. `cderun` implements an automated termination routine (sending `SIGKILL`) to safeguard CI pipelines and batch jobs from hanging indefinitely.

---

## Execution and Detection Flow

When the host's standard input is not a terminal, or when interactive mode (`--interactive` / `-i`) is disabled, `cderun` uses the following parallel flow to detect and resolve hangs:

1. **Parallel Monitoring**: The engine starts two concurrent monitoring loops: one waiting for the container's natural exit (`WaitContainer`) and another waiting for the input/output streams to complete (`AttachContainer`).
2. **I/O Completion Detection**: Once the container's output stream receives EOF (indicating the process has closed its stdout and stderr streams), the I/O monitoring loop completes (`attachDone`).
3. **Hang Timeout Window**: If the container does not naturally exit within a specified grace period after its I/O streams have finished, the engine determines that the execution is hung.
4. **Two-Step Enforced Termination**: Upon timeout expiration, if the container remains active, the engine initiates a two-phase termination procedure:
   - **Phase 1: Inspection & Asynchronous SIGKILL**: `signalKillIfRunning` inspects the container state via `rt.InspectContainer(ctx, containerID)`. If the container is still running, it issues `rt.SignalContainer(ctx, containerID, "SIGKILL")`. This inspection and signaling step is bounded by a dedicated 5-second timeout context. If `InspectContainer` fails, it falls back defensively to sending `SIGKILL`.
   - **Phase 2: Post-SIGKILL Wait Window**: After sending `SIGKILL`, the runner waits for up to another `effectiveHangTimeout` interval for `WaitContainer` to receive the container's termination.
   - **Enforced Exit Code 125**: If the container fails to exit even after this post-SIGKILL wait window, `cderun` returns an explicit exit error:

     ```text
     ExitCodeError{Code: 125, Err: fmt.Errorf("container %s failed to exit after SIGKILL timeout", containerID)}
     ```

### Output Synchronization Grace Period (`drainAttachOutput`)

When `WaitContainer` finishes *before* `AttachContainer` (i.e. the container exited while some output streams are still draining), `cderun` implements a hardcoded **5-second** grace period (`attachGracePeriod`) in `drainAttachOutput`.

During this window, the engine attempts to drain any remaining stdout/stderr bytes from `att.attachDone`. If the streams do not close within 5 seconds, the engine cancels the attach context (`att.cancelAttach()`) and drains the channel to prevent I/O blocking. This grace period operates independently of the user-configured `hang-timeout`.

---

## Configuration

The hang timeout duration can be adjusted using any of the following configuration layers (listed in priority order):

- **Default Value**: `10s` (10 seconds)
- **Format**: Go Duration string (e.g., `10s`, `5s`, `0`).
- **Special Values**: `0` or any negative duration disables the timeout, waiting indefinitely for the container to terminate.
- **P1 Override**: `--cderun-hang-timeout`
- **P2 CLI Flag**: `--hang-timeout`
- **P3 Env Var**: `CDERUN_HANG_TIMEOUT`
- **P4 Tool Setting** (`.tools.yaml`): `hangTimeout`
- **P5 Global Defaults** (`.cderun.yaml`): `hangTimeout`

### Premature Attach Failure Handling

If `cderun` encounters a premature communication or attach failure *before* the container naturally exits, the error is logged as a warning (`o.logger.Warn("failed to attach to container: %v", err)`), and the timeout value controls its recovery:

- **If `hangTimeout > 0`**: It waits up to `effectiveHangTimeout` for the container to exit (`waitForWaitResultWithTimeout`). If the container exits within this period, its natural exit code is returned. If the wait window expires before container exit, execution terminates with an exit error:

  ```text
  ExitCodeError{Code: 125, Err: fmt.Errorf("timeout waiting for container to exit after attach error")}
  ```

- **If `hangTimeout <= 0`**: The engine blocks synchronously and waits indefinitely for the container's termination signal (`waitDone` channel) via `waitForWaitResult(waitDone, true)` rather than immediately cutting off execution, returning the container's natural exit code.

---

## Application Matrix

The automated termination routine is applied selectively based on TTY and interactive settings:

| Host STDIN | `--interactive` / `-i` | Hang Timeout Protection |
| :--- | :--- | :--- |
| **Terminal (TTY)** | **Enabled** | **Deactivated** (Never sends SIGKILL) |
| **Terminal (TTY)** | Disabled | **Activated** |
| Pipe or File | Enabled | **Activated** |
| Pipe or File | Disabled | **Activated** |

### Additional Operational Notes

- **Interactive Shell Executions**: Running `cderun -i alpine sh` on a terminal deactivates the hang timeout protection (`getHangTimeout` returns `0`), letting the shell remain open indefinitely to receive user input.
- **Piped Executions**: Running piped operations like `echo "test" | cderun cat` activates hang timeout protection. After input streams close, the container has a maximum of `hang-timeout` duration (default `10s`) to exit before being terminated with `SIGKILL`.
