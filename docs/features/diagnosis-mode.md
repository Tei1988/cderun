# Feature Specification: Diagnosis Mode

## Overview

Diagnosis Mode is a feature that retrieves and displays system diagnostic information (container engine runtime status, configuration files loading status) and the list of available tools.

## Requirements

### Basic Behavior

When the `--diagnosis` flag is specified:

1. System diagnostic information and the list of available tools are gathered.
2. The collected details are displayed in the configured format.
3. Container execution and dry-runs are skipped.
4. Upon successful diagnostic information collection and rendering, the execution exits normally with status code `0`. If diagnostic errors occur, they are propagated through the root command's `RunE`, producing a standard command execution error.

## Usage

### Basic Usage

```bash
cderun --diagnosis
```

### Usage with a Subcommand

Even if a subcommand is specified, Diagnosis Mode takes precedence:

```bash
cderun --diagnosis node --version
```

## Output Formats

### YAML Format (Default)

`cderun --diagnosis` or `cderun --diagnosis --diagnosis-format yaml`

```yaml
runtime:
  name: docker
  socket: /var/run/docker.sock
  status: accessible
configs:
  global:
    - /home/user/.cderun.yaml
  tools:
    - /home/user/project/.tools.yaml
available_tools:
  - git
  - node
  - python
```

*Note: The actual output fields may vary slightly based on implementation updates.*

### JSON Format

`cderun --diagnosis --diagnosis-format json` or `cderun <subcommand> --cderun-diagnosis-format json`

```json
{
  "runtime": {
    "name": "docker",
    "socket": "/var/run/docker.sock",
    "status": "accessible"
  },
  "configs": {
    "global": [
      "/home/user/.cderun.yaml"
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

### Simple Format

`cderun --diagnosis --diagnosis-format simple`

```text
Runtime: docker (/var/run/docker.sock)
Runtime Status: accessible
Global Config: /home/user/.cderun.yaml
Tools Config: /home/user/project/.tools.yaml
Available Tools: git, node, python
```

## P1 Internal Overrides

Like other standard flags, you can use the `--cderun-` prefix to specify a Priority 1 override for Diagnosis Mode. Since Diagnosis Mode does not require a subcommand, there are no placement restrictions for the flag (though in standard Wrapper Mode with a subcommand, it must be placed after the subcommand).

```bash
cderun --cderun-diagnosis
# or
cderun --diagnosis --cderun-diagnosis-format json
```

## Environment Variables

You can enable Diagnosis Mode without flags by setting the `CDERUN_DIAGNOSIS` environment variable to `true`.

```bash
export CDERUN_DIAGNOSIS=true
cderun
```

### Output Format Environment Variable

Use the `CDERUN_DIAGNOSIS_FORMAT` environment variable to control the output format.

```bash
export CDERUN_DIAGNOSIS_FORMAT=json
cderun --diagnosis
```

Combining both `CDERUN_DIAGNOSIS=true` and `CDERUN_DIAGNOSIS_FORMAT` allows flagless diagnosis outputs in a specific format.

```bash
export CDERUN_DIAGNOSIS=true
export CDERUN_DIAGNOSIS_FORMAT=json
cderun
```

Supported format values are `yaml` (default), `json`, and `simple`.

## Configuration Files

Diagnosis Mode configurations (`diagnosis`, `diagnosisFormat`) are also supported inside global and tool-specific configuration files (`.cderun.yaml`, `.tools.yaml`). This allows you to always enable Diagnosis Mode for specific tools or set a default output format.

Key names within YAML configuration files must use camelCase (e.g., `diagnosis`, `diagnosisFormat`).
