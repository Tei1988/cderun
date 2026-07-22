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
