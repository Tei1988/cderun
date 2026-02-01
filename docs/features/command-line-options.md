# コマンドラインオプション

## 概要

`cderun`のすべてのコマンドラインオプションのリファレンス。

## 基本構文

```bash
cderun [cderun-flags] <subcommand> [passthrough-args]
```

- **[cderun-flags]**: `cderun` の動作を制御するフラグ。
  - **標準フラグ (P2)**: `--tty` や `--env` など。サブコマンドの**前**に置く必要があります。
- **\<subcommand\>**: 最初の非フラグ引数（例: `node`, `python`）。
- **[passthrough-args]**: サブコマンドに渡される引数。`--cderun-` で始まるフラグは `cderun` の優先設定（P1オーバーライド）としてパースされ、それ以外の全ての引数はサブコマンドにそのまま渡されます。

## グローバルオプション

### `--tty`
- **型**: bool
- **デフォルト**: `false`
- **説明**: 疑似TTYを割り当てる
- **用途**: インタラクティブなコマンド実行時に使用

```bash
cderun --tty bash
cderun --tty node
```

### `--interactive`, `-i`
- **型**: bool
- **デフォルト**: `false`
- **説明**: STDINを開いたままにする
- **用途**: インタラクティブな入力が必要な場合

```bash
cderun --interactive python
cderun -i bash
```

**組み合わせ例**:
```bash
cderun --tty --interactive bash
cderun -ti bash  # 短縮形
```

### `--network`
- **型**: string
- **デフォルト**: `bridge`
- **説明**: コンテナを接続するネットワーク
- **値**: `bridge`, `host`, `none`, カスタムネットワーク名

```bash
cderun --network host node server.js
cderun --network none python script.py
cderun --network my-network node app.js
```

### `--mount-socket`
- **型**: string
- **デフォルト**: `""`（空文字列）
- **説明**: コンテナランタイムソケットのパスを指定
- **用途**: cderunが接続するランタイムソケットを指定する。`--mount-cderun` 等のフラグ使用時にはコンテナ内にもマウントされます。

```bash
cderun --mount-socket /var/run/docker.sock docker ps
cderun podman images --cderun-mount-socket /run/podman/podman.sock
```

### `--cderun-mount-socket`
- **型**: string
- **説明**: 設定ファイルや環境変数を上書きしてソケットパスを強制する（P1優先順位）
- **用途**: サブコマンドの後ろでも指定可能

**注意**: ソケットパスは明示的に指定する必要があります。

### `--mount-cderun`
- **型**: bool
- **デフォルト**: `false`
- **説明**: cderunバイナリをコンテナ内の `/usr/local/bin/cderun` にマウント
- **用途**: コンテナ内でcderunを使用可能にする（再帰的実行）
- **制約**: `--mount-socket`との併用が必須

```bash
cderun --mount-cderun --mount-socket /var/run/docker.sock alpine sh
```

### `--mount-tools`
- **型**: string
- **説明**: 指定したツール（カンマ区切り）のエイリアスをコンテナ内にマウント
- **制約**: `--mount-socket`との併用が必須。対象のツールは `.tools.yaml` に定義されている必要があります。

```bash
cderun --mount-cderun --mount-socket /var/run/docker.sock --mount-tools node,python alpine sh
```

### `--mount-all-tools`
- **型**: bool
- **説明**: `.tools.yaml` に定義されているすべてのツールのエイリアスをコンテナ内にマウント
- **制約**: `--mount-socket`との併用が必須

```bash
cderun --mount-cderun --mount-socket /var/run/docker.sock --mount-all-tools alpine sh
```

### `--image`
- **型**: string
- **説明**: 使用するコンテナイメージを明示的に指定（イメージマッピングを上書き）

```bash
cderun --image node:18-alpine node --version
```

### `--env`, `-e`
- **型**: stringSlice
- **説明**: 環境変数の設定・パススルー
- **用途**: `KEY=value`（直接指定）または `KEY`（ホストから取得）

```bash
cderun --env NODE_ENV=production node app.js
cderun --env NPM_TOKEN node app.js  # ホストから取得
```

### `--cderun-env`
- **型**: stringSlice
- **説明**: 環境変数の強制上書き（P1優先順位）
- **用途**: サブコマンドの後ろでも指定可能

```bash
# サブコマンドの後ろで指定
cderun node app.js --cderun-env=NODE_ENV=production
```

### `--volume`, `-v`
- **型**: stringSlice
- **説明**: ボリュームマウント
- **用途**: `hostPath:containerPath[:ro|rw]`

```bash
cderun --volume ./data:/data python script.py
cderun -v ~/.ssh:/root/.ssh:ro git clone ...
```

### `--workdir`, `-w`
- **型**: string
- **説明**: 作業ディレクトリの指定

```bash
cderun --workdir /app node server.js
```

### `--runtime`
- **型**: string
- **デフォルト**: `docker`
- **説明**: 使用するコンテナランタイムを指定（`docker` | `podman`）

```bash
cderun --runtime podman node app.js
```

### `--remove`
- **型**: bool
- **デフォルト**: `true`
- **説明**: コンテナ終了後に自動的に削除する

```bash
cderun --remove=false node app.js  # コンテナを残す
```

### `--dry-run`
- **型**: bool
- **デフォルト**: `false`
- **説明**: 実際のコンテナ実行を行わずに、コンテナ構成を表示する

