# Feature Specification: Interactive Terminal Support

This feature implements robust interactive terminal support in `cderun`, ensuring a seamless and transparent user experience when executing interactive shells or Terminal User Interface (TUI) applications within ephemeral containers.

The implementation is designed following the technical architectural patterns documented in [go-cli-container-interaction.md](../references/go-cli-container-interaction.md) and technical investigations in [terminal-crash-investigation.md](./terminal-crash-investigation.md).

---

<!-- coderabbit-ai-generated:v1 -->

<!-- coderabbit-ai-generated-section:v1:8ad4072440ca5d929a4811319d0c272b9ec9302beb5a9fd5d5ec653473b1b9ac -->

## Feature Overview

### 1. Terminal Raw Mode

When a pseudo-TTY is allocated (`--tty` / `-t`), `cderun` uses the `golang.org/x/term` library to transition the host's standard input file descriptor (`stdinFd`, typically `0`) into "Raw Mode" via `term.MakeRaw(stdinFd)`:

- **Input Buffering & Echoing**: Disables local character echoing and line buffering, forwarding all keypresses (including terminal escape sequences, control characters like `\x03` for Ctrl+C, and raw VT100/ANSI sequences) directly to the containerized process.
- **Terminal State Restoration**: Terminal properties are safely and automatically restored on host process termination or signal interruption via `defer term.Restore(stdinFd, oldState)`.

### 2. Signal Trapping and Forwarding

`cderun` traps standard OS lifecycle signals (such as `SIGINT`, `SIGTERM`, and `SIGHUP` on Unix / `os.Interrupt` on Windows) received on the host and forwards them directly to the containerized process via the container engine APIs:

- **Signal Trapping**: Intercepts OS signals delivered to the host process. In Unix raw mode, `Ctrl+C` and `Ctrl+\` are delivered as input bytes (`\x03` and `\x1c`) and forwarded through stdin; those keypresses do not invoke the two-phase host signal handler.
- **Signal Forwarding**: Forwards trapped signals directly to the running container process via `Runtime.SignalContainer`.
- **Termination Hierarchy**: A two-phase termination model allows graceful shutdown upon the first signal while enforcing immediate context cancellation and cleanup if a second signal is received.

### 3. Window Resize Synchronization (`SIGWINCH` on Unix)

On Unix platforms, the execution engine monitors terminal window resize events (`SIGWINCH`) on the host using `setupResizeSignal`:

- **Dynamic Dimension Synchronization**: When a resize occurs, the active terminal dimension (rows and columns) is dynamically fetched from the host terminal descriptor using `termGetSize(fd)`.
- **PTY Boundary Validation**: Window geometry updates are guarded with non-negative bounds checks (`if err == nil && h >= 0 && w >= 0`). Dimension coordinates are converted to unsigned integers and dispatched to the container engine via `Runtime.ResizeContainerTTY(ctx, containerID, uint(h), uint(w))`.
- **Initial Geometry Calibration**: After `StartContainer` returns, `startResizeHandler` applies an initial resize to match current host terminal dimensions. Container execution may already have begun.
- **Layout Distortion Prevention**: Dynamic window sizing prevents interface layout distortion when executing rich console utilities, pagination tools, or text editors (such as `vim`, `nano`, `htop`, or `less`).

### 4. Robust I/O Lifecycle and Goroutine Cleanup

To prevent background goroutine leaks and terminal deadlocks:

- **Stream Monitoring**: All standard I/O streams are strictly monitored in separate goroutines.
- **Stream Termination**: Closing stream pipes lets copier goroutines blocked on those pipes return. This does not guarantee that every copier terminates when the container exits: the stdin copier may remain blocked reading host stdin after the container connection closes.

### 5. Terminal Crash Mitigation

Historical technical investigations ([T01 / terminal-crash-investigation.md](./terminal-crash-investigation.md)) describe terminal crashes during cursor wrapping and hypothesize geometry and stream synchronization mismatches; the crash cause remains unconfirmed. Mitigation is incomplete: the investigation recommends rejecting zero dimensions, but the current resize check (`h >= 0 && w >= 0`) accepts them. The initial resize also occurs after container startup, so it does not guarantee synchronized startup geometry.

---

<!-- /coderabbit-ai-generated:v1 -->

<!-- coderabbit-ai-generated:v1 -->

<!-- coderabbit-ai-generated-section:v1:0dda7ab49b7f48783aa7ef1e59674de753081f8ce61a4070b6b4cc0c03001c63 -->

## Technical Details

- **Terminal Raw Mode**: Configured via `term.MakeRaw(int(os.Stdin.Fd()))` and restored with `term.Restore`.
- **Signal Forwarding**: Orchestrated via buffered Go `os/signal` channels and the abstract `Runtime.SignalContainer` API.
- **Resize Tracking**: On Unix systems, resize tracking is trapped via `SIGWINCH` (handled in `signals_unix.go`) and applied to the container using the abstract `Runtime.ResizeContainerTTY` API. On Windows, resize tracking is currently unsupported because `setupResizeSignal` is a stub: only the initial resize is applied when the terminal checks pass, and subsequent window resize events are not handled.

<!-- /coderabbit-ai-generated:v1 -->
