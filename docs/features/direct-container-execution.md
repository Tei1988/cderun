# Feature: Direct Container Execution

## 概要

各ランタイムの独自APIを介して直接コンテナを実行する。
コマンド生成は行わず、中間表現（IR）を各ランタイムのAPIコールに変換する。

## アーキテクチャ

```
cderunフラグ → 中間表現（IR） → ランタイムAPIコール → コンテナ実行
                    ↓
               ContainerConfig
                    ↓
          runtime.CreateContainer()  ← Docker Engine API
          runtime.StartContainer()   ← Podman API
          runtime.AttachContainer()  ← containerd API
```

**メリット:**
- コマンド生成不要
- プログラマティックな制御
- エラーハンドリングが容易
- ネストした実行でも環境を引き継げる

## 中間表現（IR）

### 構造

```go
type ContainerConfig struct {
    // 基本設定
    Image       string
    Command     []string
    Args        []string
    
    // 実行オプション
    TTY         bool
    Interactive bool
    Remove      bool
    
    // ネットワーク
    Network     string
    
    // ボリューム
    Volumes     []VolumeMount
    
    // 環境変数
    Env         []string
    
    // 作業ディレクトリ
    Workdir     string
    
    // ユーザー
    User        string
}

type VolumeMount struct {
    HostPath      string
    ContainerPath string
    ReadOnly      bool
}
```

### 例

```go
config := ContainerConfig{
    Image:       "node:20-alpine",
    Command:     []string{"node"},
    Args:        []string{"app.js"},
    TTY:         true,
    Interactive: true,
    Remove:      true,
    Volumes: []VolumeMount{
        {
            HostPath:      "/home/user/project",
            ContainerPath: "/workspace",
            ReadOnly:      false,
        },
    },
    Env: []string{
        "NODE_ENV=development",
    },
    Workdir: "/workspace",
}
```

## CRIインターフェース

### 抽象化レイヤー

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

### Docker実装

```go
type DockerRuntime struct {
    client *client.Client
}

func (d *DockerRuntime) CreateContainer(ctx context.Context, config ContainerConfig) (string, error) {
    // ContainerConfigをDocker APIの形式に変換
    containerConfig := &container.Config{
        Image:        config.Image,
        Cmd:          append(config.Command, config.Args...),
        Tty:          config.TTY,
        OpenStdin:    config.Interactive,
        Env:          config.Env,
        WorkingDir:   config.Workdir,
        User:         config.User,
    }
    
    hostConfig := &container.HostConfig{
        AutoRemove:   config.Remove,
        NetworkMode:  container.NetworkMode(config.Network),
    }
    
    // ボリュームマウント
    for _, vol := range config.Volumes {
        mount := mount.Mount{
            Type:     mount.TypeBind,
            Source:   vol.HostPath,
            Target:   vol.ContainerPath,
            ReadOnly: vol.ReadOnly,
        }
        hostConfig.Mounts = append(hostConfig.Mounts, mount)
    }
    
    resp, err := d.client.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "")
    if err != nil {
        return "", err
    }
    
    return resp.ID, nil
}

func (d *DockerRuntime) StartContainer(ctx context.Context, containerID string) error {
    return d.client.ContainerStart(ctx, containerID, container.StartOptions{})
}

func (d *DockerRuntime) AttachContainer(ctx context.Context, containerID string, stdin io.Reader, stdout, stderr io.Writer) error {
    resp, err := d.client.ContainerAttach(ctx, containerID, container.AttachOptions{
        Stream: true,
        Stdin:  stdin != nil,
        Stdout: true,
        Stderr: true,
    })
    if err != nil {
        return err
    }
    defer resp.Close()
    
    // ストリームのコピー
    if stdin != nil {
        go io.Copy(resp.Conn, stdin)
    }
    
    _, err = stdcopy.StdCopy(stdout, stderr, resp.Reader)
    return err
}
```

### Podman実装

```go
type PodmanRuntime struct {
    conn *bindings.Connection
}

func (p *PodmanRuntime) CreateContainer(ctx context.Context, config ContainerConfig) (string, error) {
    // Podman APIを使った実装
    // 基本的にはDockerと同じだが、Podman固有のAPIを使用
}
```

## 実行フロー

### 基本的な実行

