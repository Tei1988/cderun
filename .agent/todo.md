# TODO / Backlog

AI 開発エージェント（Jules 等）が個別タスクとして着手できるよう構造化したバックログ。

## エージェント向け共通ルール

- 着手前に必ず `AGENTS.md` → `docs/guidelines/working-guide.md`を読むこと（環境構築は `bash scripts/setup-agent-env.sh`）
- **Spec-First**: 「仕様変更あり」のタスクは対応する `docs/features/*.md` の更新が完了条件に含まれる
- テスト追加・修正時は `docs/testing/` 以下のドキュメント（特に `organization.md` の命名規則）を遵守
- 各タスクは自己完結している。原則 1 タスク = 1 PR とし、「完了条件」をすべて満たすこと
- 記録のファイルパス・行番号は T01〜T39 が 2026-06-04 時点、T40〜T67 が 2026-07-03 時点（2026-07-06 の main マージ後に再検証済み）、T68 以降が 2026-07-06 時点のコードベースで検証済み（ずれていたら grep で再特定すること）

## タスク一覧（サマリ）

| ID | タイトル | 種別 | 優先度 | 規模 | 仕様変更 | ステータス |
| --- | --- | --- | --- | --- | --- | --- |
| T01 | TTY 経由実行でターミナルが強制終了する問題の調査 | 調査 | 高 | ? | - | DONE |
| T05 | `CLIOptions` の `Set` フィールドをポインタ型に統一 | リファクタ | 高 | 中 | - | DONE |
| T06 | `--cderun-*` フラグのボイラープレートをコード生成化 | リファクタ | 中 | 大 | - | DONE |
| T07 | `preprocessArgs` の引数ホイスト簡略化 | リファクタ | 中 | 中 | あり | DONE |
| T09 | `AttachContainer`（Docker）の stdin エラー握りつぶし修正 | バグ | 低 | 小 | - | DONE |
| T11 | 未知の `{{...}}` ディレクティブをエラーにする | 挙動変更 | 中 | 中 | あり | DONE |
| T12 | `IsRetryablePullError` を型付きエラー判定に移行 | 改善 | 中 | 小 | - | DONE |
| T14 | `Phase N` コメント前後の整理 | クリーンアップ | 低 | 小 | - | DONE |
| T15 | containerd `AttachContainer` のポーリング排除 | 改善 | 低 | 小 | - | DONE |
| T16 | ランタイム未対応機能の事前バリデーション | 改善 | 中 | 中 | - | DONE |
| T18 | `ci.yaml` のアクションをコミットハッシュ固定 | CI | 高 | 小 | - | DONE |
| T19 | CI の Go バージョン指定を `go.mod` に一本化 | CI | 低 | 小 | - | DONE |
| T20 | ランタイムテストを実ランタイムに接続する（既存 containerd ジョブの是正 + Docker / Podman 追加） | CI | 高 | 中 | - | - |
| T21 | イメージ事前取得フラグ（`--prefetch`） | 機能 | 中 | 中 | あり | DONE |
| T22 | orphan コンテナのクリーンアップ（`--prune`） | 機能 | 中 | 大 | あり | - |
| T23 | `--group-add` フラグの追加 | 機能 | 高 | 小 | あり | DONE |
| T24 | `--shm-size` フラグの追加 | 機能 | 高 | 小 | あり | DONE |
| T25 | `--init` フラグの追加 | 機能 | 高 | 小 | あり | DONE |
| T26 | `--pid` フラグの追加 | 機能 | 高 | 小 | あり | DONE |
| T27 | `--read-only` フラグの追加 | 機能 | 高 | 小 | あり | DONE |
| T28 | `--ulimit` フラグの追加 | 機能 | 中 | 小 | あり | DONE |
| T29 | `--security-opt` フラグの追加 | 機能 | 中 | 小 | あり | DONE |
| T30 | `--sysctl` フラグの追加 | 機能 | 中 | 小 | あり | DONE |
| T32 | `--dns-search` フラグの追加 | 機能 | 中 | 小 | あり | DONE |
| T33 | `--dns-option` フラグの追加 | 機能 | 中 | 小 | あり | DONE |
| T34 | `--ipc` フラグの追加 | 機能 | 中 | 小 | あり | DONE |
| T35 | `--gpus` フラグの追加 | 機能 | 中 | 中 | あり | DONE |
| T36 | `--cgroupns` フラグの追加 | 機能 | 中 | 小 | あり | DONE |
| T37 | `--pids-limit` フラグの追加 | 機能 | 中 | 小 | あり | DONE |
| T38 | `--cpu-shares` / `--cpuset-cpus` / `--cpuset-mems` フラグの追加 | 機能 | 中 | 小 | あり | DONE |
| T39 | `--restart` フラグの追加 | 機能 | 低 | 小 | あり | DONE |
| T40 | containerd: コマンド指定時にイメージの ENTRYPOINT が消失する | バグ | 高 | 中 | - | DONE |
| T41 | snapshot 一時ディレクトリが `os.Exit` によりリークする | バグ | 高 | 小 | - | DONE |
| T42 | 空文字サブコマンドで nil panic | バグ | 高 | 小 | - | DONE |
| T43 | attach エラー分岐で hang-timeout 0 が「即時タイムアウト」になる | バグ | 高 | 小 | - | DONE |
| T44 | `preprocessArgs` のフラグ lookup が機能せずサブコマンドを誤認する | バグ | 高 | 小 | - | DONE |
| T45 | containerd: cap-add / cap-drop / dns / add-host が黙って無視される | セキュリティ | 高 | 中 | - | DONE |
| T46 | 設定レイヤーのマージで `BaseDir` が汚染される | バグ | 中 | 小 | - | DONE |
| T47 | エラー時にコンテナの exit code が破棄される | 改善 | 中 | 中 | あり | DONE |
| T48 | Docker AutoRemove と `WaitContainer` の競合で exit code が失われる | バグ | 中 | 中 | - | DONE |
| T49 | Docker 明示 Remove で匿名ボリュームがリークする | バグ | 中 | 小 | - | DONE |
| T50 | pull ポリシーの未知値が `always` として動作する | 改善 | 中 | 小 | - | DONE |
| T51 | containerd: `volume` / `tmpfs` マウントが不正な OCI spec になる | バグ | 中 | 小 | - | DONE |
| T52 | コンテナ起動前後のシグナルハンドリングの隙間（SIGHUP 含む） | 改善 | 中 | 中 | あり | DONE |
| T53 | 引数ホストの `--` エスケープ対応 | 挙動変更 | 中 | 小 | あり | SUPERSEDED BY T81 |
| T54 | 環境変数の bool/int/float パース失敗が黙殺される | 改善 | 中 | 小 | - | DONE |
| T55 | CLI `--device` が不正な perms を黙認する | 改善 | 低 | 小 | - | DONE |
| T56 | ポート番号の範囲検証（0 / 負数 / 65535 超） | 改善 | 低 | 小 | - | DONE |
| T57 | `{{file:...}}` のサブパス許可と設定ファイルの信頼境界 | セキュリティ | 中 | 中 | あり | DONE |
| T58 | ランタイム自動検出が substring マッチで誤検出し得る | 改善 | 低 | 小 | - | DONE |
| T59 | クリーンアップ用 `RemoveContainer` にタイムアウトがない | 改善 | 低 | 小 | - | DONE |
| T60 | duration オプションが式解決エラーを握りつぶす | 改善 | 低 | 小 | - | DONE |
| T61 | Docker attach: stdin エラー時に出力を drain せず切断する | 改善 | 低 | 小 | - | DONE |
| T62 | containerd: `ioWait` 削除の競合と attach 順序契約の明文化 | 改善 | 低 | 小 | - | DONE |
| T63 | CI と `docs/testing/` のカバレッジ・パイプライン乖離の解消 | CI | 中 | 中 | - | DONE |
| T64 | CLI help / Makefile の文字列修正（containerd・mask-all 反映） | クリーンアップ | 低 | 小 | - | DONE |
| T65 | dead code 削除・小規模クリーンアップ一括 | クリーンアップ | 低 | 小 | - | DONE |
| T66 | テスト専用ヘルパーを `_test.go` に移動 | クリーンアップ | 低 | 小 | - | DONE |
| T67 | 早期ロガー初期化がフォーマット指定を無視し、不正レベルを黙殺する | 改善 | 低 | 小 | - | DONE |
| T68 | dry-run ゴールデンテスト基盤（L2） | テスト | 高 | 中 | - | DONE |
| T79 | ゴールデンテストの必須ケース追加（T44 判別およびT81ホイスト動作） | テスト | 中 | 小 | - | DONE |
| T69 | registry 駆動の優先順位マトリクステスト生成（L1） | テスト | 高 | 中 | - | DONE |
| T70 | `ContainerRuntime` コンフォーマンススイート（L3） | テスト | 高 | 大 | - | DONE |
| T71 | mutation testing の導入 | テスト/CI | 中 | 中 | - | - |
| T72 | 既存 coverage 系テストの段階的整理・吸収 | クリーンアップ | 低 | 大 | - | - |
| T73 | ソースコード内コメントの英語化 + `ContainerConfig` の変換契約コメント追加 | クリーンアップ | 中 | 小 | - | DONE |
| T74 | containerd がLinux専用であることをドキュメントに明記 | ドキュメント | 低 | 小 | - | DONE |
| T75 | Mac環境でのNested Execution セットアップガイドの作成 | ドキュメント | 中 | 小 | - | DONE |
| T76 | T42修正後のパニックテストを `assert.NotPanics` + エラー検証に更新 | バグ | 高 | 小 | - | DONE |
| T77 | OverlayFS/GroupAdd 自動付与がコンテナ内テスト実行に干渉する問題の修正 | バグ | 高 | 中 | - | DONE |
| T78 | `buildContainerConfig` をマウント処理ごとにサブ関数へ分割 | リファクタ | 中 | 小 | - | DONE |
| T80 | コンテナ作成前に ContainerConfig をデバッグログ出力 | 改善 | 低 | 小 | - | DONE |
| T81 | 引数ホイストにおけるダブルダッシュ（`--`）の制限廃止、およびイコール指定（`=`）制約の廃止（スペース区切りのサポート） | 改善 | 高 | 中 | あり | DONE |
| T82 | デフォルトログレベルのerror化 | 改善 | 高 | 小 | - | DONE |
| T83 | コンテナ起動成否によるログ重要度再分類およびアタッチ・終了関連 of ロジック・テスト修正 | 改善 | 高 | 中 | - | DONE |
| T84 | containerd アダプタのクライアント抽象化によるユニットテストのモック化とカバレッジ向上 | 改善 | 中 | 大 | - | DONE |
| T86 | ゴールデンテスト（L2: Golden Tests）の複合シナリオの追加 | テスト | 低 | 小 | - | DONE |
| T87 | Nested Execution Control Socket — Phase 1: プロトコル・ソケット配線 | 機能 | 中 | 中 | あり | DONE |
| T88 | Nested Execution Control Socket — Phase 2: Docker向け非対話実行の疎通 | 機能 | 中 | 中 | あり | DONE |
| T89 | Nested Execution Control Socket — Phase 3: 対話実行（Attach/Signal/Resize） | 機能 | 中 | 中 | あり | DONE |
| T90 | Nested Execution Control Socket — Phase 4: nerdctl（CLIベース）非対話実行対応 | 機能/セキュリティ | 中 | 中 | あり | - |
| T91 | Nested Execution Control Socket — Phase 5: 他APIベースエンジン対応・セキュリティポリシー・macOS検証 | 機能/セキュリティ | 中 | 中 | あり | - |
| T92 | `{{file:...}}` / `{{find_dir:...}}` に `:-default` フォールバック構文を追加 | 機能 | 中 | 小 | あり | DONE |
| T93 | `--engine` の導入（`--runtime` は非推奨エイリアスとして温存） | 機能 | 高 | 大 | あり | - |
| T94 | OCI ランタイム指定フラグ `--oci-runtime` の追加 | 機能 | 高 | 小 | あり | - |
| T95 | `--runtime` の意味を OCI ランタイム側へ切り替え | 破壊 | 中 | 小 | あり | - |
| T96 | Control Socket サーバのリクエスト context を接続の生存に紐づける | バグ | 高 | 小 | - | - |
| T97 | Control Socket サーバの accept ループ堅牢化とアイドルタイムアウト | バグ | 中 | 小 | - | - |
| T98 | Control Socket ハンドラの共通化とエラー処理方針の統一 | リファクタ | 中 | 小 | - | - |
| T99 | テストファイル命名規約の強制（親タスク） | クリーンアップ | 高 | 大 | - | - |
| T100 | テストファイル命名規約の明文化と lint / CI ゲートの追加 | クリーンアップ | 高 | 小 | - | - |
| T101 | `internal/config` のテストファイルリネームと重複削除 | クリーンアップ | 高 | 中 | - | - |
| T102 | `internal/command` のテストファイルリネームと重複削除 | クリーンアップ | 高 | 中 | - | - |
| T103 | `internal/runtime` のテストファイルリネームと重複削除 | クリーンアップ | 高 | 中 | - | - |

