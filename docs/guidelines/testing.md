# テスト実装指針 (Testing Guidelines)

`cderun` プロジェクトにおけるテストコードの実装方針とベストプラクティスを定義します。
高品質でメンテナンス性の高いテストを維持するために、以下のガイドラインに従ってください。

## 1. テストの独立性とリーク防止 (Test Isolation & Leak Prevention)

テスト間で状態（グローバル変数、パッケージ変数、モック等）が漏洩しないように徹底してください。

### 1.1. `t.Cleanup` による復元の徹底

パッケージレベルの変数（`opts` や `runtimeFactory` など）を変更する場合は、必ず `t.Cleanup` を使用して元の状態に戻してください。

```go
func TestFeature(t *testing.T) {
	// 元の状態を保存
	originalFS := opts.fs
	originalLoader := opts.configLoader

	// テスト終了時に必ず復元
	t.Cleanup(func() {
		opts.fs = originalFS
		opts.configLoader = originalLoader
	})

	// テスト用のモックをセット
	opts.fs = &config.MockFileSystem{...}
	opts.configLoader = config.NewConfigLoaderWithFS(opts.fs)

	// ... テスト実行 ...
}
```

### 1.2. 補助関数（Helper Functions）の活用

共通のセットアップ処理は `setupMockRuntime` のような補助関数にまとめ、その中で `t.Cleanup` を呼び出すことで、復元の漏れを防いでください。

### 1.3. 並列実行の安全性 (Parallel Execution Safety)

`t.Parallel()` を使用してテストを並列実行する場合は、以下の点に注意してください。

- **並列実行不可なテスト**: 以下の操作を行うテストは、プロセス全体の状態に影響を与えるため、`t.Parallel()` を使用しないでください。
  - `runCderun` ヘルパーの使用（内部で `os.Stdout` や `os.Stderr` などのグローバルなストリームを置換し、グローバルな `opts` をリセットするため）。
  - `syscall.Kill(os.Getpid(), ...)` による自プロセスへのシグナル送信（例: `robustness_test.go`）。
  - `os.Stdout` や `os.Stderr` などのグローバルなストリームの直接的な置換。

## 2. モック実装の原則 (Mocking Principles)

### 2.1. 可読性の高いロジック

モック内でのパス判定や文字列比較には、標準ライブラリの便利な関数を活用し、意図が明確になるようにしてください。
特にパスのプレフィックス判定には `strings.HasPrefix` を推奨します。

### 2.2. 防衛的なモック設計 (Defensive Mocking)

インターフェースをモックする場合、埋め込みフィールドを `nil` のままにせず、パニックを避けるために最小限のメソッドを実装してください。
特に `os.FileInfo` などをモックで返す場合、予期せぬメソッド呼び出しでテストがクラッシュしないように注意してください。

## 3. コードの依存関係とドキュメント (Dependency & Documentation)

### 3.1. 初期化順序の明示

パッケージレベルの変数や、特定の初期化順序に依存する関数（例: `defaultLoader` に依存する `NewConfigLoaderWithFS`）については、将来の開発者が意図を理解できるようにコメントで明記してください。

## 4. 命名規則と構成

`docs/features/test-organization-plan.md` に定義されている命名規則とカテゴリに従ってください。

- **命名規則:** `Test[Category]_[Package]_[Feature]_[Scenario]`
- **配置:** ロジックに応じた適切なテストファイル（`root_test.go`, `integration_test.go`, `robustness_test.go` など）を選択してください。
