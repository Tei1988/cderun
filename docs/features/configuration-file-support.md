# 機能仕様：設定ファイルサポート (完了)

## 概要

cderun自体の動作設定と、各サブコマンド（ツール）の実行設定を分離して管理する。

## 設定ファイル

### ファイル構成

設定は2つのファイルに分離：

1. **`.cderun.yaml`**: cderun自体の動作設定
2. **`.tools.yaml`**: 各サブコマンド（ツール）の実行設定

### サポートされる形式

- YAML形式のみがサポートされます。
- 厳密なデコード: 未知のフィールドやタイポが含まれている場合、エラーとなります（`KnownFields`の有効化）。
- **標準ファイル名**: `.cderun.yaml`, `.tools.yaml`

### 設定不可能なオプション

以下のモード設定は、セキュリティおよび混乱回避のため設定ファイル（YAML）ではサポートされておらず、コマンドライン引数または環境変数でのみ指定可能です。

- **ドライランモード** (`--dry-run`, `--dry-run-format`)
- **診断モード** (`--diagnosis`, `--diagnosis-format`)

### 明示的な設定ファイルの指定

標準の検索・マージ挙動をスキップし、特定のファイルを明示的に指定して実行できます。

#### 指定方法

以下のフラグまたは環境変数を使用して指定します：

| 対象 | フラグ (P2) | 内部オーバーライド (P1) | 環境変数 (P3) |
| :--- | :--- | :--- | :--- |
| **cderun設定** | `--config <path>` | `--cderun-config <path>` | `CDERUN_CONFIG` |
| **ツール設定** | `--tool-config <path>` | `--cderun-tool-config <path>` | `CDERUN_TOOL_CONFIG` |

#### 挙動

- これらの指定がある場合、通常の階層的な検索とマージは**スキップ**され、指定されたファイルのみが読み込まれます。
- 指定されたファイルが存在しない場合、cderunはエラーで終了します。
- パスには `~` または `~/` を使用してホームディレクトリを指定できます（例: `--config ~/.cderun.yaml`）。
- 指定されたファイル内の相対パス（`mounts`の`source`など）は、そのファイルが置かれているディレクトリを基準に解決されます。

### 検索とマージ

cderunは、柔軟な設定管理のため、複数の場所から設定ファイルを検索し、それらを階層的にマージします。

#### 検索順序と優先順位

設定ファイルは以下の順序で検索され、先に見つかった（優先順位が高い）ファイルの設定が、後のファイルの設定を上書きします。

1. **プロジェクト設定（親ディレクトリへの探索）**:
   - カレントディレクトリから始まり、ルートディレクトリ (`/`) に向かって親ディレクトリを遡りながら `.cderun.yaml` / `.tools.yaml` を探します。
   - 例: `./.cderun.yaml` が `../.cderun.yaml` より優先されます。

2. **ユーザー設定**:
   - `~/.config/cderun/.cderun.yaml`
   - `~/.config/cderun/.tools.yaml`

3. **システム全体設定**:
   - `/etc/cderun/.cderun.yaml`
   - `/etc/cderun/.tools.yaml`

4. **ネスト実行時の注入設定**:
   - `/run/cderun/.cderun.yaml`
   - `/run/cderun/.tools.yaml`
   - ※この設定はネスト実行（`--mount-cderun`）時に動的に生成・マウントされます。

#### マージのルール

- 見つかったすべての設定ファイルの内容がマージされます。
- 設定値が重複した場合、上記の検索順序でより優先度の高いファイルの値が採用されます。
- **リスト型の設定について**: `mounts`, `env`, `ports`, `devices` などのリスト形式の設定は、**「上書き（完全置き換え）」**となります。マージ（追加）はされません。

### 値の解決 (Value Resolution)

設定ファイル内の値は、cderunによって解釈・実行される前に、いくつかの変換プロセスを経ます（cderun式やチルダ展開など）。

詳細は [値の解決 (Value Resolution)](./value-resolution.md) を参照してください。

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
  strictEnv: false                 # 環境変数の厳密モード
  mountSocket: false               # ソケットのマウント
  mountSocketPath: /var/run/docker.sock # コンテナ内のソケットマウントパス
  mountCderun: false               # cderunバイナリのマウント
  mountCderunPath: ""              # cderunバイナリのホスト側パス
  mountTools: ["node", "python"]   # コンテナ内にマウントするツールのリスト
  mountAllTools: false             # 定義された全ツールのマウント
  # ネットワーク・セキュリティ・リソース等のデフォルト
  ports: ["8080:80"]
  user: "1000:1000"
  memory: "1g"
  cpus: 1.5
  mounts:
    - type: tmpfs
      target: /tmp
  devices:
    - /dev/fuse
logging:
  level: warn                      # ログレベル
  format: text                     # ログフォーマット (text/json)
  timestamp: true                  # タイムスタンプの有無
```

### `.tools.yaml` 例

```yaml
node:
  image: node:20-alpine
  strictEnv: true
  tty: true
  interactive: true
  network: host
  ports:
    - "3000:3000"
  mounts:
    - type: bind
      source: .
      target: /workspace
    - type: bind
      source: ~/.npm
      target: /root/.npm
  env:
    - NODE_ENV=development
  workdir: /workspace
  remove: true
  mountCderun: true
  privileged: false
  memory: "512m"
  devices:
    - /dev/fuse                    # コンテナに追加するデバイス

