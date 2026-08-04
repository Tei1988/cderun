# Feature Specification: Environment Variable Passthrough

## Overview

Environment Variable Passthrough selectively forwards host environment variables into execution containers. **By default, no environment variables are passed through.** Only variables explicitly configured are transmitted to the container.

Passthrough is supported via tool-specific configurations inside `.tools.yaml` (P4 priority), command-line flags `--env` / `-e` (P2), internal override flags `--cderun-env` (P1), and the `CDERUN_ENV` environment variable (P3). Both `KEY=value` (explicit setting) and `KEY` (passthrough from host) formats are supported.

## Intermediate Representation

`ContainerConfig.Env` is a string slice (`[]string`) containing elements in one of two formats:

### env Element Formats

1. **`KEY=value`** (explicit assignment): The value is passed as-is.
2. **`KEY`** (passthrough): The value is dynamically retrieved from the host at execution time and converted into `KEY=value` format.

## Configuration Methods

### Tool Profile Configuration

```yaml
# .tools.yaml
node:
  env:
    - NODE_ENV=production      # Explicit assignment
    - NPM_TOKEN                 # Passthrough from host
    - HOME                      # Passthrough from host
```

### Command Line Flags

```bash
# Set an explicit assignment
cderun --env NODE_ENV=production node app.js

# Pass through from the host environment
cderun --env NPM_TOKEN --env HOME node app.js

# Combined usage
cderun --env NODE_ENV=production --env NPM_TOKEN node app.js
```

### Environment Variable (P3)

Use the `CDERUN_ENV` environment variable to define multiple variables simultaneously, using a semicolon (`;`) as a separator:

```bash
export CDERUN_ENV="NODE_ENV=production;NPM_TOKEN;HOME"
cderun node app.js
```

## Priority Rules

Under the priority hierarchy, if a higher-priority configuration layer contains any definition for the `env` setting, all lower-priority layers are ignored completely (completely overwritten).

```yaml
# .tools.yaml
node:
  env:
    - NODE_ENV=development
    - PORT=3000
```

```bash
cderun --env NODE_ENV=production node app.js
# -> Only NODE_ENV=production is passed to the container.
#    The PORT=3000 mapping from .tools.yaml is completely ignored.
```

### Key Redefinitions in the Same Source

If the same key is defined multiple times inside the same source (such as duplicate CLI flags or duplicate entries in a single YAML block), **the last specified value takes precedence**.

```yaml
# .tools.yaml
node:
  env:
    - NODE_ENV=development
    - NODE_ENV=production  # This value is adopted
```

## Resolution Examples

### Example 1: Explicit Settings

```yaml
# .tools.yaml
node:
  env:
    - NODE_ENV=production
    - PORT=3000
```

```bash
cderun node app.js
# ContainerConfig.Env = ["NODE_ENV=production", "PORT=3000"]
```

### Example 2: Passthrough from Host

```yaml
# .tools.yaml
node:
  env:
    - NPM_TOKEN  # Passthrough
    - HOME       # Passthrough
```

```bash
export NPM_TOKEN=secret123
export HOME=/home/alice
cderun node app.js
# Resolved at runtime:
# ContainerConfig.Env = ["NPM_TOKEN=secret123", "HOME=/home/alice"]
```

### Example 3: Mixed Usage

```yaml
# .tools.yaml
node:
  env:
    - NODE_ENV=production  # Explicit
    - NPM_TOKEN            # Passthrough
    - PORT=3000            # Explicit
```

```bash
export NPM_TOKEN=secret123
cderun node app.js
# ContainerConfig.Env = [
#   "NODE_ENV=production",
#   "NPM_TOKEN=secret123",
#   "PORT=3000"
# ]
```

## Unset Environment Variables

### Default Behavior

If a requested passthrough variable is not defined on the host, `cderun` defaults to passing it as an empty string:

```bash
cderun --env NONEXISTENT node -e "console.log(process.env.NONEXISTENT)"
# ContainerConfig.Env = ["NONEXISTENT="]
# Output: "" (empty string)
```

### Strict Mode (`strictEnv`)

Setting `strictEnv` to `true` causes execution to immediately fail with a configuration error if any requested passthrough environment variables are missing on the host.

#### Configuration Methods

It can be configured inside `.cderun.yaml` (global defaults), `.tools.yaml` (tool profile), or via the command-line flag `--strict-env`.

```yaml
# .cderun.yaml
defaults:
  strictEnv: true
```

Or via command-line flags:

```bash
cderun --strict-env node app.js
```

Or via environment variables:

```bash
export CDERUN_STRICT_ENV=true
```

#### Behavior

```bash
cderun node app.js
Error: required environment variable not found: NPM_TOKEN
```

## Environment Resolution Logic

Prior to container creation, the execution engine scans the resolved `Env` slice. For any entries that do not contain an equals sign (`=`), the engine queries `os.Getenv(key)`. The resulting resolved `KEY=value` string is passed directly to the container runtime API.

## Best Practices

Guidance on choosing between default passthrough behavior and Strict Mode:

### Default Passthrough (`strictEnv: false`)

- **Characteristics**: Missing host variables do not trigger errors; they are passed as empty values.
- **Benefits**: Offers flexibility, allowing container tasks to run even if minor configuration details are missing.
- **Recommended Use Cases**:
  - Local ad-hoc developer tasks.
  - Optional logging levels or debugging switches.

### Strict Mode (`strictEnv: true`)

- **Characteristics**: Missing host variables halt execution instantly (fail-fast behavior).
- **Benefits**: Eliminates silent failures resulting from unset variables or configuration typos.
- **Recommended Use Cases**:
  - **Secrets and Credentials**: Critical tokens (such as `NPM_TOKEN`, `AWS_ACCESS_KEY_ID`) where omission leads to immediate execution failures.
  - **CI/CD Environments**: Ensuring strict environment consistency across automated test runners.
  - **Shared Team Profiles**: Ensuring all teammates have properly set up local environment keys as specified in `.tools.yaml`.

## Debugging and Verification

### Verifying Settings via Dry Run

```bash
cderun --dry-run node app.js
env:
  - NODE_ENV=[REDACTED]
  - NPM_TOKEN=[REDACTED]
  - HOME=[REDACTED]
```

By default, **all** environment variable values are automatically masked as `[REDACTED]` in dry-run configurations (Secure by Default). To inspect actual resolved values, disable masking using `--sensitive-env=""` (see [Sensitive Data Protection](./sensitive-data-protection.md)).
