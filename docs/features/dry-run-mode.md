# Feature: Dry Run Mode (Completed)

## 概要

実際にコンテナを実行せず、生成される中間表現（ContainerConfig）を表示する機能。

## 要件

### 基本動作
`--dry-run`フラグが指定された場合：
1. 通常通り設定を読み込み、中間表現を生成
2. コンテナを実行せず、中間表現を表示
3. 終了コード0で終了

## 使用方法

### 基本的な使用
```bash
cderun --dry-run node --version
```

### グローバルドライラン（診断情報）
サブコマンドを指定せずに `--dry-run` を実行すると、システムの診断情報と利用可能なツールの一覧を表示します。
```bash
cderun --dry-run
```

## 出力フォーマット

### YAML形式（デフォルト）
`cderun --dry-run node app.js`
```yaml
image: node:latest
command:
  - node
  - app.js
tty: true
interactive: true
remove: true
network: bridge
mounts:
  - type: bind
    source: /home/user/project
    target: /workspace
env:
  - NODE_ENV=development
workdir: /workspace
user: ""
```

### JSON形式
`cderun --dry-run --dry-run-format json node app.js`
```json
{
  "image": "node:latest",
  "command": ["node", "app.js"],
  "tty": true,
  "interactive": true,
  "remove": true,
  "network": "bridge",
  "mounts": [
    {
      "type": "bind",
      "source": "/home/user/project",
      "target": "/workspace"
    }
  ],
  "env": ["NODE_ENV=development"],
  "workdir": "/workspace",
  "user": ""
}
```

### 簡易形式
`cderun --dry-run --dry-run-format simple node app.js`
```text
Image: node:latest
Command: node app.js
TTY: true
Interactive: true
Network: bridge
Remove: true
Mounts: type=bind,source=/home/user/project,target=/workspace,readonly=false
Env: NODE_ENV=development
Workdir: /workspace
User:
Ports:
PublishAll: false
Expose:
Hostname:
DNS:
AddHosts:
Privileged: false
CapAdd:
CapDrop:
Entrypoint:
Pull: missing
Memory: 0 B
CPUs: 0
Devices:
```

## ユースケース

### 1. デバッグ
設定が正しく適用されているか確認：
```bash
cderun --dry-run python script.py
```

### 2. 設定の検証
```bash
#!/bin/bash
output=$(cderun --dry-run --dry-run-format json node --version)
image=$(echo $output | jq -r '.image')
if [[ $image == "node:20-alpine" ]]; then
  echo "Configuration is correct"
else
  echo "Unexpected image: $image"
  exit 1
fi
```

### 3. 設定ファイルのドキュメント化
```bash
cderun --dry-run --dry-run-format yaml node app.js > config-example.yaml
```

### 4. システム診断とツール一覧の確認
```bash
cderun --dry-run
# または詳細形式
cderun --dry-run -f json
```

**YAML出力例（診断情報）**:
```yaml
runtime:
  name: docker
  socket: /var/run/docker.sock
  status: accessible
configs:
  global:
    - /home/user/.cderun.yaml
  tools:
    - /home/user/project/.tools.yaml
available_tools:
  - node
  - python
  - git
```

## 他のフラグとの組み合わせ

### --verboseとの組み合わせ
```bash
cderun --dry-run --verbose node app.js
[INFO] Loading configuration from: /home/user/project/.cderun.yaml
[INFO] Resolved image: node:20-alpine
[INFO] Working directory: /home/user/project
[INFO] Environment variables: NODE_ENV=development
[INFO] Generated ContainerConfig:
image: node:20-alpine
command: [node, app.js]
...
```

## 実装上の注意

### 環境変数の展開
ドライラン時も環境変数は実際の値に展開される：
```bash
export API_KEY=secret123
cderun --dry-run --env API_KEY node app.js
env:
  - API_KEY=secret123
```

### パスの解決
相対パスは絶対パスに解決される：
```bash
cderun --dry-run node ./app.js
# mounts の source などが絶対パスに解決される
mounts:
  - type: bind
    source: /home/user/project
    target: /workspace
```
