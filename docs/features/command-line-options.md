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

### `--tty`, `-t`

- **型**: bool
- **デフォルト**: `false`
- **説明**: 疑似TTYを割り当てる
- **用途**: インタラクティブなコマンド実行時に使用

```bash
cderun --tty bash
cderun -t node
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

### `--socket-path`

- **型**: string
- **デフォルト**: 自動検出（`/var/run/docker.sock` 等）
- **説明**: コンテナランタイムソケットのホスト上のパスを指定
- **用途**: cderunが接続するランタイムソケットを指定する

```bash
cderun --socket-path /var/run/docker.sock docker ps
cderun podman images --cderun-socket-path /run/podman/podman.sock
```

### `--mount-socket`

- **型**: bool
- **デフォルト**: `false`
- **説明**: ホストのランタイムソケットをコンテナ内にマウントする
- **用途**: コンテナ内からホストのDocker/Podmanを操作する場合に使用

```bash
cderun --mount-socket docker ps
```

### `--mount-socket-path`

- **型**: string
- **デフォルト**: ホスト側のソケットパス（`--socket-path` または自動検出された値）
- **説明**: ソケットをコンテナ内にマウントする際のパスを指定
- **用途**: ホストとコンテナ内でソケットのパスを異なるものにしたい場合に使用

```bash
cderun --mount-socket --mount-socket-path /var/run/docker.sock node app.js
```

### `--mount-cderun`

- **型**: bool
- **デフォルト**: `false`
- **説明**: cderunバイナリをコンテナ内の `/usr/local/bin/cderun` にマウント
- **用途**: コンテナ内でcderunを使用可能にする（再帰的実行）
- **補足**:
  - `--mount-tools` または `--mount-all-tools` を使用する場合、このフラグは自動的に有効になります。
  - ネスト実行が検出された場合も自動的にマウントが構成されます。
  - このフラグが有効な場合、`--mount-socket` が明示的に `false` に設定されていない限り、`--mount-socket` も自動的に有効になります。

```bash
cderun --mount-cderun alpine sh
```

### `--mount-cderun-path`

- **型**: string
- **説明**: コンテナ内にマウントするホスト側のcderunバイナリのパスを指定
- **用途**: 明示的に特定のcderunバイナリをマウントしたい場合に使用

```bash
cderun --mount-cderun --mount-cderun-path /path/to/cderun alpine sh
```

### `--mount-tools`

- **型**: string
- **説明**: 指定したツール（カンマ区切り）のエイリアスをコンテナ内にマウント
- **補足**:
  - 対象のツールは `.tools.yaml` に定義されている必要があります。
  - このオプションを使用すると、`--mount-cderun` および `--mount-socket` が自動的に有効になります（明示的に `false` が指定されている場合を除く）。

```bash
cderun --mount-tools node,python alpine sh
```

### `--mount-all-tools`

- **型**: bool
- **説明**: `.tools.yaml` に定義されているすべてのツールのエイリアスをコンテナ内にマウント
- **補足**:
  - このオプションを使用すると、`--mount-cderun` および `--mount-socket` が自動的に有効になります（明示的に `false` が指定されている場合を除く）。

```bash
cderun --mount-all-tools alpine sh
```

### `--image`

- **型**: string
- **説明**: 使用するコンテナイメージを明示的に指定（イメージマッピングを上書き）

```bash
cderun --image node:18-alpine node --version
```

### `--env`, `-e`

- **型**: stringArray
- **説明**: 環境変数の設定・パススルー
- **用途**: `KEY=value`（直接指定）または `KEY`（ホストから取得）

```bash
cderun --env NODE_ENV=production node app.js
cderun --env NPM_TOKEN node app.js  # ホストから取得
```

### `--cderun-env`

- **型**: stringArray
- **説明**: 環境変数の強制上書き（P1優先順位）
- **用途**: サブコマンドの後ろでも指定可能

```bash
# サブコマンドの後ろで指定
cderun node app.js --cderun-env=NODE_ENV=production
```

### `--mount`

- **型**: stringArray
- **説明**: マウントの設定（bind, volume, tmpfsをサポート）
- **用途**: `type=bind,source=hostPath,target=containerPath[,readonly]`

```bash
cderun --mount type=bind,source=./data,target=/data python script.py
cderun --mount type=bind,source=~/.ssh,target=/root/.ssh,readonly git clone ...
cderun --mount type=tmpfs,target=/tmp alpine
```

### `--workdir`, `-w`

- **型**: string
- **説明**: 作業ディレクトリの指定

```bash
cderun --workdir /app node server.js
```

### `--strict-env`

- **型**: bool
- **デフォルト**: `false`
- **説明**: 指定された環境変数がホストに存在しない場合にエラーとする

```bash
cderun --strict-env --env NPM_TOKEN node app.js
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

