# Feature: Multi-Runtime Support

## 概要

Docker以外のコンテナランタイム（Podman等）をサポートする。
共通の`ContainerRuntime`インターフェースを定義し、各ランタイムの独自APIをラップする。

## サポートされるランタイム

### 優先度1: Docker
- デフォルトのランタイム
- 最も広く使われている
- Docker Engine APIを使用

### 優先度2: Podman
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

```go
type ContainerRuntime interface {
    // コンテナのライフサイクル
    CreateContainer(ctx context.Context, config ContainerConfig) (string, error)
    StartContainer(ctx context.Context, containerID string) error
    StopContainer(ctx context.Context, containerID string, timeout time.Duration) error
    RemoveContainer(ctx context.Context, containerID string) error
    
    // コンテナとの通信
    AttachContainer(ctx context.Context, containerID string, stdin io.Reader, stdout, stderr io.Writer) error
    ExecInContainer(ctx context.Context, containerID string, cmd []string) (int, error)
    
    // 情報取得
    InspectContainer(ctx context.Context, containerID string) (*ContainerInfo, error)
    ListContainers(ctx context.Context) ([]ContainerInfo, error)
    
    // イメージ操作
    PullImage(ctx context.Context, image string) error
    ListImages(ctx context.Context) ([]ImageInfo, error)
}
```

## ランタイムの選択

### 自動検出
1. 設定ファイルで指定されている場合、それを使用
2. 環境変数`CDERUN_RUNTIME`が設定されている場合、それを使用
3. ソケットから利用可能なランタイムを検索（docker → podman の順）

```go
func DetectRuntime() (ContainerRuntime, error) {
    // 1. 設定ファイル
    if config.Runtime != "" {
        return NewRuntime(config.Runtime, config.RuntimeSocket)
    }
    
    // 2. 環境変数
    if runtime := os.Getenv("CDERUN_RUNTIME"); runtime != "" {
        return NewRuntime(runtime, "")
    }
    
    // 3. 自動検出
    if socketExists("/var/run/docker.sock") {
        return NewDockerRuntime("/var/run/docker.sock"), nil
    }
    if socketExists("/run/podman/podman.sock") {
        return NewPodmanRuntime("/run/podman/podman.sock"), nil
    }
    
    return nil, errors.New("no container runtime found")
}
```

### 明示的な指定

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

## ランタイム固有の実装

### Docker実装
```go
type DockerRuntime struct {
    client *client.Client
    socket string
}

func NewDockerRuntime(socket string) (*DockerRuntime, error) {
    client, err := client.NewClientWithOpts(
        client.WithHost("unix://" + socket),
        client.WithAPIVersionNegotiation(),
    )
    if err != nil {
        return nil, err
    }
    
    return &DockerRuntime{
        client: client,
        socket: socket,
    }, nil
}
```

### Podman実装
```go
type PodmanRuntime struct {
    conn   *bindings.Connection
    socket string
}

func NewPodmanRuntime(socket string) (*PodmanRuntime, error) {
    conn, err := bindings.NewConnection(
        context.Background(),
        "unix://" + socket,
    )
    if err != nil {
        return nil, err
    }
    
    return &PodmanRuntime{
        conn:   conn,
        socket: socket,
    }, nil
}
```

## ランタイム情報の表示

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

### バージョン互換性チェック
```go
func (d *DockerRuntime) IsAvailable() bool {
    ctx := context.Background()
    version, err := d.client.ServerVersion(ctx)
    if err != nil {
        return false
    }
    
    // 最小バージョンチェック
    minVersion := "20.10.0"
    return compareVersion(version.Version, minVersion) >= 0
}
```

## 拡張性

### 新しいランタイムの追加
1. `ContainerRuntime`インターフェースを実装
2. ランタイムファクトリーに登録
3. 設定ファイルで使用可能に

```go
// 新しいランタイムの実装
type NerdctlRuntime struct {
    socket string
}

func (n *NerdctlRuntime) CreateContainer(ctx context.Context, config ContainerConfig) (string, error) {
    // nerdctl固有の実装
}

// ファクトリーに登録
func init() {
    RegisterRuntime("nerdctl", func(socket string) (ContainerRuntime, error) {
        return NewNerdctlRuntime(socket)
    })
}
```

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
