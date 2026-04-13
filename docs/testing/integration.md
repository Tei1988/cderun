# 統合テスト

## 概要

MockRuntime やファイルシステムとの連携を検証する。実際のコンテナランタイムが必要なテストでは `skipIfRuntimeBroken` でスキップする。

## テストの基本構造

`internal/command/integration_test.go` および `test_helpers_test.go` を使用する。

### テストライフサイクル

1. **セットアップ**: `setupTestDir` で一時ディレクトリを作成し、必要に応じて `.tools.yaml` / `.cderun.yaml` を配置する。
2. **実行**: `runCderun(args ...string)` で `cderun` のエントリーポイントをプロセス内で直接呼び出す。`os.Pipe` で stdout/stderr をキャプチャする。
3. **検証**: `testify/assert` や `testify/require` で出力・終了コードを検証する。
4. **クリーンアップ**: `t.Cleanup` でカレントディレクトリの復元や一時ファイルの削除を行う。

### ヘルパー関数

- `runCderun(args ...string) (stdout, stderr string, exitCode int, err error)`
- `setupTestDir(t *testing.T) string`
- `skipIfRuntimeBroken(t *testing.T, err error)`

## 主なテストシナリオ

- **基本的なコマンド実行**: `cderun <image> <command>` が正しく実行され、期待した出力が得られること。
- **ボリュームマウント**: ホストのファイル/ディレクトリがコンテナ内の指定パスにマウントされていること。
- **環境変数**: `--env` で指定した環境変数がコンテナ内に正しく設定されていること。
- **ポートマッピング**: `-p` で指定したポートが正しくフォワードされること。
- **cderun Expressions**: `{{file:.go-version}}`, `{{PWD}}`, `{{HOME}}` が正しく解決されること。
- **相対パスとチルダ展開**: 設定ファイル内のパスが期待通りに絶対パスに解決されること。
- **ドライラン**: `--dry-run` の出力にコンテナ設定が正しく反映されていること。
- **ポリグロット実行**: シンボリックリンク経由での実行が正しくイメージマッピングされること。
