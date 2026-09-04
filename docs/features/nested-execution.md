# Feature Specification: Nested Execution (Recursive Containers)

`cderun` supports running itself inside a container recursively. This capability is referred to as "Nested Execution" or "Recursive Execution".

> **Evolution in progress**: This document describes the current mechanism, which mounts the underlying container engine's raw socket (Mechanism 2 below) into the child. See [Nested Execution Control Socket](./nested-execution-control-socket.md) for the planned `cderun`-native control plane that will replace this, enabling runtime-agnostic and scoped nested invocations.

---

## Terminology

### Base Host (Level 0 Host)

The physical machine or VM where the initial `cderun` command is executed. The actual container runtime engine (Docker/Podman/containerd) resides and operates here.

### Execution Host

The environment where the current `cderun` command is executing. This can be either the Base Host (Level 0) or an active container (Level 1+).

### Nested Level

The depth of the recursive container execution tree:

- **Level 0**: The Base Host.
- **Level 1**: A container spawned directly from the Level 0 host.
- **Level 2**: A container spawned from within a Level 1 container, and so on.

---

## Architectural Mechanisms

Nested execution relies on three fundamental mechanisms:

1. **Binary Mounting (`--mount-cderun`, `--mount-cderun-path`, `--mount-tools`, `--mount-all-tools`)**:
   The host's `cderun` binary is mounted into the container (typically at `/usr/local/bin/cderun`).
   - Specifying `--mount-tools` or `--mount-all-tools` automatically enables `--mount-cderun`.
   - `--mount-cderun-path` allows explicitly selecting which host-side binary is mounted.

2. **Socket Mounting (`--mount-socket`)**:
   The container runtime socket (e.g., `/var/run/docker.sock`) is mounted inside the container, giving the nested `cderun` direct access to the container engine on the Base Host.
   - Enabling `--mount-cderun` automatically enables `--mount-socket` unless explicitly deactivated.

3. **Context Propagation (Snapshotting)**:
   When spawning a nested-enabled container, `cderun` creates an execution configuration "snapshot" on the host and mounts it into the container at `/run/cderun/` to propagate execution settings.

---

## Context Propagation and Snapshot Workflow

When a container that supports nested execution is started, `cderun` generates a temporary snapshot directory and mounts it inside the container.

### Snapshot Base Directory

The directory that holds the snapshot is selected per Execution Host:

- **Base Host (Level 0)**: the temp directory reported by the environment (`TMPDIR`, or the platform default) is used as-is. The snapshot path is handed to the container runtime as a bind mount source, so it must be a path the runtime can share. On macOS the per-user directory in `TMPDIR` (`/var/folders/...`) is shared with the runtime VM, while `/tmp` is not.
- **Inside a container (Level 1 or deeper)**: the value is normalized to `/tmp`. Images and package managers point `TMPDIR` at locations such as `/root/tmp` or `node_modules/.tmp`, which do not reverse-resolve to a usable Base Host path and make the nested bind mount fail.

A relative `TMPDIR` is normalized to `/tmp` at any level, since it would otherwise place the snapshot under the current working directory.

### Snapshot Creation Sequence

The snapshot creation sequence distinguishes between the path on the current Execution Host (where files are written) and the path on the Base Host (used as the mount source for the container runtime daemon).

```text
                  [Snapshot Creation Pipeline]

              Generate Unique Snapshot ID (UUID)
                              │
                              ▼
                  Construct HostContext Object
                              │
                              ▼
            Record Current Mount Mappings to HostContext
                              │
                              ▼
         Detect OverlayFS Upperdir & Append to Mounts
                              │
                              ▼
        Create Snapshot Directory on Execution Host (0700)
                              │
                              ▼
                 Is Nested Level >= 2?
                ├── [Yes] ──> Apply Reverse Path Resolution
                │             to derive Base Host path
                └── [No]  ──> Base Host path is identical to
                              Execution Host path
                              │
                              ▼
          Populate HostContext.SnapshotDir with Base Path
                              │
                              ▼
         Write config files (.cderun.yaml, .tools.yaml) (0600)
                              │
                              ▼
         Return Execution Host path and Base Host path
```

#### Directory and File Permissions

To prevent unauthorized access and credential leakage in multi-user environments, snapshots are protected with strict POSIX permissions:

- **Snapshot Directory**: `0700` (Owner read, write, and execute only).
- **Configuration Files**: `0600` (Owner read and write only).

### Snapshot Directory Contents

The generated snapshot directory contains:

- `.cderun.yaml`: Evaluated and merged global configuration.
- `.tools.yaml`: Evaluated and merged tool configuration mappings.
- **Host Context (`hostContext`)**: Metadata enabling the nested engine to map its local paths back to physical host paths.

#### Host Context Specimen

