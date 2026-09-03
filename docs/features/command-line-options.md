# Command-Line Options Reference

## Overview

This document provides a comprehensive reference of all command-line options supported by `cderun`.

### Container Runtime Compatibility Matrix

The table below summarizes support for key `cderun` configuration features across container execution engines:

| Feature / Option Flag | Docker | Podman | containerd (Direct API, Linux-only) | Notes / Adapter Behavior |
| :--- | :---: | :---: | :---: | :--- |
| **TTY Allocation** (`--tty`, `-t`) | ✅ | ✅ | ✅ | Allocates a pseudo-TTY (PTY). Callers must register I/O via `AttachContainer` before `StartContainer`; if skipped, containerd falls back to `NullIO`. |
| **Interactive STDIN** (`--interactive`, `-i`) | ✅ | ✅ | ✅ | Keeps STDIN open even if not attached. In containerd, callers must register I/O via `AttachContainer` before `StartContainer` to forward stream input (otherwise containerd defaults to `NullIO`). |
| **Port Publishing** (`-p`, `-P`) | ✅ | ✅ | ❌ | containerd API does not manage CNI host port forwarding (`ValidateConfig` returns error) |
| **Expose Ports** (`--expose`) | ✅ | ✅ | ❌ | Unsupported on direct containerd |
| **DNS Servers** (`--dns`) | ✅ | ✅ | ❌ | Unsupported on direct containerd |
| **DNS Options & Search** (`--dns-option`, `--dns-search`) | ✅ | ✅ | ❌ | Explicitly rejected by containerd `ValidateConfig` |
| **Add Host Mappings** (`--add-host`) | ✅ | ✅ | ❌ | Host entry injection not supported by containerd API |
| **Network Modes** (`--network`) | ✅ | ✅ | ⚠️ | containerd supports `host` mode only (`bridge` is rejected) |
| **Restart Policies** (`--restart`) | ✅ | ✅ | ❌ | containerd has no daemon restart manager (`ValidateConfig` returns error) |
| **Container Init** (`--init`) | ✅ | ✅ | ❌ | Tini init injection unsupported on containerd (`ValidateConfig` returns error) |
| **Memory / CPU Limits** (`-m`, `--cpus`) | ✅ | ✅ | ✅ | Mapped to Cgroups/OCI resource specifications |
| **CPU Shares & Cpuset** (`--cpu-shares`, `--cpuset-*`) | ✅ | ✅ | ✅ | Mapped directly to OCI Linux resource limits |
| **Process Limits** (`--pids-limit`) | ✅ | ✅ | ✅ | Mapped to OCI process limit `Pids.Limit` |
| **Process Ulimits** (`--ulimit`) | ✅ | ✅ | ✅ | Converted to OCI POSIX process rlimits (`specs.POSIXRlimit`) |
| **Shared Memory** (`--shm-size`) | ✅ | ✅ | ✅ | Dynamically manages `/dev/shm` tmpfs mount in containerd spec |
| **IPC Namespace** (`--ipc`) | ✅ | ✅ | ⚠️ | containerd supports `"host"`, `"private"`, or `""` (container sharing rejected) |
| **PID Namespace** (`--pid`) | ✅ | ✅ | ⚠️ | containerd supports `"host"` via `WithHostNamespace(specs.PIDNamespace)` |
| **Cgroup Namespace** (`--cgroupns`) | ✅ | ✅ | ⚠️ | containerd supports `"host"`, `"private"`, or `""` |
| **Security Options** (`--security-opt`) | ✅ | ✅ | ⚠️ | containerd maps `no-new-privileges`, `seccomp`, `apparmor`, and `label` |
| **Capabilities** (`--cap-add`, `--cap-drop`) | ✅ | ✅ | ✅ | Normalized to OCI `CAP_` prefixed capability strings |
| **Supplementary Groups** (`--group-add`) | ✅ | ✅ | ⚠️ | containerd requires numeric GIDs (group names rejected) |
| **GPU Devices** (`--gpus`) | ✅ | ✅ | ❌ | Unsupported on containerd API (`ValidateConfig` returns error) |
| **Sysctl Kernel Params** (`--sysctl`) | ✅ | ✅ | ✅ | Mapped directly to OCI `Linux.Sysctl` map |
| **Read-Only Rootfs** (`--read-only`) | ✅ | ✅ | ✅ | Mapped to `ReadonlyRootfs` (Docker) and `Root.Readonly` (containerd) |
| **Stand-alone Image Prefetching** (`--prefetch`, `--prefetch-all`) | ✅ | ✅ | ✅ | Prefetches tool images via runtime `PullImage` API without container execution |
| **Hang Timeout** (`--hang-timeout`) | ✅ | ✅ | ✅ | Managed by `cderun` execution controller after I/O finishes in non-TTY environments |
| **Control Socket Mounting** (`--mount-cderun-socket`) | ✅ | ✅ | ✅ | Native Control Socket framing (`cderun.sock`) for nested containers |

### List-Type (Array-Type) Options and Environment Variable Separator Rules

When supplying multiple values for a list-type option (e.g., `stringArray` or `[]string`) using environment variables (P3), `cderun` enforces specific separator rules depending on the variable:

- **Semicolon (`;`) Separator**:
  - `CDERUN_ENV` (e.g., `export CDERUN_ENV="KEY1=val1;KEY2=val2"`)
  - `CDERUN_MOUNT` (e.g., `export CDERUN_MOUNT="type=bind,source=./src,target=/app;type=tmpfs,target=/tmp"`)
