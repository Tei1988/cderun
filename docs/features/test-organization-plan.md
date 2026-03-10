# テスト網羅性と組織化計画 (Test Coverage & Organization Plan)

## 1. 目的

`cderun` プロジェクトにおいて、高いコード品質とメンテナンス性を維持するため、テストコードの構造を整理し、テスト駆動開発（TDD）に適したアーキテクチャを確立する。

## 2. テストしやすい構造 (Testable Architecture)

### 2.1. 依存性の注入 (DI)

- グローバル状態（`os.Chdir`, `os.Stdout`, `os.Stderr`, グローバル変数など）への依存を排除する。
- 外部依存（ファイルシステム、ランタイム、ターミナル操作など）はインターフェース（`FileSystem`, `ContainerRuntime` 等）を介して抽象化する。
- `rootOptions` を通じてこれらの依存性を注入し、テスト時にモックに差し替え可能にする。

### 2.2. ファイルシステムの抽象化

- `config.FileSystem` インターフェースに `Abs(path string) (string, error)` を追加し、相対パス解決を完全に制御可能にする。
- テストでは `MockFileSystem` を使用し、`os.Chdir` を使わずに「仮想的な作業ディレクトリ」での動作を検証する。これにより、テストの並列実行（`t.Parallel()`）を安全に行えるようにする。

### 2.3. 不変性の確保

- 設定データ（`CDERunConfig`, `ToolsConfig`）は、必要に応じて `DeepCopy` を行い、不用意なミュータブルな変更が他の処理に影響を与えないようにする。
- ネスト実行などの複雑なシナリオでは、`createSnapshot` を利用して実行時の状態を保存し、独立した環境を構築する。

## 3. テストの組織化と命名規則

### 3.1. 配置ルール

- **Unit**: 同一パッケージ内の `*_test.go`。外部依存なし。
- **Integration**: `internal/command/integration_test.go`。MockRuntime やファイルシステムとの連携を検証。
- **Robustness**: `internal/command/robustness_test.go`。信号処理、レースコンディション、ハングリカバリを検証。
- **Scenario (E2E)**: `internal/command/e2e_test.go` 等。Build tag `e2e` を付与し、実環境（Docker 等）で検証。

### 3.2. 命名規則

`Test[Category]_[Feature]_[Scenario]` 形式を厳守する。

- 例: `TestUnit_PreprocessArgs_HoistingAndPolyglot`
- 例: `TestIntegration_Execution_AlpineEcho`
- 例: `TestRobustness_SignalHandling_ContainerInteractions`
- 例: `TestScenario_Execution_NestedRecursive`

## 4. 並列実行とアイソレーション

- `t.Parallel()` を積極的に使用し、テスト実行速度を向上させる。
- グローバルな環境変数（`t.Setenv`）やプロセス共有リソースを変更するテストは、`t.Parallel()` を使用しないか、サブテスト内で慎重に扱う。
- `setupTestDir` は各テストに固有の一時ディレクトリを提供し、ファイルシステムの競合を防ぐ。

## 5. 現在の状況 (2026年3月10日時点)

- `FileSystem` インターフェースの拡張（`Abs` メソッドの追加）完了。
- `internal/command` パッケージ内の全テスト関数のリネームと整理完了。
- 主要なユニットテストへの `t.Parallel()` 導入と、データレースの修正完了。
- エッジケース（引数のホイスティング、パス解決など）のテスト拡充完了。
