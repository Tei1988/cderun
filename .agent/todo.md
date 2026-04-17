# TODO

## Code Improvement

### P-9: Add containerd Runtime Support (Priority: Medium)

Runtime implementation と CI pipeline を同時に進める。

Currently `cderun` supports Docker and Podman (Podman reuses the Docker-compatible API).
Add native containerd support as a third runtime, using the containerd Go client directly.

New files:

```text
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

### Inconsistency in containerd Support
The security validator in `internal/config/resolver.go` allows `containerd` as a valid runtime, but the `runtimeFactory` in `internal/command/root.go` does not yet support it. This should be addressed when implementing native containerd support.

### DeviceConfig YAML Format
`DeviceConfig.UnmarshalYAML` in `internal/config/path.go` currently only supports string format (`host:container[:perms]`). It should be updated to support object format as mentioned in some documentation.

### Anchor Validation Regex
`magicWordPreRegex` in `internal/config/path.go` identifies anchors for `~` even when not at the start of a path (e.g., in `/foo/~bar`), but the expansion logic in `internal/config/expression.go` only expands `~` at the start of a string. This inconsistency may lead to unexpected path traversal validation errors for paths that are not actually expanded.

### System Memory Inconsistency (containerd)
The AI system memory (context) claims that native containerd support is fully implemented, but the `internal/runtime/containerd.go` file is missing from the repository. The implementation should follow the plan outlined in TODO P-9.
