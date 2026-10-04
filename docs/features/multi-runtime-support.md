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
- **Testing Architecture**: Provides functional option `WithContainerdClient` in `internal/runtime/containerd.go` to inject mock `containerdClient` implementations, enabling zero-daemon unit testing for container lifecycles (creation, start, wait, remove, signal, resize) without requiring a running containerd socket. All runtime adapters are validated against the L3 Conformance Suite (see [ContainerRuntime Conformance Suite Guide](../testing/conformance.md)).
- **Limitations**:
  - **Platform Constraint**: **Linux-only** (`//go:build linux` build tags). It is not supported on macOS or Windows, which require virtual machines.
  - **Networking**: Only supports `host` networking. Users **must** explicitly pass `--network host` (or configure `network: host` in configurations), as the default `bridge` network setting is rejected by the containerd adapter.
  - **Port Mappings**: Port publishing (`--publish`, `-p`, `--publish-all`, `-P`) and exposing (`--expose`) are unsupported.
  - **DNS and Host Mappings**: Custom DNS servers (`--dns`) and host-to-IP mappings (`--add-host`) are unsupported.
  - **Mount Types**: Named volumes are unsupported; only `bind` and `tmpfs` mounts are supported.
  - **Linux Capabilities**: Custom capability controls (`--cap-add` and `--cap-drop`) are supported.
  - **ENTRYPOINT Inheritance**: Automatically prepends the image's defined `ENTRYPOINT` when executing command overrides, matching standard Docker behavior.

### 4. nerdctl (CLI-Based Engine)

- **Status**: Production-ready when configured with a containerd socket. Without an explicit socket, the resolver passes `/var/run/docker.sock` to nerdctl as `--address`.
- **Communication Protocol**: Interacts via the `nerdctl` CLI binary (`exec.Command`), executing subcommands like `nerdctl container create`, `start`, `wait`, `rm`, `attach`, `kill`, and `inspect`.
- **Argument Injection Defense Architecture (CWE-88)**:
  CLI-based execution presents argument injection risks if user or nested parameters (such as `Image`, `Command`, or `Env`) contain flag-like strings (e.g., `--privileged`). `nerdctl` adapter implements a 3-layer security defense via `CLIArgBuilder`:
  1. **Positional Boundary Separation**: Inserts `--` before the trailing positional block (`<image> <cmd> <args...>`), ensuring the underlying CLI parser (Cobra/pflag) interprets them as literal arguments.
  2. **Joined `--flag=value` Formatting**: Formats flag key-value pairs (e.g., `--env=KEY=VALUE`) as single joined tokens to prevent parameter splitting.
  3. **Structural `argv` Self-Checking**: Performs structural validation (`VerifyStructure`) on built `argv` prior to execution for `PullImage` and `CreateContainer` operations only, verifying that positional arguments follow `--` and each preceding flag token begins with `-`.

---

## Runtime Capability Comparison Matrix

| Feature / Capability | Docker | Podman | containerd (Direct gRPC) | nerdctl (CLI-Based) |
| :--- | :---: | :---: | :---: | :---: |
| **Supported OS** | Linux, macOS, Windows | Linux, macOS, Windows | **Linux-only** (`//go:build linux`) | **Linux-only** |
| **Communication Protocol** | HTTP Unix socket | HTTP Unix socket | gRPC Unix socket | CLI Subprocess (`nerdctl`) |
| **Bridge Networking** | Yes | Yes | No (Host network only) | Yes |
| **Port Publishing (`-p`, `-P`)** | Yes | Yes | No | Yes |
| **Custom DNS & Add-Host** | Yes | Yes | No | Yes |
| **Bind & tmpfs Mounts** | Yes | Yes | Yes | Yes |
| **Named Volume Mounts** | Yes | Yes | No | Yes |
| **Linux Capabilities (`--cap-add/drop`)** | Yes | Yes | Yes (Converted to `CAP_` prefix) | Yes |
| **Custom OCI Runtime (`--oci-runtime`)** | Yes | Yes | No | Yes |
| **Process Resource Limits (ulimits)** | Yes | Yes | Yes (Converted to POSIX rlimits) | Yes |
| **Read-Only RootFS** | Yes | Yes | Yes | Yes |
| **Host PID / IPC / Cgroup Namespaces** | Yes | Yes | Yes (Host or Private) | Yes |
| **Init Process (`--init`)** | Yes | Yes | No | Yes |
| **Restart Policies (`--restart`)** | Yes | Yes | No | Yes |

---

## Architecture and Abstraction Layer

The engine abstraction uses a unified Go interface:

```text
                           ContainerRuntime Interface
                                       │
        ┌──────────────────┬───────────┴──────┬──────────────────┐
        ▼                  ▼                  ▼                  ▼
  DockerRuntime       PodmanRuntime    ContainerdRuntime   NerdctlRuntime
  (HTTP Unix socket)  (HTTP Unix socket) (gRPC socket)     (CLI subprocess)
```

### Interface Responsibilities

The `ContainerRuntime` interface governs:

- **Lifecycle Management**: Container creation, starting, waiting, and removal.
- **I/O Operations**: Attaching stdin, stdout, and stderr streams, and supporting terminal TTY sessions.
- **Diagnostics**: Returning the engine name and connection status.
- **Signal and Control**: Forwarding signals and synchronizing TTY window size resizes.

---

## Runtime Engine Selection

`cderun` supports selecting the target container execution engine via command-line options, environment variables, or global configuration settings.

### Supported Runtime Values

The supported runtime engines are:

- `docker`: Standard Docker Engine daemon (default)
- `podman`: Podman local service API
- `containerd`: Direct containerd gRPC service (Linux only)
- `nerdctl`: `nerdctl` CLI-based container engine

### Configuration Mappings

- **Option Flag**: `--engine` (P2) / `--cderun-engine` (P1) (*Note: `--runtime` / `--cderun-runtime` are supported as deprecated aliases*)
- **Environment Variable**: `CDERUN_ENGINE` (P3) (*Note: `CDERUN_RUNTIME` is supported as a deprecated alias*)
- **Configuration Key**: `engine:` inside `.cderun.yaml` (*Note: `runtime:` is supported as a deprecated alias; `engine:` takes precedence if both are set*)
- **Default Value**: Auto-detected via socket availability (or fallback to `docker`)

### Resolution Priority Sequence for Engine Selection

1. **Phase 1 (P1) Internal Overrides**: P1 flags (`--cderun-engine` or `--cderun-runtime`) outrank environment and configuration values. When `--cderun-engine` and deprecated alias `--cderun-runtime` are both specified, `--cderun-engine` takes precedence.
2. **CLI (P2) Flags**: Explicit `--engine` (or deprecated `--runtime`) or `--socket-path` CLI flags. When both `--engine` and `--runtime` are supplied at the CLI tier, `--engine` takes precedence.
3. **Environment Variables (P3)**: `CDERUN_ENGINE` (or deprecated `CDERUN_RUNTIME`) or `CDERUN_SOCKET_PATH`.
4. **Configuration Files (P5)**: Global `engine:` (or deprecated `runtime:`) or `socketPath:` keys in `.cderun.yaml`.

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
