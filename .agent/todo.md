# TODO / Backlog

AI 開発エージェント（Jules 等）が個別タスクとして着手できるよう構造化したバックログ。

## エージェント向け共通ルール

- 着手前に必ず `AGENT.md` → `docs/guidelines/working-guide.md`を読むこと
- **Spec-First**: 「仕様変更あり」のタスクは対応する `docs/features/*.md` の更新が完了条件に含まれる
- テスト追加・修正時は `docs/testing/` 以下のドキュメント（特に `organization.md` の命名規則）を遵守
- 各タスクは自己完結している。原則 1 タスク = 1 PR とし、「完了条件」をすべて満たすこと
- 記録のファイルパス・行番号は 2026-06-04 時点のコードベースで検証済み（ずれていたら grep で再特定すること）

## タスク一覧（サマリ）

| ID | タイトル | 種別 | 優先度 | 規模 | 仕様変更 | ステータス |
| --- | --- | --- | --- | --- | --- | --- |
| T01 | TTY 経由実行でターミナルが強制終了する問題の調査 | 調査 | 高 | ? | - | - |
| T05 | `CLIOptions` の `Set` フィールドをポインタ型に統一 | リファクタ | 高 | 中 | - | - |
| T06 | `--cderun-*` フラグのボイラープレートをコード生成化 | リファクタ | 中 | 大 | - | - |
| T07 | `preprocessArgs` の引数ホイスト簡略化 | リファクタ | 中 | 中 | あり | - |
| T09 | `AttachContainer`（Docker）の stdin エラー握りつぶし修正 | バグ | 低 | 小 | - | DONE |
| T11 | 未知の `{{...}}` ディレクティブをエラーにする | 挙動変更 | 中 | 中 | あり | - |
| T12 | `IsRetryablePullError` を型付きエラー判定に移行 | 改善 | 中 | 小 | - | DONE |
| T14 | `Phase N` コメント前後の整理 | クリーンアップ | 低 | 小 | - | - |
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

依存関係・統合の注意:

- **T05 と T06 は統合可能**（`registry.go` のメタデータを single source of truth にすれば両方解決する）。別々に着手する場合は T05 → T06 の順
- **T22 は「ラベル付与」を先行サブタスクとして切り出し可能**（移行問題の縮小）

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
