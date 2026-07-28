# Tool Mounting Specification

## Overview

`cderun` provides a dynamic capability to mount other tools defined in `.tools.yaml` into the current container execution. This enables polyglot execution environments where a container can seamlessly call other containerized commands without pre-installing them on the container image.

By mounting the `cderun` binary to specific tool names inside `/usr/local/bin/`, the inner execution utilizes the **Polyglot Entry Point (Symlink Mode)** logic to run nested commands automatically.

---

## Transitive Auto-enablement

Enabling tool mounting triggers a chain of required resource mounts. When `--mount-tools` or `--mount-all-tools` is set:

1. **`--mount-cderun`** is automatically enabled (mapping the `cderun` binary into the container).
2. **`--mount-socket`** is automatically enabled (mapping the host container runtime socket into the container) unless explicitly disabled via `--mount-socket=false`.

These rules are consistently resolved across all configuration priority layers as part of [Transitive Auto-enablement](./argument-priority-logic.md#transitive-auto-enablement) mechanics.

---

## Options & Usage

### 1. `--mount-all-tools`

- **Type**: bool
- **Default**: `false`
- **Environment Variable**: `CDERUN_MOUNT_ALL_TOOLS`
- **Description**: Mount all tools defined in `.tools.yaml` as individual symlinks/mounts pointing to the `cderun` binary inside the container.

#### Usage Example

```bash
cderun --mount-all-tools sh
```

#### Under-the-hood Mount Architecture

If `.tools.yaml` defines `node`, `python`, and `git`, `cderun` configures the following mounts:

```text
Host Path                         Container Target
┌───────────────────────────┐     ┌────────────────────────────────┐
│ /var/run/docker.sock      │ ──> │ /var/run/docker.sock           │
│ /usr/local/bin/cderun     │ ──> │ /usr/local/bin/cderun (ro)     │
│ /usr/local/bin/cderun     │ ──> │ /usr/local/bin/node (ro)       │
│ /usr/local/bin/cderun     │ ──> │ /usr/local/bin/python (ro)     │
│ /usr/local/bin/cderun     │ ──> │ /usr/local/bin/git (ro)        │
└───────────────────────────┘     └────────────────────────────────┘
```

#### Inside the Container

```bash
# Executing within the container:
node --version    # Triggers 'cderun node --version' inside the container
python script.py  # Triggers 'cderun python script.py'
```

---

### 2. `--mount-tools`

- **Type**: string
- **Environment Variable**: `CDERUN_MOUNT_TOOLS`
- **Description**: A comma-separated list of specific tools defined in `.tools.yaml` to mount into the container.

#### Usage Example

```bash
cderun --mount-tools node,python sh
```

#### Inside the Container

```bash
python --version  # Executes successfully (mapped to cderun)
node --version    # Executes successfully (mapped to cderun)
git status        # Error: bash: git: command not found (not mapped)
```

---

## Polyglot Entry Point Coordination

The nested execution leverages the standard symlink lookup logic:

```bash
# User executes 'node' inside container
/usr/local/bin/node --version

# Since the binary at '/usr/local/bin/node' is a copy/mount of 'cderun',
# it reads its own argument 0 (argv[0] == "node") and translates it to:
cderun node --version
```

---

## Whitelist Verification

If a requested tool is missing from the active `.tools.yaml` file, `cderun` immediately halts with a validation error to prevent misconfiguration:

```bash
cderun --mount-tools unknown-tool alpine sh
# Error: tool "unknown-tool" not found in .tools.yaml
# Available tools: node, python, git
```

---

## Limitations and Best Practices

- **Host Runtime Access**: Sharing the socket via `--mount-socket` is required. Ensure you review the security implications (especially in rootful environments).
- **Architecture Parity**: Ensure the mounted `cderun` binary is compiled for Linux and matches the container's CPU architecture (e.g., `linux/amd64` or `linux/arm64`). Use `--mount-cderun-path` to mount a compatible pre-compiled binary when running on macOS hosts.
- **Path Overwrites**: Mounting tools into `/usr/local/bin/` will overshadow any pre-existing binaries of the same name inside the container.
- **Read-Only Mounting**: All tool binary mappings are mounted as read-only (`ro`) to protect host binary integrity.
- **Nesting Package Managers**: Commands like `npm` or `npx` must be individually defined in `.tools.yaml` alongside `node` to allow mounting them:

```yaml
# .tools.yaml
node:
  image: node:20-alpine
npm:
  image: node:20-alpine
npx:
  image: node:20-alpine
```

```bash
cderun --mount-tools node,npm,npx sh
```
