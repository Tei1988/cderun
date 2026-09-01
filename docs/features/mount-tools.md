# Feature Specification: Tool Mounting

## Overview

Tool Mounting is a feature that enables executing host-configured tools seamlessly within a container. By mounting the `cderun` binary inside the container under tool-specific names (e.g., `/usr/local/bin/node`, `/usr/local/bin/python`), `cderun` leverages its [Polyglot Entry Point](./polyglot-entry.md) functionality to run nested tools recursively on demand without requiring local software installations inside the container base image.

---

## Prerequisites

- A tools configuration file (`.tools.yaml`) must exist and define the target tools.
- Specifying `--mount-tools` or `--mount-all-tools` transitively and automatically enables both `--mount-cderun` and `--mount-socket`. This behavior follows the [Transitive Auto-enablement](./argument-priority-logic.md#transitive-auto-enablement) priority rules.

---

## Command-Line Options

### `--mount-all-tools`

- **Type**: bool
- **Default**: `false`
- **Environment Variable**: `CDERUN_MOUNT_ALL_TOOLS`
- **Description**: Mount all tools configured inside `.tools.yaml` into the container.

**Example**:

```bash
cderun --mount-all-tools sh
```

**Underlying Behavior**:

If `.tools.yaml` defines `node`, `python`, and `git`, the container is created with read-only bind mounts mapping the host `cderun` binary to each tool name under `/usr/local/bin/`:

```bash
docker run --rm \
  --mount type=bind,source=/var/run/docker.sock,target=/var/run/docker.sock \
  --mount type=bind,source=<host-cderun-path>,target=/usr/local/bin/cderun,readonly \
  --mount type=bind,source=<host-cderun-path>,target=/usr/local/bin/node,readonly \
  --mount type=bind,source=<host-cderun-path>,target=/usr/local/bin/python,readonly \
  --mount type=bind,source=<host-cderun-path>,target=/usr/local/bin/git,readonly \
  alpine:latest
```

**Executing Inside the Container**:

```bash
# Inside the container shell:
node --version    # Executed via cderun as 'node'
python script.py  # Executed via cderun as 'python'
git status        # Executed via cderun as 'git'
```

### `--mount-tools`

- **Type**: string (comma-separated list)
- **Default**: `""`
- **Environment Variable**: `CDERUN_MOUNT_TOOLS`
- **Description**: Mount only specified tools into the container. Accepts a comma-separated scalar string list (e.g., `--mount-tools node,python`).

**Example**:

```bash
cderun --mount-tools python,node sh
```

**Underlying Behavior**:

```bash
docker run --rm \
  --mount type=bind,source=/var/run/docker.sock,target=/var/run/docker.sock \
  --mount type=bind,source=<host-cderun-path>,target=/usr/local/bin/cderun,readonly \
  --mount type=bind,source=<host-cderun-path>,target=/usr/local/bin/python,readonly \
  --mount type=bind,source=<host-cderun-path>,target=/usr/local/bin/node,readonly \
  alpine:latest
```

**Executing Inside the Container**:

```bash
# Inside the container shell:
python --version  # OK (mounted)
node --version    # OK (mounted)
git status        # Uses container's native git binary when available; otherwise reports command not found / git not mounted
```

---

## Technical Implementation Details

### Target Directory Placement

Tool wrappers are mounted as read-only executable files inside the `/usr/local/bin/` directory inside the container:

```text
/usr/local/bin/
├── cderun       -> <host-cderun-path>
├── node         -> <host-cderun-path>
├── python       -> <host-cderun-path>
└── git          -> <host-cderun-path>
```

### Polyglot Entry Point Integration

Due to `cderun`'s [Polyglot Entry Point](./polyglot-entry.md) architecture, invoking a mounted tool wrapper binary by name automatically triggers `os.Args` rewriting and sets the tool name as the subcommand lookup key:

```bash
# Executing 'node --version' inside the container:
node --version

# Rewritten internally to:
cderun node --version
```

### Validation and Security Rules

Each tool name specified in `--mount-tools` or extracted from `--mount-all-tools` undergoes strict validation before mounting:

1. **Character Whitelisting (`ValidateToolName`)**: Tool names are strictly validated against allowed ASCII characters (`a-z`, `A-Z`, `0-9`, `.`, `_`, `-`). Any control characters, invalid UTF-8 sequences, or path traversal segments (`..`, `/`, `\`) trigger an immediate security validation error.
2. **Configuration Presence Check**: If a tool specified in `--mount-tools` is missing from the active `.tools.yaml` configuration, `cderun` aborts execution immediately during transitive option resolution with an explicit error:

```bash
cderun --mount-tools unknown-tool alpine sh
Error: tool "unknown-tool" not found in .tools.yaml
available tools: node, python, git
```

### Context Propagation & Snapshotting

When spawning a container with tool mounting enabled, `cderun` automatically constructs a temporary execution snapshot (`/tmp/cderun-snap-<uuid>/`) containing `.cderun.yaml` and `.tools.yaml` configuration snapshots, along with `hostContext` metadata. This snapshot directory is mounted at `/run/cderun/` inside the container to ensure nested `cderun` invocations preserve full configuration context and reverse path resolution capabilities.

---

## Practical Scenarios

### Uniform Development Workspaces

```bash
# Boot an Ubuntu workspace with all tools mounted
cderun --mount-all-tools \
  --image ubuntu:22.04 \
  bash

# Inside the Ubuntu container:
node --version
python --version
git --version
```

### CI/CD Pipeline Task Isolation

```bash
# Selectively mount node, npm, and npx wrappers
cderun --mount-tools node,npm,npx \
  sh -c '
    node --version
    npm install
    npx eslint .
  '
```

### Stand-alone Image Prefetching

Tool images defined in `.tools.yaml` can be pre-fetched ahead of execution using `--prefetch` or `--prefetch-all` without spawning container subcommands:

```bash
# Prefetch images for specific tools defined in .tools.yaml
cderun --prefetch node,python

# Prefetch images for all tools defined in .tools.yaml
cderun --prefetch-all
```

---

## Limitations & Best Practices

1. **Container Engine Dependency**: Execution requires mounting the host's container engine socket inside the container (transitively managed via `--mount-socket`).
2. **Read-Only Binaries**: Mounted tool binaries are strictly read-only (`readonly` bind mounts).
3. **Binary Shadowing**: Any executable with a colliding name already installed inside the container's base image will be shadowed (masked) in `/usr/local/bin/` by the mounted binary. The original binary inside the container image remains unmodified on disk.
4. **Platform & CPU Architecture Compatibility**: The mounted `cderun` binary must match the target container's CPU architecture and operating system (Linux). On macOS hosts, a Linux-compatible `cderun` binary must be compiled and specified via `--mount-cderun-path` (e.g. `--mount-cderun-path ./cderun_linux_arm64`).
