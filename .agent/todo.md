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
| T01 | TTY 経由実行でターミナルが強制終了する問題の調査 | 調査 | 高 | ? | - | - |
| T05 | `CLIOptions` の `Set` フィールドをポインタ型に統一 | リファクタ | 高 | 中 | - | - |
| T06 | `--cderun-*` フラグのボイラープレートをコード生成化 | リファクタ | 中 | 大 | - | - |
| T07 | `preprocessArgs` の引数ホイスト簡略化 | リファクタ | 中 | 中 | あり | - |
| T09 | `AttachContainer`（Docker）の stdin エラー握りつぶし修正 | バグ | 低 | 小 | - | DONE |
| T11 | 未知の `{{...}}` ディレクティブをエラーにする | 挙動変更 | 中 | 中 | あり | DONE |
| T12 | `IsRetryablePullError` を型付きエラー判定に移行 | 改善 | 中 | 小 | - | DONE |
| T14 | `Phase N` コメント前後の整理 | クリーンアップ | 低 | 小 | - | DONE |
| T15 | containerd `AttachContainer` のポーリング排除 | 改善 | 低 | 小 | - | - |
| T16 | ランタイム未対応機能の事前バリデーション | 改善 | 中 | 中 | - | - |
| T18 | `ci.yaml` のアクションをコミットハッシュ固定 | CI | 高 | 小 | - | - |
| T19 | CI の Go バージョン指定を `go.mod` に一本化 | CI | 低 | 小 | - | - |
| T20 | Docker / Podman のランタイムテストを CI に追加 | CI | 中 | 中 | - | - |
| T21 | イメージ事前取得フラグ（`--prefetch`） | 機能 | 中 | 中 | あり | - |
| T22 | orphan コンテナのクリーンアップ（`--prune`） | 機能 | 中 | 大 | あり | - |
| T23 | `--group-add` フラグの追加 | 機能 | 高 | 小 | あり | - |
| T24 | `--shm-size` フラグの追加 | 機能 | 高 | 小 | あり | - |
| T25 | `--init` フラグの追加 | 機能 | 高 | 小 | あり | - |
| T26 | `--pid` フラグの追加 | 機能 | 高 | 小 | あり | - |
| T27 | `--read-only` フラグの追加 | 機能 | 高 | 小 | あり | - |
| T28 | `--ulimit` フラグの追加 | 機能 | 中 | 小 | あり | - |
| T29 | `--security-opt` フラグの追加 | 機能 | 中 | 小 | あり | - |
| T30 | `--sysctl` フラグの追加 | 機能 | 中 | 小 | あり | - |
| T31 | `--runtime` を `--engine` にリネーム + OCI `--runtime` 追加 | 機能/破壊 | 高 | 中 | あり | - |
| T32 | `--dns-search` フラグの追加 | 機能 | 中 | 小 | あり | - |
| T33 | `--dns-option` フラグの追加 | 機能 | 中 | 小 | あり | - |
| T34 | `--ipc` フラグの追加 | 機能 | 中 | 小 | あり | - |
| T35 | `--gpus` フラグの追加 | 機能 | 中 | 中 | あり | - |
| T36 | `--cgroupns` フラグの追加 | 機能 | 中 | 小 | あり | - |
| T37 | `--pids-limit` フラグの追加 | 機能 | 中 | 小 | あり | - |
| T38 | `--cpu-shares` / `--cpuset-cpus` / `--cpuset-mems` フラグの追加 | 機能 | 中 | 小 | あり | - |
| T39 | `--restart` フラグの追加 | 機能 | 低 | 小 | あり | - |
| T40 | containerd: コマンド指定時にイメージの ENTRYPOINT が消失する | バグ | 高 | 中 | - | - |
| T41 | snapshot 一時ディレクトリが `os.Exit` によりリークする | バグ | 高 | 小 | - | - |
| T42 | 空文字サブコマンドで nil panic | バグ | 高 | 小 | - | - |
| T43 | attach エラー分岐で hang-timeout 0 が「即時タイムアウト」になる | バグ | 高 | 小 | - | - |
| T44 | `preprocessArgs` のフラグ lookup が機能せずサブコマンドを誤認する | バグ | 高 | 小 | - | - |
| T45 | containerd: cap-add / cap-drop / dns / add-host が黙って無視される | セキュリティ | 高 | 中 | - | DONE |
| T46 | 設定レイヤーのマージで `BaseDir` が汚染される | バグ | 中 | 小 | - | - |
| T47 | エラー時にコンテナの exit code が破棄される | 改善 | 中 | 中 | あり | - |
| T48 | Docker AutoRemove と `WaitContainer` の競合で exit code が失われる | バグ | 中 | 中 | - | - |
| T49 | Docker 明示 Remove で匿名ボリュームがリークする | バグ | 中 | 小 | - | - |
| T50 | pull ポリシーの未知値が `always` として動作する | 改善 | 中 | 小 | - | DONE |
| T51 | containerd: `volume` / `tmpfs` マウントが不正な OCI spec になる | バグ | 中 | 小 | - | DONE |
| T52 | コンテナ起動前後のシグナルハンドリングの隙間（SIGHUP 含む） | 改善 | 中 | 中 | あり | - |
| T53 | 引数ホイストの `--` エスケープ対応 | 挙動変更 | 中 | 小 | あり | - |
| T54 | 環境変数の bool/int/float パース失敗が黙殺される | 改善 | 中 | 小 | - | - |
| T55 | CLI `--device` が不正な perms を黙認する | 改善 | 低 | 小 | - | - |
| T56 | ポート番号の範囲検証（0 / 負数 / 65535 超） | 改善 | 低 | 小 | - | - |
| T57 | `{{file:...}}` のサブパス許可と設定ファイルの信頼境界 | セキュリティ | 中 | 中 | あり | - |
| T58 | ランタイム自動検出が substring マッチで誤検出し得る | 改善 | 低 | 小 | - | - |
| T59 | クリーンアップ用 `RemoveContainer` にタイムアウトがない | 改善 | 低 | 小 | - | - |
| T60 | duration オプションが式解決エラーを握りつぶす | 改善 | 低 | 小 | - | - |
| T61 | Docker attach: stdin エラー時に出力を drain せず切断する | 改善 | 低 | 小 | - | - |
| T62 | containerd: `ioWait` 削除の競合と attach 順序契約の明文化 | 改善 | 低 | 小 | - | - |
| T63 | CI と `docs/testing/` のカバレッジ・パイプライン乖離の解消 | CI | 中 | 中 | - | DONE |
| T64 | CLI help / Makefile の文字列修正（containerd・mask-all 反映） | クリーンアップ | 低 | 小 | - | - |
| T65 | dead code 削除・小規模クリーンアップ一括 | クリーンアップ | 低 | 小 | - | - |
| T66 | テスト専用ヘルパーを `_test.go` に移動 | クリーンアップ | 低 | 小 | - | - |
| T67 | 早期ロガー初期化がフォーマット指定を無視し、不正レベルを黙殺する | 改善 | 低 | 小 | - | - |
| T68 | dry-run ゴールデンテスト基盤（L2） | テスト | 高 | 中 | - | - |
| T69 | registry 駆動の優先順位マトリクステスト生成（L1） | テスト | 高 | 中 | - | - |
| T70 | `ContainerRuntime` コンフォーマンススイート（L3） | テスト | 高 | 大 | - | - |
| T71 | mutation testing の導入 | テスト/CI | 中 | 中 | - | - |
| T72 | 既存 coverage 系テストの段階的整理・吸収 | クリーンアップ | 低 | 大 | - | - |
| T73 | ソースコード内コメントの英語化 + `ContainerConfig` の変換契約コメント追加 | クリーンアップ | 中 | 小 | - | - |

依存関係・統合の注意:

- **T05 と T06 は統合可能**（`registry.go` のメタデータを single source of truth にすれば両方解決する）。別々に着手する場合は T05 → T06 の順
- **T22 は「ラベル付与」を先行サブタスクとして切り出し可能**（移行問題の縮小）
- **T69 は T05/T06 と統合実装を強く推奨**（registry メタデータからテストを生成するため、single source of truth 化と同じ作業）
- **T70 は T20 と統合実装を推奨**（CI の実ランタイムジョブをコンフォーマンススイートの器として作る）。T40/T45/T51 の再現ケースを必ず含めること
- **テスト系タスク（T68〜T72）に着手する前に `docs/testing/strategy.md` を必ず読むこと**。推奨着手順は T68 → T69 → T70 → T71 → T72

---

## T01: TTY 経由実行でターミナルが強制終了する問題の調査

- 種別: 調査（バグ再現・原因特定）
- 対象: `internal/command/root.go`（`setupTerminal`, `startResizeHandler`）

### 現象

macOS ターミナルで cderun 経由で kiro-cli を実行中、カーソルがターミナルの右端に到達するとターミナル自体が強制終了される。TTY ハンドリングまたはリサイズシグナル周りの問題の可能性あり。

### 調査の手がかり

- raw mode は stdin の fd に対して設定され（`root.go` の `setupTerminal`）、リサイズ処理は stdout の fd を基準にしている（`startResizeHandler`）
- TTY=true 時は `io.Copy` で生バイトをそのまま流すため、コンテナ側プロセスが出力するエスケープシーケンスが無加工でホストターミナルに届く
- 「右端到達で死ぬ」のは auto-wrap (DECAWM) 関連のシーケンスや、起動直後の `ResizeContainerTTY` に渡るサイズが怪しい
- 再現時は `--cderun-log-level trace` でリサイズイベントのログを確認する

### 完了条件

- 原因が特定され、再現手順とともに記録されている（修正は別タスク化してよい）


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

## T06: `--cderun-*` フラグのボイラープレートをコード生成化

- 種別: リファクタリング
- 対象: `internal/config/registry.go`、`internal/command/flags.go`、`internal/config/resolver.go`

### 問題

すべてのフラグに通常版（`--tty`）と内部オーバーライド版（`--cderun-tty`）が存在し、`rootOptions`・`CLIOptions`・`flags.go` の 3 箇所に手書きで反映する必要がある。追加漏れや不整合が起きやすい。

### 方針

生成元はゼロから作る必要がない。`internal/config/registry.go` に既に `StringOptions` / `BoolOptions` / `IntOptions` / `Float64Options` / `StringSliceOptions` としてオプションのメタデータ（Name / FieldName / EnvKey / Shorthand / Getter）が一元化されている。これを **唯一の定義源（single source of truth）** として `go generate` で `flags.go` と `CLIOptions` を生成する。

これにより `docs/guidelines/working-guide.md`（42-63 行目）の「新オプション追加チェックリスト」の大半を機械化できる（= AI エージェントの追加漏れ防止という本来の目的に直結）。

### 完了条件

- registry のメタデータ 1 箇所を変更すれば新オプションが追加できる状態になっている
- 生成コードと手書きコードの境界が明確（生成ファイルにヘッダコメント）
- `docs/guidelines/working-guide.md` のチェックリストを新手順に合わせて更新

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

