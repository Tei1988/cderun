# テスト構成

## テストカテゴリ

| カテゴリ | 目的 | 配置 / 命名 |
| :--- | :--- | :--- |
| **Unit** | 外部依存なし。ロジックの正当性を検証。 | `*_test.go` (同パッケージ) |
| **Integration** | MockRuntime、ファイルシステムとの連携を検証。 | `internal/command/integration_test.go` |
| **Robustness** | 信号、レースコンディション、タイムアウトを検証。 | `internal/command/robustness_test.go` |
| **Scenario (E2E)** | 複雑なシナリオや実環境での検証。 | `internal/command/scenario_*_test.go`, `internal/command/e2e_test.go`, `internal/command/e2e_*_test.go` (Build tag: `e2e`, 命名: `TestScenario_`) |

## 命名規則

`Test[Category]_[Feature]_[Scenario]` の形式を推奨する。

- 例: `TestUnit_Config_TildeExpansion`
- 例: `TestIntegration_Docker_PortMapping`
- 例: `TestRobustness_Signal_DoubleCtrlC`

## テストマトリックス

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

## 機能別テストマッピング

| 機能 | 対応テストコード |
| :--- | :--- |
| 引数解析 | `internal/command/root_test.go`, `internal/command/flags_test.go` |
| 引数・設定優先順位 | `internal/config/resolver_test.go`, `internal/command/root_test.go` |
| ポリグロット実行 | `internal/command/root_test.go`, `internal/command/polyglot_test.go` |
| 設定ファイルサポート | `internal/config/config_test.go`, `internal/command/integration_test.go`, `internal/config/fs_test.go` |
| マルチランタイム | `internal/runtime/docker_test.go`, `internal/runtime/podman_test.go`, `internal/runtime/mock_test.go` |
| 直接コンテナ実行 | `internal/command/root_test.go`, `internal/command/integration_test.go` |
| イメージマッピング | `internal/config/resolver_test.go` |
| 環境変数パススルー | `internal/config/resolver_test.go`, `internal/command/integration_test.go` |
| Mount Tools | `internal/command/root_test.go`, `internal/command/integration_test.go` |
| Docker互換フラグ | `internal/command/flags_test.go`, `internal/command/root_test.go`, `internal/command/e2e_device_test.go` |
| デバイスマウント | `internal/config/path_test.go`, `internal/runtime/docker_test.go`, `internal/command/e2e_device_test.go` |
| cderunバイナリマウント | `internal/command/root_test.go`, `internal/command/integration_test.go` |
| ドライランモード | `internal/command/root_test.go` |
| ログ・デバッグ | `internal/logging/logger_test.go` |
| インタラクティブ | `internal/command/robustness_test.go`, `internal/command/stdin_test.go` |
| 信号処理 | `internal/command/signals_test.go`, `internal/command/robustness_test.go` |
| Nested Execution | `internal/command/snapshot_test.go`, `internal/command/scenario_nested_test.go`, `internal/config/path_test.go` |
| 診断モード | `internal/command/root_test.go` |
| Expressions | `internal/config/resolver_test.go`, `internal/command/integration_test.go` |
| パス解決(チルダ・相対) | `internal/config/path_test.go` |
| 厳密モード(strictEnv) | `internal/command/integration_test.go`, `internal/config/resolver_test.go` |
