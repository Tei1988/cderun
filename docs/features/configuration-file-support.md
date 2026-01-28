# Feature: Configuration File Support

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
2. ホームディレクトリ: `~/.config/cderun/config.yaml`
3. システム全体: `/etc/cderun/config.yaml`

#### `.tools.yaml`の検索順序
1. カレントディレクトリ: `./.tools.yaml`
2. ホームディレクトリ: `~/.config/cderun/tools.yaml`
3. システム全体: `/etc/cderun/tools.yaml`

最初に見つかった設定ファイルを使用する。複数の設定ファイルはマージしない。

## 設定構造

設定は2つのファイルに分離：

1. **`.cderun.yaml`**: cderun自体の動作設定
2. **`.tools.yaml`**: 各サブコマンドの実行設定

## 設定スキーマ

### `.cderun.yaml` 例
```yaml
runtime: docker                    # コンテナランタイム (docker/podman)
runtimePath: /usr/local/bin/docker # ランタイムバイナリのパス
defaults:
  tty: false                       # デフォルトでTTYを有効化
  interactive: false               # デフォルトでインタラクティブモード
  network: bridge                  # デフォルトネットワーク
  remove: true                     # コンテナの自動削除
  syncWorkdir: true                # ワーキングディレクトリの同期
```

### `.tools.yaml` 例
```yaml
node:
  image: node:20-alpine
  tty: true
  interactive: true
  network: host
  volumes:
    - .:/workspace
    - ~/.npm:/root/.npm
  env:
    - NODE_ENV=development
  workdir: /workspace
  remove: true
  
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
  
- `runtimePath` (string): ランタイムバイナリの絶対パス
  - 例: `/usr/local/bin/docker`, `/opt/podman/bin/podman`
  - デフォルト: PATHから自動検出

#### `defaults` サブセクション
cderunコマンドのデフォルト動作を定義。コマンドライン引数で上書き可能。

- `tty` (bool): デフォルトでTTYを割り当てる
- `interactive` (bool): デフォルトでSTDINを開いたままにする
- `network` (string): デフォルトのネットワーク設定
- `remove` (bool): コンテナ終了後に自動削除
- `syncWorkdir` (bool): ホストのカレントディレクトリをコンテナ内で再現

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

## 優先順位

設定の優先順位（高い順）：

1. **コマンドライン引数**: `cderun --tty --network host node app.js`
2. **ツール固有設定**: `tools.node.tty`
3. **cderunデフォルト設定**: `cderun.defaults.tty`
4. **ハードコードされたデフォルト値**: プログラム内のデフォルト

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
