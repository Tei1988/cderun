# cderun Feature Specifications Index

## Overview

This directory contains the technical specifications and requirement definitions for all `cderun` features.

## Feature Document List

### Core Features

1. **[Argument Parsing](./argument-parsing.md)**
   - Explains strict boundary separation between `cderun` flags and wrapped command arguments.
   - Details the preprocessing and "hoisting" of P1 internal override flags.

2. **[Argument & Setting Priority Logic](./argument-priority-logic.md)**
   - Defines the priority hierarchy spanning P1 (internal overrides) down to P6 (hardcoded defaults).
   - Explains the exact algorithm utilized to select winning settings.

3. **[Polyglot Entry Point (Symlink Mode)](./polyglot-entry.md)**
   - Explains how creating symlinks to `cderun` (such as `node` or `python`) triggers transparent containerized execution.
   - Details tool auto-detection from executable invocation paths.

4. **[Configuration File Support](./configuration-file-support.md)**
   - Defines schemas and roles for `.cderun.yaml` (global settings) and `.tools.yaml` (tool mappings).
   - Explains upward directory searching, strict YAML decoding, and array collection overrides.

5. **[Standard Input Synchronization](./stdin-synchronization.md)**
   - Outlines how standard input attachment coordinates with container startup to prevent pipe stream loss.

6. **[Value Resolution](./value-resolution.md)**
   - Explains dynamic `{{...}}` expressions, tilde expansion, relative path absolute resolutions, and anchor boundary security validations.

7. **[Hang Timeout](./hang-timeout.md)**
   - Details how `cderun` prevents hanging container tasks in non-interactive sessions using bounded grace timeouts.

### Runtime Engines

1. **[Multi-Runtime Support](./multi-runtime-support.md)**
   - Outlines integrated support for Docker, Podman, and containerd engines.
   - Explains runtime auto-detection sequences and socket lookup performance caching.

2. **[Direct Container Execution](./direct-container-execution.md)**
   - Explains direct gRPC/SDK API integration instead of slow or insecure shell command wrapping.

3. **[Image Mapping](./image-mapping.md)**
   - Details how subcommand lookup keys translate into container images and custom registries.

### Environment & Lifecycle

1. **[Environment Variable Passthrough](./env-passthrough.md)**
   - Explains how host environment variables are selectively mapped, validated, and passed through.

2. **[Mount Tools](./mount-tools.md)**
   - Outlines the dynamic tool injection feature to mount other containerized binary aliases on demand.

3. **[Container Command Execution](./container-command-execution.md)**
   - Explains user command routing and container lifecycle coordination.

### Advanced Options

1. **[Command Line Options (Docker Parity)](./command-line-options.md)**
   - Comprehensive reference of all available CLI parameters, environment variables, short forms, and internal overrides.

2. **[Binary Mounting and Nested Execution](./nested-execution.md)**
   - Outlines recursive `cderun` execution support, directory snapshot generation, host path tracing, and macOS nested setup constraints.

3. **[Dry-Run Mode](./dry-run-mode.md)**
   - Explains how to preview resolved `ContainerConfig` structures in YAML, JSON, or Simple formats.

4. **[Logging and Debugging](./logging-debugging.md)**
   - Explains internal log trace levels, format properties, and timestamp configuration.

5. **[Interactive Terminal UX](./interactive-terminal.md)**
   - Outlines pseudo-TTY allocation, window resize (`SIGWINCH`) forwarding, and raw mode recovery.

### Utility & Administration

1. **[Version Management](./version-management.md)**
   - Outlines dynamic version metadata injection (tags, SHA, compile date) via linker variables.

2. **[Diagnosis Mode](./diagnosis-mode.md)**
   - Explains the host diagnostic analyzer (`--diagnosis`).

### Security

1. **[Security Validations](./security-validations.md)**
   - Outlines strict sanitization layers covering path chars, tool whitelist names, signals, and cgroup parameters.

2. **[Sensitive Data Protection](./sensitive-data-protection.md)**
   - Explains secret protection, environment variable masking, and fail-closed error formatting.

3. **[Signal Handling Security](./signal-handling-security.md)**
   - Outlines security checks preventing signal injection exploits on the host or runtimes.

---

## Technical Specifications and Relationships

```text
Arguments Parsing ──> Priority Resolution ──> Intermediate Representation (ContainerConfig)
                                                                 │
                                                       Runtime Engine Selector
                                                                 │
                                                    Direct API Client (Docker/gRPC)
                                                                 │
                                                       Container Lifecycle
```
