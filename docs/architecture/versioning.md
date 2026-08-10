# Version Management

`cderun` performs dynamic version management based on Git information.

---

## Overview

Previously, the version number was hardcoded in `internal/command/version.go`, but it has now migrated to a system that injects information using `ldflags` at build time.
This automatically embeds the Git tag, commit SHA, and build date and time into the binary.

---

## Implementation Details

### 1. Retention of Version Info (`internal/version`)

The `internal/version` package maintains the following variables:

- `Version`: Git tag (or `dev`)
- `Revision`: Short Git commit SHA
- `BuildDate`: ISO8601 formatted build date and time

These variables are overwritten via the `-ldflags` option during `go build`.

### 2. Local Build (`Makefile`)

Running `make build` in the development environment internally executes the following command to inject the latest information retrieved from Git:

```bash
go build -ldflags "-X github.com/cderun/cderun/internal/version.Version=v1.0.0 -X github.com/cderun/cderun/internal/version.Revision=abcdef -X github.com/cderun/cderun/internal/version.BuildDate=2026-06-04T12:00:00Z" -o cderun main.go
```

Within the `Makefile`, information is retrieved as follows:

```makefile
VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
REVISION   ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
```

### 3. Release Build (`GoReleaser`)

Official release binaries are created by `GoReleaser`. In the `ldflags` section of `.goreleaser.yaml`, variables provided by GoReleaser are mapped to the respective variables in `internal/version`.

---

## Usage

### Check Binary Version

Using the `--version` flag, you can view detailed version information:

```bash
cderun --version
```

Output Example:

```text
cderun version v1.0.0 (revision: abcdef, built: 2026-06-04T12:00:00Z)
```

### Local Development Behavior (`go run`)

When executing via `go run main.go` or similar without specifying `ldflags`, the following fallback values are used:

- `Version`: `dev`
- `Revision`: `unknown`
- `BuildDate`: `unknown`

This allows identifying that the binary has not gone through the official build procedure.
