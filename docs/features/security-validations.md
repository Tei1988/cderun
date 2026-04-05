# Security Validations

cderun implements several validation layers to prevent command injection, path traversal, and log injection attacks.

## Character Validation

The `validatePathChars` function (referred to as `ValidateSafeString` in some contexts) ensures that critical configuration strings do not contain ASCII control characters (ASCII < 32 or 127) or null bytes.

This validation is applied to:
- Image names
- User names
- Network modes
- Hostnames
- Working directories
- Entrypoint elements

Any string containing unsafe characters will cause an immediate resolution failure.

## Tool Name Safety

The `ValidateToolName` function enforces strict naming conventions for tool identifiers. Tool names are restricted to a whitelist of safe characters:
- Alphanumerics (`a-z`, `A-Z`, `0-9`)
- Dots (`.`)
- Underscores (`_`)
- Hyphens (`-`)

This prevents tool names from being used for path traversal (e.g., `../../etc/shadow`) or containing shell-sensitive characters (e.g., `| `, `;`, `:`). Tool name validation is performed before any logging or filesystem operations.

## Signal Validation

When signaling containers via `SignalContainer`, signal names are validated against a strict regular expression:
`^(?i)(SIG[A-Z0-9]+|[A-Z0-9]+|[0-9]+)$`

This allows:
- Standard signal names with or without the SIG prefix (e.g., `TERM`, `SIGKILL`)
- Numeric signal values (e.g., `9`, `15`)

Invalid signals are rejected to prevent command injection into the underlying runtime.

## Absolute Mount Targets

All mount configurations must specify an absolute path for the `Target` (container-side path). Relative paths in mount targets are ambiguous and potentially dangerous, so they are rejected during resolution.
