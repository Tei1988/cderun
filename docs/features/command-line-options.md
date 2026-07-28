# Command-Line Options Reference

## Overview

This document provides a comprehensive reference of all command-line options available in `cderun`.

### List-Type (Array) Options and Environment Variable Separator Rules

When configuring list-type options (`stringArray` CLI flags) using environment variables (P3), different variables use different separators to parse multi-value inputs:

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

*Note: When using CLI flags (P1/P2) instead of environment variables, values must be specified by repeating the flag (e.g., `--env A=1 --env B=2` or `--dns 8.8.8.8 --dns 1.1.1.1`) rather than using separators.*

## Basic Syntax

```bash
cderun [cderun-flags] <subcommand> [passthrough-args]
```

- **`[cderun-flags]`**: Options to control `cderun` execution. Standard flags (P2) must be placed **before** the subcommand.
- **`<subcommand>`**: The first non-flag argument (e.g., `node`, `python`). It serves as the lookup key in `.tools.yaml`.
- **`[passthrough-args]`**: Arguments passed directly to the subcommand. Flags prefixed with `--cderun-` are parsed as `cderun`'s highest priority overrides (P1) and are hoisted during preprocessing, while all other arguments are passed intact to the subcommand in the container.

---

## Global Options

### `--tty`, `-t`

- **Type**: bool
- **Default**: `false`
- **Environment Variable**: `CDERUN_TTY`
- **Description**: Allocate a pseudo-TTY for interactive container execution.
- **Example**:

  ```bash
  cderun --tty bash
  cderun -t node
  ```

### `--interactive`, `-i`

- **Type**: bool
- **Default**: `false`
- **Environment Variable**: `CDERUN_INTERACTIVE`
- **Description**: Keep STDIN open even if not attached.
- **Example**:

  ```bash
  cderun --interactive python
  cderun -i bash
  ```

### `--network`

- **Type**: string
- **Default**: `bridge`
- **Environment Variable**: `CDERUN_NETWORK`
- **Description**: Network mode to connect the container. Supported values: `bridge`, `host`, `none`, or a custom network name.
- **Example**:

  ```bash
  cderun --network host node server.js
  ```

### `--socket-path`

- **Type**: string
- **Default**: Auto-detected (e.g., `/var/run/docker.sock`)
- **Environment Variable**: `CDERUN_SOCKET_PATH`
- **Description**: Host path to the container runtime socket.
- **Example**:

  ```bash
  cderun --socket-path /var/run/docker.sock docker ps
  ```

### `--mount-socket`

- **Type**: bool
- **Default**: `false`
- **Environment Variable**: `CDERUN_MOUNT_SOCKET`
- **Description**: Mount the host container runtime socket into the container. Required for Docker-in-Docker or nested `cderun` executions.
- **Example**:

  ```bash
  cderun --mount-socket docker ps
  ```

### `--mount-socket-path`

- **Type**: string
- **Default**: Same as the host socket path
- **Environment Variable**: `CDERUN_MOUNT_SOCKET_PATH`
- **Description**: Container path where the socket should be mounted.
- **Example**:

  ```bash
  cderun --mount-socket --mount-socket-path /var/run/docker.sock node app.js
  ```

### `--mount-cderun`

- **Type**: bool
- **Default**: `false`
- **Environment Variable**: `CDERUN_MOUNT_CDERUN`
- **Description**: Mount the `cderun` binary into the container at `/usr/local/bin/cderun` to support nested execution.
- **Notes**: Automatically enabled when `--mount-tools` or `--mount-all-tools` is specified. It also automatically triggers `--mount-socket` unless explicitly disabled.
- **Example**:

  ```bash
  cderun --mount-cderun alpine sh
  ```

### `--mount-cderun-path`

- **Type**: string
- **Environment Variable**: `CDERUN_MOUNT_CDERUN_PATH`
- **Description**: Specify the physical host path to the `cderun` binary to mount (essential on macOS to mount a Linux-compatible cross-compiled binary instead of the Darwin binary).
- **Example**:

  ```bash
  cderun --mount-cderun --mount-cderun-path ./cderun_linux_arm64 alpine sh
  ```

### `--mount-tools`

- **Type**: string
- **Environment Variable**: `CDERUN_MOUNT_TOOLS`
- **Description**: Mount specified tools defined in `.tools.yaml` (comma-separated list) into the container as `cderun` symlinks.
- **Notes**: Automatically enables `--mount-cderun` and `--mount-socket`.
- **Example**:

  ```bash
  cderun --mount-tools node,python alpine sh
  ```

