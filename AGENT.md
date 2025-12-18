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
| docs/architecture/libraries.md | 技術スタック、使用ライブラリ  使用すべきライブラリやツール選定の基準です。 | 高 (Must Read) |
| docs/features/*.md | 機能要件定義書  個別の機能（コマンド）の実装詳細です。実装時はこれを正とします。 | 中 (Reference) |

## 3. Core Principles

1. **Context-Aware:**  
   常に docs/ 以下の最新情報をコンテキストとして持ち、既存の設計思想から逸脱しないようにしてください。
1. **Document-First:**  
   コードを書く前に、必ず関連する features ドキュメントを読み込んでください。
   ドキュメントがない機能の実装を求められた場合は、まずドキュメントの作成（または作成依頼）から始めてください。
1. **Clean Code:**  
   Goの標準的なイディオムに従い、保守性の高いコードを生成してください。

**Note to User:** AIに指示を出す際は、「docs/features/xxx.md に基づいて実装して」と伝えると最も精度が高くなります。