1. 現状はサブコマンド「前」の `--cderun-*` をエラーにしている（`root.go:1261-1264`、エラーメッセージ `"cderun internal override flag %q must be placed after the subcommand"`）。スケッチは位置を区別しない
2. `--cderun-foo value` 形式で次引数を値として取るかどうかは、本来フラグ定義を引かないと判定できない（bool フラグの直後の引数を誤って食う）

簡略化する場合も「`--cderun-*` は `=` 形式のみ許可する」と仕様を縛れば判定不要になり、最も安全.

### 完了条件

- 採用した仕様が `docs/features/argument-parsing.md` に反映されている
- サブコマンド前 `--cderun-*` の扱い、bool フラグ直後の引数の扱いについてテストがある

---

## T09: `AttachContainer`（Docker）の stdin エラー握りつぶし修正

- 種別: バグ修正
- 対象: `internal/runtime/docker.go:318-347`

### 問題

stdout が先に終了した場合、stdin エラーは `stdinDone` がすでに閉じているときだけチェックされる（`select` に `default:` があるため競合次第でスキップ）。stdin 側でエラーが発生してもほとんどの場合に握りつぶされる。

### 実装上の注意

- stdin の `io.Copy` はユーザー端末からの Read で永久にブロックし得るため、「outputDone 後に stdinDone を待つ」修正は **不可（ハングする）**
- `stdinErr` を mutex か atomic で保持し、**待たずに読むだけ** の形にするのが安全
- `context.Canceled` は正常系（明示キャンセル）なのでエラー扱いから除外すること

### 完了条件

- stdin エラーが競合に依存せず報告される
- TTY セッションが従来どおりハングせず終了する（既存テストの回帰確認）

---

## T11: 未知の `{{...}}` ディレクティブをエラーにする

- 種別: 挙動変更
- 対象: `internal/config/expression.go:270`
- 仕様変更: あり → `docs/features/value-resolution.md` の更新必須

### 問題

`{{HOM}}` のようなタイポでも無音で文字列がそのまま通り、コンテナに `{{HOM}}` を含むパスが渡ってしまう。

```go
// 現状（expression.go:270）
return "{{" + content + "}}", nil // Keep as is if unknown

// 改善案
return "", fmt.Errorf("unknown directive: %q", content)
```

### 実装上の注意（単純にエラー化すると 2 つ壊れる）

1. `resolveString` の単一式最適化パス（`expression.go:181-189`）は「結果が `{{` で始まったままか」（185 行目の `!strings.HasPrefix(res, "{{")`）で解決失敗を判定して全体スキャンへフォールバックしており、この分岐の書き換えが必要
2. 設定値にリテラルの `{{...}}` を書くユースケース（Go template / Helm テンプレートを env 値や entrypoint に渡す等）が即エラーになる

対策として、エラー化の対象を「既知ディレクティブに似たもの」（大文字英字のみ、または `env:` / `file:` / `find_dir:` 風の prefix）に限定するか、`{{{{` のようなエスケープ記法を導入する。

### 完了条件

- タイポ（`{{HOM}}` 等）が起動前にエラーになる
- リテラル `{{...}}` を通すための仕様（限定エラー化 or エスケープ記法）が決定され、`docs/features/value-resolution.md` に明記されている
- ネスト式（`{{env:{{VAR}}}}` 等）の回帰テストが通る

### 完了確認（2026-07-03）

実装済みを確認。`expression.go:352-366` で ALL_UPPER 候補と `:` を含む未知ディレクティブがエラーになり、`{{{{...}}}}` エスケープ（`expression.go:273-274`）と `docs/features/value-resolution.md` の仕様記載も存在する。

---

## T12: `IsRetryablePullError` を型付きエラー判定に移行

- 種別: 改善（堅牢性）
- 対象: `internal/runtime/common.go:12-52`

### 問題

エラーメッセージの文字列でリトライ可否を判定しており、ライブラリ側のメッセージ変更で気づかず壊れるリスクがある。また `"no such host"` は DNS 解決失敗（≒ほぼ設定ミス）であり一時的なエラーではないため、リトライするとユーザーが原因究明に詰まる。

### 方針

- `errdefs` の型付きエラー（`IsUnavailable`、`IsDeadlineExceeded` 等）を優先的に使う（`common.go:18-25` で既に部分使用済み）
- string マッチングは最小限に絞り、`"no such host"` は対象から外す
- HTTP ステータスコードで判定できるケース（429 Rate Limit、503 Unavailable）はそちらを使う

### 型付き判定の追加材料

- `"no such host"` は `*net.DNSError` として `errors.As` で取れる。`IsNotFound` フィールドで「ドメイン不存在 = 設定ミス」と「DNS サーバ不達 = 一時障害」を区別できるので、全部リトライ外しではなく後者だけ残す選択肢もある
- `"toomanyrequests"` はレジストリの errcode（`TOOMANYREQUESTS`）由来なので docker/distribution の errcode 型でマッチ可能
- `"i/o timeout"` 系は `net.Error` の `Timeout()` で取れる

### 完了条件

- string マッチが型付き判定に置き換わっている（残す場合は理由をコメントで明記）
- 「リトライすべき/すべきでない」の代表ケースについてテーブルドリブンテストがある

### 残作業メモ（2026-07-03 検証）

型付き判定（`errdefs`、`*net.DNSError` の `IsNotFound` 区別、`net.Error.Timeout()`）とテーブルドリブンテストは実装済み。ただし `common.go:49-63` の 13 エントリの string リストには「残す理由」のコメントがなく、`"timeout"` / `"rate limit"` のような広い substring が残存、`"toomanyrequests"` の errcode 型移行も未実施。小規模フォローアップとして T65 に含めず単独で対応してもよい。

---

## T14: `Phase N` コメントの整理

- 種別: クリーンアップ
- 対象: `internal/config/registry.go`、`internal/config/resolver.go`

### 問題

`// Phase 4: Complex resolution` 等のコメントは実装時のタスク分解の名残。フェーズ番号自体はもはや意味をなしておらず、`SkipResolution: true` の理由（なぜスキップするのか）だけをコメントに残す形に整理したい。

```go
// 現状
SkipResolution: true, // Phase 4: Complex resolution

// 改善案
SkipResolution: true, // resolved separately in ResolveWithFS (requires merge logic)
```

### 補足

Phase コメントは `registry.go` と `resolver.go` の両方に散在する（Phase 1〜8 を確認済み）。`grep -n 'Phase [0-9]'` で両ファイルまとめて洗い出して整理すること。

### 完了条件

- `Phase N` 形式のコメントが消え、各 `SkipResolution` に実質的な理由コメントが付いている（挙動変更なし）

### 完了確認（2026-07-03）

実装済みを確認。`internal/config/` の非テストファイルに `Phase [0-9]` 形式のコメントは残っておらず、`registry.go` の `SkipResolution` には実質的な理由コメントが付いている。

---

## T15: containerd `AttachContainer` のポーリング排除

- 種別: 改善（堅牢性）
- 対象: `internal/runtime/containerd.go:420` 付近

### 問題

containerd は IO をタスク作成時に渡す必要があるため、`AttachContainer` 内で goroutine が 100ms ごとにタスクの出現を待つポーリングになっている。Docker のネイティブ attach と比べてアーキテクチャ上の回避策であり、コンテナが瞬時に終了した場合などエッジケースで動作が不安定になる可能性がある。

### 方針（2 案、(b) 推奨）

1. (a) containerd の events API（`client.Subscribe`）で TaskStart イベントを待つ — 汎用的だがイベント購読のエラー処理が増える
2. (b) 既存の `ioMap`（`containerd.go:37-38`）と同様に `taskReady map[string]chan struct{}` を持ち、`StartContainer` がタスク生成完了時に通知する — 外部依存なし・変更量小。cderun は自分で起動したコンテナにしか attach しないため十分

### 完了条件

- 100ms ポーリングが同期プリミティブによる通知に置き換わっている
- コンテナが即終了するケースでハング・リークしないことのテストがある（`docs/testing/` のリーク防止指針を参照）

---

## T16: ランタイム未対応機能の事前バリデーション

- 種別: 改善（UX）
- 対象: `internal/runtime/containerd.go:150-155`、`internal/runtime/interface.go`（`ContainerRuntime` インターフェース）

### 問題

`--network bridge` や `--publish` を containerd ランタイムで使うと、コンテナ作成時（実行時）に初めて "not supported yet" エラーになる。設定ロードや起動前のバリデーション段階で弾く方がユーザー体験が良い。また "not supported yet" のまま放置されているため、対応予定があるなら issue 化したい。

### 方針（2 案）

1. `ContainerRuntime` インターフェースに `Capabilities()` メソッド（未対応機能の宣言）を追加し、`initContainer` の前に `resolved` の値と突き合わせる — ランタイム毎の対応差分が宣言的になり、Podman 固有の制限が将来見つかった場合にも同じ仕組みで対応できる
2. バリデーション専用メソッド `ValidateConfig(cc *ContainerConfig) error` をインターフェースに足す — さらに単純で、こちらでも十分

### 完了条件

- containerd + `--network` / `--publish` がコンテナ作成前にエラーになる
- エラーメッセージに「どのランタイムが何を未対応か」が含まれる

---

## T18: `ci.yaml` のアクションをコミットハッシュ固定

- 種別: CI / セキュリティ
- 対象: `.github/workflows/ci.yaml`

### 問題

`release.yaml` ではコミットハッシュ固定済みだが、`ci.yaml` はタグ指定のまま（`actions/checkout@v4`, `actions/setup-go@v5` 等）。タグは書き換え可能なためサプライチェーン攻撃のリスクがある。

### 方針

`pinact` で自動置換する（**LLM による手書き置換は幻覚のリスクがあるため不可**）。

```bash
pinact run .github/workflows/ci.yaml
```

`pinact run` は引数なしで `.github/workflows/` 全体を処理できる。`pinact run -u` でピン済みハッシュの更新も可能なので、定期的なメンテにも使える。

### 完了条件

- `ci.yaml` の全アクションがコミットハッシュ + バージョンコメント形式になっている
- ハッシュは pinact の出力そのまま（手書き改変なし）

---

## T19: CI の Go バージョン指定を `go.mod` に一本化

- 種別: CI / メンテナンス性
- 対象: `.github/workflows/ci.yaml`

### 問題

`ci.yaml` は `go-version: '1.25.0'` とハードコードされているが、`release.yaml` は `go-version-file: go.mod` を使っている。方針を統一し `ci.yaml` も `go-version-file: go.mod` にする。

### 補足（検証済み）

`go.mod` の go directive は `go 1.25.0` であり、ci.yaml のハードコードは現状 go.mod と一致している。したがってこの変更は挙動を変えず「バージョン管理箇所を go.mod に一本化する」だけの安全な変更。最新（1.26.x）へ上げたい場合は go.mod 側を更新すれば CI も追従する形になる。

### 完了条件

- `ci.yaml` の全ジョブが `go-version-file: go.mod` を使用し、CI がグリーン

---

## T20: Docker / Podman のランタイムテストを CI に追加

- 種別: CI / テストカバレッジ
- 対象: `.github/workflows/ci.yaml`

### 問題

