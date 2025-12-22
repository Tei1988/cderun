# Feature: Strict Argument Parsing Strategy

## 概要
`cderun` はラッパーツールであるため、自身のフラグと、ラップする対象（コンテナ内コマンド）へのフラグを厳密に区別しなければならない。

## パースのルール

### 1. フラグの境界線 (Boundary)
最初の「非フラグ引数（Non-flag argument）」を**サブコマンド名**と見なし、それ以降のパースを停止する境界線とする。

### 2. 挙動の詳細
コマンドライン引数は以下の順序で解釈されなければならない。

`cderun [cderun-flags] <subcommand> [passthrough-args]`

- **[cderun-flags]**: サブコマンド名の**前**にあるフラグのみを `cderun` の設定として解釈する。
- **\<subcommand\>**: 最初の位置引数（例: `node`, `docker`, `python`）。
- **[passthrough-args]**: サブコマンド名より**後**にある全ての引数は、たとえ `cderun` に存在するフラグ名（例: `--tty`）であっても、文字列としてそのまま保持し、コンテナへの引数として渡すこと。

## テストケース要件
以下のコマンドを実行した際の結果が保証されるテストを作成すること。

```bash
$ cderun --tty docker --tty
