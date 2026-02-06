# Feature: Environment Variable Passthrough (Completed)

## 概要

実行ホストの環境変数を選択的にコンテナに引き継ぐ機能。
**デフォルトでは環境変数は引き継がれない。**明示的に指定した環境変数のみがコンテナに渡される。

`.tools.yaml`（優先順位 P4）、`--env` フラグ（優先順位 P2）、`--cderun-env` フラグ（優先順位 P1）による指定がサポートされており、`KEY=value` 形式（明示的指定）と `KEY` 形式（ホストからの取得）の両方に対応しています。

## 中間表現での扱い

`ContainerConfig.Env` は `[]string` 型で保持し、各要素は以下のいずれかの形式をとる。

### env配列の形式

1. **`KEY=value`** (明示的指定): 指定された値をそのまま使用。
2. **`KEY`** (パススルー): 実行ホストの環境変数から値を取得して `KEY=value` 形式に変換。

## 設定方法

### ツール設定
```yaml
# .tools.yaml
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

高い優先順位のソース（CLI、環境変数、設定ファイル）に値がある場合、それより低い優先順位のソースはすべて無視されます（上書き）。

```yaml
# .tools.yaml
node:
  env:
    - NODE_ENV=development
    - PORT=3000
```

```bash
cderun --env NODE_ENV=production node app.js
# → NODE_ENV=production のみがコンテナに渡されます。
# 設定ファイル内の PORT=3000 は無視されます。
```

### 同じソース内でキーが複数回指定された場合

```yaml
# .tools.yaml
node:
  env:
    - NODE_ENV=development
    - NODE_ENV=production  # この値が使われる
```

## 実行例

### 例1: 明示的な値の設定
```yaml
# .tools.yaml
node:
  env:
    - NODE_ENV=production
    - PORT=3000
```

```bash
cderun node app.js
# ContainerConfig.Env = ["NODE_ENV=production", "PORT=3000"]
```

### 例2: 実行ホストから取得
```yaml
# .tools.yaml
node:
  env:
    - NPM_TOKEN  # 実行ホストから取得
    - HOME       # 実行ホストから取得
```

```bash
export NPM_TOKEN=secret123
export HOME=/home/alice
cderun node app.js
# 実行時に解決:
# ContainerConfig.Env = ["NPM_TOKEN=secret123", "HOME=/home/alice"]
```

### 例3: 混在
```yaml
# .tools.yaml
node:
  env:
    - NODE_ENV=production  # 明示的
    - NPM_TOKEN            # 実行ホストから
    - PORT=3000            # 明示的
```

```bash
export NPM_TOKEN=secret123
cderun node app.js
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
cderun --env NONEXISTENT node -e "console.log(process.env.NONEXISTENT)"
# ContainerConfig.Env = ["NONEXISTENT="]
# 出力: "" (空文字列)
```

### 厳密モード (Strict Mode)
`strictEnv` を `true` に設定すると、指定された環境変数が実行ホストに存在しない場合にエラーを返します。

#### 設定方法
`.cderun.yaml`（グローバル）または `.tools.yaml`（ツール固有）で設定可能です。

```yaml
# .cderun.yaml
defaults:
  strictEnv: true
```

または環境変数で指定：
```bash
export CDERUN_STRICT_ENV=true
```

#### 挙動
```bash
cderun node app.js
Error: required environment variable not found: NPM_TOKEN
```

## 環境変数の解決ロジック

コンテナを作成する前に、`Env` 配列内の各要素をスキャンし、`=` を含まない要素（キーのみの指定）については、実行ホストの `os.Getenv(key)` を呼び出して値を解決する。解決された値は `KEY=value` の形式でランタイムAPIに渡される。

## ベストプラクティスと使い分け

デフォルト動作と厳密モード（Strict Mode）の使い分けの指針を以下に示します。

### デフォルト動作 (strictEnv: false)
- **特徴**: 指定した変数がホストになくてもエラーにせず、空文字としてコンテナに渡します。
- **メリット**: 柔軟性が高く、一部の環境変数が欠けていても動作が継続できる場合に便利です。
- **推奨されるユースケース**:
  - アドホックな開発作業。
  - オプショナルな設定（ログレベルの変更など）をパススルーする場合。

### 厳密モード (strictEnv: true)
- **特徴**: 指定した変数がホストに存在しない場合、即座にエラーで停止します（Fail Fast）。
- **メリット**: 設定漏れによるサイレントな失敗（意図しないデフォルト値での動作など）を確実に防げます。
- **推奨されるユースケース**:
  - **認証情報/シークレット**: `NPM_TOKEN`, `AWS_ACCESS_KEY_ID` など、欠落すると正常に動作しないことが明らかな場合。
  - **CI/CD環境**: 実行環境の一貫性が強く求められる自動化パイプライン。
  - **チーム共有設定**: `.tools.yaml` をチームで共有しており、全員の環境が正しくセットアップされていることを保証したい場合。

## デバッグ

### dry-runでの確認
```bash
cderun --dry-run node app.js
env:
  - NODE_ENV=production
  - NPM_TOKEN=secret123
  - HOME=/home/alice
```
