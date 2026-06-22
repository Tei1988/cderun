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
- **環境変数**: `CDERUN_TTY`
- **説明**: 疑似TTYを割り当てる
- **用途**: インタラクティブなコマンド実行時に使用

```bash
cderun --tty bash
cderun -t node
```

### `--interactive`, `-i`

- **型**: bool
- **デフォルト**: `false`
- **環境変数**: `CDERUN_INTERACTIVE`
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
- **環境変数**: `CDERUN_NETWORK`
- **説明**: コンテナを接続するネットワーク。**注意**: `containerd` ランタイムは現在 `host` ネットワークのみをサポートしています。
- **値**: `bridge`, `host`, `none`, カスタムネットワーク名

```bash
cderun --network host node server.js
cderun --network none python script.py
cderun --network my-network node app.js
```

### `--socket-path`

- **型**: string
- **デフォルト**: 自動検出（`/var/run/docker.sock` 等）
- **環境変数**: `CDERUN_SOCKET_PATH`
- **説明**: コンテナランタイムソケットのホスト上のパスを指定
- **用途**: cderunが接続するランタイムソケットを指定する

```bash
cderun --socket-path /var/run/docker.sock docker ps
cderun podman images --cderun-socket-path /run/podman/podman.sock
```

### `--mount-socket`

- **型**: bool
- **デフォルト**: `false`
- **環境変数**: `CDERUN_MOUNT_SOCKET`
- **説明**: ホストのランタイムソケットをコンテナ内にマウントする
- **用途**: コンテナ内からホストのDocker/Podmanを操作する場合に使用

```bash
cderun --mount-socket docker ps
```

### `--mount-socket-path`

- **型**: string
- **デフォルト**: ホスト側のソケットパス（`--socket-path` または自動検出された値）
- **環境変数**: `CDERUN_MOUNT_SOCKET_PATH`
- **説明**: ソケットをコンテナ内にマウントする際のパスを指定
- **用途**: ホストとコンテナ内でソケットのパスを異なるものにしたい場合に使用

```bash
cderun --mount-socket --mount-socket-path /var/run/docker.sock node app.js
```

### `--mount-cderun`

- **型**: bool
- **デフォルト**: `false`
- **環境変数**: `CDERUN_MOUNT_CDERUN`
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
- **環境変数**: `CDERUN_MOUNT_CDERUN_PATH`
- **説明**: コンテナ内にマウントするホスト側のcderunバイナリのパスを指定
- **用途**: 明示的に特定のcderunバイナリをマウントしたい場合に使用

```bash
cderun --mount-cderun --mount-cderun-path /path/to/cderun alpine sh
```

### `--mount-tools`

- **型**: string
- **環境変数**: `CDERUN_MOUNT_TOOLS`
- **説明**: 指定したツール（カンマ区切り）のエイリアスをコンテナ内にマウント
- **補足**:
  - 対象のツールは `.tools.yaml` に定義されている必要があります。
  - このオプションを使用すると、`--mount-cderun` および `--mount-socket` が自動的に有効になります（明示的に `false` が指定されている場合を除く）。

```bash
cderun --mount-tools node,python alpine sh
```

### `--mount-all-tools`

- **型**: bool
- **デフォルト**: `false`
- **環境変数**: `CDERUN_MOUNT_ALL_TOOLS`
- **説明**: `.tools.yaml` に定義されているすべてのツールのエイリアスをコンテナ内にマウント
- **補足**:
  - このオプションを使用すると、`--mount-cderun` および `--mount-socket` が自動的に有効になります（明示的に `false` が指定されている場合を除く）。

```bash
cderun --mount-all-tools alpine sh
```

### `--image`

- **型**: string
- **環境変数**: `CDERUN_IMAGE`
- **説明**: 使用するコンテナイメージを明示的に指定（イメージマッピングを上書き）
- **注意**:
  - アドホック実行（設定にないツール名の指定）時には必須となります。
  - `{{env:KEY}}` などの式が使用可能です。詳細は [値の解決](./value-resolution.md) を参照してください。

