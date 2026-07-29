# Dry-Run Mode Specification

## Overview

`cderun` supports a **Dry-Run Mode** to preview the fully resolved intermediate configuration (`ContainerConfig`) before committing to the actual container lifecycle. This assists in debugging configurations, verifying resolver dynamic expressions, and validating security rules.

---

## Behavior and Rules

When `--dry-run` is requested:

1. **Subcommand Requirement**: A subcommand (e.g., `node`) must be supplied. It is used as the lookup key to search and merge configurations. Running without a subcommand will fail with an error (unless running `--diagnosis`).
2. **Configuration Resolving**: `cderun` executes all resolution layers (P1 down to P6), expanding expressions (like `{{HOME}}`), tildes (`~`), relative paths, and environment settings.
3. **Execution Skip**: The actual container creation, pull, and execution phases are completely bypassed.
4. **Configuration Display**: The resolved settings are printed to standard output in the requested format.
5. **Exit Status**: On successful render, `cderun` exits with status `0`.

---

## Formatting Options

The output style is controlled via the `dryRunFormat` (CLI: `--dry-run-format` / `-f`) property.

### 1. YAML Format (Default)

Output of `cderun --dry-run node app.js`:

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

---

### 2. JSON Format

Output of `cderun --dry-run --dry-run-format json node app.js`:

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

---

### 3. Simple Format

Output of `cderun --dry-run -f simple node app.js`:

```text
Image: node:latest
Command: "app.js"
TTY: true
Interactive: true
Network: bridge
Remove: true
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

> **Note on Units**: In the simple output format, `Memory` is displayed using human-readable binary units (such as `MiB` or `GiB`) for readability, and `CPUs` is displayed as a clean float representation (e.g., `1.5`). Additionally, all command arguments and environment definitions are individually quoted (`%q`) in this format to guard against terminal control-character injections.

---

## Practical Debugging Use Cases

### 1. Verify Configuration Resolutions

Confirm that `.tools.yaml` settings are being selected and applied correctly:

```bash
cderun --dry-run python script.py
```

### 2. Export Rendered Configurations

Output the fully resolved configuration to a file for backup or documentation:

```bash
cderun --dry-run -f yaml node app.js > resolved-node-config.yaml
```

### 3. Automate Configuration Auditing

Verify the resolved image dynamically in bash scripts before running critical tests:

```bash
#!/bin/bash
image_resolved=$(cderun --dry-run -f json node | jq -r '.image')
if [[ "$image_resolved" == "node:20-alpine" ]]; then
  echo "Image matches verified baseline."
else
  echo "Error: Unverified image target: $image_resolved"
  exit 1
fi
```

---

## Resolution Optimization & Security Masking

- **Secure by Default**: During dry-run generation, all environment values are automatically masked as `[REDACTED]` (Secure by Default) unless `--sensitive-env` configuration explicitly overrides or disables masking (see [Sensitive Data Protection](./sensitive-data-protection.md)).
- **Absolute Paths**: All relative paths configured via CLI or files are resolved to their fully qualified absolute paths (e.g., `./src` resolves to `/home/user/project/src`).
- **YAML Configuration Support**: Dry-run features can be enabled globally or per-tool inside configuration files (e.g., `dryRun: true`, `dryRunFormat: json` under `defaults`).
