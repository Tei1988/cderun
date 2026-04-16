# 機能仕様：イメージマッピング (完了)

## 概要

`cderun`はサブコマンドを適切なコンテナイメージに自動マッピングし、コンテナイメージを手動で指定する必要をなくします。

## 設定

設定は `.tools.yaml` で行います。

```yaml
# .tools.yaml
node:
  image: "node:18-alpine"
python:
  image: "python:3.11-slim"
custom-tool:
  image: "my-registry/custom:latest"

```

### エラーハンドリング

- マッピングが存在しない場合、エラーを出力して終了
- 例: `cderun unknown-tool` → `Error: no image mapping found for tool: unknown-tool`
- ユーザーは明示的に `--image` フラグでイメージを指定する必要がある

## メリット

- **便利性**: イメージ名を記憶する必要がない
- **一貫性**: 標準化されたイメージ選択
- **柔軟性**: カスタマイズ可能なマッピング
