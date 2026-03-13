# Working Guide & Coding Standards

開発を進める上でのワークフロー、ディレクトリ構成、コーディング規約です。

## 1. Development Workflow

機能追加や修正を行う際は、以下の **"Spec-First" サイクル** を回してください。

1. **Understand Specs (要件理解)**
   - ユーザーの指示に対応する `docs/features/` 以下のMarkdownファイルを確認する。
   - ドキュメントの内容とコードの現状に乖離がないか確認する。
1. **Plan (計画)**
   - どのパッケージを変更するか、新しいファイルをどこに作成するかをユーザーに提示する。
   - ディレクトリ構成（後述）に従っているか確認する。
1. **Implement (実装)**
   - テストコード（`_test.go`）の実装を推奨する（可能な限りTDDライクに）。
   - 実装を行う。
1. **Update Docs (ドキュメント更新)**
   - 実装中に仕様変更が発生した場合、コードだけでなく `docs/features/` の該当ファイルも更新する。
   - **重要**: 実装上の制約や技術的な理由でfeaturesドキュメントと矛盾が生じる場合、ドキュメントを修正して実装と一致させることが許可される。
   - 修正する場合は、変更理由をコミットメッセージに明記する。

## 2. Project Layout

Standard Go Project Layout に準拠しつつ、小規模な構成をとります。

```text
.
├── main.go               # エントリーポイント（極力シンプルに）
├── internal/             # 外部からimportされたくないコード
│   ├── command/          # Cobraのコマンド定義 (root.go, root_test.go)
│   ├── container/        # 中間表現 (config.go)
│   └── runtime/          # コンテナランタイム抽象化・実装 (interface.go, docker.go)
├── pkg/                  # (Optional) 外部公開しても良いライブラリコード
├── docs/
│   ├── features/         # 機能要件
│   ├── architecture/     # アーキテクチャ・ライブラリ選定
│   └── guidelines/       # このファイル
└── tests/                # 統合テスト（必要な場合）
```

## 3. Adding a New Option

新しいオプションを追加する際は、以下の **全経路チェックリスト** を必ず確認してください。
`cderun` のすべてのオプションは、原則として以下の全ての経路で指定可能でなければなりません。

### チェックリスト

- [ ] **P1**: フラグ定義を `internal/command/flags.go` に追加し、`rootOptions` のフィールドを `internal/command/root.go` で定義する（`--cderun-<name>`）
- [ ] **P2**: フラグ定義を `internal/command/flags.go` に追加し、`rootOptions` のフィールドを `internal/command/root.go` で定義する（`--<name>`）
- [ ] **P3**: `CDERUN_<NAME>` 環境変数を `resolver.go` の resolve 処理に追加する
- [ ] **P4**: `ToolConfig` 構造体（`config.go`）にフィールドを追加し、`resolver.go` で参照する
- [ ] **P5**: `ConfigDefaults` 構造体（`config.go`）にフィールドを追加し、`resolver.go` で参照する
- [ ] **P6**: `resolver.go` のハードコードデフォルト値を設定する
- [ ] **CLIOptions**: `CLIOptions` 構造体（`resolver.go`）に `<Name>` / `<Name>Set` / `Cderun<Name>` / `Cderun<Name>Set` フィールドを追加する
- [ ] **ResolvedConfig**: `ResolvedConfig` 構造体（`resolver.go`）に結果フィールドを追加する
- [ ] **resolveSettings**: `root.go` の `resolveSettings` で `CLIOptions` への代入を追加する
- [ ] **DeepCopy**: `ToolConfig.DeepCopy()` / `ConfigDefaults.DeepCopy()` に新フィールドのコピー処理を追加する（ポインタ型・スライス型の場合）
- [ ] **ドキュメント**: 以下のドキュメントを更新する
  - `docs/features/argument-priority-logic.md`（P1/P2/P3 のフラグ・環境変数リスト）
  - `docs/features/command-line-options.md`（オプションの説明）
  - `docs/features/configuration-file-support.md`（YAMLスキーマ）

### 例外が許容されるケース

以下の条件を**両方**満たす場合に限り、特定の経路を省略できます。

1. 省略する技術的・設計的な理由が明確である
2. `docs/features/` の該当ドキュメントに理由を明記している

**既知の例外:**

- `config` / `toolConfig`: 設定ファイルの読み込み先パスを決める前処理で使用されるため、P4/P5 に書いても無視される。P1/P2/P3 のみ有効。

## 4. Coding Guidelines

### General

