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

対象のデーモンを起動し、テスト実行ユーザーでソケットに接続できることを確認する。
CLI シナリオ用にリポジトリルートで `make build` を実行してから、対象エンジンのコマンドを実行する。

```bash
CDERUN_ENGINE=docker CDERUN_SOCKET_PATH=/var/run/docker.sock \
  go test -count=1 -tags=runtime -v ./internal/runtime/... ./internal/command/...

CDERUN_ENGINE=podman CDERUN_SOCKET_PATH="/run/user/$(id -u)/podman/podman.sock" \
  go test -count=1 -tags=runtime -v ./internal/runtime/... ./internal/command/...

CDERUN_ENGINE=containerd CDERUN_SOCKET_PATH=/run/containerd/containerd.sock CDERUN_NETWORK=host \
  go test -count=1 -tags=runtime -v ./internal/runtime/... ./internal/command/...
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

- 3つのジョブは GitHub ホストの Ubuntu 24.04 VM で動作し、フォークからの PR ではスキップされる。
- Docker は短い一時パスを使い、テスト前にデーモンからバインドマウントできることを確認する。
- Podman は rootless サービスを起動し、ソケット経由で API が応答してからテストを開始する。
- containerd は専用の状態ディレクトリで起動し、`/run/cderun-containerd/containerd.sock` へのアクセスを ACL で許可する。

なお、複数の Docker バージョンを DinD でマトリックス検証する構成は、コスト対効果の観点から採用しない（cderun は Docker API の安定した部分のみを使用しており、バージョン間差異のリスクが小さいため）。

### CI 環境特有のパス解決

ランタイムのデーモンがテストプロセスと異なるファイルシステム名前空間で動く環境（DinD 等）では、バインドマウントのソースパスがデーモン側から解決できない場合がある。

- 一時ディレクトリは環境変数 `TEST_HOST_TMP_DIR` でベースパスを切り替え可能にしている（`internal/command/scenario_test.go`）
