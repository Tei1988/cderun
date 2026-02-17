# Feature: Test Organization & Coverage Analysis Plan

## 1. 概要

`cderun` の品質を長期的かつ持続的に維持するために、現状のテスト網羅性を分析し、体系的なテスト構成と今後の改善計画を定義する。

## 2. 現状の分析

### 2.1. パッケージ別カバレッジ (2026-02時点)

| パッケージ | カバレッジ率 | 備考 |
| :--- | :--- | :--- |
| `internal/command` | 92.5% | コアロジック、フラグ解析、ドライラン、ネスト実行等は良好。 |
| `internal/config` | 90.3% | 設定の読み込み、マージ、Expression解決、パス解決等は良好。 |
| `internal/logging` | 96.1% | 高いカバレッジを維持。 |
| `internal/runtime` | 91.9% | リトライロジック、TTYリサイズ、ストリーム処理のテストが充実。 |
| `internal/container` | [no statements] | 実行ステートメントを持たない構造体定義のみだが、`internal/container/config_test.go` で初期化を検証。 |
| **Total** | **91.5%** | 全体として 90% を超える極めて高いカバレッジを維持。 |

### 2.2. 機能別テストマッピング

`docs/features/*.md` に定義された機能と現在のテストの対応状況。

| 機能 | 対応テストコード | 状態 |
| :--- | :--- | :--- |
| 引数解析 | `root_test.go`, `flags_test.go`, `test_helpers_test.go` | 良好 |
| 引数・設定優先順位 | `resolver_test.go`, `root_test.go` | 良好 |
| ポリグロット実行 | `root_test.go` (preprocessArgs), `polyglot_test.go` | 良好 |
| 設定ファイルサポート | `config_test.go`, `integration_test.go`, `fs_test.go` | 良好 |
| マルチランタイム | `docker_test.go`, `podman_test.go`, `mock_test.go` | 良好 (リトライ検証追加、MockRuntime検証追加) |
| 直接コンテナ実行 | `root_test.go` (MockRuntime), `integration_test.go` | 良好 |
| コンテナ設定初期化 | `internal/container/config_test.go` | 良好 |
| イメージマッピング | `resolver_test.go` | 良好 |
| 環境変数パススルー | `resolver_test.go`, `integration_test.go` | 良好 |
| Mount Tools | `root_test.go`, `integration_test.go` | 良好 |
| コンテナコマンド実行 | `integration_test.go`, `root_test.go` | 良好 |
| Docker互換フラグ | `flags_test.go`, `root_test.go`, `e2e_device_test.go` | 良好 |
| デバイスマウント | `path_test.go`, `resolver_test.go`, `docker_test.go`, `e2e_device_test.go` | 良好 |
| cderunバイナリマウント | `root_test.go`, `integration_test.go` | 良好 |
| ドライランモード | `root_test.go` | 良好 |
| ログ・デバッグ | `logger_test.go` | 良好 |
| インタラクティブ | `robustness_test.go` (信号、リサイズ), `stdin_test.go`, `e2e_device_test.go` | 良好 |
| 信号処理 | `signals_test.go`, `robustness_test.go` | 良好 |
| README生成 | - | 対象外 (開発フロー) |
| Nested Execution | `snapshot_test.go`, `scenario_nested_test.go`, `path_test.go` | 良好 |
| 診断モード | `root_test.go` | 良好 |
| 統合テスト(Docker) | `integration_test.go` | 良好 |
| テストカバレッジ | `Makefile` | 良好 |
| Expressions | `resolver_test.go`, `integration_test.go` | 良好 |
| パス解決(チルダ・相対) | `path_test.go` | 良好 |
| 厳密モード(strictEnv) | `integration_test.go`, `resolver_test.go` | 良好 |

## 3. 課題の特定 (解決済み)

  1. **ランタイム実装のテスト不足**: リトライロジックの検証テストを追加済み。
  2. **OS信号・TTY制御の未検証**: TTYリサイズ同期（`SIGWINCH`）の検証テストを追加済み。
  3. **Nested Execution の統合検証**: シナリオテストによる E2E 検証を追加済み。
  4. **ログローテーション**: 安定性の観点から設計変更により削除済み。
  5. **テスト用モックの不足**: 汎用的な `MockRuntime` および `sleepFunc` の導入により解決済み。
  6. **テストにおけるデータレース**: `robustness_test.go` 等での信号ハンドラとテスト本体間の共有変数へのアクセスを `sync.Mutex` で保護し、`go test -race` での検出を回避。
  7. **グローバル状態（カレントディレクトリ等）の汚染**: `os.Chdir` を使用するテストでの `t.Cleanup` による復元を徹底し、テストの実行順序や並列実行による不安定性を排除。
  8. **MockFileSystem の防衛的設計**: `mockFileInfo` で最小限のメソッドを実装し、パニックを防止するように改善済み。
  9. **統合テスト戦略の転換**: `testcontainers-go` からプロセス内実行ヘルパー（`runCderun`, `setupTestDir`）へ移行し、効率化と安定性を向上。

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

  1. **CIでの自動計測 (完了)**: GitHub Actions (`ci.yaml`) により、PR/プッシュ時に `make coverage` が実行される。カバレッジが 86.5% 未満の場合はジョブが失敗し、`coverage.out` がアーティファクト (`coverage-report`) として保存される。
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
| インタラクティブ(Stdin) | ✅ | ✅ | - | ✅ |
| Nested Execution | ✅ | ✅ | - | ✅ |
| Expressions | ✅ | ✅ | - | - |
| 厳密モード | ✅ | ✅ | - | - |
| cderunバイナリマウント | ✅ | ✅ | - | ✅ |
| 診断モード | ✅ | - | - | - |
| Mount Tools | ✅ | ✅ | - | ✅ |
| ポリグロット実行 | ✅ | - | - | - |

---
*本計画書は、cderun のテスト戦略の Source of Truth として、実装の進展に合わせて随時更新されるべきである。*