- **Comma (`,`) Separator**:
  - `CDERUN_PREFETCH`
  - `CDERUN_GROUP_ADD`
  - `CDERUN_MOUNT_TOOLS`
  - `CDERUN_DEVICE`
  - `CDERUN_PUBLISH`
  - `CDERUN_EXPOSE`
  - `CDERUN_DNS`
  - `CDERUN_DNS_SEARCH`
  - `CDERUN_DNS_OPTION`
  - `CDERUN_ADD_HOST`
  - `CDERUN_CAP_ADD`
  - `CDERUN_CAP_DROP`
  - `CDERUN_ENTRYPOINT`
  - `CDERUN_SECURITY_OPT`
  - `CDERUN_SENSITIVE_ENV`
  - `CDERUN_ULIMIT`
  - `CDERUN_SYSCTL`

*Note: When passing list-type options via CLI flags (P1/P2), separators are not used. Instead, repeat the flag (e.g., `--env A=1 --env B=2` or `--dns 8.8.8.8 --dns 1.1.1.1`). An exception is scalar string options like `--prefetch` (and `--mount-tools`), which are registered as scalar string flags whose handlers split comma-separated values (e.g., `--prefetch node,python`).*

---

## Basic Syntax

```bash
cderun [cderun-flags] <subcommand> [passthrough-args]
```

- **[cderun-flags]**: Flags that control the behavior of `cderun`.
  - **Standard Flags (P2)**: Placed **before** the subcommand (e.g., `--tty`, `--env`).
- **\<subcommand\>**: The first non-flag argument (e.g., `node`, `python`). It acts as a **Lookup Key** to load configurations from `.tools.yaml` and is not included in the container's command by default.
- **[passthrough-args]**: Any arguments appearing after the subcommand. All arguments are forwarded directly to the container's command, except for P1 internal overrides (`--cderun-*`), which are intercepted and hoisted during preprocessing.

---

## Global Options

### `--tty`, `-t`

- **Type**: bool
- **Default**: `false`
- **Environment Variable**: `CDERUN_TTY`
- **Description**: Allocate a pseudo-TTY.
- **Use Case**: Used for interactive terminal execution (e.g., shell access).

```bash
cderun --tty bash
cderun -t node
```

### `--interactive`, `-i`

- **Type**: bool
- **Default**: `false`
- **Environment Variable**: `CDERUN_INTERACTIVE`
- **Description**: Keep STDIN open even if not attached.
- **Use Case**: Necessary for reading interactive input.

```bash
cderun --interactive python
cderun -i bash
```

### `--network`

- **Type**: string
- **Default**: `bridge`
- **Environment Variable**: `CDERUN_NETWORK`
- **Description**: Connect a container to a network.
- **Supported Values**: `bridge`, `host`, `none`, or any custom network name.

```bash
cderun --network host node server.js
cderun --network none python script.py
```

### `--socket-path`

- **Type**: string
- **Default**: Auto-detected (e.g., `/var/run/docker.sock`)
- **Environment Variable**: `CDERUN_SOCKET_PATH`
- **Description**: Specify the path to the container runtime socket on the host.

```bash
cderun --socket-path /var/run/docker.sock docker ps
```

### `--mount-socket`

- **Type**: bool
- **Default**: `false`
- **Environment Variable**: `CDERUN_MOUNT_SOCKET`
- **Description**: Mount the host's container runtime socket inside the container.
- **Use Case**: Enables Docker-in-Docker or container control from within the container.

```bash
cderun --mount-socket docker ps
```

### `--mount-socket-path`

- **Type**: string
- **Default**: The host-side socket path (`--socket-path` or auto-detected socket)
- **Environment Variable**: `CDERUN_MOUNT_SOCKET_PATH`
- **Description**: Specifies the path inside the container where the socket should be mounted.
- **Validation**: Must be an absolute path and must not contain any parent traversal (`..`) segments.

```bash
cderun --mount-socket --mount-socket-path /var/run/docker.sock node app.js
```

### `--mount-cderun`

- **Type**: bool
- **Default**: `false`
- **Environment Variable**: `CDERUN_MOUNT_CDERUN`
- **Description**: Mount the host's `cderun` binary into the container at `/usr/local/bin/cderun`.
- **Note**:
  - Automatically enables `--mount-socket` unless explicitly set to `false`.
  - Automatically enabled when `--mount-tools` or `--mount-all-tools` is specified.

```bash
cderun --mount-cderun alpine sh
```

### `--mount-cderun-path`

- **Type**: string
- **Environment Variable**: `CDERUN_MOUNT_CDERUN_PATH`
- **Description**: Specifies the host-side path of the `cderun` binary to mount.
- **Use Case**: Necessary on macOS where the host binary (Darwin) cannot run in the Linux VM container, allowing you to specify a pre-compiled Linux binary.

```bash
cderun --mount-cderun --mount-cderun-path /path/to/cderun alpine sh
```

### `--mount-cderun-socket`

- **Type**: bool
- **Default**: `false`
- **Environment Variable**: `CDERUN_MOUNT_CDERUN_SOCKET`
- **Description**: Mount cderun Control Socket (`cderun.sock`) into the container for scoped nested execution (experimental).
- **Note**:
  - Creates a dedicated Control Socket inside the per-invocation snapshot directory (`<snapshotDir>/cderun.sock`) and mounts it to `/run/cderun/cderun.sock` inside the child container.
  - Currently documented as **experimental** during phased rollout (see `docs/features/nested-execution-control-socket.md`).

```bash
cderun --mount-cderun-socket alpine sh
```

### `--mount-tools`

- **Type**: string (comma-separated list)
- **Environment Variable**: `CDERUN_MOUNT_TOOLS`
- **Description**: Dynamically mount specified tool wrappers from `.tools.yaml` into the container.
- **Note**: Automatically enables `--mount-cderun` and `--mount-socket`.

