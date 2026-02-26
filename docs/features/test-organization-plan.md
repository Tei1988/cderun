# Feature: Test Organization & Coverage Analysis Plan

## 1. 概要

`cderun` の品質を長期的かつ持続的に維持するために、現状のテスト網羅性を分析し、体系的なテスト構成と今後の改善計画を定義する。

## 2. 現状の分析

### 2.1. パッケージ別カバレッジ (2026-03時点)

| パッケージ | カバレッジ率 | 備考 |
| :--- | :--- | :--- |
| `internal/command` | 90.8% | コアロジック、フラグ解析、ドライラン、ネスト実行等は良好。 |
| `internal/config` | 88.7% | 設定の読み込み、マージ、Expression解決、パス解決等は良好。 |
| `internal/logging` | 97.1% | 極めて高いカバレッジを維持。 |
| `internal/runtime` | 85.2% | リトライロジック、TTYリサイズ、ストリーム処理のテストが充実。 |
| `internal/container` | 0% (no statements) | 実行ステートメントを持たない構造体定義のみだが、`internal/container/config_test.go` 等で検証。 |
| **Total** | **89.3%** | 全体として 89% を超える高いカバレッジを維持。 |

### 2.2. 機能別テストマッピング

`docs/features/*.md` に定義された機能と現在のテストの対応状況。

| 機能 | 対応テストコード | 状態 |
| :--- | :--- | :--- |
| 引数解析 | `internal/command/root_test.go`, `internal/command/flags_test.go`, `internal/command/test_helpers_test.go` | 良好 |
| 引数・設定優先順位 | `internal/config/resolver_test.go`, `internal/command/root_test.go` | 良好 |
| ポリグロット実行 | `internal/command/root_test.go` (preprocessArgs), `internal/command/polyglot_test.go` | 良好 |
| 設定ファイルサポート | `internal/config/config_test.go`, `internal/command/integration_test.go`, `internal/config/fs_test.go` | 良好 |
| マルチランタイム | `internal/runtime/docker_test.go`, `internal/runtime/podman_test.go`, `internal/runtime/mock_test.go` | 良好 (リトライ検証追加、MockRuntime検証追加) |
| 直接コンテナ実行 | `internal/command/root_test.go` (MockRuntime), `internal/command/integration_test.go` | 良好 |
| コンテナ設定初期化 | `internal/container/config_test.go` | 良好 |
| イメージマッピング | `internal/config/resolver_test.go` | 良好 |
| 環境変数パススルー | `internal/config/resolver_test.go`, `internal/command/integration_test.go` | 良好 |
| Mount Tools | `internal/command/root_test.go`, `internal/command/integration_test.go` | 良好 |
| コンテナコマンド実行 | `internal/command/integration_test.go`, `internal/command/root_test.go` | 良好 |
| Docker互換フラグ | `internal/command/flags_test.go`, `internal/command/root_test.go`, `internal/command/e2e_device_test.go` | 良好 |
| デバイスマウント | `internal/config/path_test.go`, `internal/config/resolver_test.go`, `internal/runtime/docker_test.go`, `internal/command/e2e_device_test.go` | 良好 |
| cderunバイナリマウント | `internal/command/root_test.go`, `internal/command/integration_test.go` | 良好 |
| ドライランモード | `internal/command/root_test.go` | 良好 |
| ログ・デバッグ | `internal/logging/logger_test.go` | 良好 |
| インタラクティブ | `internal/command/robustness_test.go` (信号、リサイズ), `internal/command/stdin_test.go` | 良好 |
| 信号処理 | `internal/command/signals_test.go`, `internal/command/robustness_test.go` | 良好 |
| README生成 | - | 対象外 (開発フロー) |
| Nested Execution | `internal/command/snapshot_test.go`, `internal/command/scenario_nested_test.go`, `internal/config/path_test.go` | 良好 |
| 診断モード | `internal/command/root_test.go` | 良好 |
| 統合テスト(Docker) | `internal/command/integration_test.go` | 良好 |
| テストカバレッジ | `Makefile` | 良好 |
| Expressions | `internal/config/resolver_test.go`, `internal/command/integration_test.go` | 良好 |
| パス解決(チルダ・相対) | `internal/config/path_test.go` | 良好 |
| 厳密モード(strictEnv) | `internal/command/integration_test.go`, `internal/config/resolver_test.go` | 良好 |

