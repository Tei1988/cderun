# TODO

## Code Improvement

### P-9: Add containerd Runtime Support (Priority: Medium)

CI pipeline implementation is pending.

CI pipeline:
- Add GitHub Actions workflow (or extend existing) with a containerd service container.
- Run `go test -tags=runtime ./...` against containerd.
- Matrix: `[docker, podman, containerd]` for E2E tests.

Scope: CI workflow files.

## Terminal / TTY
- macOS ターミナルで cderun 経由で kiro-cli を実行中、カーソルがターミナルの右端に到達するとターミナル自体が強制終了される。TTY ハンドリングまたはリサイズシグナル周りの問題の可能性あり。


## Testing & Maintenance