依存関係・統合の注意:

- **T77 と T78 は同時着手を推奨**（T78 で `applySocketMount` / `applyToolMounts` を切り出してから T77 の DI を実装するとスムーズ。逆順だとコンフリクトしやすい）
- **T05 と T06 は統合可能**（`registry.go` のメタデータを single source of truth にすれば両方解決する）。別々に着手する場合は T05 → T06 の順
- **T22 は「ラベル付与」を先行サブタスクとして切り出し可能**（移行問題の縮小）
- **T69 は T05/T06 と統合実装を強く推奨**（registry メタデータからテストを生成するため、single source of truth 化と同じ作業）
- **T70 は T20 と統合実装を推奨**（CI の実ランタイムジョブをコンフォーマンススイートの器として作る）。T40/T45/T51 の再現ケースを必ず含めること
- **テスト系タスク（T69〜T72）に着手する前に `docs/testing/strategy.md` を必ず読むこと**。推奨着手順は T79 → T69 → T70 → T71 → T72（T68 は実装済み）
- **T79 は T81 においてダブルダッシュ（`--`）のホイスト制限を廃止した仕様に基づいてゴールデンテストを更新すること** (更新済み)
- **T87 → T88 → T89 は厳密な直列依存**（各フェーズは前フェーズの完了を前提とする）。規模が大きいため1タスクにまとめず、フェーズごとに1PRとすること
- **T90 は T87 完了後、T88 の直後（T89 と並行可）に着手を推奨**。CLIベースエンジン（nerdctl）を選んだユーザーが他フェーズの完了まで待たされてNested Executionを使えない、という事態を避けるための前倒し。T88 が確立するControl Socketディスパッチの配線パターンを流用したほうが手戻りが少ないため、T88 より前に着手するのは非推奨（逆順・並行実装はコンフリクトしやすい）
- **T91 は T89 完了が前提**。T90 の完了は必須ではないが、テスト基盤（敵対的テストのハーネス等）を流用できるため T90 の後が望ましい
- **Apple `container` / WSL Containers 等、ワイヤプロトコルを公開しないランタイムへの nested execution 対応は T88（できれば T89）の完了が前提条件**。T91（全APIベースエンジンパリティ）の完了を待つ必要はない。**採用方針はCLIラッパー方式（`container`/`wslc.exe`をシェルアウト）で、T90で定義する構築技法（`--`挿入・`--flag=value`結合形）・argv構造自己チェック・**そのCLI実バイナリに対する敵対的テスト**（nerdctlで通ったことは他CLIの安全性の根拠にならない。`--`の意味はcobra/pflag等、各CLI内部の引数解析ライブラリ依存のため、アダプタごとに独立して検証が必要）を実装すればControl Socket対応できる。サイドカー方式（`apple/containerization` Swift package や `Microsoft.WSL.Containers` NuGet を直接リンク）は検討したが、プラットフォームごとの別ツールチェーン・署名等の運用コストが見合わないため不採用（`docs/features/nested-execution-control-socket.md` の Relationship to Multi-Runtime Support 節に理由を記録済み。CLIすら提供しない将来のランタイムが出た場合のみ再検討）**
- **T93（`--runtime`→`--engine`リネーム、engine系統の`docker-compat-api`/`containerd-api`/`nerdctl`分類）はT90より前に完了させることを推奨**。T90はBase Hostの設定エンジンが`nerdctl`かどうかを判定する必要があり、T93が導入するAPIベース/CLIベースの区別に依存する。`internal/config/resolver.go`を両タスクが触るため、並行実装はコンフリクトしやすい

---

## T05: `CLIOptions` の `Set` フィールドをポインタ型に統一

- 種別: リファクタリング
- 優先度: 高
- 対象:
  - `internal/config/resolver.go:67-218`（`CLIOptions`。`root.go` の `rootOptions`とは別物）
  - `internal/command/root.go:258` 以降（`resolveSettings()` の `CLIOptions` 組み立て）

### 問題

`CLIOptions` では全フラグに `FooSet bool` フィールドが存在し、フィールド数が実質 2 倍になっている。`ConfigDefaults`（`internal/config/config.go:53-90`）が既に `*bool` / `*int` 等を使っているのと同様に、`CLIOptions` もポインタ型（`*bool`, `*string` 等）に統一することで `Set` フィールドを廃止できる。

- `nil` = 未指定、値あり = 明示的に指定、と意味が明確になる
- `resolveSettings()` の組み立て部分のコード量が約半分になる
- フラグ追加時の修正箇所が減る

### 方針

pflag はポインタ束縛を直接サポートしないため、cobra への束縛は `rootOptions` の値フィールドのまま維持し、`resolveSettings` での詰め替え時にジェネリックヘルパーで変換する。

```go
// T05 example
func opt[T any] (changed bool, v T) *T {
    if !changed { return nil }
    return &v
}
// 使用例: Image: opt(cmd.Flags().Changed("image"), o.image)
```

