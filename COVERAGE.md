# Test Coverage Report

## Summary

As of February 2026, the `cderun` project maintains a high level of test coverage across all major packages.

| Package | Coverage | Status |
| :--- | :--- | :--- |
| `internal/command` | 93.0% | Excellent |
| `internal/config` | 91.5% | Excellent |
| `internal/logging` | 96.1% | Excellent |
| `internal/runtime` | 96.0% | Excellent |
| **Total** | **92.8%** | **Excellent** |

## Maintenance Strategy

To maintain and improve the quality of the codebase, the following strategy is implemented:

1. **Naming Conventions**: All tests must follow the `Test[Category]_[Package]_[Feature]_[Scenario]` format.
   - Example: `TestUnit_Config_Resolver_Priority`
   - Example: `TestIntegration_Docker_PortMapping`
2. **Category Definitions**:
   - **Unit**: Logic tests without external dependencies.
   - **Integration**: Tests involving multiple components or mocks of external systems.
   - **Robustness**: Tests for signals, race conditions, and timeouts.
   - **Scenario (E2E)**: Complex workflows and real-environment verification.
3. **Continuous Measurement**:
   - Coverage is automatically measured in CI for every pull request and push.
   - Minimum threshold of 86.5% is enforced.
4. **Refactoring for Testability**:
   - Prefer interfaces for external dependencies (FileSystem, ContainerRuntime).
   - Use dependency injection to allow mocking in tests.
   - Avoid global state where possible; use `t.Cleanup` for restoration when global state mutation is unavoidable.

## Recent Improvements (2026-02)

- Improved `internal/runtime` coverage (93.4% -> 96.0%) by testing suppressed errors and default behaviors.
- Improved `internal/command` coverage (92.5% -> 93.0%) by expanding snapshot and signal tests.
- Improved `internal/config` coverage (90.3% -> 91.5%) by adding `SetBaseDir` validation for all configuration structs.
- Added complex multi-tool nested execution scenario test.
- Cleaned up unused `init()` functions to reduce noise in coverage reports.
