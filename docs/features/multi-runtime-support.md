# Multi-Runtime Support Specification

## Overview

`cderun` supports running ephemeral containers on multiple container runtimes: **Docker**, **Podman**, and **containerd**. It defines a unified `ContainerRuntime` Go interface that abstracts the underlying APIs of these runtimes.

---

## Supported Runtimes

### 1. Docker

- **Status**: Fully Supported (Default)
- **API Foundation**: Uses the official Docker Engine API (`github.com/docker/docker/client`) via Unix socket communication.
- **Features**: Automatic API version negotiation, full port/network mapping, bind/volume/tmpfs mounting, resource limits, capabilities, and interactive TTY support.

### 2. Podman

- **Status**: Fully Supported
- **API Foundation**: Communicates directly with the Podman service Unix socket using the Docker-compatible API client, enabling a drop-in replacement on Linux environments.
- **Features**: Inherits all standard execution features of the Docker runtime adapter. Supports rootless Podman execution seamlessly.

### 3. containerd (Experimental)

- **Status**: Experimental (Direct native gRPC integration)
- **API Foundation**: Connects directly to the containerd gRPC socket to bypass Docker/Podman daemon layers, enabling lightweight, low-overhead container executions.
- **Constraints & Platform Restrictions**:
  - **Linux-Only**: Direct containerd execution requires Linux (`//go:build linux` build tag). It is not supported natively on macOS or Windows without virtual machines.
  - **Networking**: Only `host` network mode is supported. The default `bridge` networking is not supported.
  - **Ports & DNS**: Port publishing (`--publish`, `-p`, `--publish-all`, `-P`), exposing (`--expose`), custom DNS (`--dns`), and custom host mappings (`--add-host`) are not supported.
  - **Mounts**: Named volumes are not supported. Only `bind` and `tmpfs` mounts can be used.
  - **Capabilities**: Full support for `--cap-add` and `--cap-drop`. The containerd adapter normalizes Docker-compatible names (e.g., `SYS_ADMIN`) into OCI-spec-compliant names (e.g., `CAP_SYS_ADMIN`).
  - **ENTRYPOINT Propagation**: If an image specifies a default `ENTRYPOINT` but the user passes custom container commands, containerd correctly prepends the `ENTRYPOINT` to ensure parity with standard Docker behavior.

---

## Architecture & Abstraction Layer

`cderun` defines a unified `ContainerRuntime` interface which isolates command execution from runtime engines:

```text
       cderun Execution Engine (Cobra/CRI)
                     │
                     ▼
          ContainerRuntime Interface
                     │
       ┌─────────────┼─────────────┐
       ▼             ▼             ▼
 DockerRuntime   PodmanRuntime  ContainerdRuntime
```

The `ContainerRuntime` interface is responsible for:

- Container Lifecycle (Create, Start, Wait, Remove)
- Standard I/O attachment (with raw TTY and interactive terminal relaying)
- Real-time window resizing (`SIGWINCH` synchronization)
- Graceful signal forwarding (`SIGINT`/`SIGTERM`)

---

## Runtime and Socket Path Selection

`cderun` resolves the container engine to use based on settings merged from configuration layers, environment variables, or CLI options.

### Auto-detection Sequence

If no engine or runtime socket path is explicitly provided, `cderun` dynamically probes the local filesystem for active container runtime sockets in the following priority order:

1. `/var/run/docker.sock` (Runtime: `docker`)
2. `/run/containerd/containerd.sock` (Runtime: `containerd`)
3. `/run/podman/podman.sock` (Runtime: `podman`)

If none of these files are found, `cderun` defaults to `docker` at `/var/run/docker.sock` (which will fail during runtime initialization with a descriptive connection error).

---

### Socket Detection Cache (Performance Optimization)

Probing the host filesystem for socket paths can introduce redundant disk I/O and `Stat` system calls on every configuration resolution. `cderun` implements an optimized caching strategy for socket auto-detection:

- **Process-Lifetime Caching**: When executing on the real OS filesystem (`RealFileSystem`), the first successfully detected runtime socket path and engine type are cached globally.
- **RWMutex Synchronization**: The global cache is thread-safe, protected by a process-wide read-write lock (`sync.RWMutex`). Subsequent config resolutions retrieve the cached runtime selection instantly, completely bypassing filesystem lookups.
- **No Dynamic Re-validation**: Once cached, host environment changes (e.g., subsequent daemon shutdowns or socket removals) are not dynamically re-validated unless the process is restarted. However, any explicit overrides—such as CLI options (`--runtime` / `--socket-path`), environment variables (`CDERUN_RUNTIME` / `CDERUN_SOCKET_PATH`), or configuration file settings (`runtime` / `socketPath` in YAML)—will completely bypass the socket auto-detection cache and force explicit path usage.
- **Dynamic Probing on Empty Cache**: If no active sockets are detected on the host during the initial probe, the cache is left empty. This allows subsequent config resolution runs to probe again, catering to situations where a container daemon is started after `cderun` begins execution.
- **Test Isolation**: Mock or in-memory filesystems used during unit tests bypass this global process-level caching, ensuring complete test case isolation and preventing state leakage.

---

### Explicit Selection Examples

#### 1. Global Settings (`.cderun.yaml`)

```yaml
runtime: podman
socketPath: /run/user/1000/podman/podman.sock
```

#### 2. Environment Variables

```bash
export CDERUN_RUNTIME=podman
export CDERUN_SOCKET_PATH=/run/user/1000/podman/podman.sock
cderun node app.js
```

#### 3. Command Line Flags

```bash
cderun --runtime containerd --socket-path /run/containerd/containerd.sock node app.js
```

---

## Diagnosis Mode

To view the active container runtime configuration, auto-detected socket path, socket path accessibility, and configuration file paths, run the diagnosis command:

```bash
cderun --diagnosis
```

The diagnostics report generated by `handleDiagnosis` includes:

- **Runtime Name**: The active container engine name (e.g., `docker`, `podman`, or `containerd`).
- **Socket Path & Accessibility**: The path to the runtime socket and whether it is accessible on the host (validated via local filesystem inspection rather than active daemon connectivity verification).
- **Configuration Paths**: Absolute paths to the resolved global config (`.cderun.yaml`) and tool config (`.tools.yaml`) files.
- **Available Tools**: A list of all subcommands/tools mapped inside `.tools.yaml`.
