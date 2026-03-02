# cderun 機能 ドキュメント

## 概要

このディレクトリには`cderun`の各機能の詳細仕様が含まれています。

## 機能一覧

### コア機能

1. **[引数解析 (完了)](./argument-parsing.md)**

  - 厳密な境界解析
  - cderunフラグとサブコマンド引数の分離

2. **[引数・設定優先順位 (完了)](./argument-priority-logic.md)**

  - P1〜P6の優先順位階層
  - CLI、環境変数、設定ファイルの解決ロジック

3. **[ポリグロットエントリーポイント (完了)](./polyglot-entry.md)**

  - シンボリックリンクによる自動ツール検出
  - 単一バイナリで複数ツールとして動作

4. **[設定ファイルサポート (完了)](./configuration-file-support.md)**

  - `.cderun.yaml`: cderun自体の設定
  - `.tools.yaml`: 各ツールでの実行設定

5. **[標準入力同期 (完了)](./stdin-synchronization.md)**

  - コンテナ起動と標準入力アタッチの同期
  - パイプ入力の信頼性向上

6. **[値の解決 (完了)](./value-resolution.md)**

  - 式（Expressions）、チルダ展開、相対パス解決

### ランタイム機能

1. **[マルチランタイムサポート (完了)](./multi-runtime-support.md)**

  - Docker / Podman サポート
  - ランタイム自動検出
  - 統一されたCRIインターフェース

2. **[直接コンテナ実行 (完了)](./direct-container-execution.md)**

  - コマンド生成なしでランタイムAPIを直接使用
  - 中間表現（ContainerConfig）からAPIコールへの変換

3. **[イメージマッピング (完了)](./image-mapping.md)**

  - サブコマンド名からイメージへの自動マッピング
  - カスタムマッピング設定

### 実行環境機能

1. **[環境変数パススルー (完了)](./env-passthrough.md)**

  - デフォルトでは引き継がない
  - 明示的指定による選択的パススルー
  - `KEY=value`と`KEY`（ホストから取得）形式のサポート

2. **[Mount Tools (完了)](./mount-tools.md)**

  - .tools.yamlに定義されたツールをコンテナ内で使用可能にする
  - cderunバイナリを複数のツール名でマウント

3. **[コンテナコマンド実行 (完了)](./container-command-execution.md)**

  - エフェメラルコンテナでのコマンド実行
  - TTY/インタラクティブサポート

### 高度な機能

1. **[Docker互換フラグ (完了)](./command-line-options.md)**

  - ポートマッピング、リソース制限、ユーザー指定など
  - Docker CLI互換のオプションサポート

2. **[cderunバイナリマウント・ネスト実行 (完了)](./nested-execution.md)**

  - `--mount-cderun` でコンテナ内でcderunを使用
  - `--mount-socket` は自動的に有効化されます
  - コンテナ内からの再帰的なcderun実行
  - 設定の動的な注入とパス変換ロジック

3. **[ドライランモード (完了)](./dry-run-mode.md)**

  - 実行前のコマンドプレビュー
  - JSON/YAML/Simple形式での出力

4. **[ログ・デバッグ (完了)](./logging-debugging.md)**

  - 詳細ログ出力
  - レベル別出力、JSON形式対応

5. **[インタラクティブ・ターミナル (完了)](./interactive-terminal.md)**

  - シグナル転送
  - TTYリサイズ同期

### 開発・検証機能

1. **[診断モード (完了)](./diagnosis-mode.md)**

  - システム診断情報と利用可能なツールの表示

## テストドキュメント

テストに関するドキュメントは [`docs/testing/`](../testing/) を参照すること。

- [E2E テスト](../testing/e2e.md)
- [統合テスト](../testing/integration.md)
- [テスト構成・網羅性計画](../testing/organization.md)
- [テストカバレッジ計測](../testing/coverage.md)

## 技術リファレンス

- **[/proc/self/mountinfo 仕様](../references/proc-self-mountinfo.md)**

## 機能間の関係

```text
引数解析 → 優先順位解決 → 中間表現(ContainerConfig)
                              ↓
                         ランタイム選択
                              ↓
                    直接コンテナ実行(CRI)
                              ↓
                         コンテナ起動
```

## 重要な設計原則

1. **中間表現の使用**: すべての設定を`ContainerConfig`に集約

2. **ランタイム抽象化**: Docker/Podmanを統一インターフェースで扱う

3. **明示的な設定**: デフォルトで安全な動作、必要に応じて明示的に指定

4. **環境の分離**: デフォルトでは環境変数を引き継がない