### `--mount-all-tools`

- **Type**: bool
- **Default**: `false`
- **Environment Variable**: `CDERUN_MOUNT_ALL_TOOLS`
- **Description**: Mount all tools defined in `.tools.yaml` into the container as `cderun` symlinks.
- **Notes**: Automatically enables `--mount-cderun` and `--mount-socket`.
- **Example**:

  ```bash
  cderun --mount-all-tools alpine sh
  ```

### `--image`

- **Type**: string
- **Environment Variable**: `CDERUN_IMAGE`
- **Description**: Explicitly specify the container image to run (overrides the configured image in `.tools.yaml`).
- **Example**:

  ```bash
  cderun --image node:20-alpine node --version
  ```

### `--env`, `-e`

- **Type**: stringArray
- **Environment Variable**: `CDERUN_ENV`
- **Description**: Set environment variables. Supports `KEY=value` (explicit assignment) and `KEY` (passthrough from host).
- **Notes**: CLI flags must be repeated (e.g., `-e A=1 -e B=2`). Semicolons are used as separators in environment variables. Supports expression resolution (e.g., `{{PWD}}`).
- **Example**:

  ```bash
  cderun --env NODE_ENV=production node app.js
  ```

### `--cderun-env`

- **Type**: stringArray
- **Description**: High-priority environment variable override (P1 resolution). Can be placed after the subcommand.
- **Example**:

  ```bash
  cderun node app.js --cderun-env NODE_ENV=production
  ```

### `--mount`

- **Type**: stringArray
- **Environment Variable**: `CDERUN_MOUNT`
- **Description**: Mount configurations. Supports `type=bind`, `type=volume`, and `type=tmpfs`.
- **Keywords**:
  - `type`: `bind` | `volume` | `tmpfs`
  - `source` (alias `src`): Path on the host.
  - `target` (alias `dst`, `destination`): Path inside the container (must be absolute).
  - `readonly`: Mount as read-only.
  - `optional`: (For `bind` type only) Skip mounting silently instead of returning an error if the host-side source directory/file does not exist.
- **Example**:

  ```bash
  cderun --mount type=bind,source=./data,target=/data python script.py
  cderun --mount type=bind,source=./config,target=/config,optional node app.js
  ```

### `--workdir`, `-w`

- **Type**: string
- **Environment Variable**: `CDERUN_WORKDIR`
- **Description**: Working directory inside the container. Reject paths containing directory traversal (`..`) if configured.
- **Example**:

  ```bash
  cderun --workdir /app node server.js
  ```

### `--strict-env`

- **Type**: bool
- **Default**: `false`
- **Environment Variable**: `CDERUN_STRICT_ENV`
- **Description**: Rejects execution with an error if any requested passthrough environment variable is missing on the host.
- **Example**:

  ```bash
  cderun --strict-env --env NPM_TOKEN node app.js
  ```

### `--runtime`

- **Type**: string
- **Environment Variable**: `CDERUN_RUNTIME`
- **Description**: Container runtime to use (`docker` | `podman` | `containerd`). If unspecified, dynamically auto-detected.
- **Example**:

  ```bash
  cderun --runtime podman node app.js
  ```

### `--remove`

- **Type**: bool
- **Default**: `true`
- **Environment Variable**: `CDERUN_REMOVE`
- **Description**: Automatically remove the container when it exits.
- **Example**:

  ```bash
  cderun --remove=false node app.js
  ```

### `--publish`, `-p`

- **Type**: stringArray
- **Environment Variable**: `CDERUN_PUBLISH`
- **Description**: Publish container ports to the host (`hostPort:containerPort`).
- **Example**:

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
- **Description**: Expose a port or a range of ports without publishing them to the host.
- **Example**:

  ```bash
  cderun --expose 80 node app.js
  ```

### `--hostname`

- **Type**: string
- **Environment Variable**: `CDERUN_HOSTNAME`
- **Description**: Hostname of the container. Must be a valid FQDN.
- **Example**:

  ```bash
  cderun --hostname my-container.local alpine hostname
  ```

### `--dns`

- **Type**: stringArray
- **Environment Variable**: `CDERUN_DNS`
- **Description**: Set custom DNS servers for the container.
- **Example**:

  ```bash
  cderun --dns 8.8.8.8 alpine ping google.com
  ```

### `--add-host`

- **Type**: stringArray
- **Environment Variable**: `CDERUN_ADD_HOST`
- **Description**: Add custom host-to-IP mappings to the container's `/etc/hosts` (`host:ip`).
- **Example**:

  ```bash
  cderun --add-host my-server:192.168.1.10 alpine ping my-server
  ```

