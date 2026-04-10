# TODO

## Code Improvement

### P-9: Add containerd Runtime Support (Priority: Medium)

- [x] Runtime implementation
- [ ] CI pipeline
- Matrix: `[docker, podman, containerd]` for E2E tests.

Scope: `internal/runtime/containerd.go`, `internal/config/resolver.go`,
`internal/command/root.go` (runtime switch), CI workflow files.
Dependency: `github.com/containerd/containerd/v2` client library.

## Terminal / TTY
- macOS ターミナルで cderun 経由で kiro-cli を実行中、カーソルがターミナルの右端に到達するとターミナル自体が強制終了される。TTY ハンドリングまたはリサイズシグナル周りの問題の可能性あり。


## Testing & Maintenance
