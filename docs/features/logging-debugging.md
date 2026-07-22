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
- `--cderun-hang-timeout`

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
2026-02-28 10:30:45 [INFO] Running: node app.js
Hello, World!
```

### DEBUG レベル

```bash
cderun --log-level debug node app.js
2026-02-28 10:30:45 [DEBUG] Loaded cderun config from: .cderun.yaml
2026-02-28 10:30:45 [DEBUG] Resolved Image: node:20-alpine
2026-02-28 10:30:45 [INFO] Running: node app.js
2026-02-28 10:30:45 [DEBUG] Image: node:20-alpine
2026-02-28 10:30:45 [DEBUG] Runtime: docker
2026-02-28 10:30:45 [DEBUG] Socket: /var/run/docker.sock
Hello, World!
2026-02-28 10:30:46 [DEBUG] Container exited with code: 0
```

#### ContainerConfig 構造体のデバッグログ出力と機密情報のマスキング

`cderun` はコンテナを生成する直前に、構築された `ContainerConfig`（コンテナイメージ名、実行コマンド、エントリーポイント、マウント情報、環境変数リスト、実行ユーザー等）の情報を `DEBUG` レベルで詳細に出力します。

このデバッグ出力では、セキュリティを担保するため、`config.MaskSensitiveEnvList` が適用されます。環境変数に登録されている機密情報（`sensitive-env` フィルタに一致、または未指定時のデフォルト全マスク等）は `[REDACTED]` でマスクされた状態でログ出力されるため、デバッグログ経由での秘密鍵やパスワードなどの機密情報の漏洩を防ぎます。

デバッグログ出力例：

```text
2026-02-28 10:30:45 [DEBUG] ContainerConfig:
  Image:      node:20-alpine
  Command:    [app.js]
  Entrypoint: []
  Mounts:
    - bind /home/user/project -> /app
  Env:        [NODE_ENV=production DB_PASSWORD=[REDACTED] API_TOKEN=[REDACTED]]
  User:       1000:1000
```

### TRACE レベル

```bash
cderun --log-level trace node app.js
2026-02-28 10:30:45 [TRACE] Loading configurations...
2026-02-28 10:30:45 [DEBUG] Loaded cderun config from: .cderun.yaml
2026-02-28 10:30:45 [TRACE] Resolving configurations for tool: node
2026-02-28 10:30:45 [DEBUG] Resolved Image: node:20-alpine
2026-02-28 10:30:45 [INFO] Running: node app.js
...
2026-02-28 10:30:45 [TRACE] Creating container...
2026-02-28 10:30:45 [TRACE] Starting container: <ID>
2026-02-28 10:30:45 [TRACE] Waiting for container: <ID>
...
```

## フォーマット

### テキスト形式（デフォルト）

```text
2026-02-28 10:30:45 [INFO] Running: node app.js
```

### JSON形式

```bash
cderun --log-format json --log-level info node app.js
{"level":"info","msg":"Running: node app.js","time":"2026-02-28T10:30:45Z"}
```

## デバッグ機能

### 1. ドライラン

実行せずにコンテナ構成を表示します。詳細は[ドライランモード](./dry-run-mode.md)を参照してください。

```bash
cderun --dry-run node app.js
```

## 内部実装の注意点

### コンテナ出力のキャプチャ (`Logs: false`)

Dockerランタイムの実装 (`internal/runtime/docker.go`) において、`ContainerAttach` のオプションで `Logs: false` を設定しています。

これは、以下の経緯と理由によります：

1. **不具合修正の経緯**:
    - 以前は `Logs: true` を使用していましたが、コンテナがまだ開始されていない状態でアタッチすると、一部のランタイム挙動によりログのダンプが完了した時点でストリームがEOFを返してしまい、出力キャプチャ用ゴルーチンが早期に終了して接続が閉じられるという問題がありました。これにより、その後に開始されたコンテナの出力がキャプチャできず、また標準入力も途切れてしまう現象が発生していました。
2. **同期処理による保証**: cderunでは、`AttachContainer` 内でアタッチが完了しゴルーチンが開始されたことを確認してから `StartContainer` を呼び出す同期メカニズム（`attachReady` チャネル）を導入しています。
3. **安全な全出力取得**: この同期メカニズムにより、コンテナの開始直後からの出力を確実にキャプチャできるため、`Logs: false` を使用してもデータの欠落は発生しません。

過去の記録では `Logs: false` でも改善しなかったとありましたが、同期メカニズムと適切に組み合わせることで、ハングおよび出力欠損の根本的な原因を解消できることが確認されました。

### 終了時のハング対策 (`CDERUN_HANG_TIMEOUT`)

I/Oが完了した後、コンテナが期待通りに終了しない（ハングする）現象が発生することがあります。cderunはこれを防ぐため、以下のタイムアウト制御を実装しています。

- **タイムアウト設定**: 環境変数 `CDERUN_HANG_TIMEOUT` で制御可能です。
- **挙動**: I/O完了後、指定された時間（デフォルト: 10s）待機してもコンテナが終了しない場合、cderunはコンテナの終了を待たずに復帰するか、必要に応じて強制終了（SIGKILL）を試みます。
  - **選択ルール**: `CDERUN_HANG_TIMEOUT` 経過時、コンテナが既に終了状態（exit status取得可能）であれば即座に復帰（return）し、プロセスが依然として実行中であれば強制終了（SIGKILL）を実行します。
- **ログ**: このタイムアウトによる終了待機や強制終了の詳細は、`DEBUG` レベルで出力されます。

## ログ出力の方針

cderunでは、ユーザーの利便性とデバッグの容易さを両立するため、以下のログ出力方針を採用しています。

### 実行完了後のログレベル

コンテナのメインプロセスが終了した後のクリーンアップや後処理に関連するログ（コンテナの強制終了、アタッチエラーなど）は、原則として `DEBUG` レベル以下で出力します。

これは、以下の理由によります：

- **Dockerの既知の挙動への対応**: Docker 29以降などで、IO完了後にコンテナが即座に終了せず、強制終了が必要になる場合があります。これらは `WARN` レベルで出力すると、正常な実行時にも頻繁に警告が表示され、ユーザーのノイズとなる可能性があります。
- **正常実行の優先**: コンテナの実行自体が成功している場合、その後の事後処理における軽微な問題や環境起因の挙動は、詳細な調査が必要な場合（DEBUG指定時）にのみ表示されるべきという考えに基づいています。

重要なエラー（コンテナの起動失敗、実行中の致命的なエラーなど）については、引き続き `ERROR` または `WARN` レベルで出力されます。