これなら cobra 側は無変更で `CLIOptions` のフィールド数だけ半減できる。さらに `registry.go` のメタデータ＋リフレクションを使えば詰め替え自体も自動化でき、T06 と統合できる。

### 完了条件

- `CLIOptions` から `FooSet` フィールドが全廃されている
- 「未指定」「明示的にゼロ値を指定」（例: `--tty=false`）の区別が全オプションで維持されている（回帰テスト）

---

## T07: `preprocessArgs` の引数ホイスト簡略化

- 種別: リファクタリング
- 対象: `internal/command/root.go:1220`（`preprocessArgs`）
- 仕様変更: あり → `docs/features/argument-parsing.md` の更新必須

### 問題

`preprocessArgs` はフラグパーサーを手書きで再実装しており、フラグが値を取るかの判定が壊れやすい。`--cderun-*` フラグだけを先に抽出する薄いフィルターに限定し、残りは cobra に委ねる構造にすることでロバスト性が上がる。

```go
func splitCderunArgs(args []string) (cderunFlags []string, rest []string) {
    for i := 0; i < len(args); i++ {
        if strings.HasPrefix(args[i], "--cderun-") {
            cderunFlags = append(cderunFlags, args[i])
            if !strings.Contains(args[i], "=") && i+1 < len(args) {
                cderunFlags = append(cderunFlags, args[i+1])
                i++
            }
        } else {
            rest = append(rest, args[i])
        }
    }
    return
}
```

### 実装上の注意（仕様差分が 2 つある）

上記スケッチは現行仕様と差分があるため、そのまま採用しないこと:

1. 現状はサブコマンド「前」の `--cderun-*` をエラーにしている（`root.go`、エラーメッセージ `"cderun internal override flag %q must be placed after the subcommand"`）。スケッチは位置を区別しない
2. `--cderun-foo value` 形式でのスペース区切りホイスト値の消費について：
   **（注意：本制限は T81 にて、Cobraの登録フラグ定義に基づいて次引数を動的に取得・判定するセマンティクスへと完全に上書き解決されました。現在は `=` 必須制約は廃止され、スペース区切りによる引数の安全なホイストと、他の `--cderun-` フラグの誤消費防止が同時にサポートされています）**

### 完了条件

- 採用した仕様が `docs/features/argument-parsing.md` に反映されている
- サブコマンド前 `--cderun-*` の扱い、およびスペース区切り/イコール付きP1オーバーライドフラグのホイスト動作のテストがある

---

## T20: ランタイムテストを実ランタイムに接続する（既存 containerd ジョブの是正 + Docker / Podman 追加）

- 種別: CI / テストカバレッジ
- 優先度: 高
- 規模: 中
- 仕様変更: なし
- 対象: `.github/workflows/ci.yaml`, `internal/runtime/conformance_suite_*_test.go`, `docs/testing/conformance.md`, `docs/testing/runtime-tests.md`

### 問題

**既存の `runtime-test-containerd` ジョブは実 containerd を一切検証していない**（2026-09-10 調査）。

- ジョブは containerd 2.2.3 と CNI プラグインを実際にインストールし、ソケットに ACL まで設定している（`ci.yaml:29-78`）
- しかし最後に走るのは `go test -v ./internal/runtime/...` で、`-tags=runtime` が付いていない（`ci.yaml:83`）
- エクスポートしている `CDERUN_RUNTIME` / `CDERUN_SOCKET_PATH` を読むのは `internal/config` のフラグ解決と `internal/command/scenario_test.go:78` だけ。後者は `//go:build runtime` なので除外される。**`internal/runtime` 配下にこの2変数を読むテストは存在しない**
- コンフォーマンススイートの入口4箇所はすべてモッククライアント。`conformance_suite_containerd_test.go` も `newMockFullClient()` と `/dummy/socket.sock` を使っている
- `//go:build runtime` のテストファイルは6つあるが全て `internal/command` にあり、`-tags=runtime` がどのジョブでも指定されていないため、**CI でもローカルの `make test` でも一度も実行されていない**

結果として、実ランタイムを起動するコストを払ったうえでモックテストを2回走らせている状態になっている。カバレッジがあるように見えるぶん検知が遅れた。

なお `docs/testing/conformance.md:128` は live conformance test を「`CDERUN_RUNTIME` プローブや `-tags=runtime` で登録できる」と将来の選択肢として記述しており、未実装であること自体はドキュメントと整合している。

### 方針

旧 T20 は「ランタイムテストは `CDERUN_RUNTIME` / `CDERUN_SOCKET_PATH` で対象を切り替える構造」を前提に Docker / Podman ジョブの追加のみを求めていたが、その前提が成立していない。**まず既存ジョブを実接続にすること**を本タスクの先頭に置く。

1. **live コンフォーマンス入口の追加**: `RunConformanceTests` に実クライアント版の factory を渡す入口を `//go:build runtime` 付きで追加する。接続先は `CDERUN_SOCKET_PATH`（未設定ならエンジンごとの既定パス）から解決する
2. **既存 containerd ジョブの是正**: `go test -tags=runtime -v ./...` に変更する。対象を `./internal/runtime/...` に限定しない（`//go:build runtime` のファイルは `internal/command` にあるため、現在の限定はタグ付きテストと噛み合っていない）
3. **Docker ジョブの追加**: runner の `/var/run/docker.sock` を使う。runner ユーザーは docker グループ所属済みのため ACL 設定は不要のはず。ただし下記「セキュリティ上の制約」を満たすこと
4. **Podman ジョブの追加**: `sudo apt-get install -y podman` 後、`systemctl --user enable --now podman.socket`。ソケットは `/run/user/$(id -u)/podman/podman.sock`
5. **サイレント劣化の防止**: ランタイムジョブ内で live テストが1件も実行されなかった場合にジョブを失敗させる。ソケットに繋がらないときは `t.Skip` で黙って緑にせず、明示的に失敗させること（今回の問題が3ヶ月気づかれなかった直接の原因がこれにあたる）

### セキュリティ上の制約（3ジョブ共通）

CI は self-hosted runner（`ouchi-kubernetes`）で動くため、fork からの `pull_request` 実行にコンテナランタイムのソケットを渡すと、リポジトリ外の第三者が runner 上で任意のコンテナを起動できる。これは追加する Docker / Podman ジョブだけでなく、**既存の containerd ジョブにも現時点で当てはまる**。

- 信頼されていない `pull_request` 実行（fork 由来）では、ホストのランタイムソケットにアクセスさせないこと。使い捨てまたは rootless のデーモンをジョブ内に立てる、あるいは当該イベントではランタイムジョブを実行しない、のいずれかで対処する
- どの方式を採るかを決め、理由を `docs/testing/runtime-tests.md` に記録すること
- fork 由来の `pull_request` がホストソケットに到達できないことを確認するテストまたは検証手順を残すこと

### 完了条件

- containerd / Docker / Podman の各ジョブが、モックではなく実デーモンに接続してコンテナを起動・待機・削除するテストを実行している
- `-tags=runtime` が CI で指定され、`internal/command` の既存タグ付きテスト6ファイルが実際に実行されている
- live テストが0件だった場合にジョブが失敗することを確認している（意図的にソケットパスを壊して赤くなることを手元で確認する）
- `make test-runtime` がローカルでも同じ経路を通る
- `docs/testing/conformance.md` の「Live Socket Testing」節が、実装済みの手順として更新されている
- `docs/testing/runtime-tests.md` に3エンジンの実行方法が記載されている

### 備考

Control Socket（T87〜T91）は「実際にネストしてコンテナが起動するか」が本質であるにもかかわらず、Phase 2/3 の検証はモック dispatcher 相手のユニットテストに留まっている。本タスクで live 経路が通ったら、T90 / T91 の完了条件に Control Socket 経由の live テストを追加すること。

---

## T22: orphan コンテナのクリーンアップ（`--prune`）

- 種別: 機能追加
- 対象: `internal/command/`、`internal/runtime/`（全ランタイム）
- 仕様変更: あり → `docs/features/` に新規仕様ドキュメントを作成（Document-First）
- 分割推奨: 「ラベル付与」を先行サブタスク（別 PR）として切り出す

### 目的

`cderun` 異常終了時に `--remove`（`registry.go:493`、デフォルト true）が効かず残ったコンテナを一括削除する。

```bash
cderun --prune
```

### 設計上の決定事項

- コンテナのラベルに `cderun=true` 等を付与して識別する（名前プレフィックスより確実）
- 誤爆リスクあり. `--dry-run` 相当の確認表示を先に出してから削除する、または `--prune --force` で強制削除、など安全策を必須とする

### ランタイム別の実装メモ（検証済み）

