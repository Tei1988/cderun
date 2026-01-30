# Feature: Strict Argument Parsing Strategy (Completed)

## 概要
`cderun` はラッパーツールであるため、自身のフラグと、ラップする対象（コンテナ内コマンド）へのフラグを厳密に区別しなければならない。

## パースのルール

### 1. フラグの境界線 (Boundary)
最初の「非フラグ引数（Non-flag argument）」を**サブコマンド名**と見なし、それ以降のパースを停止する境界線とする。

### 2. cderun内部オーバーライド (P1) の例外
`--cderun-` で始まるフラグ（P1内部オーバーライド）は、境界線より後に指定された場合でも、前処理（Hoisting）によってサブコマンドの前に移動されます。これにより、サブコマンドの後ろに配置しても `cderun` 自体の設定として常に解釈されます。

### 3. 挙動の詳細
コマンドライン引数は以下の順序で解釈されます。

`cderun [cderun-flags] <subcommand> [passthrough-args]`

- **[cderun-flags]**: サブコマンド名の**前**にあるフラグ、および位置に関わらず指定された `--cderun-` フラグ。
- **\<subcommand\>**: 最初の位置引数（例: `node`, `docker`, `python`）。
- **[passthrough-args]**: サブコマンド名より**後**にある引数のうち、`--cderun-` で始まらない全ての引数。

## テストケース要件
以下のコマンドを実行した際の結果が保証されるテストを作成すること。

```bash
$ cderun --tty docker --tty
