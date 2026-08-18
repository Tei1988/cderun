# Security Validations

`cderun` implements several validation layers to prevent command injection, path traversal, and log injection attacks.

## Character Validation

The `validatePathChars` function ensures that critical configuration strings do not contain ASCII control characters (ASCII < 32 or 127).

This validation is applied to:

- Image names
- User names
- Network modes
- Hostnames
- Working directories
- Runtime
- Socket paths (`--socket-path`, `--mount-socket-path`)
- cderun binary mount path (`--mount-cderun-path`)
- Output formats (`--dry-run-format`, `--diagnosis-format`)
- Logging settings (`--log-level`, `--log-format`)
- Entrypoint elements
- Port mappings and exposed ports (`--publish`, `--expose`)
- DNS servers and host mappings (`--dns`, `--add-host`)
- Linux capabilities (`--cap-add`, `--cap-drop`)

Any string containing unsafe characters will cause an immediate resolution failure.

## Anchor Boundary Validation

During dynamic path resolution utilizing magic words (`{{HOME}}`, `{{PWD}}`, etc.) or tildes (`~`), `cderun` validates that the final resolved absolute path does not escape or cross the boundaries of the associated anchor directory. This prevents directory traversal attacks and unauthorized access to arbitrary host system files. For more details, see [Anchor Boundary Validation](./value-resolution.md#anchor-boundary-validation).

## Working Directory Validation

The working directory validation function (`ValidateWorkdir`) strictly rejects any path containing parent directory traversal references (`..` segments) to prevent path obfuscation and directory traversal attacks within container configurations. Note that this validation and its safe absolute-path requirement apply only when a working directory is specified (non-empty); when provided, the path must be a valid absolute path and is subject to the strict traversal restrictions.

## Tool Name Safety

The `ValidateToolName` function enforces strict naming conventions for tool identifiers. Tool names are restricted to a whitelist of safe characters:

- Alphanumerics (`a-z`, `A-Z`, `0-9`)
- Dots (`.`)
- Underscores (`_`)
- Hyphens (`-`)

This prevents tool names from being used for path traversal (e.g., `../../etc/shadow`) or containing shell-sensitive characters (e.g., `|`, `;`, `:`). Tool name validation is performed before any logging or filesystem operations.

## Signal Validation

When signaling containers via `SignalContainer`, signal names are validated against a strict regular expression:
`^(?i)[A-Z0-9]+$`

This regex validation is designed to restrict allowed characters strictly to alphanumeric symbols to prevent command/argument injection attacks into the underlying runtime.

While it restricts characters to a safe set, it does not validate against a hardcoded static signal allowlist; signals that do not contain injection characters but are otherwise unknown to the host OS are processed through standard runtime error propagation.

## Device Cgroup Permissions Validation

To enforce secure device mounting, cgroup permissions for any device specified via `--device` (or corresponding configurations) are validated against a strict regular expression (`permsRegex`):
`^[rwm]+$`

This ensures that only valid permission flags (read `r`, write `w`, and mknod `m`) are specified, preventing any parameter injection or malformed input.

## Resource Settings Validation

To prevent invalid or unsafe resource configurations:

- **CPU and Memory Limits**:
  - Memory setting strings (e.g., `-500MB`) are processed via standard RAM parsing which inherently rejects negative values.
  - The direct `containerd` adapter explicitly validates that resource settings (`CPUs` and `Memory`) are non-negative, rejecting any negative values with clear validation errors before container execution.
- **Cpuset Validation**: `ValidateCpuset` restricts CPU and memory node set specifications (`--cpuset-cpus` and `--cpuset-mems`) strictly to numbers, commas, and hyphens (e.g., `0-3,5`), rejecting any parameter injection or malformed characters.
- **GPU Specification Validation**: `ValidateGPUs` restricts GPU requests (`--gpus`) to alphanumeric characters, commas, equals signs, and hyphens (e.g., `all`, `count=2`, `device=0,1`), blocking parameter injection attempts.

## Privileged Mode & Capability Warnings

When a container is configured to run in privileged mode (`--privileged` or `privileged: true` in config files), `cderun` performs deep scanning on highly privileged Linux capabilities supplied via both the `--cap-add` option (including corresponding environment variables and P1/P2 overrides) and supported configuration files (such as `.cderun.yaml` or `.tools.yaml`).

### Highly Privileged Capabilities

Scanned highly privileged capabilities include both standard and `CAP_`-prefixed forms:

- Standard: `ALL`, `SYS_ADMIN`, `NET_ADMIN`, `SYS_RAWIO`, `SYS_PTRACE`, `SYS_MODULE`
- Additional system-level/administrative capabilities:
  - `SYS_CHROOT` (chroot restriction bypasses)
  - `SYS_BOOT` (rebooting host system)
  - `SYS_TIME` (modifying system/hardware clocks)
  - `SYSLOG` (interacting with host syslog kernel logs)
  - `DAC_OVERRIDE` and `DAC_READ_SEARCH` (bypassing file read/write permissions check)
  - `LINUX_IMMUTABLE` (modifying immutable/append-only files)
  - `IPC_LOCK` and `IPC_OWNER` (locking memory and IPC resources)
  - `SYS_TTY_CONFIG` (configuring virtual terminals)
  - `LEASE` (managing file leases)
  - `AUDIT_CONTROL` (enabling/disabling system auditing)
  - `MAC_ADMIN` and `MAC_OVERRIDE` (MAC overrides)
  - `BPF` (loading BPF programs)
  - `PERFMON` (system performance monitoring)
  - `CHECKPOINT_RESTORE` (restoring processes)
  - `SYS_NICE` and `SYS_RESOURCE` (adjusting priority and system resources)

If any of these highly privileged capabilities are detected, a visible security warning at the `Warn` log level is emitted, encouraging the principle of least privilege.

## Host Namespace & Sensitive Path Mount Warnings

To encourage privilege minimization and maintain robust container isolation:

- **Host Network Mode Warning**: When host network namespace sharing is enabled (`--network host` or `network: host` in config files), `cderun` emits a `Warn` level security log. Bypassing network namespace isolation exposes the host's loopback and network services to the container, which should be restricted to trusted workloads.
- **Host PID Namespace Sharing**: Configuring the PID namespace to `"host"` (`--pid host` or `pid: host` in config files) disables process isolation. This allows processes running inside the container to view and interact with all processes on the host. `cderun` emits a visible `Warn` level security warning when host PID namespace sharing is activated.
- **Host Cgroup Namespace Sharing**: Configuring the cgroup namespace to `"host"` (`--cgroupns host` or `cgroupns: host` in config files) exposes the host system's cgroup hierarchy to the container. `cderun` emits a visible `Warn` level security log when host cgroup namespace sharing is activated.
- **Sensitive Bind Mounts**: Bind mounts that expose highly sensitive host directories (including `/`, `/boot`, `/dev`, `/etc`, `/proc`, and `/sys` or their subdirectories) are scanned. If a container is configured with any of these host-side paths as a mount source, a visible warning is logged at the `Warn` level to flag the risk and help users mitigate potential container escapes or host configuration exposure.

## Socket-Mounting and Numeric GID Access Warnings

Mounting the container runtime socket (e.g. `docker.sock`, `containerd.sock`, `podman.sock`) is a highly privileged operation that gives containerized workloads full control over the host's container engine.

- **Explicit Socket Mounting**: When `--mount-socket` is enabled, a warning is logged at the `Warn` level.
- **Manual Socket Bind Mounts**: `cderun` scans manual bind mounts and flags any source paths that match the container runtime socket path or end with common container socket filenames (such as `/docker.sock`, `/containerd.sock`, or `/podman.sock`). This prevents users from bypassing socket validation warnings via raw mounting configurations.
- **Numeric Socket Group ID Warning**: If container socket mounting is active and supplementary groups include any numeric Group ID (`--group-add <GID>`), `cderun` emits an additional security warning. Granting socket permissions through raw GIDs poses elevated risks and should be confined to trusted and verified environments.

## Sensitive Host Device Scanning

For device mappings configured via `--device`, `cderun` scans host-side device paths. If a highly sensitive host device is detected, a warning is emitted at the `Warn` log level.

Sensitive devices scanned include:

- `/dev/mem` (physical memory access)
- `/dev/kmem` (kernel memory access)
- `/dev/port` (I/O port access)
- Block devices with prefixes:
  - `/dev/sd*` (SCSI disk devices)
  - `/dev/nvme*` (NVMe storage devices)
  - `/dev/loop*` (loopback block devices)
  - `/dev/mapper/*` (logical volume mappings)

## Socket Mount Target Path Validation (`validateMountSocketPathRaw`)

To prevent relative-path container target hijacking, `cderun` strictly validates container-side socket mount paths:

- The container target path must be absolute (e.g., `/var/run/docker.sock`).
- Target paths cannot contain parent directory traversal (`..`) segments.
- This verification is executed against raw user input via `validateMountSocketPathRaw` to validate path syntax before option resolution.

## Registry Mismatch Validation

To prevent pulling and running unauthorized container images, `cderun` validates that the image specified on the command-line or environment variable matches the registry rules defined in the tools profile (`.tools.yaml`).

The registry matching checks the hostname and repository namespace (e.g. `docker.io/library/node`), ignoring differing tags or digest suffixes. If a mismatch is identified (such as specifying a custom repository `private-reg.com/library/node` when `.tools.yaml` restricts the tool to `docker.io`), execution terminates with a `RegistryMismatchError` before contacting the daemon.

## Absolute Mount Targets

All mount configurations must specify an absolute path for the `Target` (container-side path). Relative paths in mount targets are ambiguous and potentially dangerous, so they are rejected during resolution.

## Environment Variable & Parameter Validation

When validating generic `KEY=VALUE` environment variable entries (`--env`), `cderun` applies character checks strictly to the **key** portion (the part before the first `=`). This prevents the use of control characters in variable names while allowing legitimate multiline values or complex strings (such as PEM certificates) in the environment variable values themselves.

Additionally, specific parameter validators enforce strict character allowlists on networking and kernel parameter options:

- **DNS Options (`ValidateDNSOption`)**: Ensures custom DNS options (e.g., `ndots:3`) contain only ASCII alphanumerics, dots (`.`), colons (`:`), underscores (`_`), and hyphens (`-`), and explicitly rejects parent directory references (`..`).
- **Sysctl Keys (`ValidateSysctlKey`)**: Enforces non-empty sysctl keys consisting exclusively of ASCII alphanumerics, dots (`.`), underscores (`_`), and hyphens (`-`).
- **Sysctl Values (`ValidateSysctlValue`)**: Restricts sysctl values to ASCII alphanumerics, spaces (` `), dots (`.`), underscores (`_`), hyphens (`-`), and commas (`,`).
