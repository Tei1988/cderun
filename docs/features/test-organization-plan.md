# Feature: Test Organization & Coverage Analysis Plan

## 1. 概要
`cderun` の品質を長期的かつ持続的に維持するために、現状のテスト網羅性を分析し、体系的なテスト構成と今後の改善計画を定義する。

## 2. 現状の分析

### 2.1. パッケージ別カバレッジ (2025-02-14時点)
| パッケージ | カバレッジ率 | 備考 |
| :--- | :--- | :--- |
| `internal/command` | 85.5% | コアロジック、フラグ解析、ドライラン等は良好。 |
| `internal/config` | 83.9% | 設定の読み込み、マージ、パス解決等は良好。 |
| `internal/logging` | 51.5% | 基本機能はテストされているが、ログローテーション等は未実装・未テスト。 |
| `internal/runtime` | 10.7% | Docker/Podmanの実装部分がほとんどテストされていない。 |
| **Total** | **74.9%** | 全体として高いが、ランタイム依存部分に大きな空白がある。 |

### 2.2. 機能別テストマッピング
`docs/features/*.md` に定義された機能と現在のテストの対応状況。

| 機能 | 対応テストコード | 状態 |
| :--- | :--- | :--- |
| 引数解析 | `root_test.go`, `flags_test.go` | 良好 |
| 引数・設定優先順位 | `resolver_test.go`, `root_test.go` | 良好 |
| ポリグロット実行 | `root_test.go` (preprocessArgs) | 良好 |
| 設定ファイルサポート | `config_test.go` | 良好 |
| マルチランタイム | `docker_test.go`, `podman_test.go` | **不足** (モック中心) |
| 直接コンテナ実行 | `root_test.go` (MockRuntime) | 良好 |
| イメージマッピング | `resolver_test.go` | 良好 |
| 環境変数パススルー | `resolver_test.go`, `integration_test.go` | 良好 |
| Mount Tools | `root_test.go`, `integration_test.go` | 普通 |
| コンテナコマンド実行 | `integration_test.go` | 良好 |
| Docker互換フラグ | `flags_test.go` | 良好 |
| バイナリマウント | `root_test.go`, `integration_test.go` | 良好 |
| ドライランモード | `root_test.go` | 良好 |
| ログ・デバッグ | `logger_test.go` | 普通 |
| インタラクティブ | `robustness_test.go` (信号処理) | **不足** (TTYリサイズ未カバー) |
| README生成 | - | 対象外 (開発フロー) |
| Nested Execution | `snapshot_test.go`, `path_test.go` | **不足** (E2E未検証) |
| 診断モード | `root_test.go` | 良好 |

## 3. 課題の特定
1. **ランタイム実装のテスト不足**: `docker.go` 内の Pull/Create/Start/Wait 等の実際のエラーハンドリングやリトライロジックが未検証。
2. **OS信号・TTY制御の未検証**: TTYリサイズ同期（`SIGWINCH`）や、複雑なシグナル伝播がテストされていない。
3. **Nested Execution の統合検証**: コンテナ内で `cderun` が再帰的に動作し、ホストパスが正しく解決されるかどうかの E2E テストが不足している。
4. **ログローテーションの未実装**: 設定項目としては存在するが、実装およびテストが存在しない。

## 4. テストの体系化案

### 4.1. テストカテゴリの再定義
| カテゴリ | 目的 | 配置 / 命名 |
| :--- | :--- | :--- |
| **Unit (ユニット)** | 外部依存なし。ロジックの正当性を検証。 | `*_test.go` (同パッケージ) |
| **Integration (統合)** | Docker/Podman、ファイルシステムとの連携を検証。 | `internal/command/integration_test.go` |
| **Robustness (堅牢性)** | 信号、レースコンディション、タイムアウトを検証。 | `internal/command/robustness_test.go` |
| **Scenario (E2E)** | 複雑なシナリオ（Nested Execution等）を検証。 | `tests/e2e/*_test.go` |

### 4.2. 命名規則
`Test[Category]_[Feature]_[Scenario]` の形式を推奨する。
- 例: `TestUnit_Config_TildeExpansion`
- 例: `TestIntegration_Docker_PortMapping`
- 例: `TestRobustness_Signal_DoubleCtrlC`

## 5. 改善計画

### 5.1. テスト容易性の向上 (リファクタリング)
1. **ファイルシステムの抽象化**: `os` パッケージを直接呼び出すのではなく、`fs.FS` やインターフェースを介した DI を導入し、複雑なディレクトリ構造をメモリ上でテスト可能にする。
2. **ランタイムモックの強化**: 現在の `MockRuntime` をより多機能にし、ネットワークエラーやプル失敗などの異常系をシミュレート可能にする。
3. **コマンド実行のラップ**: `cderun` バイナリ自体のパスを環境変数や設定で差し替え可能にし、テスト用のダミーバイナリを使用したテストを容易にする。

### 5.2. カバレッジの継続的計測と記録
1. **`COVERAGE.md` の運用開始**: 現在のカバレッジ値を定期的に記録し、推移を可視化する。
2. **CIでの自動計測**: GitHub Actions 等で、PRごとにカバレッジレポートを生成する（エラーにはしないが、サマリーとして報告する）。

## 6. テストマトリックス (将来像)
| 機能 | Unit | Integration | Robustness | Scenario |
| :--- | :---: | :---: | :---: | :---: |
| 引数解析 | ✅ | - | - | - |
| ランタイム自動検出 | - | ✅ | - | - |
| イメージプル(リトライ) | - | ✅ | ✅ | - |
| ボリュームマウント | ✅ | ✅ | - | - |
| ポート転送 | ✅ | ✅ | - | - |
| TTYリサイズ | - | - | ✅ | - |
| Nested Execution | ✅ | - | - | ✅ |
| ログローテーション | ✅ | ✅ | - | - |

---
*本計画書は、cderun のテスト戦略の Source of Truth として、実装の進展に合わせて随時更新されるべきである。*
