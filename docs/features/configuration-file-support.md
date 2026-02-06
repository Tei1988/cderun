# Feature: Configuration File Support (Completed)

## 概要

cderun自体の動作設定と、各サブコマンド（ツール）の実行設定を分離して管理する。

## 設定ファイル

### ファイル構成

設定は2つのファイルに分離：

1. **`.cderun.yaml`**: cderun自体の動作設定
2. **`.tools.yaml`**: 各サブコマンド（ツール）の実行設定

### サポートされる形式
- YAML形式のみがサポートされます。
- **標準ファイル名**: `.cderun.yaml`, `.tools.yaml`

### 検索とマージ

cderunは、柔軟な設定管理のため、複数の場所から設定ファイルを検索し、それらを階層的にマージします。

#### 検索順序と優先順位
設定ファイルは以下の順序で検索され、先に見つかった（優先順位が高い）ファイルの設定が、後のファイルの設定を上書きします。

1.  **プロジェクト設定（親ディレクトリへの探索）**:
  *   カレントディレクトリから始まり、ルートディレクトリ (`/`) に向かって親ディレクトリを遡りながら `.cderun.yaml` / `.tools.yaml` を探します。
  *   例: `./.cderun.yaml` が `../.cderun.yaml` より優先されます。

2.  **ユーザー設定**:
  *   `~/.config/cderun/.cderun.yaml`
  *   `~/.config/cderun/.tools.yaml`

3.  **システム全体設定**:
  *   `/etc/cderun/.cderun.yaml`
  *   `/etc/cderun/.tools.yaml`

#### マージのルール
- 見つかったすべての設定ファイルの内容がマージされます。
- 設定値が重複した場合、上記の検索順序でより優先度の高いファイルの値が採用されます。
  - 例えば、`./.cderun.yaml` に `runtime: podman` があり、`~/.config/cderun/.cderun.yaml` に `runtime: docker` がある場合、`podman` が使用されます。

### 値の解決 (Value Resolution)

設定ファイル内の値は、cderunによって解釈・実行される前に、いくつかの変換プロセスを経ます。これにより、設定ファイルの柔軟性と再利用性が向上します。

#### cderun Expressions (cderun式)
設定ファイル内の任意の文字列値で、`{{...}}` という構文を使って動的に値を埋め込むことができます。

##### 種類

1.  **マジックワード (Magic Words)**
  cderunが特別な意味を持つと定義しているキーワードです。

  | ワード | 説明 |
  | :--- | :--- |
  | `{{HOME}}` | 実行ユーザーのホームディレクトリに置換されます。 |
  | `{{PWD}}`  | `cderun` コマンドを実行したカレントワーキングディレクトリに置換されます。 |

2.  **ディレクティブ (Directives)**
  `:` を含む形式で、特定のデータソースから値を読み込むよう指示します。

  | ディレクティブ | 説明 |
  | :--- | :--- |
  | `{{file:<ファイル名>}}` | 指定された `<ファイル名>` の内容を読み込み、その値で置換します。ファイルは設定ファイルと同じルール（親ディレクトリへの探索）で検索され、見つかったファイル内容の前後の空白・改行は除去されます。|

##### 使用例
```yaml
# .tools.yaml
golang:
  # .go-version の内容を読み込み、image タグに設定
  image: "golang:{{file:.go-version}}"

node:
  mounts:
    # ホームディレクトリの .npmrc をマウント
    - type: bind
      source: "{{HOME}}/.npmrc"
      target: /root/.npmrc
    # cderun 実行ディレクトリの src をマウント
    - type: bind
      source: "{{PWD}}/src"
      target: /app
```

#### チルダ展開 (Tilde Expansion)
シェルの挙動と一貫性を持たせるため、`~` または `~/` で始まるパスは、実行ユーザーのホームディレクトリに展開されます。

**例:**
```yaml
mounts:
  - type: bind
    source: ~/.kube
    target: /root/.kube
```
は、`source: /home/user/.kube` のように解決されます。

#### 相対パスの解決
`mounts` の `source` や `devices` のホストパスなど、設定値に `./` や `../` で始まる相対パスが記述されている場合、そのパスは**設定ファイルが置かれているディレクトリ**を基準に絶対パスへ変換されます。

**例:** `/home/user/project/.tools.yaml` 内の以下の記述
```yaml
mounts:
  - type: bind
    source: ./src
    target: /app
```
は、`source: /home/user/project/src` として解決されます。

#### 解決の順序とルール
値の解決は、以下の順序で実行されます。

1.  **cderun Expressions の展開**: `{{...}}` 式が評価されます。
2.  **チルダ展開**: `~` がホームディレクトリに展開されます。
3.  **相対パスの解決**: 上記の結果、値が相対パス (`./` or `../`) になった場合、その設定ファイルが置かれている場所を基準に解決されます。
4.  **パスの正規化**: 最終的なパス文字列は、標準的なパス解決ルールに従って正規化されます (例: `/path/to/work/../src` は `/path/to/src` になります)。

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
  dryRun: false                    # ドライランモードのデフォルト
  dryRunFormat: yaml               # ドライランの出力形式
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
- `mounts` ([]object): マウント設定
  - `type` (string): `bind` | `volume` | `tmpfs`
  - `source` (string): ホスト側のパス（bindの場合）
  - `target` (string, 必須): コンテナ内のパス
  - `read_only` (bool): 読み取り専用
- `devices` ([]string): デバイス追加
  - 形式: `<host-path>:<container-path>[:<permissions>]` または `<path>`

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
- `strictEnv` (bool): 環境変数の厳密モード
- `mounts` ([]object): マウント設定（上述）
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
- `devices` ([]string): デバイス追加（上述）

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
