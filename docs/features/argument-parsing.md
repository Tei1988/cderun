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

`--cderun-` で始まるフラグはサブコマンドの後に置かれていても、前処理によってサブコマンドの前に移動（ホイスト）され、`cderun` 自体の設定として解釈される。

ポリグロットモードでは `--cderun-` フラグのみがホイスト対象となる。`--tty` などのプレフィックスなし標準フラグはホイストされず、パススルー引数として扱われる。詳細は [polyglot-entry.md](./polyglot-entry.md) を参照。

## テストケース要件

```bash
cderun --tty docker --tty
# 最初の --tty は cderun フラグ、後の --tty は docker へのパススルー引数
```
