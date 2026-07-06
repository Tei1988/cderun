# ランタイムテスト

## 概要

実際のコンテナランタイム（Docker、Podman、または containerd）を使用して、システム全体の動作を検証する。
MockRuntime を使用した統合テストではカバーできない、実際のデバイスマウント、バイナリの実行、コンテナのライフサイクル、OSシグナルの挙動などを実環境で保証する。

## テストの分類

| ファイル | 実行方式 | 用途 |
| --- | --- | --- |
| `runtime_inprocess_test.go` | インプロセス実行 + 実ランタイム | 実コンテナを使った動作検証 |
| `scenario_test.go` | バイナリ実行 + 実ランタイム | ビルド済みバイナリのシナリオ検証 |
| `scenario_device_test.go` | バイナリ実行 + 実ランタイム | デバイスマウント等の特殊シナリオ |

## 実行方法

```bash
go test -v -tags=runtime ./...
# または
make test-runtime
```

## 実装ガイドライン

### ビルドタグ

ランタイムテストファイルの先頭に必ず付与する。

```go
//go:build runtime
```

### 命名規則

- インプロセス実行: `TestRuntime_` で始めること
- バイナリ実行（シナリオ）: `TestScenario_` で始めること

### テストヘルパー

- `runCderun(args ...string)`: `cderun` をインプロセスで実行する（`runtime_inprocess_test.go` 用）
- `runCderunE2E`: サブコマンドの指定を強制するヘルパー（`scenario_test.go` 用）
- `skipIfRuntimeBroken(t, err)`: ランタイム環境が利用できない場合にスキップする
- `findCderunBinary`: プロジェクトルートのビルド済みバイナリを動的に解決する（シナリオテスト用）

### サブコマンドの必須指定

`cderun` は `cderun [cderun-flags] <subcommand> [command-options]` という構造を持つ。
テスト内では **必ず `<subcommand>` を明示的に指定すること**。

### クリーンアップ

一時ディレクトリや設定ファイルは `t.TempDir()` や `t.Cleanup()` で確実に削除すること。

## CI 構成

### Docker バージョンマトリックス

`docker:dind` を GitHub Actions のサービスコンテナとして使用し、以下の3世代で検証する。

- **20.10**: レガシー環境との互換性
- **25.0**: 現在広く普及しているバージョン
- **29.0**: このマトリックスでの最新対象

### ジョブ構成

1. **Build ジョブ**: バイナリを1回だけビルドし Artifacts に保存。
2. **Unit Test ジョブ**: マトリックス外の単一ジョブで実行。
3. **Runtime Test ジョブ**: 保存済みバイナリをダウンロードして使用。`-run "^TestScenario_"` でフィルタリング。

### CI 環境特有のパス解決

GitHub Actions の DinD 環境では、Runner と Docker デーモンが動作する DinD コンテナは別物である。

- Runner のワークスペース (`/home/runner/work`) を DinD コンテナの同じパスにマウントしている。
- 一時ディレクトリは環境変数 `TEST_HOST_TMP_DIR` でベースパスを切り替え可能にしている。