- **Effective Go:** Goの公式スタイルガイドに従う。
- **Naming Conventions:**
  - 時間軸に依存した命名（`new`, `old`, `current`, `latest` など）を避ける。
  - 理由: 時間が経つと何が「新」で何が「旧」か判別不能になるため。
  - 代替案: `processed`, `original`, `initial`, `override` など、その変数の役割や状態を具体的に示す名前を使用する。
  - 例外: `newRootCmd` のように、Goの慣習として「常に新しいインスタンスを生成する」ことを示すコンストラクタ的な関数名は許可される。
- **Error Handling:**
  - エラーを握り潰さない（`_` で捨てない）。
  - エラーを返す際は、コンテキストを付与する: `fmt.Errorf("failed to open file: %w", err)`
- **Structs:** 構造体のフィールドには適切なタグ（`json:"..."`, `yaml:"..."`）を付与する。

### CLI Best Practices

- **Stdout vs Stderr:**
  - 正常な出力結果（パイプで渡すデータなど）: `Stdout`
  - ログ、警告、エラーメッセージ、進捗バー: `Stderr`
- **Context Management:**
  - Cobraのコマンドハンドラ内では、`context.Background()` ではなく `cmd.Context()` を使用してシグナルやタイムアウトを伝播させる。
  - クリーンアップ処理（コンテナ削除など）でコンテキストが必要な場合は、親コンテキストがキャンセルされていても実行されるよう `context.WithoutCancel(ctx)` を使用する。
- **Process Lifecycle:**
  - `os.Exit` を呼び出すと `defer` が実行されないため、クリーンアップが必要なリソースがある場合は、終了前に明示的に実行するか、呼び出し元で制御する。
  - ただし、本プロジェクトでは `context.WithoutCancel` を利用した単一の `defer` ブロックによるクリーンアップを推奨する。これにより、エラー発生時・パニック時・正常終了時のすべてをカバーでき、コードの重複（二重削除など）を防げる。
- **Container I/O Handling:**
  - コンテナへのアタッチなど、非同期にI/Oを行う場合は、goroutine内でのエラーをチャネル等で回収し、メインスレッドに伝播させる。
  - 呼び出し元から `nil` の `Writer` が渡された場合は、`io.Discard` を使用してパニックを回避する。
  - インタラクティブな入力（`stdin`）の終了時には、`CloseWrite()` 等を呼び出してコンテナ側にEOFを明示的に伝える。
- **Exit Codes:**
  - 成功: `0`
  - エラー: `1` (または適切な非ゼロの値)

### Testing

詳細は [テスト実装指針](./testing.md) を参照してください。

- **Test Isolation:**
  - テスト間で状態（グローバル変数、パッケージ変数、フラグなど）が漏洩しないようにする。
  - サブテスト内で変数を変更した場合は、必ず `t.Cleanup()` を使用して元の値に復元する。
- **CLI Output Capture:**
  - `os.Stdout` の直接的なモックは避け、`rootCmd.SetOut()` や `rootCmd.SetErr()` を使用する。
  - `fmt.Print` など標準出力に直接書かれるものをキャプチャする必要がある場合は、`os.Pipe()` を使用し、適切にファイルディスクリプタを管理（`defer close`）する。
- **Dependency Injection:**
  - `os.Exit` や `runtime.NewDockerRuntime` などの外部依存は、パッケージ変数として関数ポインタ（`exitFunc`, `runtimeFactory` 等）を定義し、テスト時にモックに差し替え可能にする。
- **Table-Driven Tests:** 複数のケースを検証する場合は、テーブル駆動テストを使用する。

## 4. Documentation Standards

ドキュメントを記述する際は、`markdownlint` に準拠し、一貫したフォーマットを維持してください。

### Markdown Formatting Rules

- **Indentation (MD007):** リストのインデントは一貫して **2スペース** としてください（ネストされたリストや注記を含む）。
- **Leading Dollar Signs (MD014):** コードフェンス内で、単一のコマンドを示す際に先頭に `$` を付けないでください（シェルプロンプトを模倣する場合を除き、コピー＆ペーストの利便性を優先します）。
- **Code Block Language (MD040):** すべてのコードブロックの開始フェンス（```）には、必ず言語識別子（`bash`, `yaml`, `json`, `text` 等）を指定してください。
- **Unordered Lists:** 箇条書きにはハイフン（`-`）を使用してください。
- **Headings:** 見出しの前後には空行を1つ入れてください。
- **Consistency:** 既存のドキュメントとトーンおよびスタイルを合わせてください。