containerd のインテグレーションテストジョブはあるが、Docker と Podman のテストが存在しない。`ubuntu-latest` には Docker が標準搭載されているため、Docker のランタイムテストは比較的容易に追加できる。

### 方針

ランタイムテストは `CDERUN_RUNTIME` / `CDERUN_SOCKET_PATH` 環境変数で対象を切り替える構造（ci.yaml の containerd ジョブ参照: `CDERUN_RUNTIME=containerd`, `CDERUN_SOCKET_PATH=/run/containerd/containerd.sock`）。

- **Docker**: `ubuntu-latest` の `/var/run/docker.sock` がそのまま使える。runner ユーザーは docker グループ所属済みのため ACL 設定も不要のはず
- **Podman**: `sudo apt-get install -y podman` 後、`systemctl --user enable --now podman.socket` でソケットは `/run/user/$(id -u)/podman/podman.sock`

### 完了条件

- Docker / Podman それぞれのインテグレーションテストジョブが追加され、CI がグリーン
- ジョブ構成は既存 containerd ジョブと一貫した形式

---

## T21: イメージ事前取得フラグ（`--prefetch`）

- 種別: 機能追加
- 対象: `internal/command/`、`internal/config/registry.go`
- 仕様変更: あり → `docs/features/` に新規仕様ドキュメントを作成（Document-First）

### 目的

オフライン環境への持ち込みや CI ウォームアップのため、コンテナを実行せずにイメージだけ事前取得するフラグを追加する。

```bash
cderun --prefetch-all          # .tools.yaml の全イメージを取得して終了
cderun --prefetch node,go      # 指定ツールのイメージのみ取得して終了
```

### 設計上の決定事項

- `--pull` はすでに pull ポリシー（always/missing/never、`registry.go:331-341`）で使用済みのため別名にする
- 実装は `--diagnosis`（`registry.go:539-543`）と同パターン（`handlePrefetch` を追加、設定ロード → `PullImage` を呼ぶだけ）
- サブコマンド方式は「`pull` というツールを実行する」と解釈されるため **不可**。フラグ方式が適切

### 仕様化しておくべき点（docs に明記すること）

1. pull ポリシーとの整合 — prefetch は `missing` 相当で良いか、`--pull always` 併用で強制再取得を許すか
2. **exit code** — 1 つでもイメージ取得に失敗したら非 0 で終了する（オフライン持ち込み前の検証としてはこれが重要）
3. 既存の `pullMaxRetries` / `pullBackoffBase`（`registry.go:343-354`）はそのまま再利用可能
4. オフライン用途では tag が動く（`latest` 等）と持ち込み後に意図と違うイメージになるため、`--diagnosis` 等で digest を表示できると再現性の確認に役立つ

### 完了条件

- 仕様ドキュメントが先に作成され、実装が一致している
- 全イメージ / 指定ツールの両モードが動作し、失敗時に非 0 で終了するテストがある

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
- **対応**: `--help` 等で表示されるメッセージの正確性を期すため、`registry.go` の説明文を `"default masks all variables"` 等に更新することを推奨。

### 記憶 (Memory) と実装の乖離：環境変数マスキング

- **内容**: プロジェクトの記憶（Memory）では、`internal/config/masking.go` において `sensitiveKeywords` や `maxKeywordLen` を使用したキーワードベースの高度なマスキングが実装・最適化されているとあるが、実際のコード（およびベンチマーク）では `sensitive-env` が未指定（nil）の場合に一律で `[REDACTED]` を返す「Secure by Default (Mask-all)」が実装されている。
- **対応**: 今回のドキュメント更新では「実際の実装（Mask-all）」に合わせてドキュメントを修正した。キーワードベースのマスキングを復活・導入する場合は、別途実装タスクが必要。

---

## T23: `--group-add` フラグの追加

- 種別: 機能追加
- 優先度: 高
- 対象:
  - `internal/container/config.go`（`ContainerConfig`）
  - `internal/config/registry.go`（`StringSliceOptions`）
  - `internal/config/resolver.go`（`CLIOptions`, `ResolvedConfig`）
  - `internal/config/config.go`（`ConfigDefaults`, `ToolConfig`）
  - `internal/command/root.go`（`rootOptions`, `resolveSettings`, `buildContainerConfig`, `handleDryRun`）
  - `internal/command/flags.go`（`getStringSlicePointers`）
  - `internal/runtime/docker_adapter.go`（`HostConfig.GroupAdd`）
  - `internal/runtime/containerd.go`（OCI spec `Process.User.AdditionalGids`）
- 仕様変更: あり → `docs/features/command-line-options.md` を更新

### 背景

`--mount-tools` や `--mount-socket` 使用時に、コンテナ内のプロセスが Docker ソケットにアクセスするには、ソケットファイルの所有グループ（Mac Docker Desktop では GID 102 等）に所属している必要がある。現状、supplementary group を追加する手段がないため、非 root ユーザーのイメージでは `permission denied` で失敗する。

**実運用での報告（2026-07、要再検証）**: ネスト実行 + `--mount-tools` でツールが `/var/run/docker.sock` の権限不足により実行できない事象が報告されている。ただし当該環境ではソケット自体がマウントされていなかった可能性があり、原因が「GID 不足（EACCES）」か「ソケット未マウント（ENOENT）」かは切り分けが必要。着手時はまず再現環境で `ls -la /var/run/docker.sock` と `id` を確認し、エラー種別を特定してから対応すること。

### 追加検討: ソケット GID の自動付与

GID（102 等）を環境ごとに手で調べて設定するのは UX が悪く、環境間でポータブルでもない。`--mount-socket` 有効時に、ホスト側でソケットファイルを `stat` して所有 GID を自動的に supplementary group へ追加するモードを検討する:

- 案 A: `--group-add auto` の特殊値でソケット GID を解決する
- 案 B: `--mount-socket` 時はデフォルトで自動付与し、opt-out を用意する（挙動変更になるため要仕様判断。ただし「ソケットをマウントしたのにアクセスできない」状態に価値はほぼない）
- いずれの場合もソケットの `stat` 失敗時（リモートデーモン等）は警告のみで続行する
- ネスト実行時は、コンテナ内から見えるソケットの GID とホスト側の GID が一致するとは限らないため、スナップショット経由でホスト側の情報を伝搬する必要がある点に注意

### 仕様

#### フラグ

| フラグ | 短縮形 | 型 | デフォルト | 環境変数 |
| --- | --- | --- | --- | --- |
| `--group-add` | (なし) | stringArray | (なし) | `CDERUN_GROUP_ADD` |
| `--cderun-group-add` | (なし) | stringArray | - | - |

- Docker の `--group-add` と同じ形式: グループ名（例: `docker`）またはGID（例: `102`）
- 環境変数 `CDERUN_GROUP_ADD` はカンマ区切り（例: `102,docker`）

#### 設定ファイル

```yaml
# .cderun.yaml
defaults:
  groupAdd:
    - "102"

# .tools.yaml
git:
  image: alpine/git
  groupAdd:
    - "102"
```

#### ランタイム別の変換

- **Docker / Podman**: `HostConfig.GroupAdd []string` にそのまま渡す
- **containerd**: OCI spec の `Process.User.AdditionalGids []uint32` に変換（数値のみ。名前解決はコンテナの `/etc/group` に依存するため、containerd では数値GIDのみサポート。名前が渡された場合はエラーとする）

#### dry-run simple 出力

```text
GroupAdd: 102, docker
```

#### 優先順位

既存の P1 > P2 > P3 > P4 > P5 > P6 に従う（StringSliceOption 標準パターン）。

### 実装上の注意

- `registry.go` の `StringSliceOptions` に追加する際、`EnvSep` はカンマ（`,`）を使用（`cap-add` と同一パターン）
- `config.go` の `ConfigDefaults` / `ToolConfig` に `GroupAdd []string` を追加し、`DeepCopy` / `SetBaseDir` を適切に更新（`[]string` なので `DeepCopy` に `copyStringSlice` 追加が必要）
- containerd で名前→GID変換を行わない設計判断は、コンテナイメージ内の `/etc/group` をマウント前に読むことが困難なため

### 完了条件

- [ ] 全経路チェックリスト（`docs/guidelines/working-guide.md` Section 3）を満たす
- [ ] `docs/features/command-line-options.md` に `### --group-add` セクションが追加されている
- [ ] Docker: `HostConfig.GroupAdd` に正しく渡されるユニットテスト
- [ ] containerd: 数値GIDが `AdditionalGids` に変換されるユニットテスト
- [ ] containerd: 非数値が渡された場合にエラーになるユニットテスト
- [ ] dry-run simple format に `GroupAdd` が含まれるテスト
- [ ] 既存テストが全パス

---

## T24: `--shm-size` フラグの追加

- 種別: 機能追加
- 優先度: 高
- 対象: 全経路（registry / resolver / flags / docker_adapter / containerd）
- 仕様変更: あり → `docs/features/command-line-options.md` を更新

### 背景

Puppeteer / Playwright によるブラウザテスト、ML ワークロード（PyTorch DataLoader の共有メモリ）等で `/dev/shm` のサイズ不足が頻発する。Docker デフォルトは 64MB であり、多くのケースで不足する。

### 仕様

| フラグ | 型 | デフォルト | 環境変数 |
| --- | --- | --- | --- |
| `--shm-size` | string | (Docker デフォルト: 64MB) | `CDERUN_SHM_SIZE` |

- Docker の `--shm-size` と同一形式（例: `256m`, `1g`, `2147483648`）
- `docker/go-units` の `RAMInBytes` でパース
- Docker: `HostConfig.ShmSize int64`
- containerd: OCI spec の `/dev/shm` tmpfs マウントの `size` オプション

### 完了条件

- [ ] 全経路チェックリスト満たす
- [ ] `docs/features/command-line-options.md` に記載
- [ ] Docker / containerd 両方のユニットテスト

---

## T25: `--init` フラグの追加

- 種別: 機能追加
- 優先度: 高
- 対象: 全経路（registry / resolver / flags / docker_adapter / containerd）
- 仕様変更: あり → `docs/features/command-line-options.md` を更新

### 背景

コンテナの PID 1 問題。`--init` なしだとシグナルがアプリに届かずゾンビプロセスが残る場合がある。特に `cderun` はシグナルフォワーディングを行うが、コンテナ内プロセスが PID 1 としてシグナルを適切にハンドリングしない場合に問題になる。

### 仕様

| フラグ | 型 | デフォルト | 環境変数 |
| --- | --- | --- | --- |
| `--init` | bool | `false` | `CDERUN_INIT` |

- Docker: `HostConfig.Init *bool`
- containerd: tini 等の init バイナリをコンテナに注入する機構が必要。containerd 単体では直接サポートしないため、エラーまたは警告とする設計判断が必要

### 実装上の注意

- containerd での対応方針を設計時に決定すること（エラーにする / 警告のみ / tini バイナリマウント）

### 完了条件

- [ ] 全経路チェックリスト満たす
- [ ] `docs/features/command-line-options.md` に記載
- [ ] Docker: `HostConfig.Init` に渡るテスト
- [ ] containerd: 設計決定に基づいた挙動のテスト

---

