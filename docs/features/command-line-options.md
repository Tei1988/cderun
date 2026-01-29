# コマンドラインオプション

## 概要

`cderun`のすべてのコマンドラインオプションのリファレンス。

## 基本構文

```bash
cderun [cderun-options] <subcommand> [subcommand-args]
```

**重要**: 最初の非フラグ引数がサブコマンドとして扱われ、それ以降の引数はすべてサブコマンドに渡されます。

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
- **用途**: cderunが接続するランタイムソケットを指定する。将来的にコンテナ内へのマウントもサポート予定。

```bash
cderun --mount-socket /var/run/docker.sock docker ps
cderun --mount-socket /run/podman/podman.sock podman images
```

**注意**: ソケットパスは明示的に指定する必要があります。

### `--mount-cderun`
- **型**: bool
- **デフォルト**: `false`
- **説明**: cderunバイナリをコンテナ内にマウント（開発中）
- **用途**: コンテナ内でcderunを使用可能にする
- **制約**: `--mount-socket`との併用が必須。現在はフラグのみ定義されており、実装は将来のフェーズで予定されている。

```bash
cderun --mount-cderun --mount-socket /var/run/docker.sock alpine sh
```

### `--image`
- **型**: string
- **説明**: 使用するコンテナイメージを明示的に指定（イメージマッピングを上書き）

```bash
cderun --image node:18-alpine node --version
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

### `--cderun-tty` / `--cderun-interactive`
- **型**: bool
- **説明**: 設定ファイルや環境変数を上書きしてTTY/Interactiveを強制する（P1優先順位）

## 将来追加予定のオプション

### `--env`, `-e`
環境変数の設定・パススルー（現在は `.tools.yaml` でのみ設定可能）
```bash
cderun --env NODE_ENV=production node app.js
cderun --env NPM_TOKEN node app.js  # ホストから取得
```

### `--volume`, `-v`
ボリュームマウント（現在は `.tools.yaml` でのみ設定可能）
```bash
cderun --volume ./data:/data python script.py
cderun -v ~/.ssh:/root/.ssh:ro git clone ...
```

### `--workdir`, `-w`
作業ディレクトリの指定（現在は `.tools.yaml` でのみ設定可能）
```bash
cderun --workdir /app node server.js
```

### `--dry-run`
実行せずにコマンドをプレビュー
```bash
cderun --dry-run node app.js
```

## オプションの優先順位

1. **コマンドライン引数** (最優先)
2. **環境変数** (例: `CDERUN_TTY=true`)
3. **ツール固有設定** (`.tools.yaml`)
4. **グローバルデフォルト** (`.cderun.yaml`)
5. **ハードコードされたデフォルト** (最低優先)

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
cderunのフラグは**サブコマンドの前**に指定する必要があります：

```bash
# 正しい
cderun --tty node --version

# 間違い（--ttyがnodeに渡される）
cderun node --tty --version
```

### 短縮形
現在サポートされている短縮形：
- `-i` → `--interactive`

将来追加予定：
- `-t` → `--tty`
- `-v` → `--volume`
- `-w` → `--workdir`
- `-e` → `--env`

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

**解決策**: cderunのオプションはサブコマンドの前に指定
```bash
$ cderun --tty node
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
