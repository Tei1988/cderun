# Feature Specification: Logging and Debugging

## Overview

`cderun` provides comprehensive logging and debugging features to inspect container setups and execution steps.
The logging system is designed to be completely thread-safe, incorporating optimizations (such as early log-level checks prior to acquiring mutexes) to keep performance overhead minimal.

## Log Levels

### Level Definitions

- `ERROR`: Captures fatal errors and command aborts.
- `WARN`: Records warnings and non-fatal errors (Default level for CLI).
- `INFO`: General informational lifecycle logs.
- `DEBUG`: Detailed operational traces (e.g. configuration files loaded, socket connections resolved).
- `TRACE`: Extreme fine-grained step-by-step logs (e.g. low-level argument processing, API payloads, internal timing metrics).

### Configuration Options

#### Command-Line Flags

```bash
# Print general logs (INFO)
cderun --log-level info node app.js

# Print detailed operational logs (DEBUG)
cderun --log-level debug node app.js

# Print complete trace logs (TRACE)
cderun --log-level trace node app.js
```

#### Configuration Profile

```yaml
# .cderun.yaml
logging:
  level: error  # error | warn | info | debug | trace
  format: text  # text | json
  timestamp: true
```

#### Environment Variables

```bash
export CDERUN_LOG_LEVEL=debug
export CDERUN_LOG_TIMESTAMP=true
```

## P1 Internal Overrides

Like other settings, you can override logging properties using the `--cderun-` prefixed Phase 1 (P1) flags placed after the subcommand in Wrapper Mode:

- `--cderun-log-level`
- `--cderun-log-format`
- `--cderun-log-timestamp`
- `--cderun-hang-timeout`

## Logging Output Examples

### Default (WARN/ERROR level)

```bash
cderun node app.js
Hello, World!
```

> **Note**: At default levels, no diagnostic messages (such as "Running: ...") are printed, ensuring a clean output consisting solely of the wrapped tool's output.

### INFO Level

```bash
cderun --log-level info node app.js
2026-02-28 10:30:45 [INFO] Running: node app.js
Hello, World!
```

### DEBUG Level

```bash
cderun --log-level debug node app.js
2026-02-28 10:30:45 [DEBUG] Loaded cderun config from: .cderun.yaml
2026-02-28 10:30:45 [DEBUG] Resolved Image: node:20-alpine
2026-02-28 10:30:45 [INFO] Running: node app.js
2026-02-28 10:30:45 [DEBUG] Image: node:20-alpine
2026-02-28 10:30:45 [DEBUG] Runtime: docker
2026-02-28 10:30:45 [DEBUG] Socket: /var/run/docker.sock
Hello, World!
2026-02-28 10:30:46 [DEBUG] Container exited with code: 0
```

#### ContainerConfig Debug Output and Sensitive Data Masking

Immediately prior to starting the container, `cderun` logs the finalized `ContainerConfig` structure (including image name, command, entrypoint, volume mounts, environment lists, and user context) at the `DEBUG` level.

To ensure strict compliance with security standards, the environment variables printed inside this dump are processed via `config.MaskSensitiveEnvList`. Any environment variables matched by the active `sensitive-env` patterns (or all environment variables by default) are printed as `[REDACTED]`, ensuring that authentication keys, database passwords, or credentials never leak into operational log files.

Diagnostic config dump example:

```text
2026-02-28 10:30:45 [DEBUG] ContainerConfig:
  Image:      node:20-alpine
  Command:    [app.js]
  Entrypoint: []
  Mounts:
    - bind /home/user/project -> /app
  Env:        [NODE_ENV=production DB_PASSWORD=[REDACTED] API_TOKEN=[REDACTED]]
  User:       1000:1000
```

### TRACE Level

```bash
cderun --log-level trace node app.js
2026-02-28 10:30:45 [TRACE] Loading configurations...
2026-02-28 10:30:45 [DEBUG] Loaded cderun config from: .cderun.yaml
2026-02-28 10:30:45 [TRACE] Resolving configurations for tool: node
2026-02-28 10:30:45 [DEBUG] Resolved Image: node:20-alpine
2026-02-28 10:30:45 [INFO] Running: node app.js
...
2026-02-28 10:30:45 [TRACE] Creating container...
2026-02-28 10:30:45 [TRACE] Starting container: <ID>
2026-02-28 10:30:45 [TRACE] Waiting for container: <ID>
...
```

## Formats

### Text Format (Default)

```text
2026-02-28 10:30:45 [INFO] Running: node app.js
```

### JSON Format

```bash
cderun --log-format json --log-level info node app.js
{"level":"info","msg":"Running: node app.js","time":"2026-02-28T10:30:45Z"}
```

## Debugging Utilities

### 1. Dry Run Mode

Previews the generated container IR configuration without launching the container on the host engine. For more details, see [Dry Run Mode Spec](./dry-run-mode.md).

```bash
cderun --dry-run node app.js
```

## Internal Architecture Notes

### Streaming Output Capture (`Logs: false`)

Inside the Docker runtime adapter (`internal/runtime/docker.go`), the standard `AttachContainer` configuration sets `Logs: false`.

This design decision stems from the following history:

1. **Bug Resolution**:
   - Eagerly calling attach with `Logs: true` on certain Docker daemon setups before the container starts can cause the engine to immediately deliver a blank EOF token and close the streaming connection, truncating subsequently started command outputs.
2. **Synchronization Guard**: `cderun` enforces a synchronous startup sequence where the connection copier is established and confirmed via the `attachReady` channel *before* `StartContainer` is invoked.
3. **Lossless Stream Capturing**: This orchestration ensures that setting `Logs: false` is completely safe and robust, catching all console stdout/stderr streams instantly upon startup without duplicate buffering or race conditions.

### Post-Execution Hang Safe-Termination (`CDERUN_HANG_TIMEOUT`)

Following the completion of stream I/O operations, certain container workloads may hang or fail to exit. `cderun` mitigates this via the following timeout policies:

- **Grace Timeout**: Controlled via the `--hang-timeout` option / `CDERUN_HANG_TIMEOUT` environment variable.
- **Handling**: Upon grace period expiration (default 10s), if the container is still executing, a SIGKILL signal is sent to force termination.
- **Diagnostics**: Detailed timeout events and SIGKILL executions are logged at the `DEBUG` level to prevent terminal clutter.

## Log Verbosity Guidelines

To provide a clean, noise-free CLI execution environment while preserving rich diagnostic metrics, `cderun` follows these verbosity practices:

### Post-Startup Operational Warnings

Operational events occurring after the container launches (such as connection closure warnings, container SIGKILL actions, or non-timeout attachment errors) are classified under the `Warn` level internally but are suppressed at the default `error` log level.

This guarantees that standard, successful command executions remain clean and uncluttered by daemon warnings, while still allowing developers to easily access diagnostic warnings by enabling `info` or `debug` log levels.
