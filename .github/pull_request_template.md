# 概要

<!-- 何を・なぜ変更したかを記載 -->

## 関連タスク

<!-- 例: .agent/todo.md の T40 / 該当なしの場合は「なし」 -->

## チェックリスト

- [ ] `make build` / `make test` / `make lint-go` がパスしている
- [ ] Markdown を変更した場合、`make lint-md` / `make link-check` がパスしている
- [ ] 仕様変更を伴う場合、対応する `docs/features/*.md` を同一 PR で更新した（Spec-First）
- [ ] テストを追加・修正した場合、`docs/testing/strategy.md` のチェックリストを満たしている（仕様参照コメント付き・カバレッジ駆動でない）
- [ ] `.agent/todo.md` のタスクを完了した場合、サマリテーブルのステータスを `DONE` に更新した
