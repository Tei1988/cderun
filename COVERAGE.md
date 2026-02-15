# Test Coverage Report

Generated on: 2026-02-15

## Total Coverage

| Package | Statement Coverage |
| :--- | :--- |
| `internal/command` | 91.0% |
| `internal/config` | 90.3% |
| `internal/logging` | 96.1% |
| `internal/runtime` | 92.9% |
| **Total** | **91.0%** |

## Package Breakdown

### internal/command

- `root.go`: 91.6% (including flags, resolution, and execution)
- `snapshot.go`: 86.8%
- `signals_unix.go`: 90.0%

### internal/config

- `config.go`: 82.5%
- `expression.go`: 87.2%
- `path.go`: 89.5%
- `resolver.go`: 93.8%

### internal/logging

- `logger.go`: 96.1%

### internal/runtime

- `docker.go`: 91.5%
- `mock.go`: 100.0%
- `podman.go`: 100.0%

---
*Note: Coverage is measured using `go test -coverprofile=coverage.out ./...`*
