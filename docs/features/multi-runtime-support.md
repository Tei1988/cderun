# Feature: Multi-Runtime Support (Completed)

## 概要

Docker以外のコンテナランタイム（Podman等）をサポートする。
共通の`ContainerRuntime`インターフェースを定義し、各ランタイムの独自APIをラップする。

## サポートされるランタイム

### 優先度1: Docker (Completed)
- デフォルトのランタイム
- 最も広く使われている
- Docker Engine APIを使用

### 優先度2: Podman (Completed)
- Dockerのドロップイン代替
- rootlessコンテナのサポート
- Podman APIを使用（Docker互換）

### 将来的な拡張
- nerdctl（containerdのCLI、Dockerの代替）

## アーキテクチャ

### 抽象化レイヤー

cderun独自の`ContainerRuntime`インターフェースを定義し、各ランタイムの独自APIをラップする。

```text
cderun ContainerRuntimeインターフェース
  │
  ├── DockerRuntime → Docker Engine API (HTTP over Unix socket)
  ├── PodmanRuntime → Podman API (HTTP over Unix socket)
  └── NerdctlRuntime → containerd API (gRPC)
```

### 共通インターフェースの役割

`ContainerRuntime` インターフェースは、以下の主要な責務を持つ：
- **ライフサイクル管理**: コンテナの作成、起動、終了待機、削除。
- **IO接続**: コンテナの標準入出力へのアタッチ（TTYサポート含む）。
- **メタデータ提供**: ランタイム名の識別。
- **操作**: コンテナへのシグナル送信、TTYリサイズ。

## ランタイムの選択

**現状 (Phase 4):**
Docker と Podman をフルサポートしています。Podman は Docker 互換の API を介してサポートされており、ランタイムとソケットの選択は、設定ファイル、環境変数、またはコマンドライン引数によって明示的に指定可能です。

### 解決ロジック (Completed)

1. **設定ファイル**: `.cderun.yaml` の `runtime` フィールド。
2. **環境変数**: `CDERUN_RUNTIME`, `CDERUN_SOCKET_PATH` 等。
3. **コマンドライン引数**: `--runtime`, `--socket-path` および P1 内部オーバーライド。

### 自動検出ロジック (Completed)

ソケットの存在確認によるランタイムの自動選択機能。

1. `--runtime` または `CDERUN_RUNTIME` が指定されている場合はそれを使用。
2. 指定がない場合、以下のデフォルトパスを順に確認し、最初に見つかったものを使用。
  - `/var/run/docker.sock` (Runtime: `docker`)
  - `/run/podman/podman.sock` (Runtime: `podman`)
3. いずれも見つからない場合は `docker` をデフォルトとし、`/var/run/docker.sock` を使用（実行時にエラーとなる可能性がある）。

### 明示的な指定 (Completed)

#### 設定ファイル (`.cderun.yaml`)
```yaml
runtime: podman
```

#### 環境変数
```bash
export CDERUN_RUNTIME=podman
export CDERUN_SOCKET_PATH=/run/podman/podman.sock
cderun node app.js
```

#### コマンドライン
```bash
cderun --runtime podman node app.js
```

## ランタイム固有の実装ポイント

- **Docker**: `github.com/docker/docker/client` を使用し、Unixソケット経由で接続。APIバージョンの自動ネゴシエーションを有効化。
- **Podman**: Docker 互換の API を使用。Docker クライアントライブラリを共通の基盤として利用し、Podman の Unix ソケット経由で接続。

## ランタイム情報の表示 (Completed)

### 現在のランタイム確認
サブコマンドを指定せずに `--dry-run` を実行することで、診断情報を表示できます。詳細は[ドライランモード](./dry-run-mode.md)を参照してください。

```bash
cderun --dry-run
```


## 拡張性

### 新しいランタイムの追加手順
1. `ContainerRuntime` インターフェースを実装する新しい構造体を作成。
2. `internal/command/root.go` の `runtimeFactory` に新しいランタイムを登録。
3. 設定ファイルや環境変数で新しいランタイムを選択可能にする。

## 依存ライブラリ

### Docker
```go
import (
  "github.com/docker/docker/client"
)
```
