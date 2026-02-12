# Feature: Test Organization & Coverage Analysis Plan

## 1. 概要

`cderun` の品質を長期的かつ持続的に維持するために、現状のテスト網羅性を分析し、体系的なテスト構成と今後の改善計画を定義する。

## 2. 現状の分析

### 2.1. パッケージ別カバレッジ (2026-02時点)

| パッケージ | カバレッジ率 | 備考 |
| :--- | :--- | :--- |
| `internal/command` | 87.9% | コアロジック、フラグ解析、ドライラン、ネスト実行等は良好。 |
| `internal/config` | 83.3% | 設定の読み込み、マージ、Expression解決、パス解決等は良好。 |
| `internal/logging` | 100.0% | ロギングシステムの全面的なテスト拡充により 100% を達成。 |
| `internal/runtime` | 92.9% | リトライロジック、TTYリサイズ、ストリーム処理のテストが充実. |
| **Total** | **86.7%** | 全体として極めて高いカバレッジを維持。 |

### 2.2. 機能別テストマッピング

`docs/features/*.md` に定義された機能と現在のテストの対応状況。

| 機能 | 対応テストコード | 状態 |
| :--- | :--- | :--- |
| 引数解析 | `root_test.go`, `flags_test.go` | 良好 |
| 引数・設定優先順位 | `resolver_test.go`, `root_test.go` | 良好 |
| ポリグロット実行 | `root_test.go` (preprocessArgs) | 良好 |
| 設定ファイルサポート | `config_test.go`, `merge_test.go` | 良好 |
| マルチランタイム | `docker_test.go`, `podman_test.go`, `mock_test.go` | 良好 (リトライ検証追加、MockRuntime検証追加) |
| 直接コンテナ実行 | `root_test.go` (MockRuntime), `integration_test.go` | 良好 |
| イメージマッピング | `resolver_test.go` | 良好 |
| 環境変数パススルー | `resolver_test.go`, `integration_test.go` | 良好 |
| Mount Tools | `root_test.go`, `integration_test.go` | 良好 |
| コンテナコマンド実行 | `integration_test.go`, `root_test.go` | 良好 |
| Docker互換フラグ | `flags_test.go`, `root_test.go` | 良好 |
| cderunバイナリマウント | `root_test.go`, `integration_test.go` | 良好 |
| ドライランモード | `root_test.go` | 良好 |
| ログ・デバッグ | `logger_test.go` | 良好 |
| インタラクティブ | `robustness_test.go` (信号、リサイズ) | 良好 |
| README生成 | - | 対象外 (開発フロー) |
| Nested Execution | `snapshot_test.go`, `path_test.go`, `scenario_nested_test.go` | 良好 |
| 診断モード | `root_test.go` | 良好 |
| 統合テスト(Docker) | `integration_test.go` | 良好 |
| テストカバレッジ | `Makefile` | 良好 |
| Expressions | `expression_test.go`, `resolver_test.go` | 良好 |
| パス解決(チルダ・相対) | `path_test.go` | 良好 |
| 厳密モード(strictEnv) | `integration_test.go`, `resolver_test.go` | 良好 |

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
| **Scenario (E2E)** | 複雑なシナリオ（Nested Execution等）を検証。 | `internal/command/scenario_*_test.go` |

### 4.2. 命名規則

`Test[Category]_[Feature]_[Scenario]` の形式を推奨する。

  - 例: `TestUnit_Config_TildeExpansion`
  - 例: `TestIntegration_Docker_PortMapping`
  - 例: `TestRobustness_Signal_DoubleCtrlC`

## 5. 改善計画

### 5.1. テスト容易性の向上 (完了および継続)

  1. **ファイルシステムの抽象化**: `runConfigDir` や `systemConfigDir` の差し替えメカニズムを導入済み。さらなる抽象化（`fs.FS`等）は将来の検討事項。
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
| ポート転送 | ✅ | ✅ | - | - |
| 信号処理(Ctrl+C) | - | - | ✅ | - |
| TTYリサイズ | - | - | ✅ | - |
| Nested Execution | ✅ | ✅ | - | ✅ |
| Expressions | ✅ | ✅ | - | - |
| 厳密モード | ✅ | ✅ | - | - |
| cderunバイナリマウント | ✅ | ✅ | - | ✅ |
| 診断モード | ✅ | - | - | - |
| Mount Tools | ✅ | ✅ | - | ✅ |
| ポリグロット実行 | ✅ | - | - | - |

---
*本計画書は、cderun のテスト戦略の Source of Truth として、実装の進展に合わせて随時更新されるべきである。*