python:
  image: python:3.11-slim
  tty: true
  interactive: true
  env:
    - PYTHONUNBUFFERED=1
  mounts:
    - type: bind
      source: .
      target: /app
    - type: bind
      source: ~/.cache/pip
      target: /root/.cache/pip
  workdir: /app

docker:
  image: docker:latest
  mounts:
    - type: bind
      source: /var/run/docker.sock
      target: /var/run/docker.sock
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
- `strictEnv` (bool): 指定された環境変数がホストに存在しない場合にエラーとする
- `workdir` (string): デフォルトの作業ディレクトリ
- `mountSocket` (bool): ホストのランタイムソケットをマウント
- `mountSocketPath` (string): コンテナ内のソケットマウントパス
- `mountCderun` (bool): cderunバイナリをマウント
- `mountCderunPath` (string): cderunバイナリのホスト側パス
- `mountTools` ([]string): 指定したツールをコンテナ内にマウント
- `mountAllTools` (bool): 全てのツールをコンテナ内にマウント
- `ports` ([]string): ポートマッピング
- `publishAll` (bool): 全ポート公開
- `expose` ([]string): ポート露出
- `hostname` (string): ホスト名
- `dns` ([]string): DNSサーバ
- `addHosts` ([]string): ホストマッピング。形式: `hostname:ip` (YAMLキー: `addHosts`)。
- `user` (string): 実行ユーザー
- `privileged` (bool): 特権モード
- `capAdd` ([]string): ケーパビリティ追加 (YAMLキー: `capAdd`)
- `capDrop` ([]string): ケーパビリティ削除 (YAMLキー: `capDrop`)
- `entrypoint` ([]string): エントリーポイント
- `command` ([]string): デフォルトのコマンド引数。サブコマンドの後に結合されます。
- `pull` (string): プルポリシー (`always` | `missing` | `never`)
- `memory` (string): メモリ制限
- `cpus` (float64): CPU制限
- `mounts` ([]object): マウント設定
  - `type` (string): `bind` | `volume` | `tmpfs`
  - `source` (string): ホスト側のパス（bindの場合）
  - `target` (string, 必須): コンテナ内のパス
  - `read_only` (bool): 読み取り専用
- `devices` ([]string): デバイス追加。形式: `<host-path>:<container-path>[:<permissions>]` または `<path>`。
- `env` ([]string): 環境変数

#### `logging` サブセクション

ログ出力に関する設定。

- `level` (string): ログレベル (`error` | `warn` | `info` | `debug` | `trace`)
- `format` (string): ログの出力形式 (`text` | `json`)
- `timestamp` (bool): タイムスタンプを含めるかどうか

### `.tools.yaml` （サブコマンドの設定）

各ツール名をキーとして、そのツールの実行設定を定義。
cderunのコマンドライン引数で指定できる全てのオプションを設定可能。

#### 共通オプション

- `image` (string, 必須): 使用するコンテナイメージ
- `addHosts` ([]string): ホストマッピング。形式: `hostname:ip`。
- `devices` ([]string): デバイス追加。形式: `<host-path>:<container-path>[:<permissions>]` または `<path>`。
- `tty` (bool): TTYを割り当てる（`--tty`フラグに相当）
- `interactive` (bool): STDINを開く（`--interactive`フラグに相当）
- `network` (string): ネットワーク設定（`--network`フラグに相当）
- `remove` (bool): コンテナの自動削除
- `strictEnv` (bool): 環境変数の厳密モード
- `mounts` ([]object): マウント設定（上述）
- `env` ([]string): 環境変数
  - 形式: `KEY=VALUE`
  - 例: `NODE_ENV=development`
- `workdir` (string): コンテナ内の作業ディレクトリ
- `mountSocket` (bool): ホストのランタイムソケットをマウント
- `mountSocketPath` (string): コンテナ内のソケットマウントパス
- `mountCderun` (bool): cderunバイナリをマウント
- `mountCderunPath` (string): cderunバイナリのホスト側パス
- `mountTools` ([]string): 指定したツールをコンテナ内にマウント
- `mountAllTools` (bool): 全てのツールをコンテナ内にマウント
- `ports` ([]string): ポートマッピング
- `publishAll` (bool): 全ポート公開
- `expose` ([]string): ポート露出
- `hostname` (string): ホスト名
- `dns` ([]string): DNSサーバ
- `user` (string): 実行ユーザー
- `privileged` (bool): 特権モード
- `capAdd` ([]string): ケーパビリティ追加 (YAMLキー: `capAdd`)
- `capDrop` ([]string): ケーパビリティ削除 (YAMLキー: `capDrop`)
- `entrypoint` ([]string): エントリーポイント
- `command` ([]string): デフォルトのコマンド引数
- `pull` (string): プルポリシー (`always` | `missing` | `never`)
- `memory` (string): メモリ制限
- `cpus` (float64): CPU制限

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