1. **containerd**: 既に専用 namespace `cderun`（`containerd.go:26` の `defaultNamespace`）で動いているため、namespace 内の列挙だけで cderun 製コンテナを識別でき、ラベル不要
2. **Docker / Podman**: ラベル付与＋`ContainerList` の label フィルタ（`filters.Arg("label", "cderun=true")`）が必要。現状 `CreateContainer`（`docker_adapter.go`）はラベルを一切付けていないことを確認済み
3. **ラベル付与だけ先行リリース** しておくと、prune 実装時に「ラベルなしの古いコンテナは対象外」という移行問題が小さくなる → 先行サブタスク化を推奨
4. 実行中コンテナの扱い（並行する別の cderun が使用中）は **デフォルト除外** とし、停止済みのみ削除が安全

### 完了条件

- 仕様ドキュメントが先に作成され、実装が一致している
- 全ランタイムで cderun 製コンテナのみが対象になることのテストがある
- 実行中コンテナがデフォルトで除外される

## 発見された不整合・課題

### `registry.go` の `sensitive-env` 説明文の不整合

- **内容**: `internal/config/registry.go` の `sensitive-env` オプションの `Usage` フィールドが `"default uses automatic keywords"` となっているが、現在の実際の実装（および他のドキュメント）では「未指定時はすべての環境変数をマスクする (Mask-all)」挙動となっている。
- **対応**: `--help` 等で表示されるメッセージの正確性を期すため、`registry.go` の説明文を `"default masks all variables"` 等に更新することを推奨。 (Recorded by Jules)

### `registry.go` の `runtime` 説明文の不整合

- **内容**: `internal/config/registry.go` の `runtime` オプションの `Usage` フィールドが `"Container runtime to use (docker/podman)"` となっているが、現在は `containerd` もサポートされている。
- **対応**: `Usage` 文字列を `"Container runtime to use (docker/podman/containerd)"` に更新することを推奨。 (Recorded by Jules)

### 記憶 (Memory) と実装の乖離：環境変数マスキング

- **内容**: プロジェクトの記憶（Memory）では、`internal/config/masking.go` において `sensitiveKeywords` や `maxKeywordLen` を使用したキーワードベースの高度なマスキングが実装・最適化されているとあるが、実際のコード（およびベンチマーク）では `sensitive-env` が未指定（nil）の場合に一律で `[REDACTED]` を返す「Secure by Default (Mask-all)」が実装されている。
- **対応**: 今回のドキュメント更新では「実際の実装（Mask-all）」に合わせてドキュメントを修正した。キーワードベースのマスキングを復活・導入する場合は、別途実装タスクが必要。

### Documentation Update Task: T94 (`--oci-runtime` / `--cderun-oci-runtime`)

- **内容**: T94の実装（`--oci-runtime`, `--cderun-oci-runtime`, `CDERUN_OCI_RUNTIME`, `.cderun.yaml` の `defaults.ociRuntime`）の仕様説明をドキュメントに追加する。
- **対応ドキュメント**: `docs/features/command-line-options.md`, `docs/features/multi-runtime-support.md`, `README.md`, `USAGE.md`

---

## T50: pull ポリシーの未知値が `always` として動作する

- 種別: 改善（堅牢性）
- 優先度: 中
- 対象: `internal/runtime/docker.go:125-157`、`internal/runtime/containerd.go:106-137`、`internal/config/`（検証の追加先）

### 問題

両ランタイムの `PullImage` は `== "never"` / `== "missing"` のみチェックし、それ以外の値（タイポ `nevr`、大文字 `Never`、k8s 流儀の `IfNotPresent` 等）はすべて無条件 pull にフォールスルーする。どこにもポリシー値のバリデーションがない。

### 方針

設定解決段階（single choke point）で `always` / `missing` / `never` 以外を `InvalidConfigError` にする。ランタイム側にも防御的な `fmt.Errorf("unknown pull policy %q", ...)` を置いてよい。

### 完了条件

- 不正なポリシー値が起動前にエラーになるテスト（CLI / env / YAML 各経路）

### 完了確認（2026-07 マージ後）

main 側で choke point のバリデーションが実装済みを確認（`internal/command/root.go:1100-1106` で `always` / `missing` / `never` 以外を起動前にエラー化）。ランタイム側の防御的チェック（`docker.go` / `containerd.go` の `PullImage` は依然フォールスルー）は任意項目のため未実施だが、通常の CLI 経路では未知値がランタイムに到達しなくなったため DONE とする。

---

## T51: containerd: `volume` / `tmpfs` マウントが不正な OCI spec になる

- 種別: バグ修正
- 優先度: 中
- 対象: `internal/runtime/containerd.go:213-235`、対比: `internal/runtime/docker_adapter.go:88-107`

### 問題

マウントループが `m.Type` をそのまま OCI spec に渡している。`volume` は OCI のマウントタイプではなく runc がタスク起動時に不明瞭なエラーで失敗する。`tmpfs` は `Source` が空のまま `rw`/`ro` オプションのみで出力され、runc に拒否される（source は `"tmpfs"` であるべき）。Docker 経路は 3 タイプすべて正しく処理している。

### 方針

- `volume` は network/ports と同じ「not supported by containerd runtime」の明示エラーにする
- `tmpfs` は `Type: "tmpfs", Source: "tmpfs"` + 適切なオプションで正しく構築する

### 完了条件

- containerd + `type=volume` が明示エラーになるテスト
- containerd + `type=tmpfs` が有効な OCI マウントになるテスト

---

## T53: 引数ホストの `--` エスケープ対応 (SUPERSEDED BY T81)

- 種別: 挙動変更
- 優先度: 中
- 対象: `internal/command/root.go`
- 仕様変更: あり → `docs/features/argument-parsing.md` の更新必須

### 状況

本タスク（ダブルダッシュでのホイスト無効化）は、シェル固有の解釈やコンテナ内ツールの解釈と混ざって複雑化する問題および利用ユースケースが存在しないため、**T81にて廃止・削除されました**。ダブルダッシュ `--` はホイスト無効化の役割を持たず、任意の引数位置における `--cderun-` フラグは常にホイストされます。

### 完了条件

- T81にて完全に置換・解決済み。

---

## T69: registry 駆動の優先順位マトリクステスト生成（L1）

- 種別: テスト基盤
- 優先度: 高
- 対象: `internal/config/registry.go`、`internal/config/resolver_test.go`（または生成テスト専用ファイル）
- 依存: **T05/T06（registry を single source of truth にする codegen）と統合実装を強く推奨**
- 前提: `docs/testing/strategy.md` を必ず読むこと

### 目的

P1〜P6 優先順位解決を「全オプション × 全ソース組み合わせ」で機械的に検証する。オプションごとの手書きテストでは網羅も保守も破綻するため、registry のメタデータ（`StringOptions` / `BoolOptions` / `IntOptions` / `Float64Options` / `StringSliceOptions`）からテーブルを生成する。

### 方針

- 各オプションについて「P1 のみ」「P2 のみ」…「P6 のみ」「P1+P3」「P2+P4」等の組み合わせでソースに sentinel 値を注入し、期待される勝者を assert する
- bool の「明示的 false ≠ 未指定」、リストの「明示的空リストによる上書き」もマトリクスに含める
- fast-path switch の詰め替え漏れ（値が黙って落ちるトラップ）はこのマトリクスで自動検出されるはず
- `SkipResolution: true` のオプションは専用の期待値定義を用意する

### 完了条件

- registry にオプションを 1 つ追加すると、優先順位マトリクステストが自動で拡張される
- 既知のトラップ（fast-path 詰め替え漏れ）を意図的に仕込むとテストが落ちることを確認済み

---

## T71: mutation testing の導入

- 種別: テスト / CI
- 優先度: 中
- 対象: `.github/workflows/`（夜間ジョブ）、`docs/testing/strategy.md` 第6節
- 前提: `docs/testing/strategy.md` を必ず読むこと

### 目的

「実行はされるが assert されていない」テストを定量化する。行カバレッジに代わる第一指標として mutation score を導入し、生き残ミュータントを改善対象の具体的なリストとして使う。

### 方針

- ツールは gremlins 等の Go 用 mutation testing ツールから選定（メンテナンス状況を確認して決定）
- 対象はまず `internal/config`（解決ロジック）と `internal/command`（引数解析）に絞る
- 実行は夜間 / 週次の CI ジョブ（PR ごとには回さない。遅いため）
- 初回実行の結果（ベースライン score と生き残ミュータント一覧）を記録し、上位の生き残りを T68/T69 のケース追加にフィードバックする

### 完了条件

- 夜間 CI ジョブとして mutation testing が動き、score がログ等で確認できる
- ベースラインが記録され、`docs/testing/strategy.md` の指標運用と整合している

---

## T72: 既存 coverage 系テストの段階的整理・吸収

- 種別: クリーンアップ（継続タスク）
- 優先度: 低
- 対象: `internal/config/*coverage*_test.go`、`internal/command/*coverage*_test.go` ほか（`ls internal/**/*coverage*` で列挙）
- 依存: T68 / T69 の基盤が先。前提: `docs/testing/strategy.md` 第7節

### 目的

カバレッジ駆動で追加された実装詳細依存のテスト群を、振る舞いテスト（L1/L2）に段階的に吸収し、保守コストと誤った安心感を減らす。

### 方針

