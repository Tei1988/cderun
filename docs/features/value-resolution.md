# Feature Specification: Value Resolution & Expression Engine

In `cderun`, option values specified within configuration files or command-line arguments undergo a multi-layered evaluation and conversion process prior to execution. This dynamic evaluation allows flexible, context-aware configurations while maintaining strict security constraints.

---

## Recursive Resolution Mechanics

Value resolution is recursively applied down configuration trees. This ensures that dynamic expressions, tildes, and relative paths are correctly expanded regardless of their depth.

### Resolution Targets

- **Strings**: Evaluated directly for expression expansion, tilde expansion, and relative-to-absolute path resolution. Supported across CLI flags and YAML properties (e.g., `--image`, `--workdir`, `--shm-size`, `--prefetch`).
- **Slices (`[]any` / `[]string`)**: Each element is parsed and resolved recursively (e.g., `--env`, `--mount`, `--ulimit`, `--sysctl`, `--security-opt`, `--dns`, `--entrypoint`).
- **Maps (`map[string]any` / `map[string]string`)**: Values of each key-value pair are recursively resolved.

### Sequence Splitting vs Expression Resolution Order

Options that support comma-separated list values follow a deterministic order depending on whether they are scalar string options or string-slice options:

1. **Scalar Comma-Separated Options (e.g., `--prefetch`)**:
   Dynamic expression resolution occurs **first** on the full scalar string (e.g. `{{env:TOOLS}}` resolving to `"node,python"`). The resulting resolved string is subsequently split by commas into discrete tool names.
2. **String-Slice Options via Environment Variables (e.g., `CDERUN_ENTRYPOINT`, `CDERUN_CAP_ADD`, `CDERUN_DNS`)**:
   Separation occurs **first**: the environment variable string is split into slice elements using its designated separator (comma or semicolon). Dynamic expressions within each resulting element are then evaluated recursively. When passed as repeated CLI flags (e.g., `--entrypoint /bin/sh --entrypoint -c`), Cobra constructs the slice directly and each element undergoes expression evaluation individually.

---

## cderun Expressions (`{{...}}`)

Dynamic variables and expressions can be embedded inside strings using the double-brace `{{...}}` syntax.

The expression engine automatically trims leading and trailing whitespaces within braces. For instance, `{{ HOME }}` is normalized and evaluated identically to `{{HOME}}`.

### Anchor Boundary Validation

To prevent directory traversal attacks (such as specifying `{{HOME}}/../../etc/passwd`), `cderun` enforces strict **Anchor Boundary Validation** on any path resolved via magic words, custom expressions, or tilde expansions.

#### Validation Workflow

1. **Identify Anchors**: The engine scans for any `{{...}}` expression or leading `~` that acts as a path origin (anchor).
2. **Determine Anchor Boundaries**: Each anchor is individually resolved to an absolute directory, representing the permissible "boundary directory".
3. **Resolve Path**: The path undergoes complete expression evaluation, tilde expansion, and relative path resolution, yielding a normalized absolute target path.
4. **Boundary Verification**: The engine checks whether the resolved target path resides strictly within the boundary directory of **every** active anchor. If the normalized path escapes the boundary directory (e.g., traverses up past the anchor root using `..`), the engine raises an immediate security validation error.

#### Pass/Fail Scenarios

Assuming the anchor `{{HOME}}` resolves to `/home/user`:

| Raw Input Path | Resolved Absolute Path | Verdict | Reason |
| :--- | :--- | :--- | :--- |
| `{{HOME}}/Documents/data.txt` | `/home/user/Documents/data.txt` | **PASS** | Resides within boundary directory |
| `{{HOME}}/Documents/../data.txt` | `/home/user/data.txt` | **PASS** | Resides within boundary directory |
| `{{HOME}}/..` | `/home` | **FAIL** | Escapes boundary directory (goes into parent) |
| `{{HOME}}/../../etc/passwd` | `/etc/passwd` | **FAIL** | Escapes boundary directory (directory traversal) |

#### Under-the-Hood Security Logic

Anchor boundary checks leverage Go's `filepath.Clean` and `filepath.Rel` to construct a platform-agnostic, normalized relationship between the anchor directory (e.g., `/home/user`) and the resolved target path (e.g., `/etc/passwd`).

