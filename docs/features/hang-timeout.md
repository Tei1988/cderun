# Feature Specification: Hang Timeout Protection

This document details the design, configuration, and execution conditions of the automated Hang Timeout Protection mechanism in `cderun`.

---

## Overview

In non-interactive or non-TTY environments, container execution can occasionally hang due to network blockages, blocked I/O descriptors, or zombie processes. `cderun` implements an automated termination routine (sending `SIGKILL`) to safeguard CI pipelines and batch jobs from hanging indefinitely.

---

## Execution and Detection Flow

When the host's standard input is not a terminal, or when interactive mode (`--interactive` / `-i`) is disabled, `cderun` uses the following parallel flow to detect and resolve hangs:

1. **Parallel Monitoring**: The engine starts two concurrent monitoring loops: one waiting for the container's natural exit (`WaitContainer`) and another waiting for the input/output streams to complete (`AttachContainer`).
2. **I/O Completion Detection**: Once the container's output stream receives EOF (indicating the process has closed its stdout and stderr streams), the I/O monitoring loop completes.
3. **Hang Timeout Window**: If the container does not naturally exit within a specified grace period after its I/O streams have finished, the engine determines that the execution is hung.
4. **Enforced Termination**: Upon timeout expiration, if the container remains active, the engine sends a `SIGKILL` signal to terminate the container and returns shell control to the host.

### Output Synchronization Grace Period

To ensure outstanding container outputs are not lost, `cderun` implements a hardcoded **5-second** grace period (`attachGracePeriod`) after the container has exited.

During this window, the engine attempts to drain any remaining stdout/stderr bytes. If the streams do not close within 5 seconds, the engine deactivates the attach connection to prevent I/O blocking. This grace period operates independently of the user-configured `hang-timeout`.

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

If `cderun` encounters a premature communication or attach failure *before* the container naturally exits, the timeout value controls its recovery:

- **If `hangTimeout > 0`**: It initiates the configured countdown and terminates execution if exceeded.
- **If `hangTimeout <= 0`**: The engine blocks synchronously and waits indefinitely for the container's termination signal (`waitDone` channel) rather than immediately cutting off execution.

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

- **Interactive Shell Executions**: Running `cderun -i alpine sh` on a terminal deactivates the hang timeout protection, letting the shell remain open indefinitely to receive user input.
- **Piped Executions**: Running piped operations like `echo "test" | cderun cat` activates hang timeout protection. After input streams close, the container has a maximum of `hang-timeout` duration (default `10s`) to exit before being terminated with `SIGKILL`.
