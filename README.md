# cderun

## Concept

> "All you need on your local machine is Docker or Podman."
> `cderun` generates ephemeral containers for commands like `node`, `python`,
> or `git` on demand using container runtimes (Docker/Podman). It keeps your
> host clean and ensures reproducible environments defined in a single YAML file.

## Usage

`cderun` supports four primary modes of operation:

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

Create a symlink to `cderun` with the name of the tool you want to wrap.
`cderun` will automatically detect the tool name from the executable name.

```bash
ln -s cderun node
./node --version  # Effectively runs 'cderun node --version'
```

### 3. Ad-hoc Mode

You can use `cderun` to run arbitrary commands in a containerized environment
by specifying the image. Since the first non-flag argument is always consumed
as a subcommand (lookup key), you must explicitly specify the entrypoint
if you want to execute a command that is not the default entrypoint of the image.

```bash
cderun --image=alpine --entrypoint=ls ls -l
```

### 4. Diagnosis Mode

Run `cderun` with the `--diagnosis` flag to see system diagnostics
and available tools. This mode does not require a subcommand.

```bash
cderun --diagnosis
```

## Argument Parsing & Flags

`cderun` uses a strict boundary for argument parsing. The first non-flag argument is considered the **subcommand**. This subcommand acts as a lookup key for configuration.

- **[cderun-flags]**: Arguments before the subcommand are parsed as `cderun` flags.
- **\<subcommand\>**: The lookup key (e.g., `node`, `python`). It is NOT included in the final container command by default.
- **[passthrough-args]**: All arguments after the subcommand are passed directly to the container.

The final command executed in the container consists of the passthrough-args.

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

Flags prefixed with `--cderun-` are "Internal Overrides" (P1).
They have the highest priority and **must** be placed **after**
the subcommand in standard wrapper mode.

```bash
cderun node app.js --cderun-image node:20-alpine
```

### Available Flags

- `--tty`, `-t`: Allocate a pseudo-TTY.
- `--interactive`, `-i`: Keep STDIN open even if not attached.
- `--image`: Docker image to use (overrides mapping).
- `--env`, `-e`: Set environment variables (KEY=VALUE or KEY).
- `--strict-env`: Require all environment variables to be present on the host.
- `--mount`: Attach a filesystem mount to the container (type=bind,source=...,target=...[,readonly]).
- `--workdir`, `-w`: Working directory inside the container.
- `--network`: Connect a container to a network (default: "bridge").
- `--publish`, `-p`: Publish a container's port(s) to the host.
- `--publish-all`, `-P`: Publish all exposed ports to random ports.
- `--expose`: Expose a port or a range of ports.
- `--hostname`: Container host name.
- `--dns`: Set custom DNS servers.
- `--add-host`: Add a custom host-to-IP mapping (host:ip).
- `--user`, `-u`: Username or UID (format: <name|uid>[:<group|gid>]).
- `--privileged`: Give extended privileges to this container.
- `--cap-add`: Add Linux capabilities.
- `--cap-drop`: Drop Linux capabilities.
- `--entrypoint`: Overwrite the default ENTRYPOINT of the image.
- `--pull`: Pull image before running (always, missing, never). Default is `missing`.
- `--memory`, `-m`: Memory limit.
- `--cpus`: Number of CPUs.
- `--device`: Add a host device to the container.
- `--remove`: Automatically remove the container when it exits (default: true).
- `--hang-timeout`: Grace period after I/O completion before force-terminating the container (e.g. 2s, 500ms).
- `--config`: Path to cderun config file.
- `--tool-config`: Path to tools config file.
- `--runtime`: Container runtime to use (docker/podman).
- `--socket-path`: Specify the path to the container runtime socket (e.g., `/var/run/docker.sock`).
- `--mount-socket`: Mount the container runtime socket into the container.
- `--mount-socket-path`: Path where the socket should be mounted inside the container.
- `--mount-cderun`: Mount the cderun binary into the container. Automatically enables `--mount-socket`.
- `--mount-cderun-path`: Host path to cderun binary to mount inside container.
- `--mount-tools`: Mount specified tools (comma-separated) aliases into the container.
- `--mount-all-tools`: Mount all tools defined in `.tools.yaml` into the container.
- `--dry-run`: Preview container configuration without execution. Requires a subcommand.
- `--dry-run-format`, `-f`: Output format for dry-run (yaml, json, simple).
- `--diagnosis`: Show system diagnostics and available tools.
- `--diagnosis-format`: Output format for diagnosis (yaml, json, simple).
- `--log-level`: Set log level (error, warn, info, debug, trace). Default log level is `warn`. Note: `-v` or `--verbose` are not supported.
- `--log-format`: Set log format (text, json).
- `--log-timestamp`: Include timestamp in logs (default: true).

