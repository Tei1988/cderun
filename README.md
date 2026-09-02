# cderun

## Concept

> "All you need on your local machine is Docker, Podman, or containerd."
> `cderun` generates ephemeral containers for commands like `node`, `python`,
> or `git` on demand using container runtimes (Docker/Podman/containerd). It keeps your
> host clean and ensures reproducible environments defined in a single YAML file.

```text
Host Environment                 Ephemeral Container
┌─────────────────────────┐      ┌─────────────────────────┐
│  $ node app.js          │ ───> │  [cderun]               │
│                         │      │  Running node:20-alpine │
│  (No Node.js installed  │      │  Mounts: . -> /app      │
│   on host machine)      │      │  I/O: Synchronized      │
└─────────────────────────┘      └─────────────────────────┘
```

### Core Architecture Overview

When you invoke `cderun`, the execution flows through a unified pipeline to translate user commands into secure, ephemeral containers. Below is the high-level architecture of `cderun`:

```text
 ┌─────────────────────────────────────────────────────────┐
 │                      User Command                       │
 │      e.g., cderun node app.js --cderun-image node:20     │
 └───────────────────────────┬─────────────────────────────┘
                             │
                             ▼
 ┌─────────────────────────────────────────────────────────┐
 │             1. Argument Preprocessing                   │
 │   - Detect subcommand boundaries                        │
 │   - Extract and hoist "--cderun-*" P1 override flags    │
 └───────────────────────────┬─────────────────────────────┘
                             │
                             ▼
 ┌─────────────────────────────────────────────────────────┐
 │             2. Configuration Merging                    │
 │   - Load config layers from P1 down to P6 defaults      │
 │   - Unify CLI flags, environment variables, and YAMLs   │
 └───────────────────────────┬─────────────────────────────┘
                             │
                             ▼
 ┌─────────────────────────────────────────────────────────┐
 │              3. Value Resolution Engine                 │
 │   - Parse expressions like {{HOME}} or {{file:...}}      │
 │   - Perform strict anchor boundary security validation  │
 └───────────────────────────┬─────────────────────────────┘
                             │
                             ▼
 ┌─────────────────────────────────────────────────────────┐
 │             4. Runtime Adapter Selection                │
 │   - Probe active sockets (Docker / containerd / Podman) │
 │   - Enforce runtime capability validations              │
 └───────────────────────────┬─────────────────────────────┘
                             │
                             ▼
 ┌─────────────────────────────────────────────────────────┐
 │             5. Container Life Cycle & I/O               │
 │   - Perform safe image pulls with exponential backoffs  │
 │   - Synchronize STDIN & capture exit codes gracefully   │
 └─────────────────────────────────────────────────────────┘
```

---

## Usage

`cderun` supports four primary modes of operation:

### 1. Wrapper Mode

Explicitly call `cderun` followed by the subcommand you want to run.

```bash
cderun [cderun-flags] <subcommand> [passthrough-args]
```

Example:

```bash
cderun --tty node --version
```

### 2. Symlink Mode (Polyglot Entry Point)

Create a symlink to `cderun` with the name of the tool you want to wrap.
`cderun` will automatically detect the tool name from the executable name.

```bash
ln -s cderun node
./node --version  # Effectively runs 'cderun node --version'
```

### 3. Ad-hoc Mode

You can use `cderun` to run arbitrary commands in a containerized environment
by specifying the image. Since the first non-flag argument is always consumed
as a subcommand (lookup key), you must explicitly specify the entrypoint
if you want to execute a command that is not the default entrypoint of the image.

```bash
cderun --image=alpine --entrypoint=ls ls -l
```

### 4. Diagnosis Mode

Run `cderun` with the `--diagnosis` flag to see system diagnostics
and available tools. This mode does not require a subcommand.

```bash
cderun --diagnosis
```

---

## Argument Parsing & Flags

`cderun` uses a strict boundary for argument parsing. The first non-flag argument is considered the **subcommand**. This subcommand acts as a lookup key for configuration.

- **[cderun-flags]**: Arguments before the subcommand are parsed as `cderun` flags.
- **\<subcommand\>**: The lookup key (e.g., `node`, `python`). It is NOT included in the final container command by default.
- **[passthrough-args]**: All arguments after the subcommand are passed directly to the container.

