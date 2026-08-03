# Feature Specification: Tool Mounting

## Overview

Tool Mounting is a feature that enables executing host-configured tools seamlessly within a container. By mounting the `cderun` binary inside the container under multiple tool-specific names, `cderun` leverages its Polyglot Entry Point functionality to run tools recursively without requiring local installations inside the container.

## Prerequisites

- A tools configuration file (`.tools.yaml`) must exist with the target tools defined.

Specifying `--mount-tools` or `--mount-all-tools` transitively and automatically enables both `--mount-cderun` and `--mount-socket`. This behavior follows the [Transitive Auto-enablement](./argument-priority-logic.md#transitive-auto-enablement) priority rules.

## Options

### `--mount-all-tools`

- **Type**: bool
- **Default**: `false`
- **Description**: Mount all tools configured inside `.tools.yaml` into the container.

**Example**:

```bash
cderun --mount-all-tools sh
```

**Underlying Behavior**:

If `.tools.yaml` defines `node`, `python`, and `gemini-cli`, the container runs with the following bind mounts:

```bash
docker run --rm \
  --mount type=bind,source=/var/run/docker.sock,target=/var/run/docker.sock \
  --mount type=bind,source=<host-cderun-path>,target=/usr/local/bin/cderun,readonly \
  --mount type=bind,source=<host-cderun-path>,target=/usr/local/bin/node,readonly \
  --mount type=bind,source=<host-cderun-path>,target=/usr/local/bin/python,readonly \
  --mount type=bind,source=<host-cderun-path>,target=/usr/local/bin/gemini-cli,readonly \
  alpine:latest
```

**Executing Inside the Container**:

```bash
# Inside the container shell:
node --version    # Executed via cderun as 'node'
python script.py  # Executed via cderun as 'python'
gemini-cli ask    # Executed via cderun as 'gemini-cli'
```

### `--mount-tools`

- **Type**: string
- **Default**: `""`
- **Description**: Mount only specified tools into the container (comma-separated list).

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
python --version  # OK
node --version    # OK
gemini-cli ask    # Error: gemini-cli is not mounted inside the container
```

## Implementation Details

### Target Directory

Tools are mounted as read-only executables inside the `/usr/local/bin/` directory:

```text
/usr/local/bin/
├── cderun       -> <host-cderun-path>
├── node         -> <host-cderun-path>
├── python       -> <host-cderun-path>
└── gemini-cli   -> <host-cderun-path>
```

### Polyglot Entry Point Invocation

Due to the Polyglot Entry Point architecture of `cderun`, invoking the tool binary name automatically sets the lookup key:

```bash
# Invoking 'node' inside the container:
node --version

# Translates internally to:
cderun node --version
```

### Tool Validation

If a tool specified in `--mount-tools` is missing from the active `.tools.yaml` configuration, the execution immediately fails:

```bash
cderun --mount-tools unknown-tool alpine sh
Error: tool "unknown-tool" not found in .tools.yaml
available tools: node, python, gemini-cli
```

## Practical Scenarios

### Uniform Development Environments

```bash
# Boot an Ubuntu workspace with all tools mounted
cderun --mount-all-tools \
  --image ubuntu:22.04 \
  bash

# Inside the Ubuntu container:
node --version
python --version
gemini-cli ask
```

### CI/CD Pipeline Isolation

```bash
# Selectively mount node and docker wrappers
cderun --mount-tools node,docker \
  sh -c '
    node --version
    docker build -t myapp .
    docker push myapp
  '
```

**Note**: Commands such as `npm` or `npx` must be explicitly defined in `.tools.yaml` to be mounted and executed:

```yaml
# .tools.yaml
node:
  image: node:20-alpine

npm:
  image: node:20-alpine

npx:
  image: node:20-alpine
```

This setup enables seamless command flows:

```bash
cderun --mount-tools node,npm,npx \
  sh -c '
    node --version
    npm install
    npx eslint .
  '
```

## Limitations

1. **Daemon Dependency**: Execution requires mounting the host's container engine socket inside the container (transitively managed via `--mount-socket`).
2. **Read-Only Mounts**: Mounted tool binaries are strictly read-only.
3. **Path Collisions**: Any tool with a colliding name already installed inside the container's base image will be overwritten in `/usr/local/bin/`.
4. **Binary Architecture**: The mounted `cderun` binary must match the target container's CPU architecture and operating system (as the host binary is mounted directly). On macOS hosts, a Linux-compatible `cderun` binary must be compiled and specified via `--mount-cderun-path`.

## Key Benefits

- **High Flexibility**: Mount only the subset of tools required for a given script or pipeline.
- **Zero Install Footprint**: Execute developer tools instantly without polluting container base images.
- **Unified Interface**: All nested tool execution passes through the native `cderun` pipeline for uniform settings and logging.
