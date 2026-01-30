# cderun Features ドキュメント

## 概要

このディレクトリには`cderun`の各機能の詳細仕様が含まれています。

## 機能一覧

### コア機能

1. **[引数解析 (Completed)](./argument-parsing.md)**
   - 厳密な境界解析
   - cderunフラグとサブコマンド引数の分離

2. **[引数・設定優先順位 (Completed)](./argument-priority-logic.md)**
   - P1〜P5の優先順位階層
   - CLI、環境変数、設定ファイルの解決ロジック

3. **[ポリグロットエントリーポイント (Completed)](./polyglot-entry.md)**
   - シンボリックリンクによる自動ツール検出
   - 単一バイナリで複数ツールとして動作

4. **[設定ファイルサポート (Completed)](./configuration-file-support.md)**
   - `.cderun.yaml`: cderun自体の設定
   - `.tools.yaml`: 各ツールの実行設定

### ランタイム機能

5. **[マルチランタイムサポート (Phase 1, 4予定)](./multi-runtime-support.md)**
   - Docker (Phase 1) / Podman (Phase 4予定) サポート
   - ランタイム自動検出 (Phase 2 Completed / Phase 4予定)
   - 統一されたCRIインターフェース

6. **[直接コンテナ実行 (Completed)](./direct-container-execution.md)**
   - コマンド生成なしでランタイムAPIを直接使用
   - 中間表現（ContainerConfig）からAPIコールへの変換

7. **[イメージマッピング (Completed)](./image-mapping.md)**
   - サブコマンド名からイメージへの自動マッピング
   - カスタムマッピング設定

### 実行環境機能

8. **[環境変数パススルー (Phase 3予定)](./env-passthrough.md)**
   - デフォルトでは引き継がない
   - 明示的指定による選択的パススルー
   - `KEY=value`と`KEY`（ホストから取得）形式のサポート

9. **[Mount Tools (Phase 3予定)](./mount-tools.md)**
   - .tools.yamlに定義されたツールをコンテナ内で使用可能にする
   - cderunバイナリを複数のツール名でマウント

10. **[コンテナコマンド実行 (Completed)](./container-command-execution.md)**
    - エフェメラルコンテナでのコマンド実行
    - TTY/インタラクティブサポート

### 高度な機能

11. **[cderunバイナリマウント (Phase 3予定)](./cderun-binary-mounting.md)**
    - `--mount-cderun`でコンテナ内でcderunを使用
    - クロスプラットフォームバイナリ自動ダウンロード
    - `--mount-socket`との併用必須

12. **[ドライランモード (Completed)](./dry-run-mode.md)**
    - 実行前のコマンドプレビュー
    - JSON/YAML/シェル形式での出力

13. **[ログ・デバッグ (Phase 4予定)](./logging-debugging.md)**
    - 詳細ログ出力
    - パフォーマンス監視

### メタ機能

14. **[README生成戦略](./readme-generation.md)**
    - 実装コードからREADMEを生成
    - Source of Truthの維持

## 機能間の関係

```
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
- ドライランモード

### Phase 3: 高度な機能
- 環境変数パススルー
- 作業ディレクトリ同期
- ソケット・バイナリマウント

### Phase 4: 利便性向上
- Podmanサポート
- ログ・デバッグ機能
