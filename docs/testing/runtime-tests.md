# ランタイムテスト

## 概要

実際のコンテナランタイム（Docker、Podman、または containerd）を使用して、システム全体の動作を検証する。
実デーモンによるイメージ取得とコンテナの作成・起動・待機・状態確認・削除、および CLI 経由の入出力、デバイス、マウント、ネスト実行を確認する。
設定やドライランの検証、モックのコンフォーマンスは通常の `make test` で実行する。

## テストの分類

| ファイル | 実行方式 | 用途 |
| --- | --- | --- |
| `runtime_inprocess_test.go` | インプロセス実行 + 実ランタイム | 実コンテナを使った動作検証 |
| `scenario_test.go` | CLI 経由 + 実ランタイム | コマンド実行、マウント、ビルド済みバイナリでのネスト実行 |
| `scenario_device_test.go` | CLI 経由 + 実ランタイム | デバイスマウントとパイプで渡した標準入力 |
| `feature_runtime_live_conformance_test.go` | 実ソケット + 実ランタイム | イメージ取得とコンテナのライフサイクル |

## 実行方法

対象のデーモンを起動し、テスト実行ユーザーでソケットに接続できることを確認する。
CI と同じように Alpine 内で動くバイナリを作るため、リポジトリルートで
`CGO_ENABLED=0 make build` を実行してから、対象エンジンのコマンドを実行する。

```bash
CDERUN_ENGINE=docker CDERUN_SOCKET_PATH=/var/run/docker.sock \
  make test-runtime

CDERUN_ENGINE=podman CDERUN_SOCKET_PATH="/run/user/$(id -u)/podman/podman.sock" \
  make test-runtime

CDERUN_ENGINE=containerd CDERUN_SOCKET_PATH=/run/containerd/containerd.sock CDERUN_NETWORK=host \
  make test-runtime
```

CI と `make test-runtime` は同じ [スクリプト](../../scripts/test-runtime.sh) を使用する。
ライフサイクルのサブテストと CLI の `TestRuntime_` テストを、キャッシュを無効にして実行する。

## 実装ガイドライン

### ビルドタグ

ランタイムテストファイルの先頭に必ず付与する。

```go
//go:build runtime
```

### 命名規則

- 実ランタイムを使う CLI テストは `TestRuntime_` で始めること。

### テストヘルパー

- `runCderun(args ...string)`: `cderun` をインプロセスで実行する（`runtime_inprocess_test.go` 用）
- `runCderunE2E`: サブコマンドの指定を強制するヘルパー（`scenario_test.go` 用）
- `skipIfRuntimeBroken(t, err)`: ランタイム環境の既知の制約でスキップする
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
  テストバイナリは `sudo` で実行する。containerd クライアントがイメージのユーザー情報を解決する際にローカルでスナップショットをマウントするため。

なお、複数の Docker バージョンを DinD でマトリックス検証する構成は、コスト対効果の観点から採用しない（cderun は Docker API の安定した部分のみを使用しており、バージョン間差異のリスクが小さいため）。

### CI 環境特有のパス解決

ランタイムのデーモンがテストプロセスと異なるファイルシステム名前空間で動く環境（DinD 等）では、バインドマウントのソースパスがデーモン側から解決できない場合がある。

- 一時ディレクトリは環境変数 `TEST_HOST_TMP_DIR` でベースパスを切り替え可能にしている（`internal/command/scenario_test.go`）
