# Feature Specification: Argument and Setting Priority Logic

## Overview

`cderun` loads configurations from multiple sources, including the CLI, environment variables, YAML profiles, and internal default values. When configuration conflicts arise, `cderun` resolves and determines the final settings according to the priority hierarchy defined below, ranging from **P1 (highest)** to **P6 (lowest)**.

## Resolution Hierarchy

Settings are resolved in the following priority order, from highest to lowest:

### P1: CDERUN Internal Overrides (Highest Priority)

- **Definition**: Dedicated flags designed to force-override the behavior of `cderun`. These flags enable specifying settings on the `cderun` side even when using symbolic links (Polyglot Mode) without conflicting with the arguments of wrapped tools.
- **Flag Names**: All supported standard `cderun` options have a corresponding P1 counterpart prefixed with `--cderun-`.
  - **Key P1 Flags**:
    - **Execution Control**: `--cderun-image`, `--cderun-env`, `--cderun-tty`, `--cderun-interactive`, `--cderun-workdir`, `--cderun-user`, `--cderun-group-add`, `--cderun-network`, `--cderun-runtime`, `--cderun-strict-env`, `--cderun-entrypoint`, `--cderun-pull`, `--cderun-pull-max-retries`, `--cderun-pull-backoff-base`, `--cderun-prefetch`, `--cderun-prefetch-all`, `--cderun-remove`, `--cderun-restart`, `--cderun-hang-timeout`
    - **Mounting & Isolation**: `--cderun-mount`, `--cderun-mount-tools`, `--cderun-mount-all-tools`, `--cderun-mount-cderun`, `--cderun-mount-cderun-path`, `--cderun-mount-socket`, `--cderun-mount-socket-path`, `--cderun-socket-path`
    - **Networking & Ports**: `--cderun-publish`, `--cderun-publish-all`, `--cderun-expose`, `--cderun-hostname`, `--cderun-dns`, `--cderun-dns-option`, `--cderun-dns-search`, `--cderun-add-host`
    - **Resources & Security**: `--cderun-memory`, `--cderun-cpus`, `--cderun-cpu-shares`, `--cderun-cpuset-cpus`, `--cderun-cpuset-mems`, `--cderun-device`, `--cderun-gpus`, `--cderun-ipc`, `--cderun-cgroupns`, `--cderun-sensitive-env`, `--cderun-privileged`, `--cderun-read-only`, `--cderun-pid`, `--cderun-pids-limit`, `--cderun-init`, `--cderun-security-opt`, `--cderun-ulimit`, `--cderun-shm-size`, `--cderun-sysctl`, `--cderun-cap-add`, `--cderun-cap-drop`
    - **Diagnostics & Logging**: `--cderun-dry-run`, `--cderun-dry-run-format`, `--cderun-diagnosis`, `--cderun-diagnosis-format`, `--cderun-log-level`, `--cderun-log-format`, `--cderun-log-timestamp`, `--cderun-config`, `--cderun-tool-config`
- **Behavior**: When specified, these values take absolute precedence, completely ignoring all other sources (P2–P6).
- **Placement Rules and Hoisting**:
  - **Wrapper Mode**: By rule, these flags must be placed **after** the subcommand. During preprocessing, `cderun` scans the arguments after the subcommand, extracts any `--cderun-*` flags, and internally "hoists" (relocates) them before the subcommand prior to standard parsing.
  - **Advantages**: This perfectly isolates wrapped tool-specific flags (e.g., `node --env`) from `cderun` configurations (e.g., `node --cderun-env`).
  - **Validation Check**: Specifying a P1 internal override before the subcommand in Wrapper Mode is strictly prohibited to prevent confusion with standard P2 flags, resulting in a validation error.
  - **Diagnosis Mode**: Since no subcommand is required, these flags can be placed anywhere.

#### Concrete P1 Internal Override Examples

Below are concrete invocation examples demonstrating Phase 1 internal overrides using both equals-sign (`=`) and space-separated formats:

