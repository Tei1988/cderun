# Security Validations

cderun implements several validation layers to prevent command injection, path traversal, and log injection attacks.

## Character Validation

The `validatePathChars` function ensures that critical configuration strings do not contain ASCII control characters (ASCII < 32 or 127).

This validation is applied to:

- **Scalar fields**: `image`, `user`, `network`, `hostname`, `workdir`, `runtime`, `socket-path`, `mount-socket-path`, `mount-cderun-path`, `dry-run-format`, `diagnosis-format`, `log-level`, `log-format`.
- **List elements**: `entrypoint`, `ports`, `expose`, `dns`, `add-hosts`, `cap-add`, `cap-drop`.
- **Environment variables**: The **key** portion of each environment variable.
- **Mounts and Devices**: Both source and target/destination paths.

Any string containing unsafe characters will cause an immediate resolution failure.

## Tool Name Safety

The `ValidateToolName` function enforces strict naming conventions for tool identifiers. Tool names are restricted to a whitelist of safe characters:

- Alphanumerics (`a-z`, `A-Z`, `0-9`)
- Dots (`.`)
- Underscores (`_`)
- Hyphens (`-`)

This prevents tool names from being used for path traversal (e.g., `../../etc/shadow`) or containing shell-sensitive characters (e.g., `|`, `;`, `:`). Tool name validation is performed before any logging or filesystem operations.

## Signal Validation

When signaling containers via `SignalContainer`, signal names are validated against a strict regular expression:
`^(?i)(SIG[A-Z0-9]+|[A-Z0-9]+|[0-9]+)$`

This allows:

- Standard signal names with or without the SIG prefix (e.g., `TERM`, `SIGKILL`)
- Numeric signal values (e.g., `9`, `15`)

Invalid signals are rejected to prevent command injection into the underlying runtime.

## Registry Mismatch Validation

To prevent accidental use of incorrect or unauthorized registries, `cderun` validates that the container image registry provided via CLI (or environment variables) matches the registry specified in the tool's configuration (`.tools.yaml`).

If a mismatch is detected (e.g., CLI specifies `private-reg.com/node` while configuration expects `docker.io/library/node`), `cderun` returns a `RegistryMismatchError`. This error includes both the expected and actual registries to assist in troubleshooting.

## Absolute Mount Targets

All mount configurations must specify an absolute path for the `Target` (container-side path). Relative paths in mount targets are ambiguous and potentially dangerous, so they are rejected during resolution.

## Environment Variable Validation

When validating environment variables, `cderun` applies character checks strictly to the **key** portion (the part before the first `=`). This prevents the use of control characters in variable names while allowing legitimate multiline values or complex strings (such as PEM certificates) in the values themselves.
