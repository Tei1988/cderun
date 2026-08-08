# テスト構成・網羅性分析計画

> **Note**: テストの基本原則・レイヤー構造・作成チェックリストは [テスト戦略 (strategy.md)](./strategy.md) で定義されている。本ドキュメントの記述と矛盾する場合は `strategy.md` を優先すること。特に、カバレッジ向上を動機とするテストの新規追加は禁止されている。

## 1. 概要

`cderun` の品質を長期的かつ持続的に維持するために、現状のテスト網羅性を分析し、体系的なテスト構成と今後の改善計画を定義する。

## 2. 現状の分析 (2026年3月18日時点)

### 2.1. パッケージ別カバレッジ

| パッケージ | カバレッジ率 | 備考 |
| :--- | :--- | :--- |
| `internal/command` | 87.3% | コアロジック、フラグ解析、ドライラン、ネスト実行、BDDシナリオテスト等は良好。 |
| `internal/config` | 86.9% | 設定の読み込み、マージ、Expression解決、パス解決、BDDシナリオテスト等は良好。 |
| `internal/logging` | 97.1% | 極めて高いカバレッジを維持。 |
| `internal/runtime` | 82.8% | リトライロジック、TTYリサイズ、ストリーム処理のテストが充実。 |
| `internal/container` | 0% (ステートメントなし) | 実行ステートメントを持たない構造体定義のみだが、`internal/container/config_test.go` 等で検証。 |
| **合計** | **86.8%** | 全体として 86.5% の閾値を超える高いカバレッジを維持。 |

---

### 2.2. 機能別テストマッピング

| 機能 | 対応テストコード | 状態 |
| :--- | :--- | :--- |
| 引数解析 | `internal/command/root_test.go`, `internal/command/flags_test.go` | 良好 |
| 引数・設定優先順位 | `internal/config/resolver_test.go`, `internal/config/bdd_test.go`, `internal/command/root_test.go` | 良好 |
| ポリグロット実行 | `internal/command/root_test.go`, `internal/command/polyglot_test.go` | 良好 |
| 設定ファイルサポート | `internal/config/config_test.go`, `internal/command/integration_test.go`, `internal/config/fs_test.go` | 良好 |
| マルチランタイム | `internal/runtime/docker_test.go`, `internal/runtime/podman_test.go`, `internal/runtime/mock_test.go` | 良好 |
| 直接コンテナ実行 | `internal/command/root_test.go`, `internal/command/integration_test.go` | 良好 |
| イメージマッピング | `internal/config/resolver_test.go` | 良好 |
| 環境変数パススルー | `internal/config/resolver_test.go`, `internal/command/integration_test.go`, `internal/config/bdd_test.go` | 良好 |
| Mount Tools | `internal/command/root_test.go`, `internal/command/integration_test.go` | 良好 |
| Docker互換フラグ | `internal/command/flags_test.go`, `internal/command/root_test.go`, `internal/command/scenario_device_test.go` | 良好 |
| デバイスマウント | `internal/config/path_test.go`, `internal/runtime/docker_test.go`, `internal/command/scenario_device_test.go` | 良好 |
| cderunバイナリマウント | `internal/command/root_test.go`, `internal/command/integration_test.go` | 良好 |
| ドライランモード | `internal/command/root_test.go` | 良好 |
| ログ・デバッグ | `internal/logging/logger_test.go` | 良好 |
| インタラクティブ | `internal/command/robustness_test.go`, `internal/command/stdin_test.go` | 良好 |
| 信号処理 | `internal/command/signals_test.go`, `internal/command/robustness_test.go` | 良好 |
| ハングリカバリ | `internal/command/robustness_test.go` | 良好 |
| Nested Execution | `internal/command/snapshot_test.go`, `internal/command/scenario_nested_test.go`, `internal/config/path_test.go` | 良好 |
| 診断モード | `internal/command/root_test.go` | 良好 |
| Expressions | `internal/config/resolver_test.go`, `internal/config/bdd_test.go`, `internal/command/integration_test.go` | 良好 |
| パス解決(チルダ・相対) | `internal/config/path_test.go` | 良好 |
| 厳密モード(strictEnv) | `internal/command/root_test.go`, `internal/config/resolver_test.go` | 良好 |

## 3. テストの体系化案

### 3.1. テストカテゴリ

