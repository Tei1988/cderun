# 機能仕様：マルチランタイムサポート (完了)

## 概要

Docker以外のコンテナランタイム（Podman等）をサポートする。
共通の`ContainerRuntime`インターフェースを定義し、各ランタイムの独自APIをラップする。

## サポートされるランタイム

### 優先度1: Docker (完了)

- デフォルトのランタイム
- 最も広く使われている
- Docker Engine APIを使用

### 優先度2: Podman (完了)

- Dockerのドロップイン代替
- rootlessコンテナのサポート
- Podman APIを使用（Docker互換）

### 将来的な拡張

- **containerd (Planned / In-progress)**:
  ネイティブの containerd API (gRPC) を直接利用したサポートを計画しています。
  - **開発ステータス**: 内部的な設定バリデータ（`resolver.go`）では将来の対応を見越して `containerd` を有効な値として受け入れますが、**現時点ではランタイムの実装が完了していないため（現在開発中）、指定しても実行時にエラーとなります。**
  - **目的**: Docker/Podman デーモンを経由しない、より軽量なコンテナ実行の実現。
- **nerdctl (Backlog)**:
  containerd の CLI である `nerdctl` をラップして実行する方式の検討。

## アーキテクチャ

### 抽象化レイヤー

cderun独自の`ContainerRuntime`インターフェースを定義し、各ランタイムの独自APIをラップする。

```text
cderun ContainerRuntimeインターフェース
        │
        ├── DockerRuntime     → Docker Engine API (HTTP over Unix socket) [Implemented]
        ├── PodmanRuntime     → Supported via Docker API compatibility (uses DockerRuntime)
        └── ContainerdRuntime → containerd API (gRPC) [Planned / In-progress]
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

### 解決ロジック (完了)

1. **設定ファイル**: `.cderun.yaml` の `runtime` フィールド。
2. **環境変数**: `CDERUN_RUNTIME`, `CDERUN_SOCKET_PATH` 等。
3. **コマンドライン引数**: `--runtime`, `--socket-path` および P1 内部オーバーライド。

### 自動検出ロジック (完了)

ソケットの存在確認によるランタイムの自動選択機能。

1. `--runtime` または `CDERUN_RUNTIME` が指定されている場合はそれを使用。
2. 指定がない場合、以下のデフォルトパスを順に確認し、最初に見つかったものを使用。
   - `/var/run/docker.sock` (Runtime: `docker`)
   - `/run/podman/podman.sock` (Runtime: `podman`)
3. いずれも見つからない場合は `docker` をデフォルトとし、`/var/run/docker.sock` を使用（実行時にエラーとなる可能性がある）。

### 明示的な指定 (完了)

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
- **Podman**: Docker 互換の API を使用。 Docker クライアントライブラリを共通の基盤として利用し、Podman の Unix ソケット経由で接続。
- **イメージプルのリトライ**: ネットワークの不安定さやレート制限（`toomanyrequests` 等）に対応するため、指数バックオフを伴うリトライロジック（最大3回）を実装。

## ランタイム情報の表示 (完了)

### 現在のランタイム確認

`--diagnosis` フラグを使用することで、現在のランタイム設定や接続状態を含む診断情報を表示できます。詳細は[診断モード](./diagnosis-mode.md)を参照してください。

```bash
cderun --diagnosis
```

診断情報の出力フォーマットは `--diagnosis-format` で指定可能です（`yaml`, `json`, `simple`）。

```bash
cderun --diagnosis --diagnosis-format simple
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
