# Feature Specification: Diagnosis Mode

## Overview

Diagnosis Mode is an inspection and verification feature in `cderun` that inspects the host and container engine environment, verifies socket connectivity, enumerates discovered global and tool configuration files, and lists all available tool profiles without launching a container or executing a command.

Unlike standard container execution or Dry-Run Mode (`--dry-run`), Diagnosis Mode does not require a subcommand or container image to be specified. It serves as an immediate health-check and setup validation tool for operators, automated CI environments, and nested container runners.

## Requirements and Execution Lifecycle

### Pipeline Precedence and Execution Order

Within the root command's `PreRunE` / `RunE` execution flow, Diagnosis Mode executes early in the command evaluation lifecycle:

1. **Argument Preprocessing & Hoisting**: Evaluates CLI flags, hoisted P1 internal overrides (`--cderun-diagnosis`, `--cderun-diagnosis-format`), and environment variables (`CDERUN_DIAGNOSIS`, `CDERUN_DIAGNOSIS_FORMAT`).
2. **Configuration Loading & Resolution**: Searches and loads global configuration files (`.cderun.yaml`) and tools configuration files (`.tools.yaml`), resolving winning options across the P1–P6 hierarchy.
3. **Pull Policy Validation**: Validates image pull policy settings (`always`, `missing`, `never`).
4. **Diagnosis Execution (`handleDiagnosis`)**:
   - If `resolved.Diagnosis` is `true`, `handleDiagnosis` executes immediately.
   - **Bypasses downstream operations**: Prune operations (`--prune`), image prefetching (`--prefetch`, `--prefetch-all`), subcommand existence validation (`subcommand == ""`), Dry-Run Mode (`--dry-run`), and container creation/execution are completely bypassed.
   - **Subcommand Independence**: Running `cderun --diagnosis` without any subcommand terminates successfully. If subcommands or positional arguments are supplied (e.g., `cderun --diagnosis node --version`), Diagnosis Mode still takes precedence and skips container execution.
5. **Exit Status**:
   - Upon successful gathering and rendering of diagnostic information, `cderun` exits normally with status code `0`.
   - If output formatting fails (e.g., JSON/YAML marshaling error or standard output write failure), an error is returned through `RunE`, resulting in a non-zero exit code (`1`).

### Diagnostic Data Model (`diagnosticsInfo`)

`handleDiagnosis` populates a structured data model with the following fields:

| Field Path | Type | JSON / YAML Key | Description |
| :--- | :--- | :--- | :--- |
| `Runtime.Name` | `string` | `name` | Name of the resolved container runtime or engine (e.g., `docker`, `podman`, `containerd`, `nerdctl`). |
| `Runtime.Socket` | `string` | `socket` | Path to the container runtime daemon socket on the execution host (e.g., `/var/run/docker.sock`, `/run/containerd/containerd.sock`). |
| `Runtime.Status` | `string` | `status` | Accessibility status of the socket. Evaluated via `os.Stat` (or `o.fs.Stat`). If accessible, returns `"accessible"`. If stat fails, returns `"not found or inaccessible: <error>"`. |
| `Runtime.ControlSocket` | `string` | `control_socket` (omitted if empty) | Path to the active `cderun` Control Socket. Populated when running inside a nested container where the host context provided a Control Socket (`globalCfg.HostContext.ControlSocket`). |
| `Configs.Global` | `[]string` | `global` | Ordered list of absolute paths to global configuration files discovered and loaded (e.g., `~/.config/cderun/.cderun.yaml`, `/etc/cderun/.cderun.yaml`, or working directory `.cderun.yaml`). Empty list if none found. |
| `Configs.Tools` | `[]string` | `tools` | Ordered list of absolute paths to tool definition configuration files discovered and loaded (e.g., `.tools.yaml`). Empty list if none found. |
| `AvailableTools` | `[]string` | `available_tools` (omitted if empty in JSON/YAML) | Alphabetically sorted list of all tool profile names defined in the loaded `ToolsConfig` map. |

### Socket Accessibility Resilience

When verifying `Runtime.Socket`, `cderun` performs a filesystem `Stat` check:

- If `Stat` succeeds, `Runtime.Status` is marked as `"accessible"`.
- If `Stat` fails (e.g., file not found, permission denied, or daemon not running), `handleDiagnosis` does **not** abort or return an error. Instead, it formats the error message directly into the `Runtime.Status` field (e.g., `not found or inaccessible: stat /var/run/docker.sock: no such file or directory`).
- This non-fatal behavior ensures operators can run `--diagnosis` to inspect why container commands fail without the diagnostic command itself failing.

## Usage

### Standalone Health Check

Run without arguments to verify runtime connectivity and discovered tools:

```bash
cderun --diagnosis
```

### With Subcommand Arguments

Diagnosis Mode takes precedence regardless of supplied positional arguments:

```bash
cderun --diagnosis node --version
```

### Custom Output Format Selection

Select among `yaml` (default), `json`, or `simple`:

```bash
cderun --diagnosis --diagnosis-format json
```

## Output Formats

### 1. YAML Format (Default)

`cderun --diagnosis` or `cderun --diagnosis --diagnosis-format yaml`

#### Standard Host Execution Example

