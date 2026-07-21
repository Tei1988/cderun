# Repository Guidelines for AI Agents

このリポジトリは、Go言語によるCLIツール `cderun`（Docker / Podman / containerd 上でエフェメラルコンテナとしてコマンドを実行するツール）の開発プロジェクトです。

あなたはAI開発パートナーとして、このプロジェクトの実装、リファクタリング、テスト、ドキュメント作成を支援します。

## 1. 環境セットアップ

```bash
bash scripts/setup-agent-env.sh
```

- Go のバージョンは `go.mod` の `go` directive に従います（Go 1.21+ の `GOTOOLCHAIN` 自動ダウンロードにより、ホストの Go が新しくても正しいツールチェーンが使われます）
- コンテナランタイム（Docker 等）は**ユニットテストには不要**です

## 2. ビルドと検証

作業完了前に、必ず以下がすべてパスすることを確認してください。

```bash
make build          # ビルド (go build)
make test           # ユニット・統合テスト (go test ./...)
make lint-go        # golangci-lint（.golangci.yml 準拠）
```

Markdown を変更した場合は追加で:

```bash
make lint-md        # markdownlint（.markdownlint.json 準拠）
make link-check     # ドキュメント内リンクの検証
```

**実行してはいけないもの**: `make test-runtime`（`-tags=runtime`）は実際のコンテナランタイムが必要です。実行環境に Docker 等がない場合は実行せず、CI に任せてください。

## 3. タスクの進め方

バックログは [.agent/todo.md](./.agent/todo.md) にあります。また、タスクを安全かつ再現性高く操作するための自動化スクリプトが `.agent/manage_task.py` に用意されています。
**エージェントは、タスクの確認、完了時のステータス更新や詳細手順のクリーンアップに、必ずこのスクリプトを最優先で活用してください。**

1. 着手前に本ファイル → `.agent/todo.md` → `docs/guidelines/working-guide.md` を必ず読む
2. タスク一覧の確認には `python3 .agent/manage_task.py list` を使用する。詳細手順の抽出には `python3 .agent/manage_task.py show <ID>` を使用する
3. 原則 **1 タスク = 1 PR**。タスクの「完了条件」をすべて満たすこと
4. タスク内の file:line は記録時点のもの。ズレていたら grep で再特定する
5. **Spec-First**: 「仕様変更あり」のタスクは、対応する `docs/features/*.md` の更新が完了条件に含まれる
6. タスク完了時は `python3 .agent/manage_task.py done <ID>` を実行し、サマリテーブルの更新と詳細手順の削除を自動で行う

## 4. ナレッジベース（必読ドキュメント）

| ファイルパス | 内容 | 重要度 |
| --- | --- | --- |
| docs/guidelines/working-guide.md | 作業フロー、コーディング規約、新オプション追加チェックリスト | 高 (Must Read) |
| docs/testing/strategy.md | テスト戦略・原則・作成チェックリスト。**テストを書く前に必読** | 高 (Must Read) |
| docs/guidelines/testing.md | テスト実装指針（リーク防止、モックの作り方） | 高 (Must Read) |
| docs/testing/*.md | テスト構成・命名規則（organization.md）、統合・ランタイムテスト | 高 (Must Read) |
| docs/architecture/libraries.md | 技術スタック、承認済みライブラリ | 高 (Must Read) |
| docs/features/*.md | 機能要件定義書。**実装時はこれを正とする** | 中 (Reference) |

## 5. Core Principles

1. **Context-Aware:**
   常に docs/ 以下の最新情報をコンテキストとして持ち、既存の設計思想から逸脱しないようにしてください。特に `docs/guidelines/` 以下のガイドラインを最優先で遵守してください。
1. **Document-First:**
   コードを書く前に、必ず関連する features ドキュメントを読み込んでください。
   ドキュメントがない機能の実装を求められた場合は、まずドキュメントの作成（または作成依頼）から始めてください。
1. **Testing-First:**
   テストを追加または修正する際は、必ず `docs/testing/strategy.md` のテスト原則（テストの正は仕様であり、カバレッジ駆動のテスト追加は禁止）とテスト作成チェックリストを遵守してください。命名規則は `docs/testing/organization.md` に従います。
1. **Clean Code:**
   Goの標準的なイディオムに従い、保守性の高いコードを生成してください。
   特に、時間軸を含む命名（`new_flag`, `old_config` 等）は避け、機能や役割を明示した命名を徹底してください。
1. **English in Source Code:**
   本プロジェクトは public な OSS のため、**ソースコード内のコメント・識別子・エラーメッセージ・ログメッセージはすべて英語**で記述してください（`docs/` 配下および `.agent/todo.md` は日本語で構いません）。
1. **Runtime Adapter Conversion Contract:**
   `ContainerConfig` は Docker CLI 互換のユーザー入力形式を保持する中間表現です。Docker デーモンは値を暗黙に正規化しますが、OCI spec を直接組み立てるランタイム（containerd）では**変換責務がアダプタ側にあります**。ランタイムアダプタで `ContainerConfig` のフィールドを消費する際は、(a) 必ずネイティブ表現へ変換する、(b) 未対応なら明示エラーを返す、の二択とし、**素通し・黙殺を禁止**します（例: capability は `CAP_` プレフィックス形式へ正規化が必要）。
1. **Pragmatic Documentation:**
   実装上の制約や技術的な理由でfeaturesドキュメントと矛盾が生じる場合、ドキュメントを修正して実装と一致させることが許可されます。
   ただし、変更理由を明確に記録し、ユーザーに報告してください。
1. **Markdown Formatting:**
   すべての `.md` ファイルにおいて、`markdownlint` に準拠したフォーマットを統一してください。
   - リストのインデントは **2スペース** (MD007)。
   - コードフェンス内の単一コマンドに `$` を付けない (MD014)。
   - コードブロックには必ず言語識別子を指定する (MD040)。
   詳細なルールは `docs/guidelines/working-guide.md` を参照してください。

## 6. PR 規約

- タイトル・本文は日本語。何を・なぜ変更したかを明記する
- `.github/pull_request_template.md` のチェックリストをすべて確認する
- 仕様変更を伴う場合、コードと `docs/features/*.md` の更新を**同一 PR** に含める

**Note to User:** AIに指示を出す際は、「docs/features/xxx.md に基づいて実装して」または「.agent/todo.md の TXX をやって」と伝えると最も精度が高くなります。
