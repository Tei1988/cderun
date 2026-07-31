# Command-Line Options Reference

## Overview

This document provides a comprehensive reference of all command-line options supported by `cderun`.

### List-Type (Array-Type) Options and Environment Variable Separator Rules

When supplying multiple values for a list-type option (e.g., `stringArray` or `[]string`) using environment variables (P3), `cderun` enforces specific separator rules depending on the variable:

- **Semicolon (`;`) Separator**:
  - `CDERUN_ENV` (e.g., `export CDERUN_ENV="KEY1=val1;KEY2=val2"`)
  - `CDERUN_MOUNT` (e.g., `export CDERUN_MOUNT="type=bind,source=./src,target=/app;type=tmpfs,target=/tmp"`)
- **Comma (`,`) Separator**:
  - `CDERUN_GROUP_ADD`
  - `CDERUN_MOUNT_TOOLS`
  - `CDERUN_DEVICE`
  - `CDERUN_PUBLISH`
  - `CDERUN_EXPOSE`
  - `CDERUN_DNS`
  - `CDERUN_ADD_HOST`
  - `CDERUN_CAP_ADD`
  - `CDERUN_CAP_DROP`
  - `CDERUN_ENTRYPOINT`
  - `CDERUN_SENSITIVE_ENV`

*Note: When passing list-type options via CLI flags (P1/P2), separators are not used. Instead, repeat the flag (e.g., `--env A=1 --env B=2` or `--dns 8.8.8.8 --dns 1.1.1.1`).*

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
- **Default**: Auto-detected (`docker` -> `containerd` -> `podman`)
- **Environment Variable**: `CDERUN_RUNTIME`
- **Supported Engines**: `docker`, `podman`, `containerd`.

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

```bash
cderun --read-only alpine touch /test-write
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

### `--sensitive-env`

- **Type**: stringArray
- **Environment Variable**: `CDERUN_SENSITIVE_ENV`
- **Description**: List of environment variable patterns (glob format) to mask.
- **Secure-by-Default Architecture**:
  - **Default (Unset)**: All environment variables are treated as sensitive, masking values of non-empty keys as `[REDACTED]` in dry-runs and debug logs.
  - **Explicit Disable**: Pass `--sensitive-env=""` (or YAML empty list `sensitiveEnv: []`) to completely disable masking.
  - **Fail-Closed Fallback**: If any glob pattern syntax is malformed (e.g., `[` with no closing bracket), `cderun` safely falls back to masking all non-empty environment values to prevent accidental credential leakage.

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
- **Default**: `warn`
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

Every standard command-line flag has a corresponding `--cderun-` prefixed counterpart (e.g., `--cderun-tty`, `--cderun-image`).

- **Precedence**: These internal overrides represent Phase 1 (P1) configurations, taking precedence over all other configuration layers.
- **Hoisting Mechanics**: During argument preprocessing, any `--cderun-*` flags placed **after** the subcommand are extracted and relocated **before** the subcommand internally.
- **Equals-Sign Format Constraints**: All value-taking internal overrides **must** use the equals-sign format (e.g., `--cderun-image=alpine`). Supplying a value-taking override flag without an equals-sign (e.g., `--cderun-image alpine`) is strictly rejected with a preprocessing validation error to guarantee robust argument parsing.
