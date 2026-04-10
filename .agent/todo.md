# TODO

## Code Improvement

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

## Documentation Gap Analysis
- [ ] Discrepancy: Memory suggests containerd (v2.2.2) is integrated into CI, but `go.mod` and `ci.yaml` do not reflect this. Native containerd support is still under development (P-9).
