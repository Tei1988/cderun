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

現在の CI（[`ci.yaml`](../../.github/workflows/ci.yaml)）で実行されているランタイムテストは以下のとおり。

### containerd 統合ジョブ（`runtime-test-containerd`）

- containerd（バージョン固定・sha256 検証付き）と CNI プラグインを runner にインストールし、systemd で起動する
- ソケットに ACL を設定し、非 root で接続可能にする
- `CDERUN_RUNTIME=containerd` / `CDERUN_SOCKET_PATH=/run/containerd/containerd.sock` を設定して `go test ./internal/runtime/...` を実行する

### 今後の拡張（`.agent/todo.md` を参照）

- **T20**: Docker / Podman のランタイムテストジョブの追加。`ubuntu-latest` は Docker が標準搭載のため `/var/run/docker.sock` がそのまま使える。Podman は `podman.socket` の有効化が必要
- **T70**: ランタイム間のコンフォーマンススイートを CI ジョブの器として実装する

なお、複数の Docker バージョンを DinD でマトリックス検証する構成は、コスト対効果の観点から採用しない（cderun は Docker API の安定した部分のみを使用しており、バージョン間差異のリスクが小さいため）。

### CI 環境特有のパス解決

ランタイムのデーモンがテストプロセスと異なるファイルシステム名前空間で動く環境（DinD 等）では、バインドマウントのソースパスがデーモン側から解決できない場合がある。

- 一時ディレクトリは環境変数 `TEST_HOST_TMP_DIR` でベースパスを切り替え可能にしている（`internal/command/scenario_test.go`）
