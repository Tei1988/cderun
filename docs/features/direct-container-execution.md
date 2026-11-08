# Feature Specification: Direct Container Execution

## Overview

`cderun` executes containers directly via the native API or programmatic CLI adapter of each supported container engine. Instead of dynamically generating and calling arbitrary shell commands (e.g., shell-interpolated `docker run`), the unified Intermediate Representation (IR) is mapped directly to container engine API calls or structurally verified CLI invocations on the host.

## Architecture

```text
cderun Flags → Intermediate Representation (IR) → Runtime Adapter Calls → Container Execution
                           ↓
                    ContainerConfig
                           ↓
                runtime.CreateContainer()
                runtime.AttachContainer()
                runtime.StartContainer()
                ...
```

### Implementation Status across Supported Engines

| Method | Docker (Moby API) | Podman (Compatible API) | containerd (gRPC Client) | nerdctl (CLI-Based Adapter) |
| :--- | :---: | :---: | :---: | :---: |
| `PullImage` | Supported | Supported | Supported | Supported |
| `ValidateConfig` | Supported | Supported | Supported | Supported |
| `CreateContainer` | Supported | Supported | Supported (except `--network`/`--publish`) | Supported |
| `StartContainer` | Supported | Supported | Supported | Supported |
| `WaitContainer` | Supported | Supported | Supported | Supported |
| `RemoveContainer` | Supported | Supported | Supported | Supported |
| `AttachContainer` | Supported | Supported | Supported | Supported |
| `SignalContainer` | Supported | Supported | Supported | Supported |
| `ResizeContainerTTY` | Supported | Supported | Supported | Supported |
| `InspectContainer` | Supported | Supported | Supported | Supported |
| `PruneContainers` | Supported | Supported | Supported | Supported |

**Key Benefits:**

- **No Arbitrary Shell Scripting**: Eliminates shell escaping errors, command injection vectors, and platform-specific shell interpretation quirks.
- **Programmatic Control**: Direct lifecycle management of resources, error codes, and connection monitoring.
- **Robust Error Handling**: Real-time diagnostic error codes are retrieved directly from the container engine or process exits rather than parsing localized text outputs.
- **Nested Context Preservation**: Seamless recursive execution of nested container layers.

---

## Intermediate Representation (IR): ContainerConfig

The unified data structure (`internal/container.ContainerConfig`) that handles all execution requests consistently:

- **Basic Attributes**: Container image (`Image`), entrypoint override (`Entrypoint`), command and positional arguments (`Command`).
- **Execution Control**: Pseudo-TTY allocation (`TTY`), interactive standard input (`Interactive`), automatic removal flag (`Remove`), and image pull policy (`Pull`: `always`, `missing`, `never`).
- **Networking & Host Configuration**: Network mode (`Network`), published port mappings (`Ports`), expose directives (`Expose`), publishing all exposed ports (`PublishAll`), container hostname (`Hostname`), DNS nameservers (`DNS`), DNS search domains (`DNSSearch`), DNS resolver options (`DNSOptions`), and custom `/etc/hosts` host mappings (`AddHosts`).
- **Filesystem & Mounts**: Volume and bind mounts (`Mounts` supporting bind, volume, and tmpfs mounts with `read_only` controls), host working directory (`Workdir`), and execution user/group identity (`User`).
- **Environment & Metadata**: Injected environment variables (`Env`), container metadata labels (`Labels`).
- **Security & Permissions**: Privileged container execution (`Privileged`), Linux capability additions (`CapAdd`) and drops (`CapDrop`), security options (`SecurityOpt`), and kernel sysctl tunables (`Sysctls`).
- **Namespaces & Isolation**: IPC namespace mode (`IPC`), PID namespace mode (`PID`), Cgroup namespace mode (`Cgroupns`), and custom OCI runtime selection (`OciRuntime`).
- **Resource Constraints & Limits**: Memory limit in bytes (`Memory`), CPU core allocation (`CPUs`), CPU shares (`CPUShares`), CPU core pinning (`CpusetCpus`), CPU memory node pinning (`CpusetMems`), maximum PID limits (`PidsLimit`), shared memory size in bytes (`ShmSize`), resource ulimits (`Ulimits`), and GPU allocations (`GPUs`).
- **Process Management & Lifecycle**: Container init process enablement (`Init`), restart policy (`Restart`), and supplementary group IDs (`GroupAdd`).

---

## CRI Interface: ContainerRuntime

