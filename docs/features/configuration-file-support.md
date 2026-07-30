# 機能仕様：設定ファイルサポート

## 概要

cderun自体の動作設定と、各サブコマンド（ツール）の実行設定を分離して管理する。

## 設定ファイル

### ファイル構成

設定は2つのファイルに分離されています。

1. **`.cderun.yaml`**: cderun自体の動作設定（デフォルト値、ランタイム、ログ設定など）
2. **`.tools.yaml`**: 各サブコマンド（ツール）ごとの実行設定（イメージ、マウント、環境変数など）

### サポートされる形式

- YAML形式のみがサポートされます。
- 厳密なデコード: 未知のフィールドやタイポが含まれている場合、エラーとなります（`KnownFields`の有効化）。
- **標準ファイル名**: `.cderun.yaml`, `.tools.yaml`

### 設定不可能なオプション

以下のオプションは、設定ファイルの**読み込み先パスを決める**ためのオプションであり、ファイルを読み込む前にパスが確定している必要があるため P1/P2/P3 のみ有効です。P4/P5（設定ファイル内）に記述すると、デコードエラーが発生します。

- **`config`** (`--config`): cderun設定ファイルのパス（YAML内キー: `config` は不可）
- **`toolConfig`** (`--tool-config`): ツール設定ファイルのパス（YAML内キー: `toolConfig` は不可）

> **Note**: `dryRun`, `dryRunFormat`, `diagnosis`, `diagnosisFormat` は、以前の仕様とは異なり、現在は設定ファイル内でもフルサポートされています。

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

#### マージのルール（リスト型の上書き原則）

- 検索された順序（優先順位の高い順）で設定ファイルの内容が読み込まれ、マージされます。
- **リスト型の設定（`mounts`, `env`, `ports`, `groupAdd`, `devices`, `sensitiveEnv` など）について**:
  これらの設定は、**「上書き（完全置き換え）」**となります。優先順位の高いファイルに定義があれば、低いファイルの内容はすべて無視されます。マージ（追加）はされません。
- **明示的な空リストによるオーバーライド**:
  上位の設定ファイルで明示的に空のリスト（例: `ports: []` や `env: []`）が定義されている場合、下位の設定ファイルで定義された内容は完全にクリアされ、空として扱われます。これは、コレクション全体の上書き原則によるものです。

## 設定スキーマ

設定ファイルで使用するキー名は、原則としてコマンドラインフラグに対応した**キャメルケース（camelCase）**です。

> **例外**: `mounts` 配列内の各要素（`MountConfig`）のフィールド名のみ、一部でスネークケース（snake_case）が使用されています（例: `read_only`, `optional`）。これは、一般的なコンテナ設定の慣習や互換性を考慮したものです。
>
> **注意（マウント指定の形式表記の違い）**:
>
> - **YAML設定（`.tools.yaml`, `.cderun.yaml`）内**: `read_only` や `optional` のようにスネークケースのキー名を使用します。
> - **CLI（`--mount`フラグ）内**: `readonly` や `optional` のようなキー（例: `type=bind,source=...,target=...,readonly,optional`）を使用します。CLIの引数解析やドライラン表示の出力データでは `readonly` が使用されます。
> - CLIフラグの `--mount` 構文の文字列（`type=bind,source=...`）そのものを直接 `.tools.yaml` などの `mounts` 配列要素にコピペして使用することはできません。必ず後述の構造化されたYAMLマップ形式で定義してください。

### `.cderun.yaml` (Global Settings)

#### トップレベル

- `runtime` (string): 使用するコンテナランタイム (`docker` | `podman` | `containerd`)
- `socketPath` (string): ホスト上のランタイムソケットの絶対パス
- `defaults` (object): cderunコマンドのデフォルト動作（下記参照）
- `logging` (object): ログ出力設定
  - `level`: `error` | `warn` | `info` | `debug` | `trace`
  - `format`: `text` | `json`
  - `timestamp`: bool

#### `defaults` ブロックでサポートされるフィールド

