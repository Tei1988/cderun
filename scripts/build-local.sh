#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
BUILD_DIR="${PROJECT_DIR}/.build"
DEST="${HOME}/.config/cderun/.bin"

VERSION=$(git -C "${PROJECT_DIR}" describe --tags --always --dirty 2>/dev/null || echo "dev")
REVISION=$(git -C "${PROJECT_DIR}" rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS="-X cderun/internal/version.Version=${VERSION} -X cderun/internal/version.Revision=${REVISION} -X cderun/internal/version.BuildDate=${BUILD_DATE}"

mkdir -p "${BUILD_DIR}" "${DEST}"

cd "${PROJECT_DIR}"

for GOOS in darwin linux; do
  for GOARCH in amd64 arm64; do
    OUT="${BUILD_DIR}/cderun_${GOOS}_${GOARCH}"
    echo "Building ${OUT}..."
    export CGO_ENABLED=0 GOOS GOARCH
    go build -ldflags "${LDFLAGS}" -o "${OUT}" main.go
  done
done

echo "Copying to ${DEST}..."
cp "${BUILD_DIR}"/cderun_* "${DEST}/"

if [[ "$(uname)" == "Darwin" ]]; then
  echo "Signing darwin binaries..."
  codesign --sign - "${DEST}/cderun_darwin_amd64"
  codesign --sign - "${DEST}/cderun_darwin_arm64"
fi

echo "Done. Binaries in ${DEST}:"
ls -lh "${DEST}"
