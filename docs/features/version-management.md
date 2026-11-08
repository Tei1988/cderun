# Feature Specification: Version Management

`cderun` performs dynamic version management utilizing Git metadata resolved during compilation.

## Overview

The compiled executable dynamically embeds the Git release tag, commit SHA (revision), and build timestamp. This ensures that users, developers, and automated tools can accurately identify the precise binary version when debugging, reporting issues, or auditing deployments.

## Features

- **Automated Metadata Injection**: Build details are automatically compiled into the binary using Go linker flags (`-ldflags`) managed by `Makefile` or `GoReleaser`.
- **Comprehensive Output**: The `--version` flag prints detailed compile-time metrics, including Git revision, build timestamp, and runtime operating system / architecture (`runtime.GOOS/runtime.GOARCH`).
- **Clean Command Termination**: Requesting `--version` prints metadata to standard output and terminates cleanly with exit code `0` without requiring or validating subcommand arguments or loading configuration files.
- **Development Fallbacks**: Direct invocations via `go run` or standard `go build` (without linker overrides) automatically fall back to `"dev"` for the version and `"unknown"` for the revision and build date, indicating a non-release local build.

---

## Specifications

### Compiled Version Metadata Fields

| Field | Go Variable | Description | Release Example | Fallback Value |
| :--- | :--- | :--- | :--- | :--- |
| **Version** | `version.Version` | Git release tag or dirty status | `0.0.2`, `v1.1.0-dirty` | `"dev"` |
| **Revision** | `version.Revision` | Full or shortened Git commit SHA | `abc1234` | `"unknown"` |
| **BuildDate** | `version.BuildDate` | ISO 8601 UTC compilation timestamp | `2026-03-02T12:34:56Z` | `"unknown"` |
| **OS/Arch** | Evaluated dynamically | Operating system and CPU architecture | `linux/amd64`, `darwin/arm64` | `runtime.GOOS/runtime.GOARCH` |

### Standard Output Format

Invoking `cderun --version` outputs the version string in the following standardized format:

```text
cderun version 0.0.2 (rev: abc1234, built at: 2026-03-02T12:34:56Z, linux/amd64)
```

In an un-injected local development build, invoking `--version` outputs:

```text
cderun version dev (rev: unknown, built at: unknown, linux/amd64)
```

---

## Compilation & Injection Mechanism

### 1. `internal/version` Package

The version information is encapsulated inside a dedicated, isolated package `internal/version`:

```go
package version

import (
    "fmt"
    "runtime"
)

var (
    // Version is the current version of cderun.
    Version = "dev"
    // Revision is the git commit SHA.
    Revision = "unknown"
    // BuildDate is the date when the binary was built.
    BuildDate = "unknown"
)

// Info returns a formatted string containing version information.
func Info() string {
    return fmt.Sprintf("cderun version %s (rev: %s, built at: %s, %s/%s)", Version, Revision, BuildDate, runtime.GOOS, runtime.GOARCH)
}
```

It retains no dependencies on business, CLI, or container execution logic, serving solely to store compilation metadata and format output strings via `version.Info()`.

### 2. Injection via Linker Flags (`-ldflags`)

Linker flags (`-ldflags`) are supplied during compilation to dynamically override package-level string variables in `cderun/internal/version`.

#### Makefile Integration

The root `Makefile` dynamically discovers the current Git tag, commit SHA, and ISO 8601 build date:

```makefile
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
REVISION ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS := -X cderun/internal/version.Version=$(VERSION) \
           -X cderun/internal/version.Revision=$(REVISION) \
           -X cderun/internal/version.BuildDate=$(BUILD_DATE)

go build -ldflags "$(LDFLAGS)" -o cderun main.go
```

### 3. GoReleaser CI Release Integration

During automated CI release workflows, GoReleaser overrides these fields using `.goreleaser.yaml` release templates:

```yaml
builds:
  - ldflags:
      - -X cderun/internal/version.Version={{.Version}}
      - -X cderun/internal/version.Revision={{.FullCommit}}
      - -X cderun/internal/version.BuildDate={{.Date}}
```

---

## Related Documents

- [Architecture Reference: Versioning](../architecture/versioning.md)
