# Feature: Mount Tools (Completed)

## 概要

`.tools.yaml`に定義されたツールをコンテナ内で使用可能にする機能。
cderunバイナリを複数のツール名でマウントし、ポリグロットエントリーポイント機能を活用します。

## 前提条件

- `--mount-cderun`が有効化されていること
- `--mount-socket`が指定されていること
- `.tools.yaml`が存在すること

## オプション

### `--mount-all-tools`

**型**: bool  
**デフォルト**: `false`  
**説明**: `.tools.yaml`に定義されているすべてのツールをマウント

**使用例**:
```bash
cderun --mount-cderun --mount-socket /var/run/docker.sock --mount-all-tools sh
```

**動作**:
```bash
# .tools.yamlに node, python, gemini-cli が定義されている場合
docker run --rm \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v ~/.config/cderun/bin/cderun-linux-amd64:/usr/local/bin/cderun:ro \
  -v ~/.config/cderun/bin/cderun-linux-amd64:/usr/local/bin/node:ro \
  -v ~/.config/cderun/bin/cderun-linux-amd64:/usr/local/bin/python:ro \
  -v ~/.config/cderun/bin/cderun-linux-amd64:/usr/local/bin/gemini-cli:ro \
  alpine:latest
```

**コンテナ内での使用**:
```bash
# コンテナ内で
node --version    # cderunがnodeとして実行される
python script.py  # cderunがpythonとして実行される
gemini-cli ask    # cderunがgemini-cliとして実行される
```

### `--mount-tools`

**型**: string  
**デフォルト**: `""`  
**説明**: 指定したツールのみをマウント（カンマ区切り）

**使用例**:
```bash
cderun --mount-cderun --mount-socket /var/run/docker.sock --mount-tools python,node sh
```

**動作イメージ(実際はランタイムAPIで実現)**:
```bash
docker run --rm \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v ~/.config/cderun/bin/cderun-linux-amd64:/usr/local/bin/cderun:ro \
  -v ~/.config/cderun/bin/cderun-linux-amd64:/usr/local/bin/python:ro \
  -v ~/.config/cderun/bin/cderun-linux-amd64:/usr/local/bin/node:ro \
  alpine:latest
```

**コンテナ内での使用**:
```bash
# コンテナ内で
python --version  # OK
node --version    # OK
gemini-cli ask    # エラー: マウントされていない
```

## 実装詳細

### マウント先

ツールは`/usr/local/bin/`にマウントされます:

```text
/usr/local/bin/
├── cderun       -> ~/.config/cderun/bin/cderun-linux-amd64
├── node         -> ~/.config/cderun/bin/cderun-linux-amd64
├── python       -> ~/.config/cderun/bin/cderun-linux-amd64
└── gemini-cli   -> ~/.config/cderun/bin/cderun-linux-amd64
```

### ポリグロットエントリーポイントの活用

cderunのポリグロットエントリーポイント機能により、実行ファイル名が自動的にサブコマンドとして認識されます:

```bash
# コンテナ内で "node" を実行
$ node --version

# cderunが実際に実行するコマンド
$ cderun node --version
```

### ツールの検証

指定されたツールが`.tools.yaml`に存在しない場合はエラー:

```bash
$ cderun --mount-tools unknown-tool alpine sh
Error: Tool 'unknown-tool' not found in .tools.yaml
Available tools: node, python, gemini-cli
```

## 使用例

### 開発環境の構築

```bash
# .tools.yamlにbashが定義されている場合
cderun --mount-cderun \
  --mount-socket /var/run/docker.sock \
  --mount-all-tools \
  bash

# または --image で明示的に指定
cderun --mount-cderun \
  --mount-socket /var/run/docker.sock \
  --mount-all-tools \
  --image ubuntu:22.04 \
  bash

# コンテナ内で
node --version
python --version
gemini-cli --version
```

### 特定ツールのみマウント

```bash
# .tools.yamlにshが定義されている場合
cderun --mount-cderun \
  --mount-socket /var/run/docker.sock \
  --mount-tools python,node \
  sh
```

### CI/CDパイプライン

```bash
# .tools.yamlに定義されたツールを使用
cderun --mount-cderun \
  --mount-socket /var/run/docker.sock \
  --mount-tools node,docker \
  sh -c '
    # nodeコマンドはcderun経由で実行される
    node --version
    
    # dockerコマンドもcderun経由で実行される
    docker build -t myapp .
    docker push myapp
  '
```

**注意**: `npm`や`npx`などのコマンドを使う場合は、`.tools.yaml`に別途定義する必要があります：

```yaml
# .tools.yaml
node:
  image: node:20-alpine
  
npm:
  image: node:20-alpine
  
npx:
  image: node:20-alpine
```

そうすれば以下のように使用できます：

```bash
cderun --mount-cderun \
  --mount-socket /var/run/docker.sock \
  --mount-tools node,npm,npx \
  sh -c '
    node --version
    npm install
    npx eslint .
  '
```

## 制限事項

1. **依存性**: `--mount-cderun`と`--mount-socket`が必須
2. **読み取り専用**: マウントされたツールは読み取り専用
3. **パスの上書き**: コンテナ内に同名のツールがある場合、上書きされる
4. **アーキテクチャ一致**: コンテナのアーキテクチャに合ったcderunバイナリが必要

## メリット

- **柔軟性**: 必要なツールだけを選択的にマウント
- **軽量**: 実際のツールをインストールする必要がない
- **統一インターフェース**: すべてのツールがcderun経由で実行される
- **シンプル**: ポリグロットエントリーポイントを活用