```bash
cderun --mount-tools node,python alpine sh
```

### `--mount-all-tools`

- **Type**: bool
- **Default**: `false`
- **Environment Variable**: `CDERUN_MOUNT_ALL_TOOLS`
- **Description**: Dynamically mount all tool wrappers defined in `.tools.yaml` into the container.
- **Note**: Automatically enables `--mount-cderun` and `--mount-socket`.

```bash
cderun --mount-all-tools alpine sh
```

### `--image`

- **Type**: string
- **Environment Variable**: `CDERUN_IMAGE`
- **Description**: Explicitly specify the container image to use (overriding image mappings).
- **Note**: Supports resolution expressions (e.g., `{{env:TAG}}`).

```bash
cderun --image node:18-alpine node --version
cderun --image "node:{{env:NODE_VERSION:-20-alpine}}" node --version
```

### `--env`, `-e`

- **Type**: stringArray
- **Environment Variable**: `CDERUN_ENV`
- **Description**: Set or pass through environment variables.
- **Format**: `KEY=value` (explicit setting) or `KEY` (passthrough from host).
- **Validation**: Scanned for null bytes (`\x00`) and unescaped C0/C1 control characters to prevent header/shell injection.
- **Note**: Supports dynamic resolution expressions (e.g., `{{PWD}}`).

```bash
cderun --env NODE_ENV=production node app.js
cderun --env NPM_TOKEN node app.js
cderun --env "PROJECT_DIR={{PWD}}" node app.js
```

### `--cderun-env`

- **Type**: stringArray
- **Description**: Force-override environment variables (P1 internal override).
- **Placement**: Must be specified **after** the subcommand in Wrapper Mode.

```bash
cderun node app.js --cderun-env=NODE_ENV=production
```

### `--mount`

- **Type**: stringArray
- **Environment Variable**: `CDERUN_MOUNT`
- **Description**: Specify file system mounts. Supports `bind`, `volume`, and `tmpfs`.
- **Format Parameters**:
  - `type`: `bind` | `volume` | `tmpfs` (default: `bind`)
  - `source` (aliases: `src`): Path on the host. Supports expressions (e.g., `{{HOME}}`).
  - `target` (aliases: `dst`, `destination`): Absolute path inside the container. Must be non-empty and absolute.
  - `readonly`: Mounts the file system as read-only.
  - `optional` (or `optional=true`): Skips the bind mount without failing if the host-side `source` path is missing.

```bash
cderun --mount type=bind,source=./data,target=/data python script.py
cderun --mount type=bind,source=~/.ssh,target=/root/.ssh,readonly git clone ...
cderun --mount type=bind,source=./config,target=/config,optional node app.js
cderun --mount type=tmpfs,target=/tmp alpine
```

### `--workdir`, `-w`

- **Type**: string
- **Environment Variable**: `CDERUN_WORKDIR`
- **Description**: Specify the container's working directory.
- **Validation**: Enforces absolute path checks and blocks parent directory traversals (`..`) if explicitly configured.

```bash
cderun --workdir /app node server.js
cderun --workdir "{{PWD}}/src" node app.js
```

### `--strict-env`

- **Type**: bool
- **Default**: `false`
- **Environment Variable**: `CDERUN_STRICT_ENV`
- **Description**: If true, aborts execution with an error if any requested passthrough environment variables are missing on the host.

```bash
cderun --strict-env --env NPM_TOKEN node app.js
```

### `--runtime`

- **Type**: string
- **Default**: Auto-detected (`docker` -> `containerd` -> `podman`, falling back to `docker`)
- **Environment Variable**: `CDERUN_RUNTIME`
- **Supported Engines**: `docker`, `podman`, `containerd`.
- **Auto-detection Logic**:
  When no engine is explicitly specified, `cderun` checks for active runtime sockets in the following priority order:
  1. `/var/run/docker.sock` (Docker)
  2. `/run/containerd/containerd.sock` (containerd)
  3. `/run/podman/podman.sock` (Podman)
  If no socket file exists on disk, `cderun` falls back to `docker` at `/var/run/docker.sock`.

```bash
cderun --runtime podman node app.js
```

### `--remove`

- **Type**: bool
- **Default**: `true`
- **Environment Variable**: `CDERUN_REMOVE`
- **Description**: Automatically remove the container when it exits.

```bash
cderun --remove=false node app.js
```

### `--restart`

- **Type**: string
- **Default**: `""` (no restart policy)
- **Environment Variable**: `CDERUN_RESTART`
- **Description**: Configure the container's restart policy when the container exits.
- **Details**:
  - **Docker**: Maps directly to `HostConfig.RestartPolicy`. Supported policies include `no`, `always`, `unless-stopped`, and `on-failure`. A retry count suffix (e.g., `:5` in `on-failure:5`) is valid **only** with the `on-failure` policy.
  - **containerd**: Not supported. Since containerd does not provide automatic daemon-level process/container restart management, any non-`"no"` or non-empty restart policies are explicitly rejected with a validation error containing `"restart policy is not supported yet"`.
  - **Mutual Exclusion**: The default removal behavior is `--remove=true`. `--remove=true` conflicts only with restart policies other than empty or `"no"`. To use active restart policies (like `always` or `on-failure`), you must explicitly configure `--remove=false`.
- **P1 Internal Override**: `--cderun-restart` is the corresponding Phase 1 (P1) internal override flag. It accepts a string policy and must be placed after the subcommand in Wrapper Mode.

```bash
cderun --remove=false --restart on-failure:5 node app.js
```

