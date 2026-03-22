# 機能仕様：引数・設定優先順位ロジック

## 概要

`cderun` は、複数のソース（CLI、環境変数、YAML、デフォルト値）から設定を読み込む。
設定の競合が発生した場合、以下の **P1（最高）〜 P6（最低）** の優先順位に従って値を確定させる。

## 優先順位階層 (Resolution Hierarchy)

設定は以下の P1（最高）から P6（最低）の順に解決されます。

### P1: CDERUN Internal Overrides (最高優先順位)

- **定義**: `cderun` の動作を強制的に変更・上書きするための専用フラグ。シンボリックリンク（ポリグロットモード）利用時でも、ラップされたツールの引数と衝突せずに `cderun` 側の設定を指定することを可能にします。
- **フラグ名**: `cderun` 標準フラグ（P2）のすべてに対応する `--cderun-` プレフィックス付きフラグ。
  - **実行制御**: `--cderun-tty`, `--cderun-interactive`, `--cderun-env`, `--cderun-image`, `--cderun-runtime`, `--cderun-remove`, `--cderun-workdir`, `--cderun-user`, `--cderun-privileged`, `--cderun-entrypoint`, `--cderun-pull`, `--cderun-strict-env`, `--cderun-cap-add`, `--cderun-cap-drop`, `--cderun-hang-timeout`
  - **ネットワーク**: `--cderun-network`, `--cderun-publish`, `--cderun-publish-all`, `--cderun-expose`, `--cderun-hostname`, `--cderun-dns`, `--cderun-add-host`
  - **リソース**: `--cderun-memory`, `--cderun-cpus`
  - **設定ファイル**: `--cderun-config`, `--cderun-tool-config`
  - **マウント・ツール**: `--cderun-mount`, `--cderun-socket-path`, `--cderun-mount-socket`, `--cderun-mount-socket-path`, `--cderun-mount-cderun`, `--cderun-mount-cderun-path`, `--cderun-mount-tools`, `--cderun-mount-all-tools`, `--cderun-device`
  - **診断・ログ**: `--cderun-dry-run`, `--cderun-dry-run-format`, `--cderun-diagnosis`, `--cderun-diagnosis-format`, `--cderun-log-level`, `--cderun-log-format`, `--cderun-log-timestamp`
- **挙動**: これらが指定された場合、他の全て（P2〜P6）を無視してこの値を採用する。
- **配置規則**:
  - **Wrapper Mode**: 必ず**サブコマンドの後ろ**に配置する必要があります。前処理（Hoisting）によって内部的にサブコマンドの前方に移動され、`cderun` のフラグとしてパースされます。サブコマンドより前に配置した場合はエラーとなります。
  - **Diagnosis Mode**: サブコマンドを必要としないため、任意の場所に配置可能です。

### P2: CLI Flags (ユーザーの意図)

- **定義**: 実行時にユーザーが明示的に指定した標準フラグ。サブコマンドの**前**に置く必要があります。
- **フラグ名**:
  - `--tty`, `--interactive`, `--image`, `--network`, `--runtime`, `--socket-path`, `--mount-socket`, `--mount-socket-path`, `--env`, `--workdir`, `--mount`, `--mount-cderun`, `--mount-cderun-path`, `--mount-tools`, `--mount-all-tools`, `--remove`, `--config`, `--tool-config`
  - `--publish`, `--publish-all`, `--expose`, `--hostname`, `--dns`, `--add-host`, `--user`, `--privileged`, `--cap-add`, `--cap-drop`, `--entrypoint`, `--pull`, `--strict-env`, `--memory`, `--cpus`, `--device`
  - `--dry-run`, `--dry-run-format`, `--diagnosis`, `--diagnosis-format`, `--log-level`, `--log-format`, `--log-timestamp`
- **判定条件**: ユーザーがコマンドラインで明示的にフラグを指定したこと。指定がない場合、P3以下の判定へ進みます。

### P3: Environment Variables (グローバルオーバーライド)

