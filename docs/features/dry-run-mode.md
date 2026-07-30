# 機能仕様：ドライランモード (完了)

## 概要

実際にコンテナを実行せず、生成される中間表現（ContainerConfig）を表示する機能。

## 要件

### 基本動作

`--dry-run`フラグが指定された場合：

1. サブコマンドの指定が必須（指定がない場合はエラー）
2. 通常通り設定を読み込み、中間表現を生成
3. コンテナを実行せず、中間表現を表示
4. 終了コード0で終了

## 使用方法

### 基本的な使用

```bash
cderun --dry-run node --version
```

## 出力フォーマット

### YAML形式（デフォルト）

`cderun --dry-run node app.js`

```yaml
image: node:latest
command:
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
  - NODE_ENV=[REDACTED]
workdir: /workspace
user: ""
ports:
  - 8080:80
publish_all: false
expose:
  - "80/tcp"
hostname: node-app
dns:
  - 8.8.8.8
add_hosts:
  - "my-server:192.168.1.100"
privileged: false
cap_add:
  - SYS_ADMIN
cap_drop:
  - NET_RAW
entrypoint:
  - /usr/bin/node
pull: missing
memory: 536870912
cpus: 1.5
devices:
  - path_on_host: /dev/fuse
    path_in_container: /dev/fuse
    cgroup_permissions: rwm
```

### JSON形式

`cderun --dry-run --dry-run-format json node app.js`

```json
{
  "image": "node:latest",
  "command": ["app.js"],
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
  "env": ["NODE_ENV=[REDACTED]"],
  "workdir": "/workspace",
  "user": "",
  "ports": ["8080:80"],
  "publish_all": false,
  "expose": ["80/tcp"],
  "hostname": "node-app",
  "dns": ["8.8.8.8"],
  "add_hosts": ["my-server:192.168.1.100"],
  "privileged": false,
  "cap_add": ["SYS_ADMIN"],
  "cap_drop": ["NET_RAW"],
  "entrypoint": ["/usr/bin/node"],
  "pull": "missing",
  "memory": 536870912,
  "cpus": 1.5,
  "devices": [
    {
      "path_on_host": "/dev/fuse",
      "path_in_container": "/dev/fuse",
      "cgroup_permissions": "rwm"
    }
  ]
}
```

### 簡易形式

`cderun --dry-run --dry-run-format simple node app.js`

```text
Image: node:latest
Command: "app.js"
TTY: true
Interactive: true
Network: bridge
Remove: true
Mounts: type=bind,source="/home/user/project",target="/workspace",readonly=false
Env: "NODE_ENV"="[REDACTED]"
Workdir: /workspace
User:
Ports: 8080:80
PublishAll: false
Expose: 80/tcp
Hostname: node-app
DNS: 8.8.8.8
AddHosts: my-server:192.168.1.100
Privileged: false
CapAdd: SYS_ADMIN
CapDrop: NET_RAW
GroupAdd:
Devices: /dev/fuse
Memory: 512MiB
CPUs: 1.5
Entrypoint: "/usr/bin/node"
```

> **Note**: `Memory` は `512MiB` や `1GiB` のようなバイナリ単位（MiB/GiB）を用いた人間が読みやすい形式で表示され、`CPUs` は浮動小数点数（例: `1.5`）として表示されます。

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
  false
fi
```

### 3. 設定ファイルのドキュメント化

```bash
cderun --dry-run --dry-run-format yaml node app.js > config-example.yaml
```

## 他のフラグとの組み合わせ

### --log-levelとの組み合わせ

```bash
cderun --dry-run --log-level info node app.js
[INFO] Loading configuration from: /home/user/project/.cderun.yaml
[INFO] Resolved image: node:20-alpine
[INFO] Working directory: /home/user/project
[INFO] Environment variables: NODE_ENV=development
[INFO] Generated ContainerConfig:
image: node:20-alpine
...
```

## 実装上の注意

### 設定ファイル（YAML）でのサポート

ドライランモードの設定 (`dryRun`, `dryRunFormat`) は、設定ファイル (`.cderun.yaml`, `.tools.yaml`) でもサポートされています。これにより、特定のツールに対して常にドライランを適用したり、プロジェクト全体のデフォルトの出力形式を指定したりすることが可能です。

設定ファイル内でのキー名はキャメルケース（`dryRun`, `dryRunFormat`）を使用します。

### 環境変数の展開

ドライラン時も環境変数は実際の値に解決されるが、出力上はデフォルトで**すべての値がマスクされる**（Secure by Default。[機密データ保護](./sensitive-data-protection.md) を参照）：

```bash
export API_KEY=secret123
cderun --dry-run --env API_KEY node app.js
env:
  - API_KEY=[REDACTED]
```

実際の値を出力で確認したい場合は、`--sensitive-env=""` を指定してマスクを無効化する。

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
