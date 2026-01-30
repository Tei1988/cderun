# Feature: Argument & Configuration Priority Logic (Completed)

## 概要
`cderun` は、複数のソース（CLI、環境変数、YAML、デフォルト値）から設定を読み込む。
設定の競合が発生した場合、以下の **P1（最高）〜 P5（最低）** の優先順位に従って値を確定させる。

## 優先順位階層 (Resolution Hierarchy)

### P1: CDERUN Internal Overrides (Highest Priority)
- **定義**: 動作を強制的に変更するための専用フラグ。シンボリックリンク利用時でも `cderun` 側の設定を上書きすることを想定したフラグ。
- **フラグ名**: `--cderun-tty`, `--cderun-interactive`, `--cderun-mount-socket`, `--cderun-env`
- **挙動**: これらが指定された場合、他の全て（P2〜P5）を無視してこの値を採用する。また、サブコマンドの後ろに配置しても `cderun` によって認識される。

### P2: CLI Flags (User Intent)
- **定義**: 実行時にユーザーが明示的に指定した標準フラグ。
- **フラグ名**: `--tty`, `--interactive`, `--image`, `--network`, `--runtime`, `--mount-socket` 等。
- **判定条件**: `cmd.Flags().Changed(name)` が `true` であること。
  - ※ ユーザーがフラグを入力していない場合、Cobraが持つデフォルト値は無視し、P3以下の判定へ進むこと。

### P3: Environment Variables (Global Override)
- **定義**: 実行環境全体に適用される設定。
- **主要なキー**: `CDERUN_IMAGE`, `CDERUN_TTY`, `CDERUN_INTERACTIVE`, `CDERUN_NETWORK`, `CDERUN_RUNTIME`, `CDERUN_MOUNT_SOCKET` 等。
- **挙動**: CLIでの指定がない場合、環境変数の値を確認する。設定されていればそれを採用する。
- **注意**: `DOCKER_HOST` は `cderun` 自体の設定（ソケットマウントの検出等）には使用されなくなりました。

### P4: Tool-specific config (YAML Profile)
- **定義**: 設定ファイル（`.tools.yaml`）内の、実行対象サブコマンド（ツール）に紐づく設定ブロック。
- **挙動**: CLIも環境変数も指定がない場合、この値を採用する。

```yaml
# .tools.yaml (P4 Source)
node:
  image: node:20-alpine
  interactive: true  # P4 value
  tty: true          # P4 value
```

### P5: Global defaults or Hardcoded Defaults (Lowest Priority)
- **定義**: 設定ファイル（`.cderun.yaml`）の `defaults` ブロック、またはプログラム内でハードコードされた最終フォールバック値。
- **デフォルト値:**
   - `tty: false`
   - `interactive: false`
   - `network: bridge`
   - `remove: true`
   - `runtime: docker`
   - `image`: なし (Fatal Error)
      - ※ P1〜P5のいずれでも解決できない場合、プログラムはエラーメッセージを出力して終了すること (Exit Code 1)。勝手なデフォルトイメージ（`ubuntu:latest` 等）を使用してはならない。
## 判定ロジックの実装要件

Viperの自動解決のみに頼らず、以下のロジックフローで値を解決する：

1. **CLI指定の確認 (P1, P2)**: `Changed` 状態を確認し、ユーザーの明示的な入力を最優先する。
2. **環境変数の確認 (P3)**: CLI指定がない場合、定義された環境変数の存在を確認する。
3. **ツール別設定の確認 (P4)**: `.tools.yaml` の設定を確認する。
4. **グローバルデフォルト・ハードコード値の確認 (P5)**: `.cderun.yaml` の `defaults` または最終的なフォールバック値を採用する。

## 注意点
- **明示的な未指定の扱い**: YAMLのフィールドはポインタ型（`*bool` 等）で定義し、「未設定（nil）」と「明示的なfalse」を区別できるようにする。
