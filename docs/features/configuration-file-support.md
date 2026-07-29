# Configuration File Support

## Overview

`cderun` separates its own runtime behaviors from the container configurations of individual subcommands (tools) by utilizing two separate configuration files. This decouples system configurations from individual development environment settings.

---

## Configuration Architecture

`cderun` supports configurations defined in YAML format across two standard files:

1. **`.cderun.yaml`**: Contains settings for `cderun` itself (e.g., container runtime engine, default CLI values, logger verbosity/format).
2. **`.tools.yaml`**: Contains tool execution settings mapped by subcommand names (e.g., image tags, bind mounts, and specific environment variables).

### Strict Decoding Behavior

Both configuration files are parsed using strict decoding mechanics (`KnownFields` verification is enabled). Any typo, unknown field, or malformed property within the YAML file will immediately fail configuration loading with a descriptive syntax or decoding error.

### Reserved (Non-configurable) Fields

Certain CLI options are utilized to locate configuration files. If these options are defined *inside* the configuration files themselves, it would create circular dependency locks. Therefore, the following options are **strictly prohibited** in `.cderun.yaml` and `.tools.yaml`. If present, strict decoding will fail with an error:

- **`config`** (CLI: `--config` / environment: `CDERUN_CONFIG`): Specifies the path of `.cderun.yaml`
- **`toolConfig`** (CLI: `--tool-config` / environment: `CDERUN_TOOL_CONFIG`): Specifies the path of `.tools.yaml`

---

## Hierarchical Searching & Merging

`cderun` looks up configuration files in the host environment and merges them hierarchically based on priority.

### 1. File Selection Override

Specifying an explicit file path via CLI flags (P1/P2) or environment variables (P3) **completely skips** the automatic hierarchical searching, loading only the specified file:

- **`.cderun.yaml` Path Overrides**: `--config <path>` (P2), `--cderun-config <path>` (P1), or `CDERUN_CONFIG=<path>` (P3).
- **`.tools.yaml` Path Overrides**: `--tool-config <path>` (P2), `--cderun-tool-config <path>` (P1), or `CDERUN_TOOL_CONFIG=<path>` (P3).

Paths specified in overrides support tilde expansion (e.g., `~/.cderun.yaml`). Relative paths configured inside files resolve relative to the directory containing that file.

---

### 2. Automatic Search Order (Hierarchical Priority)

If no path override is specified, `cderun` dynamically searches for configurations in the following priority order (first match wins/overrides subsequent layers):

1. **Project-level (Upward Directory Search)**:
   - Starts at the execution directory (`{{PWD}}`) and iteratively traverses up toward the root directory (`/`) looking for `.cderun.yaml` and `.tools.yaml`.
   - Allows fine-tuning of tools and defaults per repository or project tree.
2. **User-level**:
   - `~/.config/cderun/.cderun.yaml`
   - `~/.config/cderun/.tools.yaml`
3. **System-level**:
   - `/etc/cderun/.cderun.yaml`
   - `/etc/cderun/.tools.yaml`
4. **Nested Execution Injection**:
   - `/run/cderun/.cderun.yaml`
   - `/run/cderun/.tools.yaml`
   - *Note: These files are dynamically mapped under `/run/cderun/` during recursive nested runs.*

---

### 3. Merging Rules & List Collection Overriding

- **Scalar Options**: Primitive/scalar options (such as strings, integers, floats, and booleans) are merged field-by-field. Higher-priority layers overwrite individual fields of lower-priority layers.
- **List and Array Options (Collections)**: List-type options (such as `mounts`, `env`, `ports`, `groupAdd`, `devices`, `sensitiveEnv`) follow an **"All-or-Nothing Replacement"** rule. They do *not* merge or append values across different priority layers. If a higher-priority config file defines a list-type option, it completely discards any definitions of that same option found in lower-priority files.
- **Explicit Empty List Override**: Defining an explicit empty list (e.g., `ports: []` or `env: []`) in a higher-priority configuration file will completely clear out the respective collection, preventing lower-priority values from leaking.

---

## Configuration Schemas

Key names in configuration files are defined in standard **camelCase**, matching their respective CLI names.

