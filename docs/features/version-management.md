# Feature Specification: Version Management

`cderun` performs dynamic version management utilizing Git metadata resolved during compilation.

## Overview

The compiled executable dynamically embeds the Git tag, commit SHA (revision), and build timestamp. This ensures that users and developers can accurately identify the precise binary version when debugging or reporting issues.

## Features

- **Automated Metadata Injection**: Build details are automatically compiled into the binary using linker flags managed by `Makefile` or `GoReleaser`.
- **Comprehensive Output**: The `--version` flag prints detailed compile-time metrics, including Git revision and build date.
- **Development Fallbacks**: Direct invocations via `go run` or standard `go build` (without linker overrides) automatically fall back to `dev` for the version and `unknown` for the revision, indicating a non-release local build.

## Specifications

### Compiled Version Details

| Field | Description | Example |
| :--- | :--- | :--- |
| Version | Git release tag or `dev` | `0.0.2`, `v1.1.0-dirty` |
| Revision | Shortened Git commit SHA | `abc1234` |
| BuildDate | ISO 8601 formatted compilation timestamp | `2026-03-02T12:34:56Z` |
| OS/Arch | Operating system and CPU architecture | `linux/amd64`, `darwin/arm64` |

### Standard Output Format

Invoking `cderun --version` outputs the version string in the following format:

```text
cderun version 0.0.2 (rev: abc1234, built at: 2026-03-02T12:34:56Z, linux/amd64)
```

## Compilation Mechanism

### 1. `internal/version` Package

The version information is encapsulated inside a dedicated, isolated package `internal/version`. It retains no dependencies on business or command-execution logic, serving solely to store and format compilation metadata (via the `Info()` helper).

### 2. Injection via Linker Flags (`ldflags`)

Linker flags (`-ldflags`) are supplied during compilation to dynamically override the default package-level string variables in `internal/version`.

#### Makefile Example

```makefile
LDFLAGS := -X cderun/internal/version.Version=$(VERSION) \
           -X cderun/internal/version.Revision=$(REVISION) \
           -X cderun/internal/version.BuildDate=$(BUILD_DATE)

go build -ldflags "$(LDFLAGS)" -o cderun main.go
```

### 3. GoReleaser Integration

During automated CI release compilation, GoReleaser overrides these fields using `.goreleaser.yaml` templates:

```yaml
builds:
  - ldflags:
      - -X cderun/internal/version.Version={{.Version}}
      - -X cderun/internal/version.Revision={{.FullCommit}}
      - -X cderun/internal/version.BuildDate={{.Date}}
```

## Related Documents

- [Architecture Reference: Versioning](../architecture/versioning.md)
