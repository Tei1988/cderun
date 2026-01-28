# Feature: cderun Binary Mounting (Recursive Execution)

## 概要

cderunバイナリ自体をコンテナ内にマウントすることで、コンテナ内から再帰的にcderunを呼び出せるようにする機能。

これにより、`gemini-cli`のようなツールのコンテナ内から、MCPサーバーとして登録された他のツール（例：`python`、`node`等）をcderun経由で実行できる。

## 問題の背景

### ユースケース: gemini-cliからMCPサーバーを呼び出す

```bash
# 基底ホストで実行
$ cderun --mount-cderun --tty --interactive gemini-cli

# gemini-cliコンテナ内で、事前登録されたMCPサーバーを呼び出したい
gemini> use mcp server "python-tools"
# → コンテナ内でcderunを使ってpythonを起動したい
# → しかし、gemini-cliコンテナ内にcderunがない！
```

## 解決策: cderunバイナリのマウント

### 基本的な仕組み

```bash
$ cderun --mount-cderun gemini-cli

# 生成されるコマンド:
docker run --rm -t -i \
  -v /usr/local/bin/cderun:/usr/local/bin/cderun:ro \  # cderunバイナリをマウント
  -v /var/run/docker.sock:/var/run/docker.sock \       # dockerソケットもマウント
  -v /home/user/.cderun:/home/user/.cderun:ro \        # 設定ファイルもマウント
  gemini-cli:latest gemini-cli
```

### コンテナ内での動作

```bash
# gemini-cliコンテナ内
$ which cderun
/usr/local/bin/cderun

$ cderun python --version
# → ホストのdockerデーモンを使って、新しいpythonコンテナを起動
Python 3.11.0
```

## 実装詳細

### 自動マウント

#### フラグによる指定
```bash
# 明示的に指定
cderun --mount-cderun gemini-cli

# 短縮形
cderun --recursive gemini-cli
cderun -r gemini-cli
```

#### 設定ファイルによる指定
```yaml
tools:
  gemini-cli:
    image: gemini-cli:latest
    mountCderun: true  # cderunバイナリを自動マウント
```

#### グローバル設定
```yaml
cderun:
  defaults:
    mountCderun: false  # デフォルトでは無効
```

### マウントされるもの

1. **cderunバイナリ**
   - 実行ホストの`cderun`バイナリをread-onlyでマウント
   - パス: `/usr/local/bin/cderun` (または`which cderun`の結果)

2. **dockerソケット**
   - コンテナ内のcderunが基底ホストのdockerデーモンと通信できるように
   - パス: `/var/run/docker.sock`

### 実装例

```go
type MountCderunConfig struct {
    Enabled        bool
    BinaryPath     string   // cderunバイナリのパス
    SocketPath     string   // dockerソケットのパス
}

func buildMountCderunVolumes(config MountCderunConfig) []string {
    volumes := []string{}
    
    // cderunバイナリ
    if config.BinaryPath != "" {
        volumes = append(volumes, 
            fmt.Sprintf("%s:/usr/local/bin/cderun:ro", config.BinaryPath))
    }
    
    // dockerソケット
    if config.SocketPath != "" {
        volumes = append(volumes, 
            fmt.Sprintf("%s:%s", config.SocketPath, config.SocketPath))
    }
    
    return volumes
}
```

## 使用例

### 例1: gemini-cliからMCPサーバーを呼び出す

#### ホスト側の設定
```yaml
# .cderun.yaml
tools:
  gemini-cli:
    image: gemini-cli:latest
    mountCderun: true
    tty: true
    interactive: true
    
  python:
    image: python:3.11-slim
    volumes:
      - .:/workspace
```

#### 実行
```bash
# 基底ホストでgemini-cliを起動
$ cderun gemini-cli

# gemini-cliコンテナ内
gemini> use mcp server "python-tools"
# MCPサーバーの設定で、pythonツールをcderun経由で起動

# コンテナ内でcderunが実行される
$ cderun python script.py
# → ホストのdockerデーモンを使って、新しいpythonコンテナを起動
```

### 例2: 開発環境のネスト

```bash
# 基底ホストでdevコンテナを起動
$ cderun --mount-cderun dev-env

# dev-envコンテナ内で、さらに別のツールを起動
$ cderun node --version
v20.10.0

$ cderun python --version
Python 3.11.0

$ cderun docker ps
# ホストのdockerコンテナ一覧を表示
```

### 例3: CI/CDパイプライン

```yaml
# .cderun.yaml
tools:
  ci-runner:
    image: ci-runner:latest
    mountCderun: true
    volumes:
      - .:/workspace
```

