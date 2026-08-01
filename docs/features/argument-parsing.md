# Feature Specification: Argument Parsing & Hoisting

`cderun` acts as a wrapper tool. It is equipped with an argument parsing mechanism designed to strictly distinguish between flags meant for `cderun` itself and commands/arguments destined to run inside the container (passthrough arguments).

---

## Basic Syntax

```bash
cderun [cderun-flags] <subcommand> [passthrough-args]
```

- **[cderun-flags]**: Flags that control the behavior of `cderun`. This includes standard flags (P2) placed **before** the subcommand, and internal overrides (P1) starting with `--cderun-` which can be placed **after** the subcommand.
- **\<subcommand\>**: The first non-flag argument. It serves as a **Lookup Key** to locate tool configurations within `.tools.yaml` and is not included in the container's execution command by default.
- **[passthrough-args]**: All arguments following the subcommand. These are forwarded directly as arguments to the container command, except for `--cderun-` overrides which are hoisted to the front.

---

## Container Image Selection Sequence

The container image is selected according to the following precedence hierarchy. For a detailed resolution process, please refer to [Argument & Setting Priority Logic](./argument-priority-logic.md).

1. `--image` / `--cderun-image` flags (P1/P2)
2. `CDERUN_IMAGE` environment variable (P3)
3. Lookup Key `<subcommand>` matches inside `.tools.yaml` (P4)
4. Failing all of the above, execution aborts with a configuration resolution error.

---

## Commands Forwarded to the Container

- The `<subcommand>` is consumed as a lookup key and is not included in the container's execution command.
- The execution command executed within the container consists strictly of the `[passthrough-args]`.

### Execution Examples

- **Case 1: Match Found in `.tools.yaml`**

  Given a `.tools.yaml` configuration of `my-tool: {image: alpine, entrypoint: [/usr/bin/my-tool-impl]}`:

  ```bash
  cderun my-tool -l -a
  ```

  Within the container, the command executed is `/usr/bin/my-tool-impl -l -a`.

- **Case 2: Explicit `--image` and `--entrypoint` supplied**

  ```bash
  cderun --image=golang:1.22 --entrypoint=go go --version
  ```

  `go` (the subcommand) is consumed as the lookup key. Inside the container, `go --version` is executed.

- **Case 3: Unregistered Subcommand without Image Specification**

  ```bash
  cderun go --version
  ```

  If `.tools.yaml` does not define `go` and no image is supplied via environment variables or flags, execution immediately fails with a resolution error.

---

## Boundary Detection

Before invoking the standard Cobra flag parser, `cderun` runs a dedicated preprocessing step (`preprocessArgs`) to identify the boundary of the subcommand.

### 1. Sequential Scanning and Skipping

The preprocessor scans the argument list from left to right, skipping standard `cderun` flags (P2) and their values:

- **Flag-Value Association**: If a flag expects an argument (e.g., `--image`, `-w`) and does not use the equals-sign format (e.g., `--image alpine`), the subsequent argument is skipped along with the flag itself.
- **Shorthand Flags**: Handles combined shorthands (e.g., `-it`) and those requiring arguments (e.g., `-p 80:80`).

### 2. Identifying the Subcommand

The first argument encountered during this scan that is neither a registered `cderun` flag nor an associated argument value is designated as the **subcommand**.

### 3. Preserving Standard Flags Order

During boundary scanning and preprocessing, the relative order of standard (P2) flags (e.g., `--tty`, `--env`) is strictly preserved and not modified. Only Phase 1 (P1) internal override flags (`--cderun-*`) placed after the subcommand are relocated. This ensures that the downstream Cobra flag parser interprets standard flags in the exact sequence specified by the user.

---

## Phase 1 (P1) Internal Overrides Hoisting

Flags prefixed with `--cderun-` are called **"Phase 1 (P1) Internal Overrides"**, possessing the highest priority in the configuration hierarchy.

### 1. Placement Rule

In standard **Wrapper Mode**, `--cderun-` flags **must** be placed **after** the subcommand. Specifying them before the subcommand is strictly prohibited and triggers an immediate error to ensure clear flag ownership.

### 2. Hoisting Mechanics

During preprocessing (`preprocessArgs`), the preprocessor scans the argument list behind the subcommand, extracts all `--cderun-` prefixed flags and their values, and prepends (hoists) them before the subcommand. This ensures that these configuration flags are parsed as `cderun` settings instead of being passed to the container command.

#### Equals-Sign Format Constraints for Value-Taking Flags

To guarantee robust, unambiguous preprocessing, any internal override flag that takes a value (e.g., `--cderun-image`, `--cderun-workdir`) **must use the equals-sign format** (e.g., `--cderun-image=alpine`).

Specifying a value-taking override flag without an equals-sign (e.g., `--cderun-image alpine`) is strictly rejected with an explicit validation error (e.g., `cderun internal override flag "--cderun-image" must use '=' format to specify its value`) to prevent accidental hoisting of standalone flags and downstream parser corruption. Boolean override flags (e.g., `--cderun-tty`) require no value and can be hoisted autonomously.

#### Preprocessing Transformation Example

```text
[Initial User Input]
cderun node app.js --cderun-tty --cderun-image=node:20-alpine

[Post-Preprocessing (Hoisted)]
cderun --cderun-tty --cderun-image=node:20-alpine node app.js
```

By the time the Cobra parser is invoked, all `cderun`-specific settings are gathered at the front, leaving only pure passthrough arguments following the subcommand.

---

## Double-Dash (`--`) Hoisting Exemption (Not Supported)

To simplify argument parsing and avoid semantic ambiguity with shell-native and application-specific option delimiters, `cderun` does **NOT** support double-dash (`--`) for stopping or exempting arguments from hoisting.

### Rules of Behavior

1. **No Delimiter Exemption**: The argument preprocessor scans the entire list of arguments following the subcommand. It does not treat a double-dash (`--`) as a barrier to stop the extraction of `--cderun-` prefixed flags.
2. **Always Hoisted**: Any `--cderun-` prefixed flags appearing anywhere in the argument list (even after a `--` delimiter) are **always** hoisted to the front of the command as part of `cderun`'s configuration parsing.
3. **No Double-Dash Hoisting Prevention**: This design ensures robust, predictable hoisting behavior that remains independent of shell-level option interpretation. Future modifications are prohibited from introducing double-dash hoisting-prevention mechanisms to maintain maximum parsing simplicity.

---

## Symlink Mode & Polyglot Entry Point

When `cderun` is executed via a symbolic link (e.g., `node` -> `cderun`), the base name of the executable (`node`) is automatically detected as the subcommand.

In this mode:

- Only `--cderun-` prefixed flags appearing after the executable name are hoisted.
- Standard flags without the `--cderun-` prefix (such as `--env`) appearing after the executable name are treated as literal passthrough arguments and are forwarded directly to the containerized tool.

```bash
# Executed via symlink 'node':
node --env DEBUG=app app.js --cderun-env=NODE_ENV=production

# Internal Resolution Workflow:
# 1. Identifies 'node' as the subcommand from the executable name.
# 2. Scans and detects '--cderun-env=NODE_ENV=production', hoisting it to the front.
# 3. Keeps '--env DEBUG=app app.js' intact as passthrough arguments.
# 4. Executes 'node --env DEBUG=app app.js' in the container, applying 'NODE_ENV=production' internally with P1 priority.
```

---

## Test Scenario Requirements

```bash
# The first '--tty' is processed as a cderun flag (P2), while the second '--tty' is passed literally to docker.
cderun --tty docker --tty
```
