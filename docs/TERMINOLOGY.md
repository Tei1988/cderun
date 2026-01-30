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

## 引数・フラグの種類

### cderun内部オーバーライド (Internal Overrides / P1)
`--cderun-` で始まるフラグ。優先順位が最も高く（P1）、サブコマンドの後ろに配置されても `cderun` 自体の設定として解釈されます。内部的な前処理（Hoisting）によって、常にサブコマンドの前に移動した状態でパースされます。

### cderun標準フラグ (Standard Flags / P2)
`--tty` や `--env` など、`cderun` の動作を制御する標準的なフラグ。これらは**サブコマンドの前**に配置する必要があります。

### パススルー引数 (Passthrough Args)
サブコマンド名より後ろにある、`cderun` 内部オーバーライド以外の全ての引数。これらはコンテナ内のサブコマンドにそのまま渡されます。

## 環境変数の引き継ぎ

- **実行ホストの環境変数**: `cderun --env MY_VAR` で明示的に指定すれば引き継げます。
- **基底ホストの環境変数**: 実行ホストを経由しないと引き継げません。
