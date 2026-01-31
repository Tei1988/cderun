# Feature: Logging and Debugging (Phase 4予定)

## 概要

cderunの動作を詳細に確認するためのログ出力とデバッグ機能。

## ログレベル

### レベル定義
- `ERROR`: エラーのみ
- `WARN`: 警告とエラー
- `INFO`: 一般的な情報（デフォルト）
- `DEBUG`: 詳細なデバッグ情報
- `TRACE`: 最も詳細な情報（全てのコマンド実行等）

### 設定方法

#### コマンドライン
```bash
# 詳細ログ
$ cderun --verbose node app.js

# 非常に詳細なログ
$ cderun --verbose --verbose node app.js
$ cderun --log-level debug node app.js

# 最も詳細
$ cderun --verbose --verbose --verbose node app.js
$ cderun --log-level trace node app.js
```

> **Note**: `-v` shorthand is reserved for `--volume` and cannot be used for `--verbose`.

#### 設定ファイル
```yaml
# .cderun.yaml
logging:
  level: info  # error | warn | info | debug | trace
  file: ~/.cderun/logs/cderun.log
  format: text  # text | json
  timestamp: true
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
[INFO] Using configuration: /home/user/project/.cderun.yaml
[INFO] Running: node app.js
Hello, World!
```

### DEBUG レベル
```bash
$ cderun --log-level debug node app.js
[DEBUG] Loading configuration from: /home/user/project/.cderun.yaml
[DEBUG] Resolved tool: node
[DEBUG] Image: node:20-alpine
[DEBUG] Working directory: /home/user/project
[DEBUG] Volumes: [/home/user/project:/home/user/project]
[DEBUG] Environment: [NODE_ENV=development]
[INFO] Running: node app.js
[DEBUG] Executing: docker run --rm -t -i -v /home/user/project:/home/user/project -e NODE_ENV=development -w /home/user/project node:20-alpine node app.js
Hello, World!
[DEBUG] Exit code: 0
```

### TRACE レベル
```bash
$ cderun --log-level trace node app.js
[TRACE] Args: [cderun --log-level trace node app.js]
[TRACE] Preprocessing args...
[TRACE] Parsed flags: {tty:false, interactive:false, network:bridge}
[DEBUG] Loading configuration from: /home/user/project/.cderun.yaml
[TRACE] Config file content: {...}
[TRACE] Merging configurations...
[DEBUG] Resolved tool: node
[DEBUG] Image: node:20-alpine
[TRACE] Building docker command...
[TRACE] Adding flag: --rm
[TRACE] Adding volume: /home/user/project:/home/user/project
[DEBUG] Executing: docker run --rm -t -i -v /home/user/project:/home/user/project node:20-alpine node app.js
[TRACE] STDOUT: Hello, World!
[DEBUG] Exit code: 0
```

## ログファイル

### ファイル出力
```bash
# ファイルに出力
$ cderun --log-file /tmp/cderun.log node app.js

# 標準出力とファイルの両方
$ cderun --log-file /tmp/cderun.log --log-tee node app.js
```

### ローテーション
```yaml
# .cderun.yaml
logging:
  file: ~/.cderun/logs/cderun.log
  rotation:
    maxSize: 10MB
    maxAge: 7d
    maxBackups: 5
    compress: true
```

ログファイル例:
```
~/.cderun/logs/
├── cderun.log          # 現在のログ
├── cderun.log.1        # 1つ前
├── cderun.log.2.gz     # 圧縮済み
└── cderun.log.3.gz
```

## フォーマット

### テキスト形式（デフォルト）
```
2024-01-15 10:30:45 [INFO] Running: node app.js
2024-01-15 10:30:46 [DEBUG] Exit code: 0
```

### JSON形式
```bash
$ cderun --log-format json node app.js
```

```json
{"time":"2024-01-15T10:30:45Z","level":"info","msg":"Running: node app.js"}
{"time":"2024-01-15T10:30:46Z","level":"debug","msg":"Exit code: 0"}
```

### カスタムフォーマット
```yaml
# .cderun.yaml
logging:
  format: custom
  template: "[{{.Level}}] {{.Time}} - {{.Message}}"
```

## デバッグ機能

### 1. ドライラン
実行せずにコマンドを表示:
```bash
$ cderun --dry-run node app.js
image: node:latest
command:
  - node
args:
  - app.js
tty: false
interactive: false
remove: true
network: bridge
volumes: []
env: []
workdir: ""
user: ""
```

### 2. 設定のダンプ
```bash
$ cderun config dump
# .cderun.yaml
runtime: docker
defaults:
  tty: false
  interactive: false

# .tools.yaml
node:
  image: node:20-alpine
  volumes:
    - .:/workspace
```

### 3. 実行トレース
```bash
$ cderun --trace node app.js
[TRACE] 10:30:45.123 | parseArgs() started
[TRACE] 10:30:45.125 | loadConfig() started
[TRACE] 10:30:45.130 | loadConfig() completed (5ms)
[TRACE] 10:30:45.131 | resolveImage() started
[TRACE] 10:30:45.135 | resolveImage() completed (4ms)
[TRACE] 10:30:45.136 | buildContainerConfig() started
[TRACE] 10:30:45.140 | buildContainerConfig() completed (4ms)
[TRACE] 10:30:45.141 | Execute() started
Hello, World!
[TRACE] 10:30:46.200 | Execute() completed (1059ms)
```

### 4. エラーの詳細表示
```bash
$ cderun node app.js
Error: Failed to start container

# 詳細モード
$ cderun --verbose node app.js
Error: Failed to start container
Caused by: docker: Error response from daemon: pull access denied for node
Stack trace:
  at executeCommand (runtime.go:45)
  at runContainer (docker.go:123)
  at main (main.go:20)
```

## パフォーマンス分析

### タイミング情報
```bash
$ cderun --timing node app.js
Configuration load: 5ms
Image resolution: 4ms
Command build: 4ms
Container start: 250ms
Execution: 1000ms
Total: 1263ms
```

### プロファイリング
```bash
$ cderun --profile cpu node app.js
CPU profile written to: /tmp/cderun-cpu.prof

$ cderun --profile mem node app.js
Memory profile written to: /tmp/cderun-mem.prof
```

## 環境情報の出力

### システム情報
```bash
$ cderun debug info
cderun version: 0.1.0
Go version: go1.21.5
OS/Arch: linux/amd64
Runtime: docker 24.0.7
Configuration: /home/user/project/.cderun.yaml
Log level: info
```

### 診断情報
```bash
$ cderun debug diagnose
Checking cderun installation...
✓ cderun binary found
✓ Configuration file found
✓ Docker runtime available
✓ Docker daemon running
✓ Network connectivity OK

Checking tool configurations...
✓ node: image available (node:20-alpine)
✗ python: image not found (python:3.11-slim)
  Run: cderun image pull python

Summary: 1 issue found
```

## ログの検索とフィルタリング

### ログの検索
```bash
# 特定のツールのログのみ
$ cderun logs --tool node

# エラーのみ
$ cderun logs --level error

# 時間範囲指定
$ cderun logs --since "1 hour ago"
$ cderun logs --since "2024-01-15 10:00" --until "2024-01-15 11:00"
```

### ログのフォロー
```bash
# リアルタイムでログを表示
$ cderun logs --follow
```
