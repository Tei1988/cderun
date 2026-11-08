# Feature Specification: Direct Container Execution

## Overview

`cderun` executes containers directly via the native API or programmatic CLI adapter of each supported container engine. Instead of dynamically generating and calling arbitrary shell commands (e.g., shell-interpolated `docker run`), the unified Intermediate Representation (IR) is mapped directly to container engine API calls or structurally verified CLI invocations on the host.

<!-- coderabbit-ai-generated:v1 -->

<!-- coderabbit-ai-generated-section:v1:9faccc11362189901a3349972fae82efe88de24b5cd09b4873a6491162d95164 -->

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
| `CreateContainer` | Supported | Supported | Supported (`--network host` only; default bridge and other explicit modes rejected; no `--publish`) | Supported |
| `StartContainer` | Supported | Supported | Supported | Supported |
| `WaitContainer` | Supported | Supported | Supported | Supported |
| `RemoveContainer` | Supported | Supported | Supported | Supported |
| `AttachContainer` | Supported | Supported | Supported | Supported |
| `SignalContainer` | Supported | Supported | Supported | Supported |
| `ResizeContainerTTY` | Supported | Supported | Supported | Unsupported |
| `InspectContainer` | Supported | Supported | Supported | Supported |
| `PruneContainers` | Supported | Supported | Supported | Supported |

**Key Benefits:**

- **No Arbitrary Shell Scripting**: Eliminates shell escaping errors, command injection vectors, and platform-specific shell interpretation quirks.
- **Programmatic Control**: Direct lifecycle management of resources, error codes, and connection monitoring.
- **Robust Error Handling**: Real-time diagnostic error codes are retrieved directly from the container engine or process exits rather than parsing localized text outputs.
- **Nested Context Preservation**: Seamless recursive execution of nested container layers.

---

<!-- /coderabbit-ai-generated:v1 -->

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

<!-- coderabbit-ai-generated:v1 -->

<!-- coderabbit-ai-generated-section:v1:4f47d3fe2069cde075d3696943a20c5c5f3c2b3bef0b0c2ec44fca0c9f7c561f -->

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
   - Runs adapter-specific checks before image pulling; coverage varies by engine (see Eager Upfront Validation below).
   - A successful return does not guarantee that all configuration errors have been detected before pulling or creating resources.
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
   - Closes the `ready` signal channel after stream setup, but `DockerRuntime.AttachContainer` also closes it when the attach API fails. Closure alone does not indicate success: callers must also check and handle the attach error before proceeding with `StartContainer`.
6. **`WaitContainer`**:
   - Blocks until the container process exits, returning the process exit code.
7. **`RemoveContainer`**:
   - Cleans up the container and associated ephemeral resources. When `cc.Remove = true`, executed during deferred cleanup.
8. **`SignalContainer` & `ResizeContainerTTY`**:
   - Forwards OS signals (e.g. `SIGINT`, `SIGTERM`, `SIGHUP`) and window geometry resize events (`SIGWINCH`) to the container process.
9. **`InspectContainer`**:
   - Queries current container state, returning whether the container is actively running and its exit code.
10. **`PruneContainers`**:
    - Docker-compatible cleanup filters by label `cderun=true`; containerd cleanup uses namespace `cderun`. `NerdctlRuntime.PruneContainers` invokes an unfiltered `nerdctl container prune --force` in its configured scope and may also remove stopped containers created outside `cderun`.

---

<!-- /coderabbit-ai-generated:v1 -->

<!-- coderabbit-ai-generated:v1 -->

<!-- coderabbit-ai-generated-section:v1:395c8a0017295f86d59d95442cdcfbd8ea16d389266c5b5959f36a855772bdf9 -->

## Stream Multiplexing & I/O Protocol

Standard input, standard output, and standard error handling depend on whether pseudo-terminal allocation (`TTY`) is enabled:

### 1. Output Streams (TTY Disabled)

When `TTY` is disabled (default in batch or piped executions), Docker-compatible attaches multiplex standard output and standard error across a single binary stream using the `stdcopy` framing protocol:

- **Header Format**: An 8-byte binary framing header prepends each data chunk:
  - **Byte 0**: Stream type indicator:
    - `0x01`: Standard Output (`stdout`)
    - `0x02`: Standard Error (`stderr`)
    - `0x00`: Standard Input (`stdin`)
  - **Bytes 1–3**: Reserved (padding, `0x00 0x00 0x00`).
  - **Bytes 4–7**: Big-endian unsigned 32-bit integer indicating payload length in bytes.
- **Demultiplexing**: The Docker-compatible runtime adapter reads the framing header, extracts the exact frame payload, and writes it directly to the designated `stdout` or `stderr` writer without data contamination.

