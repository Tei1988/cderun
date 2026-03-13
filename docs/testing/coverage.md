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

GitHub Actions で実行されたテスト結果は自動的に [Codecov](https://codecov.io/gh/Tei1988/cderun) にアップロードされる。
Codecov 上では以下の機能が利用可能：

- プルリクエストごとのカバレッジ変化の確認
- ソースコード上でのカバレッジ可視化
- UnitテストとE2Eテストのフラグ別集計

## 3. インテグレーション・E2Eテストのカバレッジ計測

Dockerコンテナを起動するテストにおいて、`go test` プロセス内で行われるロジックのカバレッジは標準的な方法で計測される。

### CIでの計測

E2Eテストジョブでは、Dockerの複数バージョン（20.10, 25.0, 29.0）のマトリックスでテストが実行され、それぞれのカバレッジデータが Codecov 上でマージされる。

```bash
go test -v -tags=e2e -coverprofile=coverage-e2e.out ./internal/command/...
```

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

継続的インテグレーション (CI) プロセスにカバレッジ計測と Codecov へのアップロードが組み込まれている。

- GitHub Actions ([`ci.yaml`](../../.github/workflows/ci.yaml)) により、すべてのプッシュおよびプルリクエストにおいてカバレッジが計測される。
- **Unitテストジョブ**: ローカルでのカバレッジしきい値チェック（**86.5%**）を行い、満たない場合はジョブが失敗する（ファストフェイル）。
- **Codecov**: PR 上で詳細なステータスチェック（`unit` / `e2e` フラグ別のレポートを含む）を提供し、プロジェクト全体の品質管理を補完する。
- **E2Eテスト**: Dockerバージョンごとに計測され、Codecov 上で自動的にマージされる。

---
*2026年3月13日時点の仕様である。*