## T26: `--pid` フラグの追加

- 種別: 機能追加
- 優先度: 高
- 対象: 全経路（registry / resolver / flags / docker_adapter / containerd）
- 仕様変更: あり → `docs/features/command-line-options.md` を更新

### 背景

`--pid=host` でホストの PID 名前空間を共有し、デバッグや strace 等で必要。`--pid=container:<id>` も Docker はサポートするが、cderun ではエフェメラルコンテナの特性上 `host` のみで十分。

### 仕様

| フラグ | 型 | デフォルト | 環境変数 |
| --- | --- | --- | --- |
| `--pid` | string | (空 = プライベート) | `CDERUN_PID` |

- 値: `host` または空文字列
- Docker: `HostConfig.PidMode`
- containerd: OCI spec の Linux namespaces で `pid` の `path` を設定

### 完了条件

- [ ] 全経路チェックリスト満たす
- [ ] `docs/features/command-line-options.md` に記載
- [ ] Docker / containerd 両方のユニットテスト

---

## T27: `--read-only` フラグの追加

- 種別: 機能追加
- 優先度: 高
- 対象: 全経路（registry / resolver / flags / docker_adapter / containerd）
- 仕様変更: あり → `docs/features/command-line-options.md` を更新

### 背景

ルートファイルシステムを読み取り専用にすることで、セキュリティ強化・コンテナ内の不正な書き込み防止が可能。CI 環境で特に有用。

### 仕様

| フラグ | 型 | デフォルト | 環境変数 |
| --- | --- | --- | --- |
| `--read-only` | bool | `false` | `CDERUN_READ_ONLY` |

- Docker: `HostConfig.ReadonlyRootfs`
- containerd: OCI spec の `Root.Readonly`

### 完了条件

- [ ] 全経路チェックリスト満たす
- [ ] `docs/features/command-line-options.md` に記載
- [ ] Docker / containerd 両方のユニットテスト

---

## T28: `--ulimit` フラグの追加

- 種別: 機能追加
- 優先度: 中
- 対象: 全経路（registry / resolver / flags / docker_adapter / containerd）
- 仕様変更: あり → `docs/features/command-line-options.md` を更新

### 背景

`nofile`（ファイルディスクリプタ上限）を増やす必要があるワークロードが多い。Node.js の大規模プロジェクト、Go の大量並行テスト等で必要。

### 仕様

| フラグ | 型 | デフォルト | 環境変数 |
| --- | --- | --- | --- |
| `--ulimit` | stringArray | (なし) | `CDERUN_ULIMIT` |

- 形式: `<type>=<soft>:<hard>` または `<type>=<value>`（例: `nofile=65535:65535`）
- 環境変数はカンマ区切り
- Docker: `Resources.Ulimits []*Ulimit`（`docker/go-units.Ulimit` 型）
- containerd: OCI spec の `Process.Rlimits`

### 実装上の注意

- パースは `docker/go-units` の `ParseUlimit` を使用可能
- `SkipResolution: true` にして `resolveComplexOptions` でカスタムパースが必要

### 完了条件

- [ ] 全経路チェックリスト満たす
- [ ] `docs/features/command-line-options.md` に記載
- [ ] パース + Docker / containerd 変換のユニットテスト

---

## T29: `--security-opt` フラグの追加

- 種別: 機能追加
- 優先度: 中
- 対象: 全経路（registry / resolver / flags / docker_adapter / containerd）
- 仕様変更: あり → `docs/features/command-line-options.md` を更新

### 背景

`no-new-privileges`、SELinux ラベル、AppArmor プロファイル等のセキュリティオプションを指定する。特に `no-new-privileges` は Docker のベストプラクティスとして推奨されている。

### 仕様

| フラグ | 型 | デフォルト | 環境変数 |
| --- | --- | --- | --- |
| `--security-opt` | stringArray | (なし) | `CDERUN_SECURITY_OPT` |

- 形式: `key=value` または `key:value`（Docker互換）
- 環境変数はカンマ区切り
- Docker: `HostConfig.SecurityOpt []string`
- containerd: OCI spec の対応フィールド（`no-new-privileges` → `Process.NoNewPrivileges` 等）

### 実装上の注意

- containerd では各オプションを個別にマッピングする必要がある（`no-new-privileges`, `seccomp=`, `apparmor=`, `label=`）
- サポート範囲を明確に文書化すること

### 完了条件

- [ ] 全経路チェックリスト満たす
- [ ] `docs/features/command-line-options.md` に記載
- [ ] Docker: `SecurityOpt` に渡るテスト
- [ ] containerd: サポート範囲のテスト

---

## T30: `--sysctl` フラグの追加

- 種別: 機能追加
- 優先度: 中
- 対象: 全経路（registry / resolver / flags / docker_adapter / containerd）
- 仕様変更: あり → `docs/features/command-line-options.md` を更新

### 背景

`net.ipv4.ip_forward` 等のカーネルパラメータをコンテナ単位で設定する。ネットワーク関連のテストや VPN コンテナで必要になることがある。

### 仕様

| フラグ | 型 | デフォルト | 環境変数 |
| --- | --- | --- | --- |
| `--sysctl` | stringArray | (なし) | `CDERUN_SYSCTL` |

- 形式: `key=value`（例: `net.ipv4.ip_forward=1`）
- 環境変数はカンマ区切り
- Docker: `HostConfig.Sysctls map[string]string`
- containerd: OCI spec の `Linux.Sysctl map[string]string`

### 実装上の注意

- `map[string]string` 型なので、既存の `StringSliceOption` パターンとは少し異なる。`SkipResolution: true` + カスタムパースが必要
- バリデーション: `key=value` 形式のチェック

### 完了条件

- [ ] 全経路チェックリスト満たす
- [ ] `docs/features/command-line-options.md` に記載
- [ ] パース + Docker / containerd 変換のユニットテスト

---

## T31: `--runtime` を `--engine` にリネーム + OCI `--runtime` 追加

- 種別: 機能追加 + 破壊的変更
- 優先度: 高
- 対象:
  - `internal/config/registry.go`（`StringOptions` の `runtime` エントリ）
  - `internal/command/root.go`（`rootOptions`, `resolveSettings`, `runtimeFactory`）
  - `internal/config/resolver.go`（`CLIOptions`, `ResolvedConfig`）
  - `internal/config/config.go`（`CDERunConfig.Runtime`）
  - `internal/runtime/docker_adapter.go`（`HostConfig.Runtime`）
  - すべてのテスト・ドキュメント
- 仕様変更: あり → `docs/features/command-line-options.md`, `docs/features/multi-runtime-support.md` を更新

### 背景

cderun の現行 `--runtime` は「どのコンテナエンジンに接続するか」（docker / podman / containerd）を指定するフラグ。一方 Docker の `--runtime` は「コンテナ実行時の OCI ランタイム」（runc / crun / nvidia / kata）を指定する。名前の衝突により、Docker 互換の OCI ランタイム指定が追加できない。

### 仕様

#### リネーム

| 旧 | 新 | 環境変数 |
| --- | --- | --- |
| `--runtime` | `--engine` | `CDERUN_ENGINE`（旧 `CDERUN_RUNTIME` は非推奨エイリアス） |
| `--cderun-runtime` | `--cderun-engine` | - |

#### 新規追加

| フラグ | 型 | デフォルト | 環境変数 | 説明 |
| --- | --- | --- | --- | --- |
| `--runtime` | string | (空 = Docker デフォルト) | `CDERUN_OCI_RUNTIME` | OCI ランタイムの指定 |

- Docker: `HostConfig.Runtime string`
- Podman: `--runtime` オプションとして透過的に渡す
- containerd: OCI spec の runtime 指定（または未サポートエラー）

#### 移行措置

- `CDERUN_RUNTIME` 環境変数は `CDERUN_ENGINE` のエイリアスとして残す（1リリースの deprecation 期間）
- `.cderun.yaml` の `runtime:` フィールドは `engine:` にリネーム、旧キーも読み込み可能にする

### 実装上の注意

- `resolveRuntimeAndSocket` 関数の変数名 `rv.res.Runtime` は `rv.res.Engine` にリネーム
- テストの grep 範囲が広いので一括リネームツール（`gorename` 等）を使用推奨
- P1 フラグの `--cderun-runtime` → `--cderun-engine` も忘れずに

### 完了条件

- [ ] `--engine` で docker/podman/containerd を指定可能
- [ ] `--runtime` で OCI ランタイムを指定可能（Docker のみ。containerd はエラー）
- [ ] `CDERUN_RUNTIME` が非推奨警告付きで動作する移行テスト
- [ ] `.cderun.yaml` の `runtime:` キーが非推奨警告付きで動作するテスト
- [ ] 全ドキュメント更新

---

## T32: `--dns-search` フラグの追加

- 種別: 機能追加
- 優先度: 中
- 対象: 全経路
- 仕様変更: あり → `docs/features/command-line-options.md` を更新

### 背景

社内 DNS 環境で検索ドメインを設定する必要がある場合に使用。Kubernetes 環境のテストでも `svc.cluster.local` 等の検索ドメインが必要になる。

### 仕様

| フラグ | 型 | デフォルト | 環境変数 |
| --- | --- | --- | --- |
| `--dns-search` | stringArray | (なし) | `CDERUN_DNS_SEARCH` |

- 環境変数はカンマ区切り
- Docker: `HostConfig.DNSSearch []string`
- containerd: OCI spec の DNS 設定

### 完了条件

- [ ] 全経路チェックリスト満たす
- [ ] `docs/features/command-line-options.md` に記載
- [ ] Docker / containerd 両方のユニットテスト

---

## T33: `--dns-option` フラグの追加

- 種別: 機能追加
- 優先度: 中
- 対象: 全経路
- 仕様変更: あり → `docs/features/command-line-options.md` を更新

### 背景

`resolv.conf` の `options` 行を設定。`ndots:5`, `timeout:2`, `attempts:3` 等。Kubernetes 互換の DNS 設定をコンテナに注入する場合に有用。

### 仕様

| フラグ | 型 | デフォルト | 環境変数 |
| --- | --- | --- | --- |
| `--dns-option` | stringArray | (なし) | `CDERUN_DNS_OPTION` |

- 環境変数はカンマ区切り
- Docker: `HostConfig.DNSOptions []string`
- containerd: OCI spec の DNS 設定

### 完了条件

- [ ] 全経路チェックリスト満たす
- [ ] `docs/features/command-line-options.md` に記載
- [ ] Docker / containerd 両方のユニットテスト

---

## T34: `--ipc` フラグの追加

- 種別: 機能追加
- 優先度: 中
- 対象: 全経路
- 仕様変更: あり → `docs/features/command-line-options.md` を更新

### 背景

`--ipc=host` で共有メモリセグメントをホストと共有。PostgreSQL のパフォーマンスチューニングや、プロセス間通信を使うアプリケーションのテストで必要。

### 仕様

| フラグ | 型 | デフォルト | 環境変数 |
| --- | --- | --- | --- |
| `--ipc` | string | (空 = プライベート) | `CDERUN_IPC` |

