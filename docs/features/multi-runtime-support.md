# Feature Specification: Multi-Runtime Support

## Overview

`cderun` supports multiple container runtime engines, defining a common `ContainerRuntime` interface that abstracts engine-specific APIs.

---

## Supported Container Engines

### 1. Docker

- **Status**: Production-ready (default engine).
- **Communication Protocol**: Interacts via the Docker Engine API over standard HTTP Unix sockets. Supports automatic API version negotiation.

### 2. Podman

- **Status**: Production-ready.
- **Communication Protocol**: Interacts via the Podman local service Unix socket using Docker-compatible APIs.

### 3. containerd (Direct gRPC)

- **Status**: Experimental.
- **Communication Protocol**: Interacts directly with the containerd gRPC API, bypassing Docker/Podman engines for lighter execution.
- **Limitations**:
  - **Platform Constraint**: **Linux-only** (`//go:build linux` build tags). It is not supported on macOS or Windows, which require virtual machines.
  - **Networking**: Only supports `host` networking. Users **must** explicitly pass `--network host` (or configure `network: host` in configurations), as the default `bridge` network setting is rejected by the containerd adapter.
  - **Port Mappings**: Port publishing (`--publish`, `-p`, `--publish-all`, `-P`) and exposing (`--expose`) are unsupported.
  - **DNS and Host Mappings**: Custom DNS servers (`--dns`) and host-to-IP mappings (`--add-host`) are unsupported.
  - **Mount Types**: Named volumes are unsupported; only `bind` and `tmpfs` mounts are supported.
  - **Linux Capabilities**: Custom capability controls (`--cap-add` and `--cap-drop`) are supported.
  - **ENTRYPOINT Inheritance**: Automatically prepends the image's defined `ENTRYPOINT` when executing command overrides, matching standard Docker behavior.

---

## Architecture and Abstraction Layer

The engine abstraction uses a unified Go interface:

```text
               ContainerRuntime Interface
                           │
        ┌──────────────────┼──────────────────┐
        ▼                  ▼                  ▼
  DockerRuntime       PodmanRuntime    ContainerdRuntime
  (HTTP Unix socket)  (HTTP Unix socket) (gRPC socket)
```

### Interface Responsibilities

The `ContainerRuntime` interface governs:

- **Lifecycle Management**: Container creation, starting, waiting, and removal.
- **I/O Operations**: Attaching stdin, stdout, and stderr streams, and supporting terminal TTY sessions.
- **Diagnostics**: Returning the engine name and connection status.
- **Signal and Control**: Forwarding signals and synchronizing TTY window size resizes.

---

## Engine vs OCI Runtime Specification

`cderun` decouples the container engine selection from the underlying OCI runtime specification:

- **Container Engine (`--engine`, `CDERUN_ENGINE`, `engine:` in YAML)**: Specifies which container daemon engine to connect to (`docker`, `podman`, or `containerd`). Default is `docker`.
- **OCI Runtime (`--runtime`, `CDERUN_OCI_RUNTIME`, `runtime:` in YAML)**: Specifies the lower-level OCI runtime binary (e.g. `runc`, `crun`, `nvidia`, `kata`).

### Backward Compatibility and Migration

For backward compatibility with older configurations:

- `CDERUN_RUNTIME` serves as a deprecated fallback alias for `CDERUN_ENGINE` when no explicit `CDERUN_ENGINE` is set.
- Legacy `runtime:` YAML configuration entries containing container engine values (`docker`, `podman`, `containerd`) are automatically migrated to `engine:` at configuration load time and emit a deprecation warning.
- Docker engine passes the OCI runtime directly to `HostConfig.Runtime` (e.g., when specifying `--runtime nvidia`). Direct `containerd` adapter validates OCI runtime requests and rejects unsupported custom runtimes with an explicit validation error.

## Engine Selection

### Resolution Priority Sequence

1. **Phase 1 (P1) and CLI (P2) Flags**: Explicit `--engine` or `--socket-path` settings.
2. **Environment Variables (P3)**: `CDERUN_ENGINE` (or legacy `CDERUN_RUNTIME`) or `CDERUN_SOCKET_PATH`.
3. **Configuration Files (P5)**: `engine` (or legacy `runtime`) or `socketPath` keys inside `.cderun.yaml`.

### Automated Socket Detection Sequence

If no engine or socket is explicitly specified, the resolver scans for socket files in the following priority order:

1. `/var/run/docker.sock` (Launches `docker` engine).
2. `/run/containerd/containerd.sock` (Launches `containerd` engine).
3. `/run/podman/podman.sock` (Launches `podman` engine).

If no socket file is discovered, `cderun` defaults to `docker` at `/var/run/docker.sock` (which may fail at execution time if the service is stopped).

#### Performance Optimization: Process Socket Cache

To minimize redundant disk I/O and `Stat` system calls during option evaluation, `cderun` caches successful socket auto-detection results:

- **Real File System Caching**: On the actual host file system (`RealFileSystem`), the first successful socket auto-detection result is stored in a process-lifetime cache protected by a read/write lock (`sync.RWMutex`). Subsequent config resolutions bypass file system lookups and retrieve the cached engine selection.
  - **Cache Lifetime Constraint**: The cache is not dynamically re-validated. If the host socket is deleted or the container service is stopped after the initial detection, the cache still returns the cached selection. Users can force a refresh by restarting the process or explicitly specifying `--runtime` / `--socket-path`.
- **Dynamic Re-Detection Fallback**: If no active socket is found during detection, the result is **not** cached. This allows `cderun` to run a fresh scan on subsequent calls, accommodating cases where the container daemon is started in the background after `cderun` was first invoked.
- **Testing Isolation**: In tests using a mocked file system (`MockFileSystem`), caching is bypassed to prevent cross-test leakage, ensuring that each unit test performs active detection.

---

## Diagnostics Verification

To check the active container engine and socket paths, execute with the `--diagnosis` flag:

```bash
cderun --diagnosis --diagnosis-format simple
```
