# cderun 実装ガイド

## 概要

このドキュメントは`cderun`の実装を段階的に進めるためのガイドです。
各ステップは依存関係を考慮して順序付けられており、前のステップが完了してから次に進みます。

## 現在の実装状況

### 実装済み
- 基本的なCLI構造（Cobra使用）
- 引数解析の基礎（`--tty`, `--interactive`, `--network`, `--mount-socket`, `--mount-cderun`）
- ポリグロットエントリーポイント（シンボリックリンク検出）

### 未実装
- 実際のコンテナ実行
- 設定ファイル読み込み
- イメージマッピング
- 環境変数処理
- その他の高度な機能

## 実装フェーズ

### Phase 1: コア機能（必須）
基本的なコンテナ実行機能を実装

### Phase 2: 設定管理
設定ファイルとイメージマッピング

### Phase 3: 高度な機能
環境変数、マウント機能など

### Phase 4: 利便性向上
ドライラン、ログなど

---

## Phase 1: コア機能

### Step 1.1: 中間表現（ContainerConfig）の定義

**目的**: すべての設定を統一的に扱うための中間表現を定義

**参照ドキュメント**:
- `docs/features/direct-container-execution.md`
- `docs/features/container-runtime-abstraction.md`

**実装内容**:

1. `internal/container/config.go`を作成

```go
package container

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
    
    // 環境変数（["KEY=value", "KEY2=value2"]形式）
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

**テスト**:
- 構造体の初期化テスト
- デフォルト値の確認

**完了条件**:
- `ContainerConfig`構造体が定義されている
- 基本的なテストが通る

---

### Step 1.2: Dockerランタイムインターフェースの定義

**目的**: コンテナランタイムの抽象化レイヤーを定義

**参照ドキュメント**:
- `docs/features/multi-runtime-support.md`
- `docs/features/direct-container-execution.md`

**実装内容**:

1. `internal/runtime/interface.go`を作成

```go
package runtime

import (
    "context"
    "io"
    "time"
)

type ContainerRuntime interface {
    // コンテナのライフサイクル
    CreateContainer(ctx context.Context, config *container.ContainerConfig) (string, error)
    StartContainer(ctx context.Context, containerID string) error
    WaitContainer(ctx context.Context, containerID string) (int, error)
    RemoveContainer(ctx context.Context, containerID string) error
    
    // コンテナとの通信
    AttachContainer(ctx context.Context, containerID string, tty bool, stdin io.Reader, stdout, stderr io.Writer) error
    
    // 情報取得
    Name() string
}
```

**テスト**:
- インターフェースのモック実装
- 基本的な動作確認

**完了条件**:
- `ContainerRuntime`インターフェースが定義されている
- モック実装でテストが通る

---

### Step 1.3: Docker API実装

**目的**: Docker Engine APIを使った実装

**参照ドキュメント**:
- `docs/features/multi-runtime-support.md`
- `docs/features/direct-container-execution.md`

**依存ライブラリ**:
```bash
go get github.com/docker/docker/client
go get github.com/docker/docker/api/types/container
go get github.com/docker/docker/api/types/mount
go get github.com/docker/docker/pkg/stdcopy
```

**実装内容**:

1. `internal/runtime/docker.go`を作成

```go
package runtime

import (
    "context"
    "io"
    
    "github.com/docker/docker/client"
    "github.com/docker/docker/api/types/container"
    "github.com/docker/docker/api/types/mount"
    "github.com/docker/docker/pkg/stdcopy"
)

type DockerRuntime struct {
    client *client.Client
    socket string
}

func NewDockerRuntime(socket string) (*DockerRuntime, error) {
    cli, err := client.NewClientWithOpts(
        client.WithHost("unix://" + socket),
        client.WithAPIVersionNegotiation(),
    )
    if err != nil {
        return nil, err
    }
    
    return &DockerRuntime{
        client: cli,
        socket: socket,
    }, nil
}

func (d *DockerRuntime) CreateContainer(ctx context.Context, config *container.ContainerConfig) (string, error) {
    // ContainerConfigをDocker APIの形式に変換
    containerConfig := &container.Config{
        Image:      config.Image,
        Cmd:        append(config.Command, config.Args...),
        Tty:        config.TTY,
        OpenStdin:  config.Interactive,
        Env:        config.Env,
        WorkingDir: config.Workdir,
        User:       config.User,
    }
    
    hostConfig := &container.HostConfig{
        AutoRemove:  config.Remove,
        NetworkMode: container.NetworkMode(config.Network),
    }
    
    // ボリュームマウント
    for _, vol := range config.Volumes {
        m := mount.Mount{
            Type:     mount.TypeBind,
            Source:   vol.HostPath,
            Target:   vol.ContainerPath,
            ReadOnly: vol.ReadOnly,
        }
        hostConfig.Mounts = append(hostConfig.Mounts, m)
    }
    
    resp, err := d.client.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "")
    if err != nil {
        return "", err
    }
    
    return resp.ID, nil
}

