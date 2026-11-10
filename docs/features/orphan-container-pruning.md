# Feature Specification: Orphan Container Pruning

## Overview

`cderun` containers are designed to be ephemeral and are automatically removed upon execution completion by default (via `--remove=true`). However, unexpected host crashes, sudden power failures, uncatchable termination signals (`SIGKILL`), or system runner disconnections can prevent cleanup hooks from firing, leaving leftover stopped containers behind on the host daemon.

The `--prune` option provides a stand-alone utility mode to detect and remove stopped orphan containers specifically created by `cderun`, without risking the deletion of unrelated host containers or interrupting active `cderun` workloads.

---

## Core Characteristics

- **Safe By Default**: Pruning strictly targets stopped containers created by `cderun`. Active, running, or pausing containers are always skipped.
- **Stand-Alone Subcommand Bypass**: Unlike standard execution commands, invoking `cderun --prune` does not require a tool subcommand or image mapping. It is dispatched early during CLI root execution.
- **Dry-Run Preview**: Supports `--dry-run` to inspect and preview cleanup actions safely before committing deletions.
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
- **Deletion**: Stopped containers matching the label are deleted via `ContainerRemove` with `RemoveVolumes: true` (ensuring anonymous volumes do not leak) and `Force: false`. Any `404 Not Found` or `409 Conflict` errors during individual deletions are treated as ignorable lifecycle races.

### 2. containerd (Namespace Isolation)

- **Namespace Isolation**: `cderun` isolates all its containerd resources under the dedicated namespace `cderun` (defaulting to `cderun`). Unrelated system containers residing in default or Kubernetes namespaces (`k8s.io`, `moby`) are never visible to or affected by `cderun`.
- **Task & Status Inspection**: For each container in the `cderun` namespace, the runtime checks whether an active task exists. If a task is present and its status is `Running`, `Created`, `Pausing`, or `Paused`, it is skipped.
- **Task Termination & Snapshot Cleanup**: If a task is stopped or exited, the task is deleted with `WithProcessKill`, followed by container deletion using `WithSnapshotCleanup` to reclaim rootfs snapshots and disk space.

### 3. nerdctl (CLI-Based Runtime)

- **Command Invocation**: Invokes `nerdctl container prune --force` within the configured namespace context.
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

Combine `--prune` with `--dry-run` to preview the pruning action without performing deletions or connecting to daemon deletion endpoints:

```bash
cderun --prune --dry-run
```

Output:

```text
Dry-run mode: Would prune stopped orphan containers created by cderun.
```

Unlike general `--dry-run` execution which requires a valid tool subcommand (e.g. `cderun --dry-run node app.js`), combining `--dry-run` with `--prune` does not require a subcommand lookup key.

---

## Error Handling & Resilience

- **Daemon Connectivity**: If the container engine daemon or socket cannot be reached during pruning, execution aborts with a descriptive error (e.g., `failed to initialize runtime: ...` or `failed to prune orphan containers: ...`).
- **Partial Failures**: When iterating over multiple stopped containers, failure to remove a single container (for example, if another process concurrently removed it) logs a warning and continues processing remaining candidates without failing the entire batch.

## Related Documents

- [Command-Line Options](./command-line-options.md)
- [Container Command Execution](./container-command-execution.md)
- [Direct Container Execution](./direct-container-execution.md)
- [Dry-Run Mode](./dry-run-mode.md)
