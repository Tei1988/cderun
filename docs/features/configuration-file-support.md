# Feature: Configuration File Support (Completed)

## 概要

cderun自体の動作設定と、各サブコマンド（ツール）の実行設定を分離して管理する。

## 設定ファイル

### ファイル構成

設定は2つのファイルに分離：

1. **`.cderun.yaml`**: cderun自体の動作設定
2. **`.tools.yaml`**: 各サブコマンド（ツール）の実行設定

### サポートされる形式
- YAML形式のみ（`.cderun.yaml`, `.tools.yaml`）

### 検索順序

#### `.cderun.yaml`の検索順序
1. カレントディレクトリ: `./.cderun.yaml`
2. ホームディレクトリ: `~/.config/cderun/.cderun.yaml`
3. システム全体: `/etc/cderun/.cderun.yaml`

#### `.tools.yaml`の検索順序
1. カレントディレクトリ: `./.tools.yaml`
2. ホームディレクトリ: `~/.config/cderun/.tools.yaml`
3. システム全体: `/etc/cderun/.tools.yaml`

最初に見つかった設定ファイルを使用する。複数の設定ファイルはマージしない。

## 設定スキーマ

### `.cderun.yaml` 例
```yaml
runtime: docker                    # コンテナランタイム (docker/podman)
socketPath: /var/run/docker.sock   # ホスト上のランタイムソケットパス
defaults:
  tty: false                       # デフォルトでTTYを有効化
  interactive: false               # デフォルトでインタラクティブモード
  network: bridge                  # デフォルトネットワーク
  remove: true                     # コンテナの自動削除
  mountSocket: false               # ソケットのマウント
  mountSocketPath: /var/run/docker.sock # コンテナ内のソケットマウントパス
  mountCderun: false               # cderunバイナリのマウント
  dryRun: false                    # ドライランモードのデフォルト
  dryRunFormat: yaml               # ドライランの出力形式
  # ネットワーク・セキュリティ・リソース等のデフォルト
  network: bridge
  ports: ["8080:80"]
  user: "1000:1000"
  memory: "1g"
  cpus: 1.5
logging:
  level: info                      # ログレベル
  file: ./cderun.log               # ログファイルパス
  format: text                     # ログフォーマット (text/json)
  timestamp: true                  # タイムスタンプの有無
  tee: false                       # stderrとファイルの両方に出力
```

### `.tools.yaml` 例
```yaml
node:
  image: node:20-alpine
  tty: true
  interactive: true
  network: host
  ports:
    - "3000:3000"
  volumes:
    - .:/workspace
    - ~/.npm:/root/.npm
  env:
    - NODE_ENV=development
  workdir: /workspace
  remove: true
  mountCderun: true
  privileged: false
  memory: "512m"
  
python:
  image: python:3.11-slim
  tty: true
  interactive: true
  env:
    - PYTHONUNBUFFERED=1
  volumes:
    - .:/app
    - ~/.cache/pip:/root/.cache/pip
  workdir: /app
  
docker:
  image: docker:latest
  volumes:
    - /var/run/docker.sock:/var/run/docker.sock
  network: host
```

## 設定オプション詳細

### `.cderun.yaml` （cderun自体の設定）

#### トップレベル
- `runtime` (string): 使用するコンテナランタイム
  - 値: `docker` | `podman`
  - デフォルト: `docker`
  
- `socketPath` (string): ホスト上のランタイムソケットの絶対パス
  - 例: `/var/run/docker.sock`, `/run/podman/podman.sock`
  - デフォルト: 自動検出

#### `defaults` サブセクション
cderunコマンドのデフォルト動作を定義。コマンドライン引数で上書き可能。