```yaml
runtime:
  name: docker
  socket: /var/run/docker.sock
  status: accessible
configs:
  global:
    - /home/user/.config/cderun/.cderun.yaml
  tools:
    - /home/user/project/.tools.yaml
available_tools:
  - git
  - node
  - python
```

#### Nested Execution with Control Socket Example

When running inside an outer container with nested execution enabled via Control Socket:

```yaml
runtime:
  name: docker
  socket: /var/run/docker.sock
  status: accessible
  control_socket: /run/cderun/cderun.sock
configs:
  global:
    - /etc/cderun/.cderun.yaml
  tools:
    - /workspace/.tools.yaml
available_tools:
  - build-tool
  - linter
```

#### Inaccessible Socket Example

```yaml
runtime:
  name: containerd
  socket: /run/containerd/containerd.sock
  status: "not found or inaccessible: stat /run/containerd/containerd.sock: no such file or directory"
configs:
  global: []
  tools: []
available_tools: []
```

### 2. JSON Format

`cderun --diagnosis --diagnosis-format json`

JSON outputs are indented with 2 spaces for human and programmatic readability. Empty `control_socket` and empty `available_tools` fields are omitted via `omitempty`.

```json
{
  "runtime": {
    "name": "docker",
    "socket": "/var/run/docker.sock",
    "status": "accessible"
  },
  "configs": {
    "global": [
      "/home/user/.config/cderun/.cderun.yaml"
    ],
    "tools": [
      "/home/user/project/.tools.yaml"
    ]
  },
  "available_tools": [
    "git",
    "node",
    "python"
  ]
}
```

Nested execution with Control Socket active:

```json
{
  "runtime": {
    "name": "docker",
    "socket": "/var/run/docker.sock",
    "status": "accessible",
    "control_socket": "/run/cderun/cderun.sock"
  },
  "configs": {
    "global": [
      "/etc/cderun/.cderun.yaml"
    ],
    "tools": [
      "/workspace/.tools.yaml"
    ]
  },
  "available_tools": [
    "build-tool",
    "linter"
  ]
}
```

### 3. Simple Plain-Text Format

`cderun --diagnosis --diagnosis-format simple`

The Simple format emits concise, human-readable key-value lines without YAML or JSON syntax overhead:

```text
Runtime: docker (/var/run/docker.sock)
Runtime Status: accessible
Global Config: /home/user/.config/cderun/.cderun.yaml
Tools Config: /home/user/project/.tools.yaml
Available Tools: git, node, python
```

When multiple config files or tools are present, they are comma-separated:

```text
Runtime: containerd (/run/containerd/containerd.sock)
Runtime Status: accessible
Global Config: /etc/cderun/.cderun.yaml, /home/user/.config/cderun/.cderun.yaml
Tools Config: /workspace/.tools.yaml
Available Tools: cargo, go, rustc
```

When no configurations or tools are loaded, empty strings are printed:

```text
Runtime: docker (/var/run/docker.sock)
Runtime Status: accessible
Global Config:
Tools Config:
Available Tools:
```

## Configuration and Overrides Hierarchy

Diagnosis Mode integrates fully into `cderun`'s P1–P6 precedence hierarchy:

| Priority Level | Configuration Source | Example Syntax |
| :--- | :--- | :--- |
| **P1** | Hoisted Internal Override | `cderun <subcommand> --cderun-diagnosis --cderun-diagnosis-format json` |
| **P2** | CLI Command Flags | `cderun --diagnosis --diagnosis-format yaml` |
| **P3** | Environment Variables | `CDERUN_DIAGNOSIS=true`, `CDERUN_DIAGNOSIS_FORMAT=json` |
| **P4** | Tool Configuration (`.tools.yaml`) | `tools:\n  node:\n    diagnosis: true\n    diagnosisFormat: simple` |
| **P5** | Global Configuration (`.cderun.yaml`) | `defaults:\n  diagnosis: false\n  diagnosisFormat: yaml` |
| **P6** | Built-in Defaults | `diagnosis: false`, `diagnosisFormat: yaml` |

### P1 Internal Overrides

When wrapping tools in Wrapper Mode, Phase 1 internal overrides can be appended after the subcommand:

```bash
cderun node --version --cderun-diagnosis --cderun-diagnosis-format simple
```

In standalone mode (without a subcommand), `--cderun-diagnosis` can also be passed directly:

```bash
cderun --cderun-diagnosis
```

### Environment Variables

Diagnosis Mode can be enabled without modifying CLI invocations using environment variables:

- `CDERUN_DIAGNOSIS`: Set to `true`, `1`, or boolean truthy values to enable Diagnosis Mode.
- `CDERUN_DIAGNOSIS_FORMAT`: Set to `yaml`, `json`, or `simple`.

Example for CI debugging:

```bash
export CDERUN_DIAGNOSIS=true
export CDERUN_DIAGNOSIS_FORMAT=json
cderun
```

### Configuration Files (`.cderun.yaml`, `.tools.yaml`)

Both global and tool configuration files support Diagnosis Mode keys using camelCase:

```yaml
# .cderun.yaml
defaults:
  diagnosis: false
  diagnosisFormat: yaml
```

Or for a specific tool:

```yaml
# .tools.yaml
tools:
  debug-runner:
    image: alpine:latest
    diagnosis: true
    diagnosisFormat: json
```
