# Feature Specification: Dry-Run Mode

## Overview

Dry-Run Mode generates and displays the intermediate container configuration (`ContainerConfig`) representing how the container will be spawned, without actually executing the container or pulling images.

> **Note (Prefetch Exception)**: When image prefetch flags are active (`--prefetch` or `--prefetch-all`), prefetching takes precedence in the execution pipeline before subcommand handling and the dry-run branch. For example, invoking `cderun --dry-run --prefetch=node` will trigger `handlePrefetch`, which may pull the container image if the configured pull policy allows downloading.

---

## Technical Specifications

### Basic Behavior

<!-- coderabbit-ai-generated:v1 -->

<!-- coderabbit-ai-generated-section:v1:3206c766c8cf43fe103a81a6278b4fa57666292a926d01d5e79c4f88f659e7c4 -->

For ordinary dry-run execution:

1. **Subcommand Required**: A subcommand lookup key must be specified; otherwise, execution aborts with a validation error (`"--dry-run requires a subcommand"`). With `--dry-run --prune`, prune mode is dispatched before subcommand validation and prints a preview message without requiring a subcommand. Diagnosis is a separate mode, dispatched before subcommand validation, that takes precedence over dry-run execution.

   <!-- /coderabbit-ai-generated:v1 -->
2. **Evaluate Configuration**: The engine loads all configuration layers (P1 through P6) and constructs the standard intermediate configuration (`ContainerConfig`).
3. **Execution Pipeline Bypass**: The execution flow intercepts execution immediately after `ContainerConfig` construction. It completely bypasses:
   - Nested snapshot directory creation (`prepareSnapshotDir`).
   - Control Socket server startup (`controlsocket.NewServer`).
   - Image pulling and inspection (`rt.PullImage`) for standard tool execution.
   - Container creation and lifecycle tasks (`rt.CreateContainer`, `rt.StartContainer`, `rt.WaitContainer`).
4. **Display Configuration**: The generated config is formatted and outputted to stdout via the configured format.
5. **Clean Exit**: The process exits with code 0 without creating or starting any container.

### Execution Pipeline Lifecycle Comparison

| Stage / Component | Standard Execution (`cderun ...`) | Dry-Run Mode (`cderun --dry-run ...`) |
| :--- | :--- | :--- |
| **CLI Argument Parsing & Hoisting** | Executed | Executed |
| **Config Loading & Priority Resolution** | Executed | Executed |
| **Prefetch Dispatch (`--prefetch`)** | Executed (pulls images if requested) | Executed before dry-run branch (exception: pulls images if `--prefetch` specified) |
| **`ContainerConfig` Assembly** | Executed | Executed |
| **Sensitive Env Redaction** | Applied to debug logs only | Applied to presentation output (`ContainerConfig.Env`) |
| **Snapshot Directory Setup** | Created when `--mount-cderun` / nested active | Bypassed (no disk artifacts written) |
| **Control Socket Server** | Started when `--mount-cderun-socket` active | Bypassed (no socket listener bound) |
| **Image Pulling (`PullImage`)** | Executed according to `pull` policy | Bypassed (for tool command execution) |
| **Container Creation (`CreateContainer`)** | Executed | Bypassed |
| **Process Exit** | Concludes with container exit code | Exits cleanly with code 0 |

---

## Usage

```bash
cderun --dry-run node --version
cderun --dry-run --dry-run-format=json node app.js
cderun --dry-run -f simple node app.js
```

---

## Output Formats

`cderun` supports three output formats, controlled via `--dry-run-format` (or `-f`): `yaml` (default), `json`, and `simple`.

### 1. YAML Format (Default)

Command: `cderun --dry-run node app.js`

```yaml
image: node:latest
command:
  - app.js
tty: true
interactive: true
remove: true
network: bridge
mounts:
  - type: bind
    source: /home/user/project
    target: /workspace
env:
  - NODE_ENV=[REDACTED]
workdir: /workspace
user: ""
ports:
  - 8080:80
publish_all: false
expose:
  - "80/tcp"
hostname: node-app
dns:
  - 8.8.8.8
add_hosts:
  - "my-server:192.168.1.100"
privileged: false
cap_add:
  - SYS_ADMIN
cap_drop:
  - NET_RAW
entrypoint:
  - /usr/bin/node
pull: missing
memory: 536870912
cpus: 1.5
devices:
  - path_on_host: /dev/fuse
    path_in_container: /dev/fuse
    cgroup_permissions: rwm
```

### 2. JSON Format

Command: `cderun --dry-run --dry-run-format json node app.js`

```json
{
  "image": "node:latest",
  "command": [
    "app.js"
  ],
  "tty": true,
  "interactive": true,
  "remove": true,
  "network": "bridge",
  "mounts": [
    {
      "type": "bind",
      "source": "/home/user/project",
      "target": "/workspace"
    }
  ],
  "env": [
    "NODE_ENV=[REDACTED]"
  ],
  "workdir": "/workspace",
  "user": "",
  "ports": [
    "8080:80"
  ],
  "publish_all": false,
  "expose": [
    "80/tcp"
  ],
  "hostname": "node-app",
  "dns": [
    "8.8.8.8"
  ],
  "add_hosts": [
    "my-server:192.168.1.100"
  ],
  "privileged": false,
  "cap_add": [
    "SYS_ADMIN"
  ],
  "cap_drop": [
    "NET_RAW"
  ],
  "entrypoint": [
    "/usr/bin/node"
  ],
  "pull": "missing",
  "memory": 536870912,
  "cpus": 1.5,
  "devices": [
    {
      "path_on_host": "/dev/fuse",
      "path_in_container": "/dev/fuse",
      "cgroup_permissions": "rwm"
    }
  ]
}
```