```bash
# Override container image using equals-sign format
cderun node app.js --cderun-image=node:20-alpine

# Override container image using space-separated format (hoists both flag and adjacent value)
cderun node app.js --cderun-image node:20-alpine

# Override environment variables & working directory after subcommand
cderun python script.py --cderun-env=LOG_LEVEL=debug --cderun-workdir /workspace

# Combine boolean toggles and value-taking overrides in Polyglot Symlink Mode
node app.js --cderun-tty=false --cderun-shm-size 512m
```

For details regarding the hoisting mechanism, refer to [Argument Parsing & Hoisting](./argument-parsing.md).

### P2: CLI Flags (User Intent)

- **Definition**: Standard CLI options explicitly provided by the user at execution time. These must be placed **before** the subcommand.
- **Flag Names**:
  - `--tty`, `--interactive`, `--image`, `--network`, `--runtime`, `--socket-path`, `--mount-socket`, `--mount-socket-path`, `--env`, `--workdir`, `--mount`, `--mount-cderun`, `--mount-cderun-path`, `--mount-tools`, `--mount-all-tools`, `--remove`, `--config`, `--tool-config`
  - `--publish`, `--publish-all`, `--expose`, `--hostname`, `--dns`, `--dns-option`, `--dns-search`, `--add-host`, `--user`, `--group-add`, `--privileged`, `--read-only`, `--init`, `--pid`, `--pids-limit`, `--cap-add`, `--cap-drop`, `--entrypoint`, `--pull`, `--pull-max-retries`, `--pull-backoff-base`, `--prefetch`, `--prefetch-all`, `--strict-env`, `--sensitive-env`, `--memory`, `--cpus`, `--cpu-shares`, `--cpuset-cpus`, `--cpuset-mems`, `--device`, `--gpus`, `--ipc`, `--cgroupns`, `--security-opt`, `--ulimit`, `--shm-size`, `--sysctl`, `--restart`, `--hang-timeout`
  - `--dry-run`, `--dry-run-format`, `--diagnosis`, `--diagnosis-format`, `--log-level`, `--log-format`, `--log-timestamp`
- **Condition**: Applied only when explicitly specified by the user on the command line. Otherwise, the resolution proceeds to P3 and below.

### P3: Environment Variables (Global Overrides)

- **Definition**: Settings applied globally across the execution host environment.
- **Key Variables**:
  - **Configuration & Execution**: `CDERUN_CONFIG`, `CDERUN_TOOL_CONFIG`, `CDERUN_IMAGE`, `CDERUN_RUNTIME`, `CDERUN_SOCKET_PATH`, `CDERUN_REMOVE`, `CDERUN_RESTART`, `CDERUN_STRICT_ENV`, `CDERUN_HANG_TIMEOUT`, `CDERUN_PREFETCH`, `CDERUN_PREFETCH_ALL`
  - **I/O & TTY**: `CDERUN_TTY`, `CDERUN_INTERACTIVE`, `CDERUN_ENV`, `CDERUN_WORKDIR`, `CDERUN_HOSTNAME`, `CDERUN_USER`, `CDERUN_GROUP_ADD`
  - **Networking**: `CDERUN_NETWORK`, `CDERUN_PUBLISH`, `CDERUN_PUBLISH_ALL`, `CDERUN_EXPOSE`, `CDERUN_DNS`, `CDERUN_DNS_OPTION`, `CDERUN_DNS_SEARCH`, `CDERUN_ADD_HOST`
  - **Mounting & Tools**: `CDERUN_MOUNT`, `CDERUN_MOUNT_SOCKET`, `CDERUN_MOUNT_SOCKET_PATH`, `CDERUN_MOUNT_CDERUN`, `CDERUN_MOUNT_CDERUN_PATH`, `CDERUN_MOUNT_TOOLS`, `CDERUN_MOUNT_ALL_TOOLS`
  - **Resources & Privileges**: `CDERUN_MEMORY`, `CDERUN_CPUS`, `CDERUN_CPU_SHARES`, `CDERUN_CPUSET_CPUS`, `CDERUN_CPUSET_MEMS`, `CDERUN_DEVICE`, `CDERUN_GPUS`, `CDERUN_IPC`, `CDERUN_CGROUPNS`, `CDERUN_PRIVILEGED`, `CDERUN_READ_ONLY`, `CDERUN_PID`, `CDERUN_PIDS_LIMIT`, `CDERUN_INIT`, `CDERUN_SECURITY_OPT`, `CDERUN_ULIMIT`, `CDERUN_SHM_SIZE`, `CDERUN_SYSCTL`, `CDERUN_CAP_ADD`, `CDERUN_CAP_DROP`
  - **Image Retrieval**: `CDERUN_PULL`, `CDERUN_PULL_MAX_RETRIES`, `CDERUN_PULL_BACKOFF_BASE`, `CDERUN_ENTRYPOINT`
  - **Diagnostics & Logging**: `CDERUN_DRY_RUN`, `CDERUN_DRY_RUN_FORMAT`, `CDERUN_DIAGNOSIS`, `CDERUN_DIAGNOSIS_FORMAT`, `CDERUN_LOG_LEVEL`, `CDERUN_LOG_FORMAT`, `CDERUN_LOG_TIMESTAMP`, `CDERUN_SENSITIVE_ENV`