### `--publish`, `-p`

- **Type**: stringArray
- **Environment Variable**: `CDERUN_PUBLISH`
- **Description**: Publish a container's port(s) to the host.
- **Format**: `hostPort:containerPort` (e.g., `8080:80`).
- **Validation**: Strict validation enforces numeric boundaries (e.g., values must be within the `1-65535` range).

```bash
cderun -p 8080:80 nginx
```

### `--publish-all`, `-P`

- **Type**: bool
- **Default**: `false`
- **Environment Variable**: `CDERUN_PUBLISH_ALL`
- **Description**: Publish all exposed ports to random high ports on the host.

### `--expose`

- **Type**: stringArray
- **Environment Variable**: `CDERUN_EXPOSE`
- **Description**: Expose a port or range of ports.
- **Format**: `port/protocol` (e.g., `80`, `80/udp`).

```bash
cderun --expose 80 node app.js
cderun --expose 80/udp node app.js
```

### `--hostname`

- **Type**: string
- **Environment Variable**: `CDERUN_HOSTNAME`
- **Description**: Specify the container host name. Must be a valid hostname or fully qualified domain name (FQDN).

```bash
cderun --hostname my-container alpine hostname
```

### `--dns`

- **Type**: stringArray
- **Environment Variable**: `CDERUN_DNS`
- **Description**: Set custom DNS servers.

```bash
cderun --dns 8.8.8.8 alpine ping google.com
```

### `--dns-option`

- **Type**: stringArray
- **Environment Variable**: `CDERUN_DNS_OPTION`
- **Description**: Set custom DNS options (e.g., `ndots:3`).
- **Runtime Limitations**:
  - **Docker**: Supported and mapped directly to `HostConfig.DNSOptions`.
  - **containerd**: Not supported. Explicitly rejected with a validation error (`"containerd runtime: dns_options is not supported yet"`) inside `ValidateConfig` to prevent silent misconfigurations.

```bash
cderun --dns-option ndots:3 alpine cat /etc/resolv.conf
```

### `--dns-search`

- **Type**: stringArray
- **Environment Variable**: `CDERUN_DNS_SEARCH`
- **Description**: Set custom DNS search domains.
- **Runtime Limitations**:
  - **Docker**: Supported and mapped directly to `HostConfig.DNSSearch`.
  - **containerd**: Not supported. Explicitly rejected with a validation error (`"containerd runtime: dns_search is not supported yet"`) inside `ValidateConfig` to prevent silent misconfigurations.

```bash
cderun --dns-search example.com alpine ping my-service
```

### `--add-host`

- **Type**: stringArray
- **Environment Variable**: `CDERUN_ADD_HOST`
- **Description**: Add a custom host-to-IP mapping (`host:ip`).

```bash
cderun --add-host my-server:192.168.1.10 alpine ping my-server
```

### `--user`, `-u`

- **Type**: string
- **Environment Variable**: `CDERUN_USER`
- **Description**: Username or UID (format: `<name|uid>[:<group|gid>]`).

```bash
cderun -u 1000:1000 alpine whoami
```

### `--group-add`

- **Type**: stringArray
- **Environment Variable**: `CDERUN_GROUP_ADD`
- **Description**: Add supplementary groups to the container execution user.
- **Validation**: Enforces strict alphanumeric/numeric GID validation via regular expressions.
- **Runtime Limitations**:
  - **Docker / Podman**: Supports both group names (e.g., `docker`, `adm`) and numeric GIDs (e.g., `1001`).
  - **containerd**: Due to direct OCI spec translation without internal container database access, **only numeric GIDs** (e.g., `1001`) are supported. Specifying group names will trigger an explicit execution error.

```bash
cderun --group-add 1001 --group-add 1002 alpine id
```

### `--gpus`

- **Type**: string
- **Default**: `""`
- **Environment Variable**: `CDERUN_GPUS`
- **Description**: GPU devices to request from the container runtime (e.g., `all`, `count=2`, `device=0,1`).
- **Runtime Limitations**:
  - **Docker**: Supported and parsed into standard Docker `DeviceRequests`.
  - **containerd**: Not supported. Explicitly rejected with a validation error (`"containerd runtime: gpus is not supported yet"`) inside `ValidateConfig` to prevent silent misconfigurations.

```bash
cderun --gpus all nvidia/cuda nvidia-smi
```

### `--privileged`

- **Type**: bool
- **Default**: `false`
- **Environment Variable**: `CDERUN_PRIVILEGED`
- **Description**: Give extended privileges to the container.
- **Security Check**: Enforces a `Warn` level security log alert when activated.

```bash
cderun --privileged alpine ls /dev
```

### `--read-only`

- **Type**: bool
- **Default**: `false`
- **Environment Variable**: `CDERUN_READ_ONLY`
- **Description**: Mount the container's root filesystem as read-only.
- **Details**: Maps to `ReadonlyRootfs` in Docker host configuration and `Root.Readonly = true` in the containerd OCI specification.
- **P1 Internal Override**: `--cderun-read-only` is the corresponding Phase 1 (P1) internal override flag. It accepts a boolean toggle and must be placed after the subcommand in Wrapper Mode (e.g., `cderun alpine touch /test-write --cderun-read-only`).

```bash
cderun --read-only alpine touch /test-write
```

### `--init` (Container Init Process)

- **Type**: bool
- **Default**: `false`
- **Environment Variable**: `CDERUN_INIT`
- **Description**: Run an init process inside the container to forward signals and reap zombie processes.
- **Details**:
  - **Docker / Podman**: Maps to `Init` in the host configuration (e.g., standard `tini`).
  - **containerd**: Not supported. Explicitly rejected with a validation error (`"containerd runtime: init is not supported yet"`) in the containerd adapter's `ValidateConfig` method to prevent silent failures.
