# ContainerRuntime L3 Conformance Suite Guide

## Overview

The `ContainerRuntime` L3 Conformance Suite (`RunConformanceTests` in `internal/runtime/conformance_suite_test.go`) defines the standard behavioral contract for all container engine adapters in `cderun`.

The primary purpose of the conformance suite is to guarantee that every `ContainerRuntime` implementation—whether Docker, Podman, direct containerd, or MockRuntime—behaves identically when executing intermediate container configurations (`container.ContainerConfig`). When a runtime adapter does not support a specific feature (such as port forwarding or named volume mounts in containerd), the suite enforces that the adapter **must explicitly reject** the configuration with an error during `ValidateConfig`, rather than passing it through silently or dropping options unnoticed.

For more information on `cderun`'s overarching testing principles, see the [Testing Strategy](strategy.md).

---

## Suite Architecture & Capabilities Guard

The conformance test harness receives a factory function that instantiates the adapter under test, along with a `ConformanceCapabilities` struct describing the adapter's supported feature set.

### Factory Function Signature

```go
type RuntimeFactory func(t *testing.T) ContainerRuntime
```

The factory function creates a fresh `ContainerRuntime` instance for each test case and handles cleanup via `defer rt.Close()`.

### `ConformanceCapabilities` Options

The `ConformanceCapabilities` struct dictates which test cases expect successful validation versus explicit rejection:

| Capability Field | Type | Description | Engine Support Summary |
| :--- | :--- | :--- | :--- |
| `SupportsVolumes` | `bool` | Whether named volume mounts (`type=volume`) are supported. | Docker: `true`, Podman: `true`, containerd: `false` (rejected per T51) |
| `SupportsTmpfs` | `bool` | Whether `tmpfs` mounts (`type=tmpfs`) are supported. | Docker: `true`, Podman: `true`, containerd: `true` |
| `SupportsPorts` | `bool` | Whether port publishing (`-p`, `--publish`) is supported. | Docker: `true`, Podman: `true`, containerd: `false` (rejected) |
| `SupportsGPUs` | `bool` | Whether GPU passthrough (`--gpus`) is supported. | Docker: `true`, Podman: `true`, containerd: `false` (rejected) |
| `SupportsDNSSearch` | `bool` | Whether custom DNS search domains (`--dns-search`) are supported. | Docker: `true`, Podman: `true`, containerd: `false` (rejected) |
| `SupportsDNSOptions` | `bool` | Whether custom DNS options (`--dns-option`) are supported. | Docker: `true`, Podman: `true`, containerd: `false` (rejected) |
| `SupportsInit` | `bool` | Whether init process injection (`--init`) is supported. | Docker: `true`, Podman: `true`, containerd: `false` (rejected) |
| `RequiresCapPrefix` | `bool` | Whether Linux capability names require OCI `CAP_` prefixes (e.g. `CAP_SYS_ADMIN`). | Docker: `false`, Podman: `false`, containerd: `true` |

---

## Standard Conformance Test Cases

`RunConformanceTests` executes the following automated test sub-cases:

1. **`Name`**: Verifies `rt.Name()` returns a non-empty engine identifier string (e.g. `"docker"`, `"podman"`, `"containerd"`, `"mock"`).
2. **`ValidateConfig_ValidBasicConfig`**: Validates a minimal valid `ContainerConfig` containing an image reference and command.
3. **`ValidateConfig_VolumeMountContract_T51`**: Asserts that `type=volume` mounts pass when `SupportsVolumes` is `true`, or return an explicit error containing `"volume"` when `false`.
4. **`ValidateConfig_TmpfsMountContract_T51`**: Asserts that `type=tmpfs` mounts pass when `SupportsTmpfs` is `true`, or return an explicit error when `false`.
5. **`ValidateConfig_PortsContract`**: Asserts port mapping handling according to `SupportsPorts`.
6. **`ValidateConfig_DNSSearchContract`**: Asserts DNS search domain handling according to `SupportsDNSSearch`.
7. **`ValidateConfig_DNSOptionsContract`**: Asserts DNS options handling according to `SupportsDNSOptions`.
8. **`ValidateConfig_UnsupportedGPUsContract`**: Asserts GPU passthrough handling according to `SupportsGPUs`.
9. **`ValidateConfig_InitContract`**: Asserts container init process handling according to `SupportsInit`.
10. **`ValidateConfig_CapabilitiesNormalizationContract_T45`**: Verifies capability short names (e.g., `SYS_ADMIN`) and checks OCI `CAP_` prefix conversion when `RequiresCapPrefix` is `true`.
11. **`ValidateConfig_FullOptionsContract`**: Validates a comprehensive `ContainerConfig` with environment variables, resource limits (`Memory`, `CPUs`, `CPUShares`, `CpusetCpus`, `CpusetMems`, `PidsLimit`), `ShmSize`, `Workdir`, `User`, `SecurityOpt`, and `Sysctls`.
12. **`Lifecycle_CreateStartWaitInspectRemove`**: Executes a complete container lifecycle loop (`ValidateConfig` -> `CreateContainer` -> `StartContainer` -> `WaitContainer` -> `InspectContainer` -> `RemoveContainer`), asserting clean exit codes (0) and state transitions.

