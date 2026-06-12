# Sensitive Data Protection

cderun protects sensitive information from being accidentally leaked into logs or terminal output during dry-run and diagnosis modes.

## Environment Variable Masking

Sensitive environment variables are automatically masked in non-execution contexts. The masking logic utilizes a segment-based approach to identify keywords while minimizing false positives.

### Masking Keywords

The following segments (case-insensitive) trigger masking:

- `PASSWORD`
- `SECRET`
- `TOKEN`
- `KEY`
- `AUTH`
- `SIG`
- `CERT`
- `PEM`
- `PRIVATE`
- `CREDENTIALS`
- `PASSPHRASE`
- `APIKEY`
- `SESSION`
- `ACCESS`
- `JWT`
- `SALT`

### Intelligent Segmentation

The system splits keys into segments to accurately identify sensitive information while minimizing false positives. Segmentation occurs at:

- **Non-alphanumeric characters**: e.g., `DB_PASSWORD` → `["DB", "PASSWORD"]`
- **CamelCase transitions**: e.g., `apiToken` → `["API", "TOKEN"]`
- **Letter-to-digit boundaries**: e.g., `accessKey2` → `["ACCESS", "KEY", "2"]`
- **Acronyms**: Handles transitions from uppercase sequences to lowercase, e.g., `JSONToken` → `["JSON", "TOKEN"]`

A key like `MONKEY` is correctly identified as non-sensitive because none of its segments match the keywords. In contrast, `AWS_ACCESS_KEY_ID`, `dbPassword2`, or `SSL_CERT_FILE` will be masked as `[REDACTED]`.

### False Positives and Limitations

The keyword-based approach can occasionally lead to false positives where non-sensitive information is masked.

- **`ACCESS` keyword**: This keyword is included primarily to catch `AWS_ACCESS_KEY_ID`. However, it also causes variables like `ACCESS_LOG` or `LOG_ACCESS_LEVEL` to be redacted, which can hinder debugging in dry-run mode.
- **`KEY` keyword**: Similar to `ACCESS`, generic keys like `CACHE_KEY` or `CONFIG_KEY_PATH` may be masked.

Future improvements (see [TODO T08](../../.agent/todo.md)) plan to move away from automatic keyword detection towards an explicit configuration-based masking system (e.g., a `sensitiveEnv` list in config files) to provide users with more control.

## Presentation Layer Safety

Masking is applied at the presentation layer (`handleDryRun`) and logging layer, ensuring that the actual container execution still receives the plaintext values.

### Dry-Run Output

In dry-run mode, sensitive values are replaced with `[REDACTED]`. Additionally, all environment variables and command arguments are individually quoted (`%q`) in simple output format to prevent terminal injection or argument spoofing via whitespace/newlines.

### Error Message Hardening

All error messages referencing paths, tool identifiers, or user-provided input utilize quoted formatting (`%q`) to prevent log injection and ensure that malicious strings (e.g., containing newlines or control characters) cannot disrupt terminal output or log integrity.

### Secure Logging

Debug logs use quoted formatting for all resolved environment variables and configuration strings to ensure that control characters in malicious input cannot disrupt the terminal or log file structure. Masking is also applied to debug logs when environment variables are resolved, ensuring that secrets do not appear in trace output even when log level is set to `debug` or `trace`.
