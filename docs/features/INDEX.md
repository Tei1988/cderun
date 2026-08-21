# cderun Feature Specifications Index

## Overview

This directory contains the detailed technical and functional specifications for each feature of `cderun`.

## Feature Specifications

### Core Mechanisms

1. **[Argument Parsing & Hoisting](./argument-parsing.md)**
   - Explains the strict boundary parsing mechanism that separates `cderun`'s flags from the subcommand arguments.
   - Details the hoisting (relocation) mechanics of Phase 1 (P1) internal overrides.

2. **[Argument & Setting Priority Logic](./argument-priority-logic.md)**
   - Defines the configuration precedence hierarchy from P1 (internal overrides) down to P6 (defaults).
   - Explains the deterministic evaluation sequence used to select winning options.

3. **[Polyglot Entry Point (Symlink Mode)](./polyglot-entry.md)**
   - Explains how creating symbolic links to `cderun` enables transparent, native-like command execution.
   - Explains tool-name auto-detection and character whitelisting security checks.

4. **[Configuration File Support](./configuration-file-support.md)**
   - Documents the structure and role of `.cderun.yaml` (global defaults) and `.tools.yaml` (tool mapping).
   - Explains the hierarchical directory traversal, search, and list overwriting/merging sequence.

5. **[Standard Input Synchronization](./stdin-synchronization.md)**
   - Details the synchronous coordination of container startup and STDIN attachment to ensure no data loss during piped executions.

6. **[Value Resolution & Expression Engine](./value-resolution.md)**
   - Documents dynamic expression evaluation (`{{...}}`), tilde expansion, relative path resolution, anchor boundary validation, and lazy instantiation.

7. **[Hang Timeout Protection](./hang-timeout.md)**
   - Documents the automated termination logic (SIGKILL) to prevent execution hangs in non-TTY, non-interactive environments when IO finishes.

### Runtime Integration

1. **[Multi-Runtime Support](./multi-runtime-support.md)**
   - Documents native support for Docker, Podman, and containerd engines, socket auto-detection, and host socket caching.

2. **[Direct Container Execution](./direct-container-execution.md)**
   - Details the direct SDK/API communication flow with container daemons, avoiding subprocess overhead and shell injection risks.

3. **[Image Mapping & Selection](./image-mapping.md)**
   - Documents how subcommand names are mapped to specific container images, and how to customize default tags.

### Execution Environment

1. **[Environment Variable Passthrough](./env-passthrough.md)**
   - Details how host environment variables are securely and selectively passed down to the container.

2. **[Mount Tools Dynamically](./mount-tools.md)**
   - Documents the dynamic injection of other tools' `cderun` wrappers into a running container environment.

3. **[Container Command Lifecycle](./container-command-execution.md)**
   - Explains ephemeral container creation, startup, termination, exit-code capture, and automatic cleanup.

### Advanced Capabilities

1. **[Docker-Compatible CLI Options](./command-line-options.md)**
   - Outlines port publishing, resource constraints, supplementary groups, and user ID overriding, mapping them to the engine specs.

2. **[Nested Execution (Recursive Containers)](./nested-execution.md)**
   - Details how `cderun` executes itself inside a container recursively, including snapshotting, reverse path resolution, and macOS VM GID setup.

3. **[Nested Execution Control Socket](./nested-execution-control-socket.md)**
   - Specifies a `cderun`-native control plane that replaces raw engine socket mounting for nested execution, enabling runtime-agnostic and scoped nested invocations.

4. **[Dry-Run Mode](./dry-run-mode.md)**
   - Details previewing container configuration without execution, outputting in YAML, JSON, or Simple text format.

5. **[Logging & Debugging](./logging-debugging.md)**
   - Documents trace, debug, info, warn, and error levels, JSON/text formats, early initialization, and sanitized logging.

6. **[Interactive Terminal UX](./interactive-terminal.md)**
   - Details TTY allocation, Unix signal forwarding (SIGINT/SIGTERM), and terminal size (winch) synchronization.

### Management & System Diagnostics

1. **[Version Management](./version-management.md)**
   - Details Git metadata injection (Tag, SHA, BuildDate) and formatted `--version` outputs.

2. **[Diagnosis Mode](./diagnosis-mode.md)**
   - Documents system diagnostic verification and tool discovery without requiring a subcommand.

### Security Hardening

1. **[Security Validations](./security-validations.md)**
   - Details character whitelisting, absolute path enforcement on targets, and strict validation of critical fields.

2. **[Sensitive Data Protection](./sensitive-data-protection.md)**
   - Documents the secure-by-default environment masking (`[REDACTED]`), fail-closed glob fallback, and quoted logging.

3. **[Signal Handling Security](./signal-handling-security.md)**
   - Details signal validation and Unix/Windows signal constraints to prevent injection of malicious control signals.

## Technical References

- **[Proc-Self-Mountinfo Specification](../references/proc-self-mountinfo.md)**

## Execution Pipeline

```text
Argument Parsing → Priority Logic → Intermediate Config (ContainerConfig)
                                                   │
                                          Runtime Adapter Selection
                                                   │
                                        Direct Container Execution
                                                   │
                                         Lifecycle & IO Coordination
```
