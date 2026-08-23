# Feature Specification: Polyglot Entry Point (Symlink Mode)

## Overview

When `cderun` is executed under a name other than `cderun` (such as via a symbolic link or mounted tool wrapper), it interprets the invoking executable file's name as the **subcommand**. This provides a seamless, polyglot entry point where you can execute containerized commands transparently as if they were natively installed on the host or inside a container.

---

## Technical Requirements & Behavior

### Argument Rewriting Pipeline

Upon program startup (at the very beginning of the `main` function before any flag parsing or command processing occurs), `cderun` inspects and potentially rewrites command-line arguments based on the following deterministic pipeline:

1. **Executable Name Extraction**
   Extract the base name of `os.Args[0]` (the filename, excluding directory path segments and file extensions).

2. **Conditional Rewriting**
   - **Case A: Base name is `cderun` (or `cderun.exe` on Windows)**
     Do nothing. Proceed with standard Wrapper Mode or Ad-hoc Mode execution.
   - **Case B: Base name is NOT `cderun` (e.g. `node`, `python`, `git`)**
     The arguments are rewritten in a two-step process:
     - **Rewrite Step 1 (Binary Invocation Rewrite)**: Emit `"cderun"` as `os.Args[0]` to normalize standard CLI parsing across Cobra and `cderun` internal routines.
     - **Rewrite Step 2 (Hoisting & Subcommand Placement)**: Scan all arguments appearing after `os.Args[0]`. Extract only Phase 1 (P1) internal override flags prefixed with `--cderun-` (and their associated values), hoisting them before the extracted symlink subcommand. Place the extracted symlink base name immediately after the hoisted `--cderun-` flags as the subcommand, followed by all remaining original arguments in their exact sequence.

   The final structure of rewritten `os.Args` is:

```text
   os.Args = [ "cderun", ...hoisted_cderun_flags, <extracted_base_name>, ...original_arguments_excluding_hoisted_flags ]
   ```

---

## Detailed Examples

### Example 1: Basic Symlink Execution

Assume a symbolic link named `node` points to `cderun`:

```bash
# User executes the command:
node --version
```

- **Actual Process Startup**: `os.Args = ["node", "--version"]`
- **Rewritten Internal Args**: `os.Args = ["cderun", "node", "--version"]`
- **Result**: `cderun` is invoked with `node` as the subcommand lookup key. The tool maps `node` to its configured image in `.tools.yaml` (e.g., `node:20-alpine`) and executes `node --version` within a container, returning the Node.js version output.

### Example 2: Mixing P1 Override Settings with Symlink Modes

You can configure `cderun` execution options using P1 internal override flags even when invoking via a symlink:

```bash
# User executes the command:
node --cderun-tty=false --cderun-env=NODE_ENV=production --version
```

- **Actual Process Startup**: `os.Args = ["node", "--cderun-tty=false", "--cderun-env=NODE_ENV=production", "--version"]`
- **Rewritten Internal Args**: `os.Args = ["cderun", "--cderun-tty=false", "--cderun-env=NODE_ENV=production", "node", "--version"]`
- **Result**: Equivalent to running `cderun --tty=false --env NODE_ENV=production node --version`. The `cderun` engine processes `--cderun-tty=false` and `--cderun-env=NODE_ENV=production` with P1 priority, while the wrapped `node` subcommand receives `--version` as a pure passthrough argument.

---

## Hoisting Restrictions in Polyglot Mode

In **Symlink Mode** (Polyglot Entry Point), a strict hoisting restriction is enforced to prevent argument collision between `cderun` and the wrapped application:

- **Strict Prefix Rule**: Only flags prefixed with `--cderun-` (Phase 1 internal overrides) are eligible for extraction and hoisting from behind the subcommand.
- **Literal Passthrough Behavior**: Standard, non-prefixed flags (such as `--interactive`, `--tty`, `-e`, `-v`, or `--env`) that appear after the symlink executable name are **never hoisted**. They are treated as literal, raw passthrough arguments and are forwarded directly to the wrapped container tool.

### Design Rationale

This design ensures that standard flags belong strictly to the wrapped application. For instance, if a wrapped tool like `docker` or `python` has its own `--tty`, `-v`, or `--env` flags, `cderun` will never hijack or misinterpret them. If you need to override `cderun` execution settings when executing via a symlink, you **must** use the explicit `--cderun-` prefix.

---

## Security Validations on Symlink Names

In **Symlink Mode**, the invoking filename is directly extracted and consumed as the subcommand/lookup key. To enforce a secure-by-default posture and prevent directory traversal, injection, or obfuscation attacks, `cderun` applies strict security validation to the symlink name via `ValidateToolName`:

1. **Character Whitelist (`ValidateToolName`)**:
   The extracted tool name must be composed strictly of the following allowed ASCII characters:
   - Lowercase and uppercase alphanumeric characters (`a-z`, `A-Z`, `0-9`)
   - Period (`.`)
   - Underscore (`_`)
   - Hyphen (`-`)

2. **Homograph & Obfuscation Prevention**:
   Any characters outside the whitelisted ASCII set—such as Cyrillic homographs (e.g., Cyrillic 'о' -> `\u043e`), non-printable ASCII, control sequences (`\x00`, `\n`), or directory traversal segments (`..`, `/`, `\`)—are strictly rejected. If validation fails, `cderun` aborts execution immediately with an explicit security validation error. This prevents malicious symlinks from executing arbitrary binaries or bypassing configuration lookup rules.