- 値: `host`, `private`, `shareable`, `none`, `container:<id>`
- cderun のユースケースでは `host` と `private` のみ実質的に使用される
- Docker: `HostConfig.IpcMode`
- containerd: OCI spec の Linux namespaces で `ipc` 設定

### 完了条件

- [ ] 全経路チェックリスト満たす
- [ ] `docs/features/command-line-options.md` に記載
- [ ] Docker / containerd 両方のユニットテスト

---

## T35: `--gpus` フラグの追加

- 種別: 機能追加
- 優先度: 中
- 対象: 全経路
- 仕様変更: あり → `docs/features/command-line-options.md` を更新

### 背景

ML ワークロードで NVIDIA GPU をコンテナにパススルーする。`docker run --gpus all` 相当の機能。

### 仕様

| フラグ | 型 | デフォルト | 環境変数 |
| --- | --- | --- | --- |
| `--gpus` | string | (なし) | `CDERUN_GPUS` |

- 値: `all`, `"device=0,1"`, `"count=2"` 等（Docker 互換）
- Docker: `Resources.DeviceRequests []DeviceRequest` に変換
- containerd: CDI (Container Device Interface) または nvidia-container-runtime 経由

### 実装上の注意

- Docker の `--gpus` は内部で `DeviceRequest` に変換する複雑なパース処理がある
- containerd では CDI spec を利用するか、`--runtime=nvidia` と組み合わせる形になる
- 初期実装は Docker のみ対応し、containerd は未サポートエラーで良い

### 完了条件

- [ ] 全経路チェックリスト満たす
- [ ] `docs/features/command-line-options.md` に記載
- [ ] Docker: `DeviceRequests` に変換されるユニットテスト
- [ ] containerd: 未サポートエラーのテスト（T16 と連携）

---

## T36: `--cgroupns` フラグの追加

- 種別: 機能追加
- 優先度: 中
- 対象: 全経路
- 仕様変更: あり → `docs/features/command-line-options.md` を更新

### 背景

cgroup v2 環境でのネームスペース分離を制御。Docker Engine 20.10+ ではデフォルトで `private` だが、ホストの cgroup ツリーを見たい場合に `host` を指定する。

### 仕様

| フラグ | 型 | デフォルト | 環境変数 |
| --- | --- | --- | --- |
| `--cgroupns` | string | (空 = Docker デフォルト) | `CDERUN_CGROUPNS` |

- 値: `host`, `private`
- Docker: `HostConfig.CgroupnsMode`
- containerd: OCI spec の Linux namespaces で `cgroup` 設定

### 完了条件

- [ ] 全経路チェックリスト満たす
- [ ] `docs/features/command-line-options.md` に記載
- [ ] Docker / containerd 両方のユニットテスト

---

## T37: `--pids-limit` フラグの追加

- 種別: 機能追加
- 優先度: 中
- 対象: 全経路
- 仕様変更: あり → `docs/features/command-line-options.md` を更新

### 背景

fork bomb 対策。CI 環境やマルチテナント環境でプロセス数を制限する。

### 仕様

| フラグ | 型 | デフォルト | 環境変数 |
| --- | --- | --- | --- |
| `--pids-limit` | int64 | (なし = 無制限) | `CDERUN_PIDS_LIMIT` |

- `0` または `-1` は無制限
- Docker: `Resources.PidsLimit *int64`
- containerd: OCI spec の `Linux.Resources.Pids.Limit`

### 完了条件

- [ ] 全経路チェックリスト満たす
- [ ] `docs/features/command-line-options.md` に記載
- [ ] Docker / containerd 両方のユニットテスト

---

## T38: `--cpu-shares` / `--cpuset-cpus` / `--cpuset-mems` フラグの追加

- 種別: 機能追加
- 優先度: 中
- 対象: 全経路
- 仕様変更: あり → `docs/features/command-line-options.md` を更新

### 背景

`--cpus` よりも細かいリソース制御が必要な場合に使用。`--cpu-shares` は相対的な CPU ウェイト、`--cpuset-cpus` は特定 CPU コアへのピン止め。

### 仕様

| フラグ | 型 | デフォルト | 環境変数 |
| --- | --- | --- | --- |
| `--cpu-shares` | int64 | (なし) | `CDERUN_CPU_SHARES` |
| `--cpuset-cpus` | string | (なし) | `CDERUN_CPUSET_CPUS` |
| `--cpuset-mems` | string | (なし) | `CDERUN_CPUSET_MEMS` |

- `--cpu-shares`: 相対ウェイト（デフォルト 1024）
- `--cpuset-cpus`: CPU セット（例: `0-3`, `0,1`）
- `--cpuset-mems`: メモリノード（例: `0-1`）
- Docker: `Resources.CPUShares`, `Resources.CpusetCpus`, `Resources.CpusetMems`
- containerd: OCI spec の `Linux.Resources.CPU`

### 完了条件

- [ ] 全経路チェックリスト満たす
- [ ] `docs/features/command-line-options.md` に記載
- [ ] Docker / containerd 両方のユニットテスト

---

## T39: `--restart` フラグの追加

- 種別: 機能追加
- 優先度: 低
- 対象: 全経路
- 仕様変更: あり → `docs/features/command-line-options.md` を更新

### 背景

cderun はエフェメラルコンテナを前提としているが、開発中のサーバープロセスをクラッシュ時に自動再起動させたいケースがある。`--remove=false` と組み合わせて使用する。

### 仕様

| フラグ | 型 | デフォルト | 環境変数 |
| --- | --- | --- | --- |
| `--restart` | string | `no` | `CDERUN_RESTART` |

- 値: `no`, `always`, `on-failure[:max-retries]`, `unless-stopped`
- Docker: `HostConfig.RestartPolicy`
- containerd: 未サポート（エラー）— containerd はデーモンとしてのリスタート管理を持たない

### 実装上の注意

- `--remove=true`（デフォルト）と `--restart` の組み合わせは Docker ではエラーになる。cderun 側でバリデーションし、分かりやすいエラーメッセージを出す
- パース: `on-failure:3` のようなコロン区切り形式のパースが必要

### 完了条件

- [ ] 全経路チェックリスト満たす
- [ ] `docs/features/command-line-options.md` に記載
- [ ] `--remove=true` + `--restart` の排他バリデーションテスト
- [ ] Docker: `RestartPolicy` に変換されるテスト
- [ ] containerd: 未サポートエラーのテスト

---

## T40: containerd: コマンド指定時にイメージの ENTRYPOINT が消失する

- 種別: バグ修正
- 優先度: 高
- 対象: `internal/runtime/containerd.go:184-199`

### 問題

`config.Entrypoint` が空で `config.Command` が非空の場合、`args = config.Command` を `oci.WithProcessArgs(args...)` で渡している。`WithProcessArgs` は `Process.Args` を丸ごと置き換えるため、`oci.WithImageConfig(img)` が設定したイメージの ENTRYPOINT が破棄される。Docker 経路（`docker_adapter.go:25,33`）は `Cmd` のみ設定しデーモンが ENTRYPOINT を前置するため、挙動が食い違う。

例: `ENTRYPOINT ["git"]` のイメージを `cderun git status` で実行すると、Docker では `git status`、containerd では素の `status` を exec しようとして失敗する。

### 方針

`config.Entrypoint` が空の場合はイメージの OCI config を読み（`img.Spec(ctx)` / `images.Config` + unmarshal）、その Entrypoint を `config.Command` の前に連結してから `WithProcessArgs` に渡す。両方空の場合のみスキップ。

### 完了条件

- ENTRYPOINT を持つイメージ + passthrough-args の組み合わせで Docker と containerd の実行コマンドが一致する
- Entrypoint 明示指定 / Command のみ / 両方空 の各ケースのユニットテスト

---

## T41: snapshot 一時ディレクトリが `os.Exit` によりリークする

- 種別: バグ修正（リーク）
- 優先度: 高
- 対象: `internal/command/root.go:1157-1178`（cleanup defer と `o.exitFunc(exitCode)`）、`root.go:1205`（`localOpts.exitFunc = os.Exit`）、`internal/command/snapshot.go:54-132`

### 問題

`RunE` 内で snapshot クリーンアップが `defer` 登録されているが、同じクロージャが最後に `o.exitFunc(exitCode)`（本番では `os.Exit`）を呼ぶため defer が実行されず、`--mount-cderun` / `--mount-tools` / `--mount-all-tools`（または `HostContext` あり）の全実行で `TempDir()` に `cderun-snap-<uuid>` ディレクトリ（シリアライズ済み `.cderun.yaml` / `.tools.yaml` を含む）がリークする。テストは `exitFunc` を差し替えているため検出できない。

また `createSnapshot` 自身のエラーパス（`snapshot.go:63-73, 107-122`）でも `MkdirAll` 済みディレクトリが `RemoveAll` されない。

### 方針

- `exitFunc` 呼び出しの前に明示的にクリーンアップを呼ぶ、または exit code を typed エラー（例: `ExitCodeError`）で `RunE` から返し、`os.Exit` は `cmd.ExecuteContext` の完了後（`main` 側）でのみ呼ぶ構造にする（T47 と同じ構造変更で解決できるため統合推奨）
- `createSnapshot` 内にエラー時 `defer os.RemoveAll` を追加

### 完了条件

- 本番経路（`exitFunc = os.Exit`）で snapshot ディレクトリが削除されることを検証するテスト（`exitFunc` をフックして defer 実行を確認する等）
- `createSnapshot` のエラーパスでディレクトリが残らないテスト

---

## T42: 空文字サブコマンドで nil panic

- 種別: バグ修正
- 優先度: 高
- 対象: `internal/command/root.go:1087-1092`（subcommand 抽出）、`root.go:619` / `root.go:720` / `root.go:1149`（nil deref 箇所）

### 問題

`len(args) > 0` かつ `args[0] == ""` の場合、`subcommand == ""` となり `containerConfig` が nil のまま処理が進む。`cderun --dry-run ""` は `handleDryRun` で、`cderun "" foo` は `execute`（`root.go:720`）または snapshot ブロック（`root.go:1149`）で nil deref panic する。resolver の image ガード（`resolver.go:865`）も `subcommand != ""` 条件のため素通りする。シェルで空の変数を quote して渡す（`cderun "$TOOL" ...`）と実際に発生し得る。

### 方針

`root.go:1089` で `args[0] == ""` を「サブコマンドなし」として扱い、help 表示または明示エラーにする。

### 完了条件

- `cderun ""`、`cderun --dry-run ""`、`cderun "" foo` が panic せず明示的なエラー（または help）になるテスト

---

## T43: attach エラー分岐で hang-timeout 0 が「即時タイムアウト」になる

- 種別: バグ修正
- 優先度: 高
- 対象: `internal/command/root.go:982-994`（attach エラー分岐の `time.After(effectiveHangTimeout)`）、`internal/command/root_termination.go:12-24`（`getHangTimeout`）

### 問題