### `--user`, `-u`

- **Type**: string
- **Environment Variable**: `CDERUN_USER`
- **Description**: Specify the username or UID (and optionally group or GID) under which to run the container (`<name|uid>[:<group|gid>]`).
- **Example**:

  ```bash
  cderun -u 1000:1000 alpine whoami
  ```

### `--group-add`

- **Type**: stringArray
- **Environment Variable**: `CDERUN_GROUP_ADD`
- **Description**: Add supplementary user groups (group names or numeric GIDs) to the container's running user.
- **Notes**:
  - Direct `containerd` runtime supports **numeric GIDs only** (e.g., `"102"`), as containerd does not perform group name resolutions inside the container context. Non-numeric group names under containerd will fail validation.
  - Initial input values are strictly validated using regular expressions to prevent control character injections.
- **Example**:

  ```bash
  cderun --group-add 1001 --group-add 102 alpine id
  ```

### `--privileged`

- **Type**: bool
- **Default**: `false`
- **Environment Variable**: `CDERUN_PRIVILEGED`
- **Description**: Give extended privileges to the container (runs in privileged mode).
- **Example**:

  ```bash
  cderun --privileged alpine ls /dev
  ```

### `--cap-add`

- **Type**: stringArray
- **Environment Variable**: `CDERUN_CAP_ADD`
- **Description**: Add Linux capabilities.
- **Notes**:
  - Scanning is performed for highly privileged capabilities (e.g., `SYS_ADMIN`, `NET_ADMIN`, `SYS_RAWIO`, `SYS_PTRACE`, `SYS_MODULE`, `ALL`) with or without the `CAP_` prefix, logging explicit warnings at the `Warn` level when detected.
- **Example**:

  ```bash
  cderun --cap-add SYS_ADMIN alpine mount ...
  ```

### `--cap-drop`

- **Type**: stringArray
- **Environment Variable**: `CDERUN_CAP_DROP`
- **Description**: Drop Linux capabilities from the container.

### `--sensitive-env`

- **Type**: stringArray
- **Environment Variable**: `CDERUN_SENSITIVE_ENV`
- **Description**: Patterns used to match environment variable keys to mask (redact) in non-execution outputs (dry-run, diagnosis, logs).
- **Details (Secure by Default)**:
  - **Unset (Default)**: Masks **all** non-empty environment values as `[REDACTED]` (Secure by Default). Empty variables remain unmasked.
  - **Explicit Empty String (`""`) / Empty Array (`[]`)**: Completely disables environment variable masking.
  - **Explicit Patterns**: Non-empty glob patterns (e.g., `DB_*`, `*_TOKEN`) selectively mask matching environment keys, showing others in plaintext.
  - **Fail-Closed Fallback**: If pattern compilation/matching fails, fallback to masking all non-empty values (Fail-Closed).
- **Example**:

  ```bash
  cderun --sensitive-env "DB_*" --dry-run node
  ```

### `--entrypoint`

- **Type**: stringArray
- **Environment Variable**: `CDERUN_ENTRYPOINT`
- **Description**: Overwrite the default ENTRYPOINT of the container image.
- **Example**:

  ```bash
  cderun --entrypoint /bin/sh node -c "ls"
  ```

### `--pull`

- **Type**: string
- **Default**: `missing`
- **Environment Variable**: `CDERUN_PULL`
- **Description**: Execution pull policy. Supported values: `always`, `missing`, `never`. Any other unrecognized values are strictly validated and blocked during resolved settings verification.

### `--pull-max-retries`

- **Type**: int
- **Default**: `3`
- **Environment Variable**: `CDERUN_PULL_MAX_RETRIES`
- **Description**: Maximum retry count for image pulls on rate limits or transient errors (must be 1 or greater).

### `--pull-backoff-base`

- **Type**: string (Duration)
- **Default**: `1s`
- **Environment Variable**: `CDERUN_PULL_BACKOFF_BASE`
- **Description**: Base exponential backoff duration for pull retries.

### `--memory`, `-m`

- **Type**: string
- **Environment Variable**: `CDERUN_MEMORY`
- **Description**: Memory limit configuration (e.g., `512m`, `1g`, etc.). Supports parsing up to `TiB` but rejects `EiB`. Negative boundaries are validated and rejected.

### `--cpus`

- **Type**: float64
- **Environment Variable**: `CDERUN_CPUS`
- **Description**: CPU resource limit (must be non-negative).

### `--device`

