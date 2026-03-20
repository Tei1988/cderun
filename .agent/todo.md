# Remaining Issues and Refactoring Tasks

The following issues were identified during the DI refactoring and test organization process but have not been fixed yet to allow for an immediate PR submission as requested.

## High Priority: Data Races
- **internal/command/stdin_test.go**: Several tests exhibit data races due to concurrent access to `bytes.Buffer` in the `pipeMockRuntime.AttachContainer` method and the main test goroutine.
  - *Recommended Fix*: Wrap `bytes.Buffer` with a mutex (e.g., a `safeBuffer` struct) or use `io.Pipe` for synchronized data transfer between the mock runtime and the test assertions.

## Regressions
- **Mount Tools Warning**: The warning `[WARN] --mount-all-tools specified but no tools defined in .tools.yaml` is currently missing in the refactored `internal/command/root.go`.
  - *Recommended Fix*: Re-implement the check in `buildContainerConfig` or `RunE` to detect when `MountAllTools` is enabled but `toolsCfg` is empty, and log the warning via the injected logger.

## Test Stability
- **Concurrent Execution Timing**: Some tests in `internal/command/root_test.go` rely on `assert.Eventually` because of the asynchronous nature of container termination (Wait/Remove).
  - *Recommended Fix*: Further refine the `MockRuntime` state transitions or use more robust synchronization primitives to avoid non-deterministic test results.

## Regression Details (Observed during DI Refactoring)
- **Flag Mapping in root.go**: Some flags (e.g., `--workdir`, `--image`, `--pull`) are not correctly mapped to `CLIOptions` in `resolveSettings`, or their "Set" flags are missing, causing them to be ignored during resolution.
- **Polyglot Mode Flag Hoisting**: There is a discrepancy between the expected and actual image resolution in polyglot mode when using internal overrides.
- **Dry-run Output Mismatch**: The simple output format in dry-run mode sometimes produces empty results in tests due to how output streams are captured or how the configuration is built.
- **Tool Mapping in Tests**: Many unit tests fail with "no image mapping found" because they lack the necessary `.tools.yaml` setup in the injected `MockFileSystem`.
