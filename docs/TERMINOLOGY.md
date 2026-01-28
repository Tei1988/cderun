# 用語定義

## ホストの種類

cderunをネストして実行する場合、以下のように呼び分ける：

### 基底ホスト (Base Host)
最初にcderunを実行した物理マシンまたはVM。

```bash
# 基底ホスト
$ cderun --mount-cderun gemini-cli
```

### 実行ホスト (Execution Host)
直前にcderunを実行したホスト（コンテナまたは基底ホスト）。

```bash
# 基底ホスト
$ cderun --mount-cderun gemini-cli

# gemini-cliコンテナ（実行ホスト）
$ cderun python script.py
```

## 例

```
基底ホスト (物理マシン)
  ↓ cderun gemini-cli
gemini-cliコンテナ (実行ホスト)
  ↓ cderun python script.py
pythonコンテナ
```

この場合：
- **基底ホスト**: 物理マシン
- **実行ホスト**: gemini-cliコンテナ（pythonを起動する直前のホスト）

## 環境変数の引き継ぎ

- **実行ホストの環境変数**: `cderun --env MY_VAR` で明示的に指定すれば引き継げる
- **基底ホストの環境変数**: 実行ホストを経由しないと引き継げない
