# Feature Specification: Nested Execution Control Socket

## Overview

This document specifies a `cderun`-native control plane for [Nested Execution](./nested-execution.md), replacing the current practice of mounting the underlying container engine's raw socket (e.g. `/var/run/docker.sock`) into child containers.

Instead, the `cderun` process running on the Base Host opens a **Control Socket** (`cderun.sock`) scoped to the current execution tree, mounts it into the child container, and services requests from the nested `cderun` binary itself. The nested `cderun` never dials the underlying engine's socket or links its client library; it only speaks the `cderun` Control Protocol defined below.

---

## Problem Statement

The current mechanism (`--mount-socket`, see [Nested Execution](./nested-execution.md)) has three structural limitations:

1. **Backend coupling**: The nested `cderun` binary must itself implement a full `ContainerRuntime` adapter for whatever engine is active on the Base Host. This assumes the engine exposes a socket that a Go binary can dial directly (true for Docker/Podman/containerd, per [Multi-Runtime Support](./multi-runtime-support.md)).
2. **No path for OS-native runtimes without a portable wire protocol**: Runtimes such as Apple's `container` (XPC-based) or Windows' WSL Containers (WinRT/COM-based) do not expose a socket a nested Linux container could dial at all. Under the current model, nested execution is simply unavailable for these backends.
3. **Unscoped privilege**: Mounting the raw engine socket grants the container full control over the host daemon (explicitly called out as a high-privilege operation in [Nested Execution](./nested-execution.md)). There is no mechanism to restrict what a nested invocation is allowed to request.

## Goals

- Nested `cderun` invocations no longer need to know which engine (Docker/Podman/containerd) or which OS-native runtime (Apple `container`, WSL Containers) is actually in use on the Base Host.
- Nested execution becomes possible for runtimes that have no dialable socket, as long as the Base Host adapter for that runtime exists (see [Multi-Runtime Support](./multi-runtime-support.md) for the adapter-selection side of this).
- The Base Host process can scope down what a nested request is permitted to do, rather than granting raw daemon access.

## Non-Goals