- **即削除しない**（回帰価値があるため）。領域単位で進める:
  1. 対象領域の仕様が `docs/features/*.md` に明文化されていることを確認（なければ先に仕様化）
  2. その領域の coverage 系テストが検証している挙動を L1/L2 テストとして再表現
  3. mutation testing（T71）で置き換え後の検出力が落ちていないことを確認してから旧テストを削除
- 1 領域 = 1 PR。全域を一度にやらない

### 完了条件（領域ごと）

- 対象領域の `*coverage*` ファイルが消え、対応する振る舞いテストが仕様参照コメント付きで存在する
- mutation score が置き換え前より悪化していない

---

## T81: 引数ホイストにおけるダブルダッシュ（`--`）の制限廃止、およびイコール指定（`=`）制約の廃止（スペース区切りのサポート）

- 種別: 改善 / 仕様変更
- 優先度: 高
- 仕様変更: あり → `docs/features/argument-parsing.md` の更新必須

### 目的

1. ダブルダッシュ (`--`) が引数にある場合でも `--cderun-` プレフィックスのフラグを常にホイスト（回収）するようにシンプル化する。今後の複雑化を防ぐため「ダブルダッシュはホイスト無効化の役割を持たない（実装しない）」ことを仕様に明文化する。
2. 値をとる `--cderun-` 形式のオーバーライドフラグ（例：`--cderun-image`）について、`=`（イコール）の入力を必須とする制限を廃止し、通常のスペース区切りの値指定（例：`--cderun-image alpine`）を可能とする。値を取るフラグの後ろの引数は自動的にそのフラグの値としてホイスト処理される。

### 完了条件

- `preprocessArgs` から `doubleDashFound` によるホイスト無効化のロジックが完全に除去されている。
- `preprocessArgs` において、`=` が含まれない値指定型の `--cderun-` フラグについて、続く隣接引数を値として安全に抽出し、同時にホイストされるようロジックが追加されている。
- 対応するテストや仕様ドキュメントが更新されている。

---

## T82: デフォルトログレベルのerror化

- 種別: 改善
- 優先度: 高
- 仕様変更: なし

### 目的

デフォルトログレベルを `"warn"` から `"error"` に引き上げ、標準実行時のセキュリティ警告（Warnレベル）や実行開始ログ（Infoレベル）などの無用な出力を抑制する。

### 完了条件

- デフォルトログレベルが `"error"` に更新されている。
- 通常ステータスログ（`Running: ...` や `Pulling image ...`）が `info` から `debug` レベルに変更されている。

---

## T83: コンテナ起動成否によるログ重要度再分類およびアタッチ・終了関連 of ロジック・テスト修正

- 種別: 改善
- 優先度: 高
- 仕様変更: なし

### 目的

コンテナが正常起動した後のエラー（非タイムアウト接続エラー等）を致命的な runner エラーではなく警告（Warnレベル）としてハンドリングし、コンテナ側のステータスで正常終了できるようにする。

### 完了条件

- 正常なコンテナ起動後のシグナル・接続エラー等の重要度が適切に Warn レベルに下げられていること。
- 対応するテストケースが実装・確認されていること。

---

## T90: Nested Execution Control Socket — Phase 4: nerdctl（CLIベース）非対話実行対応

- 種別: 機能 / セキュリティ
- 優先度: 中
- 規模: 中
- 前提: T87 完了（T88 の直後に着手することを推奨。ディスパッチ配線パターンの流用のため。逆順・並行での実装はコンフリクトしやすい）
- 仕様変更: あり → `docs/features/nested-execution-control-socket.md`

### 背景

CLIベースアダプタ（`nerdctl`等）を一律禁止にすると、Apple containerやWSL Containersのような将来のCLIラッパー実装も含めて「その方式を選んだ時点でNested Executionを諦める」ことになってしまう。当初の方針検討でCLIベースの統合を志向していた経緯があるため、一律禁止ではなく、安全性を検証可能な形で条件付き許可する。また、これをT90（最終フェーズ）まで先送りすると、nerdctlエンジンを選んだユーザーは他フェーズが終わるまでNested Executionが一切使えず実質的な禁止になってしまうため、T88の直後（Docherと同時期）に前倒しする。

### 方針

`nerdctl`は`ContainerConfig`を最終的にargvへ変換する過程で**引数インジェクション**（CWE-88）のリスクを持つ。例えば`Image`フィールドの値を`alpine:latest`ではなく`--privileged`にされると、nerdctlのcobra/pflagパーサーがこれをフラグとして解釈し、`ContainerConfig`のどこにも`Privileged: true`と書かれていないのに特権コンテナが起動してしまう。これはシェルインジェクションとは別問題で、`exec.Command`でシェルを介さず実行しても防げない（argvがどう構築されたかではなく、受け取り側nerdctl自身のパーサーがargvトークンをどう分類するかの問題であるため）。

対策は「構築技法」と「検証」を分けて考える:

- **構築技法**（2つ）:
  1. image/command/argsのような末尾の位置引数群の直前に`--`を挿入する（`kubectl exec POD -- CMD ARGS...`と同じパターン。cobra/pflagの標準機能）
  2. env/labelなどフラグの値として渡すものは`--flag=value`の結合形にし、2トークンに分けない
- **主たる検証**: 実行直前に、構築したargvの`--`より前の部分が、cderunが意図した固定パターンと**完全一致**しているかを自己チェックする。これは構築技法が正しく適用されたことを信じるのではなく、結果そのものを検証するもので、`--`の挿入忘れや結合形の実装ミスなど、構築技法側のあらゆる実装ミスを一括で検知できる。一致しなければ実行せず明示エラーとする。
- **重要な限界**: 上記の自己チェックは「cderunが意図通りargvを組み立てたか」は保証するが、「nerdctlが実際に`--`以降を位置引数として扱うか」は別問題である。`--`が「以降は一切フラグ解析しない」という意味を持つかどうかは**nerdctl内部が使っている引数解析ライブラリ（cobra/pflag）の実装依存**であり、OS/POSIXレベルの普遍的な保証ではない。nerdctlはkubectl/docker CLIと同じcobra/pflag基盤なので信頼性は高いが、これは**nerdctl固有の話**であり、将来別のCLI（Apple containerの`ArgumentParser`ベースCLIやwslc.exe等）に一般化してはならない。
- したがって、**nerdctl実バイナリに対する敵対的テスト**（`--privileged`や`--mount=...`をimage/command/args/env等の各フィールドに仕込み、実際に無害化されることを実プロセスの起動結果で確認する）を必須の完了条件とする。cderun自身の構築ロジックの単体テストだけでは不十分。
- 3層すべて（構築技法2つ＋自己チェック）を1つの共有ヘルパーとして実装し、将来の別CLIアダプタでも再利用できるようにする（ただし敵対的テストは新しいアダプタごとに必ずゼロからやり直す）。
- Base Host の設定エンジンが`nerdctl`で、かつ上記の構築技法・自己チェック・敵対的テストのいずれかを満たしていない場合、Control Socket は黙ってフォールバックしたり劣化動作を許容したりせず、**明示的なエラーで起動・マウントを拒否する**。

### 完了条件

- `nerdctl`アダプタで、Control Socket経由の`CreateContainer`/`StartContainer`/`WaitContainer`/`RemoveContainer`（非対話）が動作する。
- 構築技法（`--`挿入・`--flag=value`結合形）が共有ヘルパーとして実装されている。
- 実行直前のargv構造自己チェック（`--`より前が意図した固定パターンと完全一致するか）が実装され、不一致時は実行せず明示エラーになることを確認するテストがある。
- **nerdctl実バイナリに対する敵対的テスト**が存在し、`--privileged`等の危険な値をimage/command/args/env等の各フィールドに仕込んでも実際に特権化されない等、実プロセスの起動結果で無害化を確認している。
- 生ソケット経由（Docker実行）との比較で、機能的に同等なユースケース（非対話実行）が動作することを確認するテストがある。
- どの経路（API/CLI/生ソケット）が使われたかが `--diagnosis` / debug ログで判別できる。

---

## T91: Nested Execution Control Socket — Phase 5: 他APIベースエンジン対応・セキュリティポリシー・macOS検証

- 種別: 機能 / セキュリティ
- 優先度: 中
- 規模: 中
- 前提: T89 完了（T90 の完了は必須ではないが、テスト基盤の再利用のため T90 の後が望ましい）
- 仕様変更: あり → `docs/features/nested-execution-control-socket.md`

### 方針

- `containerd-api` アダプタへのディスパッチを追加し、`docker-compat-api`系（Docker/Podman）と合わせて Control Socket が API ベースエンジン全体で機能パリティに達するようにする。
- Security Model 節で定義した「親の許可した設定を上限とする」権限スコープ制御（inherited-ceiling モデル）を実装する。
- macOS 上での動作制約（`nested-execution.md` の VM GID 問題）が Control Socket 経由では発生しないことを検証・記録する。

### 完了条件

