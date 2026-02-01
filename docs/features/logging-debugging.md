# Feature: Logging and Debugging (Completed)

## 概要

cderunの動作を詳細に確認するためのログ出力とデバッグ機能。

## ログレベル

### レベル定義
- `ERROR`: エラーのみ
- `WARN`: 警告とエラー
- `INFO`: 一般的な情報（デフォルト）
- `DEBUG`: 詳細なデバッグ情報
- `TRACE`: 最も詳細な情報（全ての内部ステップ、引数処理、APIコール等）

### 設定方法

#### コマンドライン
```bash
# 詳細ログ (DEBUG)
$ cderun --verbose --verbose node app.js
$ cderun --log-level debug node app.js

# 最も詳細 (TRACE)
$ cderun --verbose --verbose --verbose node app.js
$ cderun --log-level trace node app.js
```

> **Note**: `-v` shorthand is reserved for `--volume` and cannot be used for `--verbose`.

#### 設定ファイル
```yaml
# .cderun.yaml
logging:
  level: info  # error | warn | info | debug | trace
  file: ./cderun.log
  format: text  # text | json
  timestamp: true
  tee: false    # stderrとファイルの両方に出力 (デフォルト: false)
```

#### 環境変数
```bash
export CDERUN_LOG_LEVEL=debug
export CDERUN_LOG_FILE=/tmp/cderun.log
```

## ログ出力例

### INFO レベル（デフォルト）
```bash
$ cderun node app.js
2024-01-15 10:30:45 [INFO] Running: node app.js
Hello, World!
```

### DEBUG レベル
```bash
$ cderun --log-level debug node app.js
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
$ cderun --log-level trace node app.js
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

## ログファイル

### ファイル出力
```bash
# ファイルのみに出力
$ cderun --log-file /tmp/cderun.log node app.js

# 標準エラー出力とファイルの両方
$ cderun --log-file /tmp/cderun.log --log-tee node app.js
```

## フォーマット

### テキスト形式（デフォルト）
```
2024-01-15 10:30:45 [INFO] Running: node app.js
```

### JSON形式
```bash
$ cderun --log-format json node app.js
{"level":"info","msg":"Running: node app.js","time":"2024-01-15T10:30:45Z"}
```

## デバッグ機能

### 1. ドライラン
実行せずにコンテナ構成を表示します。詳細は[ドライランモード](./dry-run-mode.md)を参照してください。

```bash
$ cderun --dry-run node app.js
```

## 将来の拡張予定 (Planned)

### ログローテーション
設定ファイルでの指定はサポートされていますが、実際のローテーション処理は将来のフェーズで実装予定です。

```yaml
# .cderun.yaml
logging:
  rotation:
    maxSize: 10MB
    maxAge: 7d
    maxBackups: 5
    compress: true
```

### ユーティリティコマンド
- `cderun logs`: 過去の実行ログの表示・検索
- `cderun debug info`: システム環境および設定情報の表示
- `cderun debug diagnose`: 実行環境の診断
