# TODO

## Container Runtime
- Improve image pull rate limit handling (e.g., configurable retry strategies or registry-specific backoff).

## Testing & Maintenance
- Fix potential bug in `internal/command/root.go:431`: `config.NewExpressionResolver` is used instead of `config.NewExpressionResolverWithFS(resolved.HostContext, o.fs)`, which might cause issues during nested execution if it relies on the host's real filesystem instead of the provided `o.fs`.
- Investigate missing coverage for `SignalContainer` failure and `AttachTimeout` in `internal/command/root.go` (requires complex mocking of internal signals and constants).