`AttachContainer` がコンテナ終了前に失敗した場合の分岐で `case <-time.After(effectiveHangTimeout)` を使っているが、TTY+interactive セッション（`getHangTimeout` が 0 を返す）やユーザー指定の `--hang-timeout 0` では `time.After(0)` がほぼ即時発火し、待たずに exit code 0 のまま返る。ドキュメント（`docs/features/hang-timeout.md`）の「`0` = 無期限に待機」と真逆の挙動。同関数内の正常分岐（`root.go:1000, 1023-1031`）は `if effectiveHangTimeout > 0` で正しく分岐している。

### 方針

正常分岐と同じ `effectiveHangTimeout > 0` の構造をエラー分岐にも適用する。

### 完了条件

- hang-timeout 0 + attach エラー時に `waitDone` を無期限に待つことのテスト
- 既存の hang-timeout テストの回帰確認

---

## T44: `preprocessArgs` のフラグ lookup が機能せずサブコマンドを誤認する

- 種別: バグ修正
- 優先度: 高
- 対象: `internal/command/root.go:1252, 1260`（`cmd.Flags().Lookup` / `ShorthandLookup`）、対比: `root.go:1313-1316`（第 2 ループは `PersistentFlags()` フォールバックあり）、`internal/command/flags.go:140`（全フラグは `PersistentFlags()` に登録）

### 問題

cobra は persistent flag を `Flags()` に遅延マージする（`ParseFlags`/`execute` 時）ため、`cmd.ExecuteContext` より前に走る `preprocessArgs` の時点では `cmd.Flags().Lookup` が常に nil を返し、値付きフラグの値スキップが一切行われていない。結果、`cderun --image alpine --cderun-tty sh` では `alpine` がサブコマンドと誤認され、仕様上のエラー `"cderun internal override flag %q must be placed after the subcommand"` が発生せず P1 フラグが黙って受理される。既存テストはこの 2 挙動を区別できるケースを含まない（検証済み）。

### 方針

第 2 ループと同じ `PersistentFlags().Lookup` フォールバックを第 1 ループにも適用する（または先に `cmd.LocalFlags()` を一度呼んでマージを強制する）。T07 のリライトを行う場合はその中で解消してもよいが、判別テストケースは必ず追加する。

### 完了条件

- `cderun --image alpine --cderun-tty sh` が仕様どおりエラーになるテスト
- 値付き P2 フラグ + サブコマンドの組み合わせでサブコマンド検出が正しいことのテスト

---

## T45: containerd: cap-add / cap-drop / dns / add-host が黙って無視される

- 種別: セキュリティ / バグ修正
- 優先度: 高
- 対象: `internal/runtime/containerd.go:153-269`（マッピング欠落）、対比: `internal/runtime/docker_adapter.go:56-59`

### 問題

containerd の `CreateContainer` は `Network` / ports を明示的に拒否する（`containerd.go:171-176`）一方、`config.CapAdd` / `CapDrop` / `DNS` / `AddHosts` は黙って捨てている。`--cap-drop ALL` が成功したように見えてコンテナはデフォルト capability のまま動く「見せかけのハードニング」になり、セキュリティ上危険。

### 方針

- CapAdd / CapDrop は `oci.WithAddedCapabilities` / `oci.WithCapabilities` でマッピングする
- DNS / AddHosts は実装するか、network/ports と同じ「not supported yet」の明示エラーにする
- T16（事前バリデーション）の対象リストに capability / DNS / AddHosts を明記して連携する

### 完了条件

- containerd で `--cap-add` / `--cap-drop` が OCI spec に反映されるユニットテスト
- DNS / AddHosts が「反映される」か「明示エラーになる」かのどちらかであるテスト（黙殺の排除）

---

## T46: 設定レイヤーのマージで `BaseDir` が汚染される

- 種別: バグ修正
- 優先度: 中
- 対象: `internal/config/config.go:135-144`（`ConfigDefaults.SetBaseDir`）、`config.go:269-278`（`ToolConfig.SetBaseDir`）、マージ箇所 `config.go:530, 575`

### 問題

`ConfigDefaults.SetBaseDir` / `ToolConfig.SetBaseDir` は `MountCderunPath.BaseDir` / `MountSocketPath.BaseDir` を **`Raw == ""` でも無条件に** 設定する。`mergo.Merge(..., WithOverride)` はフィールド単位で上書きするため、上位レイヤーが `mountCderunPath` を設定していなくても非空の `BaseDir` だけが下位レイヤーの値を上書きし、`Raw` は下位レイヤーのものが残る。結果、例えば `~/.config/cderun/.cderun.yaml` に書いた相対 `mountCderunPath` がプロジェクトディレクトリ基準で解決される。`CDERunConfig.SetBaseDir`（`config.go:37-39`）や `MountConfig` / `DeviceConfig`（`path.go:132-139, 263-270`）は `Raw != ""` ガードで正しく実装されている。

### 方針

両 `SetBaseDir` の `BaseDir` 代入を `Raw != ""` でガードする（既存の正しい実装に揃える）。

### 完了条件

- 「下位レイヤーで相対 `mountCderunPath` を指定 + 上位レイヤーで未指定」の合成テストで、下位レイヤーのディレクトリ基準で解決されること

---

## T47: エラー時にコンテナの exit code が破棄される

- 種別: 改善（堅牢性）
- 優先度: 中
- 対象: `internal/command/root.go:1174-1177`、`main.go:11-14`
- 仕様変更: あり → exit code の仕様を `docs/features/`（該当ドキュメント）に明記

### 問題

`waitForCompletion` は attach 失敗・タイムアウト時にもコンテナの exit code を収集して返す（`root.go:972, 994, 1020`）が、`RunE` は `err != nil` の時点で exit code を捨てて error だけ返し、`main` は一律 exit 1 で終了する。CI で最も重要な「フレーキーな attach でも実コマンドの exit status を返す」ケースが失われる。

### 方針

- exit code を保持する typed エラー（例: `ExitCodeError{Code int}`）を導入し、`main` 側で判別して exit する
- cderun 内部エラーには docker CLI 互換の 125/126/127 系の採用を検討し、採用する場合は仕様化する
- T41（`os.Exit` による defer スキップ）と同じ構造変更なので **統合実装を推奨**

### 完了条件

- attach エラー + 非 0 exit code のケースで実 exit code がプロセスの終了コードになるテスト
- exit code 仕様がドキュメント化されている

---

## T48: Docker AutoRemove と `WaitContainer` の競合で exit code が失われる

- 種別: バグ修正（競合）
- 優先度: 中
- 対象: `internal/runtime/docker.go:206-217`（`WaitContainer`）、`internal/runtime/docker_adapter.go:53`（`AutoRemove = config.Remove`）、`internal/command/root.go:767, 951`

### 問題

`HostConfig.AutoRemove = config.Remove`（デフォルト true）の状態で、`StartContainer` 後に `ContainerWait(..., WaitConditionNotRunning)` を発行している。コンテナが即終了してデーモンが auto-remove を完了した後に wait が登録されると `errC` が "No such container" となり、実 exit code が失われて "failed to wait for container" になる。Docker CLI は auto-remove 時に `WaitConditionRemoved` を使い、start 前に wait を登録することでこれを回避している。

### 方針

- `config.Remove` に応じて wait condition を `WaitConditionRemoved` に切り替える（`WaitContainer` に Remove を渡すか、Create 時に記憶する）
- 加えて NotFound エラー時は失敗ではなくグレースフルにフォールバックする

### 完了条件

- 即終了コンテナ（例: `true` コマンド）+ `--remove` で exit code が正しく取得できるテスト（フレーク検証のため繰り返し実行）

---

## T49: Docker 明示 Remove で匿名ボリュームがリークする

- 種別: バグ修正（リーク）
- 優先度: 中
- 対象: `internal/runtime/docker.go:220-230`

### 問題

`RemoveContainer` の `RemoveOptions{Force: true}` に `RemoveVolumes: true` がない。`VOLUME` を宣言するイメージでは、明示 remove 経路（`root.go:812` のクリーンアップが AutoRemove に勝った場合等）を通るたびに匿名ボリュームが残る。デーモン側の auto-remove は匿名ボリュームを削除するため、明示経路だけの問題。エフェメラルコンテナツールとしては一行で直せるリーク。

### 方針

`RemoveVolumes: true` を追加する（containerd 側は既に `WithSnapshotCleanup` 使用済み）。

### 完了条件

- `RemoveOptions` に `RemoveVolumes: true` が渡ることのユニットテスト

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

## T52: コンテナ起動前後のシグナルハンドリングの隙間（SIGHUP 含む）

- 種別: 改善（堅牢性）
- 優先度: 中
- 対象: `internal/command/root.go:735, 757, 767, 836-864`、`internal/command/signals_unix.go:12-14`
- 仕様変更: あり → `docs/features/signal-handling-security.md` の更新

### 問題

1. `signal.Notify` はコンテナ start 直前（`root.go:757`）まで設置されないため、`PullImage` / `CreateContainer` 中の SIGINT/SIGTERM はデフォルト動作でプロセスを即死させる。`CreateContainer` 完了後〜Notify 前に受けると deferred remove が走らず orphan コンテナになる
2. forwarder 開始（757）〜 `StartContainer`（767）の間に受けたシグナルは「未起動コンテナへの転送失敗（warn のみ）」で消費され、その後コンテナは何事もなく起動する — CI の SIGTERM が黙って握りつぶされる
3. `sigChan` のバッファが 1 のため、`SignalContainer` ブロック中の連続シグナルが落ちる
4. SIGHUP を Notify していないため、ターミナルクローズで即死し orphan コンテナが残る（T22 の `--prune` は事後対策であり予防にならない）

### 方針

- `initContainer` より前に `signal.Notify` を設置し、コンテナ起動まではシグナルをキューイングするか context キャンセルで作成を中断する
- チャネルバッファを増やす（例: 4）
- SIGHUP（必要なら SIGQUIT も）を転送対象に加える。意図的に除外する場合はその設計判断を `signal-handling-security.md` に明記する

### 完了条件

- pull/create 中のシグナルで orphan コンテナが残らないテスト
- 起動直前のシグナルが失われず、起動後に転送されるかプロセスが安全に中断されるテスト
- SIGHUP の扱いが実装とドキュメントで一致している

---

## T53: 引数ホイストの `--` エスケープ対応

- 種別: 挙動変更
- 優先度: 中
- 対象: `internal/command/root.go:1299-1325`（ホイストループ）
- 仕様変更: あり → `docs/features/argument-parsing.md` の更新必須

### 問題

ホイストはサブコマンド後の全引数を走査し `--` (end-of-flags) を認識しないため、`cderun echo -- --cderun-tty` でもホイストされてしまい、リテラルの `--cderun-*` 文字列をコンテナコマンドに渡す手段が存在しない。

### 方針

リテラル `--` 以降はホイスト対象外とする。`--` 自体を strip するか残すかを仕様として決定し `argument-parsing.md` に明記する。T07 のリライトと同時に実装するのが効率的。

### 完了条件

- `--` 以降の `--cderun-*` がコンテナに素通しされるテスト
- 仕様が `docs/features/argument-parsing.md` に記載されている

---

## T54: 環境変数の bool/int/float パース失敗が黙殺される

- 種別: 改善（堅牢性）
- 優先度: 中
- 対象: `internal/config/option.go:87-93`（bool）、`option.go:208-214`（float64）、`option.go:247-253`（int）