```bash
cderun --dry-run node --version
```

### `--dry-run-format`, `-f`
- **型**: string
- **デフォルト**: `yaml`
- **説明**: ドライラン時の出力形式を指定
- **値**: `yaml`, `json`, `simple`

```bash
cderun --dry-run --dry-run-format json node --version
cderun --dry-run -f simple node --version
```

### `--verbose`
- **型**: count
- **説明**: ログ出力の詳細度を上げる
- **用途**: `--verbose` (INFO), `--verbose --verbose` (DEBUG), `--verbose --verbose --verbose` (TRACE)

```bash
cderun --verbose node app.js
```

### `--log-level`
- **型**: string
- **説明**: ログレベルを直接指定
- **値**: `error`, `warn`, `info`, `debug`, `trace`

```bash
cderun --log-level debug node app.js
```

### `--log-file`
- **型**: string
- **説明**: ログ出力先のファイルパス

```bash
cderun --log-file ./cderun.log node app.js
```

### `--log-format`
- **型**: string
- **デフォルト**: `text`
- **説明**: ログの出力形式 (`text` | `json`)

```bash
cderun --log-format json node app.js
```

### `--log-tee`
- **型**: bool
- **デフォルト**: `false`
- **説明**: ログを標準エラー出力とファイルの両方に出力（`--log-file` と併用）

```bash
cderun --log-file ./cderun.log --log-tee node app.js
```

### `--log-timestamp`
- **型**: bool
- **デフォルト**: `true`
- **説明**: ログにタイムスタンプを含める

```bash
cderun --log-timestamp=false node app.js
```

### `--cderun-*` (内部オーバーライドフラグ)
- **説明**: 設定ファイルや環境変数を上書きして動作を強制する（P1優先順位）。すべての標準フラグに対応する `--cderun-` プレフィックス付きのフラグが存在します。
  - 対応フラグ例: `--cderun-tty`, `--cderun-interactive`, `--cderun-image`, `--cderun-network`, `--cderun-remove`, `--cderun-runtime`, `--cderun-mount-socket`, `--cderun-env`, `--cderun-workdir`, `--cderun-volume`, `--cderun-mount-cderun`, `--cderun-mount-tools`, `--cderun-mount-all-tools`, `--cderun-dry-run`, `--cderun-dry-run-format`, `--cderun-log-level`, `--cderun-log-file`, `--cderun-log-format`, `--cderun-log-tee`, `--cderun-verbose`
- **挙動**: これらは**サブコマンドの後ろ**に配置する必要があります。サブコマンドの前に配置するとエラーになります。

## オプションの優先順位

1. **cderun内部オーバーライド (P1)**: `--cderun-*` フラグ
2. **コマンドライン引数 (P2)**: `--tty`, `--env` 等の標準フラグ
3. **環境変数 (P3)**: `CDERUN_MOUNT_SOCKET`, `CDERUN_TTY` 等
4. **ツール固有設定 (P4)**: `.tools.yaml`
5. **グローバルデフォルト** (P5): `.cderun.yaml`
6. **ハードコードされたデフォルト** (P6, 最低優先)

## 使用例

### 基本的な使用
```bash
# シンプルな実行
cderun node --version

# TTY付き
cderun --tty bash

# インタラクティブ
cderun -ti python
```

### ネットワーク設定
```bash
# ホストネットワーク
cderun --network host node server.js

# ネットワーク分離
cderun --network none python script.py
```

### Docker-in-Docker
```bash
# Dockerソケットマウント
cderun --mount-socket /var/run/docker.sock docker ps

# cderunの入れ子実行
cderun --mount-cderun --mount-socket /var/run/docker.sock alpine sh
```

### 複数オプションの組み合わせ
```bash
cderun --tty --interactive --network host --mount-socket /var/run/docker.sock docker sh
```

## 注意事項

### フラグの位置
cderunのフラグ（標準フラグ）は、原則として**サブコマンドの前**に指定する必要があります。

```bash
# 正しい（標準フラグ）
cderun --tty node --version

# 間違い（--ttyがnodeに渡される）
cderun node --tty --version
```

**例外**: `--cderun-*` で始まる**内部オーバーライドフラグ (P1)** は、**サブコマンドの後ろ**に指定する必要があります（前に置くとエラーになります）。

```bash
# 正しい（内部オーバーライドフラグ）
cderun node --version --cderun-tty

# 間違い
cderun --cderun-tty node --version
```

### 短縮形
現在サポートされている短縮形：
- `-i` → `--interactive`
- `-v` → `--volume`
- `-w` → `--workdir`
- `-e` → `--env`
- `-f` → `--dry-run-format`

将来追加予定：
- `-t` → `--tty`

### デフォルト値の確認
```bash
cderun --help
```

## トラブルシューティング

### オプションが認識されない
```bash
$ cderun node --tty
# --ttyがnodeに渡される
```

**解決策**: cderunの標準オプション（P2）はサブコマンドの前に指定します。
```bash
$ cderun --tty node
```

ただし、内部オーバーライド（P1）を使用する場合はサブコマンドの後ろに指定します。
```bash
$ cderun node --cderun-tty
```

### --mount-cderunが動作しない
```bash
$ cderun --mount-cderun node
Error: --mount-cderun requires --mount-socket
```

**解決策**: `--mount-socket`を併用
```bash
$ cderun --mount-cderun --mount-socket /var/run/docker.sock node
```
