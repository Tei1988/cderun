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

### 1.3. サブテストの独立性 (Subtest Independence)

`t.Run` を使用したサブテスト内では、共有のモックインスタンスを使い回さず、可能な限りサブテストごとに新鮮なインスタンス（`MockFileSystem` など）を生成してください。これにより、サブテスト間での意図しない状態の引き継ぎや、並列実行時のレースコンディションを防止できます。

### 1.4. 並列実行の安全性 (Parallel Execution Safety)

`t.Parallel()` を使用してテストを並列実行する場合は、以下の点に注意してください。

- **並列実行不可なテスト**: 以下の操作を行うテストは、プロセス全体の状態に影響を与えるため、`t.Parallel()` を使用しないでください。
  - `runCderun` ヘルパーの使用（内部で `os.Stdout` や `os.Stderr` などのグローバルなストリームを置換し、グローバルな `opts` をリセットするため）。
  - `syscall.Kill(os.Getpid(), ...)` による自プロセスへのシグナル送信（例: `robustness_test.go`）。
  - `os.Stdout` や `os.Stderr` などのグローバルなストリームの直接的な置換。
  - `t.Setenv` や `os.Chdir` を使用するテスト（これらはプロセス全体の状態を変更するため）。**可能な限り、DI（Dependency Injection）を介して環境変数や作業ディレクトリをモック化し、プロセス全体の状態変更を避けてください。**

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

## 3. モック実装の原則 (Mocking Principles)

### 3.1. 可読性の高いロジック

モック内でのパス判定や文字列比較には、標準ライブラリの便利な関数を活用し、意図が明確になるようにしてください。
特にパスのプレフィックス判定には `strings.HasPrefix` を推奨します。

### 3.2. 防衛的なモック設計 (Defensive Mocking)

インターフェースをモックする場合、埋め込みフィールドを `nil` のままにせず、パニックを避けるために最小限のメソッドを実装してください。
特に `os.FileInfo` などをモックで返す場合、予期せぬメソッド呼び出しでテストがクラッシュしないように注意してください。
**`MockFileSystem.Stat` が返す `FileInfo.Name()` は、標準の `os.Stat` と同様にベース名のみを返すように実装してください。**

## 4. アサーションのベストプラクティス (Assertion Best Practices)

### 4.1. 型アサーションの安全性

インターフェース型の戻り値を具体的な型として検証する場合は、パニックを避けるために必ず `value, ok := ...` 形式（comma-ok idiom）を使用し、`ok` が真であることを確認してから要素にアクセスしてください。

### 4.2. 浮動小数点数の比較

CPUリソースなどの浮動小数点数（`float64`）を比較する場合は、微小な精度の誤差を許容するために `assert.InDelta` または `assert.InEpsilon` を使用してください。

### 4.3. スライスの検証

スライスの内容を検証する場合、順序が重要でないなら `assert.ElementsMatch` を使用し、要素の過不足がないか厳密に確認してください。また、環境変数のリストなど、特定の組み合わせが期待される場合は、`assert.Equal` を使用して長さと内容の両方を一度に検証することを検討してください。

## 5. 命名規則と構成

`docs/testing/organization.md` に定義されている命名規則とカテゴリに従ってください。

- **命名規則:** `Test[Category]_[Feature]_[Scenario]`
- **配置:** ロジックに応じた適切なテストファイル（`root_test.go`, `integration_test.go`, `robustness_test.go` など）を選択してください。
- **カテゴリの厳守:** プロセス全体の変数を変更する操作（パッケージレベルのセッター呼び出しなど）を含むテストは `Integration` カテゴリとして分類し、純粋なロジック検証である `Unit` テストと区別してください。
