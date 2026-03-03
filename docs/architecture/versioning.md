# バージョン情報の管理

`cderun` では、Git の情報に基づいた動的なバージョン管理を行っています。

## 概要

以前は `internal/command/version.go` にバージョン番号をハードコードしていましたが、現在はビルド時に `ldflags` を使用して情報を注入する方式に移行しました。
これにより、Git タグ、コミット SHA、およびビルド日時が自動的にバイナリに組み込まれます。

## 実装の詳細

### 1. バージョン情報の保持 (`internal/version`)

`internal/version` パッケージが、以下の変数を保持しています。

- `Version`: Git タグ（または `dev`）
- `Revision`: 短縮 Git コミット SHA
- `BuildDate`: ISO8601 形式のビルド日時

これらの変数は、`go build` 時に `-ldflags` オプションを通じて上書きされます。

### 2. ローカルビルド (`Makefile`)

開発環境で `make build` を実行すると、以下のコマンドが内部で実行され、Git から取得した最新情報が注入されます。

```bash
make build
```

`Makefile` 内では以下のように情報を取得しています。

- `VERSION`: `git describe --tags --always --dirty`
- `REVISION`: `git rev-parse --short HEAD`
- `BUILD_DATE`: `date -u +%Y-%m-%dT%H:%M:%SZ`

### 3. リリースビルド (`GoReleaser`)

正式なリリースバイナリは `GoReleaser` によって作成されます。`.goreleaser.yaml` の `ldflags` セクションにて、GoReleaser が提供する変数を `internal/version` の各変数にマッピングしています。

## 利用方法

### バイナリのバージョン確認

`--version` フラグを使用することで、詳細なバージョン情報を確認できます。

```bash
$ ./cderun --version
cderun version 0.0.2 (rev: abc1234, built at: 2026-03-02T12:34:56Z, linux/amd64)
```

### 開発時の挙動 (`go run`)

`ldflags` を指定せずに `go run main.go` 等で実行した場合、以下のフォールバック値が使用されます。

- `Version`: `dev`
- `Revision`: `unknown`
- `BuildDate`: `unknown`

これにより、そのバイナリが正式なビルド手順を経ていないことを識別できます。
