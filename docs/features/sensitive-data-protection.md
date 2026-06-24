# Sensitive Data Protection

cderun protects sensitive information from being accidentally leaked into logs or terminal output during dry-run and diagnosis modes.

## Environment Variable Masking

Environment variable values are masked in non-execution contexts (dry-run and debug logs) to prevent credential leakage. cderun supports three masking modes controlled by the `sensitive-env` configuration.

### Configuration (`sensitive-env`)

The masking behavior is controlled by the `sensitive-env` option, which can be defined in configuration files, environment variables, or CLI flags.

- **Unset (Default)**: If `sensitive-env` is not specified (nil), cderun uses its **automatic keyword-based masking**. It scans environment variable keys for segments like `PASSWORD`, `SECRET`, `TOKEN`, `KEY`, etc., and masks their values.
- **Empty List**: If an explicit empty list is provided (e.g., `--sensitive-env=""` in CLI or `sensitiveEnv: []` in YAML), environment variable masking is **disabled**. This is useful for troubleshooting when you want to see all values.
- **Explicit List of Patterns**: If `sensitive-env` is provided as a non-empty list, only keys matching the specified glob patterns are masked. Automatic keyword-based masking is disabled in this mode.

### Automatic Masking Keywords

In the default mode (Unset), the following segments (case-insensitive) trigger masking:

- `PASSWORD`, `SECRET`, `TOKEN`, `KEY`, `AUTH`, `SIG`, `CERT`, `PEM`, `PRIVATE`, `CREDENTIALS`, `PASSPHRASE`, `APIKEY`, `SESSION`, `ACCESS`, `JWT`, `SALT`, `SIGNATURE`, `BEARER`, `OTP`, `SENSITIVE`.

### Pattern Matching (Explicit List)

Patterns support the `*` wildcard (glob) to match multiple keys. Matching is case-insensitive.

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