> **Exception**: Individual items inside `mounts` and `devices` configuration blocks support certain snake_case properties (e.g., `read_only`, `optional`, `path_on_host`) to align with OCI or Docker specifications.
>
> **Important Distinction on Mount Options**:
>
> - **YAML Configurations**: Must use the snake_case keys `read_only` and `optional` inside `.cderun.yaml` or `.tools.yaml`.
> - **CLI Arguments**: Use the standard `--mount` key-value syntax (e.g., `--mount type=bind,source=...,target=...,readonly,optional`). Note that CLI parsing and dry-run formats output and use `readonly`, whereas YAML configurations require `read_only`.
> - **Syntax Warning**: Do **not** copy CLI `--mount` key-value strings or `--mount` syntax directly as list items under YAML `mounts`. They must be written as structured YAML maps conforming to the schema described below.

### `.cderun.yaml` Schema

- **`runtime`** (string): Container runtime to connect with (`docker` | `podman` | `containerd`).
- **`socketPath`** (string): Absolute path to the runtime engine socket on the host.
- **`defaults`** (object): Specifies default configuration settings for CLI options (accepts all standard options like `tty`, `interactive`, `remove`, `strictEnv`, `network`, etc.).
- **`logging`** (object): Configure logging output.
  - `level` (string): Logger verbosity level (`error` | `warn` | `info` | `debug` | `trace`).
  - `format` (string): Logger format (`text` | `json`).
  - `timestamp` (bool): If `true`, prints timestamps in logs.

#### Sample `.cderun.yaml`

```yaml
runtime: docker
socketPath: /var/run/docker.sock
defaults:
  tty: true
  interactive: true
  remove: true
  network: bridge
  pull: missing
  pullMaxRetries: 3
  pullBackoffBase: 1s
  hangTimeout: 10s
logging:
  level: warn
  format: text
  timestamp: true
```

---

### `.tools.yaml` Schema

Mappings configured as `<tool_name>: <config_block>`. The `<config_block>` accepts all options available in `defaults` of `.cderun.yaml`, along with these additional or overridden fields:

- **`image`** (string, **required**): Image tag or resolution expression (e.g., `"node:20-alpine"`).
- **`logLevel`, `logFormat`** (string): Tool-specific logger overrides.
- **`logTimestamp`** (bool): Tool-specific log timestamp control.

#### Sample `.tools.yaml`

```yaml
node:
  image: "node:20-alpine"
  workdir: /workspace
  mounts:
    - type: bind
      source: .
      target: /workspace
      read_only: false
    - type: bind
      source: ~/.npmrc
      target: /root/.npmrc
      read_only: true
  env:
    - NODE_ENV=development

python:
  image: "python:3.11-slim"
  workdir: /app
  mounts:
    - type: bind
      source: .
      target: /app
  env:
    - PYTHONUNBUFFERED=1
```

---

## Detailed Block Configurations

### 1. Mount Configurations (`mounts`)

Each item in the `mounts` list supports the following attributes:

- `type` (string): Mount strategy (`bind` | `volume` | `tmpfs`). Default: `bind`.
- `source` (string): Host filesystem path (required for `bind` type).
- `target` (string, **required**): Destination absolute path inside the container.
- `read_only` (bool): Mount the path as read-only inside the container.
- `optional` (bool): For `bind` type only. If `true`, the mount is silently skipped if the source path does not exist on the host filesystem.

### 2. Device Configurations (`devices`)

Accepts both structured objects and shorthand colon-delimited strings.

#### Object Notation

The following config-level fields in `.tools.yaml` or `.cderun.yaml` map directly to dry-run output properties:

- `source` (string, **required**): Path to the device on the host. Maps to dry-run field `path_on_host`.
- `destination` (string, **required**): Path to map the device inside the container. Maps to dry-run field `path_in_container`.
- `permissions` (string): Access permissions (e.g., `rwm`). Default: `rwm`. Maps to dry-run field `cgroup_permissions`.

#### String Notation

Syntax: `<host-path>:<container-path>[:<permissions>]` (e.g., `/dev/fuse:/dev/fuse:rwm`). Permissions are strictly validated against `^[rwm]+$`.
These colon-delimited segments map respectively to `path_on_host`, `path_in_container`, and `cgroup_permissions` in the dry-run output and intermediate representation.

```yaml
devices:
  - source: /dev/fuse
    destination: /dev/fuse
    permissions: rwm
  - "/dev/snd:/dev/snd:rw"
```
