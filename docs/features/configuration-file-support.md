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

#### マージのルール

- 検索された順序（優先順位の高い順）で設定ファイルの内容が読み込まれ、マージされます。
- **リスト型の設定（`mounts`, `env`, `ports`, `devices` など）について**:
  これらの設定は、**「上書き（完全置き換え）」**となります。優先順位の高いファイルに定義があれば、低いファイルの内容はすべて無視されます。マージ（追加）はされません。

## 設定スキーマ

設定ファイルで使用するキー名は、コマンドラインフラグに対応した**キャメルケース（camelCase）**です。

### `.cderun.yaml` (Global Settings)

#### トップレベル

- `runtime` (string): 使用するコンテナランタイム (`docker` | `podman`)
- `socketPath` (string): ホスト上のランタイムソケットの絶対パス
- `defaults` (object): cderunコマンドのデフォルト動作（下記参照）
- `logging` (object): ログ出力設定
  - `level`: `error` | `warn` | `info` | `debug` | `trace`
  - `format`: `text` | `json`
  - `timestamp`: bool

#### `defaults` ブロックでサポートされるフィールド

- `tty`, `interactive`, `remove`, `strictEnv` (bool)
- `network`, `workdir`, `hostname`, `user`, `pull`, `memory`, `hangTimeout` (string)
- `cpus` (float64)
- `mountCderun`, `mountAllTools`, `mountSocket`, `privileged`, `publishAll` (bool)
- `mountCderunPath`, `mountSocketPath` (ConfigPath object/string)
- `mountTools`, `ports`, `expose`, `dns`, `addHosts`, `capAdd`, `capDrop`, `entrypoint`, `env` ([]string)
- `dryRun`, `diagnosis` (bool)
- `dryRunFormat`, `diagnosisFormat` (string)
- `mounts` ([]MountConfig)
- `devices` ([]DeviceConfig)

### `.tools.yaml` (Tool Mappings)

各ツール名をキーとして、そのツールの実行設定を定義します。`defaults` ブロックでサポートされているすべてのフィールドに加え、以下のフィールドが必須です。

- `image` (string, 必須): 使用するコンテナイメージ

また、以下のログ設定をツールごとに上書き可能です。

- `logLevel`, `logFormat` (string)
- `logTimestamp` (bool)

## 設定オプション詳細

### マウント設定 (`mounts`)

`mounts` 配列の各要素は以下のフィールドを持ちます。

- `type`: `bind` | `volume` | `tmpfs`
- `source`: ホスト側のパス（bindの場合）
- `target` (必須): コンテナ内のパス
- `readOnly`: bool

### デバイス設定 (`devices`)

`devices` 配列の各要素は、文字列形式 (`<host-path>:<container-path>[:<permissions>]`) またはオブジェクト形式で記述可能です。
