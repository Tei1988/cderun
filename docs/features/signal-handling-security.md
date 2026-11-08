# Signal Handling Security

`cderun` provides secure mechanisms for signaling containers, ensuring that signal names and values are strictly validated before being passed to the container runtime and that host signal events are safely synchronized across the container lifecycle.

## Signal Name Validation

All signals sent via the `SignalContainer` method in adapters that implement validation (such as Docker and Mock runtimes) are validated against a case-insensitive regular expression:
`^(?i)[A-Z0-9]+$`

This regex pattern ensures that only alphanumeric characters are accepted, preventing any shell metacharacters, whitespace, or control characters from being passed to the runtime. This effectively blocks command injection into the underlying shell or container runtime. Note that CLI wrappers such as `NerdctlRuntime.SignalContainer` pass the signal as a joined argument via `exec.CommandContext` without applying `signalRegex`.

Note that this validation acts as a character whitelist rather than a static signal allowlist; unknown alphanumeric signals are passed to the engine, which will subsequently handle any unsupported signal errors natively.

## Zero-Gap Early Signal Registration & Safe Cleanup

To prevent the creation of orphan containers, `cderun` registers standard signals (`SIGINT`, `SIGTERM`, `SIGHUP`, `SIGQUIT` on Unix-like systems, or `os.Interrupt` on Windows) at the very beginning of the command execution lifecycle, prior to image pulling or container creation.

If a termination signal is received *before* the container has begun startup:

- The execution is safely cancelled via context cancellation (`ctxG`).
- Ongoing image pulling or container creation operations are immediately aborted.
- Any partially created containers have deferred removal attempted when `initContainer` has removal enabled (`cc.Remove = true`, which is the default unless `--remove=false` is specified). Removal runs with an uncancelled base context via `context.WithoutCancel` to ensure cleanup commands execute even though the execution context is cancelled, though successful deletion remains subject to runtime availability.
- The container is prevented from starting if cancellation is triggered before the start phase.

## SIGHUP and SIGQUIT Support & Buffer Optimization

In addition to `SIGINT` and `SIGTERM`, `cderun` listens to and handles:

- **SIGHUP**: Sent when the controlling terminal is closed. Handling SIGHUP ensures that terminal closure does not leak orphan containers on the host.
- **SIGQUIT**: Sent via terminal quit character.

To reduce the likelihood of rapidly repeated or consecutive signals being dropped, the internal signal channel is constructed with a best-effort buffer size of 4 (`make(chan os.Signal, 4)`). Because Go's `signal.Notify` sends non-blocking notifications, signals may still be dropped if this buffer fills.

## Runtime Isolation

Signal validation is implemented at the runtime abstraction layer. This prevents malicious subcommands or environment-based overrides from injecting arbitrary commands or metacharacters into the runtime's signal delivery path (e.g., `SIGTERM; rm -rf /`).

If a signal containing restricted characters is provided, the validation fails immediately, preventing potential injection vulnerabilities before any calls are made to the underlying engine.

## Error Handling

The runtime abstraction suppresses "not found" or "conflict" errors (e.g., if the container has already exited) to ensure that signaling is idempotent and does not cause unexpected CLI failures during cleanup or shutdown sequences.

## Signal Forwarding and Lifecycle Synchronization

During command execution, the early signal channel feeds into `startSignalForwarder`. The forwarder delegates signal dispatch decisions to a thread-safe execution state coordinator (`executionState`), which enforces the following three-phase lifecycle state machine:

### 1. Pre-Startup Phase (`!startupBegun`)

- **State**: The container has not begun its startup routine (e.g., during image pulling, container creation, or terminal setup).
- **Behavior**: Receiving any registered signal immediately sets `cancelCtx = true`, invoking the root execution context cancellation (`cancel()`).
- **Teardown**: Image pull and container creation abort immediately. Deferred cleanup functions attempt container removal when removal is enabled (`cc.Remove = true`), running with an uncancelled timeout context (`context.WithoutCancel(ctx)`).

### 2. Startup In-Flight Phase (`startupBegun && !containerRunning`)

- **State**: `rt.StartContainer` has been invoked or is actively progressing, but has not yet returned.
- **Behavior**: When the first signal arrives (`firstSignal = true`), `HandleSignal` queues the signal name into `deferredSignals` and returns `forwardImmediate = true`.
- **Immediate Forwarding Attempt & Deferred Queuing**: The forwarder spawns an asynchronous `rt.SignalContainer` attempt immediately. Additionally, when `rt.StartContainer` returns successfully, `MarkRunning()` replays all queued signals sequentially. Consequently, the container runtime may receive two forwarding calls for that signal.
- **Secondary Signals**: If a second signal arrives while startup is still in-flight, `cancelCtx` is triggered, cancelling `ctxG` to abort the host execution.

### 3. Running Container Phase (`containerRunning`)

- **First Signal**: The signal is forwarded immediately and asynchronously to the running container via `rt.SignalContainer(ctx, containerID, sigName)` in a separate goroutine. This allows the containerized workload to perform graceful shutdown or signal handling (e.g., catching `SIGINT` or `SIGTERM`).
- **Second Signal**: If a second interrupt or termination signal is received by the host process while the container is running, `HandleSignal` returns `cancelCtx = true`, immediately cancelling `ctxG`. Note that if deferred signal replay is currently in progress, it executes synchronously against the parent context `ctx` rather than `ctxG`; teardown proceeds after that call returns. When removal is enabled (`cc.Remove = true`), deferred cleanup attempts to remove the container.
