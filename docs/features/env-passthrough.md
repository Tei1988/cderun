# Feature: Environment Variable Passthrough (Completed)

## 概要

実行ホストの環境変数を選択的にコンテナに引き継ぐ機能。
**デフォルトでは環境変数は引き継がれない。**明示的に指定した環境変数のみがコンテナに渡される。

現状では、`.tools.yaml`（優先順位 P4: ツール別設定）での環境変数指定（`KEY=value` 形式のみ）がサポートされており、ホスト環境変数の解決（`KEY` のみの指定）や `--env` フラグによる指定は Phase 3 で実装予定です。

## 中間表現での扱い

`ContainerConfig.Env` は `[]string` 型で保持し、各要素は以下のいずれかの形式をとる。

### env配列の形式

1. **`KEY=value`** (明示的指定): 指定された値をそのまま使用。
2. **`KEY`** (パススルー): 実行ホストの環境変数から値を取得して `KEY=value` 形式に変換。

## 設定方法

### ツール設定
```yaml
tools:
  node:
    env:
      - NODE_ENV=production      # 明示的な値
      - NPM_TOKEN                 # 実行ホストから取得
      - HOME                      # 実行ホストから取得
```

### コマンドライン
```bash
# 明示的な値を設定
cderun --env NODE_ENV=production node app.js

# 実行ホストから取得
cderun --env NPM_TOKEN --env HOME node app.js

# 混在
cderun --env NODE_ENV=production --env NPM_TOKEN node app.js
```

## 優先順位

後から指定された値が優先される：

```yaml
tools:
  node:
    env:
      - NODE_ENV=development  # 設定ファイル
```

```bash
$ cderun --env NODE_ENV=production node app.js
# → NODE_ENV=production が使われる（コマンドラインが優先）
```

### 同じキーが複数回指定された場合

```yaml
tools:
  node:
    env:
      - NODE_ENV=development
      - NODE_ENV=production  # この値が使われる
```

## 実行例

### 例1: 明示的な値の設定
```yaml
tools:
  node:
    env:
      - NODE_ENV=production
      - PORT=3000
```

```bash
$ cderun node app.js
# ContainerConfig.Env = ["NODE_ENV=production", "PORT=3000"]
```

### 例2: 実行ホストから取得
```yaml
tools:
  node:
    env:
      - NPM_TOKEN  # 実行ホストから取得
      - HOME       # 実行ホストから取得
```

```bash
$ export NPM_TOKEN=secret123
$ export HOME=/home/alice
$ cderun node app.js
# 実行時に解決:
# ContainerConfig.Env = ["NPM_TOKEN=secret123", "HOME=/home/alice"]
```

### 例3: 混在
```yaml
tools:
  node:
    env:
      - NODE_ENV=production  # 明示的
      - NPM_TOKEN            # 実行ホストから
      - PORT=3000            # 明示的
```

```bash
$ export NPM_TOKEN=secret123
$ cderun node app.js
# ContainerConfig.Env = [
#   "NODE_ENV=production",
#   "NPM_TOKEN=secret123",
#   "PORT=3000"
# ]
```

## 環境変数が存在しない場合

### デフォルト動作
実行ホストに存在しない環境変数は空文字列として渡される：

```bash
$ cderun --env NONEXISTENT node -e "console.log(process.env.NONEXISTENT)"
# ContainerConfig.Env = ["NONEXISTENT="]
# 出力: "" (空文字列)
```

### 厳密モード（将来の拡張）
```yaml
cderun:
  defaults:
    strictEnv: true  # 存在しない環境変数でエラー
```

```bash
$ cderun node app.js
Error: Required environment variable not found: NPM_TOKEN
```

## 環境変数の解決ロジック

コンテナを作成する前に、`Env` 配列内の各要素をスキャンし、`=` を含まない要素（キーのみの指定）については、実行ホストの `os.Getenv(key)` を呼び出して値を解決する。解決された値は `KEY=value` の形式でランタイムAPIに渡される。

## デバッグ

### dry-runでの確認
```bash
$ cderun --dry-run node app.js
env:
  - NODE_ENV=production
  - NPM_TOKEN=secret123
  - HOME=/home/alice
```