- This document does not redesign the `ContainerRuntime` interface or existing engine adapters (Docker/Podman/containerd). The Control Socket is an additional consumer of the existing `ContainerConfig` IR and `ContainerRuntime` interface described in [Direct Container Execution](./direct-container-execution.md), not a replacement for them.
- **Remote/cross-host use is explicitly out of scope**, and should not be pursued as an extension of this mechanism. The Control Socket is a Unix domain socket, local to the Base Host's filesystem/mount namespace only.

  This was considered and deliberately rejected for this iteration, not merely deferred for lack of time. The reasons are structural, not just "not implemented yet":
  - **Mount resolution changes category, not just transport.** [Reverse Path Resolution](./nested-execution.md#reverse-path-resolution) works today because it only rewrites a path string — the underlying data already exists on the Base Host's disk. Over a remote transport, the client's mount source does not exist on the remote host at all; "resolution" would have to mean transferring/materializing file content (rsync-like), not remapping a prefix. This is a different problem, not a parameterization of the existing one.
  - **Write-back semantics are undefined.** Local bind mounts are transparently read/write in both directions. A remote mount would need an explicit answer for whether container writes sync back to the client, when, and how conflicts are handled — there is no existing mechanism this could reuse.
  - **The trust boundary disappears.** The local Unix socket's security model relies on filesystem permissions and mount namespace scoping (see Security Model above). A remote listener requires real authentication and authorization of arbitrary network clients requesting code execution — a categorically larger security surface that deserves its own dedicated review, not an incidental extension of this spec.
  - **It substantially overlaps existing tools** (`docker context` over SSH, devcontainers-over-SSH). Whether `cderun` should compete in that space is a product-scope question, independent of whether the Control Socket protocol could technically carry it.

  If remote execution is revisited in the future, it should be its own spec with its own explicit trade-off analysis — not folded into the Control Socket mechanism defined here.

---

## Architecture

```text
 Base Host cderun (Level N)                    Container (Level N+1)
┌───────────────────────────────┐             ┌───────────────────────────┐
│ 1. Create per-invocation       │             │                           │
│    Control Socket              │             │  cderun (nested binary)  │
│    <snapshotDir>/cderun.sock   │─── mount ──▶│  /run/cderun/cderun.sock  │
│                                 │             │                           │
│ 2. Accept loop (goroutine)     │◀── RPC ─────│  Sends ContainerConfig    │
│    while WaitContainer() blocks│             │  over Control Protocol   │
│                                 │             │                           │
│ 3. Dispatch to active adapter  │             │                           │
│    (Docker / Podman /          │             │                           │
│     containerd / sidecar)      │             │                           │
│                                 │             │                           │
│ 4. Teardown socket on exit     │             │                           │
│    (success, error, or signal) │             │                           │
└───────────────────────────────┘             └───────────────────────────┘
```

### No Persistent Daemon

The Base Host `cderun` process already blocks on `WaitContainer` for the full lifetime of the execution tree it spawned (see [Direct Container Execution](./direct-container-execution.md), Execution Flow step 6). The Control Socket accept loop runs as a goroutine for exactly that same lifetime — no separate long-running system service, autostart mechanism, or lifecycle management is introduced.

### Placement in the Snapshot Directory

The Control Socket is created inside the existing per-invocation snapshot directory (see [Nested Execution](./nested-execution.md), Snapshot Creation Sequence), alongside `.cderun.yaml` and `.tools.yaml`:

```text
<snapshotDir>/
  .cderun.yaml
  .tools.yaml
  cderun.sock       # new: Control Socket
```

Its path on the Base Host is recorded in `hostContext` and propagated to the nested `cderun` the same way `hostContext.binPath` and `hostContext.mounts` already are.

```yaml
hostContext:
  binPath: "/usr/local/bin/cderun"
  snapshotDir: "/tmp/cderun-snap-xxxx"
  controlSocket: "/tmp/cderun-snap-xxxx/cderun.sock"   # new field
  level: 1
```

### Cleanup

The socket file and accept-loop goroutine must be torn down through the same deferred cleanup path already used for the snapshot directory. **This is the same class of bug as [T41](../../.agent/todo.md) (snapshot temp directory leaking via `os.Exit`)** — any early-exit path (fatal error, `os.Exit`, forced signal termination) must close the listener and remove the socket file, not just the happy path.

---

## Control Protocol

### Design Principle

The protocol is a direct network projection of the existing `ContainerRuntime` interface. It does not introduce a new abstraction — it exposes the same `ContainerConfig` IR and the same lifecycle methods (`CreateContainer`, `StartContainer`, `AttachContainer`, `WaitContainer`, `RemoveContainer`, `SignalContainer`, `ResizeContainerTTY`) that the Base Host adapter already implements, per [Direct Container Execution](./direct-container-execution.md).

### Framing

Length-prefixed JSON messages over the Unix domain socket:

```text
[4-byte big-endian length][JSON payload]
```

JSON is chosen over gRPC/protobuf for a first implementation because the channel is always local (single machine, single mount namespace) and framing simplicity aids debuggability (`--diagnosis` / `--log-level trace` should be able to dump raw frames). Switching the wire format later is an internal detail as long as the version-negotiation contract below is honored.

### Request/Response Shape

- The connection opens with a version handshake (see Version Compatibility Contract below) before any other traffic.
- Requests carry: method name and a `ContainerConfig` payload (or lifecycle identifiers for non-create calls).
- `AttachContainer` uses the same multiplexed `stdcopy`-style stream framing already used for Docker/Podman attach, layered over the same connection.
- Responses carry: status (success/error) and method-specific results (container ID, exit code, error detail).
- **The request schema has no engine/adapter-selection field.** Which `ContainerRuntime` implementation serves a given Control Socket is decided once by the Base Host at socket-creation time (see Security Model). This is not a validated-and-rejected input — the field structurally does not exist, so there is nothing for a nested request to set or for a future change to accidentally start honoring.

### Version Negotiation

Unlike the current model, which piggybacks on the Docker Engine API's own version negotiation, the Control Protocol is a compatibility surface `cderun` fully owns. This is a new contract, separate from (and unrelated to) the `ContainerConfig` field conversion rules in [Direct Container Execution](./direct-container-execution.md) — that one governs turning IR fields into a runtime's native struct; this one governs whether a nested `cderun` binary is allowed to talk to a given Base Host server at all.

### Version Compatibility Contract

- The connection begins with a single handshake message, sent before any `ContainerConfig` traffic: the nested `cderun` declares its Control Protocol version; the Base Host server responds with either acceptance or an explicit rejection carrying both the client's and the server's version.
- **A version the server does not recognize as compatible must fail the handshake outright.** The server must not attempt to interpret, partially process, or best-effort-downgrade a request carried over a connection that failed the handshake.
- **No per-request re-negotiation.** Once a connection's handshake succeeds, every request on that connection is assumed to match the negotiated version; the server does not need to re-validate the version field on every subsequent message.

#### Protocol Version Is Decoupled From `cderun` Release Version

Compatibility is judged strictly on the **Control Protocol version**, not on the nested and Base Host `cderun` binaries sharing the same release version. Two binaries of different releases that both speak Control Protocol v1 are expected to interoperate. This mirrors the Docker Engine API's own version negotiation (see [Multi-Runtime Support](./multi-runtime-support.md)) and is a deliberate choice against requiring binary lockstep, for two reasons:

- **Cross-compiled mounting already breaks lockstep in practice.** [Nested Execution](./nested-execution.md)'s macOS constraints already require mounting a separately cross-compiled Linux binary via `--mount-cderun-path`. Requiring that binary's release version to exactly match the Base Host on every invocation would make routine `cderun` upgrades (including changes unrelated to nested execution) break existing nested workflows until the mounted binary is rebuilt.
- **Multi-level nesting compounds the problem.** A binary-lockstep requirement would need every level (0, 1, 2, ...) to carry the exact same release version; a protocol-version requirement only needs each level to speak a mutually supported Control Protocol version.

This decoupling shifts the burden onto keeping the Control Protocol's definition of "compatible" honest:

- **The protocol version must be bumped for any change that could alter nested-invocation behavior**, not only for changes to the wire message schema. A change that leaves the request/response shape untouched but alters what a nested request is permitted to do, or how a method behaves, is a protocol-breaking change and must bump the version.
- **Binary version skew is logged, not hidden.** The handshake response includes both sides' full `cderun` release version (not just the protocol version) in the Base Host's structured/trace logs, so that a behavioral discrepancy between a same-protocol-version but different-release nested/host pair remains diagnosable after the fact, even though it is not blocked.
- Compatibility is judged by the server against a fixed set of versions it explicitly supports (an allow-list), not by a heuristic (e.g. "same major version assumed compatible") — new versions must be added to that set deliberately as the protocol evolves.
- A rejected handshake is a normal, expected outcome (e.g. an old nested binary against a newer Base Host after a `cderun` upgrade) and must produce an actionable error message telling the user to re-run `--mount-cderun-path` with a matching binary, not a generic connection failure.

---

## Security Model

The Base Host `cderun` process is a policy enforcement point, not a transparent proxy:

- A nested request's effective `ContainerConfig` (image, mounts, capabilities, privileged mode) is capped by the **parent's own granted configuration** — a nested invocation cannot request broader privileges than the container it is running inside was itself given.
- This directly addresses the security warning already present in [Nested Execution](./nested-execution.md) regarding unscoped raw socket mounts (particularly rootful Docker/containerd sockets).
- Exact policy rules (allow-list vs. inherited-ceiling model) are an implementation detail to be finalized against `docs/features/security-validations.md` conventions; this document only fixes the requirement that *some* enforcement point must exist, where today there is none.

### Engine/Adapter Selection Is Fixed at the Base Host

Which `ContainerRuntime` adapter serves a given Control Socket is a Base Host configuration decision made once, at socket-creation time — never a property of an individual request. A nested invocation has no way to request a different engine than whatever the Base Host is already running (e.g. it cannot ask a Docker-backed Control Socket to route a request to Podman instead). As stated under Request/Response Shape, this is enforced by the request schema simply not containing an engine-selection field, not by validating and rejecting one. This closes off engine-switching as an escalation path in its own right (a nested caller cannot pick a specific adapter it believes is weaker or exploitable), independent of the CLI-argument-injection concerns addressed below.

### CLI-Based Adapters Require an Argument Injection Defense

A CLI-based adapter (`nerdctl`, a future `docker-compat-cli`, or a CLI-wrapper implementation of Apple `container` / WSL Containers) turns `ContainerConfig` fields into a literal argv for a subprocess. This step is exposed to **argument injection** (CWE-88), which is a distinct problem from shell injection:

- **Shell injection** happens when a command line is built as a single string and handed to a shell (`sh -c "..."`) for re-interpretation. cderun already avoids this everywhere by invoking subprocesses with an argv slice (e.g. Go's `exec.Command(name, arg1, arg2, ...)`), never through a shell — no special characters are re-parsed, because there is no shell involved.
- **Argument injection** happens even with a shell-free `exec.Command` call, because it does not depend on the shell at all — it depends on how the *target program's own* flag parser classifies each argv token it receives. Concretely: if a nested request sets `Image` to the string `--privileged` instead of `alpine:latest`, and the adapter builds `exec.Command("nerdctl", "run", "--privileged")`, the OS delivers exactly that argv to `nerdctl` — nothing is corrupted in transit — but `nerdctl`'s own argument parser (cobra/pflag) sees a token matching a known flag name and turns on privileged mode, even though the nested `ContainerConfig` never set `Privileged: true` anywhere. Avoiding a shell does not prevent this, because no shell is involved in either version of the call.

Note: [T81](../../.agent/todo.md) deliberately gave `--` no special role in cderun's *own* `--cderun-*` flag hoisting — but that is an unrelated design question (whether a user-supplied `--` should disable hoisting in cderun's own preprocessing). It says nothing about whether cderun should *emit* a `--` marker when constructing a downstream CLI invocation, which is the defense below.

**Construction technique**: a CLI-based adapter builds its argv using two conventions:

1. **Insert `--` before the trailing positional block** (image, command, args) — e.g. `nerdctl run [flags...] -- <image> <cmd> <arg...>`. Because image/command/args are already trailing positional values in the Docker-compatible CLI grammar, one `--` covers all of them at once.
2. **Use joined `--flag=value` form** for anything passed as a flag's value (env vars, labels, etc.), never `--flag` and `value` as two separate argv tokens — this prevents the value from being split off and reinterpreted as a new flag, regardless of its content.

**Primary verification — a structural self-check on the constructed argv, not trust in the construction code**: immediately before executing, the adapter asserts that the argv segment *before* `--` matches cderun's own intended, fixed flag pattern for that call **exactly** — no extra, missing, or reordered tokens. This is stronger than trusting that (1) and (2) were applied correctly everywhere: it catches the outcome of *any* construction mistake (a forgotten `--`, an unjoined flag pair, a value that leaked into the wrong segment) as a single, generic, hard-to-forget check, because it validates cderun's own output against cderun's own expectation — it does not need to know or trust anything about the target CLI's parsing behavior at all. If the check fails, the adapter must refuse to execute and return an explicit error, never execute the mismatched argv "to be safe."

**This self-check does not, by itself, prove the target CLI treats the post-`--` segment as purely positional.** Whether `--` means "stop all flag parsing" is a property of *the specific parsing library the target CLI is built on* — it is not a universal OS or POSIX guarantee. `nerdctl` is built on Go's `cobra`/`pflag`, the same foundation as `kubectl` (which already uses exactly this `--` pattern for `kubectl exec POD -- CMD ARGS...`) and the Docker CLI itself, where this behavior is long-standing and well-tested. That does **not** transfer to a different CLI built on a different library — a future CLI-wrapper adapter for Apple `container` (built on Swift's `ArgumentParser`) or `wslc.exe` (a distinct, unverified parser) must not assume `--` behaves the same way just because it worked for `nerdctl`.

