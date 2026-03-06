# 機能仕様：引数・設定優先順位ロジック (完了)

## 概要

`cderun` は、複数のソース（CLI、環境変数、YAML、デフォルト値）から設定を読み込む。
設定の競合が発生した場合、以下の **P1（最高）〜 P6（最低）** の優先順位に従って値を確定させる。

## 優先順位階層 (Resolution Hierarchy)

### P1: CDERUN Internal Overrides (最高優先順位)

- **定義**: 動作を強制的に変更するための専用フラグ。シンボリックリンク利用時でも `cderun` 側の設定を上書きすることを想定したフラグ。
- **フラグ名**: `cderun` 標準フラグのすべてに対応する `--cderun-` プレフィックス付きフラグ。
  - **実行制御**: `--cderun-tty`, `--cderun-interactive`, `--cderun-image`, `--cderun-runtime`, `--cderun-remove`, `--cderun-workdir`, `--cderun-user`, `--cderun-privileged`, `--cderun-entrypoint`, `--cderun-pull`, `--cderun-strict-env`, `--cderun-cap-add`, `--cderun-cap-drop`
  - **ネットワーク**: `--cderun-network`, `--cderun-publish`, `--cderun-publish-all`, `--cderun-expose`, `--cderun-hostname`, `--cderun-dns`, `--cderun-add-host`
  - **リソース**: `--cderun-memory`, `--cderun-cpus`
  - **設定ファイル**: `--cderun-config`, `--cderun-tool-config`
  - **マウント・ツール**: `--cderun-mount`, `--cderun-socket-path`, `--cderun-mount-socket`, `--cderun-mount-socket-path`, `--cderun-mount-cderun`, `--cderun-mount-cderun-path`, `--cderun-mount-tools`, `--cderun-mount-all-tools`, `--cderun-device`
  - **診断・ログ**: `--cderun-dry-run`, `--cderun-dry-run-format`, `--cderun-diagnosis`, `--cderun-diagnosis-format`, `--cderun-log-level`, `--cderun-log-format`, `--cderun-log-timestamp`
- **挙動**: これらが指定された場合、他の全て（P2〜P5）を無視してこの値を採用する。また、これらは**サブコマンドの後ろ**に配置する必要があります。

### P2: CLI Flags (ユーザーの意図)

- **定義**: 実行時にユーザーが明示的に指定した標準フラグ。
- **フラグ名**:
  - `--tty`, `--interactive`, `--image`, `--network`, `--runtime`, `--socket-path`, `--mount-socket`, `--mount-socket-path`, `--env`, `--workdir`, `--mount`, `--mount-cderun`, `--mount-cderun-path`, `--mount-tools`, `--mount-all-tools`, `--remove`, `--config`, `--tool-config`
  - `--publish`, `--publish-all`, `--expose`, `--hostname`, `--dns`, `--add-host`, `--user`, `--privileged`, `--cap-add`, `--cap-drop`, `--entrypoint`, `--pull`, `--strict-env`, `--memory`, `--cpus`, `--device`
  - `--dry-run`, `--dry-run-format`, `--diagnosis`, `--diagnosis-format`, `--log-level`, `--log-format`, `--log-timestamp`
- **判定条件**: `cmd.Flags().Changed(name)` が `true` であること。
  - ※ ユーザーがフラグを入力していない場合、Cobraが持つデフォルト値は無視し、P3以下の判定へ進むこと。

### P3: Environment Variables (グローバルオーバーライド)

- **定義**: 実行環境全体に適用される設定。
- **主要なキー**: `CDERUN_CONFIG`(*), `CDERUN_TOOL_CONFIG`(*), `CDERUN_IMAGE`, `CDERUN_TTY`, `CDERUN_INTERACTIVE`, `CDERUN_REMOVE`, `CDERUN_WORKDIR`, `CDERUN_NETWORK`, `CDERUN_RUNTIME`, `CDERUN_SOCKET_PATH`, `CDERUN_STRICT_ENV`, `CDERUN_MOUNT_SOCKET`, `CDERUN_MOUNT_SOCKET_PATH`, `CDERUN_ENV`, `CDERUN_MOUNT`, `CDERUN_MOUNT_TOOLS`, `CDERUN_MOUNT_CDERUN`, `CDERUN_MOUNT_CDERUN_PATH`, `CDERUN_MOUNT_ALL_TOOLS`, `CDERUN_PUBLISH`, `CDERUN_PUBLISH_ALL`, `CDERUN_EXPOSE`, `CDERUN_HOSTNAME`, `CDERUN_DNS`, `CDERUN_ADD_HOST`, `CDERUN_USER`, `CDERUN_PRIVILEGED`, `CDERUN_CAP_ADD`, `CDERUN_CAP_DROP`, `CDERUN_ENTRYPOINT`, `CDERUN_PULL`, `CDERUN_MEMORY`, `CDERUN_CPUS`, `CDERUN_DEVICE`, `CDERUN_DRY_RUN`, `CDERUN_DRY_RUN_FORMAT`, `CDERUN_DIAGNOSIS`, `CDERUN_DIAGNOSIS_FORMAT`, `CDERUN_LOG_LEVEL`, `CDERUN_LOG_FORMAT`, `CDERUN_LOG_TIMESTAMP`, `CDERUN_HANG_TIMEOUT`
- **注記 (*)**: `CDERUN_CONFIG` および `CDERUN_TOOL_CONFIG` は、設定ファイルのパスを特定するための前処理で使用されます。これらは通常の優先順位（P1-P5）システム外で動作します。
- **セパレータ**: `CDERUN_ENV` および `CDERUN_MOUNT` はセミコロン (`;`) を、`CDERUN_MOUNT_TOOLS`, `CDERUN_PUBLISH`, `CDERUN_EXPOSE`, `CDERUN_DNS`, `CDERUN_ADD_HOST`, `CDERUN_CAP_ADD`, `CDERUN_CAP_DROP`, `CDERUN_ENTRYPOINT`, `CDERUN_DEVICE` はカンマ (`,`) をセパレータとして使用します。
- **挙動**: CLIでの指定がない場合、環境変数の値を確認する。設定されていればそれを採用する。

