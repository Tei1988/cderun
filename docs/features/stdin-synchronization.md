# Feature Specification: Standard Input Synchronization

This document explains the standard input (STDIN) synchronization mechanism implemented in `cderun` to ensure reliable pipe-based and interactive command execution.

## The Challenge: Race Conditions in Pipe Executions

When running shell pipes (e.g., `echo "test" | cderun -i ... cat`), a severe race condition can occur between the following operations:

1. **STDIN Attachment**: `cderun` invokes the container runtime's attachment API to stream host STDIN to the container.
2. **Container Start**: `cderun` invokes the container runtime's start API to boot the target containerized command (e.g., `cat`).
3. **STDIN Consumption**: The containerized command opens and starts reading from its internal STDIN.

If the host STDIN data is available instantly (as with `echo` or files), and `cderun` eagerly streams the entire input stream *before* the containerized process officially shifts into a running state, the data may be dropped or fail to be processed by the container daemon.

Furthermore, if the data is small (such as a single word or character), `cderun` might read the EOF from the host pipe, transmit the data, and immediately invoke `CloseWrite()` on the stream before the container process even registers. This can cause the containerized process to exit with an immediate EOF without ever reading the transmitted data.

## Solution: Synchronized STDIN

To ensure deterministic, reliable pipe and interactive execution, `cderun` incorporates a synchronized STDIN mechanism utilizing a custom `syncReader`.

### 1. Deferred STDIN Streaming

When interactive mode is requested (`cc.Interactive = true`), the host's STDIN stream is wrapped with a custom `syncReader` before being passed to the runtime adapter's `AttachContainer` method:

```go
// internal/command/root.go

var stdin io.Reader
startSignal := make(chan struct{})
if cc.Interactive {
    stdin = &syncReader{
        inner: cmd.InOrStdin(),
        ready: startSignal,
        ctx:   ctx,
    }
}
```

This reader blocks any incoming `Read` operations until a ready signal (`startSignal`) is dispatched:

```go
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

### 2. Container Startup Signal

The ready signal is dispatched to unblock the `syncReader` by closing `att.startSignal` (`close(att.startSignal)`) **only after** the `StartContainer` API call successfully returns:

```go
// internal/command/root.go
if err := state.GetRuntime().StartContainer(ctxG, state.GetContainerID()); err != nil {
    return 0, &ExitCodeError{Code: 125, Err: fmt.Errorf("failed to start container: %w", err)}
}
close(att.startSignal) // Signal stdin to start reading
```

This guarantees that host data transmission begins only after container startup completes, rather than implying the workload process has begun reading STDIN.

### 3. Docker Container Configuration: `StdinOnce` and `OpenStdin`

For Docker runtimes (`internal/runtime/docker_adapter.go`), container creation sets both `OpenStdin` and `StdinOnce` to match the interactive configuration:

```go
// internal/runtime/docker_adapter.go
containerConfig := &dockercontainer.Config{
    ...
    StdinOnce: config.Interactive,
    OpenStdin: config.Interactive,
    ...
}
```

- **`OpenStdin: true`**: Keeps the standard input descriptor open for the container process.
- **`StdinOnce: true`**: In Docker, closing the client connection triggers container STDIN closure upon attach EOF only when TTY is disabled (`TTY: false`). When a TTY is allocated (`-t` / `--tty`), EOF on the attachment connection does not close container STDIN, and the container process may continue running. In non-TTY sessions (such as `echo "data" | cderun -i --image alpine --entrypoint cat cat`), configuring `StdinOnce: true` ensures that when `cderun` finishes streaming host STDIN and closes its write end, Docker propagates the EOF to the container process, allowing it to exit with status `0`. Under default Docker settings (`StdinOnce: false`), the daemon keeps the input descriptor open indefinitely even in non-TTY mode, causing non-interactive commands to hang.

### 4. CloseWrite Grace Period (Docker)

In the Docker adapter (`internal/runtime/docker.go`), once `io.Copy(resp.Conn, stdin)` finishes copying all input bytes, it sleeps for a configurable grace period defaulting to **100 milliseconds** (`attachCloseWriteGrace`) before calling `resp.CloseWrite()`:

```go
// internal/runtime/docker.go
d.logger.Debug("STDIN copy to container %s finished: %d bytes", containerID, n)
if err := d.sleepFunc(ctx, d.attachCloseWriteGrace); err == nil {
    d.logger.Trace("Calling CloseWrite on container %s connection", containerID)
    if err := resp.CloseWrite(); err != nil {
        d.logger.Debug("STDIN CloseWrite to container %s failed: %v", containerID, err)
    }
}
```

This short delay serves as a mitigation to reduce the risk of small payloads being truncated before the Docker daemon processes them, rather than a guarantee of daemon completion.

### 5. Disable Historical Log Dumping on Attach

During `AttachContainer`, `Logs: false` is configured on the attachment settings.

Since `cderun` attaches streams *before* starting the container, there are no historical logs to dump. Enabling `Logs: true` can cause the daemon to transmit a blank log-dump sequence and prematurely terminate or disrupt the streaming socket connection on certain Docker engine versions. Setting `Logs: false` ensures a clean, real-time-only stream connection.

## Key Benefits

- **Deterministic Pipeline Execution**: Pipes and redirections function reliably even for small, fast-executing payloads.
- **Controlled Stream Initiation**: Data streaming starts only when container startup completes.
- **Accurate Non-TTY EOF Handling**: EOF signals on the host are forwarded safely in non-TTY modes, avoiding hanging pipeline commands.

## Hang Timeout Safe Fallback

To prevent containers from hanging indefinitely under unexpected edge cases, `cderun` implements an automatic fallback timeout mechanism for non-interactive, non-TTY sessions.

For more details, see [Hang Timeout Feature Specification](./hang-timeout.md).