### 3. Simple Format

Command: `cderun --dry-run --dry-run-format simple node app.js`

```text
Image: node:latest
Command: "app.js"
TTY: true
Interactive: true
Network: bridge
Remove: true
ReadOnly: false
Init: false
Mounts: type=bind,source="/home/user/project",target="/workspace",readonly=false
Env: "NODE_ENV"="[REDACTED]"
Workdir: /workspace
User:
Ports: 8080:80
PublishAll: false
Expose: 80/tcp
Hostname: node-app
DNS: 8.8.8.8
AddHosts: my-server:192.168.1.100
Privileged: false
CapAdd: SYS_ADMIN
CapDrop: NET_RAW
GroupAdd:
Devices: /dev/fuse
Memory: 512MiB
CPUs: 1.5
Entrypoint: "/usr/bin/node"
```

#### Field Rendering Rules in Simple Format

The simple text formatter (`formatDryRunSimple`) prints human-readable key-value lines according to exact field serialization specifications:

- **Command & Entrypoint**: Arguments in `Command` and `Entrypoint` slices are individually double-quoted via `strconv.Quote` to prevent terminal injection and ambiguous whitespace interpretation.
- **Environment Variables (`Env`)**: Formatted as `"KEY"="VALUE"` if an equals sign is present, or `"KEY"` for bare keys. Sensitive values are redacted as `"KEY"="[REDACTED]"` prior to quoting.
- **Mounts**: Formatted as comma-separated specifiers: `type=<type>,source="<source>",target="<target>",readonly=<bool>`.
- **Memory**: Serialized using binary human-readable units via `units.BytesSize` (e.g., `512MiB`, `1GiB`, `256KiB`). If `Memory <= 0`, the line is omitted.
- **CPUs**: Serialized as a compact floating-point string using `strconv.FormatFloat(cfg.CPUs, 'f', -1, 64)` (e.g., `1.5`, `0.5`). If `CPUs <= 0`, the line is omitted.
- **Devices**: If `PathOnHost == PathInContainer` and `CgroupPermissions == "rwm"`, rendered using the shorthand notation (`path_on_host`). Otherwise, formatted explicitly as `path_on_host:path_in_container:cgroup_permissions`.
- **Ulimits**: Serialized as comma-separated entries in `name=soft:hard` syntax (e.g., `nofile=1024:2048`).
- **Sysctls**: Serialized as sorted comma-separated key-value pairs (`key=value`) to ensure deterministic output.
- **Conditional Resource Flags**: Fields such as `Pid`, `ShmSize`, `IPC`, `GPUs`, `Cgroupns`, `PidsLimit`, `CPUShares`, `CpusetCpus`, `CpusetMems`, `Restart` (when not `"no"`), `OciRuntime`, `SecurityOpt`, `DNSSearch`, and `DNSOptions` are conditionally emitted only when explicitly configured or non-empty.

---

## Integration and Customizations

### Configuration Files Support

Dry-Run parameters (`dryRun`, `dryRunFormat`) can be configured inside `.cderun.yaml` or `.tools.yaml` (using camelCase keys), allowing you to force dry-runs on certain tools or customize default presentation formats:

```yaml
# .cderun.yaml
defaults:
  dryRun: false
  dryRunFormat: json
```

### Secure Environment Masking

In Dry-Run outputs across all formats (YAML, JSON, Simple), environment variable values are masked as `[REDACTED]` by default (**Secure by Default**):

```yaml
env:
  - API_KEY=[REDACTED]
  - DB_PASSWORD=[REDACTED]
```

- **Empty Values**: If an environment variable has an empty value (e.g., `KEY=`), it remains unmasked as `KEY=`.
- **Disabling Masking**: To inspect actual resolved values during debugging, pass `--sensitive-env=""` on the command line. Note that specifying an empty slice in a discovered configuration file (`defaults.sensitiveEnv: []`) may not disable masking because configuration layer merging via `mergo.WithOverride` ignores empty slices; for dependable unmasking across all configurations, use `--sensitive-env=""` via CLI.
- **Pattern-Based Masking**: Passing patterns (e.g., `--sensitive-env="*TOKEN*"`) selectively redacts matching variables while displaying non-matching variables in plaintext.
- For in-depth masking specifications and pattern glob matching rules, refer to [Sensitive Data Protection](./sensitive-data-protection.md).

### Absolute Path Resolution

Path-valued configuration fields—especially mount sources on the host—are fully resolved to absolute paths before being rendered in the Dry-Run output. Note that raw passthrough command arguments in `ContainerConfig.Command` (such as `./app.js`) are treated as literal command inputs and are not rewritten or converted to absolute paths.