### 問題

`CDERUN_TTY=yes`、`CDERUN_PULL_MAX_RETRIES=abc`、`CDERUN_CPUS=two` などパース不能な環境変数は診断なしで下位優先度ソースにフォールスルーする。未知 YAML キーや未知ディレクティブ（T11）をエラーにする本パッケージの厳格な姿勢と矛盾し、挙動が黙って変わる。

### 方針

環境変数が設定されているのにパース不能な場合は `InvalidConfigError` を返す（ヘルパーにエラー戻り値を通す必要あり）。互換性を重視するなら最低限 `logging.Warn` を出す。

### 完了条件

- 不正な bool/int/float 環境変数がエラー（または警告）になるテーブルドリブンテスト

---

## T55: CLI `--device` が不正な perms を黙認する

- 種別: 改善（堅牢性）
- 優先度: 低
- 対象: `internal/config/path.go:345-384`（`ParseDeviceConfig`）、`path.go:199-210`（YAML 側の重複検証）、`internal/config/resolver_helpers.go:85-99`

### 問題

`ParseDeviceConfig("/dev/x:/dev/y:bogus")` は `bogus` が `permsRegex` に不一致でも黙ってコンテナパスに折り込む（`Destination = "/dev/y:bogus"`、perms は `rwm`）。YAML 経路の `UnmarshalYAML` は明示的な perms 検証でエラーにするため、CLI / `CDERUN_DEVICE` 経路とで挙動が非対称。

### 方針

perms 検証を `ParseDeviceConfig` 側に移し（最終セグメントが `^[rwm]+$` 不一致かつ残りが有効ペアなら `ok=false`）、`UnmarshalYAML` の重複検証を削除する。

### 完了条件

- CLI / env / YAML の 3 経路で不正 perms が同一のエラーになるテスト

---

## T56: ポート番号の範囲検証（0 / 負数 / 65535 超）

- 種別: 改善（堅牢性）
- 優先度: 低
- 対象: `internal/config/path.go:753-809`（`ValidatePort`）、`path.go:876-906`（`ValidateExposePort`）

### 問題

数値チェックが素の `strconv.Atoi` のみで、`-1`、`+80`、`0`、`70000` を受理する。`-p 70000:80` や `--expose -5` が「セキュリティ検証」を通過し、ランタイム層で不明瞭に失敗する。

### 方針

共通の `validatePortNumber(s string, allowZero bool)` で 1〜65535 を強制（host port の 0 =ランダム割当のみ許可）。`expose` の範囲指定には開始 ≤ 終了のチェックも追加。

### 完了条件

- 境界値（0 / 1 / 65535 / 65536 / 負数 / `+80`）のテーブルドリブンテスト

---

## T57: `{{file:...}}` のサブパス許可と設定ファイルの信頼境界

- 種別: セキュリティ（設計判断が必要）
- 優先度: 中
- 対象: `internal/config/expression.go:369-371`（絶対パス・`..` のみ拒否）、対比: `expression.go:439`（`find_dir` は `/` `\` を全面拒否）、`internal/config/config.go:440-486`（`FindConfigs` の上方探索）
- 仕様変更: あり → `docs/features/value-resolution.md` に脅威モデルを明記

### 問題

`resolveFile` は `.ssh/id_rsa` のような相対サブパスを受理し、`FindConfigs` は cwd から `/` まで祖先ディレクトリを探索して設定を自動ロードする。クローンしたリポジトリの `.tools.yaml` に `env: ["X={{file:.ssh/id_rsa}}"]` が仕込まれていた場合、`$HOME` 配下の cwd で `cderun <tool>` を実行するだけで `~/.ssh/id_rsa` が読まれ、ネットワークアクセスし得るコンテナへ注入される。`find_dir` がパス区切りを全面拒否しているのと非対称。

### 方針（いずれかを設計判断）

1. `file:` を `find_dir` と同様に単一ファイル名に制限する
2. プロジェクトサブツリー内の dotfile 風の名前に制限する
3. direnv 流の trust プロンプト（所有者が異なる設定の初回確認）を導入する
4. 最低限、現状の脅威モデルを `value-resolution.md` に明記する

### 完了条件

- 採用した方針が実装され、`docs/features/value-resolution.md` に脅威モデルとともに記載されている
- 祖先ディレクトリの機密ファイル読み出しシナリオのテスト（採用方針に応じて「拒否される」or「明示 opt-in が必要」）

---

## T58: ランタイム自動検出が substring マッチで誤検出し得る

- 種別: 改善（堅牢性）
- 優先度: 低
- 対象: `internal/config/resolver.go:953-961`

### 問題

`strings.Contains(SocketPath, "podman")` のようなパス全体への substring マッチのため、`/home/podman-migration/docker.sock` が podman と誤検出される等、親ディレクトリ名の影響を受ける。

### 方針

パスの basename（`docker.sock` / `podman.sock` / `containerd.sock`）でマッチし、不一致は docker フォールバックにする。

### 完了条件

- 紛らわしいパス（親ディレクトリにランタイム名を含む等）での検出結果のテーブルドリブンテスト

---

## T59: クリーンアップ用 `RemoveContainer` にタイムアウトがない

- 種別: 改善（堅牢性）
- 優先度: 低
- 対象: `internal/command/root.go:808-816`

### 問題

deferred cleanup は `context.WithoutCancel(ctx)` を使っており（ユーザーキャンセルを生き延びる意図は正しい）、上限がないためデーモンソケットが詰まるとワークロード完了後に cderun が永久にハングする。

### 方針

`context.WithTimeout`（例: 30s）でラップし、期限切れ時はログを出して諦める。

### 完了条件

- ハングする mock ランタイムで cleanup がタイムアウト後に返るテスト

---

## T60: duration オプションが式解決エラーを握りつぶす

- 種別: 改善（堅牢性）
- 優先度: 低
- 対象: `internal/config/resolver.go:696-709`（`applyDurationOption`）、対比: `resolver.go:734-744`（`applyMemoryOption` は正しい）

### 問題

`hang-timeout` / `pull-backoff-base` 内の `{{...}}` 式が失敗すると、未解決文字列が `time.ParseDuration` に渡り `invalid hang-timeout value "{{env:...}}"` という誤解を招くエラーになる。`applyMemoryOption` は先に `rv.r.Error()` をチェックして本来のディレクティブエラーを返している。

### 方針

`applyDurationOption` にも `if exprErr := rv.r.Error(); exprErr != nil { return exprErr }` を追加する（`resolver.go:699` 付近）。

### 完了条件

- 式解決に失敗する duration 値で、ディレクティブ本来のエラーが返るテスト

---

## T61: Docker attach: stdin エラー時に出力を drain せず切断する

- 種別: 改善
- 優先度: 低
- 対象: `internal/runtime/docker.go:351-358`

### 問題

`<-stdinDone` 分岐で stdin エラーがあると即 return し、deferred `resp.Close()` が出力コピーを途中で殺すため、stdin 失敗前後にコンテナが出力したデータが失われ得る。

### 方針

stdin エラー時も短い猶予（既存の `attachCloseWriteGrace` パターン）で `outputDone` を drain してから stdin エラーを返す。

### 完了条件

- stdin エラー発生時に直前のコンテナ出力が失われないテスト

---

## T62: containerd: `ioWait` 削除の競合と attach 順序契約の明文化

- 種別: 改善（保守性・潜在競合）
- 優先度: 低
- 対象: `internal/runtime/containerd.go:339-341`（`RemoveContainer` の `ioWait` 削除）、`containerd.go:278-284, 403-407, 448-453`、`internal/runtime/interface.go:23`

### 問題

1. `RemoveContainer` が `r.ioWait[containerID]` を削除するが、このエントリは `AttachContainer` が登録・削除の所有権を持つ。attach が `waitC` でブロック中に Remove が走ると `notifyWait` が no-op になり ctx キャンセルでしか抜けられない。現在の呼び出し順（defer LIFO）では顕在化しないが、T22 の prune が任意 ID に `RemoveContainer` を呼ぶと踏む
2. containerd の `AttachContainer` は `StartContainer` より先に呼ばれないと `cio.NullIO` にフォールバックして全 IO が黙って捨てられるが、この順序前提が `ContainerRuntime` インターフェースに文書化されていない（Docker は順序不問）

### 方針

- `containerd.go:339-341` の削除処理を除去する（ioMap/ioWait の所有権は Attach/Start 側に置く）
- `interface.go` の `AttachContainer` に順序契約をコメントで明記し、containerd の `StartContainer` が `NullIO` フォールバックする際は warn ログを出す

### 完了条件

- attach 中に Remove しても notify が失われないテスト
- インターフェースコメントと warn ログの追加

---

## T63: CI と `docs/testing/` のカバレッジ・パイプライン乖離の解消

- 種別: CI / ドキュメント
- 優先度: 中
- 対象: `.github/workflows/ci.yaml`、`docs/testing/coverage.md`、`docs/testing/runtime-tests.md`、`codecov.yml`

### 問題

`docs/testing/coverage.md`（Codecov 自動アップロード、unit ジョブの 86.5% fast-fail、Docker 20.10/25.0/29.0 の E2E マトリクス + カバレッジマージ）と `docs/testing/runtime-tests.md` の「CI 構成」（docker:dind マトリクス、Build-artifact ジョブ、`TEST_HOST_TMP_DIR`）が記述するパイプラインは、実際の `ci.yaml`（`go test -v ./...` + containerd 統合ジョブのみ）に存在しない。`codecov.yml` は `after_n_builds: 4` を期待するが CI は一度もアップロードしない。containerd ジョブは逆にドキュメント側に記載がない。

### 方針（設計判断が必要）

1. ドキュメントどおりのパイプラインを復元する（T20 の Docker/Podman ジョブ追加と統合可能）
2. または現状の CI に合わせて両ドキュメントと `codecov.yml` を書き直す

どちらを選ぶか判断し、選ばなかった側の記述を残さないこと。

### 完了条件

- `ci.yaml` と `docs/testing/` の記述が完全に一致している
- `codecov.yml` の期待値が実際のアップロード数と一致している（または Codecov を廃止）

### 完了確認（2026-07-06）

「現実を正としつつ理想像の価値ある部分を段階的に取り込む」方針で解消済み:

- `ci.yaml`: unit ジョブに `-coverprofile` + Codecov アップロード（`unit` フラグ）を追加
- `codecov.yml`: `after_n_builds: 4 → 1` に修正。project ステータスの 86.5% ハード閾値を `informational` に変更（`docs/testing/strategy.md` 第6節と整合）
- `docs/testing/coverage.md`: CI 節を実態（unit アップロードのみ + containerd ジョブ）に書き換え。runtime フラグは T20 で追加と明記
- `docs/testing/runtime-tests.md`: 実在しない DinD マトリクス / Build-artifact ジョブの記述を削除し、実在する containerd ジョブを記載。Docker 3 バージョンマトリクスは不採用と判断を明記
- 残タスク: T20 実装時に `runtime` フラグのアップロード追加と `after_n_builds` の更新（T20 に記載済みの完了条件と合わせて対応）

---

## T64: CLI help / Makefile の文字列修正（containerd・mask-all 反映）

- 種別: クリーンアップ
- 優先度: 低
- 対象: `internal/config/registry.go:210, 277`、`Makefile:16`

### 問題

1. `registry.go:210` の `--sensitive-env` Usage が "(default uses automatic keywords)" — 実装（`masking.go:18-21`）は未指定時に全値マスクであり、キーワードベースのマスキングはコードに存在しない
2. `registry.go:277` の `--runtime` Usage が "(docker/podman)" — containerd がサポート済み（`resolver.go:1180-1185`）。※ T31 のリネームが先に着手される場合はそちらに折り込む
3. `Makefile:16` の `test-runtime` の echo も "(Docker/Podman)" のまま

### 方針

- `--sensitive-env`: "(unset: all values masked; empty: masking disabled)" 等、実挙動に合わせる
- `--runtime`: "(docker/podman/containerd)" にする
- Makefile の echo に containerd を追記

### 完了条件

- 3 箇所の文字列が実装と一致し、help のスナップショットテスト（あれば）が更新されている

---

## T65: dead code 削除・小規模クリーンアップ一括

- 種別: クリーンアップ
- 優先度: 低
- 対象: 下記の各箇所（挙動変更なし）

### 内容

1. `internal/config/resolver.go:229-232` — 未使用の `ptr[T]` ヘルパー。`// ResolveWithFS combines...` の doc コメントが誤ってこの関数に付いている。関数を削除し、コメントを本来の `ResolveWithFS`（`resolver.go:748`）に移す
2. `internal/config/path.go:389` — `magicWordPreRegex` はコンパイルされるが全リポジトリで未参照。削除
3. `internal/config/resolver.go:323-325` — `initFieldInfo`（`resolver.go:283-285`）の挿入条件により到達不能な防御分岐。削除またはコメント化
4. `internal/runtime/docker_adapter.go:61` — `Tmpfs: make(map[string]string)` は生成後一切書き込まれない dead フィールド初期化（tmpfs は `Mounts` 経由で処理済み）。削除
5. `internal/runtime/docker.go:40` — `signalRegex` の第 2 選択肢が第 1・第 3 を包含しており実質 `^(?i)[A-Z0-9]+$`。containerd の `parseSignal`（`containerd.go:486-514`）流の検証に揃えるか正規表現を簡潔化
6. `internal/config/registry.go:208-217` — `sensitive-env` が `resolveEarly`（`resolver.go:830-849`）と `resolveCustomParsing`（`resolver.go:1086-1094`）で二重解決されている。registry エントリに `SkipResolution: true // resolved early in resolveEarly (needed for masking during resolution)` を付ける

