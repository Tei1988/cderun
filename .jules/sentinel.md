# Sentinel Journal - Security Learnings

## 2026-02-12 - Insecure Temporary Files and Path Traversal in Expressions

**Vulnerability:**
1. Temporary snapshot directories and files were created with world-readable permissions (0755/0644), potentially exposing sensitive configuration data (including resolved secrets) to other users on the host.
2. The `{{ file:... }}` expression resolver allowed arbitrary host file reads via path traversal (e.g., `{{ file:../../etc/passwd }}`).

**Learning:**
- Undocumented features like `{{ file:... }}` can introduce significant security gaps if not properly audited and restricted.
- Standard library defaults for file creation (often `0644`) or directory creation (`0755`) are frequently too permissive for sensitive internal tool data.

**Prevention:**
- Always use the most restrictive permissions possible for temporary data (`0700` for dirs, `0600` for files).
- Validate all user-supplied input used in filesystem operations, especially when using functions that search hierarchies, to prevent escaping the intended scope.