### P4: Tool-specific config (YAMLプロファイル)

- **定義**: 設定ファイル（`.tools.yaml`）内の、実行対象サブコマンド（ツール）に紐づく設定ブロック。
- **挙動**: CLIも環境変数も指定がない場合、この値を採用する。

```yaml
# .tools.yaml (P4 Source)
node:
  image: node:20-alpine
  interactive: true  # P4 value
  tty: true          # P4 value
```

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
  - `dryRunFormat: yaml`
  - `diagnosisFormat: yaml`
  - `image`: なし (Fatal Error)
    - ※ P1〜P6のいずれでも解決できない場合、プログラムはエラーメッセージを出力して終了すること (Exit Code 1)。勝手なデフォルトイメージ（`ubuntu:latest` 等）を使用してはならない。

## 判定ロジックの実装要件

以下のロジックフローで値を解決する：

1. **CLI指定の確認 (P1, P2)**: `Changed` 状態を確認し、ユーザーの明示的な入力を最優先する。
2. **環境変数の確認 (P3)**: CLI指定がない場合、定義された環境変数の存在を確認する。
3. **ツール別設定の確認 (P4)**: `.tools.yaml` の設定を確認する。
4. **グローバルデフォルトの確認 (P5)**: `.cderun.yaml` の `defaults` を確認する。
5. **ハードコード値の確認 (P6)**: 最終的なフォールバック値を採用する。

## コレクション型（リスト型）の設定について

`mounts`, `devices`, `env`, `ports`, `mountTools` などのリスト形式の設定についても、スカラ型（`image`, `network` 等）と同様に **「優先順位の高いソースに値があれば、それより低い優先順位のソースはすべて無視される（上書き）」** という挙動になります。

以前のバージョンでは `mounts` など一部の項目で「マージ（追加）」される挙動がありましたが、現在は一貫して「上書き」に変更されています。

### 例: 環境変数の解決

1. P1 (`--cderun-env`) があれば、それを使用し、P2〜P5を無視。
2. P1 がなく P2 (`--env`) があれば、それを使用し、P3〜P5を無視。
3. P1, P2 がなく P3 (`CDERUN_ENV`) があれば、それを使用し、P4〜P5を無視。
4. ...以下同様。

※ 同一の優先順位内（例：複数の `--env` フラグ、またはひとつの設定ファイル内）では、すべての値が合算され、キーが重複する場合は最後に指定された値が優先されます。

## 注意点

- **明示的な未指定の扱い**: YAMLのフィールドはポインタ型（`*bool` 等）で定義し、「未設定（nil）」と「明示的なfalse」を区別できるようにする。
- **YAML非サポートの設定**: `dry-run` および `diagnosis` モードの設定は、設定ファイル（P4/P5）ではサポートされておらず、CLI（P1/P2）または環境変数（P3）でのみ有効です。

## 実行制御用環境変数

これらは優先順位階層（P1-P6）とは別に、実行時の挙動を直接制御するために使用されます。

- **`CDERUN_HANG_TIMEOUT`**: I/O完了後（またはエラー発生時）、コンテナが終了するのを待機する猶予期間（デフォルト: `2s`）。`500ms`, `5s` のようなGoのDuration形式で指定します。タイムアウト経過後、コンテナが既に終了していれば即座に復帰します。コンテナが実行中の場合、インタラクティブモード（`--interactive` / `-i`）が無効であるか、またはホスト標準入力が端末ではない場合に限り、`SIGKILL` による強制終了を試行します。

---
*2026年3月6日時点の仕様である。*
