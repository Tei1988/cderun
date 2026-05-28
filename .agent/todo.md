# TODO

## Terminal / TTY
- macOS ターミナルで cderun 経由で kiro-cli を実行中、カーソルがターミナルの右端に到達するとターミナル自体が強制終了される。TTY ハンドリングまたはリサイズシグナル周りの問題の可能性あり。

## Testing & Maintenance

### DeviceConfig YAML Format
`DeviceConfig.UnmarshalYAML` in `internal/config/path.go` currently only supports string format (`host:container[:perms]`). It should be updated to support object format as mentioned in some documentation.

### Anchor Validation Regex
`magicWordPreRegex` in `internal/config/path.go` identifies anchors for `~` even when not at the start of a path (e.g., in `/foo/~bar`), but the expansion logic in `internal/config/expression.go` only expands `~` at the start of a string. This inconsistency may lead to unexpected path traversal validation errors for paths that are not actually expanded.
