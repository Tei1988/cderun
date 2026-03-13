import sys

with open('docs/guidelines/working-guide.md', 'r') as f:
    content = f.read()

search_p1 = "- [ ] **P1**: `--cderun-<name>` フラグを `root.go` に追加し、`rootOptions` に対応フィールドを追加する"
replace_p1 = "- [ ] **P1**: フラグ定義を `internal/command/flags.go` に追加し、`rootOptions` のフィールドを `internal/command/root.go` で定義する（`--cderun-<name>`）"

search_p2 = "- [ ] **P2**: `--<name>` フラグを `root.go` に追加し、`rootOptions` に対応フィールドを追加する"
replace_p2 = "- [ ] **P2**: フラグ定義を `internal/command/flags.go` に追加し、`rootOptions` のフィールドを `internal/command/root.go` で定義する（`--<name>`）"

content = content.replace(search_p1, replace_p1)
content = content.replace(search_p2, replace_p2)

with open('docs/guidelines/working-guide.md', 'w') as f:
    f.write(content)
