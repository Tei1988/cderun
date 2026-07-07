# Feature: Test Coverage Reporting

[![codecov](https://codecov.io/gh/Tei1988/cderun/graph/badge.svg)](https://codecov.io/gh/Tei1988/cderun)

## 1. 概要

`cderun` のコードベースが、ユニットテストおよびE2Eテストによってどの程度カバーされているかを計測・可視化する仕組みを導入する。
これにより、テストが不十分な箇所を特定し、品質の高い開発サイクルを維持することが可能になる。

## 2. 実装方針

Go言語に標準で組み込まれているカバレッジ計測ツールを活用する。

### 2.1. カバレッジプロファイルの生成

`go test` コマンドに `-coverprofile` フラグを指定することで、カバレッジデータを file に出力する。

```bash
# プロジェクト全体のカバレッジプロファイルを coverage.out に生成
go test ./... -cover -coverprofile=coverage.out
```

### 2.2. レポートの閲覧

生成したプロファイルを用いて、複数の形式でレポートを確認できるようにする。

#### a) HTMLレポート (推奨)

ソースコード上で、どの行がテストでカバーされているかを視覚的に確認できるHTMLファイルを生成する。これが最も詳細で分かりやすい。

```bash
# coverage.out から coverage.html を生成
go tool cover -html=coverage.out -o coverage.html

# 生成されたファイルをブラウザで開く
open coverage.html # (macOS)
# または xdg-open coverage.html (Linux)
```

#### b) 関数別レポート (テキスト形式)

関数ごとのカバレッジ率をターミナルで素早く確認する。

```bash
go tool cover -func=coverage.out
```

#### c) Codecov (クラウド)

GitHub Actions の unit テストジョブで計測されたカバレッジは、`unit` フラグ付きで [Codecov](https://codecov.io/gh/Tei1988/cderun) にアップロードされる。
Codecov 上では以下の機能が利用可能：

- プルリクエストごとのカバレッジ変化の確認
- ソースコード上でのカバレッジ可視化
- フラグ別集計（現状は `unit` のみ。`runtime` フラグはランタイムテストの CI ジョブ追加時に有効化する）

## 3. ランタイムテストのカバレッジ計測

ランタイムテスト（`-tags=runtime`）でも、`go test` プロセス内で行われるロジックのカバレッジは標準的な方法で計測できる。

```bash
go test -v -tags=runtime -coverprofile=coverage-runtime.out ./internal/command/...
```

現状の CI ではランタイムテストのカバレッジ計測・アップロードは行っていない。Docker / Podman のランタイムテストジョブの追加（`.agent/todo.md` の T20）と同時に、`runtime` フラグ付きのアップロードを追加する計画である。

## 4. 自動化

開発者が容易にカバレッジレポートを生成できるよう、これらのコマンドを `Makefile` やシェルスクリプトにまとめる。

**Makefile のターゲット:**

```makefile
.PHONY: test
test:
	@echo "Running all tests..."
	@go test -v ./...

.PHONY: coverage
coverage:
	@echo "Generating coverage report..."
	@go test ./... -cover -coverprofile=coverage.out
	@echo "Done. To view HTML report, run: go tool cover -html=coverage.out"

.PHONY: coverage-html
coverage-html: coverage
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Generated coverage.html"
```

## 5. CIへの統合

GitHub Actions ([`ci.yaml`](../../.github/workflows/ci.yaml)) の構成は以下のとおり。

- **Unitテストジョブ**: すべてのプッシュおよびプルリクエストで `-coverprofile` 付きのテストを実行し、`unit` フラグで Codecov にアップロードする。
- **containerd 統合ジョブ**: 実際の containerd に対して `internal/runtime` のテストを実行する（カバレッジ計測なし。詳細は [runtime-tests.md](./runtime-tests.md) を参照）。
- **Codecov のステータス**: カバレッジは参考指標であり、目標値として扱わない（[テスト戦略 第6節](./strategy.md) を参照）。ステータスチェックはすべて `informational` であり、カバレッジ低下で CI は失敗しない。急落の検知と PR 上での可視化のために使う。
- **今後**: Docker / Podman のランタイムテストジョブ（T20）と `runtime` フラグのアップロード追加時に、`codecov.yml` の `after_n_builds` をレポート数に合わせて更新すること。
