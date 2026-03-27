# 機能仕様：引数解析

## 概要

`cderun` はラッパーツールであり、自身のフラグと、コンテナ内で実行するコマンドへの引数を厳密に区別する。

## 基本構文

```text
cderun [cderun-flags] <subcommand> [passthrough-args]
```

- **[cderun-flags]**: `cderun` 自体の動作を制御するフラグ。サブコマンドの**前**に置く標準フラグ（P2）と、`--cderun-` で始まる内部オーバーライドフラグ（P1）に分けられる。
- **\<subcommand\>**: 最初の非フラグ引数。`.tools.yaml` から設定を読み込むための**キー**としてのみ使用される。コンテナの `CMD` には含まれない。
- **[passthrough-args]**: サブコマンド以降の全引数。コンテナ内で実行されるコマンドの引数として渡される。`--cderun-` フラグが含まれる場合は前方にホイストされる。

## イメージ名の決定

以下の優先順位で決定される。詳細は [引数・設定優先順位](./argument-priority-logic.md) を参照。

1. `--image` / `--cderun-image` フラグ (P1/P2)
2. `CDERUN_IMAGE` 環境変数 (P3)
3. `<subcommand>` をキーとして `.tools.yaml` から検索 (P4)
4. いずれも解決できない場合はエラー終了

## コンテナに渡されるコマンド

- `<subcommand>` はキーとして消費され、コンテナの `CMD` には含まれない
- コンテナの `CMD` は `[passthrough-args]` のみで構成される

### 例

```bash
# .tools.yaml に my-tool: {image: alpine, entrypoint: [/usr/bin/my-tool-impl]} がある場合
cderun my-tool -l -a
# → コンテナ内で /usr/bin/my-tool-impl -l -a が実行される

# --image で明示指定する場合
cderun --image=golang:1.22 --entrypoint=go go --version
# → コンテナ内で go --version が実行される（go はキーとして消費）

# イメージが特定できない場合
cderun go --version
# → エラー
```

## P1 内部オーバーライドのホイスト

`--cderun-` で始まるフラグは、サブコマンドの**後ろ**に置かれていても、前処理（`preprocessArgs`）によってサブコマンドの**前**に移動（ホイスト）され、`cderun` 自体の設定として解釈されます。

### ホイストの仕組み

cderun は実行時に引数リストをスキャンし、`--cderun-` プレフィックスを持つフラグを発見すると、それをサブコマンドの境界を越えて前方に移動させます。これにより、Cobra などのコマンドラインパーサーがそれらを `cderun` コマンド自体のフラグとして正しく認識できるようになります。

```bash
# 実行時の入力
cderun node app.js --cderun-tty --cderun-image node:20-alpine

# 内部的なホイスト後の引数
cderun --cderun-tty --cderun-image node:20-alpine node app.js
```

### ホイストの重要性とシンボリックリンク

ホイストは、特に**シンボリックリンク（ポリグロットモード）**において重要な役割を果たします。

ポリグロットモードでは、サブコマンド名の後にある引数のうち、**`--cderun-` プレフィックスが付いたフラグのみ**がホイストされます。これにより、ラップ対象のツールが持つ同名のフラグ（例: `--tty`, `--env`）との衝突を回避できます。

```bash
# node が cderun へのシンボリックリンクの場合
node --env DEBUG=app app.js --cderun-env NODE_ENV=production
```

この例では：

1. `--env DEBUG=app` はホイストされず、`node`（コンテナ内の実行コマンド）にそのまま渡されます。
2. `--cderun-env NODE_ENV=production` はホイストされ、`cderun` 自体の環境変数設定として処理されます。

詳細は [polyglot-entry.md](./polyglot-entry.md) を参照してください。

### サブコマンドを必要としないモード

`--diagnosis` フラグが指定された場合、`cderun` は診断モードとして動作し、サブコマンドの指定を必要としません。

このモードでも `preprocessArgs` による前処理（`--cderun-` フラグのホイスト等）は通常通り実行されますが、診断情報の出力後にプログラムが早期終了（Early Return）するため、ホイストされたフラグの多く（イメージの上書きや環境変数の追加など）は実際の動作に影響を与えません。

## テストケース要件

```bash
cderun --tty docker --tty
# 最初の --tty は cderun フラグ、後の --tty は docker へのパススルー引数
```
