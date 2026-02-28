# Feature: Test Coverage Reporting

## 1. 概要

`cderun` のコードベースが、ユニットテストおよびインテグレーションテストによってどの程度カバーされているかを計測・可視化する仕組みを導入する。
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

## 3. インテグレーションテストのカバレッジ計測

Dockerコンテナを起動するインテグレーションテストは、`go test` プロセスとは別の `cderun` プロセスを実行するため、標準的な方法ではカバレッジを計測できない。
これを解決するため、以下の手順を踏む。

1. **カバレッジ計測用テストバイナリのビルド**:
    `go test -c -cover` コマンドを使い、カバレッジ計測が可能なテスト専用バイナリを生成する。

```bash
go test -c ./internal/command -cover -o cderun.test
```

1. **テストバイナリの実行**:
    生成されたテストバイナリを実行する際に、カバレッジプロファイルの出力先を指定する。インテグレーションテストのロジック内で、このテストバイナリを `cderun` の実行ファイルとして使用する。

```bash
# テストコード内から、os/exec などで以下のように実行するイメージ
./cderun.test -test.run ^TestIntegration$ -test.coverprofile=integration.cover.out
```

1. **プロファイルの統合 (任意)**:
    ユニットテストとインテグレーションテストで別々に生成されたカバレッジプロファイルは、ツールを使ってマージし、プロジェクト全体のカバレッジとして集計することも可能。

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

## 5. CIへの統合 (既実装)

継続的インテグレーション (CI) プロセスにカバレッジ計測が組み込まれています。

- GitHub Actions ([e2e-test.yml](../../.github/workflows/e2e-test.yml)) により、すべてのプッシュおよびプルリクエストにおいて `make coverage` が実行されます。
- 詳細はワークフロー（[e2e-test.yml](../../.github/workflows/e2e-test.yml)）の `Verify Coverage Threshold` ステップを参照してください。
- 現在のカバレッジ率は **89.5%**（2026年2月時点）であり、しきい値（**86.5%**）を上回る非常に高い水準を維持しています。これらの値はワークフロー定義（`Verify Coverage Threshold` ステップ）と一致しています。
- カバレッジ率がしきい値を下回った場合、CIジョブは失敗します。
- `coverage.out` はアーティファクト (`coverage-report`) として保存され、詳細な解析に利用できます。