| カテゴリ | 目的 | 配置 / 命名 |
| :--- | :--- | :--- |
| **Unit** | 外部依存なし。ロジックの正当性を検証。 | `*_test.go` (同パッケージ) |
| **Integration** | MockRuntime、MockFileSystem との連携を検証。 | `internal/command/integration_test.go` 等 |
| **Robustness** | 信号、レースコンディション、タイムアウトを検証。 | `internal/command/robustness_test.go` |
| **Scenario** | 複雑なシナリオや実環境での検証。BDD（Given-When-Then）形式を推奨。 | `internal/command/scenario_*_test.go`, `internal/command/bdd_test.go`, `internal/config/bdd_test.go` |

### 3.2. 命名規則

`Test[Category]_[Feature]_[Scenario]` の形式を推奨する。

- 例: `TestUnit_Config_TildeExpansion`
- 例: `TestIntegration_Docker_PortMapping`
- 例: `TestScenario_ConfigResolution_ComplexOverrides`

### 3.3. スコープ別テストファイルの分割とコンフリクト回避ルール (Scope-Specific Test Files)

AIエージェントや複数の開発者が並列で開発を進める際、巨大な既存テストファイル（例: `resolver_test.go`, `root_test.go`）の末尾にテストを追記していくと、ほぼ確実に Git のマージコンフリクトが発生します。

これを防止するため、以下の**「スコープ別テストファイルルール」**を徹底します：

1. **既存の巨大ファイルへの追記禁止**

  - 既存のテストケースそのものを修正・更新する場合を除き、新規追加するテストケースを `resolver_test.go` や `root_test.go` 等の末尾に追記することを禁止します。

2. **スコープを絞った新規テストファイルの作成**

  - 新機能の追加、バグ修正、特定のテーマに対するテストの追加を行う際は、必ずスコープが明確な新しいテストファイルを作成してください。
  - 命名例:

    - 新機能: `feature_shm_size_test.go`
    - バグ修正: `bugfix_issue42_test.go`
    - テーマ別: `resolver_robustness_test.go`, `command_extra_scenarios_test.go`

3. **ファイル分離によるコンフリクト確率の低減**

  - テストファイルを機能やチケット、タスク単位で細かく分離することにより、他の開発ブランチとの衝突の確率を大幅に低減し、スムーズに並行開発を進めやすくなります。

## 4. 改善計画

### 4.1. テスト容易性の向上 (Testability)

1. **依存性の注入 (DI) の徹底**: `FileSystem` や `ContainerRuntime` などのインターフェースを介した抽象化を維持し、グローバル状態への依存を排除する。
2. **ファイルシステムの抽象化**: `FileSystem.Abs` および `MockFileSystem.WD` を活用し、`os.Chdir` を使わずに仮想的な作業ディレクトリでのパス解決を実現する。これにより 100% の `t.Parallel()` 互換性を確保する。
3. **並列実行の安全性**: プロセス全体の状態を変更する操作（`os.Chdir`, `t.Setenv` 等）を排除し、モック化された環境でテストを実行する。**ただし、Scenarioカテゴリなど、ふるまいを重視するテストにおいては、必要に応じて実環境に近い状態での検証も許容される。**
4. **不変性の確保**: 設定情報のスナップショット（`DeepCopy`）を活用し、テスト間の干渉を防止する。
5. **BDDスタイルの導入**: `bdd_test.go` において Given-When-Then 構造を採用し、仕様としての可読性を向上させる。
6. **テストの粒度と分離**: ユニットテストでは厳格な分離とモック化を行い、シナリオテストでは実際の利用シーンに近い形での統合的な動作検証を行う。

## 5. テストマトリックス (2026年3月18日時点)

| 機能 | Unit | Integration | Robustness | Scenario |
| :--- | :---: | :---: | :---: | :---: |
| 引数解析 | ✅ | - | - | - |
| ランタイム自動検出 | ✅ | ✅ | - | - |
| イメージプル(リトライ) | ✅ | ✅ | ✅ | - |
| ボリュームマウント | ✅ | ✅ | - | ✅ |
| デバイスマウント | ✅ | ✅ | - | ✅ |
| ポート転送 | ✅ | ✅ | - | - |
| 信号処理(Ctrl+C) | ✅ | - | ✅ | - |
| TTYリサイズ | - | - | ✅ | - |
| インタラクティブ(Stdin) | ✅ | ✅ | - | - |
| Nested Execution | ✅ | ✅ | - | ✅ |
| Expressions | ✅ | ✅ | - | ✅ |
| 厳密モード | ✅ | ✅ | - | - |
| cderunバイナリマウント | ✅ | ✅ | - | ✅ |
| 診断モード | ✅ | - | - | ✅ |
| Mount Tools | ✅ | ✅ | - | ✅ |
| ポリグロット実行 | ✅ | ✅ | - | - |
