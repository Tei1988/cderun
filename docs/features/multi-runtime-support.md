# Feature: Multi-Runtime Support (Phase 4 In Progress)

## 概要

Docker以外のコンテナランタイム（Podman等）をサポートする。
共通の`ContainerRuntime`インターフェースを定義し、各ランタイムの独自APIをラップする。

## サポートされるランタイム

### 優先度1: Docker (Completed)
- デフォルトのランタイム
- 最も広く使われている
- Docker Engine APIを使用

### 優先度2: Podman (Phase 4予定 - In Progress)
- Dockerのドロップイン代替
- rootlessコンテナのサポート
- Podman APIを使用（Docker互換）
- 現在はスタブ実装のみ

### 将来的な拡張
- nerdctl（containerdのCLI、Dockerの代替）

## アーキテクチャ

### 抽象化レイヤー

cderun独自の`ContainerRuntime`インターフェースを定義し、各ランタイムの独自APIをラップする。

```
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

**現状 (Phase 4 In Progress):**
Docker をフルサポートし、Podman はスタブ実装（"not implemented yet" エラーを返す状態）として準備されています。ランタイムとソケットの選択は、設定ファイル、環境変数、またはコマンドライン引数によって明示的に指定可能です。

### 解決ロジック (Completed)

1. **設定ファイル**: `.cderun.yaml` の `runtime` フィールド。
2. **環境変数**: `CDERUN_RUNTIME`, `CDERUN_MOUNT_SOCKET` 等。
3. **コマンドライン引数**: `--runtime`, `--mount-socket` および P1 内部オーバーライド。

### 自動検出ロジック (Future Phase)

ソケットの存在確認によるランタイムの自動選択機能は将来のフェーズで実装予定です。

1. デフォルトのソケットパス（`/var/run/docker.sock`, `/run/podman/podman.sock` 等）が存在するかを順に確認。

### 明示的な指定 (Completed)

#### 設定ファイル (`.cderun.yaml`)
```yaml
runtime: podman
runtimePath: /usr/bin/podman
```

#### 環境変数
```bash
export CDERUN_RUNTIME=podman
export CDERUN_MOUNT_SOCKET=/run/podman/podman.sock
cderun node app.js
```

#### コマンドライン
```bash
cderun --runtime podman node app.js
```

## ランタイム固有の実装ポイント

- **Docker**: `github.com/docker/docker/client` を使用し、Unixソケット経由で接続。APIバージョンの自動ネゴシエーションを有効化。
- **Podman (Phase 4予定)**: Podman API を使用予定。現在は初期スタブ実装のみ。

## ランタイム情報の表示 (Planned)

### 現在のランタイム確認
```bash
$ cderun debug info
...
Runtime: docker 24.0.7
Socket: /var/run/docker.sock
```

## エラーハンドリング

### 指定されたランタイムが利用不可
```bash
$ cderun --runtime podman node app.js
Error: podman runtime is not implemented yet
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
