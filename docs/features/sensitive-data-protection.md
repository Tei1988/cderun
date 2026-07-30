# Sensitive Data Protection

cderun protects sensitive information from being accidentally leaked into logs or terminal output during dry-run and diagnosis modes.

## Environment Variable Masking

Environment variable values are masked in non-execution contexts (dry-run and debug logs) to prevent credential leakage. cderun follows a "Secure by Default" approach where all environment variables are considered sensitive unless an explicit configuration is provided.

### Configuration (`sensitive-env`)

The masking behavior is controlled by the `sensitive-env` option, which can be defined in configuration files, environment variables, or CLI flags.

- **Unset (Default)**: If `sensitive-env` is not specified (nil), **all** environment variable values are masked as `[REDACTED]`. This "Secure by Default" approach ensures maximum safety for users who have not yet configured their sensitive environment variables.
- **Empty List**: If an explicit empty list is provided (e.g., `--sensitive-env=""` in CLI or `sensitiveEnv: []` in YAML), environment variable masking is **disabled**. This is useful for troubleshooting when you want to see all values.
- **Explicit List of Patterns**: If `sensitive-env` is provided as a non-empty list, only keys matching the specified glob patterns are masked. All other variables will be displayed in plaintext.
- **Empty Values Preservation**: Note that environment variable masking under both the default (Secure by Default) and fallback (Fail-Closed) modes will only mask non-empty environment values as `[REDACTED]`. Empty environment values (e.g., `KEY=`) remain unchanged and are preserved as-is without modification.

### Pattern Matching (Explicit List)

Patterns support the `*` wildcard (glob) to match multiple keys. Matching is case-insensitive.

#### Fail-Closed Logic

cderun implements fail-closed logic for pattern matching. If a glob pattern is malformed (e.g., `[` without a closing bracket), `path.Match` will return an error. In this case, cderun redacts the value to prevent accidental exposure of potentially sensitive information.

```yaml
defaults:
  sensitiveEnv:
    - "MY_API_KEY"
    - "DB_*"
    - "*_PASSWORD"
```

## Presentation Layer Safety

Masking is applied at the presentation layer (`handleDryRun`) and logging layer, ensuring that the actual container execution still receives the plaintext values.

### Dry-Run Output

In dry-run mode, sensitive values are replaced with `[REDACTED]`. Additionally, all environment variables and command arguments are individually quoted (`%q`) in simple output format to prevent terminal injection or argument spoofing via whitespace/newlines.

### Error Message Hardening

All error messages referencing paths, tool identifiers, or user-provided input utilize quoted formatting (`%q`) to prevent log injection and ensure that malicious strings (e.g., containing newlines or control characters) cannot disrupt terminal output or log integrity.

### Secure Logging

Debug logs use quoted formatting for all resolved environment variables and configuration strings to ensure that control characters in malicious input cannot disrupt the terminal or log file structure. Masking is also applied to debug logs when environment variables are resolved.

### Pre-Creation Config Inspection (T80)

To provide developers with visibility into the container payload being dispatched, `cderun` logs selected fields of the `ContainerConfig` structure at the `DEBUG` log level prior to runtime initialization and image pulling (rather than immediately before container creation).

Since this includes environment variables merged from various priority layers (P1 to P6), logging them in plaintext could bypass presentation-layer protection and leak secrets. To resolve this, `cderun` intercepts the list and passes it through `config.MaskSensitiveEnvList` before formatting.

- **Operation**: The logging helper `logContainerConfig` formats and logs only selected fields (such as Image, Command, Entrypoint, Mounts, Env, and User) rather than the entire configuration struct. In doing so, it processes the environment list using `config.MaskSensitiveEnvList`, replacing any sensitive values that match the active masking filters (including the default "Mask-All" state) with `[REDACTED]`.
- **Significance**: When masking is active, this guarantees that sensitive environment variable values within the environment list (`cc.Env`) are redacted in the logs under the `DEBUG` log level, while the runtime still receives the raw plaintext values. This protection applies specifically to environment variables within `cc.Env` and does not cover other separately logged fields (such as command arguments or entrypoints) or configurations where masking has been explicitly disabled. (Note: This redaction is guaranteed specifically for `DEBUG` logging, as the helper method logs using `o.logger.Debug`).