```yaml
hostContext:
  binPath: "/usr/local/bin/cderun"          # Location of the binary on the Base Host
  snapshotDir: "/tmp/cderun-snap-xxxx"     # Location of the snapshot on the Base Host
  workingDir: "/home/user/project"         # Current working directory on the Base Host
  level: 1                                 # Current execution nest level
  mounts:                                  # Path mapping table
    - source: "/home/user/project"
      target: "/app"
      level: 1
```

---

## Reverse Path Resolution

Because the container runtime daemon runs on the **Base Host** (Level 0), any directories specified as bind mount sources by a nested container (e.g., `--mount .:/src` running inside a Level 1 container) must be translated to paths that are physically accessible on the Base Host.

`cderun` implements **Reverse Path Resolution** to map container-local paths back to original host paths.

### Resolution Steps

1. **Verify Nest Level**: Checks if `Level >= 1`. If execution is on the Base Host (Level 0), path translation is skipped.
2. **Absolute Path Conversion**: Converts the requested mount source (such as `./src` or `/app/src`) to an absolute path on the current Execution Host.
3. **Lookup Match**: Scans the `hostContext.mounts` table, matching the path against the recorded `target` prefixes.
4. **Precedence Rules**:
   - **Longest Match (Longest Target Prefix)**: If multiple mappings match (e.g., `/app` and `/app/src`), the longest matched `target` prefix is selected to favor more specific mounts over generic ones.
   - **Deepest Level Priority**: If targets are of equal length, the mapping with the highest `level` (the most recent nested mount) is selected.
5. **Path Construction**: Replaces the matching `target` prefix with its corresponding `source` path from the Base Host, and appends the remaining relative path segments.

### Concrete Example

1. **Level 0 (Base Host)**: User executes `cderun --mount type=bind,source=.,target=/app node`.
   - Host Path: `/home/user/project`
   - Container Path (L1): `/app`
2. **Level 1 (Container)**: Nested execution runs `cderun --mount type=bind,source=./src,target=/src go build`.
   - Requested Path: `./src`
   - Absolute Container Path (L1): `/app/src`
   - **Reverse Translation**:
     - Scans `hostContext.mounts`, matching `/app/src` against the target prefix `/app`.
     - Replaces `/app` with the host source `/home/user/project`.
     - Output host path: `/home/user/project/src`.
3. **Runtime Spawning**: Instructs the Docker daemon to mount `/home/user/project/src` to `/src` inside the new Level 2 container.

---

## Automated Host Root Detection via OverlayFS

If `cderun` detects that it is executing inside a container with an OverlayFS root filesystem, it automatically parses `/proc/self/mountinfo` to extract the host-side `upperdir` path.

It then appends a fallback root mapping (`source: <upperdir>, target: /`) to `hostContext.mounts`. This allows mounting files that reside in the container's scratch space (such as files created in `/tmp`) into nested containers, even if those paths do not belong to a pre-existing volume or bind mount.

For more information, see the [/proc/self/mountinfo Specification](../references/proc-self-mountinfo.md).

---

## macOS Nested Execution Constraints

Running nested containers on macOS requires additional configurations because container engines run inside a Linux virtual machine:

```text
  macOS Host (Darwin)                   Linux VM (Docker / Podman)
 ┌─────────────────────────┐           ┌─────────────────────────────┐
 │ Compile linux binary:   │           │ Container Environment       │
 │ GOOS=linux GOARCH=<arch>│ ────────> │                             │
 │  --mount-cderun-path    │           │ Runs: cderun (Linux)        │
 │ (e.g. arm64 or amd64)   │           │ Writes: /tmp/cderun-snap-...│
 │                         │           │                             │
 │ Socket:                 │           │ Socket Mounted:             │
 │ /var/run/docker.sock    │ ────────> │ /var/run/docker.sock        │
 │ (GID may differ on host)│           │ (Requires <VM_GID>/groupAdd)│
 └─────────────────────────┘           └─────────────────────────────┘
```

### 1. Cross-Compilation Requirements

Because macOS uses the Darwin kernel, the host's `cderun` binary cannot execute inside a Linux container.

- **Solution**: Compile a Linux binary of `cderun` (`GOOS=linux GOARCH=amd64` or `GOOS=linux GOARCH=arm64`) and specify its location using `--mount-cderun-path` to mount it into the container.

### 2. VM Socket GID Authorization

The Group ID (GID) of the container socket inside the macOS Linux VM might not match the user's GID inside the container.

- **Solution**: Find the numeric GID of the socket inside the Linux VM and pass it using the `--cderun-group-add` flag (e.g., `--cderun-group-add=102`) or via the `groupAdd` array in YAML configurations. This grants the container user the necessary permissions to access the socket.

> **⚠️ Security Warning**:
> Sharing the container runtime socket (especially with rootful Docker, Podman, or containerd) grants broad control over the host daemon, representing a high privilege operation. It can lead to container escape or host configuration exposures. This capability should only be shared with trusted container workloads.
>
> In contrast, sharing a rootless Podman socket restricts the impact strictly to the user-scoped daemon instance, significantly reducing the security blast radius compared to a rootful engine.
