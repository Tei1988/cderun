# Project Context for AI Agents

## 1. Introduction

このリポジトリは、Go言語（Golang）によるCLIツールの開発プロジェクトです。

あなたはAI開発パートナーとして、このプロジェクトの実装、リファクタリング、テスト、ドキュメント作成を支援します。

## 2. Directory Structure & Knowledge Base

プロジェクトのルールと知識は、以下のドキュメントに分割されています。

作業を開始する前に、必ずこれらを参照してください。

| ファイルパス | 内容 | 重要度 |
| --- | --- | --- |
| docs/guidelines/working-guide.md | 作業フロー、コーディング規約、プロジェクト構成  実装を進める際の手順とルールです。必ず遵守してください。 | 高 (Must Read) |
| docs/guidelines/testing.md | テスト実装指針  リーク防止やモックの作り方などの詳細なガイドラインです。必ず遵守してください。 | 高 (Must Read) |
| docs/architecture/libraries.md | 技術スタック、使用ライブラリ  使用すべきライブラリやツール選定の基準です。 | 高 (Must Read) |
| docs/features/*.md | 機能要件定義書  個別の機能（コマンド）の実装詳細です。実装時はこれを正とします。 | 中 (Reference) |
| docs/testing/*.md | テスト関連ドキュメント  E2E・統合テストの設計方針、カバレッジ計測、テスト構成計画。**テストを実装する際は必ず参照すること。** | 高 (Must Read) |

## 3. Core Principles

1. **Context-Aware:**
   常に docs/ 以下の最新情報をコンテキストとして持ち、既存の設計思想から逸脱しないようにしてください。特に `docs/guidelines/` 以下のガイドラインを最優先で遵守してください。
1. **Document-First:**
   コードを書く前に、必ず関連する features ドキュメントを読み込んでください。
   ドキュメントがない機能の実装を求められた場合は、まずドキュメントの作成（または作成依頼）から始めてください。
1. **Testing-First:**
  テストを追加または修正する際は、必ず `docs/testing/` 以下のドキュメントを読み込んでください。
  特に `organization.md` に定義されている命名規則とカテゴリの分類を厳守してください。
1. **Clean Code:**
   Goの標準的なイディオムに従い、保守性の高いコードを生成してください。
   特に、時間軸を含む命名（`new_flag`, `old_config` 等）は避け、機能や役割を明示した命名を徹底してください。
1. **Pragmatic Documentation:**
   実装上の制約や技術的な理由でfeaturesドキュメントと矛盾が生じる場合、ドキュメントを修正して実装と一致させることが許可されます。
   ただし、変更理由を明確に記録し、ユーザーに報告してください。
1. **Markdown Formatting:**
  すべての `.md` ファイルにおいて、`markdownlint` に準拠したフォーマットを統一してください。
  - リストのインデントは **2スペース** (MD007)。
  - コードフェンス内の単一コマンドに `$` を付けない (MD014)。
  - コードブロックには必ず言語識別子を指定する (MD040)。
  詳細なルールは `docs/guidelines/working-guide.md` を参照してください。

**Note to User:** AIに指示を出す際は、「docs/features/xxx.md に基づいて実装して」と伝えると最も精度が高くなります。