---

## Onboarding Procedure for New ContainerRuntime Adapters

To onboard a new `ContainerRuntime` implementation into the L3 Conformance Suite, follow these steps:

### Step 1: Implement the `ContainerRuntime` Interface

Ensure your adapter implements all methods defined in `internal/runtime/runtime.go`:

```go
type ContainerRuntime interface {
	Name() string
	ValidateConfig(cfg *container.ContainerConfig) error
	PullImage(ctx context.Context, image string, opts PullOptions) error
	CreateContainer(ctx context.Context, cfg *container.ContainerConfig) (string, error)
	StartContainer(ctx context.Context, id string) error
	AttachContainer(ctx context.Context, id string, streams IOStreams) error
	WaitContainer(ctx context.Context, id string) (int, error)
	InspectContainer(ctx context.Context, id string) (bool, int, error)
	RemoveContainer(ctx context.Context, id string) error
	SignalContainer(ctx context.Context, id string, sig os.Signal) error
	ResizeContainerTTY(ctx context.Context, id string, rows, cols uint16) error
	Close() error
}
```

### Step 2: Implement Fail-Fast Guards in `ValidateConfig`

Your adapter's `ValidateConfig` method MUST check for options that your engine cannot support and return an explicit error rather than silently ignoring them. For example:

```go
if len(cfg.Ports) > 0 {
	return fmt.Errorf("myruntime runtime: ports mapping is not supported yet")
}
```

### Step 3: Define Factory and Capabilities in Test File

Create a test function in `internal/runtime/conformance_suite_test.go` (or a dedicated test file in `internal/runtime/`) that initializes your adapter and capability declarations:

```go
func TestConformance_MyNewRuntime(t *testing.T) {
	factory := func(t *testing.T) ContainerRuntime {
		// Initialize adapter (using mock client or test fixture)
		return NewMyNewRuntime(...)
	}
	caps := ConformanceCapabilities{
		SupportsVolumes:    true,
		SupportsTmpfs:      true,
		SupportsPorts:      false, // Set according to adapter capabilities
		SupportsGPUs:       false,
		SupportsDNSSearch:  false,
		SupportsDNSOptions: false,
		SupportsInit:       false,
		RequiresCapPrefix:  true,
	}
	RunConformanceTests(t, factory, caps)
}
```

### Step 4: Run Unit and Integration Conformance Tests

- **Unit Testing (Mock Client)**: Run tests without a live daemon requirement during standard unit testing:

  ```bash
  go test -v ./internal/runtime/ -run TestConformance_
  ```

- **Integration Testing (Live Socket)**: When testing against live container daemons in CI or integration environments, set `CDERUN_RUNTIME`:

  ```bash
  CDERUN_RUNTIME=mynewruntime go test -v ./internal/runtime/ -run TestConformance_
  ```

---

## Execution Environment & Image Reference Guidelines

1. **Fully Qualified Image References**: Always use fully qualified image references (e.g., `docker.io/library/alpine:latest`) in test fixtures to ensure compatibility across Docker, Podman, and containerd reference resolvers.
2. **Resource Cleanup**: Always ensure containers and sockets are cleaned up using `defer rt.Close()` and `RemoveContainer` in lifecycle tests.
3. **Fail-Fast Principles**: Never pass unsupported configuration parameters through to OCI or engine specs without validation. Misconfigurations must fail at `ValidateConfig` time before container creation starts.