// 他のメソッドも実装...
```

**テスト**:
- Dockerが利用可能な環境でのインテグレーションテスト
- コンテナの作成・起動・削除

**完了条件**:
- Docker APIを使ったコンテナ実行が動作する
- テストが通る

---

### Step 1.4: 基本的なコンテナ実行フロー

**目的**: CLIからコンテナを実際に実行

**参照ドキュメント**:
- `docs/features/direct-container-execution.md`
- `docs/features/argument-parsing.md`

**実装内容**:

1. `cmd/root.go`を修正

```go
func (cmd *cobra.Command, args []string) error {
    if len(args) == 0 {
        return cmd.Help()
    }
    
    subcommand := args[0]
    passthroughArgs := args[1:]
    
    // ContainerConfigの構築
    config := &container.ContainerConfig{
        Image:       "alpine:latest", // 仮のイメージ
        Command:     []string{subcommand},
        Args:        passthroughArgs,
        TTY:         tty,
        Interactive: interactive,
        Network:     network,
        Remove:      true,
    }
    
    // ランタイムの初期化
    runtime, err := runtime.NewDockerRuntime("/var/run/docker.sock")
    if err != nil {
        return err
    }
    
    // コンテナの実行
    ctx := context.Background()
    
    containerID, err := runtime.CreateContainer(ctx, config)
    if err != nil {
        return err
    }
    
    if err := runtime.StartContainer(ctx, containerID); err != nil {
        return err
    }
    
    if config.TTY || config.Interactive {
        if err := runtime.AttachContainer(ctx, containerID, config.TTY, os.Stdin, os.Stdout, os.Stderr); err != nil {
            return err
        }
    }
    
    exitCode, err := runtime.WaitContainer(ctx, containerID)
    if err != nil {
        return err
    }
    
    os.Exit(exitCode)
    return nil
}
```

**テスト**:
```bash
# 基本的な実行
cderun sh -c "echo hello"

# TTY付き
cderun --tty sh

# インタラクティブ
cderun -ti sh
```

**完了条件**:
- `cderun`でコンテナが実行される
- TTY/インタラクティブモードが動作する
- 終了コードが正しく返される

---

## Phase 2: 設定管理

### Step 2.1: 設定ファイル読み込み

**目的**: `.cderun.yaml`と`.tools.yaml`を読み込む

**参照ドキュメント**:
- `docs/features/configuration-file-support.md`

**依存ライブラリ**:
```bash
go get gopkg.in/yaml.v3
```

**実装内容**:

1. `internal/config/config.go`を作成

```go
package config

type CderunConfig struct {
    Runtime     string   `yaml:"runtime"`
    RuntimePath string   `yaml:"runtimePath"`
    Defaults    Defaults `yaml:"defaults"`
}

type Defaults struct {
    TTY         bool   `yaml:"tty"`
    Interactive bool   `yaml:"interactive"`
    Network     string `yaml:"network"`
    Remove      bool   `yaml:"remove"`
    SyncWorkdir bool   `yaml:"syncWorkdir"`
}

type ToolsConfig map[string]ToolConfig

type ToolConfig struct {
    Image   string   `yaml:"image"`
    TTY     *bool    `yaml:"tty"`
    Interactive *bool `yaml:"interactive"`
    Network string   `yaml:"network"`
    Remove  *bool    `yaml:"remove"`
    Volumes []string `yaml:"volumes"`
    Env     []string `yaml:"env"`
    Workdir string   `yaml:"workdir"`
}

func LoadCderunConfig() (*CderunConfig, error) {
    // 検索順序: ./.cderun.yaml -> ~/.config/cderun/config.yaml -> /etc/cderun/config.yaml
}

