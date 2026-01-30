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
$ cderun --dry-run node --version
```

## 出力フォーマット

### YAML形式（デフォルト）
```bash
$ cderun --dry-run node app.js
image: node:latest
command:
  - node
args:
  - app.js
tty: true
interactive: true
remove: true
volumes:
  - hostPath: /home/user/project
    containerPath: /workspace
    readOnly: false
env:
  - NODE_ENV=development
workdir: /workspace
```

### JSON形式
```bash
$ cderun --dry-run --format json node app.js
{
  "image": "node:latest",
  "command": ["node"],
  "args": ["app.js"],
  "tty": true,
  "interactive": true,
  "remove": true,
  "volumes": [
    {
      "hostPath": "/home/user/project",
      "containerPath": "/workspace",
      "readOnly": false
    }
  ],
  "env": ["NODE_ENV=development"],
  "workdir": "/workspace"
}
```

### 簡易形式
```bash
$ cderun --dry-run --format simple node app.js
Image: node:latest
Command: node app.js
Volumes: /home/user/project:/workspace
Env: NODE_ENV=development
Workdir: /workspace
```

## ユースケース

### 1. デバッグ
設定が正しく適用されているか確認：
```bash
$ cderun --dry-run python script.py
```

### 2. 設定の検証
```bash
#!/bin/bash
output=$(cderun --dry-run --format json node --version)
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
$ cderun --dry-run --format yaml node app.js > config-example.yaml
```

## 他のフラグとの組み合わせ

### --verboseとの組み合わせ
```bash
$ cderun --dry-run --verbose node app.js
[INFO] Loading configuration from: /home/user/project/.cderun.yaml
[INFO] Resolved image: node:20-alpine
[INFO] Working directory: /home/user/project
[INFO] Environment variables: NODE_ENV=development
[INFO] Generated ContainerConfig:
image: node:20-alpine
command: [node]
args: [app.js]
...
```

## 実装上の注意

### 環境変数の展開
ドライラン時も環境変数は実際の値に展開される：
```bash
$ export API_KEY=secret123
$ cderun --dry-run --env API_KEY node app.js
env:
  - API_KEY=secret123
```

### パスの解決
相対パスは絶対パスに解決される：
```bash
$ cderun --dry-run node ./app.js
volumes:
  - hostPath: /home/user/project
    containerPath: /home/user/project
```
