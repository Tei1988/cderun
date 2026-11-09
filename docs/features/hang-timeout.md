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
   - **Runtime-Specific Scope & Container Engine Limitations**:
     - **Docker Non-TTY Limitation**: While Docker generally completes output stream monitoring upon EOF in non-interactive sessions, specifying `-i` without a TTY sets `StdinOnce: true`. If Docker's internal stream wait is registered after container start, the attachment connection may wait for container exit before closing. In such scenarios, a process that closes its output descriptors and subsequently hangs can bypass this EOF-based timeout.
     - **containerd Output-Drain Limitation**: For containerd, `AttachContainer` completion signals task exit rather than completion of output stream copying. Consequently, a containerd process that closes its output descriptors and subsequently hangs will not trigger this timeout mechanism, and waiting on `attachDone` or the grace period does not guarantee that remaining output has completely drained.
3. **Hang Timeout Window**: If the container does not naturally exit within a specified grace period after its I/O streams have finished, the engine determines that the execution is hung.
4. **Two-Step Enforced Termination**: Upon timeout expiration, if the container remains active, the engine initiates a two-phase termination procedure:
   - **Phase 1: Inspection & Asynchronous SIGKILL**: `signalKillIfRunning` inspects the container state via `rt.InspectContainer(ctx, containerID)`. If the container is still running, it issues `rt.SignalContainer(ctx, containerID, "SIGKILL")`. This inspection and signaling step is bounded by a dedicated 5-second timeout context. If `InspectContainer` fails, it falls back defensively to sending `SIGKILL`.
   - **Phase 2: Post-SIGKILL Wait Window**: After sending `SIGKILL`, the runner waits for up to another `effectiveHangTimeout` interval for `WaitContainer` to receive the container's termination.
   - **Enforced Exit Code 125**: If the container fails to exit even after this post-SIGKILL wait window, `cderun` returns an explicit exit error:

     ```text
     ExitCodeError{Code: 125, Err: fmt.Errorf("container %s failed to exit after SIGKILL timeout", containerID)}
     ```

### Output Synchronization Grace Period (`drainAttachOutput`)

`AttachContainer` writes container stdout and stderr directly to the configured output writers. When `WaitContainer` finishes *before* `AttachContainer` (i.e. the container exited while output streams are still draining), `cderun` implements a hardcoded **5-second** grace period (`attachGracePeriod`) in `drainAttachOutput`.

During this window, `drainAttachOutput` waits for completion via `att.attachDone` (which conveys the attach completion error rather than output bytes). If the attach routine does not complete within 5 seconds, the engine cancels the attach context (`att.cancelAttach()`) and drains `att.attachDone` to prevent goroutine leaks. Notably, the post-cancellation receive on `<-att.attachDone` has no timeout; with adapters such as nerdctl, an unblocked interactive stdin read or pending CLI subprocess wait (`cmd.Wait()`) can block indefinitely, meaning the 5-second grace period does not strictly bound total drain time. This grace period operates independently of the user-configured `hang-timeout`.

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

### Post-Startup Attach Failure Handling

If `attachContainer` fails before attachment is ready, it attempts to return immediately with exit code 125. However, because certain runtime implementations (such as `DockerRuntime.AttachContainer`) close the `attachReady` channel before returning an attachment error, `attachContainer` may select that readiness signal and proceed to start the container despite the failure.

If communication or attachment fails *after* container startup, the error is logged as a warning (`o.logger.Warn("failed to attach to container: %v", err)`), and recovery is governed by `effectiveHangTimeout`:

- **If `effectiveHangTimeout > 0`**: The runner waits up to `effectiveHangTimeout` for the container to exit (`waitForWaitResultWithTimeout`). Should the container terminate within this window, its natural exit code is returned. If the wait window expires before container termination, execution terminates with an exit error:

  ```text
  ExitCodeError{Code: 125, Err: fmt.Errorf("timeout waiting for container to exit after attach error")}
  ```

  *Distinction from Termination Flow*: Unlike the two-step termination flow for standard execution hangs, this attach-error timeout path returns exit code 125 directly without calling `signalKillIfRunning`. Consequently, if container removal is disabled (`--remove=false`), the container may continue running in the background after `cderun` exits.

- **If `effectiveHangTimeout <= 0`**: The engine blocks synchronously and waits indefinitely for the container's termination signal (`waitDone` channel) via `waitForWaitResult(waitDone, true)` rather than prematurely terminating, returning the container's natural exit code.

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
