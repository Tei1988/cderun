# Environment Variable Passthrough Specification

## Overview

`cderun` enforces an isolated container model where **host environment variables are not inherited by default**. Environment variables must be explicitly specified to be passed into the container.

This behavior can be configured via `.tools.yaml` (P4), `.cderun.yaml` (P5), standard CLI flags `--env` / `-e` (P2), internal overrides `--cderun-env` (P1), or the `CDERUN_ENV` environment variable (P3). Both explicit key-value assignments (`KEY=value`) and host-passthrough definitions (`KEY`) are supported.

---

## Intermediate Representation

Inside `ContainerConfig.Env` (the intermediate configuration format), environment settings are stored as a string slice (`[]string`). Each element conforms to one of two syntaxes:

1. **`KEY=value` (Explicit Value)**: The variable will be configured with the specified `value` inside the container.
2. **`KEY` (Passthrough)**: The value of `KEY` will be resolved dynamically from the host environment at runtime.

---

## Configuration Methods

### 1. Tool Configuration (`.tools.yaml`)

```yaml
node:
  env:
    - NODE_ENV=production      # Explicit key-value assignment
    - NPM_TOKEN                # Dynamically retrieve from host env
    - HOME                     # Dynamically retrieve from host env
```

### 2. Command Line Options

```bash
# Explicit value
cderun --env NODE_ENV=production node app.js

# Host passthrough (repeated flags)
cderun --env NPM_TOKEN --env HOME node app.js

# Mixed definition
cderun --env NODE_ENV=production --env NPM_TOKEN node app.js
```

### 3. Environment Variable `CDERUN_ENV` (P3)

Use the environment variable `CDERUN_ENV` to pass multiple environment configurations. Values are split using **semicolons (`;`)** as the separator:

```bash
export CDERUN_ENV="NODE_ENV=production;NPM_TOKEN;HOME"
cderun node app.js
```

---

## Resolution Logic & Key Validation

Before executing the container:

1. **Splitting and Validating Keys**: Each entry in the `Env` slice is analyzed. The key portion (everything before the first `=` sign) is extracted and strictly validated using `ValidateEnvKey` to reject null bytes (`\x00`) and command injection attempts. If a key is invalid, execution is immediately aborted with a validation error.
2. **Value Fetching**: If an entry consists of only a key (no `=` sign), `cderun` queries the host environment using `os.Getenv(key)` (or `fs.LookupEnv`) to fetch the value and transforms it into the native `KEY=value` representation.
3. **Control Character Safe Values**: Only the keys are subject to strict character restrictions. The values can safely contain newlines or control characters (such as multiline PEM certificates) without failing validation.

---

## Default Behavior vs. Strict Mode (`strictEnv`)

### Default Mode (`strictEnv: false`)

If a requested passthrough environment variable is missing on the host, `cderun` configures it as empty inside the container:

```bash
cderun --env NONEXISTENT node -e "console.log(JSON.stringify(process.env.NONEXISTENT))"
# Configures: "NONEXISTENT="
# Output: ""
```

### Strict Mode (`strictEnv: true`)

When `strictEnv` is enabled, `cderun` immediately aborts execution with an error if a requested passthrough variable is missing or empty on the host.

#### Configuration Options

Enable strict mode via CLI, configuration files, or environment variables:

```yaml
# .cderun.yaml
defaults:
  strictEnv: true
```

```bash
# CLI Flag
cderun --strict-env node app.js

# Environment Variable
export CDERUN_STRICT_ENV=true
```

#### Behavior

```bash
cderun node app.js
# Error: required environment variable not found: NPM_TOKEN
```

---

## Precedence and Overriding

Higher priority sources (like CLI overrides) completely **replace** environment configurations from lower priority sources rather than merging them.

```yaml
# .tools.yaml
node:
  env:
    - NODE_ENV=development
    - PORT=3000
```

```bash
cderun --env NODE_ENV=production node app.js
# Only "NODE_ENV=production" is passed to the container.
# "PORT=3000" from .tools.yaml is ignored.
```

If the same key is defined multiple times within a single source, the **last** defined occurrence takes precedence:

```yaml
# .tools.yaml
node:
  env:
    - NODE_ENV=development
    - NODE_ENV=production  # This value wins
```

---

## Dry-Run Output & Sensitive Masking

To verify the resolved environment variables without launching a container, run with `--dry-run`:

```bash
cderun --dry-run node app.js
```

By default, all environment variable values are automatically masked as `[REDACTED]` in dry-run outputs and debug logs to ensure secret protection:

```yaml
env:
  - NODE_ENV=[REDACTED]
  - NPM_TOKEN=[REDACTED]
```

To view the plaintext values during debugging, disable masking explicitly using `--sensitive-env=""`.