- `tty` (bool): デフォルトでTTYを割り当てる
- `interactive` (bool): デフォルトでSTDINを開いたままにする
- `network` (string): デフォルトのネットワーク設定
- `remove` (bool): コンテナ終了後に自動削除
- `mountSocket` (bool): ホストのランタイムソケットをマウント
- `mountSocketPath` (string): コンテナ内のソケットマウントパス
- `mountCderun` (bool): cderunバイナリをマウント
- `dryRun` (bool): ドライランモードのデフォルト値
- `dryRunFormat` (string): ドライランの出力形式 (`yaml` | `json` | `simple`)
- `ports` ([]string): ポートマッピング
- `publishAll` (bool): 全ポート公開
- `expose` ([]string): ポート露出
- `hostname` (string): ホスト名
- `dns` ([]string): DNSサーバ
- `addHosts` ([]string): ホストマップ
- `user` (string): 実行ユーザー
- `privileged` (bool): 特権モード
- `capAdd` ([]string): ケーパビリティ追加
- `capDrop` ([]string): ケーパビリティ削除
- `entrypoint` ([]string): エントリーポイント
- `pull` (string): プルポリシー (`always` | `missing` | `never`)
- `memory` (string): メモリ制限
- `cpus` (float64): CPU制限
- `tmpfs` ([]string): tmpfsマウント
- `devices` ([]string): デバイス追加

#### `logging` サブセクション
ログ出力に関する設定。

- `level` (string): ログレベル (`error` | `warn` | `info` | `debug` | `trace`)
- `file` (string): ログ出力先のファイルパス
- `format` (string): ログの出力形式 (`text` | `json`)
- `timestamp` (bool): タイムスタンプを含めるかどうか
- `tee` (bool): 標準エラー出力とファイルの両方に出力するかどうか

### `.tools.yaml` （サブコマンドの設定）

各ツール名をキーとして、そのツールの実行設定を定義。
cderunのコマンドライン引数で指定できる全てのオプションを設定可能。

#### 共通オプション
- `image` (string, 必須): 使用するコンテナイメージ
- `tty` (bool): TTYを割り当てる（`--tty`フラグに相当）
- `interactive` (bool): STDINを開く（`--interactive`フラグに相当）
- `network` (string): ネットワーク設定（`--network`フラグに相当）
- `remove` (bool): コンテナの自動削除
- `volumes` ([]string): ボリュームマウント
  - 形式: `<host-path>:<container-path>[:<options>]`
  - 例: `.:/workspace`, `~/.npm:/root/.npm:ro`
- `env` ([]string): 環境変数
  - 形式: `KEY=VALUE`
  - 例: `NODE_ENV=development`
- `workdir` (string): コンテナ内の作業ディレクトリ
- `mountSocket` (bool): ホストのランタイムソケットをマウント
- `mountSocketPath` (string): コンテナ内のソケットマウントパス
- `mountCderun` (bool): cderunバイナリをマウント
- `dryRun` (bool): ドライランモード
- `dryRunFormat` (string): ドライラン形式
- `ports` ([]string): ポートマッピング
- `publishAll` (bool): 全ポート公開
- `expose` ([]string): ポート露出
- `hostname` (string): ホスト名
- `dns` ([]string): DNSサーバ
- `addHosts` ([]string): ホストマップ
- `user` (string): 実行ユーザー
- `privileged` (bool): 特権モード
- `capAdd` ([]string): ケーパビリティ追加
- `capDrop` ([]string): ケーパビリティ削除
- `entrypoint` ([]string): エントリーポイント
- `pull` (string): プルポリシー (`always` | `missing` | `never`)
- `memory` (string): メモリ制限
- `cpus` (float64): CPU制限
- `tmpfs` ([]string): tmpfsマウント
- `devices` ([]string): デバイス追加

## 優先順位

設定の優先順位（高い順）については、[引数・設定優先順位](./argument-priority-logic.md)を参照してください。

1. **CDERUN内部オーバーライド**: `--cderun-tty` 等
2. **コマンドライン引数**: `--tty`, `--network` 等
3. **環境変数**: `CDERUN_TTY` 等
4. **ツール固有設定**: `.tools.yaml` 内の設定
5. **cderunデフォルト設定**: `.cderun.yaml` の `defaults` または `logging`
6. **ハードコード値**: 内部デフォルト値

### 例

`.cderun.yaml`:
```yaml
defaults:
  tty: false        # cderunのデフォルト
  network: bridge
```

`.tools.yaml`:
```yaml
node:
  tty: true         # nodeツールの設定（cderunデフォルトを上書き）
  network: host
```

実行例：
```bash
# tty=true, network=host (ツール設定を使用)
cderun node app.js

# tty=false, network=host (コマンドライン引数が最優先)
cderun --tty=false node app.js

# tty=true, network=mynet (コマンドライン引数が最優先)
cderun --network mynet node app.js
```