func LoadToolsConfig() (ToolsConfig, error) {
    // 検索順序: ./.tools.yaml -> ~/.config/cderun/tools.yaml -> /etc/cderun/tools.yaml
}
```

**テスト**:
- 各検索パスからの読み込み
- YAML解析エラーのハンドリング

**完了条件**:
- 設定ファイルが正しく読み込まれる
- 存在しない場合のデフォルト動作

---

### Step 2.2: イメージマッピング

**目的**: サブコマンド名からイメージを解決

**参照ドキュメント**:
- `docs/features/image-mapping.md`

**実装内容**:

1. `internal/image/mapper.go`を作成

```go
package image

var defaultMappings = map[string]string{
    "node":   "node:alpine",
    "python": "python:alpine",
    "go":     "golang:alpine",
    "rust":   "rust:alpine",
    "bash":   "alpine:latest",
    "sh":     "alpine:latest",
}

type ImageMapper struct {
    toolsConfig config.ToolsConfig
}

func (m *ImageMapper) ResolveImage(toolName string) (string, error) {
    // 1. .tools.yamlから検索
    if tool, ok := m.toolsConfig[toolName]; ok {
        return tool.Image, nil
    }
    
    // 2. デフォルトマッピングから検索
    if image, ok := defaultMappings[toolName]; ok {
        return image, nil
    }
    
    // 3. エラー
    return "", fmt.Errorf("no image mapping found for '%s'", toolName)
}
```

**テスト**:
- デフォルトマッピングの解決
- `.tools.yaml`からの解決
- 存在しないツールのエラー

**完了条件**:
- イメージが正しく解決される
- エラーメッセージが適切

---

### Step 2.3: 優先順位解決

**目的**: CLI、環境変数、設定ファイルの優先順位を実装

**参照ドキュメント**:
- `docs/features/argument-priority-logic.md`

**実装内容**:

1. `internal/config/resolver.go`を作成

```go
package config

// P1: CLI Override Flags (最優先)
// P2: Standard CLI Flags
// P3: Environment Variables
// P4: Tool-Specific Config
// P5: Global Defaults

func ResolveBool(
    cliValue bool,
    cliChanged bool,
    envName string,
    toolValue *bool,
    defaultValue bool,
) bool {
    // P1 & P2: CLI
    if cliChanged {
        return cliValue
    }
    
    // P3: Environment
    if val, set := os.LookupEnv(envName); set {
        return parseBool(val)
    }
    
    // P4: Tool Config
    if toolValue != nil {
        return *toolValue
    }
    
    // P5: Default
    return defaultValue
}
```

**テスト**:
- 各優先順位レベルでの解決
- 組み合わせテスト

**完了条件**:
- 優先順位が正しく動作する
- すべてのテストが通る

---

## Phase 3: 高度な機能

### Step 3.1: 環境変数パススルー

**目的**: 環境変数の選択的な引き継ぎ

**参照ドキュメント**:
- `docs/features/env-passthrough.md`

**実装内容**:

1. `internal/env/resolver.go`を作成

```go
package env

