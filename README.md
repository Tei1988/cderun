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

Flags prefixed with `--cderun-` are **"Internal Overrides" (P1)**. They have the highest priority (P1 > P2 CLI Flags > P3 Env Vars > P4 Tool Config > P5 Global Config > P6 Hardcoded Defaults).

In standard **Wrapper Mode**, these flags **must** be placed **after** the subcommand. `cderun` performs a "Hoisting" operation during preprocessing, moving these flags before the subcommand internally so they are parsed as `cderun` settings rather than being passed to the subcommand as passthrough arguments. Placing a P1 flag before the subcommand in Wrapper Mode will result in an error.

```bash
# Standard Wrapper Mode: P1 flags go AFTER the subcommand
cderun node app.js --cderun-image node:20-alpine

# WRONG (will result in an error):
cderun --cderun-image node:20-alpine node app.js
```

#### Hoisting Mechanics

Hoisting is a preprocessing step that scans the argument list to separate `cderun`'s internal overrides from the arguments intended for the wrapped tool.

1. **Detection**: `cderun` identifies the **subcommand** (the first non-flag argument that is not a value associated with a flag registered in `cderun`'s dynamic `FlagSet`).
2. **Extraction**: It gathers all flags prefixed with `--cderun-` (and their values) that appear *after* the subcommand.
3. **Hoisting**: These gathered flags are internally moved before the subcommand. This ensures they are correctly parsed as `cderun` overrides (P1) and prevents them from being passed to the containerized tool.

In **Symlink Mode (Polyglot Entry Point)**, only `--cderun-` prefixed flags are hoisted. This prevents collisions between `cderun`'s internal settings and the flags of the wrapped tool (e.g., `node --tty` passes `--tty` to `node`, while `node --cderun-tty` enables `cderun`'s TTY allocation).

**Note on Diagnosis Mode**: In `--diagnosis` mode, since no subcommand boundary exists, P1 flags can be placed anywhere.

### Available Flags

#### Execution & Identity

- `--tty`, `-t`: Allocate a pseudo-TTY. (Default: `false`)
- `--interactive`, `-i`: Keep STDIN open even if not attached. (Default: `false`)
- `--image`: Container image to use.
- `--entrypoint`: Overwrite the default ENTRYPOINT of the image.
- `--user`, `-u`: Username or UID (format: `<name|uid>[:<group|gid>]`).
- `--workdir`, `-w`: Working directory inside the container.
- `--env`, `-e`: Set environment variables (`KEY=VALUE` or `KEY` for host passthrough).
- `--strict-env`: Require all passed environment variables to be present on the host. (Default: `false`)
- `--pull`: Pull image before running (`always`, `missing`, `never`). (Default: `missing`)
- `--pull-max-retries`: Maximum number of retries for image pull. (Default: `3`)
- `--pull-backoff-base`: Base duration for exponential backoff during image pull (e.g. `1s`, `500ms`). (Default: `1s`)
- `--remove`: Automatically remove the container when it exits. (Default: `true`)
- `--hang-timeout`: Grace period after I/O completion before force-terminating the container (e.g. `10s`, `5s`, `0` for infinite). This applies to non-interactive or non-TTY sessions. (Default: `10s`)

#### Network & Ports

- `--network`: Connect a container to a network. (Default: `bridge`)
- `--hostname`: Container host name.
- `--publish`, `-p`: Publish a container's port(s) to the host.
- `--publish-all`, `-P`: Publish all exposed ports to random ports. (Default: `false`)
- `--expose`: Expose a port or a range of ports.
- `--dns`: Set custom DNS servers.
- `--add-host`: Add a custom host-to-IP mapping (`host:ip`).

#### Resources & Security

- `--memory`, `-m`: Memory limit (e.g., `512m`, `1g`).
- `--cpus`: Number of CPUs (float).
- `--device`: Add a host device to the container.
- `--privileged`: Give extended privileges to this container. (Default: `false`)
- `--cap-add`: Add Linux capabilities.
- `--cap-drop`: Drop Linux capabilities.

#### Mounting & Nested Execution

- `--mount`: Attach a filesystem mount (`type=bind,source=...,target=...[,readonly][,optional]`). The `optional` parameter skips bind mounts if the source path is missing on the host.
- `--mount-socket`: Mount the container runtime socket into the container. (Default: `false`)
- `--mount-cderun`: Mount the `cderun` binary into the container. (Enables `--mount-socket` automatically)
- `--mount-tools`: Mount specified tools (comma-separated) defined in `.tools.yaml` into the container.
- `--mount-all-tools`: Mount all tools defined in `.tools.yaml` into the container.
- `--socket-path`: Path to the runtime socket on the host.
- `--mount-socket-path`: Path where the socket should be mounted inside the container.
- `--mount-cderun-path`: Host path to the `cderun` binary to mount.

#### Diagnostics & Logging

