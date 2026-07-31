# Feature Specification: Configuration File Support

## Overview

`cderun` supports separating runner execution defaults from tool-specific containerization configurations.

---

## Configuration Files

The configuration structure is divided into two separate files:

1. **`.cderun.yaml`**: Governs global execution settings for `cderun` (such as runner engine selection, logging parameters, and default options).
2. **`.tools.yaml`**: Specifies execution parameters for individual subcommands/tools (such as image mappings, custom bind mounts, and default environment variables).

### Format and Decoding Rules

- Only **YAML** format is supported.
- **Strict Decoding**: To prevent configuration errors and silent typos, configurations are decoded strictly (using `KnownFields` verification). The presence of any unregistered keys or unrecognized options immediately aborts execution with a syntax/decoding error.
- **Standard Filenames**: `.cderun.yaml` and `.tools.yaml` are the standard filenames.

### Unbound (Non-Configurational) Options

Options used to determine the paths of configuration files themselves cannot be specified within configuration files. These options are:

- **`config`** (`--config`): Path to the `cderun` configuration file.
- **`toolConfig`** (`--tool-config`): Path to the `tools` configuration file.

*Specifying these keys inside any `.cderun.yaml` or `.tools.yaml` file is strictly prohibited and triggers a decoding validation error.*

### Overriding Configuration Paths

Users can bypass standard search sequences and explicitly specify configuration files using command-line flags or environment variables:

| Setting Target | Standard Flag (P2) | Internal Override (P1) | Environment Variable (P3) |
| :--- | :--- | :--- | :--- |
| **cderun Config** | `--config <path>` | `--cderun-config <path>` | `CDERUN_CONFIG` |
| **tools Config** | `--tool-config <path>` | `--cderun-tool-config <path>` | `CDERUN_TOOL_CONFIG` |

- **Behavior**: Specifying these paths deactivates directory-traversal searches. Only the designated files are loaded. If the specified files do not exist, execution aborts with an error.
- **Tilde Expansion**: Paths starting with `~` or `~/` are expanded to the user's home directory.
- **Relative Paths**: Relative paths (such as `./custom.yaml`) specified within explicit configurations are resolved relative to the directory containing the file itself.

---

## Hierarchical Search and Merging Rules

To provide flexible defaults and project-specific configurations, `cderun` searches multiple locations and merges discovered configurations.

### Search Priority Sequence

Configurations are loaded in ascending order of priority (higher layers override lower layers):

1. **Nested execution injection** (P5 / P4 defaults):
   - `/run/cderun/.cderun.yaml`
   - `/run/cderun/.tools.yaml`
   *Note: Dynamically generated and mounted during nested execution (`--mount-cderun`).*
2. **System-wide defaults**:
   - `/etc/cderun/.cderun.yaml`
   - `/etc/cderun/.tools.yaml`
3. **User-level configuration**:
   - `~/.config/cderun/.cderun.yaml`
   - `~/.config/cderun/.tools.yaml`
4. **Project-level directory-traversal**:
   - Starts at the execution host's current working directory and traverses upwards towards the file system root (`/`), searching for `.cderun.yaml` and `.tools.yaml` at each level. Configurations found closer to the working directory override those found further up.

### Merging and List Overwriting Principles

- **Field Merging**: Scalar options (such as strings, booleans, and integers) are merged field-by-field.
- **Collection Overwriting**: For list-type configurations (such as `mounts`, `env`, `ports`, `groupAdd`, `devices`, and `sensitiveEnv`), `cderun` uses an **overwrite (complete replacement)** approach. List elements are **never** appended or merged. If a higher-priority layer defines a list option, the corresponding list from lower-priority layers is completely discarded.
- **Explicit Empty List Overwrites**: If a higher-priority configuration explicitly defines an empty collection (e.g., `ports: []` or `env: []`), it completely clears and overrides any lists specified in lower-priority layers, rather than being ignored as unset.

---

## Configuration Schemas

Configuration fields use **camelCase** keys.

> **Exception**: Individual parameters within the `mounts` array (such as `read_only` or `optional`) use `snake_case` keys to maintain compatibility with standard container engines.

### `.cderun.yaml` Schema

#### Root Keys

- `runtime` (string): Target container engine (`docker` | `podman` | `containerd`).
- `socketPath` (string): Host socket absolute path.
- `defaults` (object): Default runner options (fields detailed below).
- `logging` (object): Logging output options:
  - `level`: `error` | `warn` | `info` | `debug` | `trace`
  - `format`: `text` | `json`
  - `timestamp`: bool

#### `defaults` Fields

- `tty`, `interactive`, `remove`, `strictEnv` (bool)
- `network`, `workdir`, `hostname`, `user`, `pull`, `pullBackoffBase`, `memory`, `hangTimeout` (string)
- `cpus` (float64)
- `pullMaxRetries` (int)
- `mountCderun`, `mountAllTools`, `mountSocket`, `privileged`, `publishAll` (bool)
- `mountCderunPath`, `mountSocketPath` (string)
- `mountTools`, `ports`, `expose`, `dns`, `addHosts`, `groupAdd`, `capAdd`, `capDrop`, `entrypoint`, `env`, `sensitiveEnv` ([]string)
- `dryRun`, `diagnosis` (bool)
- `dryRunFormat`, `diagnosisFormat` (string)
- `mounts` ([]MountConfig)
- `devices` (slice of objects or strings)

#### `.cderun.yaml` Example

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

### `.tools.yaml` Schema

Mapped by subcommand/tool name. Supports all fields defined under `defaults`, with the addition of:

- `image` (string, **Required**): Target container image.

It also supports overriding tool-specific logging settings:

- `logLevel`, `logFormat` (string)
- `logTimestamp` (bool)

#### `.tools.yaml` Example

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
    - "NODE_ENV=development"

python:
  image: "python:3.11-slim"
  workdir: /app
  mounts:
    - type: bind
      source: .
      target: /app
  env:
    - "PYTHONUNBUFFERED=1"
```

---

## Complex Option Structs

### Mount Configurations (`mounts`)

Each object under `mounts` supports the following keys:

- `type`: `bind` | `volume` | `tmpfs` (default: `bind`).
- `source`: Path on the host. Supports expressions (e.g., `{{HOME}}`).
- `target` (**Required**): Absolute path inside the container.
- `read_only` (bool): Mount the path as read-only.
- `optional` (bool): If true and the host-side `source` is missing, `cderun` skips the bind mount instead of failing.

### Device Configurations (`devices`)

Device configurations can be defined as structured objects or raw strings:

#### Structured Object

- `source` (**Required**): Host device path.
- `destination` (**Required**): Container device path.
- `permissions` (string): Device cgroup access permissions (e.g., `rwm`).

#### Raw String

Formatted as `<host-path>:<container-path>[:<permissions>]`. Permissions must conform strictly to `^[rwm]+$`.

```yaml
devices:
  - source: /dev/fuse
    destination: /dev/fuse
    permissions: rwm
  - "/dev/snd:/dev/snd:rw"
```