- **P1 Internal Override**: `--cderun-init` is the corresponding Phase 1 (P1) internal override flag. It accepts a boolean toggle (e.g., `--cderun-init` or `--cderun-init=false`), and must be placed after the subcommand in Wrapper Mode (e.g., `cderun alpine touch /test --cderun-init`).

```bash
cderun --init alpine ps aux
```

### `--ipc`

- **Type**: string
- **Default**: `""` (private IPC namespace or runtime default)
- **Environment Variable**: `CDERUN_IPC`
- **Description**: Configure the IPC namespace for the container.
- **Details**:
  - **Docker**: Maps directly to `HostConfig.IpcMode`. Supported modes include `host`, `private`, `shareable`, `none`, and `container:<id>`.
  - **containerd**: Supports `"host"`, `"private"`, or `""` (empty). Any other values (such as empty `container:` or `container:<id>`) are strictly rejected with an explicit validation error (`"containerd runtime: unsupported IPC namespace mode: %q"`). If set to `"host"`, containerd configures the OCI spec to share the host's IPC namespace.
- **P1 Internal Override**: `--cderun-ipc` is the corresponding Phase 1 (P1) internal override flag. It accepts a string value and must be placed after the subcommand in Wrapper Mode.

```bash
# Share host IPC namespace
cderun --ipc host alpine ipcs
```

### `--pid`

- **Type**: string
- **Default**: `""` (private PID namespace)
- **Environment Variable**: `CDERUN_PID`
- **Description**: Configure the PID namespace for the container.
- **Supported Values**:
  - `""` (empty string, default): Use a private, isolated PID namespace inside the container.
  - `"host"`: Share the host system's PID namespace with the container. This allows processes in the container to see all processes on the host. Any other values are strictly rejected with an error.
- **Details**:
  - **Docker / Podman**: Maps to `PidMode` in the host configuration.
  - **containerd**: Appends `WithHostNamespace(specs.PIDNamespace)` to the containerd OCI spec options when configured as `"host"`.
- **P1 Internal Override**: `--cderun-pid` is the corresponding Phase 1 (P1) internal override flag. It accepts a string value (e.g., `host` or `""`), supporting both space-separated and equals-sign formats (e.g., `--cderun-pid host` or `--cderun-pid=host`), and must be placed after the subcommand in Wrapper Mode.

```bash
# Share host PID namespace
cderun --pid host alpine ps aux
```

### `--pids-limit`

- **Type**: int64
- **Default**: `0` (no limit)
- **Environment Variable**: `CDERUN_PIDS_LIMIT`
- **Description**: Limit the maximum number of active processes/threads inside the container (fork-bomb protection).
- **Details**:
  - **Docker**: Maps to `HostConfig.Resources.PidsLimit`.
  - **containerd**: Maps to OCI spec limit `Linux.Resources.Pids.Limit` (applies if value is greater than 0 or equals -1).
- **P1 Internal Override**: `--cderun-pids-limit` is the corresponding Phase 1 (P1) internal override flag. It accepts an integer value and must be placed after the subcommand in Wrapper Mode.

```bash
cderun --pids-limit 100 alpine forkbomb
```

### `--security-opt`

- **Type**: stringArray
- **Default**: `none`
- **Environment Variable**: `CDERUN_SECURITY_OPT`
- **Description**: Configure container security options (such as `no-new-privileges`, `seccomp=unconfined`, and AppArmor profiles).
- **Details**:
  - **Docker**: Maps directly to `HostConfig.SecurityOpt` in the Docker host configuration.
  - **Podman**: Mapped to security options supported natively by the Podman engine.
  - **containerd**: Mutated in `CreateContainer` via a named helper `applySecurityOptions`. Empty AppArmor profiles (such as `apparmor=` or `apparmor:` with no profile name) are strictly rejected with explicit validation errors. Options like `seccomp=unconfined` initialize `s.Linux` if nil to prevent silent specification mutation failures.
- **P1 Internal Override**: `--cderun-security-opt` is the corresponding Phase 1 (P1) internal override flag. It accepts string values and must be placed after the subcommand in Wrapper Mode (e.g., `cderun alpine sh --cderun-security-opt=no-new-privileges`).

```bash
cderun --security-opt no-new-privileges alpine sh
```

### `--ulimit` (Process Resource Limits)

- **Type**: stringArray
- **Default**: `none`
- **Environment Variable**: `CDERUN_ULIMIT`
- **Description**: Configure process resource limits (ulimits) in container execution environments.
- **Format**: `<type>=<soft>:<hard>` or `<type>=<value>` (e.g., `nofile=65535:65535`, `nofile=65535`).
- **Validation**: Limit values (both soft and hard) must be at least `-1` (where `-1` represents unlimited).
- **Details**:
  - **Specification Parsing**: Specifications (such as `nofile=1024:2048`) are parsed via the `github.com/docker/go-units` standard parser.
  - **Docker**: Maps directly to `HostConfig.Resources.Ulimits` in the Docker host configuration.
  - **Podman**: Mapped to ulimit configurations supported natively by the Podman engine.
  - **containerd**: Converted into standard POSIX OCI process rlimits (`specs.POSIXRlimit` inside `Process.Rlimits`) via custom `oci.SpecOpts` in the containerd adapter.
- **P1 Internal Override**: `--cderun-ulimit` is the corresponding Phase 1 (P1) internal override flag. It accepts string values and must be placed after the subcommand in Wrapper Mode (e.g., `cderun alpine ulimit -n --cderun-ulimit=nofile=1024:2048`).

