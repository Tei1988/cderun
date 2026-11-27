# Feature Specification: Orphan Container Pruning

## Overview

`cderun` containers are designed to be ephemeral and are automatically removed upon execution completion by default (via `--remove=true`). However, unexpected host crashes, sudden power failures, uncatchable termination signals (`SIGKILL`), or system runner disconnections can prevent cleanup hooks from firing, leaving leftover stopped containers behind on the host daemon.

The `--prune` option provides a stand-alone utility mode to remove stopped containers. The scope of container discovery and cleanup depends on the configured container runtime (e.g. label-based filtering on Docker/Podman versus namespace-wide evaluation on containerd/nerdctl).

---

## Core Characteristics

- **Runtime-Specific Scope**: Pruning uses runtime-specific selection criteria. Docker and Podman restrict pruning to containers tagged with `cderun=true`, while containerd and nerdctl evaluate containers across their target namespace.
- **Stand-Alone Subcommand Bypass**: Unlike standard execution commands, invoking `cderun --prune` does not require a tool subcommand or image mapping. It is dispatched early during CLI root execution.
- **Dry-Run Preview Limitation**: Supports `--dry-run` to output a generic dry-run notification. Note that `--prune --dry-run` does not instantiate the container runtime or scan daemon resources, so it does not preview individual container IDs that would be deleted.
- **Multi-Runtime Engine Parity**: Operates across Docker, Podman, containerd, and nerdctl with engine-specific identification mechanisms.

---

## Runtime-Specific Identification & Cleanup Mechanics

Each container engine employs distinct container tagging and lifecycle management models. `cderun` abstracts these differences through the `ContainerRuntime.PruneContainers` interface method:

```go
PruneContainers(ctx context.Context) ([]string, error)
```

### 1. Docker & Podman (API-Based Runtimes)

- **Labeling Mechanism**: Every container created by `cderun` is tagged with the label `cderun=true` during creation.
- **Discovery**: Containers are queried via daemon client listing with label filter `label=cderun=true` and `All: true`.
- **Active State Protection**: Container state (`c.State`) is checked. If the state indicates active execution (such as `running`, `paused`, or `restarting`), the container is logged at `Debug` level and skipped.
- **Deletion**: Stopped containers matching the label are deleted via `ContainerRemove` with `RemoveVolumes: true` (ensuring anonymous volumes do not leak) and `Force: true`. Note that because `Force: true` is supplied to ensure removal of stubborn stopped containers, if an un-pruned container unexpectedly transitions to a running state in the interval between discovery and removal, it may be terminated. Any `404 Not Found` or `409 Conflict` errors during individual deletions are treated as ignorable lifecycle races.

### 2. containerd (Namespace Isolation)

- **Namespace Scope**: `cderun` isolates all its containerd resources under the dedicated namespace `cderun` (defaulting to `cderun`). Pruning queries all containers residing within this namespace without an additional ownership-label filter. Unrelated system containers residing in default or Kubernetes namespaces (`k8s.io`, `moby`) are unaffected, but any non-cderun container manually created inside the `cderun` namespace will be evaluated for pruning.
- **Task & Status Inspection**: For each container in the `cderun` namespace, the runtime checks whether an active task exists. If a task is present and its status is successfully queried as `Running`, `Created`, `Pausing`, or `Paused`, it is skipped.
- **Task Termination & Snapshot Cleanup**: If `task.Status` indicates the task is not running, or if retrieving `task.Status` returns an error, the runtime proceeds to delete the task using `task.Delete(cCtx, client.WithProcessKill)`. Subsequently, container deletion is executed with `WithSnapshotCleanup` to reclaim rootfs snapshots and disk space.
- **Startup Race Warning**: In containerd, there is a timing window after `CreateContainer` succeeds but before `NewTask` is executed where a newly created container has no associated task. If `PruneContainers` executes during this brief window, it treats the container as an untasked orphan and deletes the container along with its snapshot rootfs. Users are strongly advised not to run `cderun --prune` concurrently while new `cderun` containers are starting up.

### 3. nerdctl (CLI-Based Runtime)

