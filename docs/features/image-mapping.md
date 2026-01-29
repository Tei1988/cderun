# イメージマッピング (Phase 2予定)

## 概要

`cderun`はサブコマンドを適切なコンテナイメージに自動マッピングし、Dockerイメージを手動で指定する必要をなくします。

## 設定

```yaml
# ~/.config/cderun/config.yaml
images:
  node: "node:18-alpine"
  python: "python:3.11-slim"
  custom-tool: "my-registry/custom:latest"
```

### エラーハンドリング
- マッピングが存在しない場合、エラーを出力して終了
- 例: `cderun unknown-tool` → `Error: No image mapping found for 'unknown-tool'`
- ユーザーは明示的に `--image` フラグでイメージを指定する必要がある

## メリット

- **便利性**: イメージ名を記憶する必要がない
- **一貫性**: 標準化されたイメージ選択
- **柔軟性**: カスタマイズ可能なマッピング