## 3. 課題の特定 (解決済み)

  1. **ランタイム実装のテスト不足**: リトライロジックの検証テストを追加済み。
  2. **OS信号・TTY制御の未検証**: TTYリサイズ同期（`SIGWINCH`）の検証テストを追加済み。
  3. **Nested Execution の統合検証**: シナリオテストによる E2E 検証を追加済み.
  4. **ログローテーション**: 安定性の観点から設計変更により削除済み。
  5. **テスト用モックの不足**: 汎用的な `MockRuntime` および `sleepFunc` の導入により解決済み。
  6. **テストにおけるデータレース**: `robustness_test.go` 等での信号ハンドラとテスト本体間の共有変数へのアクセスを `sync.Mutex` で保護し、`go test -race` での検出を回避。
  7. **グローバル状態（カレントディレクトリ等）の汚染**: `os.Chdir` を使用するテストでの `t.Cleanup` による復元を徹底し、テストの実行順序や並列実行による不安定性を排除。

## 4. テストの体系化案

### 4.1. テストカテゴリの再定義

| カテゴリ | 目的 | 配置 / 命名 |
| :--- | :--- | :--- |
| **Unit (ユニット)** | 外部依存なし。ロジックの正当性を検証。 | `*_test.go` (同パッケージ) |
| **Integration (統合)** | MockRuntime、ファイルシステムとの連携を検証。 | `internal/command/integration_test.go` |
| **Robustness (堅牢性)** | 信号、レースコンディション、タイムアウトを検証。 | `internal/command/robustness_test.go` |
| **Scenario (E2E)** | 複雑なシナリオ（Nested Execution等）や実環境での検証。 | `internal/command/scenario_*_test.go`, `internal/command/e2e_*_test.go` (Build tag: `e2e`) |

### 4.2. 命名規則

`Test[Category]_[Feature]_[Scenario]` の形式を推奨する。

  - 例: `TestUnit_Config_TildeExpansion`
  - 例: `TestIntegration_Docker_PortMapping`
  - 例: `TestRobustness_Signal_DoubleCtrlC`

## 5. 改善計画

### 5.1. テスト容易性の向上 (完了および継続)

  1. **ファイルシステムの抽象化**: `FileSystem` インターフェースおよび `ConfigLoader` 構造体を導入し、Getwd, Stat, ReadFile, UserHomeDir 等の操作を抽象化済み。これにより、グローバル状態に依存しないテストが可能となった。
  2. **ランタイムモックの強化 (完了)**: スレッドセーフかつ全操作を記録可能な `MockRuntime` を実装済み。
  3. **コマンド実行のラップ (完了)**: `cderun` バイナリ自体のパスを `MountCderunPath` 等で制御可能にし、テストの柔軟性を向上。
  4. **設定の不変性 (Immutability) の確保**: `createSnapshot` 等での設定情報のディープコピーを徹底し、副作用による不具合やテストの干渉を防止。

### 5.2. カバレッジの継続的計測と記録

  1. **CIでの自動計測 (完了)**: GitHub Actions (`ci.yaml`) により、PR/プッシュ時に `make coverage` が実行される。カバレッジが 86.5% 未満の場合はジョブが失敗し、`coverage.out` がアーティファクト (`coverage-report`) として保存される。詳細は [Test Coverage Reporting](test-coverage-reporting.md) を参照。
  2. **`COVERAGE.md` の運用検討**: カバレッジの推移を視覚化するためのドキュメント化。

## 6. テストマトリックス (2026-02時点)

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
| 診断モード | ✅ | - | - | - |
| Mount Tools | ✅ | ✅ | - | ✅ |
| ポリグロット実行 | ✅ | ✅ | - | - |

---
*本計画書は、cderun のテスト戦略の Source of Truth として、実装の進展に合わせて随時更新されるべきである。*