Containerd uses separate streams through `cio.WithStreams(stdin, stdout, stderr)`. Nerdctl assigns the supplied writers directly to the attach command's `Stdout` and `Stderr`. Neither adapter applies `stdcopy` framing to these paths.

### 2. Raw Stream Pass-Through (TTY Enabled)

When `TTY` is enabled (`-t` / `--tty`), the container engine allocates a virtual pseudo-terminal (PTY). In PTY mode:

- Standard output and standard error are combined by the PTY driver into a single raw byte stream without framing headers.
- Data is piped directly to the host's terminal output descriptor.

---

<!-- /coderabbit-ai-generated:v1 -->

<!-- coderabbit-ai-generated:v1 -->

<!-- coderabbit-ai-generated-section:v1:56c2d08fd1e180c5bd4ab09c9ffd51978c15cf95d400ac73fdc3e6d087d556d5 -->

## Engine Conversion Contract

`ContainerConfig` is an intermediate representation that preserves the Docker CLI-compatible shape of user input.

When a runtime adapter consumes a `ContainerConfig` field, exactly two outcomes are permitted:

1. **Convert it to the runtime's native representation.** For example, capability names must be normalized to the `CAP_`-prefixed form the runtime expects.
2. **Return an explicit error** if the field or value is unsupported by that runtime (see the containerd limitations in [Multi-Runtime Support](./multi-runtime-support.md)).

**Silent pass-through and silent drop are both strictly prohibited.** An adapter must never forward a field unmodified on the assumption the underlying runtime "probably" accepts the same shape Docker does, and must never ignore a field it does not know how to translate. Either behavior turns a configuration gap into a silent correctness or security bug (see [T45](../../.agent/todo.md) for a historical incident where unsupported fields were silently dropped by the containerd adapter).

### Eager Upfront Validation Contract

The conversion contract above describes the required adapter behavior; current `ValidateConfig` coverage varies by runtime:

- **Docker**: Dereferences the configuration without a nil guard, so nil input panics. It checks only `ShmSize`: non-empty values must parse as byte sizes and must not be negative (zero is accepted). GPU, restart-policy, port, and mount conversion checks occur later in container creation.
- **Nerdctl**: Rejects nil configurations, empty image names, commas and parent traversal in mount fields, parent traversal in device paths, and NUL bytes in environment entries. It does not run `parseGPUs`, `parseDockerRestartPolicy`, `nat.ParsePortSpecs`, or `buildDockerExposedPorts`.
- **Containerd**: Checks unsupported features, namespaces/security, resources, networking, mounts, and groups. It accepts `--network host` and an empty IR network value, but rejects other explicit modes, including the normal configuration default `bridge`. Users must specify `--network host` for the normal containerd configuration. Port publishing and `--oci-runtime` are also unsupported.

Callers cannot rely on all GPU, restart-policy, port, or mount checks completing before image pulling. Passing `ValidateConfig` is only a partial preflight check, not proof that creation will accept every field.

---

<!-- /coderabbit-ai-generated:v1 -->

<!-- coderabbit-ai-generated:v1 -->

<!-- coderabbit-ai-generated-section:v1:3929df4386b757193855e876322f99a577605989e86c6b804a6ab5a8ee288288 -->

## Execution Flow

```text
1. ValidateConfig(config)
           ↓
2. PullImage(config.Image, config.Pull)
           ↓
3. CreateContainer(config) -> containerID
           ↓
4. AttachContainer(containerID, stdin, stdout, stderr, readyChan)
           ↓ (readyChan closed; also check attach error)
5. StartContainer(containerID)
           ↓
6. Forward Signals & TTY Window Resizes
           ↓
7. WaitContainer(containerID) -> exitCode
           ↓
8. RemoveContainer(containerID) [if config.Remove]
```

### Basic Execution Sequence

1. **Pre-flight Validation**: Invoke `ValidateConfig` with a non-nil `ContainerConfig` for the chosen engine's available checks; further errors may arise after image pulling.
2. **Image Availability**: Call `PullImage` according to the configured pull policy.
3. **Container Creation**: Invoke `CreateContainer` with the configuration, returning a unique container ID.
4. **Scheduled Cleanup**: If `config.Remove` is true, schedule `RemoveContainer` via `defer` or context cleanups to ensure ephemeral behavior.
5. **I/O Attachment**: If `TTY` or `Interactive` is enabled, call `AttachContainer` to hook up stdin, stdout, and stderr streams.
6. **Container Startup**: Check and handle attach errors as well as readiness before calling `StartContainer`; `ready` closure alone is not proof of successful attachment.
7. **Signal & Resize Forwarders**: Spin up background routines to forward OS signals and window resizing events to the running container.
8. **Completion Wait**: Block on `WaitContainer` to await container process exit and retrieve the final exit status code.

---

<!-- /coderabbit-ai-generated:v1 -->

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
