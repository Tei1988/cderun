#!/usr/bin/env bash
# AIエージェント（Google Jules 等）の実行環境セットアップスクリプト。
# 冪等に動作する。コンテナランタイム（Docker等）は不要（ユニットテストのみ実行可能な環境を作る）。
set -euo pipefail

cd "$(dirname "$0")/.."

echo "==> Checking Go toolchain..."
if ! command -v go >/dev/null 2>&1; then
    echo "ERROR: Go toolchain not found. Install Go 1.25+ first (https://go.dev/dl/)." >&2
    echo "       go.mod の go directive のバージョンは GOTOOLCHAIN により自動取得されます。" >&2
    exit 1
fi
go version

echo "==> Downloading module dependencies..."
go mod download

echo "==> Verifying the project builds..."
go build ./...

echo "==> Installing golangci-lint (optional, for 'make lint-go')..."
if ! command -v golangci-lint >/dev/null 2>&1; then
    # v2 系 → v1 系の順に試す。lint はCIでも実行されるため、失敗しても継続する。
    go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest 2>/dev/null ||
        go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest 2>/dev/null ||
        echo "WARN: golangci-lint のインストールに失敗しました（任意ツールのため続行します）"
fi

echo "==> Setup complete."
echo "    Verify:  make test && make lint-go"
echo "    Docs:    AGENTS.md / .agent/todo.md"