- **Evaluation Timing (`CDERUN_CONFIG` / `CDERUN_TOOL_CONFIG`)**: These variables are evaluated *before* searching for configuration files to determine the respective configuration loading paths.
- **List Separators**:
  - Semicolon (`;`): `CDERUN_ENV`, `CDERUN_MOUNT`
  - Comma (`,`): `CDERUN_PREFETCH`, `CDERUN_GROUP_ADD`, `CDERUN_MOUNT_TOOLS`, `CDERUN_PUBLISH`, `CDERUN_EXPOSE`, `CDERUN_DNS`, `CDERUN_DNS_OPTION`, `CDERUN_DNS_SEARCH`, `CDERUN_ADD_HOST`, `CDERUN_CAP_ADD`, `CDERUN_CAP_DROP`, `CDERUN_ENTRYPOINT`, `CDERUN_DEVICE`, `CDERUN_SECURITY_OPT`, `CDERUN_SENSITIVE_ENV`, `CDERUN_ULIMIT`, `CDERUN_SYSCTL`

### P4: Tool-specific Config (YAML Profile)

- **Definition**: The configuration block inside the tools configuration file (`.tools.yaml`) tied to the target subcommand (tool) being executed.
- **Behavior**: Selected if neither CLI flags nor environment variables are specified.

### P5: Global Defaults (Profile Default)

- **Definition**: The global `defaults` configuration block inside the default configuration file (`.cderun.yaml`).
- **Behavior**: Selected if no configurations are specified from P1 to P4.

### P6: Hardcoded Defaults (Lowest Priority)

