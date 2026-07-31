# Feature Specification: Dry-Run Mode

## Overview

Dry-Run Mode generates and displays the intermediate container configuration (`ContainerConfig`) representing how the container will be spawned, without actually executing the container.

---

## Technical Specifications

### Basic Behavior

When the `--dry-run` flag is activated:

1. **Subcommand Required**: A subcommand lookup key must be specified; otherwise, execution aborts with a validation error.
2. **Evaluate Configuration**: The engine loads all configuration layers and constructs the standard intermediate configuration (`ContainerConfig`).
3. **Display Configuration**: The generated config is formatted and outputted to stdout.
4. **Clean Exit**: The process exits with code 0 without creating or starting any container.

---

## Usage

```bash
cderun --dry-run node --version
```

---

## Output Formats

`cderun` supports three output formats, controlled via `--dry-run-format` (or `-f`):

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
  "command": ["app.js"],
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
  "env": ["NODE_ENV=[REDACTED]"],
  "workdir": "/workspace",
  "user": "",
  "ports": ["8080:80"],
  "publish_all": false,
  "expose": ["80/tcp"],
  "hostname": "node-app",
  "dns": ["8.8.8.8"],
  "add_hosts": ["my-server:192.168.1.100"],
  "privileged": false,
  "cap_add": ["SYS_ADMIN"],
  "cap_drop": ["NET_RAW"],
  "entrypoint": ["/usr/bin/node"],
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
Command: app.js
TTY: true
Interactive: true
Network: bridge
Remove: true
Mounts: type=bind,source=/home/user/project,target=/workspace,readonly=false
Env: "NODE_ENV"="[REDACTED]"
Workdir: /workspace
User:
Ports:
PublishAll: false
Expose:
Hostname:
DNS:
AddHosts:
Privileged: false
CapAdd:
CapDrop:
Entrypoint:
Pull: missing
Memory: 512MiB
CPUs: 1.5
Devices:
```

*Note: In Simple Format, `Memory` is displayed in human-readable binary units (e.g., `512MiB` or `1GiB`), and `CPUs` is rendered as a floating point value (e.g., `1.5`).*

---

## Integration and Customizations

### Configuration Files Support

Dry-Run parameters (`dryRun`, `dryRunFormat`) can be configured inside `.cderun.yaml` or `.tools.yaml` (using camelCase keys), allowing you to force dry-runs on certain tools or customize default presentation formats.

### Secure Environment Masking

In Dry-Run outputs, all environment variable values are masked as `[REDACTED]` by default (**Secure by Default**).

To inspect environmental values during debugging, pass `--sensitive-env=""` to disable masking. For details, see [Sensitive Data Protection](./sensitive-data-protection.md).

### Absolute Path Resolution

Path-valued configuration fields—especially mount sources on the host—are fully resolved to absolute paths before being rendered in the Dry-Run output. Note that raw passthrough command arguments in `ContainerConfig.Command` (such as `./app.js`) are treated as literal command inputs and are not rewritten or converted to absolute paths.