*(All flags have a corresponding `--cderun-` prefixed P1 override counterpart)*

## Environment Variables

`cderun` can be configured using environment variables. Almost all CLI flags have a corresponding `CDERUN_` prefixed environment variable (e.g., `CDERUN_IMAGE`, `CDERUN_TTY`, `CDERUN_REMOVE`).

Key variables include:

- `CDERUN_CONFIG`: Path to cderun config file.
- `CDERUN_TOOL_CONFIG`: Path to tools config file.
- `CDERUN_HANG_TIMEOUT`: Grace period for non-interactive or non-TTY sessions (default: `2s`). Forcefully terminates the container (SIGKILL) if it hangs past the grace period after I/O completion.
- `CDERUN_STRICT_ENV`: If set to `true`, requires all environment variables to be present on the host.
- `CDERUN_DRY_RUN`: If set to `true`, enables dry-run mode.
- `CDERUN_DRY_RUN_FORMAT`: Output format for dry-run (yaml, json, simple).
- `CDERUN_DIAGNOSIS`: If set to `true`, enables diagnosis mode.
- `CDERUN_DIAGNOSIS_FORMAT`: Output format for diagnosis (yaml, json, simple).
- `CDERUN_PUBLISH_ALL`: If set to `true`, publish all exposed ports to random ports.

Note: List-type variables like `CDERUN_ENV` and `CDERUN_MOUNT` use semicolon (`;`) as a separator, while others like `CDERUN_MOUNT_TOOLS`, `CDERUN_DEVICE`, `CDERUN_PUBLISH`, `CDERUN_EXPOSE`, `CDERUN_DNS`, `CDERUN_ADD_HOST`, `CDERUN_CAP_ADD`, `CDERUN_CAP_DROP`, and `CDERUN_ENTRYPOINT` use comma (`,`).

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
  level: warn
  format: text
```

### `.tools.yaml` (Tool Mappings)

Defines how specific tools should be containerized.

```yaml
node:
  image: node:20-alpine
  mounts:
    - type: bind
      source: .
      target: /app
  workdir: /app
python:
  image: python:3.11-slim
```

## Features

### Multi-Runtime Support & Auto-detection

`cderun` supports both **Docker** and **Podman**. It can automatically detect
the available runtime by checking for common Unix socket paths.

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
- Enables recursive container execution without installing tools
  in the container image.

### Unified Value Resolution

- **Expressions**: Use `{{HOME}}`, `{{PWD}}`, `{{file:name}}`, and `{{find_dir:name}}`
  in configuration files and CLI flags.
- **Tilde Expansion**: `~` and `~/` paths are expanded to the user's home directory.
- **Relative Path Handling**: Intelligent absolute path resolution based on the
  origin of the setting (config file location vs. current directory).
- See [Value Resolution](docs/features/value-resolution.md) for details.

### Nested Execution Support

- Transparently handles `cderun` execution inside a `cderun`-managed container.
- Automatically propagates host context and settings via snapshots.
- Implements "Reverse Path Resolution" to translate container-local paths
  back to host paths for nested mounts.
- Detects OverlayFS upperdir for automatic root filesystem mapping.

## Development & Testing

### Running Tests

To run the unit tests:

```bash
make test
# or
go test ./...
```

To run the End-to-End (E2E) tests which require a running Docker or Podman environment:

```bash
go test -tags=runtime ./...
```

### Generating Coverage Report

To generate a test coverage report:

```bash
make coverage
```

---
*This project is under active development.*