```bash
cderun --ulimit nofile=1024:2048 alpine ulimit -n
```

### `--shm-size`

- **Type**: string
- **Default**: `none`
- **Environment Variable**: `CDERUN_SHM_SIZE`
- **Description**: Configure the size of the shared memory partition (`/dev/shm`) for containers.
- **Details**:
  - **Shared Memory Allocation**: Controls the size of the ephemeral `/dev/shm` partition mounted inside the container. If unspecified or set to `"none"`, memory size limits are determined by the container engine defaults (e.g., Docker defaults shared memory to 64MB, which is often insufficient for modern multi-threaded apps, databases, or browsers like Puppeteer/Chrome).
  - **Supported Formats**: The size string is parsed using `github.com/docker/go-units`'s standard RAM in bytes parser (`RAMInBytes`), which uses binary multipliers (e.g., `1g` and `1gb` equal `1,073,741,824` bytes, or 1024^3). It accepts non-negative numeric values, including zero and fractional sizes (such as `1.5g`):
    - Bytes (e.g., `2147483648` or `2048b`)
    - Kilobytes (e.g., `1024k` or `1024kb`)
    - Megabytes (e.g., `256m` or `256mb`)
    - Gigabytes (e.g., `1g` or `1gb`)
    - *Note: Invalid format inputs will trigger validation errors. Size values must be non-negative (at least 0).*
  - **Docker / Podman**: Directly maps to `ShmSize` (in bytes) inside the Docker/Podman Host Configuration.
  - **containerd**: Dynamically manages the `/dev/shm` tmpfs mount within the OCI specification using the helper spec opt `getShmSizeSpecOpt`:
    1. Scans existing container mounts. If a `/dev/shm` mount exists:
       - If it is not a `tmpfs` mount, it returns a validation error.
       - If it is a `tmpfs` mount, it preserves all other existing mount options while replacing any existing `size=` options with the new size option.
    2. If no `/dev/shm` mount exists, it appends a new `tmpfs` mount specifically for `/dev/shm` configured with the specified size and default mounting options (`nosuid`, `noexec`, `nodev`, `mode=1777`).
- **P1 Internal Override**: `--cderun-shm-size` is the corresponding Phase 1 (P1) internal override flag. It accepts a string value and must be placed after the subcommand in Wrapper Mode (e.g., `cderun alpine sh --cderun-shm-size=256m`).

```bash
cderun --shm-size 256m alpine df -h /dev/shm
```

### `--cap-add`

- **Type**: stringArray
- **Environment Variable**: `CDERUN_CAP_ADD`
- **Description**: Add Linux capabilities.
- **Security Auditing**: Highly privileged capabilities (such as `ALL`, `SYS_ADMIN`, `NET_ADMIN`, `SYS_RAWIO`, `SYS_PTRACE`, `SYS_MODULE` with or without `CAP_` prefix) are scanned and trigger a `Warn` level audit alert to encourage the principle of least privilege.

```bash
cderun --cap-add SYS_ADMIN alpine mount ...
```

### `--cap-drop`

- **Type**: stringArray
- **Environment Variable**: `CDERUN_CAP_DROP`
- **Description**: Drop Linux capabilities.

### `--cgroupns`

- **Type**: string
- **Default**: `""` (private cgroup namespace or runtime default)
- **Environment Variable**: `CDERUN_CGROUPNS`
- **Description**: Configure the cgroup namespace for the container.
- **Details**:
  - **Docker**: Maps directly to `HostConfig.CgroupnsMode`. Supported modes are `host` and `private`.
  - **containerd**: Supports `"host"`, `"private"`, or `""` (empty). Any other values are strictly rejected with an explicit validation error (`"containerd runtime: unsupported cgroup namespace mode: %q"`). If set to `"host"` or `"private"`, containerd configures/appends the OCI spec namespaces accordingly.
- **P1 Internal Override**: `--cderun-cgroupns` is the corresponding Phase 1 (P1) internal override flag. It accepts a string value and must be placed after the subcommand in Wrapper Mode.

```bash
cderun --cgroupns private alpine ls -la
```

### `--sensitive-env`

- **Type**: stringArray
- **Environment Variable**: `CDERUN_SENSITIVE_ENV`
- **Description**: List of environment variable patterns (glob format) to mask.
- **Secure-by-Default Architecture**:
  - **Default (Unset)**: All environment variables are treated as sensitive, masking values of non-empty keys as `[REDACTED]` in dry-runs and debug logs.
  - **Explicit Disable**: Pass `--sensitive-env=""` (or YAML empty list `sensitiveEnv: []`) to completely disable masking.
  - **Fail-Closed Fallback**: If any glob pattern syntax is malformed (e.g., `[` with no closing bracket), `cderun` safely falls back to masking all non-empty environment values to prevent accidental credential leakage.
  - **High-Performance Matching**: Pattern matching is optimized via allocation-free fast-path matching (`fastMatchFold`) for standard glob patterns (`*suffix`, `prefix*`, `*substr*`, and exact matches), bypassing heavy glob evaluations on key execution paths.

```bash
# Secure-by-default behavior masking all environment variables
cderun --dry-run node

# Selectively mask keys starting with DB_
cderun --sensitive-env "DB_*" --dry-run node
```

### `--entrypoint`

- **Type**: stringArray
- **Environment Variable**: `CDERUN_ENTRYPOINT`
- **Description**: Overwrite the default ENTRYPOINT of the image.

```bash
cderun --entrypoint /bin/sh node -c "ls"
```

### `--pull`

