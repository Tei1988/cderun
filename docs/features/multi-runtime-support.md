# Feature: Multi-Runtime Support

## 概要

Docker以外のコンテナランタイム（Podman等）をサポートする。
共通の`ContainerRuntime`インターフェースを定義し、各ランタイムの独自APIをラップする。

## サポートされるランタイム

### 優先度1: Docker
- デフォルトのランタイム
- 最も広く使われている
- Docker Engine APIを使用

### 優先度2: Podman (Phase 5予定)
- Dockerのドロップイン代替
- rootlessコンテナのサポート
- Podman APIを使用（Docker互換）

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

## ランタイムの選択

**現状 (Phase 1):**
現在は Docker のみをサポートしており、ランタイムの自動検出は行われません。デフォルトで `/var/run/docker.sock` を使用し、`--mount-socket` フラグでパスを明示的に変更可能です。

### 自動検出ロジック (Phase 5予定)

1. **設定ファイル**: `.cderun.yaml` 等で `runtime` が指定されているか。
2. **環境変数**: `CDERUN_RUNTIME` が設定されているか。
3. **ソケット検索**: デフォルトのソケットパス（`/var/run/docker.sock`, `/run/podman/podman.sock` 等）が存在するかを順に確認。

### 明示的な指定 (Phase 2 Completed)

#### 設定ファイル
```yaml
cderun:
  runtime: podman
  runtimeSocket: /run/podman/podman.sock
```

#### 環境変数
```bash
export CDERUN_RUNTIME=podman
export DOCKER_HOST=unix:///run/podman/podman.sock
cderun node app.js
```

#### コマンドライン
```bash
cderun --runtime podman node app.js
```

## ランタイム固有の実装ポイント

- **Docker**: `github.com/docker/docker/client` を使用し、Unixソケット経由で接続。APIバージョンの自動ネゴシエーションを有効化。
- **Podman (Phase 5予定)**: `github.com/containers/podman/v4/pkg/bindings` を使用。Docker互換APIを提供しているため、基本的な構造はDockerと同様。

## ランタイム情報の表示 (Phase 4予定)

### 現在のランタイム確認
```bash
$ cderun --version
cderun version 0.1.0
Runtime: docker 24.0.7
Socket: /var/run/docker.sock
```

```bash
$ cderun runtime info
Runtime: docker
Socket: /var/run/docker.sock
Version: 24.0.7
Available: true
```

### 利用可能なランタイム一覧
```bash
$ cderun runtime list
Available runtimes:
  * docker  (/var/run/docker.sock) - version 24.0.7
    podman  (/run/podman/podman.sock) - version 4.8.0
    
* = currently selected
```

## エラーハンドリング

### ランタイムが見つからない
```bash
$ cderun node app.js
Error: No container runtime found
Please install Docker or Podman, or specify a runtime socket in configuration
```

### 指定されたランタイムが利用不可
```bash
$ cderun --runtime podman node app.js
Error: Runtime 'podman' is not available
Socket '/run/podman/podman.sock' not found
Available runtimes: docker
```

### バージョン互換性チェック (Phase 4予定)
各ランタイムの `ServerVersion` APIを呼び出し、必要な最小バージョンを満たしているか確認。

## 拡張性

### 新しいランタイムの追加手順
1. `ContainerRuntime` インターフェースを実装する新しい構造体を作成。
2. 内部のランタイムファクトリーまたはレジストリに新しいランタイムを登録。
3. 設定ファイルや自動検出ロジックで新しいランタイムを選択可能にする。

## 依存ライブラリ

### Docker
```go
import (
    "github.com/docker/docker/client"
)
```

### Podman
```go
import (
    "github.com/containers/podman/v4/pkg/bindings"
)
```
