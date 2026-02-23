# Standard Input Synchronization

This document explains the synchronization mechanism for standard input (STDIN) in `cderun` and why it is necessary for reliable piped execution.

## The Problem: Race Conditions in Piped Execution

When running a command like `echo "test" | cderun ... cat`, there is a race condition between the following events:

1. **STDIN Attachment**: `cderun` calls Docker's `AttachContainer` API to connect the host's STDIN to the container.
2. **Container Start**: `cderun` calls Docker's `StartContainer` API to actually begin execution of the command (e.g., `cat`).
3. **STDIN Consumption**: The command inside the container (e.g., `cat`) starts reading from its STDIN.

If the host's STDIN is a pipe (like from `echo`), the data is available immediately. If `cderun` starts copying this data to the container's input stream *before* the container has actually started, some Docker versions or configurations might drop the data or fail to deliver it correctly to the process when it eventually starts.

Furthermore, if the data is small (like "test\n"), `cderun` might finish copying all data and call `CloseWrite()` on the connection before the containerized process has even had a chance to open its own STDIN. This often results in the process seeing an immediate EOF and exiting (or in some cases, never seeing the data at all and hanging).

## The Solution: Synchronized STDIN

To ensure reliable piped input, `cderun` implements a synchronization mechanism using a `syncReader`.

### 1. Delayed STDIN Reading

`cderun` wraps the host's STDIN in a `syncReader` before passing it to the runtime's `AttachContainer` method. This reader blocks any `Read` calls until it receives a signal.

### 2. Signal on Container Start

The signal to unblock the `syncReader` is sent only *after* the `StartContainer` API call has successfully returned. This guarantees that the container is officially "running" before any data from the host's STDIN is pushed into the container's input stream.

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

### 3. Handling Initial Output with Logs: true

In `internal/runtime/docker.go`, the `AttachContainer` call uses `Logs: true`.

While `Logs: false` was previously considered to avoid potential stream instability, it was found that `Logs: true` is necessary to ensure that no output is lost if the container starts and produces output very quickly after the attachment. Any potential instability is handled by the `attachReady` signal and proper synchronization of the container start. For more details on why `Logs: true` is maintained, see [Logging and Debugging](logging-debugging.md).

## Benefits

- **Reliability**: Piped input works consistently even for very fast-executing commands or small data sets.
- **No Data Loss**: Data is only sent when the container is ready to receive it.
- **Correct EOF Handling**: The EOF from the host's STDIN is delivered to the containerized process at the correct time.

## Verification

This behavior is verified by unit tests in `internal/command/stdin_test.go` which simulate delayed container startup with immediate STDIN availability.