If the computed relative path begins with `../` (or `..\` on Windows) or is `..`, it indicates that the target path escapes the boundary. In such cases, execution is aborted with an error resembling:

```text
path traversal detected: "{{HOME}}/../../etc/passwd" escapes anchor boundary "/home/user"
```

This ensures full defense against directory traversals across different operating systems.

### Multiple Anchors Evaluation

If a single path string contains multiple anchors (e.g., `{{HOME}}/{{PWD}}/file`), the resolved absolute path must simultaneously satisfy the boundary verification check for **every** anchor present.

For example, if `{{HOME}}` is `/home/user` and `{{PWD}}` is `/work`, the path `{{HOME}}/{{PWD}}/file` evaluates to `/home/user/work/file`, which is then rejected because it does not lie within `/work`'s boundary directory.

In nested expressions (e.g., `{{env:DIR:-{{HOME}}}}`), the inner expression is evaluated first to supply the fallback parameters for the outer expression. Boundary checking is ultimately applied to the resolved anchors that contribute to the final path.

---

## Expression Types

### 1. Magic Words

Magic words represent internal, pre-defined constants representing the host's execution context.

| Keyword | Description |
| :--- | :--- |
| `{{HOME}}` | Expands to the home directory of the current host execution user. |
| `{{PWD}}` | Expands to the current working directory of the host execution environment. |
| `{{BASE_HOME}}` | Expands to the home directory of the **base host** (Level 0 host/VM). Under nested execution, `{{HOME}}` expands to the container's home (e.g., `/root`), whereas `{{BASE_HOME}}` preserves the original physical host's home path. |
| `{{BASE_PWD}}` | Expands to the working directory of the **base host** (Level 0 host/VM). Under nested execution, `{{BASE_PWD}}` retains the initial directory where the host process was started. |

### 2. Directives

Directives use the format `{{type:parameter}}` to query dynamic data sources.

| Directive | Description |
| :--- | :--- |
| `{{file:<filename>}}` | Reads the content of `<filename>`. The engine searches for this file by traversing upwards from the current directory, then fallback searching through `~/.config/cderun/`, `/etc/cderun/`, and `/run/cderun/`. The content is stripped of leading/trailing whitespaces. Files exceeding **1MB** (`MaxDirectiveFileSize`) are strictly rejected. Parameters must be simple filenames without path separators or parent directory traversal (`..`) segments. Supports fallbacks using `{{file:<filename>:-default}}` when the file is missing, cannot be read/stat, or when trimmed content is empty. |
| `{{find_dir:<name>}}` | Traverses upwards searching for a directory or file named `<name>`, returning its absolute path on the host. Parameters must be simple names without path separators or parent directory traversal (`..`) segments. Supports fallbacks using `{{find_dir:<name>:-default}}` when the item is missing. |
| `{{env:<var_name>}}` | Queries the host environment variable `<var_name>`. Supports fallbacks using the `{{env:KEY:-default}}` syntax, which evaluates to `default` if the variable is empty or unset. Fallbacks can also contain nested expressions (e.g., `{{env:TAG:-{{file:.version}}}}`). |

### Directive Fallback Syntax & Rules (`:-default`)

All three directives (`file:`, `find_dir:`, and `env:`) support the `:-default` fallback syntax, which uses the first `:-` delimiter to separate the primary target name from its default value.

#### Fallback Trigger vs. Immediate Error Conditions

1. **Trigger Conditions (Fallback Is Evaluated)**:
   - **`file:`**: Target file does not exist, file read/stat fails (e.g., target is a directory or lacks read permissions), or trimmed file content is empty.
   - **`find_dir:`**: Target file or directory name is not found during upward directory traversal, or absolute-path resolution (`r.fs.Abs(dir)`) fails after the target is found.
   - **`env:`**: Environment variable is unset or empty (`""`).
   - *Behavior on Trigger*: When a fallback trigger occurs and a default value is provided, the resolution engine evaluates the default value, does **NOT** record a sticky error, and proceeds with the resolved fallback string.

2. **Immediate Error Conditions (Bypass Fallback)**:
   - **Parameter Validation Failure**: Target names containing path separators (`/` or `\`), parent directory traversal (`..`), control characters, or invalid UTF-8 sequences trigger immediate validation errors.
   - **Size Limit Violation**: Files exceeding the **1MB** (`MaxDirectiveFileSize`) threshold trigger an immediate size limit error.
   - *Behavior on Error*: Execution immediately aborts with a validation or size limit error without evaluating the `:-default` fallback.

#### Fallback Value Processing & Reverse Path Resolution

- **Nested Expression Evaluation**: Fallback default strings are evaluated recursively inside-out. For instance, in `{{find_dir:master:-{{PWD}}}}`, `{{PWD}}` is evaluated first to supply the fallback value.
- **Sticky Error Isolation**: A fallback-eligible condition (such as a missing file) caches a non-fatal error internally for the target name, allowing default evaluation to proceed without dirtying the resolver's sticky error state (`setError`).
- **Reverse Path Resolution Exemption**: Default values provided for `find_dir:` bypass host path reverse resolution (`applyReverseResolution`), as defaults represent explicitly specified fallback expression strings rather than dynamically discovered host paths.

### 3. Unrecognized and Unknown Expressions

To prevent silent failures and typos (such as typing `{{HOM}}` instead of `{{HOME}}`), the engine implements **Strict Resolution** rules for unrecognized brace contents:

1. **Immediate Failure**: If the content resembles a magic word (all uppercase letters and underscores, such as `{{HOM}}`) or a directive (contains a `:`, such as `{{envv:KEY}}`), the engine immediately throws an invalid expression error and halts execution.
2. **Literal Preservation**: If the content does not fit either of those patterns (e.g., `{{foo}}`), it is assumed to belong to another templating engine (such as Go templates or Ansible) and is preserved literally without changes.

### 4. Double-Brace Escaping Syntax

If you need to pass a literal string containing double braces to the container (such as passing a raw `{{HOME}}` string to `echo`), you can escape it by wrapping it inside an outer pair of braces:

- `{{ {{HOME}} }}` → `{{HOME}}`
- `{{{{HOME}}}}` → `{{HOME}}`
- `{{ {{file:config}} }}` → `{{file:config}}`

This escaping mechanism bypasses evaluation and also prevents strict resolution checks from failing on non-standard expressions.

### 5. Nested Expressions

Expressions can be nested to configure complex fallback scenarios across `env:`, `file:`, and `find_dir:` directives. The expression engine evaluates expressions from the inside out.

**Examples:**

```text
{{env:VERSION:-{{file:.version:-1.0.0}}}}
```

1. The inner expression `{{file:.version:-1.0.0}}` is evaluated first (e.g., resolving to `1.2.3` if `.version` exists, or `1.0.0` if missing).
2. The outer expression is evaluated with the fallback parameter: `{{env:VERSION:-1.2.3}}`.
3. If the host environment variable `VERSION` is set, its value is used; otherwise, it falls back to `1.2.3`.

```text
{{find_dir:master:-{{PWD}}}}
```

1. The inner expression `{{PWD}}` evaluates to the current working directory path (e.g., `/project/app`).
2. The outer `find_dir:master` directive traverses upwards for `master`. If not found, it falls back to `/project/app`.

---

## Security Hardening and Constraints

The value resolution engine implements multiple security layers:

### 1. Directive Parameter Restrictions

- **Absolute Paths Prohibited**: Parameters of `{{file:...}}` and `{{find_dir:...}}` must be simple filenames or directory names. Specifying absolute paths (such as `/etc/passwd`) is strictly rejected.
- **Path Separation Blocked**: Parameters must not contain path separators (`/` or `\`).
- **Parent Traversals Blocked**: Specifying `..` within directive parameters (e.g., `{{file:../config}}`) is strictly prohibited and triggers an immediate error.

### 2. Container Target Path Safety

Because container-side paths—such as the target directories in mount configs (`mc.Target`) and destination directories in device mappings (`dc.Destination`)—reside inside the container, they are **never** subjected to host-side absolute path conversions (such as those done by `SetBaseDir`).

This ensures that relative target directories are not silently mapped to host directories. During evaluation, container target paths must be absolute and non-empty, and relative inputs will be correctly detected and rejected.

### 3. Null-Byte, Control Character, and Invalid UTF-8 Injections Guard

To prevent string truncation and injection vulnerabilities in operating system, terminal, and container execution APIs, the engine scans environmental keys, values, and paths for null bytes (`\x00`), unescaped C0/C1 control characters (via `unicode.IsControl`), and invalid UTF-8 byte sequences. The presence of any null byte, control character, or invalid UTF-8 sequence triggers an immediate security validation error.

### 4. "Sticky Error" Pattern

To prevent invalid or compromised configurations from propagating, the evaluation engine implements a **Sticky Error** pattern:

1. The first resolution or security validation error encountered is captured and stored internally.
2. For all subsequent evaluation steps within the *same resolver instance*, the resolver transitions to a safe, non-evaluating fallback state where raw input strings are returned unmodified. Sibling expressions evaluated by the same resolver instance following a resolution failure remain unresolved.
3. Note that because each anchor path resolution creates a fresh `ExpressionResolver` instance, a resolution failure only affects subsequent evaluations using that same resolver instance; it does not leave expressions for other independent anchors unresolved.
4. Upon completing the resolution sweep, the retained error is propagated to the calling sequence, cleanly aborting execution.

---

## Tilde Expansion

Path strings starting with `~` or `~/` are expanded to the current user's home directory.

**Example:**

- `~/.ssh` -> `/home/user/.ssh`
- `~/project` -> `/home/user/project`

**Constraint**: Tilde expansion only applies if the tilde character resides at the **absolute start** of the string. Mid-path tildes (e.g., `/foo/~bar`) are treated as literal characters.

---

## Relative Path Resolution

Non-absolute paths (including bare values such as `config.yaml` as well as relative paths starting with `./` or `../`) are automatically converted to absolute paths using the following criteria:

1. **Within Configuration Files**: Resolved relative to the directory containing the configuration file (`BaseDir`).
2. **Within CLI Flags**: Resolved relative to the host's current working directory (`{{PWD}}`).

**Example**: Inside `/home/user/project/.tools.yaml`:

```yaml
mounts:
  - type: bind
    source: ./src
    target: /app
```

The `source` path is resolved to `/home/user/project/src`.

---

## Evaluation Sequence

Values are evaluated according to the following strict sequence:

```text
  Raw Input String (CLI / Env / YAML)
                 │
                 ▼
 ┌──────────────────────────────────────────────────┐
 │ Step 1: Null-Byte & UTF-8 / Control-Char Check   │ ── Invalid ──> Immediate Security Error
 └────────────────────────┬─────────────────────────┘
                          │
                          ▼
 ┌──────────────────────────────────────────────────┐
 │ Step 2: Evaluate {{...}} Dynamic Expressions     │ ── Error ────> Store Sticky Error
 └────────────────────────┬─────────────────────────┘
                          │
                          ▼
 ┌──────────────────────────────────────────────────┐
 │ Step 3: Expand Leading Tilde (~ / ~/)            │
 └────────────────────────┬─────────────────────────┘
                          │
                          ▼
 ┌──────────────────────────────────────────────────┐
 │ Step 4: Convert Relative Paths (non-absolute)    │
 └────────────────────────┬─────────────────────────┘
                          │
                          ▼
 ┌──────────────────────────────────────────────────┐
 │ Step 5: Path Cleaning & Normalization (Clean)    │
 └────────────────────────┬─────────────────────────┘
                          │
                          ▼
 ┌──────────────────────────────────────────────────┐
 │ Step 6: Anchor Boundary Safety Verification      │ ── Escapes ──> Store Sticky Error
 └────────────────────────┬─────────────────────────┘
                          │
                          ▼
            Final Resolved Value / Target
```

1. **Null-Byte, UTF-8 & Control-Character Validation**: Scan input strings for null bytes (`\x00`), unescaped C0/C1 control characters, and invalid UTF-8 byte sequences.
2. **cderun Expressions**: Evaluate double-brace `{{...}}` expressions.
3. **Tilde Expansion**: Resolve leading tildes to home directories.
4. **Relative Path Resolution**: Convert non-absolute paths (including bare values like `config.yaml` and relative paths starting with `./` or `../`) to absolute paths relative to configuration or CLI base contexts (`BaseDir` or `{{PWD}}`).
5. **Path Cleaning**: Normalize paths by removing redundant separators and parent traversals (e.g., `/home/user/project/../src` -> `/home/user/src`).
6. **Anchor Boundary Safety Verification**: Verify cleaned path against anchor boundaries (`validateAnchorBoundaries`), raising a sticky error if escaping anchor root.

---

## Performance Optimizations

### Process Socket Cache

Process socket caches store the auto-detected socket paths to avoid repeated disk I/O, as described in [Multi-Runtime Support](./multi-runtime-support.md).

### Lazy Resolver Instantiation

To minimize runtime overhead and resource usage, the `ExpressionResolver` is instantiated lazily via `getR()` only when:

- An expression marker (`{{`) is detected.
- A tilde expansion (`~`) is required.
- A relative path requires absolute conversion.
- Execution occurs under a nested context (`Level > 0`).

Static commands that do not utilize dynamic variables bypass resolver instantiation and filesystem scans entirely, resulting in near-zero overhead.