The final command executed in the container consists of the passthrough-args.

### Illustration

```text
cderun --tty docker --tty
  |      |     |      |
  |      |     |      +-- Passthrough argument (passed to docker)
  |      |     +--------- Subcommand
  |      +--------------- cderun flag (TTY: true)
  +---------------------- cderun command
```

### P1 Internal Overrides & Hoisting Mechanics

Flags prefixed with `--cderun-` are **"Internal Overrides" (P1)**. They have the highest priority in the resolution hierarchy.

| Priority | Level | Source | Description |
| :--- | :--- | :--- | :--- |
| **Highest** | **P1** | **Internal Overrides** | `--cderun-*` flags (placed after subcommand) |
| | **P2** | **CLI Flags** | Standard flags (placed before subcommand) |
| | **P3** | **Env Vars** | `CDERUN_*` environment variables |
| | **P4** | **Tool Config** | `.tools.yaml` specific tool settings |
| | **P5** | **Global Config** | `.cderun.yaml` default settings |
| **Lowest** | **P6** | **Defaults** | Hardcoded internal defaults |

In standard **Wrapper Mode**, these flags **must** be placed **after** the subcommand. `cderun` performs a "Hoisting" operation during preprocessing, moving these flags before the subcommand internally so they are parsed as `cderun` settings rather than being passed to the container.

```bash
# Standard Wrapper Mode: P1 flags go AFTER the subcommand
cderun node app.js --cderun-image node:20-alpine

# WRONG (will result in an error):
cderun --cderun-image node:20-alpine node app.js
```

#### Detailed Explanation of the Configuration Priority

The P1–P6 priority layers allow highly flexible execution setups.

- **P1 (Internal Overrides)**: Allows the caller to force-override any option dynamically, regardless of what has been configured globally or tool-wise. These flags are intercepted before subcommand execution.
- **P2 (CLI Flags)**: Represents regular options provided to `cderun` before the subcommand.
- **P3 (Env Vars)**: Enables environment-based configuration overrides on the host. List values are comma-separated or semicolon-separated (such as `CDERUN_ENV`).
- **P4 (Tool Config)**: Defined inside `.tools.yaml` for specific subcommands. This is where you configure tool-specific Docker images, mounts, and default working directories.
- **P5 (Global Config)**: Defined inside `.cderun.yaml` and contains global defaults like fallback container runtime or log level.
- **P6 (Defaults)**: Hardcoded settings within the binary itself, ensuring safe execution even without configuration files.

#### Detailed Hoisting Mechanics

Hoisting ensures that `cderun` settings do not conflict with the flags of the tool you are wrapping.

```text
                                  [Hoisting Process]
  Initial Input:
  ┌─────────────────────────────────────────────────────────────┐
  │ cderun  │  --tty  │  node  │  app.js  │ --cderun-image=node │
  └─────────────────────────────────┬───────────────────────────┘
                                    │
                                    ▼ [Extract --cderun-*]
  Hoisted Input:
  ┌─────────────────────────────────────────────────────────────┐
  │ cderun  │ --cderun-image=node │  --tty  │  node  │  app.js  │
  └─────────────────────────────────────────────────────────────┘
```

1. **Boundary Detection**: `cderun` scans for the subcommand boundary. It robustly identifies standard and persistent flags that take arguments (such as `--image`) to avoid misidentifying values as subcommands.
2. **Extraction**: It gathers all `--cderun-` prefixed flags (and their associated values) that appear *after* the subcommand.
3. **Internal Relocation**: These flags are moved before the subcommand internally before parsing begins.

#### Equals-Sign and Space-Separated Formats for Value-Taking Flags

To provide a natural, user-friendly CLI experience, internal override flags that take a value (e.g., `--cderun-image`, `--cderun-workdir`) can specify their value using either the space-separated format or the equals-sign format:

- **Space-Separated**: `--cderun-image alpine` or `--cderun-workdir /app`
- **Equals-Sign**: `--cderun-image=alpine` or `--cderun-workdir=/app`

