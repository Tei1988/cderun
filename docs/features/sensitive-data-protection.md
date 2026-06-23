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
- `SIGNATURE`
- `BEARER`
- `OTP`
- `SENSITIVE`

### Intelligent Segmentation

The system performs a single-pass scan of the key string to accurately identify sensitive information while minimizing false positives and memory allocations. Segmentation occurs at:

- **Non-alphanumeric characters**: e.g., `DB_PASSWORD` → `DB`, `PASSWORD`
- **CamelCase transitions**: e.g., `apiToken` → `api`, `Token`
- **Letter-to-digit boundaries**: e.g., `accessKey2` → `accessKey`, `2`
- **Acronyms**: Handles transitions from uppercase sequences to lowercase, e.g., `JSONToken` → `JSON`, `Token`

A key like `MONKEY` is correctly identified as non-sensitive because none of its segments match the keywords. In contrast, `AWS_ACCESS_KEY_ID`, `dbPassword2`, `accessKey2`, or `SSL_CERT_FILE` will be masked as `[REDACTED]`.

## Presentation Layer Safety

Masking is applied at the presentation layer (`handleDryRun`) and logging layer, ensuring that the actual container execution still receives the plaintext values.

### Dry-Run Output

In dry-run mode, sensitive values are replaced with `[REDACTED]`. Additionally, all environment variables and command arguments are individually quoted (`%q`) in simple output format to prevent terminal injection or argument spoofing via whitespace/newlines.

### Error Message Hardening

All error messages referencing paths, tool identifiers, or user-provided input utilize quoted formatting (`%q`) to prevent log injection and ensure that malicious strings (e.g., containing newlines or control characters) cannot disrupt terminal output or log integrity.

### Secure Logging

Debug logs use quoted formatting for all resolved environment variables and configuration strings to ensure that control characters in malicious input cannot disrupt the terminal or log file structure.
