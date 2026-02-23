# Feature: Logging and Debugging (Completed)

## 概要

cderunの動作を詳細に確認するためのログ出力とデバッグ機能。
内部的にはスレッドセーフな設計となっており、ロック競合を最小限に抑えるための最適化（ロック取得前のログレベル判定など）が施されています。

## ログレベル

### レベル定義

- `ERROR`: エラーのみ
- `WARN`: 警告とエラー（デフォルト）
- `INFO`: 一般的な情報
- `DEBUG`: 詳細なデバッグ情報
- `TRACE`: 最も詳細な情報（全ての内部ステップ、引数処理、APIコール等）

### 設定方法

#### コマンドライン

```bash
# 情報の表示 (INFO)
cderun --log-level info node app.js

# 詳細ログ (DEBUG)
cderun --log-level debug node app.js

# 最も詳細 (TRACE)
cderun --log-level trace node app.js
```

#### 設定ファイル

```yaml
# .cderun.yaml
logging:
  level: warn  # error | warn | info | debug | trace
  format: text  # text | json
  timestamp: true
```

#### 環境変数

```bash
export CDERUN_LOG_LEVEL=debug
export CDERUN_LOG_TIMESTAMP=true
```

## P1 Internal Overrides

他の設定同様、`--cderun-` プレフィックスを用いた Priority 1 オーバーライドが可能です。サブコマンドの後に指定し、設定ファイルや環境変数の値を強制的に上書きします。

- `--cderun-log-level`
- `--cderun-log-format`
- `--cderun-log-timestamp`

## ログ出力例

### デフォルト（WARNレベル）

```bash
cderun node app.js
Hello, World!
```

> **Note**: デフォルトでは `INFO` レベルの "Running: ..." 等のメッセージは表示されず、コマンドの出力のみが表示されます。

### INFO レベル

```bash
cderun --log-level info node app.js
2024-01-15 10:30:45 [INFO] Running: node app.js
Hello, World!
```

### DEBUG レベル

```bash
cderun --log-level debug node app.js
2024-01-15 10:30:45 [DEBUG] Loaded cderun config from: .cderun.yaml
2024-01-15 10:30:45 [DEBUG] Resolved Image: node:20-alpine
2024-01-15 10:30:45 [INFO] Running: node app.js
2024-01-15 10:30:45 [DEBUG] Image: node:20-alpine
2024-01-15 10:30:45 [DEBUG] Runtime: docker
2024-01-15 10:30:45 [DEBUG] Socket: /var/run/docker.sock
Hello, World!
2024-01-15 10:30:46 [DEBUG] Container exited with code: 0
```

### TRACE レベル

```bash
cderun --log-level trace node app.js
2024-01-15 10:30:45 [TRACE] Loading configurations...
2024-01-15 10:30:45 [DEBUG] Loaded cderun config from: .cderun.yaml
2024-01-15 10:30:45 [TRACE] Resolving configurations for tool: node
2024-01-15 10:30:45 [DEBUG] Resolved Image: node:20-alpine
2024-01-15 10:30:45 [INFO] Running: node app.js
...
2024-01-15 10:30:45 [TRACE] Creating container...
2024-01-15 10:30:45 [TRACE] Starting container: <ID>
2024-01-15 10:30:45 [TRACE] Waiting for container: <ID>
...
```

## フォーマット

### テキスト形式（デフォルト）

```text
2024-01-15 10:30:45 [INFO] Running: node app.js
```

### JSON形式

```bash
cderun --log-format json --log-level info node app.js
{"level":"info","msg":"Running: node app.js","time":"2024-01-15T10:30:45Z"}
```

## デバッグ機能

### 1. ドライラン

実行せずにコンテナ構成を表示します。詳細は[ドライランモード](./dry-run-mode.md)を参照してください。

```bash
cderun --dry-run node app.js
```

## 内部実装の注意点

### コンテナ出力のキャプチャ (`Logs: true`)

Dockerランタイムの実装 (`internal/runtime/docker.go`) において、`ContainerAttach` のオプションで `Logs: true` を設定しています。

これは、以下の経緯と理由によります：
1. **不具合修正の経緯**:
    - もともと `Logs: true` でしたが、標準入力からパイプで値を渡した際にプログラムが終了せずハングし、かつ出力が表示されないという不具合がありました。
    - その対策として、過去の修正で `Logs: false` への変更が試みられましたが、状況は改善されず（ハングおよび出力欠損が継続）、本質的な解決には至りませんでした。
    - そのため、出力を確実に取得するための本来の設定である `Logs: true` を維持した上で、ハングと出力欠損の根本原因に対して別の方法（STDINの同期処理の改善や `CloseWrite` の適切な呼び出し、アタッチ完了後のコンテナ起動制御）で対処することとしました。
2. **データの欠落防止**: コンテナの実行が非常に速い場合、`ContainerStart` の後にアタッチを試みると、最初の出力（stdout/stderr）が既に生成されてしまい、キャプチャできない可能性があります。
3. **副作用の抑制**: cderunは原則として実行ごとに新しいコンテナを作成・削除するため、`Logs: true` を指定しても重複した出力が発生することはなく、安全に全出力を取得できます。

今後、同様の不具合対策として安易に `Logs: false` へ変更し、本質的な解決を遠ざけることのないよう、この経緯を記録しています。