During argument preprocessing, `cderun` looks up the registration metadata of standard options. If the encountered `--cderun-` flag is registered to expect a value and is specified without an equals-sign, the preprocessor automatically consumes the next adjacent argument as its value, hoisting both arguments together.

#### Hoisting with Boolean vs. Value-Taking Overrides

- **Boolean Override Flags**: Override flags acting as boolean toggles (such as `--cderun-read-only`, `--cderun-tty`, or `--cderun-privileged`) do not consume a separate adjacent argument and are hoisted autonomously. However, they may accept an optional inline value using the equals-sign format (e.g., `--cderun-tty=false`).
- **Value-Taking Override Flags**: Override flags expecting values (such as `--cderun-image`, `--cderun-workdir`, or `--cderun-env`) consume the subsequent adjacent parameter as their value when written in the space-separated format. If a value-taking flag is followed by another `--cderun-` flag, preprocessing rejects it with a validation error to prevent parsing corruption.

This mechanism is especially critical in **Symlink Mode (Polyglot Entry Point)**, where it allows you to configure `cderun`'s behavior (e.g., `node --cderun-tty=false`) without affecting the arguments passed to the wrapped tool (e.g., `node --version`).

#### Double-Dash (`--`) Hoisting Exemption (Not Supported)

To simplify argument parsing and avoid semantic ambiguity, `cderun` does **NOT** support double-dash (`--`) for stopping or exempting arguments from hoisting:

1. **No Delimiter Exemption**: The preprocessor scans the entire list of arguments after the subcommand and does not treat `--` as a barrier to halt the extraction of `--cderun-` flags.
2. **Always Hoisted**: Any `--cderun-` prefixed flags appearing anywhere in the argument list (even after a `--` delimiter) are always hoisted to the front. This design keeps the hoisting behavior predictable and fully independent of shell-level option interpretation.

### Available Flags

#### Execution & Identity

- `--tty`, `-t`: Allocate a pseudo-TTY. (Default: `false`)
- `--interactive`, `-i`: Keep STDIN open even if not attached. (Default: `false`)
- `--image`: Container image to use.
- `--entrypoint`: Overwrite the default ENTRYPOINT of the image.
- `--user`, `-u`: Username or UID (format: `<name|uid>[:<group|gid>]`).
- `--workdir`, `-w`: Working directory inside the container.
- `--env`, `-e`: Set environment variables (`KEY=VALUE` or `KEY` for host passthrough).
- `--strict-env`: Require all passed environment variables to be present on the host. (Default: `false`)
- `--pull`: Pull image before running (`always`, `missing`, `never`). (Default: `missing`)
- `--pull-max-retries`: Maximum number of retries for image pull (1 or greater). (Default: `3`)
- `--pull-backoff-base`: Base duration for exponential backoff during image pull (e.g. `1s`, `500ms`). (Default: `1s`)
- `--prefetch`: Prefetch specified tool images defined in `.tools.yaml`. Accepts a comma-separated list of tool names as a scalar string flag (e.g., `--prefetch node,python`). Supports template expressions (e.g., `{{env:...}}`).
- `--prefetch-all`: Prefetch all tool images defined in `.tools.yaml`. (Default: `false`)
- `--remove`: Automatically remove the container when it exits. (Default: `true`)
- `--restart`: Configure the container's restart policy when the container exits (e.g., `no`, `always`, `on-failure:5`). Configuring any non-`no` restart policy requires setting `--remove=false` due to default removal behavior and resolver constraints. Note: containerd does not support restart policies.
- `--hang-timeout`: Grace period after I/O completion before force-terminating the container (e.g. `10s`, `5s`, `0` for infinite). This applies to non-interactive or non-TTY sessions. (Default: `10s`)

#### Network & Ports

- `--network`: Connect a container to a network. (Default: `bridge`)
- `--hostname`: Container host name.
- `--publish`, `-p`: Publish a container's port(s) to the host.
- `--publish-all`, `-P`: Publish all exposed ports to random ports. (Default: `false`)
- `--expose`: Expose a port or a range of ports.
- `--dns`: Set custom DNS servers.
- `--dns-option`: Set custom DNS options (e.g., `ndots:3`). Note: containerd does not support DNS options.
- `--dns-search`: Set custom DNS search domains. Note: containerd does not support DNS search domains.
- `--add-host`: Add a custom host-to-IP mapping (`host:ip`).

