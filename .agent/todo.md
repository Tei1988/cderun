# TODO

## Code Improvement

### P-1: Unified Option Definition Table

Options are currently duplicated across `flags.go` (P1/P2), `resolver.go` (P3/P4/P5/P6),
and `config.go`. Define a central `OptionRegistry` that maps flag names to their
respective environment variables, YAML keys, and defaults.
Use a unified schema like `map[string]any` or struct+reflection, and generate
`registerFlags()` / `resolveSettings()` / `ResolveWithFS()` from this table via loops.

Scope: `flags.go`, `root.go` (resolveSettings), `resolver.go` (ResolveWithFS), `option.go`
Migration: **COMPLETED**. All option types (String, Bool, Int, Float64, StringSlice) are unified.

### P-4: Remove Global Variables `opts` / `rootCmd`

Current `init()` creates global state, forcing a separate code path for tests via
`ExecuteContextWithOptions`. Unify by always creating fresh state:

```go
// Before
var opts = defaultOptions()
var rootCmd *cobra.Command
func init() { rootCmd = newRootCmd(&opts) }

// After — delete init() and globals
func Execute(rawArgs []string) error {
    return ExecuteContext(context.Background(), rawArgs)
}
func ExecuteContextWithOptions(ctx context.Context, rawArgs []string, setup func(*rootOptions, *cobra.Command)) error {
    o := defaultOptions()
    o.logger = logging.NewLogger()
    cmd := newRootCmd(&o)
    if setup != nil { setup(&o, cmd) }
    // ... same flow
}
```

Production and test paths become identical. No state leaks between runs.

### P-6: Typed Error Handling

All errors are currently `fmt.Errorf` strings, making programmatic error inspection difficult.
Introduce typed errors incrementally, starting with the most common patterns:

```go
// internal/errors.go (new file)
type ImageNotFoundError struct{ Tool string }
func (e *ImageNotFoundError) Error() string { return "no image mapping found for tool: " + e.Tool }

type RuntimeInitError struct{ Runtime string; Err error }
func (e *RuntimeInitError) Error() string { return "failed to initialize runtime: " + e.Err.Error() }
func (e *RuntimeInitError) Unwrap() error { return e.Err }

type InvalidConfigError struct{ Field, Value string; Err error }
func (e *InvalidConfigError) Error() string {
    return fmt.Sprintf("invalid %s value %q: %v", e.Field, e.Value, e.Err)
}
func (e *InvalidConfigError) Unwrap() error { return e.Err }
```

Usage: `return nil, &ImageNotFoundError{Tool: subcommand}`
Tests: `assert.ErrorAs(t, err, &ImageNotFoundError{})`
Migrate from `fmt.Errorf` gradually — image-not-found, invalid-config, runtime-init first.

### P-7: Isolate Docker SDK Dependency via Adapter Layer

`docker.go` directly uses Docker SDK types (`dockercontainer.Config`, `network.NetworkingConfig`).
Extract conversion logic into a dedicated adapter:

```
internal/runtime/
  interface.go          # ContainerRuntime (unchanged)
  docker.go             # DockerRuntime — delegates to adapter for type conversion
  docker_adapter.go     # Docker SDK type conversion (new)
  podman.go             # Wraps DockerRuntime (unchanged)
```

```go
// docker_adapter.go
func toDockerContainerConfig(cc *container.ContainerConfig) (
    *dockercontainer.Config, *dockercontainer.HostConfig, *network.NetworkingConfig,
) {
    // Consolidate all container.ContainerConfig → Docker SDK type mapping here
}
```

Move conversion logic from `CreateContainer` in `docker.go` to `docker_adapter.go`.
Future Podman native API support only requires adding `podman_adapter.go`.

### P-9: Add containerd Runtime Support (Priority: Medium)

Runtime implementation と CI pipeline を同時に進める。

Currently `cderun` supports Docker and Podman (Podman reuses the Docker-compatible API).
Add native containerd support as a third runtime, using the containerd Go client directly.

New files:

```
internal/runtime/
  containerd.go         # ContainerdRuntime implementing ContainerRuntime interface
  containerd_test.go    # Unit tests with mock containerd client
```

Implementation outline:

```go
// containerd.go
type ContainerdRuntime struct {
    client *containerd.Client
}

func NewContainerdRuntime(socket string) (ContainerRuntime, error) {
    c, err := containerd.New(socket)
    if err != nil { return nil, err }
    return &ContainerdRuntime{client: c}, nil
}

func (r *ContainerdRuntime) Name() string { return "containerd" }
// Implement remaining ContainerRuntime methods using containerd client API:
// PullImage       → client.Pull()
// CreateContainer → client.NewContainer() + NewTask()
// StartContainer  → task.Start()
// WaitContainer   → task.Wait()
// AttachContainer → task.IO() with cio options
// RemoveContainer → container.Delete() + task.Delete()
// SignalContainer → task.Kill()
// ResizeContainerTTY → task.Resize()
// InspectContainer   → task.Status()
```

Auto-detection changes in `resolver.go`:

```go
// Add containerd socket detection (after docker, before_podman fallback)
} else if _, err := fs.Stat("/run/containerd/containerd.sock"); err == nil {
    res.Runtime = "containerd"
    res.SocketPath = "/run/containerd/containerd.sock"
}
```

Runtime instantiation (add `"containerd"` case to the existing switch):

```go
case "containerd":
    rt, err = runtime.NewContainerdRuntime(resolved.SocketPath)
```

CI pipeline (parallel with runtime implementation):
- Add GitHub Actions workflow (or extend existing) with a containerd service container.
- Run `go test -tags=runtime ./...` against containerd.
- Matrix: `[docker, podman, containerd]` for E2E tests.

Scope: `internal/runtime/containerd.go`, `internal/config/resolver.go`,
`internal/command/root.go` (runtime switch), CI workflow files.
Dependency: `github.com/containerd/containerd/v2` client library.

## Terminal / TTY
- macOS ターミナルで cderun 経由で kiro-cli を実行中、カーソルがターミナルの右端に到達するとターミナル自体が強制終了される。TTY ハンドリングまたはリサイズシグナル周りの問題の可能性あり。


## Testing & Maintenance

- Outdated usage string for `hang-timeout` in `internal/command/flags.go` still mentions `2s` instead of the 10s default.
