# Sensitive Data Protection

`cderun` protects sensitive information from being accidentally leaked into logs or terminal output during dry-run and diagnosis modes.

## Environment Variable Masking

Environment variable values are masked in non-execution contexts (dry-run and debug logs) to prevent credential leakage. `cderun` follows a "Secure by Default" approach where all environment variables are considered sensitive unless an explicit configuration is provided.

### Configuration (`sensitive-env`)

The masking behavior is controlled by the `sensitive-env` option, which can be defined in configuration files, environment variables, or CLI flags.

- **Unset (Default)**: If `sensitive-env` is not specified (nil), **all** environment variable values are masked as `[REDACTED]`. This "Secure by Default" approach ensures maximum safety for users who have not yet configured their sensitive environment variables.
- **Empty List**: If an explicit empty list is provided (e.g., `--sensitive-env=""` in CLI or `sensitiveEnv: []` in YAML), environment variable masking is **disabled**. This is useful for troubleshooting when you want to see all values.
- **Explicit List of Patterns**: If `sensitive-env` is provided as a non-empty list, only keys matching the specified glob patterns are masked. All other variables will be displayed in plaintext.
- **Empty Values Preservation**: Note that environment variable masking under both the default (Secure by Default) and fallback (Fail-Closed) modes will only mask non-empty environment values as `[REDACTED]`. Empty environment values (e.g., `KEY=`) remain unchanged and are preserved as-is without modification.

### Pattern Matching (Explicit List)

Patterns support the `*` wildcard (glob) to match multiple keys. Matching is case-insensitive.

To maximize performance on key execution paths, `cderun` uses custom, case-insensitive helper functions: `equalFoldASCII`, `hasSuffixFoldASCII`, `hasPrefixFoldASCII`, and `containsFoldASCII`. These ASCII comparison helpers avoid per-comparison allocations, although pattern analysis and fallback matching may allocate. They support:

- Exact matches (e.g., `MY_API_KEY`)
- Suffix wildcards (e.g., `DB_*`)
- Prefix wildcards (e.g., `*_PASSWORD`)
- Substring wildcards (e.g., `*SECRET*`)

#### Fail-Closed Logic

`cderun` implements fail-closed logic for pattern matching. While configuration validation rejects invalid patterns at startup, `matchPreAnalyzed` treats any runtime `path.Match` errors as positive matches that mask the value to prevent accidental exposure of potentially sensitive information.

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
