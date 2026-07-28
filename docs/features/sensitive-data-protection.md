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

To provide developers with complete visibility into the exact container payload being dispatched, `cderun` logs the entire built `ContainerConfig` at the `DEBUG` log level immediately prior to invoking the runtime-specific container creation API (e.g., Docker or containerd).

Since this struct contains all environment variables merged from various priority layers (P1 to P6), logging it directly would bypass standard presentation-layer masking and leak secrets into the terminal or log files. To resolve this, `cderun` intercepts the configuration and passes the env slice through `config.MaskSensitiveEnvList` before printing.

- **Operation**: The logging helper `logContainerConfig` format-masks the environment list, replacing any sensitive values that match the active masking filters (including the default "Mask-All" state) with `[REDACTED]`.
- **Significance**: This ensures that even under maximum verbosity (`--log-level debug` or `trace`), credentials like database passwords, API tokens, and TLS keys are guaranteed to remain invisible in the logs while the underlying runtime still receives the raw plaintext secrets.
