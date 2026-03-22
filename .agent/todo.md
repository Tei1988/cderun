# TODO

## Testing & Maintenance
- Refactor signal handling in `internal/command/root.go` to be mockable. Currently, it listens for real OS signals which makes unit testing the "second signal termination" logic risky.
- Fix `internal/command/root.go:431` to use `config.NewExpressionResolverWithFS(resolved.HostContext, o.fs)` instead of `config.NewExpressionResolver(resolved.HostContext)` to ensure nested execution respects mock filesystems during tests.

## Container Runtime
- Improve image pull rate limit handling (e.g., configurable retry strategies or registry-specific backoff).
