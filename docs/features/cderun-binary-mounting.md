# Feature: cderun Binary Mounting (Recursive Execution) (Completed)

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
$ cderun --mount-cderun --mount-socket /var/run/docker.sock gemini-cli

# 生成されるコマンド (イメージ):
# docker run --rm -t -i \
#   -v /usr/local/bin/cderun:/usr/local/bin/cderun:ro \  # cderunバイナリをマウント
#   -v /var/run/docker.sock:/var/run/docker.sock \       # dockerソケットをマウント
#   gemini-cli:latest gemini-cli
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
cderun --mount-cderun --mount-socket /var/run/docker.sock gemini-cli
```

#### 設定ファイルによる指定
```yaml
# .tools.yaml
gemini-cli:
  image: gemini-cli:latest
  mountCderun: true  # cderunバイナリを自動マウント
```

#### グローバル設定
```yaml
# .cderun.yaml
defaults:
  mountCderun: false  # デフォルトでは無効
```

### マウントされるもの

1. **cderunバイナリ**
   - 実行ホストの`cderun`バイナリをread-onlyでマウント
   - パス: `/usr/local/bin/cderun`

2. **ランタイムソケット**
   - コンテナ内のcderunが基底ホストのコンテナランタイム（Docker等）と通信できるように
   - 指定されたソケットパスを同じパスでコンテナ内にマウント

> **Note**: 設定ファイル（`.cderun.yaml` や `.tools.yaml`）は自動的にはマウントされません。コンテナ内からcderunを使用する場合、必要な設定は環境変数経由で渡すか、ボリュームマウントを使用して設定ファイルをコンテナ内の検索パス（例：カレントディレクトリ）に配置する必要があります。

## 使用例

### 例1: gemini-cliからMCPサーバーを呼び出す

#### ホスト側の設定
```yaml
# .tools.yaml
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
$ cderun --mount-socket /var/run/docker.sock gemini-cli

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
$ cderun --mount-cderun --mount-socket /var/run/docker.sock dev-env

# dev-envコンテナ内で、さらに別のツールを起動
$ cderun node --version
v20.10.0

$ cderun python --version
Python 3.11.0

$ cderun docker ps
# ホストのdockerコンテナ一覧を表示
```

## セキュリティ考慮事項

### リスク

1. **ランタイムソケットへのアクセス**
   - コンテナ内から基底ホストのコンテナランタイムへの完全アクセス
   - コンテナエスケープのリスク

2. **再帰的な実行**
   - 無限ループの可能性（cderun → cderun → cderun...）
   - リソース枯渇のリスク

### 対策

#### read-onlyマウント
- cderunバイナリはread-onlyでマウントされます。
- `/usr/local/bin/cderun:/usr/local/bin/cderun:ro`

## 制限事項

### 1. 環境の分離

**重要**: コンテナ内からcderunで起動されるツールは、**新しい独立したコンテナ**で実行されます。

```bash
# 基底ホスト
$ cderun --mount-cderun --mount-socket /var/run/docker.sock gemini-cli

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

## 実装状況

1. **Phase 1-3 Completed**: 基本的なcderunバイナリおよびソケットのマウントが実装済み。
