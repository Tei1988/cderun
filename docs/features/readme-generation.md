# Feature: README Generation Strategy

## 概要
`README.md` は静的なファイルではなく、実装されたコード（フラグ、設定構造体、ロジック）を反映した動的な成果物として扱われるべきである。
実装完了後、以下の構成要件に従って `README.md` (英語) を生成/更新すること。

## Source of Truth (情報源)
READMEの内容は、以下の情報と整合性が取れていなければならない。
1. **実装コード**: `cmd/` 内で定義された実際のフラグ名、デフォルト値。
2. **構造体定義**: `config` パッケージ等で定義された YAML マッピング構造（`image` の多態性など）。
3. **Feature Docs**: `docs/features/` 配下の仕様（優先順位ロジックなど）。

## READMEの構成要件 (Structure Requirements)

### 1. Header & Concept
以下のコンセプト文言を冒頭に使用すること。

> **Concept**
> "All you need on your local machine is Docker."
> `cderun` generates ephemeral containers for commands like `node`, `python`, or `git` on demand. It keeps your host clean and ensures reproducible environments defined in a single YAML file.

### 2. Usage Section
以下の3つのパターンを、実際のコードの挙動に基づいて説明すること。
- **Wrapper Mode**: `cderun [flags] <subcommand>`
- **Symlink Mode**: `ln -s cderun node` -> `./node`
- **Ad-hoc Mode**: `cderun --image=ubuntu bash`

### 3. Argument Parsing & Flags (Important)
`04-argument-parsing.md` の仕様に基づき、**「どこまでが cderun の引数で、どこからがコンテナへの引数か」** を明確に図解または例示すること。

### 4. Configuration Schema
実装された構造体に基づき、網羅的な `cderun.yaml` の例を提示すること。
特に以下の高度な機能を省略せずに記載すること。
- **Mount Cderun**: ホストの `cderun` をコンテナ内にマウントする設定。
- **Priority Logic**: P1〜P6 の優先順位（Env vs Flag vs Config）。

## 出力フォーマット
- 言語: 英語 (English)
- 形式: Markdown