- **Type**: string
- **Default**: `missing`
- **Environment Variable**: `CDERUN_PULL`
- **Supported Values**: `always`, `missing`, `never`.
- **Validation**: Unrecognized values trigger an immediate invalid configuration error at startup.

```bash
cderun --pull always node
```

### `--pull-max-retries`

- **Type**: int
- **Default**: `3`
- **Environment Variable**: `CDERUN_PULL_MAX_RETRIES`
- **Description**: Maximum retry count for image pull operations.

### `--pull-backoff-base`

- **Type**: string (Duration)
- **Default**: `1s`
- **Environment Variable**: `CDERUN_PULL_BACKOFF_BASE`
- **Description**: Base exponential backoff duration for retry delays (e.g., `1s`, `500ms`).

### `--prefetch`

- **Type**: string (comma-separated list)
- **Default**: `""`
- **Environment Variable**: `CDERUN_PREFETCH`
- **Description**: Stand-alone prefetching mode. Prefetch specified tool images defined in `.tools.yaml` without executing container subcommands.
- **Format**: Comma-separated list of tool names (e.g., `node,python`).
- **Details**:
  - Looks up each specified tool in `.tools.yaml` and resolves dynamic expressions (such as `{{env:...}}` or `{{file:...}}`) in its image string.
  - Initializes the configured container runtime engine and pulls each image sequentially using the configured pull retry (`--pull-max-retries`) and exponential backoff (`--pull-backoff-base`) settings.
  - Returns an explicit error if any requested tool is not defined in `.tools.yaml`, lacks a configured image, or fails to pull.
  - Note that `--prefetch` is a scalar string flag (not a `stringArray`), whose handler splits comma-separated values (e.g., `--prefetch node,python`) rather than repeating the flag.
- **P1 Internal Override**: `--cderun-prefetch` is the corresponding Phase 1 (P1) internal override flag. It accepts a comma-separated list of tool names (e.g., `--cderun-prefetch=node,python` or `--cderun-prefetch node,python`).

```bash
cderun --prefetch node,python
```

### `--prefetch-all`

- **Type**: bool
- **Default**: `false`
- **Environment Variable**: `CDERUN_PREFETCH_ALL`
- **Description**: Stand-alone prefetching mode. Prefetch all tool images defined in `.tools.yaml` without executing container subcommands.
- **Details**: Scans all tools configured in `.tools.yaml`, resolves dynamic template expressions in their image strings, and pulls all tool images sequentially using the active pull policy and retry settings.
- **P1 Internal Override**: `--cderun-prefetch-all` is the corresponding Phase 1 (P1) internal override flag.

```bash
cderun --prefetch-all
```

### `--memory`, `-m`

- **Type**: string
- **Environment Variable**: `CDERUN_MEMORY`
- **Description**: Limit memory allocation (e.g., `512m`, `1g`).
- **Validation**: Validates that memory limits are non-negative.

```bash
cderun -m 512m node
```

### `--cpus`

- **Type**: float64
- **Environment Variable**: `CDERUN_CPUS`
- **Description**: Limit CPU allocation.
- **Validation**: Validates that CPU limits are non-negative.

```bash
cderun --cpus 1.5 node
```

### `--cpu-shares`

- **Type**: int64
- **Default**: `0` (relative default weights)
- **Environment Variable**: `CDERUN_CPU_SHARES`
- **Description**: Limit container CPU access weight (relative weight).
- **Details**:
  - **Docker**: Maps to `HostConfig.Resources.CPUShares`.
  - **containerd**: Maps to OCI spec limit `Linux.Resources.CPU.Shares = uint64(CPUShares)` (applies if value is greater than 0).
- **P1 Internal Override**: `--cderun-cpu-shares` is the corresponding Phase 1 (P1) internal override flag. It accepts an integer value and must be placed after the subcommand in Wrapper Mode.

```bash
cderun --cpu-shares 512 alpine sh
```

### `--cpuset-cpus`

- **Type**: string
- **Default**: `""`
- **Environment Variable**: `CDERUN_CPUSET_CPUS`
- **Description**: CPUs in which to allow execution (e.g., `0-3`, `0,1`).
- **Details**:
  - **Docker**: Maps to `HostConfig.Resources.CpusetCpus`.
  - **containerd**: Maps to OCI spec field `Linux.Resources.CPU.Cpus`.
- **P1 Internal Override**: `--cderun-cpuset-cpus` is the corresponding Phase 1 (P1) internal override flag. It accepts a string and must be placed after the subcommand in Wrapper Mode.

```bash
cderun --cpuset-cpus "0,1" alpine sh
```

### `--cpuset-mems`

- **Type**: string
- **Default**: `""`
- **Environment Variable**: `CDERUN_CPUSET_MEMS`
- **Description**: Memory nodes (MEMs) in which to allow execution (e.g., `0-3`, `0,1`). Only effective on NUMA systems.
- **Details**:
  - **Docker**: Maps to `HostConfig.Resources.CpusetMems`.
  - **containerd**: Maps to OCI spec field `Linux.Resources.CPU.Mems`.
- **P1 Internal Override**: `--cderun-cpuset-mems` is the corresponding Phase 1 (P1) internal override flag. It accepts a string and must be placed after the subcommand in Wrapper Mode.

```bash
cderun --cpuset-mems "0" alpine sh
```

### `--sysctl`

- **Type**: stringArray
- **Environment Variable**: `CDERUN_SYSCTL`
- **Description**: Configure kernel parameters (sysctl) at runtime.
- **Format**: `key=value` (e.g., `net.ipv4.ip_forward=1`).
- **Validation**: Param values must be in key=value format and support dynamic expression resolution.
- **Details**:
  - **Docker / Podman**: Maps directly to `Sysctls` map inside `HostConfig`.
  - **containerd**: Maps directly to OCI specification's `Linux.Sysctl` (`map[string]string`).