- `--config`: Path to `cderun` config file (`.cderun.yaml`).
- `--tool-config`: Path to tools config file (`.tools.yaml`).
- `--runtime`: Container runtime to use (`docker`/`podman`).
- `--dry-run`: Preview container configuration without execution. (Requires a subcommand)
- `--dry-run-format`, `-f`: Output format for dry-run (`yaml`, `json`, `simple`).
- `--diagnosis`: Show system diagnostics and available tools. (No subcommand required)
- `--diagnosis-format`: Output format for diagnosis (`yaml`, `json`, `simple`).
- `--log-level`: Set log level (`error`, `warn`, `info`, `debug`, `trace`). (Default: `warn`)
- `--log-format`: Set log format (`text`, `json`).
- `--log-timestamp`: Include timestamp in logs. (Default: `true`)

*(All flags have a corresponding `--cderun-` prefixed P1 override counterpart.)*

## Environment Variables

`cderun` can be configured using environment variables. Almost all CLI flags have a corresponding `CDERUN_` prefixed environment variable (e.g., `CDERUN_IMAGE`, `CDERUN_TTY`, `CDERUN_REMOVE`).

Key variables include:

- `CDERUN_IMAGE`: Container image to use.
- `CDERUN_CONFIG`: Path to cderun config file.
- `CDERUN_TOOL_CONFIG`: Path to tools config file.
- `CDERUN_RUNTIME`: Container runtime to use (docker/podman).
- `CDERUN_PULL_MAX_RETRIES`: Maximum number of retries for image pull (default: `3`).
- `CDERUN_PULL_BACKOFF_BASE`: Base duration for exponential backoff during image pull (default: `1s`).
- `CDERUN_HANG_TIMEOUT`: Grace period for non-interactive or non-TTY sessions (default: `10s`).
- `CDERUN_STRICT_ENV`: If set to `true`, requires all environment variables to be present on the host.
- `CDERUN_DRY_RUN`: If set to `true`, enables dry-run mode.
- `CDERUN_DRY_RUN_FORMAT`: Output format for dry-run (yaml, json, simple).
- `CDERUN_DIAGNOSIS`: If set to `true`, enables diagnosis mode.
- `CDERUN_DIAGNOSIS_FORMAT`: Output format for diagnosis (yaml, json, simple).
- `CDERUN_PUBLISH_ALL`: If set to `true`, publish all exposed ports to random ports.
- `CDERUN_LOG_LEVEL`: Set log level (error, warn, info, debug, trace).
- `CDERUN_LOG_FORMAT`: Set log format (text, json).
- `CDERUN_LOG_TIMESTAMP`: Include timestamp in logs.

**Note on List-type Options:**

- **CLI Flags (P1/P2)**: List-type flags must be repeated for each item.
  - Example: `--env A=1 --env B=2`
- **Environment Variables (P3)**: Use specific separators depending on the variable.
  - Semicolon (`;`): `CDERUN_ENV`, `CDERUN_MOUNT`
  - Comma (`,`): `CDERUN_MOUNT_TOOLS`, `CDERUN_DEVICE`, `CDERUN_PUBLISH`, `CDERUN_EXPOSE`, `CDERUN_DNS`, `CDERUN_ADD_HOST`, `CDERUN_CAP_ADD`, `CDERUN_CAP_DROP`, `CDERUN_ENTRYPOINT`

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

### Nested Execution Support

- Transparently handles `cderun` execution inside a `cderun`-managed container.
- Automatically propagates host context and settings via snapshots.
- **Reverse Path Resolution**: Translates container-local paths back to host paths for nested mounts, ensuring the host-side Docker/Podman daemon can resolve volume sources.
- **OverlayFS Detection**: Automatically detects the root filesystem's `upperdir` to map container files back to the host.

### Sensitive Data Masking

`cderun` protects your secrets. Environment variables containing sensitive keywords (like `PASSWORD`, `SECRET`, `TOKEN`, `JWT`) are automatically masked (`[REDACTED]`) in:

- Dry-run output (`--dry-run`)
- Debug logs (`--log-level debug`)

The masking logic is intelligent and handles CamelCase (e.g., `dbPassword`) and different separators (e.g., `API_KEY`).

### Unified Value Resolution

- **Expressions**: Use `{{HOME}}`, `{{PWD}}`, `{{BASE_HOME}}`, `{{BASE_PWD}}`, `{{file:name}}` (limit 1MB), `{{find_dir:name}}`, `{{env:KEY}}`, and `{{env:KEY:-default}}`
  in configuration files and CLI flags.
- **Tilde Expansion**: `~` and `~/` paths are expanded to the user's home directory.
- **Relative Path Handling**: Intelligent absolute path resolution based on the
  origin of the setting (config file location vs. current directory).
- See [Value Resolution](docs/features/value-resolution.md) for details.

## Best Practices

### Consistent Development Environments

Use `{{file:.go-version}}` or `{{file:.nvmrc}}` in your tool configuration to ensure the container image tag matches your project's version file:

```yaml
# .tools.yaml
go:
  image: "golang:{{file:.go-version}}"
```

### Context-Aware Pathing

Use `{{find_dir:.git}}` to reference the project root regardless of your current working directory:

```bash
cderun --mount type=bind,source="{{find_dir:.git}}/logs",target=/logs my-tool
```

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