- **Definition**: The final fallback values hardcoded inside the `cderun` binary.
- **Default Values**:
  - `tty: false`
  - `interactive: false`
  - `network: bridge` (Default network for Docker/Podman engines; note that direct containerd adapter requires `--network host` as `bridge` is rejected)
  - `remove: true`
  - `readOnly: false`
  - `init: false` (Docker/Podman tini init; rejected by containerd adapter)
  - `pid: ""` (Empty string defaults to private PID namespace or runtime default; `"host"` shares host PID namespace)
  - `shmSize: ""` (Empty string uses container engine default, e.g. Docker 64MB; zero or empty produces no adapter overrides)
  - `ipc: ""` (Empty string defaults to private IPC namespace; `"host"` shares host IPC namespace)
  - `cgroupns: ""` (Empty string uses runtime default cgroup namespace mode; `"host"` or `"private"` configures cgroup namespace explicitly)
  - `pidsLimit: 0` (`0` skips the adapter override; `-1` explicitly requests unlimited pids, passed directly to Docker and mapped by containerd to OCI unlimited)
  - `cpuShares: 0` (Zero represents relative default CPU weight with no adapter override)
  - `restart: ""` (Empty string represents no restart policy)
  - `prefetchAll: false` (`false` disables automatic tool image prefetching on invocation)
  - `runtime`: None (automatically detected from available sockets in the order of `docker` -> `containerd` -> `podman`, falling back to `docker` if none are found)
  - `cpusetCpus`: "" (Empty string skips cpuset CPU core restrictions)
  - `cpusetMems`: "" (Empty string skips NUMA memory node restrictions)
  - `gpus`: "" (Empty string skips GPU device requests)
  - `sysctls`: nil (Empty/nil list skips sysctl kernel parameter overrides)
  - `pull: missing`
  - `pullMaxRetries: 3`
  - `pullBackoffBase: 1s`
  - `logLevel: error`
  - `logFormat: text`
  - `logTimestamp: true`
  - `strictEnv: false`
  - `mountSocket: false`
  - `mountCderun: false`
  - `mountAllTools: false`
  - `privileged: false`
  - `publishAll: false`
  - `dryRun: false`
  - `dryRunFormat: yaml`
  - `diagnosis: false`
  - `diagnosisFormat: yaml`
  - `hangTimeout: 10s`
  - `sensitiveEnv: nil` (Unset; means all environment variable values are masked)
  - `image`: None (Trigger Fatal Error if unresolved)

## Resolution Example

Consider a scenario where the `tty` option is specified in multiple locations:

1. **P6 (Fallback)**: `false` (hardcoded)
2. **P5 (Global)**: `tty: true` in `.cderun.yaml`
3. **P4 (Tool)**: `tty: false` in the `node` section of `.tools.yaml`
4. **P3 (Env)**: `export CDERUN_TTY=true`
5. **P2 (CLI P2)**: `cderun --tty=false node ...`
6. **P1 (CLI P1)**: `cderun node ... --cderun-tty=true`

In this case, the final value adopted is **P1's `true`**. If P1 were not specified, P2 (`false`) would win; if P2 were missing, P3 (`true`) would win, and so forth.

## Collection and List-type Option Resolution

List configurations such as `mounts`, `devices`, `env`, `ports`, and `mountTools` follow the same resolution model: **"If a higher-priority source contains a value, lower-priority sources are completely ignored (overwritten)."**

**Critical Principle**:
To support intentional removal or clearing of lists, specifying an **explicitly empty list** in a higher-priority source (e.g., `mounts: []` in YAML or `export CDERUN_ENV=""` in environment variables) is treated as an intentional empty setting. This completely overwrites and disables configuration values defined in any lower-priority sources.

- **Example**: If `.tools.yaml` (P4) defines `mounts: []`, any `mounts` specified in `.cderun.yaml` (P5) are completely ignored, resulting in an empty mount list.
- **Environment Variable Example**: Setting `export CDERUN_ENV=""` (P3) disables any environment variables defined in `.tools.yaml` (P4) or `.cderun.yaml` (P5).

### Intra-source Deduplication

Within a single source (e.g., only CLI flags or inside a single YAML block), if a duplicate list-type key is defined (especially environment keys), **the last specified value wins**.

- **Example**: Running `cderun --env A=1 --env A=2 node` results in environment variable `A` holding the value `2` inside the container.

## Transitive Auto-enablement

Certain options are dynamically and transitively enabled based on the resolution state of other options. These transitive rules apply **only when the target option remains unconfigured (`nil`) across all P1 to P5 layers**:

1. **`mountCderun` Auto-enablement**:
   If `mountTools` is specified (non-empty) or if `mountAllTools` resolves to `true`, `mountCderun` is transitively set to `true`.
2. **`mountSocket` Auto-enablement**:
   If `mountCderun` resolves to `true` (including transitively), `mountSocket` is transitively set to `true`.

**Note**: If an option is explicitly set to `false` in any layer (P1–P5), its dynamic auto-enablement is suppressed. For example, if `.cderun.yaml` configures `mountSocket: false`, the socket will not be mounted even if `mountCderun` resolves to `true`.
