# Feature Specification: Standard Input Synchronization

This document explains the standard input (STDIN) synchronization mechanism implemented in `cderun` to ensure reliable pipe-based command execution.

## The Challenge: Race Conditions in Pipe Executions

When running shell pipes (e.g., `echo "test" | cderun ... cat`), a severe race condition can occur between the following operations:

1. **STDIN Attachment**: `cderun` invokes the container runtime's attachment API to stream host STDIN to the container.
2. **Container Start**: `cderun` invokes the container runtime's start API to boot the target containerized command (e.g., `cat`).
3. **STDIN Consumption**: The containerized command opens and starts reading from its internal STDIN.

If the host STDIN data is available instantly (as with `echo` or files), and `cderun` eagerly streams the entire input stream *before* the containerized process officially shifts into a running state, the data may be dropped or fail to be processed by the container daemon.

Furthermore, if the data is small (such as a single word or character), `cderun` might read the EOF from the host pipe, transmit the data, and immediately invoke `CloseWrite()` on the stream before the container process even registers. This can cause the containerized process to exit with an immediate EOF without ever reading the transmitted data.

## Solution: Synchronized STDIN

To ensure deterministic, reliable pipe execution, `cderun` incorporates a synchronized STDIN mechanism utilizing a custom `syncReader`.

### 1. Deferred STDIN Streaming

The host's STDIN stream is wrapped with a custom `syncReader` before being passed to the runtime adapter's `AttachContainer` method. This reader blocks any incoming `Read` operations until a ready signal is dispatched.

### 2. Container Startup Signal

The ready signal is dispatched to unblock the `syncReader` **only after** the `StartContainer` API call successfully returns. This guarantees that host data is only transmitted when the container is officially "running" and ready to consume input.

```go
// internal/command/root.go

type syncReader struct {
	inner io.Reader
	ready <-chan struct{}
	ctx   context.Context
}

func (s *syncReader) Read(p []byte) (n int, err error) {
	select {
	case <-s.ctx.Done():
		return 0, s.ctx.Err()
	case <-s.ready:
		return s.inner.Read(p)
	}
}
```

### 3. StdinOnce for Reliable EOF Propagation

For Docker runtimes (`internal/runtime/docker.go`), when interactive mode is active, `StdinOnce: true` is configured on the attachment options.

Under default Docker settings (`StdinOnce: false`), the daemon may keep the container input stream open indefinitely even after the client closes its end. For non-TTY, pipe-based commands, this prevents the containerized process (e.g., `cat`) from detecting EOF, causing it to hang.

Configuring `StdinOnce: true` ensures that when `cderun` finishes streaming host STDIN and closes its write end, the Docker daemon propagates the EOF directly to the container process, allowing the command to terminate normally with exit status `0`.

### 4. CloseWrite Grace Period (Docker)

In the Docker adapter (`internal/runtime/docker.go`), once the input copier finishes copying all bytes, it sleeps for **100 milliseconds** (`attachCloseWriteGrace`) before invoking `CloseWrite()`. This short delay guarantees that small payloads are fully flushed and processed by the Docker daemon before the connection is severed.

### 5. Disable Historical Log Dumping on Attach

During `AttachContainer`, `Logs: false` is configured on the attachment settings.

Since `cderun` attaches streams *before* starting the container, there are no historical logs to dump. Enabling `Logs: true` can cause the daemon to transmit a blank log-dump sequence and prematurely terminate or disrupt the streaming socket connection on certain Docker engine versions. Setting `Logs: false` ensures a clean, real-time-only stream connection.

## Key Benefits

- **Deterministic Pipeline Execution**: Pipes and redirections function reliably even for extremely small, fast-executing payloads.
- **Zero Data Loss**: Data streaming starts only when the container is fully initialized and listening.
- **Accurate EOF Handling**: EOF signals on the host are forwarded safely, avoiding hanging pipeline commands.

## Verification

This behavior is verified via unit tests inside `internal/command/stdin_test.go`, which simulate immediate stdin streaming and artificial container startup delays.

## Hang Timeout Safe Fallback

To prevent containers from hanging indefinitely under unexpected edge cases, `cderun` implements an automatic fallback timeout mechanism for non-interactive, non-TTY sessions.

For more details, see [Hang Timeout Feature Specification](./hang-timeout.md).
