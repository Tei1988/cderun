# cderun

**Concept**

> "All you need on your local machine is Docker or Podman."
> `cderun` generates ephemeral containers for commands like `node`, `python`, or `git` on demand using container runtimes (Docker/Podman). It keeps your host clean and ensures reproducible environments defined in a single YAML file.

## Usage

`cderun` supports three primary modes of operation:

### 1. Wrapper Mode
Explicitly call `cderun` followed by the subcommand you want to run.
```bash
cderun [cderun-flags] <subcommand> [passthrough-args]
```
Example:
```bash
cderun --tty node --version
```

### 2. Symlink Mode (Polyglot Entry Point)
Create a symlink to `cderun` with the name of the tool you want to wrap. `cderun` will automatically detect the tool name from the executable name.
```bash
ln -s cderun node
./node --version  # Effectively runs 'cderun node --version'
```

### 3. Ad-hoc Mode
You can use `cderun` to run arbitrary commands in a containerized environment by specifying the image.
```bash
cderun --image alpine ls -l
```

### 4. Global Dry Run (Diagnostics)
Run `cderun` with the `--dry-run` flag but without a subcommand to see system diagnostics and available tools.
```bash
cderun --dry-run
```

## Argument Parsing & Flags

`cderun` uses a strict boundary for argument parsing. The first non-flag argument is considered the **subcommand**. All arguments before it are parsed as `cderun` flags, and all arguments after it (including flags) are passed directly to the subcommand.

### Illustration
```text
cderun --tty docker --tty
  |      |     |      |
  |      |     |      +-- Passthrough argument (passed to docker)
  |      |     +--------- Subcommand
  |      +--------------- cderun flag (TTY: true)
  +---------------------- cderun command
```

### P1 Internal Overrides
Flags prefixed with `--cderun-` are "Internal Overrides" (P1). They have the highest priority and can be placed **after** the subcommand if needed.

```bash
cderun node app.js --cderun-image node:20-alpine
```

### Available Flags
- `--tty`: Allocate a pseudo-TTY.
- `--interactive`, `-i`: Keep STDIN open even if not attached.
- `--image`: Docker image to use (overrides mapping).
- `--env`, `-e`: Set environment variables (KEY=VALUE or KEY).
- `--volume`, `-v`: Bind mount a volume (hostPath:containerPath[:ro|rw]).
- `--workdir`, `-w`: Working directory inside the container.
- `--network`: Connect a container to a network (default: "bridge").
- `--publish`, `-p`: Publish a container's port(s) to the host.
- `--user`, `-u`: Username or UID to use.
- `--privileged`: Give extended privileges to this container.
- `--pull`: Pull image before running (always, missing, never).
- `--memory`, `-m`: Memory limit.
- `--remove`: Automatically remove the container when it exits (default: true).
- `--runtime`: Container runtime to use (docker/podman).
- `--socket-path`: Specify the path to the container runtime socket (e.g., `/var/run/docker.sock`).
- `--mount-socket`: Mount the container runtime socket into the container.
- `--mount-socket-path`: Path where the socket should be mounted inside the container.
- `--mount-cderun`: Mount the cderun binary into the container. Requires `--mount-socket`.
- `--mount-tools`: Mount specified tools (comma-separated) aliases into the container.
- `--mount-all-tools`: Mount all tools defined in `.tools.yaml` into the container.
- `--dry-run`: Preview container configuration or show system diagnostics.
- `--dry-run-format`, `-f`: Output format (yaml, json, simple).
- `--verbose`: Enable verbose logging (repeat for more detail).
- `--log-level`: Set log level (error, warn, info, debug, trace).
- `--log-file`: Set log file path.
- `--log-format`: Set log format (text, json).
- `--log-tee`: Output log to both stderr and log file.
- `--log-timestamp`: Include timestamp in logs (default: true).

*(All flags have a corresponding `--cderun-` prefixed P1 override counterpart)*

## Configuration

`cderun` uses two configuration files to manage its behavior.

### `.cderun.yaml` (Global Settings)
Used for general settings and defaults.
```yaml
runtime: docker
defaults:
  tty: true
  interactive: true
  remove: true
logging:
  level: info
  format: text
```

### `.tools.yaml` (Tool Mappings)
Defines how specific tools should be containerized.
```yaml
node:
  image: node:20-alpine
  volumes:
    - ".:/app"
  workdir: /app
python:
  image: python:3.11-slim
```

## Features

### Multi-Runtime Support & Auto-detection
`cderun` supports both **Docker** and **Podman**. It can automatically detect the available runtime by checking for common Unix socket paths.

### Intelligent Argument Parsing
- Strict boundary parsing separates `cderun` flags from subcommand arguments.
- Prevents flag conflicts between `cderun` and wrapped commands.
- Supports complex command structures with P1 internal overrides.

### Polyglot Entry Point
- Single binary can act as multiple tools via symlinks.
- Automatic tool detection from executable name.
- Seamless integration with existing workflows.

### Advanced Tool Mounting
- Mount the `cderun` binary and other defined tools into the container.
- Enables recursive container execution without installing tools in the container image.

---
*This project is under active development.*
