# Signal Handling Security

cderun provides secure mechanisms for signaling containers, ensuring that signal names and values are strictly validated before being passed to the container runtime.

## Signal Name Validation

All signals sent via the `SignalContainer` method (available in both Docker and Mock runtimes) are validated against a case-insensitive regular expression:
`^(?i)[A-Z0-9]+$`

This regex pattern ensures that only alphanumeric characters are accepted, preventing any shell metacharacters, whitespace, or control characters from being passed to the runtime. This effectively blocks command injection into the underlying shell or container runtime.

Note that this validation acts as a character whitelist rather than a static signal allowlist; unknown alphanumeric signals are passed to the engine, which will subsequently handle any unsupported signal errors natively.

## Runtime Isolation

Signal validation is implemented at the runtime abstraction layer. This prevents malicious subcommands or environment-based overrides from injecting arbitrary commands or metacharacters into the runtime's signal delivery path (e.g., `SIGTERM; rm -rf /`).

If a signal containing restricted characters is provided, the validation fails immediately, preventing potential injection vulnerabilities before any calls are made to the underlying engine.

## Error Handling

The runtime abstraction suppresses "not found" or "conflict" errors (e.g., if the container has already exited) to ensure that signaling is idempotent and does not cause unexpected CLI failures during cleanup or shutdown sequences.

## Early Registration and Lifecycle Synchronization

`cderun` registers standard OS signals (`SIGINT`, `SIGTERM`, `SIGHUP`, `SIGQUIT`) early in the command execution lifecycle (at the start of the execution engine) using a buffered signal channel of size 4 to prevent signal dropping and ensure robust cleanup under all termination paths.

To synchronize signal handling with the container lifecycle, `cderun` maintains a thread-safe execution state machine (`executionState`) that tracks transitions between the following states:

- **Pre-Start**: The container is being initialized or created (e.g., image pulling). Signals received during this phase immediately cancel the execution context to abort the startup cleanly and trigger deferred container/snapshot removals.
- **Startup In-Flight**: The container has been created and is in the process of starting up. To prevent orphan containers or partial initialization states, signals received during this phase are queued/deferred and safely forwarded once startup is complete, rather than immediately terminating the host process context.
- **Running**: The container is running normally. Signals are forwarded directly to the container runtime to manage terminal-like signal propagation.
