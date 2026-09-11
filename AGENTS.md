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

`make test-runtime`（`-tags=runtime`）は実際のコンテナランタイムを必要とします。Docker 等が使えない環境では実行しないでください。

> **注意（2026-09-10 時点）**: `-tags=runtime` のテストは **CI でも実行されていません**（`.github/workflows/ci.yaml` のどのジョブも `-tags=runtime` を指定していないため、`internal/command` にあるタグ付きテスト6ファイルはどこでも実行されない）。**「CI が見てくれる」という前提を置かないこと。** ランタイムの挙動に依存する変更を行った場合は、Docker 等が使える環境で `make test-runtime` を実行して確認するか、未検証である旨を PR に明記してください。この状態は T20 で解消予定です。

## 3. タスクの進め方

バックログは [.agent/todo.md](./.agent/todo.md) にあります。また、タスクを安全かつ再現性高く操作するための自動化スクリプトが `.agent/manage_task.py` に用意されています。
**エージェントは、タスクの確認、完了時のステータス更新や詳細手順のクリーンアップに、必ずこのスクリプトを最優先で活用してください。**

1. 着手前に本ファイル → `.agent/todo.md` → `docs/guidelines/working-guide.md` を必ず読む
2. タスク一覧の確認には `python3 .agent/manage_task.py list` を使用する。詳細手順の抽出には `python3 .agent/manage_task.py show <ID>` を使用する
3. 原則 **1 タスク = 1 PR**。タスクの「完了条件」をすべて満たすこと
4. **1 PR に収まらないと判断した場合は、実装に着手せず中断してください**。黙って撤退するのではなく、分割案を `.agent/todo.md` に新規タスクとして起票し、ユーザーに報告すること。巨大なまま放置されたタスクは誰にも拾われず、放置されている事実自体も可視化されません
5. タスク内の file:line は記録時点のもの。ズレていたら grep で再特定する
6. **Spec-First**: 「仕様変更あり」のタスクは、対応する `docs/features/*.md` の更新が完了条件に含まれる
7. タスク完了時は `python3 .agent/manage_task.py done <ID>` を実行し、サマリテーブルの更新と詳細手順の削除を自動で行う
8. **タスク ID は `T` + 数字のみ**（例: `T93`）。`.agent/manage_task.py` が ID を `T\d+` で解析するため、英字サフィックス（`T31a` 等）を付けるとタスクが `list` に現れず、`done` がセクション境界を誤検出して隣のタスクごと削除します。既存タスクを分割する場合も、末尾に新しい番号を採番してください
9. **「完了条件」はチェックボックス（`- [ ]` / `- [x]`）ではなく素の箇条書き（`-`）で書くこと**。テンプレート由来のチェック済みマークが残ると、未着手のタスクが完了済みに見えます
10. **`DONE` にしたタスクの詳細セクションは残さず削除してください**（`python3 .agent/manage_task.py done <ID>` が自動で行います。既に `DONE` でセクションだけ残っている場合は `delete-details <ID>`）。完了済みタスクの詳細が残っていると、未達の作業指示として読まれ、出荷済み機能が再実装される事故につながります。**ステータスの正はサマリテーブルであり、詳細セクションの記述ではありません**

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
   テストファイル名は**何を検証するか**で命名します。作業の性質を表す語（`improvement` / `expansion` / `refinement` / `comprehensive` / `additional` / `extra` / `more` / `deep`）およびエージェント名（`jules` 等）をファイル名に含めないでください。命名例: `feature_shm_size_test.go`、`bugfix_issue42_test.go`、`resolver_robustness_test.go`。
1. **Level-Aware（Base Host と Nested の区別）:**
   `cderun` は同一のコードが **Level 0（Base Host: macOS / Linux / Windows）** と **Level 1 以降（コンテナ内）** の双方で動作します。環境依存の値（`TMPDIR`、`HOME`、パス、ソケット、ユーザー / グループ ID 等）を扱う変更は、両方の Level での挙動を必ず明示し、両方にテストを持たせてください。
   **片方の Level でのみ正しい振る舞いを、無条件に適用しないこと。** 実例として、コンテナ内では正しい `/tmp` へのスナップショット正規化を Level 0 にも無条件適用した結果、macOS で起動不能になった事例があります（`docs/features/nested-execution.md` の Snapshot Base Directory 節を参照）。
1. **Clean Code:**
   Goの標準的なイディオムに従い、保守性の高いコードを生成してください。
   特に、時間軸を含む命名（`new_flag`, `old_config` 等）は避け、機能や役割を明示した命名を徹底してください。
1. **English in Source Code:**
   本プロジェクトは public な OSS のため、**ソースコード内のコメント・識別子・エラーメッセージ・ログメッセージはすべて英語**で記述してください（`docs/` 配下および `.agent/todo.md` は日本語で構いません）。
1. **Runtime Adapter Conversion Contract:**
   ランタイムアダプタが `ContainerConfig` を消費する際の変換規約（素通し・黙殺の禁止など）は仕様として `docs/features/direct-container-execution.md` の Conversion Contract セクションに定義されています。**実装時はそちらを正とし、本項目はその存在を示すポインタです**。
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