// ResolveEnv は環境変数リストを解決
// "KEY=value" -> そのまま
// "KEY" -> os.Getenv("KEY")から取得
func ResolveEnv(envList []string) []string {
    resolved := make([]string, 0, len(envList))
    
    for _, env := range envList {
        if strings.Contains(env, "=") {
            resolved = append(resolved, env)
        } else {
            key := env
            value := os.Getenv(key)
            resolved = append(resolved, fmt.Sprintf("%s=%s", key, value))
        }
    }
    
    return resolved
}
```

**テスト**:
- `KEY=value`形式
- `KEY`形式（パススルー）
- 存在しない環境変数

**完了条件**:
- 環境変数が正しく解決される
- デフォルトでは引き継がれない

---

### Step 3.2: 作業ディレクトリ同期

**目的**: ホストのカレントディレクトリをコンテナ内で再現

**参照ドキュメント**:
- `docs/features/workdir-sync.md`

**実装内容**:

```go
func AddWorkdirMount(config *container.ContainerConfig) error {
    cwd, err := os.Getwd()
    if err != nil {
        return err
    }
    
    config.Volumes = append(config.Volumes, container.VolumeMount{
        HostPath:      cwd,
        ContainerPath: cwd,
        ReadOnly:      false,
    })
    config.Workdir = cwd
    
    return nil
}
```

**テスト**:
- 相対パスの動作確認
- ファイル操作の確認

**完了条件**:
- カレントディレクトリが正しくマウントされる
- 相対パスが動作する

---

### Step 3.3: ソケットマウント

**目的**: `--mount-socket`の実装

**参照ドキュメント**:
- `docs/features/docker-in-docker-support.md`

**実装内容**:

```go
if mountSocket != "" {
    config.Volumes = append(config.Volumes, container.VolumeMount{
        HostPath:      mountSocket,
        ContainerPath: mountSocket,
        ReadOnly:      false,
    })
}
```

**テスト**:
```bash
cderun --mount-socket /var/run/docker.sock docker ps
```

**完了条件**:
- ソケットが正しくマウントされる
- コンテナ内からDockerが使える

---

### Step 3.4: cderunバイナリマウント

**目的**: `--mount-cderun`の実装

**参照ドキュメント**:
- `docs/features/cderun-binary-mounting.md`

**実装内容**:

1. アーキテクチャ検出
2. バイナリダウンロード（必要に応じて）
3. バイナリマウント

```go
func AddCderunMount(config *container.ContainerConfig, targetArch string) error {
    binaryPath := fmt.Sprintf("~/.config/cderun/bin/cderun-%s", targetArch)
    
    config.Volumes = append(config.Volumes, container.VolumeMount{
        HostPath:      binaryPath,
        ContainerPath: "/usr/local/bin/cderun",
        ReadOnly:      true,
    })
    
    return nil
}
```

**テスト**:
```bash
cderun --mount-cderun --mount-socket /var/run/docker.sock sh
# コンテナ内で
cderun node --version
```

**完了条件**:
- cderunがコンテナ内で使える
- `--mount-socket`との併用チェック

---

### Step 3.5: ツールマウント

**目的**: `--mount-tools`と`--mount-all-tools`の実装

**参照ドキュメント**:
- `docs/features/mount-tools.md`

**実装内容**:

```go
func AddToolMounts(config *container.ContainerConfig, tools []string, cderunBinary string) error {
    for _, tool := range tools {
        config.Volumes = append(config.Volumes, container.VolumeMount{
            HostPath:      cderunBinary,
            ContainerPath: fmt.Sprintf("/usr/local/bin/%s", tool),
            ReadOnly:      true,
        })
    }
    return nil
}
```

**テスト**:
```bash
cderun --mount-cderun --mount-socket /var/run/docker.sock --mount-tools node,python sh
```

**完了条件**:
- 指定したツールがマウントされる
- `.tools.yaml`の検証が動作する

---

## Phase 4: 利便性向上

### Step 4.1: ドライランモード

**目的**: `--dry-run`の実装

**参照ドキュメント**:
- `docs/features/dry-run-mode.md`

**実装内容**:

```go
if dryRun {
    fmt.Printf("Would execute:\n")
    fmt.Printf("Image: %s\n", config.Image)
    fmt.Printf("Command: %v\n", config.Command)
    fmt.Printf("Args: %v\n", config.Args)
    // ...
    return nil
}
```

**完了条件**:
- コマンドがプレビューされる
- 実際には実行されない

---

### Step 4.2: ログ・デバッグ

**目的**: `--verbose`の実装

**参照ドキュメント**:
- `docs/features/logging-debugging.md`

**実装内容**:

```go
if verbose {
    log.Printf("Creating container with config: %+v\n", config)
    log.Printf("Container ID: %s\n", containerID)
}
```

**完了条件**:
- 詳細ログが出力される

---

## 実装チェックリスト

### Phase 1
- [ ] Step 1.1: ContainerConfig定義
- [ ] Step 1.2: Runtimeインターフェース
- [ ] Step 1.3: Docker API実装
- [ ] Step 1.4: 基本実行フロー

### Phase 2
- [ ] Step 2.1: 設定ファイル読み込み
- [ ] Step 2.2: イメージマッピング
- [ ] Step 2.3: 優先順位解決

### Phase 3
- [ ] Step 3.1: 環境変数パススルー
- [ ] Step 3.2: 作業ディレクトリ同期
- [ ] Step 3.3: ソケットマウント
- [ ] Step 3.4: cderunバイナリマウント
- [ ] Step 3.5: ツールマウント

### Phase 4
- [ ] Step 4.1: ドライランモード
- [ ] Step 4.2: ログ・デバッグ

## 各ステップの完了基準

1. **コードが動作する**: 実装した機能が期待通りに動作
2. **テストが通る**: ユニットテストとインテグレーションテスト
3. **ドキュメントと一致**: featuresドキュメントの仕様通り
4. **エラーハンドリング**: 適切なエラーメッセージ

## 注意事項

- 各ステップは独立してテスト可能にする
- 前のステップが完了してから次に進む
- 既存のコードを壊さない
- featuresドキュメントを常に参照する
