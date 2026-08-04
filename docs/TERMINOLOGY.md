# Terminology Definitions

## Host Classifications

When executing `cderun` recursively (nested execution), the following host environments are distinguished:

### Base Host (Level 0)

The physical computer or virtual machine where the initial `cderun` execution begins. This is where the container runtime engine (Docker or Podman) is physically running. This corresponds to nested context Level 0.

```bash
# Executed on the Base Host (Level 0):
cderun --mount-cderun gemini-cli
```

### Execution Host

The immediate parent environment where the current `cderun` process is executing. This can be either the Base Host or a container environment (Level 1 and above).

```bash
# 1. Executed on the Base Host (Execution Host is the Base Host):
cderun --mount-cderun node

# 2. Executed inside the 'node' container (Execution Host is the 'node' container):
cderun python script.py
```

## Example Walkthrough

```text
Base Host (Physical Machine)
  ↓ cderun gemini-cli
gemini-cli Container (Execution Host)
  ↓ cderun python script.py
python Container
```

In this setup:

- **Base Host**: The physical machine.
- **Execution Host**: The `gemini-cli` container (the host environment executing the command to spawn the `python` container).

## cderun Expressions

Special string templates specified inside configuration profiles (`.cderun.yaml`, `.tools.yaml`) or CLI flags to resolve dynamic properties. They use the `{{...}}` double-brace syntax.
`cderun` parses and evaluates these expressions at run-time, replacing them with resolved host metrics.

### Categories

- **Magic Words**: Keywords with predefined meanings, such as `{{HOME}}` or `{{PWD}}`.
- **Directives**: Directives that instruct the resolver to perform specific I/O actions, indicated by a colon prefix, such as `{{file:.go-version}}` (reads the specified file's contents).

## Argument & Flag Classifications

### cderun Internal Overrides (Phase 1 / P1)

CLI options prefixed with `--cderun-`. They hold the highest precedence (P1) and, in standard Wrapper Mode, must be placed **after** the subcommand. Argument preprocessing ("Hoisting") extracts and relocates these flags before the subcommand internally prior to parsing. Placing them before the subcommand in Wrapper Mode results in a validation error (no placement restrictions apply in Diagnosis Mode).

### cderun Standard Flags (Phase 2 / P2)

Standard configurations (such as `--tty` or `--env`) that manage the container lifecycle. These must be placed **before** the subcommand.

### Passthrough Arguments

All arguments placed after the subcommand, excluding `--cderun-` prefixed override flags. These arguments are forwarded directly to the containerized tool.

### Hoisting

The preprocessing mechanism in `cderun` that automatically scans the arguments following the subcommand, extracts P1 internal overrides, and relocates them to the front. This isolates configurations designated for `cderun` from the arguments passed to the wrapped tool.

## Environment Variable Inheritance

- **Execution Host Variables**: Dynamically inherited by specifying them in `--env <KEY>` on the CLI or configurations.
- **Base Host Variables**: Cannot be inherited directly by nested child containers without being passed sequentially through each intermediate execution host layer.
