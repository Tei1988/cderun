# cderun

**Concept**

> "All you need on your local machine is Docker."
> `cderun` generates ephemeral containers for commands like `node`, `python`, or `git` on demand. It keeps your host clean and ensures reproducible environments.

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
You can use `cderun` to run arbitrary commands in a containerized environment by specifying the subcommand and its arguments.
```bash
cderun bash
```

## Argument Parsing & Flags

`cderun` uses a strict boundary for argument parsing. The first non-flag argument is considered the **subcommand**. All arguments before it are parsed as `cderun` flags, and all arguments after it (including flags) are passed directly to the subcommand.

### Illustration
```bash
$ cderun --tty docker --tty
  |      |     |      |
  |      |     |      +-- Passthrough argument (passed to docker)
  |      |     +--------- Subcommand
  |      +--------------- cderun flag (TTY: true)
  +---------------------- cderun command
```

### Available Flags
- `--tty`: Allocate a pseudo-TTY.
- `--interactive`, `-i`: Keep STDIN open even if not attached.
- `--image`: Docker image to use (overrides mapping).
- `--network`: Connect a container to a network (default: "bridge").
- `--remove`: Automatically remove the container when it exits (default: true).
- `--runtime`: Container runtime to use (docker/podman).
- `--mount-socket`: Specify the path to the container runtime socket (e.g., `/var/run/docker.sock`).
- `--mount-cderun`: (Planned) Mount the cderun binary into the container. Currently requires `--mount-socket`.
- `--cderun-tty`: Override TTY setting (highest priority, can be used after subcommand).
- `--cderun-interactive`: Override interactive setting (highest priority, can be used after subcommand).
- `--dry-run`: Preview container configuration without execution.
- `--format`, `-f`: Output format (yaml, json, simple).

## Features

### Multi-Runtime Support
`cderun` uses an abstraction layer to support multiple container runtimes:
- **Docker** (default)
- **Podman** (Planned - Phase 4)
- Extensible architecture for future runtimes (containerd, Lima, etc.)

### Advanced Tool Configuration
`cderun` supports tool-specific settings in `.tools.yaml`, allowing you to pre-configure:
- **Volumes**: Map host directories to container paths.
- **Environment Variables**: Define static environment variables for the tool.
- **Working Directory**: Set the default working directory inside the container.

### Intelligent Argument Parsing
- Strict boundary parsing separates `cderun` flags from subcommand arguments
- Prevents flag conflicts between `cderun` and wrapped commands
- Supports complex command structures

### Polyglot Entry Point
- Single binary can act as multiple tools via symlinks
- Automatic tool detection from executable name
- Seamless integration with existing workflows

### Clean Host Environment
- All commands run in ephemeral containers
- No need to install development tools locally
- Consistent, reproducible environments

---
*This project is under active development.*
