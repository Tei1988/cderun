# Test Coverage Report

## Summary

Date: 2026-02
Total Coverage: **91.3%**

## Package Coverage

| Package | Coverage |
| :--- | :---: |
| `internal/command` | 90.1% |
| `internal/config` | 90.3% |
| `internal/logging` | 96.1% |
| `internal/runtime` | 92.4% |
| `internal/container` | [no statements] |
| **Total** | **91.3%** |

## Key Findings

- **Core Logic**: `internal/command/root.go` maintains high coverage (above 90% for most core methods) even after refactoring for better testability.
- **Configuration**: Path and expression resolution logic is well-tested, reaching over 90% in most areas.
- **Runtime Implementation**: Docker and Podman runtime abstractions are verified through both unit tests and integration tests.
- **High Quality**: The overall coverage remains consistently above 90%, ensuring high maintainability and reliability.

## Uncovered Areas

- `Execute` and `ExecuteContext` in `internal/command/root.go` report 0% as they are wrappers that call `ExecuteContextWithOptions`, which is used in tests. In a real application, these are the entry points.
- Some edge cases in error handling across all packages.
- Platform-specific logic (e.g., Windows vs Unix signals) where tests run in a single environment.