```bash
cderun --image node:18-alpine node --version
cderun --image "node:{{env:NODE_VERSION:-20-alpine}}" node --version
```

### `--env`, `-e`

- **型**: stringArray
- **環境変数**: `CDERUN_ENV`
- **説明**: 環境変数の設定・パススルー
- **用途**: `KEY=value`（直接指定）または `KEY`（ホストから取得）
- **補足**:
  - CLIフラグ（P1/P2）では、複数の環境変数を指定する場合、フラグを繰り返す必要があります（例: `-e A=1 -e B=2`）。
  - 環境変数 `CDERUN_ENV` (P3) では、セミコロン (`;`) をセパレータとして使用します（例: `export CDERUN_ENV="A=1;B=2"`）。
  - 値には `{{PWD}}` などの式が使用可能です。詳細は [値の解決](./value-resolution.md) を参照してください。

```bash
cderun --env NODE_ENV=production node app.js
cderun --env NPM_TOKEN node app.js  # ホストから取得
cderun --env "PROJECT_DIR={{PWD}}" node app.js
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
- **環境変数**: `CDERUN_MOUNT`
- **説明**: マウントの設定（bind, volume, tmpfsをサポート）
- **用途**: `type=bind,source=hostPath,target=containerPath[,readonly][,optional]`
- **キーワード**:
  - `type`: `bind` | `volume` | `tmpfs`
  - `source` (エイリアス: `src`): ホスト側のパス
  - `target` (エイリアス: `dst`, `destination`): コンテナ内のパス
  - `readonly`: 読み取り専用マウント
  - `optional`: ホスト側のソースが存在しなくてもエラーにせずスキップする（`type=bind` のみ）
- **補足**:
  - `optional`（または `optional=true`）を指定すると、`type=bind` の場合にホスト側の `source` パスが存在しなくてもエラーにせず、マウントをスキップします。
  - CLIフラグ（P1/P2）では、複数のマウントを指定する場合、フラグを繰り返す必要があります。
  - 環境変数 `CDERUN_MOUNT` (P3) では、セミコロン (`;`) をセパレータとして使用します。
  - `source` や `target` には `{{HOME}}` などの式が使用可能です。詳細は [値の解決](./value-resolution.md) を参照してください。

```bash
cderun --mount type=bind,source=./data,target=/data python script.py
cderun --mount type=bind,source=~/.ssh,target=/root/.ssh,readonly git clone ...
cderun --mount type=bind,source=./config,target=/config,optional node app.js
cderun --mount type=tmpfs,target=/tmp alpine
cderun --mount "type=bind,source={{HOME}}/.npmrc,target=/root/.npmrc" node app.js
```

### `--workdir`, `-w`

- **型**: string
- **環境変数**: `CDERUN_WORKDIR`
- **説明**: 作業ディレクトリの指定
- **補足**: `{{PWD}}` などの式が使用可能です。詳細は [値の解決](./value-resolution.md) を参照してください。

```bash
cderun --workdir /app node server.js
cderun --workdir "{{PWD}}/src" node app.js
```

### `--strict-env`

- **型**: bool
- **デフォルト**: `false`
- **環境変数**: `CDERUN_STRICT_ENV`
- **説明**: 指定された環境変数がホストに存在しない場合にエラーとする

```bash
cderun --strict-env --env NPM_TOKEN node app.js
```

### `--runtime`

- **型**: string
- **デフォルト**: `docker`
- **環境変数**: `CDERUN_RUNTIME`
- **説明**: 使用するコンテナランタイムを指定（`docker` | `podman` | `containerd`）。**注意**: `containerd` は現在実験的なサポートです。

```bash
cderun --runtime podman node app.js
```

### `--remove`

- **型**: bool
- **デフォルト**: `true`
- **環境変数**: `CDERUN_REMOVE`
- **説明**: コンテナ終了後に自動的に削除する

```bash
cderun --remove=false node app.js  # コンテナを残す
```

### `--publish`, `-p`

- **型**: stringArray
- **環境変数**: `CDERUN_PUBLISH`
- **説明**: ポートマッピング（ホストポート:コンテナポート）
- **用途**: コンテナのポートをホストに公開
- **補足**:
  - CLIフラグ（P1/P2）では、複数のポートを指定する場合、フラグを繰り返す必要があります。
  - 環境変数 `CDERUN_PUBLISH` (P3) では、カンマ (`,`) をセパレータとして使用します。
  - P1/P2/P3 のいずれかで**明示的に空のリスト**（YAMLでの `[]` や環境変数での空文字列）を指定した場合、それは意図的な「空の設定」とみなされ、下位レベルの設定を上書き（無効化）します。

```bash
cderun -p 8080:80 nginx
```

### `--publish-all`, `-P`

- **型**: bool
- **デフォルト**: `false`
- **環境変数**: `CDERUN_PUBLISH_ALL`
- **説明**: すべての公開ポートをランダムなポートにマッピング

### `--expose`

- **型**: stringArray
- **環境変数**: `CDERUN_EXPOSE`
- **説明**: 特定のポートまたはポート範囲を公開
- **補足**:
  - CLIフラグ（P1/P2）では、フラグを繰り返して複数指定します。
  - 環境変数 `CDERUN_EXPOSE` (P3) では、カンマ (`,`) をセパレータとして使用します。

```bash
cderun --expose 80 node app.js
cderun --expose 80/udp node app.js
```

### `--hostname`

- **型**: string
- **環境変数**: `CDERUN_HOSTNAME`
- **説明**: コンテナのホスト名

```bash
cderun --hostname my-container alpine hostname
```

### `--dns`

- **型**: stringArray
- **環境変数**: `CDERUN_DNS`
- **説明**: カスタムDNSサーバの設定
- **補足**:
  - CLIフラグ（P1/P2）では、フラグを繰り返して複数指定します。
  - 環境変数 `CDERUN_DNS` (P3) では、カンマ (`,`) をセパレータとして使用します。

```bash
cderun --dns 8.8.8.8 alpine ping google.com
```

### `--add-host`

- **型**: stringArray
- **環境変数**: `CDERUN_ADD_HOST`
- **説明**: `/etc/hosts` へのカスタムホストマッピングの追加 (host:ip)
- **補足**:
  - CLIフラグ（P1/P2）では、フラグを繰り返して複数指定します。
  - 環境変数 `CDERUN_ADD_HOST` (P3) では、カンマ (`,`) をセパレータとして使用します。

```bash
cderun --add-host my-server:192.168.1.10 alpine ping my-server
```

### `--user`, `-u`

- **型**: string
- **環境変数**: `CDERUN_USER`
- **説明**: 実行ユーザー/UID (format: <name|uid>[:<group|gid>])

```bash
cderun -u 1000:1000 alpine whoami
```

### `--privileged`

- **型**: bool
- **デフォルト**: `false`
- **環境変数**: `CDERUN_PRIVILEGED`
- **説明**: 特権モードで実行

```bash
cderun --privileged alpine ls /dev
```

### `--cap-add`

- **型**: stringArray
- **環境変数**: `CDERUN_CAP_ADD`
- **説明**: Linuxケーパビリティの追加
- **補足**:
  - CLIフラグ（P1/P2）では、フラグを繰り返して複数指定します。
  - 環境変数 `CDERUN_CAP_ADD` (P3) では、カンマ (`,`) をセパレータとして使用します。

```bash
cderun --cap-add SYS_ADMIN alpine mount ...
```

### `--cap-drop`

- **型**: stringArray
- **環境変数**: `CDERUN_CAP_DROP`
- **説明**: Linuxケーパビリティの削除
- **補足**:
  - CLIフラグ（P1/P2）では、フラグを繰り返して複数指定します。
  - 環境変数 `CDERUN_CAP_DROP` (P3) では、カンマ (`,`) をセパレータとして使用します。

### `--entrypoint`

- **型**: stringArray
- **環境変数**: `CDERUN_ENTRYPOINT`
- **説明**: イメージのデフォルトENTRYPOINTを上書き
- **補足**:
  - CLIフラグ（P1/P2）では、フラグを繰り返して複数指定します。
  - 環境変数 `CDERUN_ENTRYPOINT` (P3) では、カンマ (`,`) をセパレータとして使用します。

```bash
cderun --entrypoint /bin/sh node -c "ls"
```

### `--pull`

- **型**: string
- **デフォルト**: `missing`
- **環境変数**: `CDERUN_PULL`
- **値**: `always`, `missing`, `never`
- **説明**: 実行前のイメージプルポリシー

### `--pull-max-retries`

- **型**: int
- **デフォルト**: `3`
- **環境変数**: `CDERUN_PULL_MAX_RETRIES`
- **説明**: イメージプル失敗時の最大リトライ回数。

### `--pull-backoff-base`

- **型**: string (Duration)
- **デフォルト**: `1s`
- **環境変数**: `CDERUN_PULL_BACKOFF_BASE`
- **説明**: イメージプルリトライ時の指数バックオフの基底時間（例: `1s`, `500ms`）。

### `--memory`, `-m`

- **型**: string
- **環境変数**: `CDERUN_MEMORY`
- **説明**: メモリ制限 (例: `512m`, `1g`)

### `--cpus`

- **型**: float64
- **環境変数**: `CDERUN_CPUS`
- **説明**: CPU数制限

### `--device`

- **型**: stringArray
- **環境変数**: `CDERUN_DEVICE`
- **説明**: ホストデバイスをコンテナに追加
- **補足**:
  - CLIフラグ（P1/P2）では、フラグを繰り返して複数指定します。
  - 環境変数 `CDERUN_DEVICE` (P3) では、カンマ (`,`) をセパレータとして使用します。

```bash
cderun --device /dev/fuse alpine ls /dev/fuse
```

### `--config`

- **型**: string
- **環境変数**: `CDERUN_CONFIG`
- **説明**: cderun自体の設定ファイル（`.cderun.yaml` 相当）を明示的に指定。パスの先頭に `~` または `~/` を使用してホームディレクトリを指定できます。
- **効果**: 指定された場合、標準の階層的検索とマージをスキップします。

```bash
cderun --config my-cderun.yaml node app.js
cderun --config ~/.config/cderun/custom.yaml node app.js
```

### `--tool-config`

- **型**: string
- **環境変数**: `CDERUN_TOOL_CONFIG`
- **説明**: ツール実行設定ファイル（`.tools.yaml` 相当）を明示的に指定。パスの先頭に `~` または `~/` を使用してホームディレクトリを指定できます。
- **効果**: 指定された場合、標準の階層的検索とマージをスキップします。

```bash
cderun --tool-config my-tools.yaml node app.js
cderun --tool-config ~/tools-config.yaml node app.js
```

### `--dry-run`

- **型**: bool
- **デフォルト**: `false`
- **環境変数**: `CDERUN_DRY_RUN`
- **説明**: 実際のコンテナ実行を行わずに、コンテナ構成を表示する

```bash
cderun --dry-run node --version
```

### `--dry-run-format`, `-f`

- **型**: string
- **デフォルト**: `yaml`
- **環境変数**: `CDERUN_DRY_RUN_FORMAT`
- **説明**: ドライラン時の出力形式を指定
- **値**: `yaml`, `json`, `simple`

```bash
cderun --dry-run --dry-run-format json node --version
cderun --dry-run -f simple node --version
```

### `--diagnosis`

- **型**: bool
- **デフォルト**: `false`
- **環境変数**: `CDERUN_DIAGNOSIS`
- **説明**: システム診断情報と利用可能なツールの一覧を表示する。このモードはサブコマンドの指定を必要としません。

```bash
cderun --diagnosis
```

### `--diagnosis-format`

- **型**: string
- **デフォルト**: `yaml`
- **環境変数**: `CDERUN_DIAGNOSIS_FORMAT`
- **説明**: 診断情報の出力形式を指定
- **値**: `yaml`, `json`, `simple`

```bash
cderun --diagnosis --diagnosis-format json
```

### `--hang-timeout`

- **型**: string (Duration)
- **デフォルト**: `10s`
- **環境変数**: `CDERUN_HANG_TIMEOUT`
- **説明**: 非インタラクティブまたは非TTYセッションにおける、I/O完了後の強制終了猶予時間。
- **形式**: Go の Duration 形式（例: `10s`, `5s`, `0`）。
- **補足**:
  - `0` を指定すると、コンテナが自然に終了するまで無期限に待機します。
  - ホストの標準入力が端末であり、かつインタラクティブモード（`--interactive` / `-i`）が有効な場合は、この設定に関わらず無期限に待機します。
- **詳細**: [ハングタイムアウト](./hang-timeout.md) を参照

```bash
cderun --hang-timeout 5s node script.js
```

### `--log-level`

- **型**: string
- **デフォルト**: `warn`
- **環境変数**: `CDERUN_LOG_LEVEL`
- **説明**: ログレベルを直接指定
- **値**: `error`, `warn`, `info`, `debug`, `trace` (`warn` のエイリアスとして `warning` も使用可能)
- **注意**: `-v` や `--verbose` フラグは意図的にサポートされていません。代わりに `--log-level` を使用してください。

```bash
cderun --log-level info node app.js
```

### `--log-format`

- **型**: string
- **デフォルト**: `text`
- **環境変数**: `CDERUN_LOG_FORMAT`
- **説明**: ログの出力形式 (`text` | `json`)

```bash
cderun --log-format json --log-level info node app.js
```

### `--log-timestamp`

- **型**: bool
- **デフォルト**: `true`
- **環境変数**: `CDERUN_LOG_TIMESTAMP`
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
    `--cderun-entrypoint`, `--cderun-pull`, `--cderun-pull-max-retries`,
    `--cderun-pull-backoff-base`, `--cderun-strict-env`, `--cderun-cap-add`,
    `--cderun-cap-drop`, `--cderun-hang-timeout`
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

- **挙動**: これらは**サブコマンドの後ろ**に配置する必要があります。サブコマンドの前に配置するとエラーになります（Diagnosis Mode を除く）。
- **配置規則**:
  - **Wrapper Mode**: 必ずサブコマンドの後ろに配置してください。
  - **Diagnosis Mode**: サブコマンドがないため、任意の場所に配置可能です。

詳細な動作（ホイスト機能）については [引数解析](./argument-parsing.md) を参照してください。

## その他の設定オプション

### `strictEnv`

- **説明**: 指定された環境変数がホストに存在しない場合にエラーとする設定。
- **指定方法**: `.cderun.yaml`, `.tools.yaml` の `strictEnv` フィールド、
  環境変数 `CDERUN_STRICT_ENV=true`、またはコマンドラインフラグ `--strict-env` で指定します。

## オプションの優先順位

優先順位の詳細は [引数・設定優先順位](./argument-priority-logic.md) を参照。

### 実行制御用環境変数

これらは優先順位階層（P1-P6）とは別に、実行時の挙動を直接制御するために使用されます。

- **`CDERUN_HANG_TIMEOUT`**: 非インタラクティブまたは非TTYセッションにおける、I/O完了後の終了猶予時間（デフォルト: `10s`）。詳細な動作条件については [ハングタイムアウト](./hang-timeout.md) を参照してください。
- **`CDERUN_REMOVE`**: 自動的にコンテナを削除するかどうか（デフォルト: `true`）。

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

---

**例外**: `--cderun-*` で始まる**内部オーバーライドフラグ (P1)** は、通常**サブコマンドの後ろ**に指定する必要があります（前に置くとエラーになります）。ただし、サブコマンドを必要としない **Diagnosis Mode** では任意の場所に配置できます。

```bash
# 正しい（Wrapper Mode での内部オーバーライドフラグ）
cderun node --version --cderun-tty

# 間違い（Wrapper Mode）
cderun --cderun-tty node --version

# 正しい（Diagnosis Mode）
cderun --diagnosis --cderun-log-level=debug
cderun --cderun-log-level=debug --diagnosis
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