### 完了条件

- 全項目適用後、既存テストが全パス（挙動変更なしの確認）

---

## T66: テスト専用ヘルパーを `_test.go` に移動

- 種別: クリーンアップ（保守性）
- 優先度: 低
- 対象: `internal/command/testhelpers.go`（`TerminationMockRuntime`, `RootErrorFS`）、`internal/command/run_helpers.go`（`runCderun`, `runCderunCore`）、`internal/command/runtime_helper.go`

### 問題

これらのシンボルは `*_test.go`（および相互）からしか参照されていない（grep 検証済み）のに通常ビルドに含まれ、`runtime.MockRuntime` / `config.MockFileSystem` を本番コンパイル単位に引き込んでいる。

### 方針

- `*_test.go` にリネームする（`runtime_helper.go` は `//go:build runtime` タグを `_test.go` ファイル内に維持）
- ついでに `findCderunBinary`（`runtime_helper.go:61-86`）が「`cderun` という名前のディレクトリ」を誤ってバイナリとして返す問題を `!info.IsDir()` チェックで修正する

### 完了条件

- 非テストビルドに mock 型が含まれない（`go build ./...` 後のシンボル確認 or ファイル構成レビュー）
- 既存テストが全パス

---

## T67: 早期ロガー初期化がフォーマット指定を無視し、不正レベルを黙殺する

- 種別: 改善（UX）
- 優先度: 低
- 対象: `internal/command/root.go:1069-1079`（`_ = o.logger.Init(initialLevel, "text", true)`）、`root.go:1120`（2 回目の Init）

### 問題

設定ロード中の早期ロガーは `CDERUN_LOG_FORMAT` / `--log-format` / `--log-timestamp` を無視して常に `text` + timestamp で出力し、不正な `--log-level` / `CDERUN_LOG_LEVEL` は黙って warn にフォールバックして 2 回目の Init まで報告されない。早期の debug ログを JSON で機械処理できず、レベルのタイポのエラーが遅れて出る。

### 方針

早期 Init でも CLI / env のフォーマット・タイムスタンプ指定を反映し、不正レベルは即時エラーにする。

### 完了条件

- 早期ログが指定フォーマットで出力されるテスト
- 不正な log-level が設定ロード前にエラーになるテスト

---

## T68: dry-run ゴールデンテスト基盤（L2）

- 種別: テスト基盤
- 優先度: 高
- 対象: `internal/command/`（または専用の `test/golden/` ディレクトリ）、`docs/testing/strategy.md` 第3節 L2
- 前提: `docs/testing/strategy.md` を必ず読むこと

### 目的

「CLI 呼び出し + 設定ファイル」の組み合わせコーパスに対して `--dry-run --dry-run-format json` の出力をスナップショットとして固定し、引数解析 → ホイスト → P1〜P6 解決 → `ContainerConfig` 構築のパイプライン全体の回帰をひとつの仕組みで検知する。デーモン不要・高速・hermetic。

### 方針

- コーパスは「1 ケース = 1 ディレクトリ」（`args.txt` + `.cderun.yaml` + `.tools.yaml` + `env.txt` + `expected.json`）等の宣言的な構造にし、ケース追加をデータ追加だけで行えるようにする
- スナップショット更新は明示フラグ（例: `-update`）でのみ許可
- 出力の環境依存部分（ホームディレクトリ、PWD 等）は正規化してから比較する
- **必須ケース**: T44 の判別ケース（`cderun --image alpine --cderun-tty sh` → エラー）、`--` エスケープ（T53）、ネスト実行のスナップショット設定、明示的な空リストによる上書き、`{{...}}` 式の解決
- 秘匿値マスキング（デフォルト mask-all）が dry-run 出力に効いていることもゴールデンで固定する

### 完了条件

- コーパス実行の共通ハーネスが存在し、`go test ./...` で毎 PR 実行される
- 主要機能（引数解析 / 優先順位 / マウント / env / 式解決）を各 1 ケース以上カバー
- ケース追加手順が `docs/testing/strategy.md` または専用 README に記載されている

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

## T70: `ContainerRuntime` コンフォーマンススイート（L3）

- 種別: テスト基盤
- 優先度: 高
- 対象: `internal/runtime/`（`conformance_test.go` 等）、`.github/workflows/ci.yaml`
- 依存: **T20（Docker/Podman の CI ジョブ）と統合実装を推奨**。T16（事前バリデーション）、T40/T45/T51（containerd パリティバグ）と密接に関連
- 前提: `docs/testing/strategy.md` を必ず読むこと

### 目的

「同じ `ContainerConfig` を渡したら、全ランタイム実装で同じ観測可能な挙動になる」ことを共通の契約テストスイートで保証する。T40（ENTRYPOINT 消失）・T45（cap-drop 黙殺）・T51（不正 OCI spec）のようなパリティバグをクラスごと検出可能にする。

### 方針

- `ContainerRuntime` の各メソッドに対する契約（例: 「Entrypoint 未指定 + Command 指定時はイメージの ENTRYPOINT が前置される」「未対応機能は明示エラーを返す」）をテスト関数群として定義し、実装をパラメータ化して全ランタイムで実行する
- Mock は毎 PR、実ランタイム（Docker / Podman / containerd）は CI ジョブ（`-tags=runtime`）で同一スイートを回す
- 「未対応」を返すことが正しいランタイムには、期待値を capability 宣言（T16 の `Capabilities()` 案）として表現できるとよい
- 検証用イメージは軽量なもの（alpine / busybox + ENTRYPOINT 付きカスタム）に固定し、digest 固定で再現性を確保

### 完了条件

- 契約スイートが Mock + 最低 1 つの実ランタイムで CI 実行されている
- T40 / T45 / T51 の再現ケースがスイートに含まれ、修正前は落ち、修正後に通ることが確認されている
- 新ランタイム追加時の手順（スイートへの組み込み方）が文書化されている

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

## T73: ソースコード内コメントの英語化 + `ContainerConfig` の変換契約コメント追加

- 種別: クリーンアップ
- 優先度: 中
- 対象: `internal/runtime/containerd_test.go`、`internal/container/config.go`、その他 Go ソース全域
- 前提: `AGENTS.md` の「English in Source Code」および「Runtime Adapter Conversion Contract」原則を参照

### 背景

本プロジェクトは public な OSS のため、ソースコード内のコメントは英語で統一する（`AGENTS.md` にルール化済み）。また、containerd で capability が `CAP_` プレフィックスなしのまま OCI spec に渡っていたバグの再発防止として、「Docker は暗黙に正規化するが OCI spec 直組み立てのランタイムでは変換責務がアダプタ側にある」という契約を `ContainerConfig` の doc comment として明文化する。

### 作業内容

1. **日本語コメントの英語化**: `internal/runtime/containerd_test.go` の `TestUnit_Containerd_NormalizeCapabilities` 直前のコメント（206-207 行付近）を英語化する。英訳案:

   ```go
   // docs/features/command-line-options.md: --cap-add / --cap-drop accept Docker-compatible
   // short names (e.g. SYS_ADMIN); the OCI spec requires the CAP_-prefixed form.
   ```

2. **残存する日本語コメントの掃引**: `grep -rn '[ぁ-ヿ一-鿿]' --include='*.go' .` で全 Go ソースを確認し、コメント・エラーメッセージ・ログメッセージ内の日本語を英語化する。
   **注意**: `internal/config/edge_cases_test.go:78` 付近の `ユーザー_TOKEN` は Unicode キーのマスキング検証用の**意図的なテストデータ**であり、対象外（変更しないこと）。
3. **`ContainerConfig` の契約コメント追加**: `internal/container/config.go` の `ContainerConfig` struct の doc comment に変換契約を追記する。ドラフト:

   ```go
   // ContainerConfig represents the intermediate representation of a container execution request.
   //
   // Field values hold Docker-CLI-compatible notation as entered by the user
   // (e.g. CapAdd: "SYS_ADMIN", not "CAP_SYS_ADMIN"). The Docker daemon normalizes
   // such notation implicitly, but runtimes that build an OCI spec directly
   // (containerd) do NOT: each runtime adapter is responsible for converting every
   // field it consumes into its native representation, and for returning an explicit
   // error for fields it cannot support — never pass a value through unconverted or
   // drop it silently.
   ```

### 完了条件

- Go ソース内のコメント・エラーメッセージ・ログメッセージから日本語が消えている（意図的なテストデータを除く）
- `ContainerConfig` の doc comment に変換契約が記載されている
- 挙動変更なし（既存テストが全パス）
