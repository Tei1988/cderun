# TODO

## Code Improvement

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

