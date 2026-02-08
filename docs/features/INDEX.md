# cderun Features ドキュメント

## 概要

このディレクトリには`cderun`の各機能の詳細仕様が含まれています。

## 機能一覧

### コア機能

1. **[引数解析 (Completed)](./argument-parsing.md)**
  - 厳密な境界解析
  - cderunフラグとサブコマンド引数の分離

2. **[引数・設定優先順位 (Completed)](./argument-priority-logic.md)**
  - P1〜P6の優先順位階層
  - CLI、環境変数、設定ファイルの解決ロジック

3. **[ポリグロットエントリーポイント (Completed)](./polyglot-entry.md)**
  - シンボリックリンクによる自動ツール検出
  - 単一バイナリで複数ツールとして動作

4. **[設定ファイルサポート (Completed)](./configuration-file-support.md)**
  - `.cderun.yaml`: cderun自体の設定
  - `.tools.yaml`: 各ツールでの実行設定

### ランタイム機能

5. **[マルチランタイムサポート (Completed)](./multi-runtime-support.md)**
  - Docker / Podman サポート
  - ランタイム自動検出
  - 統一されたCRIインターフェース

6. **[直接コンテナ実行 (Completed)](./direct-container-execution.md)**
  - コマンド生成なしでランタイムAPIを直接使用
  - 中間表現（ContainerConfig）からAPIコールへの変換

7. **[イメージマッピング (Completed)](./image-mapping.md)**
  - サブコマンド名からイメージへの自動マッピング
  - カスタムマッピング設定

### 実行環境機能

8. **[環境変数パススルー (Completed)](./env-passthrough.md)**
  - デフォルトでは引き継がない
  - 明示的指定による選択的パススルー
  - `KEY=value`と`KEY`（ホストから取得）形式のサポート

9. **[Mount Tools (Completed)](./mount-tools.md)**
  - .tools.yamlに定義されたツールをコンテナ内で使用可能にする
  - cderunバイナリを複数のツール名でマウント

10. **[コンテナコマンド実行 (Completed)](./container-command-execution.md)**
    - エフェメラルコンテナでのコマンド実行
    - TTY/インタラクティブサポート

### 高度な機能

11. **[Docker互換フラグ (Completed)](./command-line-options.md)**
  - ポートマッピング、リソース制限、ユーザー指定など
  - Docker CLI互換のオプションサポート

12. **[cderunバイナリマウント (Completed)](./cderun-binary-mounting.md)**
    - `--mount-cderun`でコンテナ内でcderunを使用
    - `--mount-socket` (boolean) との併用必須

13. **[ドライランモード (Completed)](./dry-run-mode.md)**
  - 実行前のコマンドプレビュー
  - JSON/YAML/Simple形式での出力

14. **[ログ・デバッグ (Completed)](./logging-debugging.md)**
  - 詳細ログ出力
  - レベル別出力、ファイル出力、JSON形式対応

15. **[インタラクティブ・ターミナル (Completed)](./interactive-terminal.md)**
  - シグナル転送
  - TTYリサイズ同期

16. **[README生成戦略 (Completed)](./readme-generation.md)**
    - 実装コードからREADMEを生成
    - Source of Truthの維持

17. **[Nested Execution (Specification)](./nested-execution.md)**
    - コンテナ内からの再帰的なcderun実行
    - 設定の動的な注入とパス変換ロジック

### 開発・検証機能

18. **[インテグレーションテスト (Completed)](./integration-testing-with-docker.md)**
  - testcontainers-go を利用した実ランタイムでの検証
19. **[テストカバレッジ計測 (Completed)](./test-coverage-reporting.md)**
  - コードカバレッジの可視化と自動計測

20. **[診断モード (Completed)](./diagnosis-mode.md)**
  - システム診断情報と利用可能なツールの表示

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

## 実装優先順位

### Phase 1: コア機能 (Completed)
- 引数解析
- ポリグロットエントリーポイント
- Docker CRI実装
- 基本的なコンテナ実行

### Phase 2: 設定管理 (Completed)
- 設定ファイル読み込み
- イメージマッピング
- 優先順位解決
- ドライランモード (Phase 4から前倒しで完了)

### Phase 3: 高度な機能 (Completed)
- 環境変数パススルー
- ソケット・バイナリマウント・ツールマウント

### Phase 4: 利便性向上 (Completed)
- Podmanサポート
- ログ・デバッグ機能
- インタラクティブ・ターミナル

### Phase 5: Docker互換フラグの拡充 (Completed)
- ポートマッピング、リソース制限、ユーザー指定、ケーパビリティ等

### Phase 6: 高度な設定解決機能 (Completed)
- 設定ファイルの階層的検索とマージ
- 動的値解決 (Expressions)
- 相対パス・チルダ解決

### Phase 7: 機能改善と拡張 (Completed)
- TTYフラグの短縮形 `-t` の追加
- 環境変数の厳密モード `strictEnv` の実装

### Phase 8: インテグレーションテスト (Completed)
- testcontainers-go の導入
- 各種インテグレーションテストの実装

### Phase 9: テストカバレッジ計測 (Completed)
- Makefile の作成とカバレッジ計測の自動化

### Phase 10: 引数解釈とコマンド組み立ての改善 (Completed)
- サブコマンドのキー化とイメージ解決
- コンテナコマンドの厳密な組み立て
