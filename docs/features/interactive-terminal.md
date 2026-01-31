# Interactive Terminal Support (Phase 4)

This feature implements robust interactive terminal support for `cderun`, ensuring a seamless "transparent" experience when running interactive shells or TUI applications inside containers.

The implementation is based on the technical patterns described in [docs/references/go-cli-container-interaction.md](../references/go-cli-container-interaction.md).

## Features

### 1. Terminal Raw Mode
When TTY is enabled (`--tty`), the host's terminal is set to "Raw Mode" using `golang.org/x/term`. This disables local echo and line buffering, allowing all key strokes (including control characters) to be sent directly to the containerized process. The terminal state is automatically restored upon exit.

### 2. Signal Handling and Forwarding
`cderun` captures lifecycle signals (`SIGINT`, `SIGTERM`) received on the host and forwards them to the containerized process via the container runtime API. This ensures that pressing `Ctrl+C` or sending a termination signal to `cderun` correctly cleans up the process inside the container.

### 3. Window Resize Synchronization (SIGWINCH)
`cderun` monitors the host terminal for window resize events (`SIGWINCH`). When the terminal is resized, the new dimensions (rows and columns) are dynamically synchronized with the container's TTY, preventing display corruption in TUI applications like `vim` or `htop`.

### 4. Robust I/O Management and Cleanup
I/O streams are managed to prevent goroutine leaks. Connections are properly closed when the container exits, ensuring that all background relay goroutines terminate correctly.

### 5. Windows ConPTY Support (Future)
Support for Windows Pseudo Console (ConPTY) is planned for a future phase to provide a consistent interactive experience on Windows hosts.

## Implementation Details
- **Raw Mode**: `term.MakeRaw(int(os.Stdin.Fd()))`
- **Signal Forwarding**: `os/signal` and `Runtime.SignalContainer`
- **Resize**: `syscall.SIGWINCH` and `Runtime.ResizeContainerTTY`
