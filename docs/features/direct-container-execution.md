# Feature Specification: Direct Container Execution

## Overview

`cderun` executes containers directly via the native API of each supported container engine. Instead of dynamically generating and calling terminal shell commands (e.g., calling `docker run`), the unified Intermediate Representation (IR) is mapped directly to container engine API calls on the host.

## Architecture

```text
cderun Flags → Intermediate Representation (IR) → Runtime API Calls → Container Execution
                           ↓
                    ContainerConfig
                           ↓
                runtime.CreateContainer()
                runtime.StartContainer()
                runtime.AttachContainer()
                ...
```

### Implementation Status (CRI Interface)

| Method | Docker (Moby) | Podman (Compatible API) | containerd (gRPC, Experimental) |
| :--- | :---: | :---: | :---: |
| `CreateContainer` | Supported | Supported | Supported (except `--network`/`--publish`) |
| `StartContainer` | Supported | Supported | Supported |
| `WaitContainer` | Supported | Supported | Supported |
| `RemoveContainer` | Supported | Supported | Supported |
| `AttachContainer` | Supported | Supported | Supported |
| `SignalContainer` | Supported | Supported | Supported |
| `ResizeContainerTTY` | Supported | Supported | Supported |

**Benefits:**

- **No Command Generation**: Eliminates shell escaping errors, command injection vectors, and platform-specific terminal quirks.
- **Programmatic Control**: Direct lifecycle management of resources and connection monitoring.
- **Robust Error Handling**: Real-time diagnostic error codes can be retrieved directly from the container engine instead of parsing standard error text outputs.
- **Nested Context Preservation**: Seamless recursive execution of nested container layers.

## Intermediate Representation (IR): ContainerConfig

The unified data structure that handles all execution requests consistently:

- **Basic Attributes**: Image name, command (`Command`). These are consolidated at run-time and passed to the container runtime.
- **Execution Control**: TTY, interactive mode, automatic removal flag (`Remove`), and image pull policy (`Pull`).
- **Networking**: Network mode, port mapping (`Ports`), publishing all exposed ports (`PublishAll`), exposing ports (`Expose`), hostname, DNS servers, and custom host mappings (`AddHosts`).
- **Environment**: Volumes and bind mounts (including host, container, and tmpfs with `readonly` capabilities), environment variables, working directory, and execution user.
- **Security & Resources**: Privileged mode (`Privileged`), capability additions/removals (`CapAdd`/`CapDrop`), memory limit, and CPU limit.
- **Other**: Entrypoint override, device additions.

## CRI Interface: ContainerRuntime

The common abstraction interface that normalizes the differences between each container runtime:

- **Lifecycle Management**: Methods for creation, startup, waiting, and deletion.
- **I/O Control**: Dynamic attachment of standard I/O streams (stdin, stdout, stderr).
- **Control Operations**: Sending OS signals (`SignalContainer`) and synchronizing TTY window resizes (`ResizeContainerTTY`).
- **Stream Multiplexing**: When a TTY is disabled, standard output and standard error streams are separated using a multiplexed stream format (standard `stdcopy` format) and handled appropriately.

### Key Runtime Adaptation Details

- **Docker Implementation**: Implemented via the official Docker Engine API (`github.com/docker/docker/client`).
- **Podman Implementation**: Implemented via the Docker-compatible local API of Podman, sharing the core Docker client library infrastructure.
- **Translation Logic**: Maps the abstract `ContainerConfig` into runtime-specific configurations (such as Docker's `Config`, `HostConfig`, and `NetworkingConfig`).

### Conversion Contract

`ContainerConfig` is an intermediate representation that preserves the Docker CLI-compatible shape of user input. The Docker daemon itself implicitly normalizes many of these values, but runtimes that assemble an OCI spec directly (containerd) must perform that normalization in the adapter — the conversion responsibility sits on the adapter, not the daemon.

When a runtime adapter consumes a `ContainerConfig` field, exactly two outcomes are permitted:

1. **Convert it to the runtime's native representation.** For example, capability names must be normalized to the `CAP_`-prefixed form the runtime expects.
2. **Return an explicit error** if the field or value is unsupported by that runtime (see the containerd limitations in [Multi-Runtime Support](./multi-runtime-support.md)).

**Silent pass-through and silent drop are both prohibited.** An adapter must never forward a field unmodified on the assumption the underlying runtime "probably" accepts the same shape Docker does, and must never ignore a field it does not know how to translate. Either behavior turns a configuration gap into a silent correctness or security bug (see [T45](../../.agent/todo.md) for a historical incident where unsupported fields were silently dropped by the containerd adapter).

This contract applies to every current and future `ContainerRuntime` implementation, including adapters reached indirectly through the [Nested Execution Control Socket](./nested-execution-control-socket.md).

## Execution Flow

### Basic Execution Sequence

1. **Container Creation**: Invoke `CreateContainer` with the configuration, returning a unique container ID.
2. **Scheduled Cleanup**: If `config.Remove` is true, schedule `RemoveContainer` via `defer` or context cleanups to ensure ephemeral behavior.
3. **Container Startup**: Call `StartContainer` to boot the container namespace.
4. **I/O Attachment**: If `TTY` or `Interactive` is enabled, call `AttachContainer` to hook up stdin, stdout, and stderr streams.
5. **Signal & Resize Forwarders**: Spin up background routines to forward OS signals and window resizing events to the running container.
6. **Completion Wait**: Block on `WaitContainer` to await container process exit and retrieve the final exit status code.

## Resolving Nested Execution

By utilizing direct container engine APIs over raw socket mounts, `cderun` executed within a container can naturally interface with the host's container daemon.

- **Shared Runtime Socket**: Containers can mount the host's runtime socket, allowing nested `cderun` executions to spawn companion sibling containers directly on the host rather than requiring nested Docker-in-Docker daemons.
- **Environment Inheritance**: Current execution host environment variables are mounted or injected directly into the `ContainerConfig` of the child container.
- **Planned Evolution**: See [Nested Execution Control Socket](./nested-execution-control-socket.md) for a `cderun`-native control plane that projects this same `ContainerConfig` / `ContainerRuntime` abstraction over a socket the Base Host itself owns, instead of requiring nested `cderun` to dial the engine's raw socket directly.

## Roadmap

### Phase 1: Core Functionality (Completed)

- Definition of Intermediate Representation (`ContainerConfig`)
- Docker CRI Adapter implementation
- Standard execution flow orchestration

### Phase 2: Configuration Management (Completed)

- Loading configuration profiles
- Subcommand image mapping
- Layered precedence resolution
- Dry-run preview mode

### Phase 3: Advanced Functionality (Completed)

- Dynamic environment passthrough
- Auto-detection and mounting of the `cderun` binary, sockets, and tools

### Phase 4: Enhancements (Completed)

- Podman CRI Adapter integration
- Enhanced error diagnostics and robust signal forwarding
- Multi-threaded log streaming and TTY resizing

### Phase 5: Docker-compatible Flag Expansion (Completed)

- Detailed resource constraints, namespace overrides (such as `--pid`), network controls, and capability security validations added to the IR and implemented across all adapters.

## Dependency Libraries

```go
import (
    "github.com/docker/docker/client"
    "github.com/docker/docker/api/types/container"
    "github.com/docker/docker/api/types/mount"
)
```