The common abstraction interface (`internal/runtime.ContainerRuntime`) that normalizes the differences between each container runtime:

```go
type ContainerRuntime interface {
    // Image management
    PullImage(ctx context.Context, image string, pullPolicy string, maxRetries int, backoffBase time.Duration) error

    // Container lifecycle
    ValidateConfig(config *container.ContainerConfig) error
    CreateContainer(ctx context.Context, config *container.ContainerConfig) (string, error)
    StartContainer(ctx context.Context, containerID string) error
    WaitContainer(ctx context.Context, containerID string) (int, error)
    RemoveContainer(ctx context.Context, containerID string) error

    // Container communication
    AttachContainer(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer, ready chan<- struct{}) error
    ResizeContainerTTY(ctx context.Context, containerID string, rows, cols uint) error
    SignalContainer(ctx context.Context, containerID string, sig string) error

    // Information
    Name() string
    InspectContainer(ctx context.Context, containerID string) (isRunning bool, exitCode int, err error)

    // Cleanup
    PruneContainers(ctx context.Context) ([]string, error)
    Close() error
}
```

### Lifecycle Method Contracts

1. **`ValidateConfig`**:
   - Performs eager upfront validation of `ContainerConfig` before contacting daemons or creating resources.
   - Verifies that all requested configuration options are supported by the selected engine. Unsupported options return an explicit error immediately.
2. **`PullImage`**:
   - Evaluates the pull policy (`always`, `missing`, `never`).
   - Implements exponential backoff retry mechanics for transient network or registry errors.
3. **`CreateContainer`**:
   - Translates `ContainerConfig` into runtime-native container specifications (e.g., Docker `Config`/`HostConfig`/`NetworkingConfig`, OCI spec, or nerdctl argv).
   - Returns a unique string container ID.
4. **`StartContainer`**:
   - Starts a previously created container.
   - **Containerd Order Invariant**: For containerd, `AttachContainer` **must** be called before `StartContainer` is invoked. Calling `StartContainer` prior to attaching causes container standard I/O to be silently discarded or directed to null I/O readers/writers.
5. **`AttachContainer`**:
   - Connects host standard streams (stdin, stdout, stderr) to the container I/O pipes.
   - Closes the `ready` signal channel once stream setup is established to notify the runner that `StartContainer` can safely proceed.
6. **`WaitContainer`**:
   - Blocks until the container process exits, returning the process exit code.
7. **`RemoveContainer`**:
   - Cleans up the container and associated ephemeral resources. When `cc.Remove = true`, executed during deferred cleanup.
8. **`SignalContainer` & `ResizeContainerTTY`**:
   - Forwards OS signals (e.g. `SIGINT`, `SIGTERM`, `SIGHUP`) and window geometry resize events (`SIGWINCH`) to the container process.
9. **`InspectContainer`**:
   - Queries current container state, returning whether the container is actively running and its exit code.
10. **`PruneContainers`**:
    - Scans for orphan stopped containers generated by `cderun` (identified via label `cderun=true` or containerd namespace `cderun`) and removes them.

---

## Stream Multiplexing & I/O Protocol

Standard input, standard output, and standard error handling depend on whether pseudo-terminal allocation (`TTY`) is enabled:

### 1. Multiplexed Streams (TTY Disabled)

When `TTY` is disabled (default in batch or piped executions), standard output and standard error are multiplexed across a single binary stream using the standard `stdcopy` framing protocol:

- **Header Format**: An 8-byte binary framing header prepends each data chunk:
  - **Byte 0**: Stream type indicator:
    - `0x01`: Standard Output (`stdout`)
    - `0x02`: Standard Error (`stderr`)
    - `0x00`: Standard Input (`stdin`)
  - **Bytes 1–3**: Reserved (padding, `0x00 0x00 0x00`).
  - **Bytes 4–7**: Big-endian unsigned 32-bit integer indicating payload length in bytes.
- **Demultiplexing**: The runtime adapter reads the framing header, extracts the exact frame payload, and writes it directly to the designated `stdout` or `stderr` writer without data contamination.

### 2. Raw Stream Pass-Through (TTY Enabled)

When `TTY` is enabled (`-t` / `--tty`), the container engine allocates a virtual pseudo-terminal (PTY). In PTY mode:

- Standard output and standard error are combined by the PTY driver into a single raw byte stream without framing headers.
- Data is piped directly to the host's terminal output descriptor.

---

