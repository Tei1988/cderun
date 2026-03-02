# E2E テスト

## 概要

実際のコンテナランタイム（Docker または Podman）を使用して、システム全体の動作をエンドツーエンドで検証する。
ユニットテストや MockRuntime を使用した統合テストではカバーできない、実際のデバイスマウント、バイナリの実行、コンテナのライフサイクル、OSシグナルの挙動などを実環境で保証する。

## 実行方法

```bash
go test -v -tags=e2e ./...
# または
make test-e2e
```

## 実装ガイドライン

### ビルドタグ

E2E テストファイルの先頭に必ず付与する。

```go
//go:build e2e
```

### 命名規則

`TestScenario_` で始めること（CI でのフィルタリングのため）。

### テストヘルパー

`internal/command/test_helpers_test.go` のヘルパー関数を使用する。

- `runCderun(args ...string)`: `cderun` をプロセス内で実行し、stdout/stderr/終了コードを返す。
- `runCderunE2E`: サブコマンドの指定を強制するヘルパー。
- `skipIfDockerBroken(t, err)`: ランタイム環境が利用できない場合にスキップする。
- `findCderunBinary`: プロジェクトルートのビルド済みバイナリを動的に解決する（ネスト実行テスト用）。

### サブコマンドの必須指定

`cderun` は `cderun [cderun-flags] <subcommand> [command-options]` という構造を持つ。
テスト内では **必ず `<subcommand>` を明示的に指定すること**。欠落するとコンテナの引数がサブコマンドとして誤認される。

`cderun` は `--` セパレータをサポートしていない。引数は直接追加する形式をとること。

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
3. **E2E Test ジョブ**: 保存済みバイナリをダウンロードして使用。`-run "^TestScenario_"` でフィルタリング。

### CI 環境特有のパス解決

GitHub Actions の DinD 環境では、Runner と Docker デーモンが動作する DinD コンテナは別物である。

- Runner のワークスペース (`/home/runner/work`) を DinD コンテナの同じパスにマウントしている。
- 一時ディレクトリは環境変数 `TEST_HOST_TMP_DIR` でベースパスを切り替え可能にしている。

## 主要なテストシナリオ

- **デバイスマウント**: `--device` フラグによるホストデバイスのマウントと、コンテナ内からのアクセス。
- **実際のバイナリ実行**: 特定のイメージ（Alpine, Node.js 等）を使用した実行と出力の検証。
- **ネスト実行**: ホストのソケットとバイナリをマウントし、コンテナ内からさらにコンテナを起動する一連のフロー。
- **診断モード**: `TestScenario_DockerVersion` で `--diagnosis` を実行し、対象の Docker バージョンをログに残す。