```bash
# CI/CDスクリプト内
$ cderun ci-runner

# ci-runnerコンテナ内
$ cderun docker build -t app:latest .
$ cderun docker push app:latest
$ cderun node test
$ cderun python lint.py
```

## 設定オプション

### ツール固有の設定

```yaml
tools:
  gemini-cli:
    mountCderun: true
    mountCderunConfig:
      binaryPath: /usr/local/bin/cderun  # カスタムパス
      socketPath: /var/run/docker.sock
```

### グローバル設定

```yaml
cderun:
  recursive:
    enabled: false  # デフォルトでは無効
    binaryPath: /usr/local/bin/cderun
    socketPath: /var/run/docker.sock
```

## セキュリティ考慮事項

### リスク

1. **Dockerソケットへのアクセス**
   - コンテナ内から基底ホストのdockerデーモンへの完全アクセス
   - コンテナエスケープのリスク

2. **再帰的な実行**
   - 無限ループの可能性（cderun → cderun → cderun...）
   - リソース枯渇のリスク

3. **設定ファイルの露出**
   - 基底ホストの設定ファイルがコンテナ内から読める
   - 機密情報が含まれる可能性

### 対策

#### 1. 深さ制限
```go
const MaxRecursionDepth = 3

func checkRecursionDepth() error {
    depth := os.Getenv("CDERUN_DEPTH")
    if depth == "" {
        depth = "0"
    }
    
    d, _ := strconv.Atoi(depth)
    if d >= MaxRecursionDepth {
        return fmt.Errorf("maximum recursion depth exceeded: %d", d)
    }
    
    // 次のレベルに深さを渡す
    os.Setenv("CDERUN_DEPTH", strconv.Itoa(d+1))
    return nil
}
```

#### 2. 警告表示
```bash
$ cderun --mount-cderun gemini-cli
Warning: Mounting cderun binary grants container access to host Docker daemon
This is equivalent to root access. Continue? [y/N]
```

#### 3. read-onlyマウント
```bash
# cderunバイナリと設定ファイルはread-onlyでマウント
-v /usr/local/bin/cderun:/usr/local/bin/cderun:ro
-v ~/.cderun/config.yaml:~/.cderun/config.yaml:ro
```

## 制限事項

### 1. 環境の分離

**重要**: コンテナ内からcderunで起動されるツールは、**新しい独立したコンテナ**で実行されます。

```bash
# 基底ホスト
$ cderun --mount-cderun gemini-cli

# gemini-cliコンテナ（実行ホスト）
$ export MY_VAR=hello
$ cderun python -c "import os; print(os.getenv('MY_VAR'))"
None  # ← 環境変数は引き継がれない！
```

#### 何が引き継がれないか

- ❌ **環境変数**: 実行ホスト（gemini-cliコンテナ）内で設定した環境変数
- ❌ **インストールしたパッケージ**: 実行ホスト内でインストールしたもの
- ❌ **ファイルシステム**: 実行ホスト内のファイル（基底ホストからマウントされていない限り）
- ❌ **プロセス**: 実行ホスト内で実行中のプロセス

#### 何が使えるか

- ✅ **基底ホストのファイルシステム**: 基底ホストからマウントされたディレクトリ
- ✅ **基底ホストのdockerデーモン**: 同じdockerデーモンを使用

### 2. プラットフォーム依存
- cderunバイナリは実行ホストと同じアーキテクチャである必要がある
- Linux/amd64の実行ホストから、Linux/arm64のコンテナでは動作しない

### 3. バイナリの互換性
- コンテナ内のライブラリとの互換性が必要
- 静的リンクされたバイナリが推奨

### 4. パフォーマンス
- ネストされたコンテナ起動のオーバーヘッド
- 深いネストは非推奨

## 実装の優先順位

1. **Phase 1**: 基本的なcderunバイナリマウント
2. **Phase 2**: dockerソケットのマウント
3. **Phase 3**: 再帰深さ制限
4. **Phase 4**: セキュリティ警告とプロンプト

## 推奨される使用方法

### ✅ 推奨

```yaml
# 開発環境やCI/CDでの使用
tools:
  dev-env:
    mountCderun: true
  
  gemini-cli:
    mountCderun: true
```

### ⚠️ 注意が必要

```yaml
# 本番環境での使用は慎重に
tools:
  production-app:
    mountCderun: false  # 本番では無効化
```

### ❌ 非推奨

```yaml
# 全てのツールでデフォルト有効化
cderun:
  defaults:
    mountCderun: true  # 非推奨
```