## Engine Conversion Contract

`ContainerConfig` is an intermediate representation that preserves the Docker CLI-compatible shape of user input.

When a runtime adapter consumes a `ContainerConfig` field, exactly two outcomes are permitted:

1. **Convert it to the runtime's native representation.** For example, capability names must be normalized to the `CAP_`-prefixed form the runtime expects.
2. **Return an explicit error** if the field or value is unsupported by that runtime (see the containerd limitations in [Multi-Runtime Support](./multi-runtime-support.md)).

**Silent pass-through and silent drop are both strictly prohibited.** An adapter must never forward a field unmodified on the assumption the underlying runtime "probably" accepts the same shape Docker does, and must never ignore a field it does not know how to translate. Either behavior turns a configuration gap into a silent correctness or security bug (see [T45](../../.agent/todo.md) for a historical incident where unsupported fields were silently dropped by the containerd adapter).

### Eager Upfront Validation Contract

To uphold the conversion contract, runtime adapters enforce eager validation in `ValidateConfig`:

- **Nil Configuration**: Passing a nil configuration returns an immediate error.
- **GPU Specifications**: Validated using `parseGPUs`.
- **Restart Policies**: Validated using `parseDockerRestartPolicy`. Non-`on-failure` policies with retry counts or malformed syntax are rejected.
- **Port Mappings & Expositions**: Validated using port specification parsers (`nat.ParsePortSpecs` and `buildDockerExposedPorts`).
- **Mount Integrity**: Validated via mount parsers. Unsupported optional mounts and invalid mount types are rejected.
- **Shared Memory Size**: Non-empty `ShmSize` strings must parse into valid positive byte sizes (e.g., `64m`, `1g`).
- **Engine-Specific Unsupported Fields**: Runtimes reject features they cannot support (e.g., containerd rejecting `--network`, `--publish`, and `--oci-runtime`).

---

## Execution Flow

```text
1. ValidateConfig(config)
           ↓
2. PullImage(config.Image, config.Pull)
           ↓
3. CreateContainer(config) -> containerID
           ↓
4. AttachContainer(containerID, stdin, stdout, stderr, readyChan)
           ↓ (readyChan closed)
5. StartContainer(containerID)
           ↓
6. Forward Signals & TTY Window Resizes
           ↓
7. WaitContainer(containerID) -> exitCode
           ↓
8. RemoveContainer(containerID) [if config.Remove]
```

### Basic Execution Sequence

1. **Pre-flight Validation**: Invoke `ValidateConfig` with `ContainerConfig` to ensure all fields are valid for the chosen engine.
2. **Image Availability**: Call `PullImage` according to the configured pull policy.
3. **Container Creation**: Invoke `CreateContainer` with the configuration, returning a unique container ID.
4. **Scheduled Cleanup**: If `config.Remove` is true, schedule `RemoveContainer` via `defer` or context cleanups to ensure ephemeral behavior.
5. **I/O Attachment**: If `TTY` or `Interactive` is enabled, call `AttachContainer` to hook up stdin, stdout, and stderr streams.
6. **Container Startup**: Call `StartContainer` to boot the container namespace once streams are ready.
7. **Signal & Resize Forwarders**: Spin up background routines to forward OS signals and window resizing events to the running container.
8. **Completion Wait**: Block on `WaitContainer` to await container process exit and retrieve the final exit status code.

---

## Resolving Nested Execution

By utilizing direct container engine APIs over raw socket mounts or Control Socket IPC, `cderun` executed within a container naturally interfaces with the host's container engine:

- **Shared Runtime Socket**: Containers can mount the host's runtime socket (`--mount-socket`), allowing nested `cderun` executions to spawn companion sibling containers directly on the host rather than requiring nested Docker-in-Docker daemons.
- **Nested Execution Control Socket**: Enables a scoped, runtime-agnostic IPC control plane (`--mount-cderun-socket`) where nested `cderun` binaries communicate directly with the Base Host over length-prefixed protocol frames. This projects the same `ContainerConfig` / `ContainerRuntime` abstraction across security boundaries while enforcing inherited-ceiling policy constraints. See [Nested Execution Control Socket](./nested-execution-control-socket.md).

---

## Dependency Libraries

```go
import (
    "github.com/docker/docker/client"
    "github.com/docker/docker/api/types/container"
    "github.com/docker/docker/api/types/mount"
    "github.com/docker/docker/pkg/stdcopy"
)
```
