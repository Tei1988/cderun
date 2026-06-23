# Sensitive Data Protection

cderun protects sensitive information from being accidentally leaked into logs or terminal output during dry-run and diagnosis modes.

## Environment Variable Masking

Environment variable values are masked in non-execution contexts (dry-run and debug logs) to prevent credential leakage. cderun follows a "Secure by Default" approach where all environment variables are considered sensitive unless an explicit configuration is provided.

### Configuration (`sensitive-env`)

The masking behavior is controlled by the `sensitive-env` option, which can be defined in configuration files, environment variables, or CLI flags.

- **Unset (Default)**: If `sensitive-env` is not specified, **all** environment variable values are masked as `[REDACTED]`. This ensures maximum safety.
- **Explicit List**: If `sensitive-env` is provided as a list, only keys matching the specified patterns are masked. All other environment variables will be displayed in plaintext.
- **Empty List**: If an empty list is explicitly provided (e.g., `--sensitive-env=""` in CLI or `sensitiveEnv: []` in YAML), **no** environment variables will be masked.

### Pattern Matching

Patterns support the `*` wildcard (glob) to match multiple keys. Matching is case-insensitive.

```yaml
defaults:
  sensitiveEnv:
    - "MY_API_KEY"
    - "DB_*"
    - "*_PASSWORD"
```

### Transition from Keyword-based Masking

Previous versions of cderun used a hardcoded list of keywords (like `PASSWORD`, `SECRET`, `ACCESS`) for automatic masking. This approach was retired because it produced frequent false positives (e.g., masking `ACCESS_LOG`). Users who relied on this behavior should now explicitly list their sensitive patterns.

## Presentation Layer Safety

Masking is applied at the presentation layer (`handleDryRun`) and logging layer, ensuring that the actual container execution still receives the plaintext values.

### Dry-Run Output

In dry-run mode, sensitive values are replaced with `[REDACTED]`. Additionally, all environment variables and command arguments are individually quoted (`%q`) in simple output format to prevent terminal injection or argument spoofing via whitespace/newlines.

### Error Message Hardening

All error messages referencing paths, tool identifiers, or user-provided input utilize quoted formatting (`%q`) to prevent log injection and ensure that malicious strings (e.g., containing newlines or control characters) cannot disrupt terminal output or log integrity.

### Secure Logging

Debug logs use quoted formatting for all resolved environment variables and configuration strings to ensure that control characters in malicious input cannot disrupt the terminal or log file structure. Masking is also applied to debug logs when environment variables are resolved.