- **定義**: 実行環境全体に適用される設定。
- **主要なキー**: `CDERUN_CONFIG`, `CDERUN_TOOL_CONFIG`, `CDERUN_IMAGE`, `CDERUN_TTY`, `CDERUN_INTERACTIVE`, `CDERUN_REMOVE`, `CDERUN_WORKDIR`, `CDERUN_NETWORK`, `CDERUN_RUNTIME`, `CDERUN_SOCKET_PATH`, `CDERUN_STRICT_ENV`, `CDERUN_MOUNT_SOCKET`, `CDERUN_MOUNT_SOCKET_PATH`, `CDERUN_ENV`, `CDERUN_MOUNT`, `CDERUN_MOUNT_TOOLS`, `CDERUN_MOUNT_CDERUN`, `CDERUN_MOUNT_CDERUN_PATH`, `CDERUN_MOUNT_ALL_TOOLS`, `CDERUN_PUBLISH`, `CDERUN_PUBLISH_ALL`, `CDERUN_EXPOSE`, `CDERUN_HOSTNAME`, `CDERUN_DNS`, `CDERUN_ADD_HOST`, `CDERUN_USER`, `CDERUN_PRIVILEGED`, `CDERUN_CAP_ADD`, `CDERUN_CAP_DROP`, `CDERUN_ENTRYPOINT`, `CDERUN_PULL`, `CDERUN_MEMORY`, `CDERUN_CPUS`, `CDERUN_DEVICE`, `CDERUN_DRY_RUN`, `CDERUN_DRY_RUN_FORMAT`, `CDERUN_DIAGNOSIS`, `CDERUN_DIAGNOSIS_FORMAT`, `CDERUN_LOG_LEVEL`, `CDERUN_LOG_FORMAT`, `CDERUN_LOG_TIMESTAMP`, `CDERUN_HANG_TIMEOUT`
- **注記 (`CDERUN_CONFIG` / `CDERUN_TOOL_CONFIG`)**: これらは P4/P5 の設定ファイルの**読み込み先パスを決める**ために、設定ファイルの探索前に評価されます。
- **セパレータ**:
  - セミコロン (`;`): `CDERUN_ENV`, `CDERUN_MOUNT`
  - カンマ (`,`): `CDERUN_MOUNT_TOOLS`, `CDERUN_PUBLISH`, `CDERUN_EXPOSE`, `CDERUN_DNS`, `CDERUN_ADD_HOST`, `CDERUN_CAP_ADD`, `CDERUN_CAP_DROP`, `CDERUN_ENTRYPOINT`, `CDERUN_DEVICE`

### P4: Tool-specific config (YAMLプロファイル)

- **定義**: 設定ファイル（`.tools.yaml`）内の、実行対象サブコマンド（ツール）に紐づく設定ブロック。
- **挙動**: CLIも環境変数も指定がない場合、この値を採用する。

### P5: Global defaults (プロファイルデフォルト)

- **定義**: 設定ファイル（`.cderun.yaml`）の `defaults` ブロック。
- **挙動**: P1〜P4のいずれも指定がない場合、この値を採用する。

### P6: Hardcoded Defaults (最低優先順位)

- **定義**: プログラム内でハードコードされた最終フォールバック値。
- **デフォルト値:**
  - `tty: false`
  - `interactive: false`
  - `network: bridge`
  - `remove: true`
  - `runtime: docker`
  - `pull: missing`
  - `logLevel: warn`
  - `logFormat: text`
  - `logTimestamp: true`
  - `strictEnv: false`
  - `mountSocket: false`
  - `mountCderun: false`
  - `mountAllTools: false`
  - `privileged: false`
  - `publishAll: false`
  - `dryRun: false`
  - `dryRunFormat: yaml`
  - `diagnosis: false`
  - `diagnosisFormat: yaml`
  - `hangTimeout: 2s`
  - `image`: なし (Fatal Error)

## コレクション型（リスト型）の設定について

`mounts`, `devices`, `env`, `ports`, `mountTools` などのリスト形式の設定も、スカラ型と同様に **「優先順位の高いソースに値があれば、それより低い優先順位のソースはすべて無視される（上書き）」** という挙動になります。

**重要な注意点**:
実装上、高い優先順位のソースにおいて**空のリスト（値なし）**が検出された場合、それは「未指定」とみなされ、より低い優先順位のソースへフォールバックします。

- **例**: `.tools.yaml` (P4) で `mounts: []` と明示的に空のリストを指定しても、`.cderun.yaml` (P5) に `mounts` の定義があれば、P5 の値が採用されます。
- **意図**: これにより、上位レベルでの意図しない「設定の全消去」を防ぎ、デフォルト設定を安全に維持します。

## 特殊な連動ロジック (Transitive Auto-enablement)

一部のオプションは、他のオプションの設定状況に基づいて連鎖的に自動有効化されます。これらのロジックは、対象のオプションが P1〜P5 のどのレベルでも**明示的に設定（`nil` 以外）されていない場合にのみ**適用されます。

1. **`mountCderun` の自動有効化**:
   `mountTools`（指定あり）または `mountAllTools: true` の場合、`mountCderun` は自動的に `true` になります。
2. **`mountSocket` の自動有効化**:
   `mountCderun` が `true`（自動有効化されたものを含む）の場合、`mountSocket` は自動的に `true` になります。

**注意**: いずれかの優先順位（P1〜P5）で明示的に `false` が指定されている場合、上記の自動有効化は行われません。例えば、`.cderun.yaml` で `mountSocket: false` と設定されている場合、`mountCderun` が有効であってもソケットはマウントされません。

---
*2026年3月17日時点の仕様である。*
