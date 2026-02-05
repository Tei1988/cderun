# Nested Execution (Recursive Execution)

cderun supports running itself inside a container it has created. This is referred to as "Nested Execution" or "Recursive Execution".

## Terminology

### Base Host
The physical machine or VM where the initial `cderun` was executed. This is where the container runtime (Docker/Podman) is actually running.

### Execution Host
The environment where the current `cderun` command is running. This can be the Base Host or a container.

### Nested Level
The depth of execution.
- Level 0: Base Host
- Level 1: Container started from Level 0
- Level 2: Container started from Level 1, and so on.

## How it Works

Nested execution relies on three main features:

1.  **Binary Mounting (`--mount-cderun`)**: The `cderun` binary from the Base Host is mounted into the container (typically at `/usr/local/bin/cderun`).
2.  **Socket Mounting (`--mount-socket`)**: The container runtime socket (e.g., `/var/run/docker.sock`) is mounted into the container, allowing `cderun` inside the container to talk to the Docker/Podman daemon on the Base Host.
3.  **Context Propagation (Snapshot)**: When a container is started with nested execution enabled, `cderun` creates a "Snapshot" of its current configuration and host context, and mounts it into the container.

## Context Propagation & Snapshots

When `cderun` starts a container that might perform nested calls, it generates a snapshot directory on the host (e.g., `/tmp/cderun-snapshots-<uuid>`) and mounts it to `/run/cderun/` inside the container.

This snapshot contains:
- `.cderun.yaml`: The merged global configuration from the Execution Host.
- `.tools.yaml`: The merged tool configurations from the Execution Host.
- **Host Context**: Metadata that allows the nested `cderun` to understand its relationship to the Base Host.

### Host Context (`hostContext`)

The `.cderun.yaml` in the snapshot includes a `hostContext` section:

```yaml
hostContext:
  binPath: "/usr/local/bin/cderun"          # Location of the binary on the Base Host
  snapshotDir: "/tmp/cderun-snap-xxxx"     # Location of the snapshot dir on the Base Host
  workingDir: "/home/user/project"         # Host-side path of the current CWD
  level: 1                                 # Current nesting level
  mounts:                                  # Mapping of Base Host paths to current container paths
    - source: "/home/user/project"
      target: "/app"
      level: 1
```

## Host Path Tracking (Reverse Path Resolution)

Since the container runtime daemon runs on the Base Host, any mount `source` path provided to it must be a path that exists on the Base Host.

When `cderun` is running inside a container (Level >= 1) and wants to mount a directory (e.g., `--mount .:/src`), it must translate the container-local path (`/app`) back to the Base Host path (`/home/user/project`).

### Resolution Logic

1.  Resolve the requested path to an absolute path in the current container (e.g., `./` -> `/app`).
2.  Search `hostContext.mounts` for the best matching `target` prefix.
3.  **Priority**:
    - Higher `level` takes precedence (more recent nesting level).
    - Longer `target` path takes precedence (more specific match).
4.  Replace the `target` prefix with the corresponding `source` (Base Host path).

Example:
If `/app` is mapped to `/home/user/project` at Level 1, then a request to mount `/app/src` from within the container will be translated to `/home/user/project/src` before being sent to the container runtime.

## Configuration Discovery Priority

Inside a container, `cderun` looks for configuration files in the following order (highest priority first):

1.  Current Directory (`.cderun.yaml`, `.tools.yaml`)
2.  Parent Directories
3.  User Home (`~/.config/cderun/`)
4.  System Path (`/etc/cderun/`)
5.  **Snapshot Path (`/run/cderun/`)** - Lowest priority, providing the base configuration from the parent host.

## Challenges and Limitations

- **Filesystem Visibility**: Only paths that are already mounted from the Base Host into the current container can be successfully mounted into a nested container.
- **Socket Permissions**: The user inside the container must have permissions to access the mounted container runtime socket.
- **Cleanup**: Snapshots are created in the host's `/tmp` directory. While `cderun` attempts to clean them up on exit, unexpected termination might leave orphaned files.
