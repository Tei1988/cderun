#!/usr/bin/env bash
set -euo pipefail

go test -count=1 -tags=runtime -v "$@" \
  -run '^TestConformance_LiveSocketRuntime$/^Lifecycle_' ./internal/runtime
go test -count=1 -tags=runtime -v "$@" \
  -run '^TestRuntime_' ./internal/command