Requirements:

- The construction technique and the structural self-check are implemented as a single shared, adapter-agnostic helper, not reimplemented ad hoc per CLI-based adapter.
- For **every** CLI-based adapter, an adversarial test suite must execute the *actual target binary* (not just unit-test cderun's own argv-construction code) with flag-like strings (e.g. `--privileged`, `--mount=...`) smuggled through every nested-overridable field, and assert the real process observably did not gain the injected behavior. Passing this suite for `nerdctl` must never be treated as evidence that a different CLI-based adapter is also safe — each adapter earns its own pass independently, precisely because `--` support is library-dependent, not universal.
- Applied in combination with, not instead of, [Engine/Adapter Selection Is Fixed at the Base Host](#engineadapter-selection-is-fixed-at-the-base-host) and the inherited-ceiling privilege scoping above — these defenses close the argument-injection vector specifically; they are not a substitute for capping what a nested request may request in the first place.

If a CLI-based adapter does not implement the construction technique, the structural self-check, and pass its own adversarial test suite against the real binary, the Control Socket must fail to start (or refuse the mount) with an explicit, actionable error rather than silently falling back to raw socket mounting or allowing degraded/unsafe dispatch.

**This applies equally to `nerdctl` and to a CLI-wrapper implementation of Apple `container` (`container`) or WSL Containers (`wslc.exe`).** A sidecar-backed adapter (linking `apple/containerization` or `Microsoft.WSL.Containers` directly and exposing a structured socket protocol instead of a subprocess argv) does not need this defense at all, since it has no argv-construction step — but a CLI-wrapper implementation of the same runtime remains a valid, cheaper option for Nested Execution support as long as it independently earns its own adversarial-test pass.

---

## Relationship to Multi-Runtime Support

Because nested `cderun` only ever speaks the Control Protocol, the Base Host adapter behind the socket can be any `ContainerRuntime` implementation that is either API-based, or CLI-based with the argument injection defense described above (see [CLI-Based Adapters Require an Argument Injection Defense](#cli-based-adapters-require-an-argument-injection-defense)). See [Multi-Runtime Support](./multi-runtime-support.md) for adapter selection; this document only defines the transport nested invocations use to reach whichever adapter is active.

For future adapters targeting OS-native runtimes with no portable wire protocol (Apple `container`, WSL Containers): **the chosen strategy is a CLI wrapper** around `container` / `wslc.exe`, implementing the full argument injection defense (construction technique, structural self-check, and an independent adversarial-test pass against that specific binary — see the library-dependence note above), not a sidecar. A sidecar (a native helper process linking `apple/containerization` or `Microsoft.WSL.Containers` directly, needing no argv-construction defense at all) was considered and is deliberately not adopted — maintaining a separate native toolchain, build, and release pipeline per platform (Swift on macOS with its own signing/notarization concerns, .NET/C++ on Windows) is a materially higher ongoing cost than a CLI wrapper for equivalent functionality, and the argument injection defense already makes the CLI-wrapper path safe enough for Control Socket dispatch. This record is kept (rather than deleted) for the same reason the Non-Goals section above keeps its rejected-remote-execution reasoning: if a future OS-native runtime ships with no CLI at all — only a native library — the sidecar strategy becomes the only option, and this reasoning should not need to be rediscovered from scratch.

---

## Compatibility and Migration

This is an additive, opt-in mechanism, not a replacement of `--mount-socket` in its first iteration:

- A new flag, `--mount-cderun-socket`, enables the Control Socket mount. `--mount-cderun` does **not** imply it automatically at first (unlike `--mount-socket`, which it does imply today).
- The existing `--mount-socket` / raw engine socket behavior remains the default until the Control Socket path reaches feature parity (all `ContainerRuntime` methods, all engines, macOS VM constraints re-verified).
- Promoting the Control Socket to the default (and deprecating raw socket mounting) is a separate, later decision gated on that parity, and would follow the standard breaking-change process (major version, changelog, migration notes) — not part of this spec.

### Experimental Status During Rollout

`--mount-cderun-socket` is documented as **experimental** for the full duration of the rollout phases below. This is a deliberate scoping choice: it means intermediate phases are not required to guarantee stable, final-shape behavior for every combination of flag and engine — a phase can land with partial coverage (e.g. Docker only, non-interactive only) without that partial state itself being a compatibility promise.

Two things stay non-negotiable even while experimental, consistent with this project's general stance against silent/undiagnosable behavior:

- **Which path actually ran must always be observable.** Whether a given invocation used the Control Socket or fell back to the raw engine socket must be visible via `--diagnosis` and/or debug-level logs. "Experimental" relaxes the stability guarantee, not the diagnosability one.
- **Graduation is an explicit, later decision**, not a fade-out. The flag stops being documented as experimental only once the Rollout Phases below are complete and parity is confirmed across engines — see Compatibility and Migration above.

### Rollout Phases

Mirroring the phased rollout already used for the original CRI work (see [Direct Container Execution](./direct-container-execution.md)'s Roadmap), implementation is split into sequential, independently mergeable phases rather than a single large change:

1. **Phase 1 — Protocol and Socket Plumbing**: Control Socket creation/mount/cleanup, the handshake and Version Compatibility Contract. `--mount-cderun-socket` exists but does not yet change container execution behavior.
2. **Phase 2 — Docker Dispatch (Non-Interactive)**: `CreateContainer` / `StartContainer` / `WaitContainer` / `RemoveContainer` routed over the Control Socket to the Docker adapter. First phase where the mechanism is actually functional, for non-TTY/non-interactive execution.
3. **Phase 3 — Interactive I/O (Attach, Signal, Resize)**: stdin/stdout/stderr multiplexing, signal forwarding, and TTY resize over the Control Socket. Kept separate from Phase 2 because this is historically where the most subtle bugs have occurred (see T43, T52, T61, T62).
4. **Phase 4 — `nerdctl` Dispatch (Non-Interactive)**: the argument-injection construction technique, structural self-check, and adversarial test suite from [CLI-Based Adapters Require an Argument Injection Defense](#cli-based-adapters-require-an-argument-injection-defense), applied to route non-interactive dispatch through `nerdctl`. Deliberately scheduled right after Phase 2, not deferred to the end — a CLI-based engine choice should not leave a user without Nested Execution for the entire rollout.
5. **Phase 5 — Remaining API-Based Engines, Security Policy, macOS Verification**: `containerd-api` dispatch (in addition to the `docker-compat-api` family covering Docker/Podman), the inherited-ceiling privilege scoping described under Security Model, and re-verification of the macOS VM constraints from [Nested Execution](./nested-execution.md).

Apple `container` / WSL Containers adapters (see [Relationship to Multi-Runtime Support](#relationship-to-multi-runtime-support)) depend on at least Phase 2, and realistically Phase 3, being complete — they are not part of this rollout and are tracked separately once a working Control Socket exists to build on. When undertaken, they follow the Phase 4 pattern (CLI-wrapper dispatch, scheduled promptly rather than deferred), each earning its own independent adversarial-test pass per the library-dependence note above.

## Open Questions for Implementation

- Whether a single execution tree needs one Control Socket per nested level or one shared socket reused across the whole tree (current lean: one per level, mirroring the existing per-invocation snapshot directory).
- Whether wire format should move to protobuf/gRPC once the method surface stabilizes.
- Exact shape of the security policy (allow-list vs. inherited-ceiling) referenced above.
