# 機能仕様：ツールマウント (完了)

## 概要

`.tools.yaml`に定義されたツールをコンテナ内で使用可能にする機能。
cderunバイナリを複数のツール名でマウントし、ポリグロットエントリーポイント機能を活用します。

## 前提条件

- `.tools.yaml` が存在し、対象のツールが定義されていること

`--mount-tools` または `--mount-all-tools` を使用すると、`--mount-cderun` および `--mount-socket` が自動的に有効になります。詳細は [nested-execution.md](./nested-execution.md) を参照してください。

## オプション

### `--mount-all-tools`

**型**: bool
**デフォルト**: `false`
**説明**: `.tools.yaml`に定義されているすべてのツールをマウント

**使用例**:

```bash
cderun --mount-all-tools sh
```

**動作**:

```bash
# .tools.yamlに node, python, gemini-cli が定義されている場合
docker run --rm \
  --mount type=bind,source=/var/run/docker.sock,target=/var/run/docker.sock \
  --mount type=bind,source=<ホスト側cderunのパス>,target=/usr/local/bin/cderun,readonly \
  --mount type=bind,source=<ホスト側cderunのパス>,target=/usr/local/bin/node,readonly \
  --mount type=bind,source=<ホスト側cderunのパス>,target=/usr/local/bin/python,readonly \
  --mount type=bind,source=<ホスト側cderunのパス>,target=/usr/local/bin/gemini-cli,readonly \
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
cderun --mount-tools python,node sh
```

**動作イメージ(実際はランタイムAPIで実現)**:

```bash
docker run --rm \
  --mount type=bind,source=/var/run/docker.sock,target=/var/run/docker.sock \
  --mount type=bind,source=<ホスト側cderunのパス>,target=/usr/local/bin/cderun,readonly \
  --mount type=bind,source=<ホスト側cderunのパス>,target=/usr/local/bin/python,readonly \
  --mount type=bind,source=<ホスト側cderunのパス>,target=/usr/local/bin/node,readonly \
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
├── cderun       -> <ホスト側cderunのパス>
├── node         -> <ホスト側cderunのパス>
├── python       -> <ホスト側cderunのパス>
└── gemini-cli   -> <ホスト側cderunのパス>
```

### ポリグロットエントリーポイントの活用

cderunのポリグロットエントリーポイント機能により、実行ファイル名が自動的にサブコマンドとして認識されます:

```bash
# コンテナ内で "node" を実行
node --version

# cderunが実際に実行するコマンド
cderun node --version
```

### ツールの検証

指定されたツールが`.tools.yaml`に存在しない場合はエラー:

```bash
cderun --mount-tools unknown-tool alpine sh
Error: tool "unknown-tool" not found in .tools.yaml
available tools: node, python, gemini-cli
```

## 使用例

### 開発環境の構築

```bash
# .tools.yamlにbashが定義されている場合
cderun --mount-all-tools bash

# または --image で明示的に指定
cderun --mount-all-tools \
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
cderun --mount-tools python,node sh
```

### CI/CDパイプライン

```bash
# .tools.yamlに定義されたツールを使用
cderun --mount-tools node,docker \
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
cderun --mount-tools node,npm,npx \
  sh -c '
    node --version
    npm install
    npx eslint .
  '
```

## 制限事項

1. **依存性**: 動作にはホストのコンテナランタイムソケットへのアクセスが必要（通常は `--mount-socket` によって自動的にマウントされます）
2. **読み取り専用**: マウントされたツールは読み取り専用
3. **パスの上書き**: コンテナ内に同名のツールがある場合、上書きされる
4. **アーキテクチャ一致**: コンテナのアーキテクチャに合ったcderunバイナリが必要（実行中のバイナリがそのままマウントされるため）

## メリット

- **柔軟性**: 必要なツールだけを選択的にマウント
- **軽量**: 実際のツールをインストールする必要がない
- **統一インターフェース**: すべてのツールがcderun経由で実行される
- **シンプル**: ポリグロットエントリーポイントを活用