### `--publish`, `-p`

- **型**: stringArray
- **説明**: ポートマッピング（ホストポート:コンテナポート）
- **用途**: コンテナのポートをホストに公開

```bash
cderun -p 8080:80 nginx
```

### `--publish-all`, `-P`

- **型**: bool
- **デフォルト**: `false`
- **説明**: すべての公開ポートをランダムなポートにマッピング

### `--expose`

- **型**: stringArray
- **説明**: 特定のポートまたはポート範囲を公開

```bash
cderun --expose 80 node app.js
cderun --expose 80/udp node app.js
```

### `--hostname`

- **型**: string
- **説明**: コンテナのホスト名

```bash
cderun --hostname my-container alpine hostname
```

### `--dns`

- **型**: stringArray
- **説明**: カスタムDNSサーバの設定

```bash
cderun --dns 8.8.8.8 alpine ping google.com
```

### `--add-host`

- **型**: stringArray
- **説明**: `/etc/hosts` へのカスタムホストマッピングの追加 (host:ip)

```bash
cderun --add-host my-server:192.168.1.10 alpine ping my-server
```

### `--user`, `-u`

- **型**: string
- **説明**: 実行ユーザー/UID (format: <name|uid>[:<group|gid>])

```bash
cderun -u 1000:1000 alpine whoami
```

### `--privileged`

- **型**: bool
- **デフォルト**: `false`
- **説明**: 特権モードで実行

```bash
cderun --privileged alpine ls /dev
```

### `--cap-add`, `--cap-drop`

- **型**: stringArray
- **説明**: Linuxケーパビリティの追加/削除

```bash
cderun --cap-add SYS_ADMIN alpine mount ...
```

### `--entrypoint`

- **型**: stringArray
- **説明**: イメージのデフォルトENTRYPOINTを上書き

```bash
cderun --entrypoint /bin/sh node -c "ls"
```

### `--pull`

- **型**: string
- **デフォルト**: `missing`
- **値**: `always`, `missing`, `never`
- **説明**: 実行前のイメージプルポリシー

### `--memory`, `-m`

- **型**: string
- **説明**: メモリ制限 (例: `512m`, `1g`)

### `--cpus`

- **型**: float64
- **説明**: CPU数制限

### `--device`

- **型**: stringArray
- **説明**: ホストデバイスをコンテナに追加

```bash
cderun --device /dev/fuse alpine ls /dev/fuse
```

### `--config`

- **型**: string
- **説明**: cderun自体の設定ファイル（`.cderun.yaml` 相当）を明示的に指定。パスの先頭に `~` または `~/` を使用してホームディレクトリを指定できます。
- **効果**: 指定された場合、標準の階層的検索とマージをスキップします。

```bash
cderun --config my-cderun.yaml node app.js
cderun --config ~/.config/cderun/custom.yaml node app.js
```

### `--tool-config`

- **型**: string
- **説明**: ツール実行設定ファイル（`.tools.yaml` 相当）を明示的に指定。パスの先頭に `~` または `~/` を使用してホームディレクトリを指定できます。
- **効果**: 指定された場合、標準の階層的検索とマージをスキップします。

