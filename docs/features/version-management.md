# バージョン管理

`cderun` は Git の情報を利用した動的なバージョン管理を行っています。

## 概要

実行バイナリには、ビルド時の Git タグ、コミット SHA、およびビルド日時が自動的に組み込まれます。これにより、デバッグ時や Issue 報告時に、どのバージョンのバイナリを使用しているかを正確に把握できます。

## 特徴

- **自動情報注入**: `Makefile` や `GoReleaser` を通じて、ビルド時に自動的に情報が埋め込まれます。
- **詳細な出力**: `--version` フラグにより、リビジョンやビルド日時を含む詳細な情報を表示します。
- **開発時フォールバック**: `go run` や直接の `go build`（ldflags なし）では、バージョンが `dev`、リビジョンが `unknown` として表示され、リリース版でないことが一目でわかります。

## 詳細仕様

### 保持される情報

| 項目 | 説明 | 例 |
| :--- | :--- | :--- |
| Version | Git タグまたは `dev` | `0.0.2`, `v1.1.0-dirty` |
| Revision | 短縮 Git コミット SHA | `abc1234` |
| BuildDate | ISO8601 形式のビルド日時 | `2026-03-02T12:34:56Z` |
| OS/Arch | 実行環境の OS とアーキテクチャ | `linux/amd64`, `darwin/arm64` |

### 出力フォーマット

`cderun --version` を実行した際の出力例：

```text
cderun version 0.0.2 (rev: abc1234, built at: 2026-03-02T12:34:56Z, linux/amd64)

```

## 実装の仕組み

### 1. `internal/version` パッケージ

バージョン情報を一元管理する独立したパッケージです。ビジネスロジックには依存せず、情報の保持と整形（`Info()` 関数）のみを担当します。

### 2. `ldflags` による注入

ビルド時に Go のリンカーフラグ (`-ldflags`) を使用して、`internal/version` パッケージ内の変数を直接書き換えます。

#### Makefile での例

```makefile
LDFLAGS := -X cderun/internal/version.Version=$(VERSION) \
           -X cderun/internal/version.Revision=$(REVISION) \
           -X cderun/internal/version.BuildDate=$(BUILD_DATE)

go build -ldflags "$(LDFLAGS)" -o cderun main.go

```

### 3. GoReleaser との統合

公式リリースの際は `.goreleaser.yaml` によって同様の注入が行われます。

```yaml
builds:
  - ldflags:
      - -X cderun/internal/version.Version={{.Version}}
      - -X cderun/internal/version.Revision={{.FullCommit}}
      - -X cderun/internal/version.BuildDate={{.Date}}

```

## 関連ドキュメント

- [アーキテクチャ: バージョン管理](../architecture/versioning.md)
