# Feature Specification: Polyglot Entry Point (Symlink Mode)

## Overview

When `cderun` is executed under a name other than `cderun` (such as via a symbolic link), it interprets the invoking executable file's name as the **subcommand**. This provides a seamless, polyglot entry point where you can execute containerized commands transparently as if they were natively installed.

---

## Technical Requirements & Behavior

### Argument Rewriting Logic

Upon program startup (at the very beginning of the `main` function), `cderun` inspects and potentially rewrites command-line arguments based on the following rules:

1. **Check Executable Name**
   Extract the base name of `os.Args[0]` (the filename, excluding directory path segments).

2. **Conditional Rewriting**
   - **Case A: Base name is `cderun`**
     Do nothing. Proceed with standard execution.
   - **Case B: Base name is NOT `cderun`**
     The arguments are rewritten in a two-step process:
     - **Rewrite step 1 (Binary invocation rewrite)**: Emit the `cderun` command as `os.Args[0]`.
     - **Rewrite step 2 (Hoisting & Subcommand placement)**: Hoist only `--cderun-*` flags before the symlink-derived subcommand, then preserve the subcommand and all remaining arguments in their original order.
     The final structure of rewritten `os.Args` is:
     `os.Args = [ "cderun", ...hoisted_cderun_flags, <extracted_base_name>, ...original_arguments_excluding_hoisted_flags ]`

---

## Detailed Examples

### Example 1: Basic Symlink Execution

Assume a symlink named `node` points to `cderun`:

```bash
# User executes the command:
node --version
```

- **Actual Process Startup**: `os.Args = ["node", "--version"]`
- **Rewritten Internal Args**: `os.Args = ["cderun", "node", "--version"]`
- **Result**: `cderun` is invoked with `node` as the subcommand lookup key. The tool maps `node` to its configured image and executes the command within a container, returning the Node.js version.

### Example 2: Mixing settings with Symlink Modes

You can configure `cderun` behavior using P1 internal override flags even when invoking via a symlink:

```bash
# User executes the command:
node --cderun-tty=false --version
```

- **Actual Process Startup**: `os.Args = ["node", "--cderun-tty=false", "--version"]`
- **Rewritten Internal Args**: `os.Args = ["cderun", "--cderun-tty=false", "node", "--version"]`
- **Result**: Equivalent to running `cderun --tty=false node --version`. The `cderun` engine processes `--cderun-tty=false` as a high-priority P1 setting, while the wrapped `node` subcommand runs and processes the `--version` passthrough argument in original order.

---

## Hoisting Restrictions in Polyglot Mode

In **Symlink Mode** (Polyglot Entry Point), a strict hoisting restriction is enforced to prevent argument collision:

- **Rule**: Only flags prefixed with `--cderun-` (P1 internal overrides) are eligible for hoisting from behind the subcommand.
- **Behavior**: Standard, non-prefixed `cderun` flags (such as `--interactive` or `--tty`) that appear *after* the subcommand are **never hoisted**. They are treated as literal, raw passthrough arguments and are forwarded directly to the wrapped container tool.

This design ensures that standard flags belong strictly to the wrapped application (e.g., if the wrapped binary has its own `--tty` or `--interactive` flag, `cderun` will not intercept or hijack it). If you need to override `cderun` parameters when executing via a symlink, you **must** use the explicit `--cderun-` prefix.

---

## Security Validations on Symlink Names

In **Symlink Mode**, the invoking filename is directly extracted and consumed as the subcommand/lookup key. To enforce a secure-by-default posture and prevent directory traversal, injection, or obfuscation attacks, `cderun` applies strict security validation to the symlink name:

1. **Character Whitelist (ValidateToolName)**:
   The extracted name must be composed strictly of the following allowed ASCII characters:
   - Alphanumeric characters (`a-z`, `A-Z`, `0-9`)
   - Period (`.`)
   - Underscore (`_`)
   - Hyphen (`-`)

2. **Homograph & Obfuscation Prevention**:
   Any characters outside the whitelisted ASCII set—such as Cyrillic characters (e.g., Cyrillic 'о' -> `\u043e`), non-printable chars, control sequences, or directory traversal segments—are strictly rejected. If validation fails, `cderun` aborts execution immediately with a clear security validation error. This prevents malicious symlinks from executing arbitrary tools or bypassing security checks.