#### Resources & Security

- `--memory`, `-m`: Memory limit (e.g., `512m`, `1g`).
- `--cpus`: Number of CPUs (float).
- `--cpu-shares`: Limit container CPU access weight (relative weight).
- `--cpuset-cpus`: CPUs in which to allow execution (e.g., `0-3`, `0,1`).
- `--cpuset-mems`: Memory nodes (MEMs) in which to allow execution (e.g., `0-3`, `0,1`). Only effective on NUMA systems.
- `--device`: Add a host device to the container.
- `--gpus`: GPU devices to request (e.g., `all`, `count=2`, `device=0,1`). Note: containerd does not support GPU requests.
- `--ipc`: Configure the IPC namespace mode. Accepts `"host"` or `"private"`; an empty value (`""`) uses the runtime default. Note: containerd only supports `"host"`, `"private"`, or `""` (empty).
- `--cgroupns`: Configure the cgroup namespace mode. Accepts `"host"` or `"private"`; an empty value (`""`) uses the runtime default. Note: containerd only supports `"host"`, `"private"`, or `""` (empty).
- `--sensitive-env`: List of environment variable patterns to mask. By default, **all** environment variable values are masked (Secure by Default).
- `--privileged`: Give extended privileges to this container. (Default: `false`)
- `--read-only`: Mount the container's root filesystem as read-only. Maps to `ReadonlyRootfs` in Docker host configuration and `Root.Readonly = true` in the containerd OCI spec. (Default: `false`)
- `--pid`: Configure the PID namespace for the container. Accepts `"host"` or `""` (private). Maps to `PidMode` in Docker host configuration, and appends `WithHostNamespace(specs.PIDNamespace)` to the containerd OCI spec options when configured as `"host"`. (Default: `""`)
- `--pids-limit`: Limit the maximum number of active processes/threads inside the container (fork-bomb protection).
- `--init`: Run an init process inside the container to forward signals and reap zombie processes. Maps to `Init` in the Docker host configuration, and is explicitly rejected with a validation error ('containerd runtime: init is not supported yet') in the containerd adapter's `ValidateConfig` method. (Default: `false`)
- `--security-opt`: Configure security options (such as `no-new-privileges`, `seccomp=unconfined`, and AppArmor profiles). Maps directly to `HostConfig.SecurityOpt` in the Docker host configuration, or mapped supported profiles in Podman. In containerd, empty AppArmor profiles are strictly rejected (e.g., `apparmor=` or `apparmor:` with no profile name) and valid options are applied to the OCI spec via helper `applySecurityOptions`. (Default: none)
- `--ulimit`: Configure process resource limits (ulimits) in container execution environments. Parses specifications (such as `nofile=1024:2048`) via the `go-units` standard parser. Maps directly to `HostConfig.Resources.Ulimits` in the Docker host configuration or Podman host configuration, and is converted into POSIX OCI process rlimits (`specs.POSIXRlimit` inside `Process.Rlimits`) via custom spec options in containerd. (Default: none)
- `--shm-size`: Configure the size of `/dev/shm` for containers. Maps to `ShmSize` in the Docker host configuration or Podman host configuration. In containerd, `getShmSizeSpecOpt` updates existing `tmpfs` `/dev/shm` mount `size=` options (preserving other options), appends a new `tmpfs` `/dev/shm` mount when none exists, or returns an error if an existing `/dev/shm` mount is not `tmpfs` (see [Command-Line Options Reference](docs/features/command-line-options.md#--shm-size)). (Default: none)
- `--sysctl`: Configure kernel parameters (sysctl) at runtime (format: `key=value`, e.g., `net.ipv4.ip_forward=1`). Maps directly to `Sysctls` map inside `HostConfig` in Docker/Podman, and `Linux.Sysctl` in containerd OCI spec.
- `--cap-add`: Add Linux capabilities.
- `--cap-drop`: Drop Linux capabilities.
- `--group-add`: Add supplementary groups to the container (group name or GID). Note: containerd only supports numeric GIDs.

#### Mounting & Nested Execution

- `--mount`: Attach a filesystem mount (`type=bind,source=...,target=...[,readonly][,optional]`). The `optional` parameter skips bind mounts if the source path is missing on the host.
- `--mount-socket`: Mount the container runtime socket into the container. (Default: `false`)
- `--mount-cderun`: Mount the `cderun` binary into the container. (Enables `--mount-socket` automatically)
- `--mount-cderun-socket`: Mount cderun Control Socket (`cderun.sock`) into container for nested execution (experimental, supporting Phase 1 protocol framing and Phase 2 non-interactive container lifecycle dispatch). (Default: `false`)
- `--mount-tools`: Mount specified tools (comma-separated) defined in `.tools.yaml` into the container.
- `--mount-all-tools`: Mount all tools defined in `.tools.yaml` into the container.
- `--socket-path`: Path to the runtime socket on the host.
- `--mount-socket-path`: Path where the socket should be mounted inside the container.
- `--mount-cderun-path`: Host path to the `cderun` binary to mount.

#### Diagnostics & Logging

- `--config`: Path to `cderun` config file (`.cderun.yaml`).
- `--tool-config`: Path to tools config file (`.tools.yaml`).
- `--runtime`: Container runtime to use (`docker`/`podman`/`containerd`).
- `--dry-run`: Preview container configuration without execution. (Requires a subcommand)
- `--dry-run-format`, `-f`: Output format for dry-run (`yaml`, `json`, `simple`).
- `--diagnosis`: Show system diagnostics and available tools. (No subcommand required)
- `--diagnosis-format`: Output format for diagnosis (`yaml`, `json`, `simple`).
- `--log-level`: Set log level (`error`, `warn`, `info`, `debug`, `trace`). (Note: `warning` is also accepted as an alias for `warn`. Default: `error`)
- `--log-format`: Set log format (`text`, `json`).
- `--log-timestamp`: Include timestamp in logs. (Default: `true`)

*(All flags have a corresponding `--cderun-` prefixed P1 override counterpart.)*

---

## Value Resolution & Expression Engine

`cderun` features a unified dynamic value resolution engine (`ExpressionResolver`) that evaluates string inputs, slices, and maps recursively in configuration files and CLI flags.

### 1. Recursive Value Resolution

Value resolution is applied recursively to complex configuration structures:

- **Strings**: Directly evaluated for expression expansions, tilde expansion, and relative-to-absolute path resolution.
- **Slices & Lists**: Every element is parsed and resolved recursively.
- **Maps**: Key-value pairs are evaluated dynamically to ensure nested configurations inherit proper resolved paths.

### 2. Expressions (`{{...}}`)

Expressions can be used to inject host-context or dynamic values into options like `--image`, `--env`, and `--mount`:

- **Magic Words**:
  - `{{HOME}}`: Resolves to the user's home directory path.
  - `{{PWD}}`: Resolves to the current working directory of the execution host.
  - `{{BASE_HOME}}`: Path to the home directory on the *base host* (Level 0 physical machine/VM), ensuring correct referencing even when executing recursively inside a nested container.
  - `{{BASE_PWD}}`: Path to the working directory on the *base host*.
- **Directives**:
  - `{{file:path}}`: Reads the content of a file (e.g., `{{file:.go-version}}`). Performs upward directory traversal searching, trimming trailing and leading whitespace. Limit: 1MB (`MaxDirectiveFileSize`). Supports fallbacks using `{{file:path:-default}}` when missing, stat/read error, or empty.
  - `{{find_dir:name}}`: Upwardly searches for a directory or file of the specified name and returns its absolute path (e.g., `{{find_dir:.git}}`). Supports fallbacks using `{{find_dir:name:-default}}` when missing.
  - `{{env:KEY:-default}}`: Resolves environment variables on the execution host, supporting an optional fallback default value.

#### Detailed Value Resolution and Escaping Mechanics

- **Recursive Processing**: Resolving is performed recursively. If a resolved environment variable or file contents itself contain a dynamic expression, it is evaluated sequentially to prevent unexpanded parameters.
- **Double-Brace Escaping**: To pass raw double-brace strings without triggering resolution, you can escape them by nesting them inside an outer set of double braces:
  - `{{ {{HOME}} }}` resolves to the literal string `{{HOME}}`
  - `{{{{HOME}}}}` resolves to the literal string `{{HOME}}`
  - This escaping mechanism bypasses evaluation and prevents strict resolution checks from failing on non-standard expressions.
- **Size and Traversal Safety**: The file directive strictly limits output parsing to 1MB to prevent out-of-memory states, and strictly enforces parent-directory traversal security boundaries to prevent host environment leaks.

### 3. Tilde Expansion & Relative Path Resolution

Paths starting with `~` or `~/` are expanded to the home directory. Relative paths starting with `./` or `../` are automatically resolved to absolute paths relative to:

- The configuration file's parent directory (for YAML properties).
- The current working directory (`{{PWD}}`) (for CLI flags).

### 4. Anchor Boundary Validation & Directory Traversal Prevention

To maintain strict security boundaries, any path resolved via expressions or tildes undergoes **Anchor Boundary Validation**.

- **Rule**: The finalized absolute path must remain within the boundary directory defined by the expression's anchor (e.g., `{{HOME}}` or `{{PWD}}`).
- **Directory Traversal Defense**: Parent traversals using `..` (such as `../`) are allowed within anchor-based path resolution, provided that the normalized final absolute path remains within the anchor's boundary directory (e.g., `{{HOME}}/Documents/..` resolves to `{{HOME}}` and is permitted). However, any traversals that escape the anchor's boundary directory (such as `{{HOME}}/..` or `{{HOME}}/../../etc/passwd`) will trigger an immediate validation error. Absolute paths are strictly forbidden inside local subpaths for safety.
- **Multiple Anchors Validation**: If a path contains multiple expressions or anchors (e.g., `{{HOME}}/{{PWD}}/file`), the final resolved path must simultaneously satisfy the boundary check for **all** anchors present.

### 5. String Safety & Validation Rules

To prevent string truncation and security injection attacks during environment transmission and config handling:

- **Null Byte, Control Character & Invalid UTF-8 Rejection**: Environmental keys, values, and paths are strictly scanned for null bytes (`\x00`), unescaped C0/C1 control characters, and invalid UTF-8 byte sequences. The presence of any null byte, control character, or invalid UTF-8 sequence triggers an immediate security validation error.
- **Container Target Path Safety**: Target paths in mount configurations (e.g. `mc.Target`) and destination paths in device mappings inside the container must be non-empty and absolute. Because these are container-side paths, they are NOT processed by host-side relative path resolution (e.g. `SetBaseDir` does not apply to them) to guarantee that relative container paths are correctly caught and rejected instead of leaking base host directories.

### 6. "Sticky Error" Pattern

The value resolution engine implements a **Sticky Error** pattern. The very first validation or resolution error encountered is stored internally. Subsequent resolution attempts gracefully return the original raw (unresolved) string to avoid compounding errors, and the final execution is securely aborted by propagating the retained error.

### 7. Lazy Resolver Instantiation Optimization

To optimize performance and resource footprint, the `ExpressionResolver` is instantiated lazily via `getR()` only when the configuration actually requires expression parsing (detecting `{{...}}`), tilde expansion (`~`), relative path resolution, or when executing under a nested context (`Level > 0`).

This ensures that simple, static executions bypass the file system probe and resolution engine overhead entirely, while still preserving fully robust and secure path resolutions when dynamic features are needed.

---

## Nested Execution Support

`cderun` supports running itself inside a container recursively. This enables nested environments where tools within containers can call other containerized tools seamlessly.

### 1. Snapshotting & Context Propagation

When nested execution is triggered (via `--mount-cderun`, `--mount-tools`, etc., or when executing inside a Level 1+ container), `cderun` creates a safe execution snapshot:

- Generates a directory `/tmp/cderun-snap-<uuid>/` with `0700` permission.
- Writes `.cderun.yaml` and `.tools.yaml` with `0600` permission containing merged configurations.
- Appends `hostContext` metadata indicating `binPath`, `snapshotDir`, `workingDir`, `level`, and active host-container mount mappings.

### 2. Reverse Path Resolution

Because container runtimes (Docker/Podman/containerd) run on the base host, any nested mount requests must specify a host-accessible path. `cderun` translates container-local paths back to host paths using `hostContext.mounts` mapping.

- **Matching Rule**: Matches target paths using a **longest-match** prefix-matching heuristic, falling back to the deepest nesting level to ensure correct base-host mapping.

### 3. macOS Setup Constraints

Nested execution on macOS requires special consideration because containers run inside a Linux VM:

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

- **Architecture**: A Linux-compiled binary of `cderun` (e.g., `cderun_linux_arm64` or `cderun_linux_amd64`) must be mounted instead of the macOS binary. Specify this with `--mount-cderun-path`.
- **Socket Permissions**: You must specify the VM's numeric socket Group ID explicitly via the `--cderun-group-add` CLI flag or `groupAdd` array (e.g. `"102"`) so the container user is granted access.

- See [Advanced Usage: Nested Execution on macOS](USAGE.md) for detailed step-by-step instructions.

---

## Sensitive Data Masking

`cderun` enforces a "Secure by Default" posture to protect your secrets.

- **Default Masking (Mask-all)**: If `--sensitive-env` is unset, **all** environment variable values are automatically masked as `[REDACTED]` in dry-run outputs and debug logs.
- **Pattern Matching**: Specify a list of glob patterns (e.g., `DB_*`, `*_PASSWORD`) in `--sensitive-env` to selectively mask matching keys while leaving other keys plaintext.
- **Disabling Masking**: Pass an explicit empty value (e.g. `--sensitive-env=""` or `sensitiveEnv: []` in YAML) to disable masking.
- **Hardening**: Error messages, dry-run values, and log records are securely quoted and validated to prevent log injection or terminal disruption.

---

## Multi-Runtime Support & Auto-detection

`cderun` integrates natively with standard container engines and supports **Docker**, **Podman**, and experimental **containerd**.

### 1. Auto-detection Socket Sequence

If no engine is explicitly specified, `cderun` checks for socket files in the following priority order:

1. `/var/run/docker.sock` (Docker)
2. `/run/containerd/containerd.sock` (containerd)
3. `/run/podman/podman.sock` (Podman)

Matches are isolated to the path's base name to avoid misdetection in nested paths. If no socket is found, it defaults to `docker` at `/var/run/docker.sock`.

### 2. containerd Runtime Limitations & Compatibility Matrix

Direct containerd integration operates natively via the containerd gRPC API. Please note the following constraints:

- **Platform**: direct containerd is **Linux-only** (`//go:build linux`).
- **Networking**: Only `host` networking is supported; default `bridge` network is not supported.
- **Ports & DNS**: Port mapping (`--publish`), exposure (`--expose`), custom DNS (`--dns`), and host mapping (`--add-host`) are not supported.
- **Mounts**: Named volume-type mounts are not supported (use `bind` or `tmpfs` mounts).
- **Adaptation Contract**: The containerd adapter enforces a strict contract to validate and convert Docker-compatible fields (e.g. mapping capabilities like `SYS_ADMIN` to `CAP_SYS_ADMIN`), returning explicit errors for unsupported fields rather than passing them through silently.

For a detailed feature comparison table across Docker, Podman, and direct containerd, see the [Container Runtime Compatibility Matrix](docs/features/command-line-options.md#container-runtime-compatibility-matrix).

---

## Environment Variables

Almost all CLI flags have a corresponding `CDERUN_` prefixed environment variable (e.g., `CDERUN_IMAGE`, `CDERUN_TTY`, `CDERUN_REMOVE`).

Key variables include:

- `CDERUN_IMAGE`: Container image to use.
- `CDERUN_CONFIG`: Path to cderun config file.
- `CDERUN_TOOL_CONFIG`: Path to tools config file.
- `CDERUN_RUNTIME`: Container runtime to use (docker/podman/containerd).
- `CDERUN_PULL_MAX_RETRIES`: Maximum number of retries for image pull (default: `3`).
- `CDERUN_PULL_BACKOFF_BASE`: Base duration for exponential backoff during image pull (default: `1s`).
- `CDERUN_PREFETCH`: Prefetch specified tool images (comma-separated list).
- `CDERUN_PREFETCH_ALL`: Prefetch all tool images defined in `.tools.yaml`.
- `CDERUN_HANG_TIMEOUT`: Grace period for non-interactive or non-TTY sessions (default: `10s`).
- `CDERUN_STRICT_ENV`: If set to `true`, requires all environment variables to be present on the host.
- `CDERUN_DRY_RUN`: If set to `true`, enables dry-run mode.
- `CDERUN_DRY_RUN_FORMAT`: Output format for dry-run (yaml, json, simple).
- `CDERUN_DIAGNOSIS`: If set to `true`, enables diagnosis mode.
- `CDERUN_DIAGNOSIS_FORMAT`: Output format for diagnosis (yaml, json, simple).
- `CDERUN_PUBLISH_ALL`: If set to `true`, publish all exposed ports to random ports.
- `CDERUN_LOG_LEVEL`: Set log level (error, warn, info, debug, trace).
- `CDERUN_LOG_FORMAT`: Set log format (text, json).
- `CDERUN_LOG_TIMESTAMP`: Include timestamp in logs.
- `CDERUN_SENSITIVE_ENV`: List of environment variable patterns to mask.

**Note on List-type Options:**

- **CLI Flags (P1/P2)**: List-type flags must be repeated for each item (e.g., `--env A=1 --env B=2`). Scalar string flags that accept multiple values (such as `--prefetch` and `--mount-tools`) use comma-separated values instead (e.g., `--prefetch node,python`).
- **Environment Variables (P3)**: Use specific separators depending on the variable.
  - **Semicolon (`;`)**: `CDERUN_ENV`, `CDERUN_MOUNT`
  - **Comma (`,`)**: All other list-type variables, including `CDERUN_GROUP_ADD`, `CDERUN_MOUNT_TOOLS`, `CDERUN_DEVICE`, `CDERUN_PUBLISH`, `CDERUN_EXPOSE`, `CDERUN_DNS`, `CDERUN_DNS_SEARCH`, `CDERUN_DNS_OPTION`, `CDERUN_ADD_HOST`, `CDERUN_CAP_ADD`, `CDERUN_CAP_DROP`, `CDERUN_ENTRYPOINT`, `CDERUN_SECURITY_OPT`, `CDERUN_SENSITIVE_ENV`, `CDERUN_ULIMIT`, `CDERUN_SYSCTL`, `CDERUN_PREFETCH`

---

## Configuration

`cderun` uses two configuration files to manage its behavior.

### `.cderun.yaml` (Global Settings)

Used for general settings and defaults.

```yaml
runtime: docker
defaults:
  tty: true
  interactive: true
  remove: true
logging:
  level: error
  format: text
```

### `.tools.yaml` (Tool Mappings)

Defines how specific tools should be containerized.

```yaml
node:
  image: node:20-alpine
  mounts:
    - type: bind
      source: .
      target: /app
  workdir: /app
python:
  image: python:3.11-slim
```

---

## Best Practices

### Consistent Development Environments

Use `{{file:.go-version}}` or `{{file:.nvmrc}}` in your tool configuration to ensure the container image tag matches your project's version file:

```yaml
# .tools.yaml
go:
  image: "golang:{{file:.go-version}}"
node:
  image: "node:{{file:.nvmrc}}-alpine"
```

### Context-Aware Pathing

Use `{{find_dir:.git}}` to reference the project root regardless of your current working directory. This is especially useful for mounting shared resources like `node_modules` or `logs` from the repository root:

```bash
# Mount logs from repository root
cderun --mount type=bind,source="{{find_dir:.git}}/logs",target=/logs my-tool

# Reference configuration in repository root
cderun --env "CONFIG_PATH={{find_dir:package.json}}/config/app.json" my-node-app
```

### Environment-Based Image Selection

Leverage environment variables with default values to switch between different image versions easily:

```bash
# Uses NODE_VERSION env var if set, otherwise falls back to 20-alpine
cderun --image "node:{{env:NODE_VERSION:-20-alpine}}" node --version
```

---

## Development & Testing

### Running Tests

To run the unit tests:

```bash
make test
# or
go test ./...
```

To run the End-to-End (E2E) tests which require a running Docker, Podman, or containerd environment:

```bash
go test -tags=runtime ./...
```

### Generating Coverage Report

To generate a test coverage report:

```bash
make coverage
```

---
*This project is under active development.*