- **Command Invocation**: Invokes `nerdctl container prune --force`.
- **Target Namespace Selection**: `cderun` does not pass a `--namespace` flag to nerdctl during execution. Nerdctl selects its target namespace from its own configuration file or the `CONTAINERD_NAMESPACE` environment variable (defaulting to namespace `default` in nerdctl v1.7.7, rather than `cderun`).
- **Pruning Scope & Created Tasks**: Because `nerdctl container prune` removes all stopped containers in the target namespace without applying an ownership label filter, pruning under nerdctl will remove all stopped containers within that namespace, not only those generated by `cderun`. Furthermore, in nerdctl v1.7.7, container pruning also removes containers whose tasks are in the `Created` state, meaning containers currently undergoing startup initialization in that namespace can be deleted alongside their snapshots.
- **Anonymous Volume Removal Warning**: Nerdctl passes `removeAnonVolumes=true` when running `container prune`. As a result, all anonymous volumes associated with pruned containers in the target namespace—including those belonging to non-cderun workloads—will be deleted.
- **Output Parsing**: Captures stdout, skipping header lines (e.g., `Deleted Containers:`), and extracts deleted container IDs.

### 4. Nested Execution Control Socket

- In nested execution environments where inner `cderun` communicates with a parent host over the Control Socket, the adapter delegates `PruneContainers` to the underlying host runtime adapter.

---

## Configuration & Precedence

Orphan container pruning can be initiated via CLI flags, environment variables, or configuration files according to the standard priority hierarchy:

| Priority | Source | Syntax | Description |
| :--- | :--- | :--- | :--- |
| **P1** | Internal Override | `--cderun-prune` | Hoisted Phase 1 internal override flag. |
| **P2** | CLI Flag | `--prune` | Standard command-line flag. |
| **P3** | Environment Variable | `CDERUN_PRUNE=true` | Shell environment variable toggle. |
| **P4** | Tool Configuration | `prune: true` | Defined per tool in `.tools.yaml`. When per-tool `prune: true` takes precedence, invoking the tool prunes containers and exits successfully without executing the requested command. |
| **P5** | Global Configuration | `prune: true` | Defined under defaults in `.cderun.yaml`. |
| **P6** | Built-in Default | `false` | Disabled by default. |

---

## Usage Scenarios

### 1. Standard Prune

```bash
cderun --prune
```

Output when stopped orphan containers are removed:

```text
Pruned 2 orphan container(s): 8a93bc7e12f0, 4b82c19f5a3d
```

Output when no orphan containers are found:

```text
No orphan containers found to prune.
```

### 2. Dry-Run Preview

Combine `--prune` with `--dry-run` to output the dry-run notification message without performing deletions or connecting to daemon deletion endpoints:

```bash
cderun --prune --dry-run
```

Output:

```text
Dry-run mode: Would prune stopped orphan containers created by cderun.
```

Unlike general `--dry-run` execution which requires a valid tool subcommand (e.g. `cderun --dry-run node app.js`), combining `--dry-run` with `--prune` does not require a subcommand lookup key. Note that candidate container IDs are not listed during dry-run preview.

---

## Error Handling & Resilience

- **Daemon Connectivity**: If the container engine daemon or socket cannot be reached during pruning, execution aborts with a descriptive error (e.g., `failed to initialize runtime: ...` or `failed to prune orphan containers: ...`).
- **Partial Failures**: When iterating over multiple stopped containers under Docker, Podman, or containerd, failure to remove a single container (for example, if another process concurrently removed it) logs a warning and continues processing remaining candidates without failing the entire batch. Under nerdctl, individual deletion failures are logged to stderr by nerdctl itself while returning exit code 0; because `cderun` discards nerdctl stderr, these partial failures may be silent and "No orphan containers found to prune." may be returned even if unpruned containers remain.

## Related Documents

- [Command-Line Options](./command-line-options.md)
- [Container Command Execution](./container-command-execution.md)
- [Direct Container Execution](./direct-container-execution.md)
- [Dry-Run Mode](./dry-run-mode.md)