```bash
cderun --tool-config my-tools.yaml node app.js
cderun --tool-config ~/tools-config.yaml node app.js
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

### `--diagnosis`

- **型**: bool
- **デフォルト**: `false`
- **説明**: システム診断情報と利用可能なツールの一覧を表示する

```bash
cderun --diagnosis
```

### `--diagnosis-format`

- **型**: string
- **デフォルト**: `yaml`
- **説明**: 診断情報の出力形式を指定
- **値**: `yaml`, `json`, `simple`

```bash
cderun --diagnosis --diagnosis-format json
```

### `--log-level`

- **型**: string
- **デフォルト**: `warn`
- **説明**: ログレベルを直接指定
- **値**: `error`, `warn`, `info`, `debug`, `trace`
- **注意**: `-v` や `--verbose` フラグは意図的にサポートされていません。代わりに `--log-level` を使用してください。

```bash
cderun --log-level info node app.js
```

### `--log-format`

- **型**: string
- **デフォルト**: `text`
- **説明**: ログの出力形式 (`text` | `json`)

```bash
cderun --log-format json --log-level info node app.js
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
- **カテゴリ別の対応フラグ例**:

  - **実行制御**: `--cderun-tty`, `--cderun-interactive`, `--cderun-env`,
    `--cderun-image`, `--cderun-runtime`, `--cderun-remove`,
    `--cderun-workdir`, `--cderun-user`, `--cderun-privileged`,
    `--cderun-entrypoint`, `--cderun-pull`, `--cderun-strict-env`, `--cderun-cap-add`,
    `--cderun-cap-drop`
  - **ネットワーク**: `--cderun-network`, `--cderun-publish`,
    `--cderun-publish-all`, `--cderun-expose`, `--cderun-hostname`,
    `--cderun-dns`, `--cderun-add-host`
  - **リソース**: `--cderun-memory`, `--cderun-cpus`
  - **マウント・ツール**: `--cderun-mount`, `--cderun-socket-path`,
    `--cderun-mount-socket`, `--cderun-mount-socket-path`,
    `--cderun-mount-cderun`, `--cderun-mount-cderun-path`,
    `--cderun-mount-tools`, `--cderun-mount-all-tools`, `--cderun-device`
  - **設定ファイル**: `--cderun-config`, `--cderun-tool-config`
  - **診断・ログ**: `--cderun-dry-run`, `--cderun-dry-run-format`,
    `--cderun-diagnosis`, `--cderun-diagnosis-format`,
    `--cderun-log-level`, `--cderun-log-format`,
    `--cderun-log-timestamp`

- **挙動**: これらは**サブコマンドの後ろ**に配置する必要があります。サブコマンドの前に配置するとエラーになります。

## その他の設定オプション

### `strictEnv`

- **説明**: 指定された環境変数がホストに存在しない場合にエラーとする設定。
- **指定方法**: `.cderun.yaml`, `.tools.yaml` の `strictEnv` フィールド、
  環境変数 `CDERUN_STRICT_ENV=true`、またはコマンドラインフラグ `--strict-env` で指定します。

## オプションの優先順位

1. **cderun内部オーバーライド (P1)**: `--cderun-*` フラグ
2. **コマンドライン引数 (P2)**: `--tty`, `--env` 等の標準フラグ
3. **環境変数 (P3)**: `CDERUN_SOCKET_PATH`, `CDERUN_MOUNT_SOCKET`,
   `CDERUN_TTY` 等。
   - **セパレータ**:
     - セミコロン (`;`): `CDERUN_ENV`, `CDERUN_MOUNT`
     - カンマ (`,`): `CDERUN_MOUNT_TOOLS`, `CDERUN_PUBLISH`, `CDERUN_EXPOSE`, `CDERUN_DNS`, `CDERUN_ADD_HOST`, `CDERUN_CAP_ADD`, `CDERUN_CAP_DROP`, `CDERUN_ENTRYPOINT`, `CDERUN_DEVICE`
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
cderun --mount-socket docker ps

# cderunの入れ子実行（ソケットは自動的にマウントされます）
cderun --mount-cderun alpine sh

# Mac等でホストとコンテナのマウントパスを変える場合
cderun --socket-path ~/.rd/docker.sock --mount-socket \
  --mount-socket-path /var/run/docker.sock docker ps
```

### 複数オプションの組み合わせ

```bash
cderun --tty --interactive --network host --mount-socket docker sh
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

- `-t` → `--tty`
- `-i` → `--interactive`
- `-w` → `--workdir`
- `-e` → `--env`
- `-f` → `--dry-run-format`
- `-p` → `--publish`
- `-P` → `--publish-all`
- `-u` → `--user`
- `-m` → `--memory`

### デフォルト値の確認

```bash
cderun --help
```

## トラブルシューティング

### オプションが認識されない

```bash
cderun node --tty
# --ttyがnodeに渡される
```

**解決策**: cderunの標準オプション（P2）はサブコマンドの前に指定します。

```bash
cderun --tty node
```

ただし、内部オーバーライド（P1）を使用する場合はサブコマンドの後ろに指定します。

```bash
cderun node --version --cderun-tty
```
