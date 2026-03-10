# テスト構成・網羅性分析計画

## 1. 概要

`cderun` の品質を長期的かつ持続的に維持するために、現状のテスト網羅性を分析し、体系的なテスト構成と今後の改善計画を定義する。

## 2. 現状の分析 (2026年3月10日時点)

### 2.1. パッケージ別カバレッジ

| パッケージ | カバレッジ率 | 備考 |
| :--- | :--- | :--- |
| `internal/command` | 90.5% | コアロジック、フラグ解析、ドライラン、ネスト実行等は良好。 |
| `internal/config` | 88.6% | 設定の読み込み、マージ、Expression解決、パス解決等は良好。 |
| `internal/logging` | 97.1% | 極めて高いカバレッジを維持。 |
| `internal/runtime` | 82.8% | リトライロジック、TTYリサイズ、ストリーム処理のテストが充実。 |
| `internal/container` | 0% (ステートメントなし) | 実行ステートメントを持たない構造体定義のみだが、`internal/container/config_test.go` 等で検証。 |
| **合計** | **88.8%** | 全体として 88% を超える極めて高いカバレッジを維持。 |

### 2.2. 機能別テストマッピング

| 機能 | 対応テストコード | 状態 |
| :--- | :--- | :--- |
| 引数解析 | `internal/command/root_test.go`, `internal/command/flags_test.go` | 良好 |
| 引数・設定優先順位 | `internal/config/resolver_test.go`, `internal/command/root_test.go` | 良好 |
| ポリグロット実行 | `internal/command/root_test.go`, `internal/command/polyglot_test.go` | 良好 |
| 設定ファイルサポート | `internal/config/config_test.go`, `internal/command/integration_test.go`, `internal/config/fs_test.go` | 良好 |
| マルチランタイム | `internal/runtime/docker_test.go`, `internal/runtime/podman_test.go`, `internal/runtime/mock_test.go` | 良好 |
| 直接コンテナ実行 | `internal/command/root_test.go`, `internal/command/integration_test.go` | 良好 |
| イメージマッピング | `internal/config/resolver_test.go` | 良好 |
| 環境変数パススルー | `internal/config/resolver_test.go`, `internal/command/integration_test.go` | 良好 |
| Mount Tools | `internal/command/root_test.go`, `internal/command/integration_test.go` | 良好 |
| Docker互換フラグ | `internal/command/flags_test.go`, `internal/command/root_test.go`, `internal/command/e2e_device_test.go` | 良好 |
| デバイスマウント | `internal/config/path_test.go`, `internal/runtime/docker_test.go`, `internal/command/e2e_device_test.go` | 良好 |
| cderunバイナリマウント | `internal/command/root_test.go`, `internal/command/integration_test.go` | 良好 |
| ドライランモード | `internal/command/root_test.go` | 良好 |
| ログ・デバッグ | `internal/logging/logger_test.go` | 良好 |
| インタラクティブ | `internal/command/robustness_test.go`, `internal/command/stdin_test.go` | 良好 |
| 信号処理 | `internal/command/signals_test.go`, `internal/command/robustness_test.go` | 良好 |
| ハングリカバリ | `internal/command/docker_hang_test.go` | 良好 |
| Nested Execution | `internal/command/snapshot_test.go`, `internal/command/scenario_nested_test.go`, `internal/config/path_test.go` | 良好 |
| 診断モード | `internal/command/root_test.go` | 良好 |
| Expressions | `internal/config/resolver_test.go`, `internal/command/integration_test.go` | 良好 |
| パス解決(チルダ・相対) | `internal/config/path_test.go` | 良好 |
| 厳密モード(strictEnv) | `internal/command/integration_test.go`, `internal/config/resolver_test.go` | 良好 |

## 3. テストの体系化案

### 3.1. テストカテゴリ

| カテゴリ | 目的 | 配置 / 命名 |
| :--- | :--- | :--- |
| **Unit** | 外部依存なし。ロジックの正当性を検証。 | `*_test.go` (同パッケージ) |
| **Integration** | MockRuntime、ファイルシステムとの連携を検証。 | `internal/command/integration_test.go` |
| **Robustness** | 信号、レースコンディション、タイムアウトを検証。 | `internal/command/robustness_test.go` |
| **Scenario (E2E)** | 複雑なシナリオや実環境での検証。 | `internal/command/scenario_*_test.go`, `internal/command/e2e_test.go`, `internal/command/e2e_*_test.go` (Build tag: `e2e`, 命名: `TestScenario_`) |

### 3.2. 命名規則

`Test[Category]_[Feature]_[Scenario]` の形式を推奨する。

- 例: `TestUnit_Config_TildeExpansion`
- 例: `TestIntegration_Docker_PortMapping`
- 例: `TestRobustness_Signal_DoubleCtrlC`

## 4. 改善計画

### 4.1. テスト容易性の向上 (Testability)

1. **依存性の注入 (DI) の徹底**: `FileSystem` や `ContainerRuntime` などのインターフェースを介した抽象化を維持し、グローバル状態への依存を排除する。
2. **ファイルシステムの抽象化**: `FileSystem.Abs` メソッドを活用し、テスト実行時の作業ディレクトリ（CWD）に依存しないパス解決を実現する。
3. **並列実行の安全性**: プロセス全体の状態を変更する操作（`os.Chdir` 等）を最小限にし、必要な場合は `sync.Mutex` 等で保護する。
4. **不変性の確保**: 設定情報のスナップショット（`DeepCopy`）を活用し、テスト間の干渉を防止する。

## 5. テストマトリックス (2026年3月10日時点)

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
| Expressions | ✅ | ✅ | - | - |
| 厳密モード | ✅ | ✅ | - | - |
| cderunバイナリマウント | ✅ | ✅ | - | ✅ |
| 診断モード | ✅ | - | - | ✅ |
| Mount Tools | ✅ | ✅ | - | ✅ |
| ポリグロット実行 | ✅ | ✅ | - | - |