```go
func Run(config ContainerConfig, runtime ContainerRuntime) (int, error) {
    ctx := context.Background()
    
    // 1. イメージのプル（必要に応じて）
    if err := runtime.PullImage(ctx, config.Image); err != nil {
        return 1, err
    }
    
    // 2. コンテナの作成
    containerID, err := runtime.CreateContainer(ctx, config)
    if err != nil {
        return 1, err
    }
    
    // 3. クリーンアップの設定
    if config.Remove {
        defer runtime.RemoveContainer(ctx, containerID)
    }
    
    // 4. コンテナの起動
    if err := runtime.StartContainer(ctx, containerID); err != nil {
        return 1, err
    }
    
    // 5. アタッチ（TTY/Interactiveの場合）
    if config.TTY || config.Interactive {
        if err := runtime.AttachContainer(ctx, containerID, os.Stdin, os.Stdout, os.Stderr); err != nil {
            return 1, err
        }
    }
    
    // 6. 終了コードの取得
    info, err := runtime.InspectContainer(ctx, containerID)
    if err != nil {
        return 1, err
    }
    
    return info.ExitCode, nil
}
```

## ネストした実行の解決

### 問題の解決

CRIを直接使うことで、コンテナ内からcderunを実行しても、**同じランタイムインスタンスを使用**できる。

```go
// グローバルなランタイムインスタンス
var globalRuntime ContainerRuntime

func init() {
    // 環境変数でランタイムを共有
    socketPath := os.Getenv("DOCKER_HOST")
    if socketPath == "" {
        socketPath = "/var/run/docker.sock"
    }
    
    client, err := client.NewClientWithOpts(
        client.WithHost("unix://" + socketPath),
    )
    if err != nil {
        log.Fatal(err)
    }
    
    globalRuntime = &DockerRuntime{client: client}
}
```

### gemini-cliからの実行

```bash
# ホスト
$ cderun --mount-cderun gemini-cli

# gemini-cliコンテナ内
$ export MY_VAR=hello
$ cderun python -c "import os; print(os.getenv('MY_VAR'))"
```

**cderunの動作:**
1. 中間表現を作成
2. 環境変数`MY_VAR`を中間表現に追加
3. CRI経由でpythonコンテナを作成・起動
4. **gemini-cliコンテナの環境変数を引き継げる**

```go
func Run(config ContainerConfig, runtime ContainerRuntime) (int, error) {
    // 親コンテナの環境変数を引き継ぐ
    if inContainer() {
        parentEnv := os.Environ()
        config.Env = append(config.Env, parentEnv...)
    }
    
    // ... 実行
}
```

## 設定

### ランタイムの選択

```yaml
cderun:
  runtime: docker  # docker | podman
  runtimeSocket: /var/run/docker.sock
  
  # CRI設定
  cri:
    timeout: 30s
    pullPolicy: ifNotPresent  # always | ifNotPresent | never
```

### ツール設定

```yaml
tools:
  python:
    image: python:3.11-slim
    tty: true
    interactive: true
    volumes:
      - .:/workspace
    env:
      - PYTHONUNBUFFERED=1
    workdir: /workspace
```

## メリット

### 1. コマンド生成不要

```go
// 従来
cmd := fmt.Sprintf("docker run --rm -t -i -v %s:%s %s %s", ...)
exec.Command("sh", "-c", cmd).Run()

// 新方式
config := ContainerConfig{...}
runtime.CreateContainer(ctx, config)
runtime.StartContainer(ctx, containerID)
```

### 2. エラーハンドリング

```go
// 詳細なエラー情報が取得できる
if err := runtime.CreateContainer(ctx, config); err != nil {
    if errors.Is(err, ErrImageNotFound) {
        // イメージが見つからない
    } else if errors.Is(err, ErrInvalidConfig) {
        // 設定が不正
    }
}
```

### 3. 環境の引き継ぎ

```go
// 親コンテナの環境を引き継ぐ
if inContainer() {
    config.Env = append(config.Env, os.Environ()...)
}
```

### 4. プログラマティックな制御

```go
// コンテナの状態を監視
go func() {
    for {
        info, _ := runtime.InspectContainer(ctx, containerID)
        log.Printf("Status: %s, CPU: %.2f%%", info.State, info.CPUUsage)
        time.Sleep(1 * time.Second)
    }
}()
```

## 実装の優先順位

1. **Phase 1**: 中間表現の定義
2. **Phase 2**: Docker CRI実装
3. **Phase 3**: 基本的な実行フロー
4. **Phase 4**: 環境変数の引き継ぎ
5. **Phase 5**: Podman CRI実装
6. **Phase 6**: エラーハンドリングの強化

## 依存ライブラリ

```go
import (
    "github.com/docker/docker/client"
    "github.com/docker/docker/api/types/container"
    "github.com/docker/docker/api/types/mount"
)
```
