# Feature: Diagnosis Mode

## 概要

システムの診断情報（ランタイムの状態、設定ファイルの読み込み状況）および利用可能なツールの一覧を表示する機能。

## 要件

### 基本動作

`--diagnosis`フラグが指定された場合：

1. システム診断情報と利用可能なツールの一覧を収集
2. 設定されたフォーマットで情報を表示
3. コンテナの実行（およびドライラン）をスキップ
4. 終了コード0で終了

## 使用方法

### 基本的な使用

```bash
cderun --diagnosis
```

### サブコマンドとの併用

サブコマンドが指定されていても、診断モードが優先されます。

```bash
cderun --diagnosis node --version
```

## 出力フォーマット

### YAML形式（デフォルト）

`cderun --diagnosis`

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
  - git
  - node
  - python
```

### JSON形式

`cderun --diagnosis --diagnosis-format json`

```json
{
  "runtime": {
    "name": "docker",
    "socket": "/var/run/docker.sock",
    "status": "accessible"
  },
  "configs": {
    "global": [
      "/home/user/.cderun.yaml"
    ],
    "tools": [
      "/home/user/project/.tools.yaml"
    ]
  },
  "available_tools": [
    "git",
    "node",
    "python"
  ]
}
```

### 簡易形式

`cderun --diagnosis --diagnosis-format simple`

```text
Runtime: docker (/var/run/docker.sock)
Runtime Status: accessible
Global Config: /home/user/.cderun.yaml
Tools Config: /home/user/project/.tools.yaml
Available Tools: git, node, python
```

## P1 Internal Overrides

他のフラグ同様、`--cderun-` プレフィックスを用いた Priority 1 オーバーライドが可能です。サブコマンドの後に指定する場合に使用します。

```bash
cderun node app.js --cderun-diagnosis
```

## 環境変数

`CDERUN_DIAGNOSIS` 環境変数を `true` に設定することで、フラグなしで診断モードを有効にできます。

```bash
export CDERUN_DIAGNOSIS=true
cderun
```

### 出力フォーマットの環境変数

`CDERUN_DIAGNOSIS_FORMAT` 環境変数を使用して、出力フォーマットを制御できます。

```bash
export CDERUN_DIAGNOSIS_FORMAT=json
cderun --diagnosis
```

`CDERUN_DIAGNOSIS=true` と組み合わせることで、フラグなしで特定のフォーマットの診断出力を得ることができます。

```bash
export CDERUN_DIAGNOSIS=true
export CDERUN_DIAGNOSIS_FORMAT=json
cderun
```

利用可能な値は `yaml`（デフォルト）、`json`、`simple` です。