- Docker/Podman（`docker-compat-api`）・containerd（`containerd-api`）で Control Socket 経由のネスト実行が Phase 2/3 と同等に動作する。
- ネスト先からの要求が親の `ContainerConfig` を超える権限（イメージ・マウント・capability 等）を要求した場合に拒否されることを確認するテストがある。
- macOS 環境での検証結果が `docs/features/nested-execution-control-socket.md` に記録されている。
- 本フェーズの完了をもって `--mount-cderun-socket` の experimental 表記を外すかどうかを判断する（`docs/features/nested-execution-control-socket.md` の Compatibility and Migration 節の手順に従う）。

---

## T92: `{{file:...}}` / `{{find_dir:...}}` に `:-default` フォールバック構文を追加

- 種別: 機能
- 優先度: 中
- 仕様変更: あり

### 目的

`env` ディレクティブにのみ存在するフォールバック構文 `:-default` を `file` / `find_dir` にも横展開し、参照先が存在しない環境でも設定を壊さずに済むようにする。

動機となったユースケース: git worktree 運用では、本体側のチェックアウトを含む親ディレクトリをマウントする必要がある。現在は `{{find_dir:master}}`（`master` を上方探索し、それを含む親ディレクトリを返す）でこれを実現しているが、`master` が存在しない環境では解決に失敗して実行全体が中断してしまう。フォールバックがあれば `{{find_dir:master:-{{PWD}}}}` と書け、「worktree 運用ならその親、そうでなければカレントディレクトリ」を1つの式で表現できる。

### 仕様

1. 構文: `{{file:NAME:-DEFAULT}}` / `{{find_dir:NAME:-DEFAULT}}`。`resolveEnv`（`internal/config/expression.go`）と同じく最初の `:-` で `strings.Cut` する。
2. フォールバックの発火条件は**使用箇所に依存しない一律のルール**とする（マウント時のみ等の文脈依存ルールにはしない）。
    - フォールバックする: 対象が見つからない（`file not found` / `item not found for find_dir`）、Stat / ReadFile の失敗（`file:` にディレクトリを渡した場合を含む）。
    - フォールバックしない（従来どおり即エラー）: 引数自体の検証失敗（制御文字・不正 UTF-8・`..` traversal・絶対パス・パス区切りを含む名前）、`MaxDirectiveFileSize` 超過。
    - 理由: 引数不正は設定ミスであり、黙って隠すと発見が遅れる。サイズ上限はガードなのでフォールバックさせない。
3. `file:` で内容が空（`TrimSpace` 後に空文字）の場合は `env` の挙動（`hasDefault && val == ""`）に揃えて DEFAULT を返す。
4. DEFAULT 側もネストした式を解決できること。`resolveString` は `resolveDirective` を呼ぶ前に content を再帰解決するため（`{{env:DIR:-{{HOME}}}}` を支える既存の仕組み）、追加実装なしで動くはずだが、テストで担保する。
5. DEFAULT 値は `resolveEnv` と同様に `validatePathChars` で検証する。
6. フォールバックが成立した場合は Sticky Error を汚さない（`setError` を呼ばない）こと。
7. `resolveFile` の fileCache は失敗もキャッシュしている。同一 NAME を DEFAULT 付き / なしの両方で評価してもキャッシュが誤用されないこと（キャッシュは NAME 単位で err を保持したまま、呼び出し側で err を見て DEFAULT に落とす形にすれば要件を満たす）。
8. `resolveFindDir` は結果に `applyReverseResolution` を適用しているが、DEFAULT は「式として解決済みの文字列」なのでそのまま返す（ネスト実行時は `{{BASE_PWD}}` 等を明示的に書く）。
9. 互換性: 現在は `{{file:a:-b}}` がファイル名 `a:-b` の探索として扱われる。本変更でこれは分割されるようになる。`env` が既に同じ構文を持つこと、`:-` を含むファイル名が現実的でないことから許容する。

### 完了条件

- `{{file:NAME:-DEFAULT}}` / `{{find_dir:NAME:-DEFAULT}}` が動作する。
- 仕様 2 の発火条件どおりに分岐する（フォールバックするケース / せずにエラーになるケースの双方にテストがある）。
- ネストした DEFAULT（`{{find_dir:master:-{{PWD}}}}`）のテストがある。
- フォールバック成立時に Sticky Error が汚れないことのテストがある。
- `docs/features/value-resolution.md` および `README.md` の Value Resolution & Expression Engine セクションが更新されている。
- `make build` / `make test` / `make lint-go` / `make lint-md` がパスする。

### 対象ファイル

- `internal/config/expression.go`（`resolveFile` / `resolveFindDir` を修正。`resolveEnv` が参考実装）
- `docs/features/value-resolution.md`
- `README.md`

### スコープ外（検討のうえ不採用とした案）

- git worktree 専用ディレクティブ（`{{git_common_dir}}` 等）の追加: Expression は汎用機能であり、特定ツール・特定ワークフロー専用の構文は持たせない。
- 汎用の文字列変換パイプ（`{{file:.git|trim_prefix:gitdir: }}` 等）の導入: Expression がミニ言語化し、アンカー境界検証・エスケープとの組み合わせが爆発する。加えて worktree のケースでは `.git` ファイルの `gitdir:` を剥がしても指すのは `<common>/.git/worktrees/<name>` であり、git が `commondir` 経由で必要とする共通 `.git` には 2 階層足りないため、文字列変換だけでは目的を達成できない。
- マウントの `optional` の意味拡張（式の解決エラーもスキップ扱いにする）: 「マウントの解決時に限り」という文脈依存ルールになるため。

---

## T93: `--engine` の導入（`--runtime` は非推奨エイリアスとして温存）

- 種別: 機能
- 優先度: 高
- 規模: 大
- 前提: なし（T94 と並行可。**T90 より前に完了させること**）
- 仕様変更: あり → `docs/features/command-line-options.md`, `docs/features/multi-runtime-support.md`

### 背景

cderun の現行 `--runtime` は「どのコンテナエンジンに接続するか」（docker / podman / containerd）を指定するフラグ。一方 Docker の `--runtime` は「コンテナ実行時の OCI ランタイム」（runc / crun / nvidia / kata）を指定する。名前の衝突により、Docker 互換の OCI ランタイム指定が追加できない。

本タスクは**エンジン指定側を `--engine` へ移す作業のみ**を扱い、`--runtime` の意味は変えない（`--engine` の非推奨エイリアスとして従来どおり動作する）。破壊的変更を含まないため単独で出荷できる。

分割の理由（2026-09-10 計測）: 旧 T31 は「リネーム」と「同名フラグへの別意味の付け替え」を1タスクに含んでいた。そのため `runtime` に触れる Go ファイル 146（うちテスト 123）・1,776 行・ドキュメント 32 ファイルのすべてを「エンジン指定か / OCI ランタイム指定か」で個別判定する必要があり、旧 T31 が推奨していた一括リネームツール（`gorename` 等）が原理的に使えなかった。意味の付け替えを T95 に切り出すことで、本タスクは `Runtime` → `Engine` の機械的リネーム + 互換シムに単純化される。

### 仕様

#### リネーム

| 旧 | 新 | 備考 |
| --- | --- | --- |
| `--runtime` | `--engine` | 旧名は非推奨警告付きで従来どおり動作 |
| `--cderun-runtime` | `--cderun-engine` | 同上 |
| `CDERUN_RUNTIME` | `CDERUN_ENGINE` | 同上 |
| `.cderun.yaml` の `runtime:` | `engine:` | 同上。両方指定時は `engine:` を優先し警告 |

内部識別子も併せてリネームする: `CDERunConfig.Runtime` → `.Engine`、`ResolvedConfig.Runtime` → `.Engine`、`resolveRuntimeAndSocket` → `resolveEngineAndSocket`、`rv.res.Runtime`（非テスト 15 箇所）→ `rv.res.Engine`、`runtimeFactory` → `engineFactory`。

#### エンジン系統の分類（T90 の前提）

`--engine` の値に加えて、各エンジンが **API ベース**（`docker-compat-api`: docker / podman、`containerd-api`: containerd）か **CLI ベース**（`nerdctl` 等）かを判定できる内部分類を用意する。T90 が「Base Host の設定エンジンが `nerdctl` かどうか」を判定するために必要。

- ユーザー向けの受理値は `docker` / `podman` / `containerd` のまま変更しない（`nerdctl` の受理は T90 の範囲）
- 分類の定義は `docs/features/nested-execution-control-socket.md` の adapter family に合わせる

注記: 旧 T31 の本文にはこの分類が書かれていなかったが、「タスク間の依存・推奨順序」節の T31 → T90 依存の記述がこれを前提としていたため、本タスクに明示的に含める。

#### 移行措置

- 旧名（フラグ・環境変数・YAML キー）に 1 リリースの deprecation 期間を設け、使用時に非推奨警告を出す（`logging.Warn`）
- 期間終了後の扱いは T95 で決める

### 完了条件