```bash
cderun --sysctl net.ipv4.ip_forward=1 alpine sh
```

### `--device`

- **Type**: stringArray
- **Environment Variable**: `CDERUN_DEVICE`
- **Description**: Add a host device to the container.
- **Format**: `<host-path>:<container-path>[:<permissions>]` (e.g., `/dev/fuse:/dev/fuse:rwm`).
- **Permissions Validation**: Permissions are strictly validated against `^[rwm]+$`.
- **Host Security Auditing**: Highly sensitive host paths (such as `/dev/mem`, `/dev/kmem`, `/dev/port`, and block devices like `/dev/sd*`, `/dev/nvme*`, `/dev/loop*`, `/dev/mapper/*`) are scanned and trigger explicit security warnings at the `Warn` log level.
- **Container Destination Safety**: Container-side destination paths (`dc.Destination`) must be absolute and non-empty. Relative target directories are strictly blocked from conversion into absolute host directories, guaranteeing immediate validation errors.

```bash
cderun --device /dev/fuse alpine ls /dev/fuse
```

### `--config`

- **Type**: string
- **Environment Variable**: `CDERUN_CONFIG`
- **Description**: Path to `cderun` configuration file (overriding search logic). Supports tilde (`~`) expansions.

### `--tool-config`

- **Type**: string
- **Environment Variable**: `CDERUN_TOOL_CONFIG`
- **Description**: Path to `tools` configuration file (overriding search logic). Supports tilde (`~`) expansions.

### `--dry-run`

- **Type**: bool
- **Default**: `false`
- **Environment Variable**: `CDERUN_DRY_RUN`
- **Description**: Generate and output the container configuration intermediate representation without executing the container.

```bash
cderun --dry-run node --version
```

### `--dry-run-format`, `-f`

- **Type**: string
- **Default**: `yaml`
- **Environment Variable**: `CDERUN_DRY_RUN_FORMAT`
- **Supported Formats**: `yaml`, `json`, `simple`.

### `--diagnosis`

- **Type**: bool
- **Default**: `false`
- **Environment Variable**: `CDERUN_DIAGNOSIS`
- **Description**: Display active container engine diagnostics and available tools. No subcommand is required.

```bash
cderun --diagnosis
```

### `--diagnosis-format`

- **Type**: string
- **Default**: `yaml`
- **Environment Variable**: `CDERUN_DIAGNOSIS_FORMAT`
- **Supported Formats**: `yaml`, `json`, `simple`.

### `--hang-timeout`

- **Type**: string (Duration)
- **Default**: `10s`
- **Environment Variable**: `CDERUN_HANG_TIMEOUT`
- **Description**: Period to wait for the container to terminate after I/O finishes in non-TTY, non-interactive environments before sending `SIGKILL`.
- **Special Values**: `0` or `<= 0` disables the timeout, waiting indefinitely.
- **Premature Attach Handling**: If an attach error occurs before container termination, setting `hang-timeout` to `<= 0` prevents immediate execution cutoff, causing the engine to block synchronously until the container naturally finishes.

```bash
cderun --hang-timeout 5s node script.js
```

### `--log-level`

- **Type**: string
- **Default**: `error`
- **Environment Variable**: `CDERUN_LOG_LEVEL`
- **Supported Values**: `error`, `warn`, `warning` (alias), `info`, `debug`, `trace`.

### `--log-format`

- **Type**: string
- **Default**: `text`
- **Environment Variable**: `CDERUN_LOG_FORMAT`
- **Supported Values**: `text`, `json`.

### `--log-timestamp`

- **Type**: bool
- **Default**: `true`
- **Environment Variable**: `CDERUN_LOG_TIMESTAMP`
- **Description**: Print timestamps in log outputs.

---

## P1 Internal Overrides (`--cderun-*`)

Standard command-line flags registered in `BoolOptions`, `StringOptions`, `IntOptions`, `Float64Options`, and `StringSliceOptions` have a corresponding `--cderun-` prefixed counterpart (e.g., `--cderun-tty`, `--cderun-image`, `--cderun-env`).

- **Precedence**: These internal overrides represent Phase 1 (P1) configurations, taking precedence over all other configuration layers (P2 through P6).
- **Hoisting Mechanics**: During argument preprocessing (`preprocessArgs`), any `--cderun-*` flags placed **after** the subcommand are extracted and relocated **before** the subcommand internally prior to Cobra flag parsing.
- **Equals-Sign & Space-Separated Formats**: Value-taking internal overrides can be specified using either the equals-sign format (e.g., `--cderun-image=alpine`) or the space-separated format (e.g., `--cderun-image alpine`). `cderun`'s preprocessor uses option registration metadata to correctly identify value-taking options and consumes the subsequent adjacent parameter as their value during the hoisting scan.
- **Adjacent Parameter Protection**: If a value-taking `--cderun-` flag in space-separated format is followed immediately by another `--cderun-` flag, preprocessing strictly rejects it with a validation error to prevent parsing corruption.
- **Boolean Override Toggles**: Boolean overrides (e.g., `--cderun-tty`, `--cderun-read-only`, `--cderun-privileged`) do not consume subsequent adjacent arguments and are hoisted autonomously. Optional inline boolean values may be passed using the equals-sign format (e.g., `--cderun-tty=false`).
- **Symlink / Polyglot Isolation**: In Symlink Mode, `--cderun-*` flags are extracted and hoisted without modifying any arguments destined for the wrapped executable.
