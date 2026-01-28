# Feature: Environment Variable Passthrough

## 概要

実行ホストの環境変数を選択的にコンテナに引き継ぐ機能。
**デフォルトでは環境変数は引き継がれない。**明示的に指定した環境変数のみがコンテナに渡される。

## 中間表現での扱い

```go
type ContainerConfig struct {
    // ...
    Env []string  // ["KEY=value", "KEY2=value2", "KEY3"] の形式
}
```

### env配列の形式

1. **`KEY=value`** - 明示的な値を設定
2. **`KEY`** - 実行ホストの環境変数から値を取得（パススルー）

```go
config := ContainerConfig{
    Env: []string{
        "NODE_ENV=production",  // 明示的な値
        "NPM_TOKEN",             // 実行ホストから取得
        "HOME",                  // 実行ホストから取得
    },
}

// 実行時に変換
resolvedEnv := []string{
    "NODE_ENV=production",
    "NPM_TOKEN=" + os.Getenv("NPM_TOKEN"),
    "HOME=" + os.Getenv("HOME"),
}
```

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
# 出力: undefined
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

## 実装

### 環境変数の解決

```go
func ResolveEnv(envList []string) []string {
    resolved := make([]string, 0, len(envList))
    
    for _, env := range envList {
        if strings.Contains(env, "=") {
            // KEY=value 形式 → そのまま使用
            resolved = append(resolved, env)
        } else {
            // KEY 形式 → 実行ホストから取得
            key := env
            value := os.Getenv(key)
            resolved = append(resolved, fmt.Sprintf("%s=%s", key, value))
        }
    }
    
    return resolved
}
```

### 使用例

```go
config := ContainerConfig{
    Env: []string{
        "NODE_ENV=production",
        "NPM_TOKEN",
        "HOME",
    },
}

// 実行前に解決
config.Env = ResolveEnv(config.Env)
// → ["NODE_ENV=production", "NPM_TOKEN=secret123", "HOME=/home/alice"]

runtime.CreateContainer(ctx, config)
```

## デバッグ

### dry-runでの確認
```bash
$ cderun --dry-run node app.js
env:
  - NODE_ENV=production
  - NPM_TOKEN=secret123
  - HOME=/home/alice
```
