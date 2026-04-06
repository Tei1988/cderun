# Signal Handling Security

cderun provides secure mechanisms for signaling containers, ensuring that signal names and values are strictly validated before being passed to the container runtime.

## Signal Name Validation

All signals sent via the `SignalContainer` method (available in both Docker and Mock runtimes) are validated against a case-insensitive regular expression:
`^(?i)(SIG[A-Z0-9]+|[A-Z0-9]+|[0-9]+)$`

This regex pattern ensures that:

- Standard signal names (e.g., `TERM`, `KILL`, `HUP`) are accepted.
- Signal names with the `SIG` prefix (e.g., `SIGTERM`, `SIGINT`) are accepted.
- Numeric signal values (e.g., `9`, `15`) are accepted.
- Any string containing shell metacharacters, whitespace, or control characters is rejected.

## Runtime Isolation

Signal validation is implemented at the runtime abstraction layer. This prevents malicious subcommands or environment-based overrides from injecting arbitrary commands into the runtime's signal delivery path (e.g., `SIGTERM; rm -rf /`).

If an invalid signal is provided, the runtime returns an error immediately without making any calls to the underlying engine (e.g., the Docker API).

## Error Handling

The runtime abstraction suppresses "not found" or "conflict" errors (e.g., if the container has already exited) to ensure that signaling is idempotent and does not cause unexpected CLI failures during cleanup or shutdown sequences.
