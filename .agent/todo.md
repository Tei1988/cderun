# TODO / Backlog

AI 開発エージェント（Jules 等）が個別タスクとして着手できるよう構造化したバックログ。

## エージェント向け共通ルール

- 着手前に必ず `AGENT.md` → `docs/guidelines/working-guide.md` を読むこと
- **Spec-First**: 「仕様変更あり」のタスクは対応する `docs/features/*.md` の更新が完了条件に含まれる
- テスト追加・修正時は `docs/testing/` 以下のドキュメント（特に `organization.md` の命名規則）を遵守
- 各タスクは自己完結している。原則 1 タスク = 1 PR とし、「完了条件」をすべて満たすこと
- 記録のファイルパス・行番号は 2026-06-04 時点のコードベースで検証済み（ずれていたら grep で再特定すること）

## タスク一覧（サマリ）

| ID | タイトル | 種別 | 優先度 | 規模 | 仕様変更 |
| --- | --- | --- | --- | --- | --- |
| T01 | TTY 経由実行でターミナルが強制終了する問題の調査 | 調査 | 高 | ? | - |
| T05 | `CLIOptions` の `Set` フィールドをポインタ型に統一 | リファクタ | 高 | 中 | - |
| T06 | `--cderun-*` フラグのボイラープレートをコード生成化 | リファクタ | 中 | 大 | - |
| T07 | `preprocessArgs` の引数ホイスト簡略化 | リファクタ | 中 | 中 | あり |
| T08 | `MaskSensitiveEnv` を `sensitiveEnv` 明示指定方式に再設計 | 設計変更 | 中 | 中 | あり |
| T09 | `AttachContainer`（Docker）の stdin エラー握りつぶし修正 | バグ | 低 | 小 | - |
| T12 | `IsRetryablePullError` を型付きエラー判定に移行 | 改善 | 中 | 小 | - |
| T14 | `Phase N` コメント前後の整理 | クリーンアップ | 低 | 小 | - |
| T15 | containerd `AttachContainer` のポーリング排除 | 改善 | 低 | 小 | - |
| T16 | ランタイム未対応機能の事前バリデーション | 改善 | 中 | 中 | - |
| T18 | `ci.yaml` のアクションをコミットハッシュ固定 | CI | 高 | 小 | - |
| T19 | CI の Go バージョン指定を `go.mod` に一本化 | CI | 低 | 小 | - |
| T20 | Docker / Podman のランタイムテストを CI に追加 | CI | 中 | 中 | - |
| T21 | イメージ事前取得フラグ（`--prefetch`） | 機能 | 中 | 中 | あり |
| T22 | orphan コンテナのクリーンアップ（`--prune`） | 機能 | 中 | 大 | あり |

依存関係・統合の注意:

- **T05 と T06 は統合可能**（`registry.go` のメタデータを single source of truth にすれば両方解決する）。別々に着手する場合は T05 → T06 の順
- **T08 は旧項目「`ACCESS` キーワードによる偽陽性」を吸収済み**（T08 採用でキーワード方式自体が消えるため）
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

簡略化する場合も「`--cderun-*` は `=` 形式のみ許可する」と仕様を縛れば判定不要になり、最も安全。

### 完了条件

- 採用した仕様が `docs/features/argument-parsing.md` に反映されている
- サブコマンド前 `--cderun-*` の扱い、bool フラグ直後の引数の扱いについてテストがある

---

## T08: `MaskSensitiveEnv` を `sensitiveEnv` 明示指定方式に再設計

- 種別: 設計変更
- 対象: `internal/config/masking.go`
- 仕様変更: あり → `docs/features/sensitive-data-protection.md` の全面改訂必須
- 備考: 旧項目「`ACCESS` キーワードによる偽陽性」を本タスクに統合済み

### 問題（背景）

現在のキーワードベースの自動判定は偽陽性が多い。例えば `ACCESS` がセンシティブキーワード（`masking.go:10-27`）に含まれているため、`ACCESS_LOG` や `ACCESS_LEVEL` のような機密でない環境変数も `[REDACTED]` にマスクされ、dry-run 出力でのデバッグを妨げる。なお `ACCESS` が入っている経緯は `docs/features/sensitive-data-protection.md` に明記されており、`AWS_ACCESS_KEY_ID` のマスクが目的。

### 方針

- `sensitiveEnv` が未指定の場合: dry-run 出力で **全ての env 値を隠す**（安全なデフォルト）
- `sensitiveEnv` が指定されている場合: 指定したキーにマッチする値のみ隠し、それ以外は表示する
- 既存のキーワード自動判定ロジック（`masking.go`）は不要になるため削除

```yaml
defaults:
  sensitiveEnv:
    - "MY_API_KEY"
    - "DB_*"  # glob パターン対応も検討
```

### 実装上の注意

1. **Spec-First**: `docs/features/sensitive-data-protection.md` の全面改訂が必須（現行ドキュメントはキーワード方式を仕様として明記している）
2. マスクの適用箇所は dry-run（`root.go` の `config.MaskSensitiveEnvList`）だけでなくデバッグログ経路（`internal/config/resolver_helpers.go` の `logging.Debug("Resolved Env: ...")`）にもある。`MaskSensitiveEnv` の呼び出し元を全て洗うこと
3. glob は標準ライブラリの `path.Match` で足りる（`*` のみサポートで十分なら依存追加不要）
4. セマンティクス再検討の余地: 「指定キーにマッチする値のみ隠す」の逆（指定したものだけ **表示する** allowlist 方式）も検討すること。allowlist の方が「隠し忘れ」が起きない

### 完了条件

- どちらのセマンティクスを採用したかが docs に明記され、実装と一致している
- 全マスク経路（dry-run / デバッグログ）が新方式に切り替わっている
- 旧キーワードリストが削除されている

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
- 誤爆リスクあり。`--dry-run` 相当の確認表示を先に出してから削除する、または `--prune --force` で強制削除、など安全策を必須とする

### ランタイム別の実装メモ（検証済み）

1. **containerd**: 既に専用 namespace `cderun`（`containerd.go:26` の `defaultNamespace`）で動いているため、namespace 内の列挙だけで cderun 製コンテナを識別でき、ラベル不要
2. **Docker / Podman**: ラベル付与＋`ContainerList` の label フィルタ（`filters.Arg("label", "cderun=true")`）が必要。現状 `CreateContainer`（`docker_adapter.go`）はラベルを一切付けていないことを確認済み
3. **ラベル付与だけ先行リリース** しておくと、prune 実装時に「ラベルなしの古いコンテナは対象外」という移行問題が小さくなる → 先行サブタスク化を推奨
4. 実行中コンテナの扱い（並行する別の cderun が使用中）は **デフォルト除外** とし、停止済みのみ削除が安全

### 完了条件

- 仕様ドキュメントが先に作成され、実装が一致している
- 全ランタイムで cderun 製コンテナのみが対象になることのテストがある
- 実行中コンテナがデフォルトで除外される

---

