# Working Guide & Coding Standards
開発を進める上でのワークフロー、ディレクトリ構成、コーディング規約です。

## 1. Development Workflow
機能追加や修正を行う際は、以下の **"Spec-First" サイクル** を回してください。

1. **Understand Specs (要件理解)**
   - ユーザーの指示に対応する `docs/features/` 以下のMarkdownファイルを確認する。
   - ドキュメントの内容とコードの現状に乖離がないか確認する。
1. **Plan (計画)**
   - どのパッケージを変更するか、新しいファイルをどこに作成するかをユーザーに提示する。
   - ディレクトリ構成（後述）に従っているか確認する。
1. **Implement (実装)**
   - テストコード（`_test.go`）の実装を推奨する（可能な限りTDDライクに）。
   - 実装を行う。
1. **Update Docs (ドキュメント更新)**
   - 実装中に仕様変更が発生した場合、コードだけでなく `docs/features/` の該当ファイルも更新する。
   - **重要**: 実装上の制約や技術的な理由でfeaturesドキュメントと矛盾が生じる場合、ドキュメントを修正して実装と一致させることが許可される。
   - 修正する場合は、変更理由をコミットメッセージに明記する。

## 2. Project Layout
Standard Go Project Layout に準拠します。

```text
.
├── cmd/
│   └── [app-name]/
│       └── main.go       # エントリーポイント（極力シンプルに）
├── internal/             # 外部からimportされたくないコード
│   ├── command/          # Cobraのコマンド定義 (cmd/root.go, cmd/subcmd.go)
│   ├── usecase/          # アプリケーションのビジネスロジック
│   └── util/             # 汎用ユーティリティ
├── pkg/                  # (Optional) 外部公開しても良いライブラリコード
├── docs/
│   ├── features/         # 機能要件
│   ├── architecture/     # アーキテクチャ・ライブラリ選定
│   └── guidelines/       # このファイル
└── tests/                # 統合テスト（必要な場合）
```

## 3. Coding Guidelines
### General
- **Effective Go:** Goの公式スタイルガイドに従う。
- **Error Handling:**
  - エラーを握り潰さない（`_` で捨てない）。
  - エラーを返す際は、コンテキストを付与する: `fmt.Errorf("failed to open file: %w", err)`
- **Structs:** 構造体のフィールドには適切なタグ（`json:"..."`, `yaml:"..."`）を付与する。
### CLI Best Practices
- **Stdout vs Stderr:**
  - 正常な出力結果（パイプで渡すデータなど）: `Stdout`
  - ログ、警告、エラーメッセージ、進捗バー: `Stderr`
- **Exit Codes:**
  - 成功: `0`
  - エラー: `1` (または適切な非ゼロの値)
### Testing
- **Table-Driven Tests:** 複数のケースを検証する場合は、テーブル駆動テストを使用する。
- **Mocking:** 外部APIやファイルシステムへの依存は、インターフェースを切ってモック可能にする。