- **Type**: stringArray
- **Environment Variable**: `CDERUN_DEVICE`
- **Description**: Mount physical host devices into the container.
- **Syntax**: `<host-path>:<container-path>[:<cgroup-permissions>]` (e.g., `/dev/fuse:/dev/fuse:rwm`).
- **Notes**: Permissions are strictly validated against `^[rwm]+$` to block parameter injections.

### `--config`

- **Type**: string
- **Environment Variable**: `CDERUN_CONFIG`
- **Description**: Explicitly specify the `.cderun.yaml` global settings file path (bypasses automatic search). Supports tilde expansion (`~`).

### `--tool-config`

- **Type**: string
- **Environment Variable**: `CDERUN_TOOL_CONFIG`
- **Description**: Explicitly specify the `.tools.yaml` tools settings file path (bypasses automatic search). Supports tilde expansion (`~`).

### `--dry-run`

- **Type**: bool
- **Default**: `false`
- **Environment Variable**: `CDERUN_DRY_RUN`
- **Description**: Render and display the parsed container configuration (intermediate `ContainerConfig` representation) without actually executing the container.
- **Notes**: Requires a subcommand lookup key.
- **Example**:

  ```bash
  cderun --dry-run node
  ```

### `--dry-run-format`, `-f`

- **Type**: string
- **Default**: `yaml`
- **Environment Variable**: `CDERUN_DRY_RUN_FORMAT`
- **Description**: Specify the dry-run output format (`yaml`, `json`, `simple`).

### `--diagnosis`

- **Type**: bool
- **Default**: `false`
- **Environment Variable**: `CDERUN_DIAGNOSIS`
- **Description**: Display CLI execution diagnostics, cache statuses, and available mapped tools (does not require a subcommand).
- **Example**:

  ```bash
  cderun --diagnosis
  ```

### `--diagnosis-format`

- **Type**: string
- **Default**: `yaml`
- **Environment Variable**: `CDERUN_DIAGNOSIS_FORMAT`
- **Description**: Format to render diagnosis output (`yaml`, `json`, `simple`).

### `--hang-timeout`

- **Type**: string (Duration)
- **Default**: `10s`
- **Environment Variable**: `CDERUN_HANG_TIMEOUT`
- **Description**: Grace timeout after I/O finishes before terminating the container in non-TTY, non-interactive environments.
- **Notes**:
  - If `<= 0` (or `0`), `cderun` will wait indefinitely for the container process to naturally exit.
  - If a premature attach communication error occurs, `cderun` uses the hang-timeout check: if it is `0` or `<= 0`, it blocks indefinitely on `waitDone` to gracefully wait for exit rather than exiting prematurely.

### `--log-level`

- **Type**: string
- **Default**: `warn`
- **Environment Variable**: `CDERUN_LOG_LEVEL`
- **Description**: Internal logger verbosity (`error`, `warn` / `warning`, `info`, `debug`, `trace`). Verified and loaded early in the execution flow.

### `--log-format`

- **Type**: string
- **Default**: `text`
- **Environment Variable**: `CDERUN_LOG_FORMAT`
- **Description**: Logger output format (`text`, `json`).

### `--log-timestamp`

- **Type**: bool
- **Default**: `true`
- **Environment Variable**: `CDERUN_LOG_TIMESTAMP`
- **Description**: Control whether timestamps are outputted in logs.

---

## P1 "Internal Override" Flags (`--cderun-*`)

To prevent arguments passed to subcommands from conflicting with `cderun` configuration settings, `cderun` supports high-priority **Internal Override (P1)** flags.

Every CLI option listed above has an equivalent `--cderun-` prefixed counterpart (e.g., `--cderun-image`, `--cderun-tty`, `--cderun-env`).

### Hoisting Mechanics

In **Wrapper Mode**, internal override flags must be placed **after** the subcommand lookup key. During argument preprocessing, `cderun` scans the arguments, identifies the subcommand boundary, extracts all `--cderun-*` prefixed flags, and hoists them internally to the front for parsing, separating them from the direct container command arguments.

```bash
# Correct Wrapper Mode P1 usage:
cderun node app.js --cderun-image node:20-alpine --cderun-tty

# Incorrect usage (causes verification error):
cderun --cderun-image node:20-alpine node app.js
```

### Double-Dash (`--`) Delimiter Support

To completely skip hoisting and pass literal `--cderun-` strings directly to the container, place them after a double-dash (`--`) delimiter:

```bash
# The '--cderun-tty' string is passed literally as an argument to 'echo'
cderun echo -- --cderun-tty
```