- `--engine` / `--cderun-engine` / `CDERUN_ENGINE` / `.cderun.yaml` の `engine:` で docker / podman / containerd を指定できる
- 旧 `--runtime` / `--cderun-runtime` / `CDERUN_RUNTIME` / `runtime:` が非推奨警告付きで従来どおり動作する（各経路にテストがある）
- `engine:` と `runtime:` の同時指定で `engine:` が優先され警告が出るテストがある
- エンジン系統（API ベース / CLI ベース）を判定する内部 API があり、テストがある
- `docs/features/command-line-options.md` / `multi-runtime-support.md` / `README.md` / `USAGE.md` が更新されている
- `make generate` の生成物（`internal/config/cli_options.gen.go`, `internal/command/root_flags.gen.go`）が再生成・コミットされている
- `make build` / `make test` / `make lint-go` / `make lint-md` がパスする

### 対象ファイル

- `internal/config/registry.go`（`StringOptions` の `runtime` エントリ。`:384` 付近）
- `internal/config/resolver.go`（`CLIOptions`, `ResolvedConfig`）
- `internal/config/resolver_validation.go`（許可値の検証。`:426` 付近）
- `internal/config/config.go`（`CDERunConfig.Runtime`）
- `internal/command/root.go`（`rootOptions`, `resolveSettings`, `runtimeFactory`, `resolveRuntimeAndSocket`）
- 生成物: `internal/config/cli_options.gen.go`, `internal/command/root_flags.gen.go`
- テスト・ドキュメント全般

### スコープ外

- `--runtime` の意味を OCI ランタイム側へ切り替えること（T95）
- OCI ランタイム指定フラグそのものの追加（T94）
- `internal/runtime` パッケージ名および `ContainerRuntime` インターフェース名のリネーム: 実害がなく、差分を不必要に膨らませるため。必要なら別タスクとして起票する

---

## T95: `--runtime` の意味を OCI ランタイム側へ切り替え

- 種別: 破壊的変更
- 優先度: 中
- 規模: 小
- 前提: **T93・T94 の完了、および T93 の deprecation 期間（1 リリース）の経過**
- 仕様変更: あり → `docs/features/command-line-options.md`, `docs/features/multi-runtime-support.md`

### 背景

T93 の完了時点で `--runtime` はエンジン指定の非推奨エイリアスになっている。deprecation 期間の終了後、この名前を Docker 互換の意味（OCI ランタイム指定）へ移す。

同じフラグ名の意味が変わる唯一の破壊的変更であり、単独 PR に閉じ込めることで影響範囲とレビュー対象を最小化する。

### 仕様

- `--runtime` / `CDERUN_RUNTIME` / `.cderun.yaml` の `runtime:` を、エンジン指定のエイリアスから **OCI ランタイム指定のエイリアス**（`--oci-runtime` と同義）へ切り替える
- 切り替え後に値が `docker` / `podman` / `containerd` のいずれかだった場合は、OCI ランタイム名として黙って解釈せず、「エンジン指定は `--engine` に移動した」旨の明示エラーを出す
- `--oci-runtime` を残すか `--runtime` に一本化するかを本タスクで判断し、ドキュメントに記録する

### 完了条件

- `--runtime` が OCI ランタイム指定として動作する
- 旧エンジン値（`docker` / `podman` / `containerd`）を `--runtime` に渡すと `--engine` への移行を促す明示エラーになるテストがある
- ドキュメント・`README.md`・`USAGE.md` が更新され、破壊的変更が明記されている
- `make build` / `make test` / `make lint-go` / `make lint-md` がパスする

---

## T96: Control Socket サーバのリクエスト context を接続の生存に紐づける

- 種別: バグ
- 優先度: 高
- 規模: 小
- 仕様変更: なし
- 対象: `internal/runtime/controlsocket/server.go`

### 問題

`buildRequestContext`（`server.go:209-215`）は `RequestFrame.Deadline` が無いとき `context.WithCancel(context.Background())` を返し、この context は `dispatchRequest` が戻るまで cancel されない。つまり**リクエストの context がクライアント接続の生存に紐づいていない**。

`WaitContainer` のような長時間ブロックする RPC の実行中にネスト側クライアントが死ぬと:

1. `Server.Close()` は接続を閉じる（`server.go:563-565`）が、ハンドラは `d.WaitContainer` の内部でブロックしたままで、conn を閉じても ctx は cancel されない
2. ハンドラゴルーチンが終わらないため、直後の `s.wg.Wait()`（`server.go:568`）が返らない
3. 結果として Base Host の `cderun` が終了できず、内側のコンテナは孤児として残る

クライアント側は ctx キャンセル時に `conn.SetDeadline(time.Now())` で読みを叩き起こす watchdog を既に持っている（`client.go:276-286`, `client.go:375-385`）。サーバ側にだけ同等の仕組みが無く、**プロトコルの両端で堅牢性の水準が非対称**になっている。

### 仕様

- 各リクエストの context を、接続の切断および `Server.Close()` の両方で cancel されるようにする
- 接続の切断検知は、クライアント側の watchdog と対称な方法で実装する
- `Server.Close()` のシャットダウン契約として実現可能な有界待機（bounded-wait）ポリシーを定義する: ディスパッチャーハンドラ待機処理自体をデッドライン意識（deadline-aware）にし、無条件の `s.wg.Wait()` ではなくコンテキスト意識型完了シグナル（`select` / チャネル）を用いる。設定されたデッドライン経過後に `Close` が迅速に復帰（return）し、`Close` を超えて生存するハンドラの追跡・クリーンアップ方法を定義する。

### 完了条件

- 長時間ブロックする RPC（`WaitContainer`）の実行中にクライアント接続を切断すると、サーバ側のハンドラが速やかに終了するテストがある
- `Server.Close()` が有界待機ポリシーに従い、設定されたデッドライン内に迅速に復帰することを確認するリグレッションテスト（意図的に終了しないハンドラを用いてハングせず復帰・ログ出力・追跡を行うことを検証）がある
- `make build` / `make test` / `make lint-go` がパスする

### 関連

- 孤児コンテナの後始末という観点では T22（`--prune`）と関連する。本タスクは「Base Host が終了できない」ことの解消に限定し、孤児コンテナの回収は T22 の範囲とする

---

## T97: Control Socket サーバの accept ループ堅牢化とアイドルタイムアウト

- 種別: バグ
- 優先度: 中
- 規模: 小
- 仕様変更: なし
- 対象: `internal/runtime/controlsocket/server.go`

### 問題

**1. accept ループが任意のエラーで恒久停止する**

`acceptLoop`（`server.go:96-125`）は `Accept()` がエラーを返すと、`s.closed` が閉じていない場合でも Debug ログを1行出して `return` する。`EMFILE`（fd 枯渇）や `ECONNABORTED` のような一時的なエラーでもサーバが以後まったく接続を受け付けなくなり、しかもデフォルトログレベルでは何も表示されない（T82 でデフォルトが error 化されているため）。ネスト実行ツリー全体を支えるサーバとしては脆い。

**2. ハンドシェイク後にアイドルタイムアウトが無い**

`handleConn` はハンドシェイクに 5 秒の read deadline を設定するが、成立後は `conn.SetReadDeadline(time.Time{})` で完全に解除する（`server.go:184`）。以後リクエストを送ってこないクライアントがゴルーチンと fd を無期限に保持する。

### 仕様

- `Accept()` のエラーは一時的なものと恒久的なものを区別し、一時エラーは指数バックオフで再試行する（`net/http` の accept ループが参考実装）
- 恒久エラーで停止する場合は Debug ではなく Warn 以上で、停止した事実が分かるログを出す
- ハンドシェイク成立後の接続にアイドルタイムアウトを設ける。ただしアイドルタイムアウト免除は `WaitContainer` / `AttachContainer` RPC 操作の処理期間中に限定（スコープ化）し、ディスパッチ終了後（次のリクエスト処理前）に通常のアイドルデッドラインを復元・再設定する。これにより、長時間無通信になり得る操作中の接続保護と、操作完了後の通常アイドルタイムアウト回収を両立する。

### 完了条件

- 一時的な accept エラーの後もサーバが接続を受け付け続けることを確認するテストがある
- 恒久エラーでの停止時に Warn 以上のログが出ることを確認するテストがある
- アイドル接続が回収されることを確認するテストがある
- `WaitContainer` および対話実行（`AttachContainer`）の処理中はアイドルタイムアウト適用から除外され切断されないことを確認するテストがある
- `WaitContainer` を送信してレスポンスを受信した後に無通信（黙り込む）状態になった接続について、次のリクエスト処理前に通常のアイドルデッドラインが復元され正しく切断・回収されることを検証するテストがある
- `make build` / `make test` / `make lint-go` がパスする

---

## T98: Control Socket ハンドラの共通化とエラー処理方針の統一

- 種別: リファクタ
- 優先度: 中
- 規模: 小
- 仕様変更: なし
- 対象: `internal/runtime/controlsocket/server.go`

### 問題

**1. 6つの RPC ハンドラが同一の定型コードを逐語コピーしている**

`handleCreateContainer` / `handleStartContainer` / `handleWaitContainer` / `handleRemoveContainer` / `handleSignalContainer` / `handleResizeContainerTTY` が、いずれも以下を繰り返している:

- 前置き: `s.mu` ロック → `s.dispatcher` 取得 → アンロック → nil チェック → `json.Unmarshal(payload, &args)` → 失敗時 `sendErrorResponse`
- 後置き: `resp := ResponseFrame{Success: true}` → `json.Marshal` → `WriteFrame`

合計で約100行。ハンドラを1つ追加するたびにこの定型が増える。ジェネリクスを使った1つのヘルパー（引数型でパラメタライズし、実処理だけをクロージャで受ける）に畳める。

**2. `json.Marshal` のエラー処理が不統一**

`respBytes, _ := json.Marshal(resp)` の形で戻り値のエラーを捨てている箇所が6つある。一方 `handleCreateContainer` は結果構造体の marshal エラーは丁寧に検査して `sendErrorResponse` している（`server.go:281-285`）。同じ関数の中で方針が割れている。

`ResponseFrame` の marshal は実際には失敗しないが、「失敗しないから捨てる」のか「検査すべきだが漏れている」のかがコードから読み取れない。方針を決めて統一すること。

### 仕様

- 共通ヘルパーを1つ用意し、6ハンドラをその上に載せ替える
- `json.Marshal` のエラー方針を統一する。捨てる場合は、なぜ失敗し得ないのかをコメントで明示する
- 挙動（ワイヤ上のフレーム内容・エラーメッセージ文言）は変更しないこと。既存テストがそのまま通ることを確認する

### 完了条件

- 6ハンドラの定型コードが共通化され、`server.go` の行数が有意に減っている
- `json.Marshal` のエラー処理方針が統一され、捨てている箇所には理由のコメントがある
- 既存の Control Socket テストが変更なしで通る
- `make build` / `make test` / `make lint-go` がパスする

### 検討事項（実装者が判断する）

- `ResponseFrame.Payload` / `RequestFrame.Payload` が `[]byte` のため、`encoding/json` の仕様上 base64 文字列としてエンコードされ、JSON の中に base64 で JSON を埋める二重エンコードになっている。`json.RawMessage` にすればデバッグ時にフレーム内容がそのまま読める。ただしワイヤ互換が壊れるため、`CurrentProtocolVersion` の扱いと合わせて判断すること。互換を壊す判断をするなら本タスクから切り出して別タスクにする

---

## T99: テストファイル命名規約の強制

- 種別: クリーンアップ / ルール整備
- 優先度: 高
- 規模: 大
- 仕様変更: なし
- 対象: `internal/**/*_test.go`, `AGENTS.md`, `docs/testing/organization.md`

### 問題

テストコードが 47,224 行に対し実装は 15,131 行（3.1倍）。`internal/config` は実装14ファイルに対しテスト104ファイル（2026-09-10 計測）。

`docs/testing/organization.md` 3.3 の「スコープ別テストファイルルール」自体は正しく、コンフリクト回避のために新規ファイルを作る運用は守られている。崩れているのは**命名**で、237 テストファイル中 **83 ファイルが活動名ベース**になっている:

- `jules_test_refinement_extra_test.go`
- `feature_scenarios_comprehensive_resilience_expansion_test.go`
- `test_improvement_additional_test.go`
- `path_extra_coverage_test.go`

規約が例示しているのは機能・チケット・テーマ単位のスコープ名（`feature_shm_size_test.go`, `bugfix_issue42_test.go`, `resolver_robustness_test.go`）であり、「improvement」「expansion」「refinement」「comprehensive」「extra」「deep」といった**作業の性質**を表す語は規約違反にあたる。

実害は、何を守るテストなのかがファイル名から判別できないことで、重複が検出されないまま増え続ける点にある。テストを消す判断ができないため、体積は単調増加する。

### 分割

規模が大きく 1 PR に収まらないため、以下に分割して実施する（AGENTS.md 第3節 項目4）。

- **T100**: 規約の明文化と lint / CI ゲートの追加（新規・変更ファイルのみを対象）
- **T101**: `internal/config` のリネーム
- **T102**: `internal/command` のリネーム
- **T103**: `internal/runtime` のリネーム

T101〜T103 は T100 の完了を待たずに着手してよい（禁止語の一覧は既に `AGENTS.md` の Testing-First に記載済みのため）。ただし T100 が先に入っていれば各リネーム PR がその場で検証されるので、**推奨順序は T100 → T101〜T103（3つは並行可）**。

### 完了条件

- T100 〜 T103 がすべて完了している
- **全ツリーを対象とした違反ゼロ検証**（禁止語を含むテストファイル名が 0 件）がパスし、それを CI の必須チェックとして固定している
- 重複削除の結果としてテスト行数が削減されている（削減率は問わないが、削除したテストの一覧が各 PR の説明にある）
- `make test` の結果が変更前と同じで、カバレッジが有意に下がっていない

---

## T100: テストファイル命名規約の明文化と lint / CI ゲートの追加

- 種別: クリーンアップ / ルール整備
- 優先度: 高
- 規模: 小
- 前提: なし
- 仕様変更: なし
- 対象: `AGENTS.md`, `docs/testing/organization.md`, `Makefile`, `.github/workflows/ci.yaml`

### 仕様

- 禁止語（`improvement` / `expansion` / `refinement` / `comprehensive` / `additional` / `extra` / `more` / `deep` / `jules` 等のエージェント名）と命名例を `docs/testing/organization.md` 3.3 に明記する（`AGENTS.md` の Testing-First には記載済みのため、そこからの参照で足りるなら重複させない）
- 禁止語を含むテストファイル名を検出する lint コマンド（`make lint-test-names` 等）を追加する
- CI の必須チェックに組み込む。**対象は新規・追加されたファイル名に限定する**（既存の違反 83 件は T101〜T103 で解消するため、この時点で全ツリーを対象にすると CI が赤のままになる）
- 既存違反を除外する仕組みを使う場合、除外リストは自動生成とし、手動追記でゲートを迂回できない構造にすること

### 完了条件

- `make lint-test-names` 相当のコマンドが存在し、禁止語を含むファイル名を検出する
- 新規・変更されたテストファイルが CI で必ず検証される（既存違反の有無に関わらず独立して判定されること）
- 除外リストを用いる場合、手動追記による迂回ができないことを確認するテストまたは手順がある
- `docs/testing/organization.md` に禁止語と命名例が記載されている
- `make lint-go` / `make lint-md` がパスする

### 補足

全ツリーを対象とする違反ゼロ検証は、T101〜T103 の完了後に親タスク T99 の完了条件として実施する。本タスクの完了条件には含めない（含めると T101〜T103 が本タスクに依存し、循環する）。

---

## T101: `internal/config` のテストファイルリネームと重複削除

- 種別: クリーンアップ
- 優先度: 高
- 規模: 中
- 前提: なし（T100 が先に入っていることを推奨）
- 仕様変更: なし
- 対象: `internal/config/*_test.go`

### 仕様

1. 禁止語を含むファイル名を、何を検証しているかに基づいて改名する（`feature_*` / `bugfix_*` / `<対象>_robustness_*` 等）。1ファイルが複数テーマを含む場合は分割する
2. **リネームのみの PR と、重複削除を含む PR を分ける。先にリネームだけを済ませること**。両方を1つの PR に入れると差分が読めなくなる
3. 重複削除では、同一の入力・同一の assertion を別ファイルで繰り返しているものを1つに統合する

### 完了条件

- `internal/config` 配下に禁止語を含むテストファイル名が 0 件
- **削除したテストの一覧が PR 説明にある**（何を失ったかがレビュー可能であること）
- `make test` がパスし、`internal/config` のカバレッジが有意に下がっていない
- `make lint-go` がパスする

---

## T102: `internal/command` のテストファイルリネームと重複削除

- 種別: クリーンアップ
- 優先度: 高
- 規模: 中
- 前提: なし（T100 が先に入っていることを推奨。T101 と並行可）
- 仕様変更: なし
- 対象: `internal/command/*_test.go`

### 仕様

T101 と同じ（リネーム先行、重複削除は別 PR）。

### 完了条件

- `internal/command` 配下に禁止語を含むテストファイル名が 0 件
- 削除したテストの一覧が PR 説明にある
- `make test` がパスし、`internal/command` のカバレッジが有意に下がっていない
- `make lint-go` がパスする

---

## T103: `internal/runtime` のテストファイルリネームと重複削除

- 種別: クリーンアップ
- 優先度: 高
- 規模: 中
- 前提: なし（T100 が先に入っていることを推奨。T101 / T102 と並行可）
- 仕様変更: なし
- 対象: `internal/runtime/**/*_test.go`

### 仕様

T101 と同じ（リネーム先行、重複削除は別 PR）。

### 完了条件

- `internal/runtime` 配下（`controlsocket` を含む）に禁止語を含むテストファイル名が 0 件
- 削除したテストの一覧が PR 説明にある
- `make test` がパスし、`internal/runtime` のカバレッジが有意に下がっていない
- `make lint-go` がパスする

