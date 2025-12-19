# Feature: Argument & Configuration Priority Logic

## 概要
`cderun` は、複数のソース（CLI、環境変数、YAML、デフォルト値）から設定を読み込む。
設定の競合が発生した場合、以下の **P1（最高）〜 P5（最低）** の優先順位に従って値を確定させる。

## 優先順位階層 (Resolution Hierarchy)

### P1: CDERUN Override Flags (Highest Priority)
- **定義**: 動作を強制的に変更するための専用フラグ。
- **フラグ名**: `--cderun-tty`, `--cderun-interactive`
- **挙動**: これらが指定された場合、他の全て（P2〜P5）を無視してこの値を採用する。

### P2: Standard CLI Flags (User Intent)
- **定義**: 実行時にユーザーが明示的に指定した標準フラグ。
- **フラグ名**: `--tty`, `--interactive`, `--image`, `--entrypoint`
- **判定条件**: `cmd.Flags().Changed(name)` が `true` であること。
  - ※ ユーザーがフラグを入力していない場合、Cobraが持つデフォルト値は無視し、P3以下の判定へ進むこと。

### P3: Environment Variables (Global Override)
- **定義**: 実行環境全体に適用される設定。
- **キー**: `CDERUN_TTY`, `CDERUN_INTERACTIVE`, `CDERUN_IMAGE` 等。
- **挙動**: CLIでの指定がない場合、環境変数の値を確認する。設定されていればそれを採用する。

### P4: Command-Specific Configuration (YAML Profile)
- **定義**: 設定ファイル（`config.yaml`）内の、実行対象サブコマンドに紐づく設定ブロック。
- **挙動**: CLIも環境変数も指定がない場合、このプロファイル値を「そのコマンドのデフォルト」として採用する。

```yaml
# config.yaml (P4 Source)
gemini:
  interactive: true  # P4 value
  tty: true          # P4 value
```

### P5: Hardcoded Defaults (Lowest Priority)
- **定義**: プログラム内でハードコードされた最終フォールバック値。
- **値:**
   - `tty: false`
   - `interactive: false`
   - `image`: なし (Fatal Error)
      - ※ P1〜P4のいずれでも解決できない場合、プログラムはエラーメッセージを出力して終了すること (Exit Code 1)。勝手なデフォルトイメージ（`ubuntu:latest` 等）を使用してはならない。 
## 判定ロジックの実装要件
Julesは、Viperの自動解決のみに頼らず、以下のロジックフローで値を解決するヘルパー関数を実装すること。

```go
// 疑似コード: 値解決の流れ
func resolveBool(flagName, envName string, configVal *bool, defaultVal bool) bool {
    // 1. P1 & P2: CLI指定があるかチェック
    if cmd.Flags().Changed(flagName) {
        return cmd.Flags().GetBool(flagName)
    }
    
    // 2. P3: 環境変数が設定されているかチェック
    if val, set := os.LookupEnv(envName); set {
        return parseBool(val)
    }

    // 3. P4: YAML設定があるかチェック (nilチェック)
    if configVal != nil {
        return *configVal
    }

    // 4. P5: デフォルト
    return defaultVal
}
```

## 注意点
- YAMLのポインタ扱い: YAMLから読み込む構造体のフィールド（`Interactive`, `Tty` 等）は、`bool` ではなく `*bool` (ポインタ) として定義すること。これにより、「YAMLに記述がない(nil)」と「falseと記述されている」を明確に区別する。