- `tty`, `interactive`, `remove`, `strictEnv` (bool)
- `network`, `workdir`, `hostname`, `user`, `pull`, `pullBackoffBase`, `memory`, `hangTimeout` (string)
- `cpus` (float64)
- `pullMaxRetries` (int)
- `mountCderun`, `mountAllTools`, `mountSocket`, `privileged`, `publishAll` (bool)
- `mountCderunPath`, `mountSocketPath` (string)
- `mountTools`, `ports`, `expose`, `dns`, `addHosts`, `groupAdd`, `capAdd`, `capDrop`, `entrypoint`, `env`, `sensitiveEnv` ([]string)
- `dryRun`, `diagnosis` (bool)
- `dryRunFormat`, `diagnosisFormat` (string)
- `mounts` ([]MountConfig)
- `devices` (slice of `DeviceConfig` object or string)

```yaml
# .cderun.yaml の設定例
runtime: docker
socketPath: /var/run/docker.sock
defaults:
  tty: true
  interactive: true
  remove: true
  network: bridge
  pull: missing
  pullMaxRetries: 3
  pullBackoffBase: 1s
  hangTimeout: 10s
  sensitiveEnv: null # 指定なし = すべての環境変数を自動マスク
logging:
  level: warn
  format: text
  timestamp: true
```

### `.tools.yaml` (Tool Mappings)

各ツール名をキーとして、そのツールの実行設定を定義します。`defaults` ブロックでサポートされているすべてのフィールドに加え、以下のフィールドが必須です。

- `image` (string, 必須): 使用するコンテナイメージ

また、以下のログ設定をツールごとに上書き可能です。

- `logLevel`, `logFormat` (string)
- `logTimestamp` (bool)

```yaml
# .tools.yaml の設定例
node:
  image: "node:20-alpine"
  workdir: /workspace
  mounts:
    - type: bind
      source: .
      target: /workspace
      read_only: false
    - type: bind
      source: ~/.npmrc
      target: /root/.npmrc
      read_only: true
  env:
    - "NODE_ENV=development"

python:
  image: "python:3.11-slim"
  workdir: /app
  mounts:
    - type: bind
      source: .
      target: /app
  env:
    - "PYTHONUNBUFFERED=1"
```

## 設定オプション詳細

### マウント設定 (`mounts`)

`mounts` 配列の各要素は以下のフィールドを持ちます。

- `type`: `bind` | `volume` | `tmpfs` (デフォルト: `bind`)
- `source`: ホスト側のパス
- `target` (必須): コンテナ内のパス
- `read_only`: bool
- `optional`: bool (`type=bind` の場合、ホスト側のソースが存在しなくてもエラーにせずスキップする)

### デバイス設定 (`devices`)

`devices` 配列の各要素は、オブジェクト形式または文字列形式をサポートしています。

#### オブジェクト形式

YAMLファイルで定義するオブジェクト形式のフィールドは、ドライランの出力表示などのインターフェースと一対一で対応しています。

- `source` (必須): ホスト側のデバイスパス（ドライラン出力の `path_on_host` フィールドにマッピングされます）
- `destination` (必須): コンテナ内のデバイスパス（ドライラン出力の `path_in_container` フィールドにマッピングされます）
- `permissions`: デバイスの権限 (例: `rwm`)。デフォルトは `rwm`（ドライラン出力の `cgroup_permissions` フィールドにマッピングされます）。

#### 文字列形式

`<host-path>:<container-path>[:<permissions>]` の形式で記述します。コロンで区切られた各フィールドは、それぞれオブジェクト形式の `source`、`destination`、および `permissions`（ドライラン出力の `path_on_host`、`path_in_container`、および `cgroup_permissions`）へと解決されます。

許可される権限パターン（cgroup permissions）は `rwm` の組み合わせです（例: `rw` や `r`）。安全性を保つために、正規表現検証が行われます。

```yaml
# デバイス指定の例
devices:
  - source: /dev/fuse
    destination: /dev/fuse
    permissions: rwm
  - "/dev/snd:/dev/snd:rw"
```
